// Package aws provides AWS Cost Explorer integration
package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	internalConfig "github.com/lvonguyen/finops-platform/internal/config"
	"github.com/lvonguyen/finops-platform/internal/aggregator"
)

// CostProvider implements aggregator.CostProvider for AWS
type CostProvider struct {
	client *costexplorer.Client
	config internalConfig.AWSConfig
}

// NewCostProvider creates a new AWS cost provider
func NewCostProvider(ctx context.Context, cfg internalConfig.AWSConfig) (*CostProvider, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("AWS provider is disabled")
	}

	// Load AWS configuration
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// If role ARN specified, assume role
	if cfg.RoleARN != "" {
		stsClient := sts.NewFromConfig(awsCfg)
		creds := stscreds.NewAssumeRoleProvider(stsClient, cfg.RoleARN)
		awsCfg.Credentials = aws.NewCredentialsCache(creds)
	}

	client := costexplorer.NewFromConfig(awsCfg)

	return &CostProvider{
		client: client,
		config: cfg,
	}, nil
}

// Name returns the provider name
func (p *CostProvider) Name() string {
	return "aws"
}

// GetCosts retrieves costs from AWS Cost Explorer
func (p *CostProvider) GetCosts(ctx context.Context, start, end time.Time) ([]aggregator.CostEntry, error) {
	entries := make([]aggregator.CostEntry, 0)

	granularity := types.GranularityDaily
	if p.config.Granularity == "MONTHLY" {
		granularity = types.GranularityMonthly
	}

	// Build group by dimensions
	groupBy := make([]types.GroupDefinition, 0)
	for _, g := range p.config.GroupBy {
		groupBy = append(groupBy, types.GroupDefinition{
			Type: types.GroupDefinitionTypeDimension,
			Key:  aws.String(g),
		})
	}
	if len(groupBy) == 0 {
		groupBy = []types.GroupDefinition{
			{Type: types.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")},
			{Type: types.GroupDefinitionTypeDimension, Key: aws.String("LINKED_ACCOUNT")},
		}
	}

	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(start.Format("2006-01-02")),
			End:   aws.String(end.Format("2006-01-02")),
		},
		Granularity: granularity,
		Metrics:     []string{"UnblendedCost", "UsageQuantity"},
		GroupBy:     groupBy,
	}

	// Handle pagination manually
	for {
		output, err := p.client.GetCostAndUsage(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to get cost data: %w", err)
		}

		for _, result := range output.ResultsByTime {
			date, _ := time.Parse("2006-01-02", *result.TimePeriod.Start)

			for _, group := range result.Groups {
				cost := 0.0
				usage := 0.0

				if unblended, ok := group.Metrics["UnblendedCost"]; ok {
					if unblended.Amount != nil {
						fmt.Sscanf(*unblended.Amount, "%f", &cost)
					}
				}

				if usageQty, ok := group.Metrics["UsageQuantity"]; ok {
					if usageQty.Amount != nil {
						fmt.Sscanf(*usageQty.Amount, "%f", &usage)
					}
				}

				// Parse group keys
				var service, accountID string
				for i, key := range group.Keys {
					if i == 0 {
						service = key
					} else if i == 1 {
						accountID = key
					}
				}

				entries = append(entries, aggregator.CostEntry{
					Provider:    "aws",
					AccountID:   accountID,
					Service:     service,
					Date:        date,
					Cost:        cost,
					Currency:    "USD",
					UsageAmount: usage,
				})
			}
		}

		// Check for more pages
		if output.NextPageToken == nil {
			break
		}
		input.NextPageToken = output.NextPageToken
	}

	return entries, nil
}

// GetBudgets retrieves budget status from AWS
func (p *CostProvider) GetBudgets(ctx context.Context) ([]aggregator.BudgetStatus, error) {
	// AWS Budgets API would be used here
	// For now, return empty
	return nil, nil
}

