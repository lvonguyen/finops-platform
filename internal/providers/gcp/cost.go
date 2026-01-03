// Package gcp provides GCP Cloud Billing integration
package gcp

import (
	"context"
	"fmt"
	"time"

	billing "cloud.google.com/go/billing/budgets/apiv1"
	"cloud.google.com/go/billing/budgets/apiv1/budgetspb"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/lvonguyen/finops-platform/internal/aggregator"
	"github.com/lvonguyen/finops-platform/internal/config"
)

// CostProvider implements aggregator.CostProvider for GCP
type CostProvider struct {
	budgetClient *billing.BudgetClient
	config       config.GCPConfig
}

// NewCostProvider creates a new GCP cost provider
func NewCostProvider(ctx context.Context, cfg config.GCPConfig) (*CostProvider, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("GCP provider is disabled")
	}

	var opts []option.ClientOption

	// Use Workload Identity Federation if configured
	if cfg.WIFConfigPath != "" {
		opts = append(opts, option.WithCredentialsFile(cfg.WIFConfigPath))
	}

	budgetClient, err := billing.NewBudgetClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create budget client: %w", err)
	}

	return &CostProvider{
		budgetClient: budgetClient,
		config:       cfg,
	}, nil
}

// Name returns the provider name
func (p *CostProvider) Name() string {
	return "gcp"
}

// GetCosts retrieves costs from GCP
// Note: GCP doesn't have a direct cost API like AWS/Azure.
// Typically you export billing to BigQuery and query that.
// This implementation uses the Billing Budgets API for budget info.
func (p *CostProvider) GetCosts(ctx context.Context, start, end time.Time) ([]aggregator.CostEntry, error) {
	// GCP cost data is typically accessed via:
	// 1. BigQuery billing export (recommended)
	// 2. Cloud Billing API (limited)
	//
	// For a complete implementation, you would:
	// 1. Set up billing export to BigQuery
	// 2. Query BigQuery for cost data
	//
	// This is a stub that would be replaced with BigQuery queries

	entries := make([]aggregator.CostEntry, 0)

	// Example BigQuery query that would be used:
	// SELECT
	//   service.description as service,
	//   project.id as project_id,
	//   location.region as region,
	//   DATE(usage_start_time) as date,
	//   SUM(cost) as cost
	// FROM `project.dataset.gcp_billing_export_v1_*`
	// WHERE DATE(usage_start_time) BETWEEN @start AND @end
	// GROUP BY 1, 2, 3, 4

	return entries, nil
}

// GetBudgets retrieves budget status from GCP
func (p *CostProvider) GetBudgets(ctx context.Context) ([]aggregator.BudgetStatus, error) {
	statuses := make([]aggregator.BudgetStatus, 0)

	parent := fmt.Sprintf("billingAccounts/%s", p.config.BillingAccount)

	req := &budgetspb.ListBudgetsRequest{
		Parent: parent,
	}

	it := p.budgetClient.ListBudgets(ctx, req)
	for {
		budget, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list budgets: %w", err)
		}

		var limit float64
		if budget.Amount != nil && budget.Amount.GetSpecifiedAmount() != nil {
			limit = float64(budget.Amount.GetSpecifiedAmount().Units)
		}

		statuses = append(statuses, aggregator.BudgetStatus{
			BudgetName: budget.DisplayName,
			Provider:   "gcp",
			Limit:      limit,
		})
	}

	return statuses, nil
}

// Close closes the GCP clients
func (p *CostProvider) Close() error {
	return p.budgetClient.Close()
}

