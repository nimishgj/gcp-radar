// Package metrics defines the Prometheus gauges and meta-metrics that
// gcp-radar exposes at /metrics. All resource/cost series are gauges so the
// observability backend can aggregate them with max()/lastNotNull per bucket
// (a gauge re-scraped within a billing period must not be summed over time).
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// --- Resource inventory (Cloud Asset Inventory) ---

	// ResourceCount is the number of resources of a given CAI asset type in a
	// project, split by location. One series per (asset_type, location).
	ResourceCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gcp_resource_count",
			Help: "Number of GCP resources by Cloud Asset Inventory asset type and location",
		},
		[]string{"project_id", "asset_type", "location"},
	)

	// ResourceTypesTotal is the number of distinct asset types seen in a project.
	ResourceTypesTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gcp_resource_types_total",
			Help: "Number of distinct Cloud Asset Inventory asset types present in the project",
		},
		[]string{"project_id"},
	)

	// --- Cost (BigQuery billing export) ---
	// period is the billing-month start as yyyymmdd (matches aws-radar CUR).

	CostTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gcp_cost_total_usd",
			Help: "Month-to-date total cost from the BigQuery billing export, in the billing currency (assumed USD)",
		},
		[]string{"project_id", "period"},
	)

	CostByService = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gcp_cost_by_service_usd",
			Help: "Month-to-date cost by GCP service from the BigQuery billing export",
		},
		[]string{"project_id", "service", "period"},
	)

	CostBySKU = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gcp_cost_by_sku_usd",
			Help: "Month-to-date cost by service/SKU from the BigQuery billing export (top N by cost)",
		},
		[]string{"project_id", "service", "sku", "period"},
	)

	CostLastProcessed = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gcp_cost_last_processed_timestamp",
			Help: "Unix timestamp of the last successful billing-export query",
		},
		[]string{"project_id"},
	)

	// --- Collection meta-metrics ---

	CollectionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gcp_radar_collection_duration_seconds",
			Help:    "Duration of a collector run in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"collector"},
	)

	CollectionUp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gcp_radar_collection_up",
			Help: "1 if the last run of a collector succeeded, 0 otherwise",
		},
		[]string{"project_id", "collector"},
	)

	CollectionErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gcp_radar_collection_errors_total",
			Help: "Total number of collector run errors",
		},
		[]string{"project_id", "collector"},
	)
)

// ResetAll clears the resource/cost gauges before each collection cycle so
// that resources (or cost series) that disappear are not reported with a
// stale value. Meta-metrics are intentionally not reset.
func ResetAll() {
	ResourceCount.Reset()
	ResourceTypesTotal.Reset()
	CostTotal.Reset()
	CostByService.Reset()
	CostBySKU.Reset()
}
