# GCP Auth Setup Guide

The bare minimum to let GCP Radar read a project. Unlike AWS (one big
read-only policy across ~90 APIs), GCP Radar needs **one service account** with
**two or three predefined roles** — because Cloud Asset Inventory already has a
single "read the inventory" role and BigQuery a single "read data" role.

GCP Radar only ever makes **read-only** calls.

## What it needs

| Feature | API to enable | Role on the service account |
| --- | --- | --- |
| Resource counts (always) | `cloudasset.googleapis.com` | `roles/cloudasset.viewer` |
| Cost (only if `billing.enabled`) | `bigquery.googleapis.com` | `roles/bigquery.dataViewer` **and** `roles/bigquery.jobUser` |

> The BigQuery roles are granted on the project that **hosts the billing export
> dataset** (`billing.billing_project_id`), which is usually the same project.

## Setup (gcloud)

```bash
PROJECT_ID=my-project
SA=gcp-radar

# 1. Enable the APIs
gcloud services enable cloudasset.googleapis.com bigquery.googleapis.com \
  --project "$PROJECT_ID"

# 2. Create the service account
gcloud iam service-accounts create "$SA" \
  --project "$PROJECT_ID" \
  --display-name "GCP Radar monitoring agent"

SA_EMAIL="${SA}@${PROJECT_ID}.iam.gserviceaccount.com"

# 3. Grant resource-count access (always)
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member "serviceAccount:${SA_EMAIL}" \
  --role roles/cloudasset.viewer

# 4. Grant cost access (only if you enable the billing collector)
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member "serviceAccount:${SA_EMAIL}" \
  --role roles/bigquery.dataViewer
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member "serviceAccount:${SA_EMAIL}" \
  --role roles/bigquery.jobUser
```

That is the entire permission setup. Now pick **one** way for GCP Radar to
authenticate as this service account.

## Auth option A — Workload Identity (recommended, keyless, for GKE)

No key files to store or rotate. Bind the Kubernetes ServiceAccount that runs
the pod to the GCP service account:

```bash
K8S_NAMESPACE=observability
K8S_SA=gcp-radar

# Let the k8s SA impersonate the GCP SA
gcloud iam service-accounts add-iam-policy-binding "$SA_EMAIL" \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:${PROJECT_ID}.svc.id.goog[${K8S_NAMESPACE}/${K8S_SA}]"

# Annotate the k8s SA so GKE maps it to the GCP SA
kubectl annotate serviceaccount "$K8S_SA" -n "$K8S_NAMESPACE" \
  iam.gke.io/gcp-service-account="$SA_EMAIL"
```

GCP Radar uses Application Default Credentials, so it picks this up
automatically — no config changes needed. (The Helm chart sets this annotation
for you.)

## Auth option B — Service-account key (for local use or non-GKE)

```bash
gcloud iam service-accounts keys create gcp-radar-key.json \
  --iam-account "$SA_EMAIL"

export GOOGLE_APPLICATION_CREDENTIALS="$PWD/gcp-radar-key.json"
./bin/gcp-radar
```

In Kubernetes, store the key in a Secret (e.g. via External Secrets) and mount
it, pointing `GOOGLE_APPLICATION_CREDENTIALS` at the mount path.

> **Keep keys out of version control.** The repo's `.gitignore` ignores `*.json`
> for this reason. Prefer option A wherever possible.

## For local development (no service account)

You can just use your own credentials:

```bash
gcloud auth application-default login
```

GCP Radar then runs against whatever project you set in `config.yaml`, using
your user's permissions.

## Billing export prerequisite

The cost collector reads a **BigQuery billing export** — the GCP analog of an
AWS CUR. It must be configured once, in the Cloud Billing console, before the
`billing` collector returns data:
<https://cloud.google.com/billing/docs/how-to/export-data-bigquery>

Then set `billing.dataset` / `billing.table` in `config.yaml` to that export
table (e.g. `gcp_billing_export_v1_0123AB_4567CD_89EF01`).

## Verify

```bash
# Resource counts (needs roles/cloudasset.viewer)
gcloud asset search-all-resources --scope="projects/${PROJECT_ID}" --limit 1

# Billing export readable (needs the BigQuery roles)
bq query --use_legacy_sql=false \
  "SELECT COUNT(*) FROM \`${PROJECT_ID}.<dataset>.<table>\` LIMIT 1"
```

If both succeed as the service account, GCP Radar has everything it needs.
