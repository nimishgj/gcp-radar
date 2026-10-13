package collector

import (
	"context"
	"fmt"
	"sort"
	"time"

	"cloud.google.com/go/bigquery"
	appconfig "github.com/nimishgj/gcp-radar/internal/config"
	"github.com/nimishgj/gcp-radar/internal/metrics"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/iterator"
)

// BillingRow is one aggregated row from the billing-export query:
// cost for a (service, sku) pair over the current billing month.
type BillingRow struct {
	Service string
	SKU     string
	Cost    float64
}

// billingQuerier runs a SQL query against the BigQuery billing export. The real
// implementation wraps a *bigquery.Client; tests substitute a fake.
type billingQuerier interface {
	Query(ctx context.Context, sql string) ([]BillingRow, error)
	Close() error
}

// BillingCollector reads month-to-date cost from the BigQuery billing export
// and publishes gcp_cost_* gauges. It mirrors aws-radar's CUR collector.
type BillingCollector struct {
	projectID string
	cfg       appconfig.BillingConfig
	querier   billingQuerier
	now       func() time.Time
}

func NewBillingCollector(projectID string, cfg appconfig.BillingConfig, querier billingQuerier) *BillingCollector {
	return &BillingCollector{projectID: projectID, cfg: cfg, querier: querier, now: time.Now}
}

func (c *BillingCollector) Name() string { return "billing" }

func (c *BillingCollector) Collect(ctx context.Context) error {
	period := billingPeriod(c.now())

	rows, err := c.querier.Query(ctx, c.buildQuery())
	if err != nil {
		return err
	}

	var total float64
	byService := make(map[string]float64)
	for _, r := range rows {
		total += r.Cost
		byService[r.Service] += r.Cost
	}

	metrics.CostTotal.WithLabelValues(c.projectID, period).Set(total)
	for svc, cost := range byService {
		metrics.CostByService.WithLabelValues(c.projectID, svc, period).Set(cost)
	}

	for _, r := range topNRows(rows, c.cfg.MaxSKUs) {
		metrics.CostBySKU.WithLabelValues(c.projectID, r.Service, r.SKU, period).Set(r.Cost)
	}

	metrics.CostLastProcessed.WithLabelValues(c.projectID).Set(float64(c.now().Unix()))

	log.Debug().
		Str("project_id", c.projectID).
		Float64("total", total).
		Int("services", len(byService)).
		Int("rows", len(rows)).
		Msg("Billing-export collection completed")

	return nil
}

// buildQuery aggregates MTD cost by service/SKU for the monitored project.
// Dataset/Table/ProjectID come from operator-controlled config.
func (c *BillingCollector) buildQuery() string {
	table := fmt.Sprintf("`%s.%s.%s`", c.cfg.BillingProjectID, c.cfg.Dataset, c.cfg.Table)
	return fmt.Sprintf(`SELECT
  service.description AS service,
  sku.description AS sku,
  SUM(cost) AS cost
FROM %s
WHERE project.id = '%s'
  AND usage_start_time >= TIMESTAMP(DATE_TRUNC(CURRENT_DATE(), MONTH))
GROUP BY service, sku
HAVING cost != 0
ORDER BY cost DESC`, table, c.projectID)
}

// topNRows returns the n most expensive rows (n <= 0 means all).
func topNRows(rows []BillingRow, n int) []BillingRow {
	sorted := make([]BillingRow, len(rows))
	copy(sorted, rows)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Cost > sorted[j].Cost })
	if n > 0 && len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

// billingPeriod returns the current billing-month start as yyyymmdd (UTC),
// matching aws-radar's CUR period label.
func billingPeriod(now time.Time) string {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start.Format("20060102")
}

// realBillingQuerier wraps a BigQuery client pinned to the export's location.
type realBillingQuerier struct {
	client   *bigquery.Client
	location string
}

func newBillingQuerier(ctx context.Context, billingProjectID, location string) (billingQuerier, error) {
	client, err := bigquery.NewClient(ctx, billingProjectID)
	if err != nil {
		return nil, err
	}
	return &realBillingQuerier{client: client, location: location}, nil
}

func (q *realBillingQuerier) Query(ctx context.Context, sql string) ([]BillingRow, error) {
	query := q.client.Query(sql)
	if q.location != "" {
		query.Location = q.location
	}

	it, err := query.Read(ctx)
	if err != nil {
		return nil, err
	}

	var rows []BillingRow
	for {
		var r struct {
			Service string  `bigquery:"service"`
			SKU     string  `bigquery:"sku"`
			Cost    float64 `bigquery:"cost"`
		}
		err := it.Next(&r)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, BillingRow{Service: r.Service, SKU: r.SKU, Cost: r.Cost})
	}
	return rows, nil
}

func (q *realBillingQuerier) Close() error { return q.client.Close() }
