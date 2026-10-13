package collector

import (
	"context"

	asset "cloud.google.com/go/asset/apiv1"
	"cloud.google.com/go/asset/apiv1/assetpb"
	appconfig "github.com/nimishgj/gcp-radar/internal/config"
	"github.com/nimishgj/gcp-radar/internal/metrics"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/iterator"
)

// ResourceResult is one resource returned by Cloud Asset Inventory, reduced to
// the fields gcp-radar counts on.
type ResourceResult struct {
	AssetType string
	Location  string
}

// assetSearcher enumerates resources in a scope. The real implementation wraps
// the Cloud Asset Inventory SearchAllResources iterator; tests substitute a fake.
type assetSearcher interface {
	Search(ctx context.Context, scope string, assetTypes []string) ([]ResourceResult, error)
	Close() error
}

// AssetCollector counts resources via Cloud Asset Inventory and publishes them
// as gcp_resource_count{asset_type,location}. One CAI call replaces the
// per-service collectors aws-radar needs, since GCP maintains the inventory.
type AssetCollector struct {
	projectID string
	cfg       appconfig.AssetsConfig
	searcher  assetSearcher
}

func NewAssetCollector(projectID string, cfg appconfig.AssetsConfig, searcher assetSearcher) *AssetCollector {
	return &AssetCollector{projectID: projectID, cfg: cfg, searcher: searcher}
}

func (c *AssetCollector) Name() string { return "assets" }

func (c *AssetCollector) Collect(ctx context.Context) error {
	scope := "projects/" + c.projectID

	results, err := c.searcher.Search(ctx, scope, c.cfg.IncludeTypes)
	if err != nil {
		return err
	}

	exclude := make(map[string]struct{}, len(c.cfg.ExcludeTypes))
	for _, t := range c.cfg.ExcludeTypes {
		exclude[t] = struct{}{}
	}

	type key struct{ assetType, location string }
	counts := make(map[key]float64)
	types := make(map[string]struct{})

	for _, r := range results {
		if _, skip := exclude[r.AssetType]; skip {
			continue
		}
		loc := r.Location
		if loc == "" {
			loc = "global"
		}
		counts[key{r.AssetType, loc}]++
		types[r.AssetType] = struct{}{}
	}

	for k, n := range counts {
		metrics.ResourceCount.WithLabelValues(c.projectID, k.assetType, k.location).Set(n)
	}
	metrics.ResourceTypesTotal.WithLabelValues(c.projectID).Set(float64(len(types)))

	log.Debug().
		Str("project_id", c.projectID).
		Int("resources", len(results)).
		Int("asset_types", len(types)).
		Msg("Cloud Asset Inventory collection completed")

	return nil
}

// realAssetSearcher wraps the Cloud Asset Inventory client.
type realAssetSearcher struct {
	client *asset.Client
}

// newAssetSearcher builds a Cloud Asset Inventory searcher using Application
// Default Credentials (Workload Identity on GKE, or a mounted SA key).
func newAssetSearcher(ctx context.Context) (assetSearcher, error) {
	client, err := asset.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &realAssetSearcher{client: client}, nil
}

func (s *realAssetSearcher) Search(ctx context.Context, scope string, assetTypes []string) ([]ResourceResult, error) {
	req := &assetpb.SearchAllResourcesRequest{
		Scope:      scope,
		AssetTypes: assetTypes,
		PageSize:   500,
	}

	var results []ResourceResult
	it := s.client.SearchAllResources(ctx, req)
	for {
		r, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		results = append(results, ResourceResult{AssetType: r.GetAssetType(), Location: r.GetLocation()})
	}
	return results, nil
}

func (s *realAssetSearcher) Close() error { return s.client.Close() }
