package collector

import (
	"context"
	"time"

	appconfig "github.com/nimishgj/gcp-radar/internal/config"
	"github.com/nimishgj/gcp-radar/internal/metrics"
	"github.com/rs/zerolog/log"
)

// Collector is one unit of collection (assets, billing). Each runs on every
// cycle and sets its own gauges.
type Collector interface {
	Name() string
	Collect(ctx context.Context) error
}

// Orchestrator runs the enabled collectors on an interval for one project.
type Orchestrator struct {
	projectID  string
	collectors []Collector
	interval   time.Duration
	timeout    time.Duration
	closers    []func() error
}

// NewOrchestrator builds the enabled collectors, creating the GCP clients they
// need via Application Default Credentials. Returns an error if a required
// client cannot be created.
func NewOrchestrator(ctx context.Context, cfg *appconfig.Config) (*Orchestrator, error) {
	o := &Orchestrator{
		projectID: cfg.GCP.ProjectID,
		interval:  cfg.Collection.Interval,
		timeout:   cfg.Collection.Timeout,
	}

	// A client that cannot be created (e.g. missing credentials) is logged and
	// its collector skipped, so the agent still serves /metrics and /health
	// instead of crash-looping. The absence of its series + a log line signal
	// the problem.
	if cfg.Assets.Enabled {
		searcher, err := newAssetSearcher(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Skipping assets collector: could not create Cloud Asset Inventory client")
		} else {
			o.collectors = append(o.collectors, NewAssetCollector(cfg.GCP.ProjectID, cfg.Assets, searcher))
			o.closers = append(o.closers, searcher.Close)
		}
	}

	if cfg.Billing.Enabled {
		querier, err := newBillingQuerier(ctx, cfg.Billing.BillingProjectID, cfg.Billing.Location)
		if err != nil {
			log.Error().Err(err).Msg("Skipping billing collector: could not create BigQuery client")
		} else {
			o.collectors = append(o.collectors, NewBillingCollector(cfg.GCP.ProjectID, cfg.Billing, querier))
			o.closers = append(o.closers, querier.Close)
		}
	}

	if len(o.collectors) == 0 {
		log.Warn().Msg("No collectors enabled; nothing will be collected")
	}

	return o, nil
}

// Start runs an initial collection, then one per interval until ctx is done.
func (o *Orchestrator) Start(ctx context.Context) {
	log.Info().
		Str("project_id", o.projectID).
		Dur("interval", o.interval).
		Int("collectors", len(o.collectors)).
		Msg("Starting collector orchestrator")

	o.collect(ctx)

	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Stopping collector orchestrator")
			o.closeAll()
			return
		case <-ticker.C:
			o.collect(ctx)
		}
	}
}

func (o *Orchestrator) collect(ctx context.Context) {
	log.Info().Msg("Starting collection cycle")
	start := time.Now()

	metrics.ResetAll()

	for _, c := range o.collectors {
		o.runCollector(ctx, c)
	}

	log.Info().Dur("duration", time.Since(start)).Msg("Collection cycle completed")
}

func (o *Orchestrator) runCollector(ctx context.Context, c Collector) {
	cctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	start := time.Now()
	err := c.Collect(cctx)
	metrics.CollectionDuration.WithLabelValues(c.Name()).Observe(time.Since(start).Seconds())

	if err != nil {
		log.Error().Err(err).Str("collector", c.Name()).Msg("Collection failed")
		metrics.CollectionUp.WithLabelValues(o.projectID, c.Name()).Set(0)
		metrics.CollectionErrors.WithLabelValues(o.projectID, c.Name()).Inc()
		return
	}
	metrics.CollectionUp.WithLabelValues(o.projectID, c.Name()).Set(1)
}

func (o *Orchestrator) closeAll() {
	for _, c := range o.closers {
		if err := c(); err != nil {
			log.Warn().Err(err).Msg("Error closing GCP client")
		}
	}
}
