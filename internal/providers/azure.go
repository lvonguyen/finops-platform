// Package providers implements cloud-specific cost data retrieval.
package providers

import (
	"context"
	"time"

	"github.com/lvonguyen/finops-platform/internal/normalizer"
)

// AzureConfig holds Azure Cost Management configuration
type AzureConfig struct {
	SubscriptionID string
	TenantID       string
	Granularity    string
}

// AzureCostManagement retrieves cost data from Azure Cost Management
type AzureCostManagement struct {
	config AzureConfig
}

// NewAzureCostManagement creates a new Azure Cost Management client
func NewAzureCostManagement(ctx context.Context, cfg AzureConfig) (*AzureCostManagement, error) {
	return &AzureCostManagement{
		config: cfg,
	}, nil
}

// Name returns the provider name
func (a *AzureCostManagement) Name() string {
	return "azure-cost-management"
}

// Cloud returns the cloud provider
func (a *AzureCostManagement) Cloud() string {
	return "azure"
}

// GetCosts retrieves cost data for the specified date range
func (a *AzureCostManagement) GetCosts(ctx context.Context, start, end time.Time) ([]normalizer.CostRecord, error) {
	// In production: Use Azure Cost Management SDK
	// github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/costmanagement/armcostmanagement
	return []normalizer.CostRecord{}, nil
}

