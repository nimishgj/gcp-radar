package collector

import (
	"context"
	"errors"
	"testing"

	"github.com/nimishgj/gcp-radar/internal/config"
	"github.com/nimishgj/gcp-radar/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type fakeAssetSearcher struct {
	results       []ResourceResult
	err           error
	gotScope      string
	gotAssetTypes []string
	searchCalled  bool
}

func (f *fakeAssetSearcher) Search(_ context.Context, scope string, assetTypes []string) ([]ResourceResult, error) {
	f.searchCalled = true
	f.gotScope = scope
	f.gotAssetTypes = assetTypes
	return f.results, f.err
}

func (f *fakeAssetSearcher) Close() error { return nil }

func TestAssetCollector_CountsByTypeAndLocation(t *testing.T) {
	metrics.ResetAll()

	fake := &fakeAssetSearcher{results: []ResourceResult{
		{AssetType: "compute.googleapis.com/Instance", Location: "us-central1"},
		{AssetType: "compute.googleapis.com/Instance", Location: "us-central1"},
		{AssetType: "compute.googleapis.com/Instance", Location: "europe-west1"},
		{AssetType: "storage.googleapis.com/Bucket", Location: ""}, // -> "global"
	}}

	c := NewAssetCollector("proj-1", config.AssetsConfig{}, fake)
	if err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if got := testutil.ToFloat64(metrics.ResourceCount.WithLabelValues("proj-1", "compute.googleapis.com/Instance", "us-central1")); got != 2 {
		t.Errorf("us-central1 instances = %v, want 2", got)
	}
	if got := testutil.ToFloat64(metrics.ResourceCount.WithLabelValues("proj-1", "compute.googleapis.com/Instance", "europe-west1")); got != 1 {
		t.Errorf("europe-west1 instances = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.ResourceCount.WithLabelValues("proj-1", "storage.googleapis.com/Bucket", "global")); got != 1 {
		t.Errorf("global buckets = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.ResourceTypesTotal.WithLabelValues("proj-1")); got != 2 {
		t.Errorf("distinct types = %v, want 2", got)
	}
	if fake.gotScope != "projects/proj-1" {
		t.Errorf("scope = %q, want projects/proj-1", fake.gotScope)
	}
}

func TestAssetCollector_ExcludeTypes(t *testing.T) {
	metrics.ResetAll()

	fake := &fakeAssetSearcher{results: []ResourceResult{
		{AssetType: "compute.googleapis.com/Instance", Location: "us-central1"},
		{AssetType: "logging.googleapis.com/LogBucket", Location: "global"},
	}}

	c := NewAssetCollector("proj-1", config.AssetsConfig{
		ExcludeTypes: []string{"logging.googleapis.com/LogBucket"},
	}, fake)
	if err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if got := testutil.ToFloat64(metrics.ResourceTypesTotal.WithLabelValues("proj-1")); got != 1 {
		t.Errorf("distinct types after exclude = %v, want 1", got)
	}
}

func TestAssetCollector_IncludeTypesPassedThrough(t *testing.T) {
	metrics.ResetAll()
	fake := &fakeAssetSearcher{}
	c := NewAssetCollector("proj-1", config.AssetsConfig{
		IncludeTypes: []string{"compute.googleapis.com/Instance"},
	}, fake)
	if err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(fake.gotAssetTypes) != 1 || fake.gotAssetTypes[0] != "compute.googleapis.com/Instance" {
		t.Errorf("assetTypes passed to searcher = %v, want the include list", fake.gotAssetTypes)
	}
}

func TestAssetCollector_SearchError(t *testing.T) {
	metrics.ResetAll()
	fake := &fakeAssetSearcher{err: errors.New("permission denied")}
	c := NewAssetCollector("proj-1", config.AssetsConfig{}, fake)
	if err := c.Collect(context.Background()); err == nil {
		t.Fatal("Collect() error = nil, want error")
	}
}
