package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// No config file present (empty working dir, no env override): defaults apply.
	t.Chdir(t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Collection.Interval != 300*time.Second {
		t.Errorf("Collection.Interval = %s, want 300s", cfg.Collection.Interval)
	}
	if !cfg.Assets.Enabled {
		t.Errorf("Assets.Enabled = false, want true (default)")
	}
	if cfg.Billing.Enabled {
		t.Errorf("Billing.Enabled = true, want false (default)")
	}
	if cfg.Billing.MaxSKUs != 100 {
		t.Errorf("Billing.MaxSKUs = %d, want 100", cfg.Billing.MaxSKUs)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
gcp:
  project_id: my-project
assets:
  enabled: true
  include_types:
    - compute.googleapis.com/Instance
billing:
  enabled: true
  dataset: billing
  table: gcp_billing_export_v1_ABC
  location: EU
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GCP_RADAR_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.GCP.ProjectID != "my-project" {
		t.Errorf("GCP.ProjectID = %q, want my-project", cfg.GCP.ProjectID)
	}
	if len(cfg.Assets.IncludeTypes) != 1 || cfg.Assets.IncludeTypes[0] != "compute.googleapis.com/Instance" {
		t.Errorf("Assets.IncludeTypes = %v, want [compute.googleapis.com/Instance]", cfg.Assets.IncludeTypes)
	}
	if !cfg.Billing.Enabled || cfg.Billing.Table != "gcp_billing_export_v1_ABC" {
		t.Errorf("Billing = %+v, want enabled with the configured table", cfg.Billing)
	}
	if cfg.Billing.Location != "EU" {
		t.Errorf("Billing.Location = %q, want EU", cfg.Billing.Location)
	}
	// BillingProjectID defaults to GCP.ProjectID when unset.
	if cfg.Billing.BillingProjectID != "my-project" {
		t.Errorf("Billing.BillingProjectID = %q, want my-project (defaulted)", cfg.Billing.BillingProjectID)
	}
}
