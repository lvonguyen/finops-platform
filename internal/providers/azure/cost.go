// Package azure provides Azure Cost Management integration
package azure

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/costmanagement/armcostmanagement"

	"github.com/lvonguyen/finops-platform/internal/aggregator"
	"github.com/lvonguyen/finops-platform/internal/config"
)

// CostProvider implements aggregator.CostProvider for Azure
type CostProvider struct {
	client *armcostmanagement.QueryClient
	config config.AzureConfig
}

// NewCostProvider creates a new Azure cost provider
func NewCostProvider(ctx context.Context, cfg config.AzureConfig) (*CostProvider, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("Azure provider is disabled")
	}

	var cred *azidentity.DefaultAzureCredential
	var err error

	if cfg.UseMSI {
		cred, err = azidentity.NewDefaultAzureCredential(nil)
	} else {
		cred, err = azidentity.NewDefaultAzureCredential(&azidentity.DefaultAzureCredentialOptions{
			TenantID: cfg.TenantID,
		})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	client, err := armcostmanagement.NewQueryClient(cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cost management client: %w", err)
	}

	return &CostProvider{
		client: client,
		config: cfg,
	}, nil
}

// Name returns the provider name
func (p *CostProvider) Name() string {
	return "azure"
}

// GetCosts retrieves costs from Azure Cost Management
func (p *CostProvider) GetCosts(ctx context.Context, start, end time.Time) ([]aggregator.CostEntry, error) {
	entries := make([]aggregator.CostEntry, 0)

	granularity := armcostmanagement.GranularityType("Daily")
	if p.config.Granularity == "MONTHLY" {
		granularity = armcostmanagement.GranularityType("Monthly")
	}

	for _, subscriptionID := range p.config.SubscriptionIDs {
		scope := fmt.Sprintf("/subscriptions/%s", subscriptionID)

		// Build query
		query := armcostmanagement.QueryDefinition{
			Type:      toPtr(armcostmanagement.ExportTypeActualCost),
			Timeframe: toPtr(armcostmanagement.TimeframeTypeCustom),
			TimePeriod: &armcostmanagement.QueryTimePeriod{
				From: &start,
				To:   &end,
			},
			Dataset: &armcostmanagement.QueryDataset{
				Granularity: &granularity,
				Grouping: []*armcostmanagement.QueryGrouping{
					{
						Type: toPtr(armcostmanagement.QueryColumnTypeDimension),
						Name: toPtr("ServiceName"),
					},
					{
						Type: toPtr(armcostmanagement.QueryColumnTypeDimension),
						Name: toPtr("ResourceLocation"),
					},
				},
				Aggregation: map[string]*armcostmanagement.QueryAggregation{
					"totalCost": {
						Name:     toPtr("Cost"),
						Function: toPtr(armcostmanagement.FunctionTypeSum),
					},
				},
			},
		}

		result, err := p.client.Usage(ctx, scope, query, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to query costs for %s: %w", subscriptionID, err)
		}

		// Parse results
		if result.Properties != nil && result.Properties.Rows != nil {
			for _, row := range result.Properties.Rows {
				if len(row) < 4 {
					continue
				}

				// Row format: [cost, date, serviceName, region]
				cost, _ := row[0].(float64)
				dateStr, _ := row[1].(string)
				service, _ := row[2].(string)
				region, _ := row[3].(string)

				date, _ := time.Parse("20060102", dateStr)

				entries = append(entries, aggregator.CostEntry{
					Provider:  "azure",
					AccountID: subscriptionID,
					Service:   service,
					Region:    region,
					Date:      date,
					Cost:      cost,
					Currency:  "USD",
				})
			}
		}
	}

	return entries, nil
}

// GetBudgets retrieves budget status from Azure
func (p *CostProvider) GetBudgets(ctx context.Context) ([]aggregator.BudgetStatus, error) {
	return nil, nil
}

func toPtr[T any](v T) *T {
	return &v
}

