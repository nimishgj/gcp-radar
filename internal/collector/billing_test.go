package collector

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nimishgj/gcp-radar/internal/config"
	"github.com/nimishgj/gcp-radar/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type fakeBillingQuerier struct {
	rows   []BillingRow
	err    error
	gotSQL string
}

func (f *fakeBillingQuerier) Query(_ context.Context, sql string) ([]BillingRow, error) {
	f.gotSQL = sql
	return f.rows, f.err
}

func (f *fakeBillingQuerier) Close() error { return nil }

func fixedNow() time.Time { return time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC) }

func TestBillingCollector_Aggregates(t *testing.T) {
	metrics.ResetAll()

	fake := &fakeBillingQuerier{rows: []BillingRow{
		{Service: "Compute Engine", SKU: "N2 Instance Core", Cost: 100},
		{Service: "Compute Engine", SKU: "N2 Instance Ram", Cost: 50},
		{Service: "Cloud Storage", SKU: "Standard Storage", Cost: 25},
	}}

	c := NewBillingCollector("proj-1", config.BillingConfig{BillingProjectID: "proj-1", Dataset: "billing", Table: "t", MaxSKUs: 100}, fake)
	c.now = fixedNow

	if err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	const period = "20261001"

	if got := testutil.ToFloat64(metrics.CostTotal.WithLabelValues("proj-1", period)); got != 175 {
		t.Errorf("total = %v, want 175", got)
	}
	if got := testutil.ToFloat64(metrics.CostByService.WithLabelValues("proj-1", "Compute Engine", period)); got != 150 {
		t.Errorf("Compute Engine = %v, want 150", got)
	}
	if got := testutil.ToFloat64(metrics.CostByService.WithLabelValues("proj-1", "Cloud Storage", period)); got != 25 {
		t.Errorf("Cloud Storage = %v, want 25", got)
	}
	if got := testutil.ToFloat64(metrics.CostBySKU.WithLabelValues("proj-1", "Compute Engine", "N2 Instance Core", period)); got != 100 {
		t.Errorf("SKU N2 Instance Core = %v, want 100", got)
	}
	if testutil.ToFloat64(metrics.CostLastProcessed.WithLabelValues("proj-1")) != float64(fixedNow().Unix()) {
		t.Errorf("last processed timestamp not set to now")
	}
}

func TestBillingCollector_TopNSKUs(t *testing.T) {
	metrics.ResetAll()

	fake := &fakeBillingQuerier{rows: []BillingRow{
		{Service: "A", SKU: "big", Cost: 100},
		{Service: "A", SKU: "mid", Cost: 50},
		{Service: "A", SKU: "small", Cost: 1},
	}}

	c := NewBillingCollector("proj-1", config.BillingConfig{MaxSKUs: 2}, fake)
	c.now = fixedNow
	if err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	const period = "20261001"
	// The two biggest SKUs are published; the smallest is dropped.
	if got := testutil.ToFloat64(metrics.CostBySKU.WithLabelValues("proj-1", "A", "big", period)); got != 100 {
		t.Errorf("big SKU = %v, want 100", got)
	}
	if got := testutil.CollectAndCount(metrics.CostBySKU); got != 2 {
		t.Errorf("published SKU series = %d, want 2 (top-N cap)", got)
	}
	// Service total still reflects all rows, not just the top-N.
	if got := testutil.ToFloat64(metrics.CostByService.WithLabelValues("proj-1", "A", period)); got != 151 {
		t.Errorf("service total = %v, want 151", got)
	}
}

func TestBillingCollector_QueryShape(t *testing.T) {
	metrics.ResetAll()
	fake := &fakeBillingQuerier{}
	c := NewBillingCollector("proj-1", config.BillingConfig{BillingProjectID: "billing-proj", Dataset: "ds", Table: "tbl"}, fake)
	c.now = fixedNow
	if err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	for _, want := range []string{"`billing-proj.ds.tbl`", "project.id = 'proj-1'", "DATE_TRUNC(CURRENT_DATE(), MONTH)"} {
		if !strings.Contains(fake.gotSQL, want) {
			t.Errorf("query missing %q; got:\n%s", want, fake.gotSQL)
		}
	}
}

func TestBillingCollector_QueryError(t *testing.T) {
	metrics.ResetAll()
	fake := &fakeBillingQuerier{err: errors.New("bq boom")}
	c := NewBillingCollector("proj-1", config.BillingConfig{}, fake)
	c.now = fixedNow
	if err := c.Collect(context.Background()); err == nil {
		t.Fatal("Collect() error = nil, want error")
	}
}
