# GCP Radar

[![CI](https://github.com/nimishgj/gcp-radar/actions/workflows/ci.yaml/badge.svg)](https://github.com/nimishgj/gcp-radar/actions/workflows/ci.yaml)
[![Release](https://github.com/nimishgj/gcp-radar/actions/workflows/release.yaml/badge.svg)](https://github.com/nimishgj/gcp-radar/actions/workflows/release.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nimishgj/gcp-radar)](https://goreportcard.com/report/github.com/nimishgj/gcp-radar)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

A lightweight GCP resource- and cost-monitoring agent. It counts the resources
in a project with **Cloud Asset Inventory** and reads month-to-date spend from
the **BigQuery billing export**, exposing both as Prometheus metrics for
scraping into any observability backend.

GCP Radar is the GCP counterpart to [aws-radar](https://github.com/nimishgj/aws-radar).
Where AWS needs one collector per service (it has no "list everything" API),
GCP maintains an inventory for you — so GCP Radar counts *every* resource type
through a single Cloud Asset Inventory call.

## What it collects

| Area | Source | Cost to run |
| --- | --- | --- |
| Resource counts | Cloud Asset Inventory (`SearchAllResources`) | Free |
| Spend (MTD) | BigQuery billing export | Negligible (within the BigQuery free tier at this scale) |

### Metrics

| Metric | Labels | Description |
| --- | --- | --- |
| `gcp_resource_count` | `project_id, asset_type, location` | Resources per CAI asset type and location |
| `gcp_resource_types_total` | `project_id` | Distinct asset types present |
| `gcp_cost_total_usd` | `project_id, period` | Month-to-date total cost |
| `gcp_cost_by_service_usd` | `project_id, service, period` | MTD cost by service |
| `gcp_cost_by_sku_usd` | `project_id, service, sku, period` | MTD cost by SKU (top N) |
| `gcp_cost_last_processed_timestamp` | `project_id` | Unix ts of the last billing query |
| `gcp_radar_collection_up` | `project_id, collector` | 1 if the last run succeeded |
| `gcp_radar_collection_errors_total` | `project_id, collector` | Collector error count |
| `gcp_radar_collection_duration_seconds` | `collector` | Collector run duration |

> `period` is the billing-month start as `yyyymmdd`. Cost series are **gauges**;
> aggregate them with `max()` / `lastNotNull` per bucket (never `sum()` over
> time, which double-counts re-scrapes).

## Configuration

GCP Radar reads `config.yaml` from the working directory or `/app`, overridable
via `GCP_RADAR_CONFIG`. One process monitors one project.

```yaml
gcp:
  project_id: my-project
assets:
  enabled: true
billing:
  enabled: true
  dataset: billing
  table: gcp_billing_export_v1_0123AB_4567CD_89EF01
  location: US
```

See [`config.yaml`](config.yaml) for all options.

### Authentication

GCP Radar uses **Application Default Credentials**, so the same binary works:

- **keyless** on GKE via Workload Identity (recommended — nothing to rotate), or
- with a mounted **service-account key** at `GOOGLE_APPLICATION_CREDENTIALS`.

See [`docs/gcp-auth-setup.md`](docs/gcp-auth-setup.md) for the bare-minimum
service-account + roles setup (copy-paste `gcloud` commands).

## GCP prerequisites (per monitored project)

1. Enable the APIs: `cloudasset.googleapis.com`, `bigquery.googleapis.com`.
2. Grant the service account:
   - `roles/cloudasset.viewer` (resource counts)
   - `roles/bigquery.dataViewer` + `roles/bigquery.jobUser` (cost)
3. Configure a [billing export to BigQuery](https://cloud.google.com/billing/docs/how-to/export-data-bigquery)
   (the GCP analog of an AWS CUR) if you enable the cost collector.
4. For keyless auth, bind the Kubernetes ServiceAccount to the GCP service
   account via Workload Identity.

## Running locally

```bash
make build
GCP_RADAR_CONFIG=./config.yaml ./bin/gcp-radar
# metrics on http://localhost:9090/metrics
```

With ADC configured (`gcloud auth application-default login`), the collectors
run against your current project.

## Development

```bash
make ci      # fmt-check, mod-check, lint, build, test — the single source of truth
make test    # tests only
```

## Releases

Tagging `v*` on `main` runs GoReleaser: cross-platform binaries + archives on
the GitHub Release, and a multi-arch image to `ghcr.io/nimishgj/gcp-radar`.
Releases are gated on CI passing for the tagged commit.

## License

[Apache 2.0](LICENSE)
