package config

import (
	"os"
	"time"

	"github.com/spf13/viper"
)

// Config is the top-level configuration for a single gcp-radar process.
// Like aws-radar, one process monitors one project; the Helm chart renders
// one Deployment + ConfigMap per project ("instance").
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Collection CollectionConfig `mapstructure:"collection"`
	GCP        GCPConfig        `mapstructure:"gcp"`
	Assets     AssetsConfig     `mapstructure:"assets"`
	Billing    BillingConfig    `mapstructure:"billing"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

type ServerConfig struct {
	Port        int    `mapstructure:"port"`
	MetricsPath string `mapstructure:"metrics_path"`
	HealthPath  string `mapstructure:"health_path"`
}

type CollectionConfig struct {
	Interval time.Duration `mapstructure:"interval"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

// GCPConfig identifies the project to monitor. Credentials come from
// Application Default Credentials (Workload Identity on GKE, or a mounted
// service-account key via GOOGLE_APPLICATION_CREDENTIALS) — never from config.
type GCPConfig struct {
	ProjectID string `mapstructure:"project_id"`
}

// AssetsConfig controls the Cloud Asset Inventory resource-count collector.
// IncludeTypes/ExcludeTypes filter on the CAI asset type
// (e.g. "compute.googleapis.com/Instance"); empty IncludeTypes means all.
type AssetsConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	IncludeTypes []string `mapstructure:"include_types"`
	ExcludeTypes []string `mapstructure:"exclude_types"`
}

// BillingConfig controls the BigQuery billing-export cost collector.
// Dataset/Table point at the standard usage cost export table
// (gcp_billing_export_v1_<BILLING_ACCOUNT_ID>). BillingProjectID is the
// project that hosts the export dataset and runs the query jobs; it defaults
// to GCP.ProjectID when empty. MaxSKUs caps the per-SKU series (top N by cost).
type BillingConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	BillingProjectID string `mapstructure:"billing_project_id"`
	Dataset          string `mapstructure:"dataset"`
	Table            string `mapstructure:"table"`
	Location         string `mapstructure:"location"`
	MaxSKUs          int    `mapstructure:"max_skus"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load reads configuration from config.yaml (cwd or /app), overridable via the
// GCP_RADAR_CONFIG env var, and applies defaults for everything unset.
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/app")

	if configPath := os.Getenv("GCP_RADAR_CONFIG"); configPath != "" {
		v.SetConfigFile(configPath)
	}

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if cfg.Billing.BillingProjectID == "" {
		cfg.Billing.BillingProjectID = cfg.GCP.ProjectID
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 9090)
	v.SetDefault("server.metrics_path", "/metrics")
	v.SetDefault("server.health_path", "/health")
	v.SetDefault("collection.interval", "300s")
	v.SetDefault("collection.timeout", "120s")
	v.SetDefault("assets.enabled", true)
	v.SetDefault("billing.enabled", false)
	v.SetDefault("billing.location", "US")
	v.SetDefault("billing.max_skus", 100)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
}
