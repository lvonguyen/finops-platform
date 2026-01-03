// Package providers implements cloud-specific cost data retrieval.
package providers

import (
	"context"
	"time"

	"github.com/lvonguyen/finops-platform/internal/normalizer"
)

// GCPConfig holds GCP Cloud Billing configuration
type GCPConfig struct {
	ProjectID      string
	BillingAccount string
	Dataset        string
}

// GCPBilling retrieves cost data from GCP Cloud Billing via BigQuery export
type GCPBilling struct {
	config GCPConfig
}

// NewGCPBilling creates a new GCP Billing client
func NewGCPBilling(ctx context.Context, cfg GCPConfig) (*GCPBilling, error) {
	return &GCPBilling{
		config: cfg,
	}, nil
}

// Name returns the provider name
func (g *GCPBilling) Name() string {
	return "gcp-billing"
}

// Cloud returns the cloud provider
func (g *GCPBilling) Cloud() string {
	return "gcp"
}

// GetCosts retrieves cost data for the specified date range
func (g *GCPBilling) GetCosts(ctx context.Context, start, end time.Time) ([]normalizer.CostRecord, error) {
	// In production: Use BigQuery SDK to query billing export dataset
	// cloud.google.com/go/bigquery
	return []normalizer.CostRecord{}, nil
}

