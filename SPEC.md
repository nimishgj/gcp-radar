# GCP Radar — Design

_2026-10-13_

## Goal

A GCP analog of [aws-radar](https://github.com/nimishgj/aws-radar): a lightweight
Go agent that counts GCP resources and tracks month-to-date cost, exposes
Prometheus metrics, ships as cross-platform binaries + a multi-arch image via
GoReleaser, and deploys via Helm into any observability stack.

## Key decisions

| Decision | Choice | Why |
| --- | --- | --- |
| Resource enumeration | **Cloud Asset Inventory** (one `SearchAllResources` call) | GCP maintains an inventory; avoids aws-radar's 87 per-service collectors. Free. |
| Cost | **BigQuery billing export** | GCP analog of AWS CUR. Negligible query cost at this scale. |
| Scan scope | **Per-project** (one process per project) | Mirrors aws-radar's instance model and the Helm per-instance chart. |
| Auth | **ADC**; chart supports Workload Identity (default) or mounted SA key | Keyless on GKE; no code branches. |

## Architecture

```
cmd/gcp-radar/main.go      load config, start orchestrator + HTTP server, handle signals
internal/config            viper-based config (single project per process)
internal/collector
  collector.go             Orchestrator: builds enabled collectors, runs them each interval
  asset.go                 Cloud Asset Inventory -> gcp_resource_count
  billing.go               BigQuery billing export -> gcp_cost_* (MTD)
internal/metrics           Prometheus gauge + meta-metric definitions
internal/server            /metrics, /health, /
```

Collectors depend on **narrow interfaces** (`assetSearcher`, `billingQuerier`)
so the aggregation logic is unit-tested with fakes; the real implementations
wrap the GCP client libraries and are the only code that needs credentials.

## Data flow

1. Orchestrator ticks every `collection.interval` (default 5m).
2. `metrics.ResetAll()` clears resource/cost gauges (so disappeared resources
   drop out).
3. Each enabled collector runs with a per-run timeout; success/failure recorded
   in `gcp_radar_collection_up` / `_errors_total`.
4. Prometheus (via the OTel Collector) scrapes `/metrics` into the backend.

### Asset collector

`SearchAllResources(scope="projects/<id>", assetTypes=include)` → count by
`(asset_type, location)`; apply `exclude_types` client-side; publish
`gcp_resource_count` + `gcp_resource_types_total`.

### Billing collector

Query the export table for MTD cost grouped by `service`/`sku`
(`usage_start_time >= DATE_TRUNC(CURRENT_DATE(), MONTH)`), then publish total,
per-service, and top-N per-SKU gauges. `period` = month start `yyyymmdd`.

## Metrics

See the table in [README.md](README.md#metrics). All resource/cost series are
gauges; aggregate with `max()`/`lastNotNull` per bucket.

## Packaging & deployment

- `make ci` (fmt-check, mod-check, lint, build, test) is the single source of
  truth, runnable locally and in CI.
- GoReleaser: binaries + archives + checksums on the GitHub Release; multi-arch
  image to `ghcr.io/nimishgj/gcp-radar`. Release gated on CI success.
- Helm chart (`helm-charts/charts/gcp-radar`, separate repo) renders one
  Deployment/Service/ConfigMap per project; auth via Workload Identity (default)
  or a mounted SA-key Secret. _(Follow-up, not in this repo.)_

## GCP prerequisites (per project, operator-provisioned)

Enable `cloudasset.googleapis.com` + `bigquery.googleapis.com`; grant the SA
`roles/cloudasset.viewer`, `roles/bigquery.dataViewer`, `roles/bigquery.jobUser`;
configure a billing export to BigQuery; bind Workload Identity for keyless auth.

## Out of scope (v1)

- Multi-project / org-wide scanning (per-project only).
- The Helm chart and deployment wiring (tracked as a follow-up).
- Cost allocation by label/tag (can be added as a further metric later).
