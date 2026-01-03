// Package providers implements cloud-specific cost data retrieval.
package providers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"

	"github.com/lvonguyen/finops-platform/internal/normalizer"
)

// CostProvider defines the interface for cloud cost providers
type CostProvider interface {
	Name() string
	Cloud() string
	GetCosts(ctx context.Context, start, end time.Time) ([]normalizer.CostRecord, error)
}

// AWSConfig holds AWS Cost Explorer configuration
type AWSConfig struct {
	Region      string
	Granularity string // DAILY or HOURLY
	GroupBy     []string
}

// AWSCostExplorer retrieves cost data from AWS Cost Explorer
type AWSCostExplorer struct {
	client      *costexplorer.Client
	granularity types.Granularity
	groupBy     []string
}

// NewAWSCostExplorer creates a new AWS Cost Explorer client
func NewAWSCostExplorer(ctx context.Context, cfg AWSConfig) (*AWSCostExplorer, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := costexplorer.NewFromConfig(awsCfg)

	granularity := types.GranularityDaily
	if cfg.Granularity == "HOURLY" {
		granularity = types.GranularityHourly
	}

	groupBy := cfg.GroupBy
	if len(groupBy) == 0 {
		groupBy = []string{"SERVICE", "LINKED_ACCOUNT"}
	}

	return &AWSCostExplorer{
		client:      client,
		granularity: granularity,
		groupBy:     groupBy,
	}, nil
}

// Name returns the provider name
func (a *AWSCostExplorer) Name() string {
	return "aws-cost-explorer"
}

// Cloud returns the cloud provider
func (a *AWSCostExplorer) Cloud() string {
	return "aws"
}

// GetCosts retrieves cost data for the specified date range
func (a *AWSCostExplorer) GetCosts(ctx context.Context, start, end time.Time) ([]normalizer.CostRecord, error) {
	// Build group by definitions
	var groupByDefs []types.GroupDefinition
	for _, g := range a.groupBy {
		groupByDefs = append(groupByDefs, types.GroupDefinition{
			Type: types.GroupDefinitionTypeDimension,
			Key:  aws.String(g),
		})
	}

	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(start.Format("2006-01-02")),
			End:   aws.String(end.Format("2006-01-02")),
		},
		Granularity: a.granularity,
		Metrics:     []string{"UnblendedCost", "UsageQuantity"},
		GroupBy:     groupByDefs,
	}

	var allRecords []normalizer.CostRecord

	// Handle pagination manually
	for {
		result, err := a.client.GetCostAndUsage(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to get cost data: %w", err)
		}

		records := a.parseResults(result.ResultsByTime)
		allRecords = append(allRecords, records...)

		// Check for more pages
		if result.NextPageToken == nil {
			break
		}
		input.NextPageToken = result.NextPageToken
	}

	return allRecords, nil
}

// parseResults converts AWS response to normalized records
func (a *AWSCostExplorer) parseResults(results []types.ResultByTime) []normalizer.CostRecord {
	var records []normalizer.CostRecord

	for _, result := range results {
		startDate, _ := time.Parse("2006-01-02", *result.TimePeriod.Start)

		for _, group := range result.Groups {
			record := normalizer.CostRecord{
				Cloud:    "aws",
				Date:     startDate,
				Currency: "USD",
			}

			// Parse group keys
			for i, key := range group.Keys {
				if i < len(a.groupBy) {
					switch a.groupBy[i] {
					case "SERVICE":
						record.CloudService = key
						record.Service = normalizer.NormalizeService("aws", key)
					case "LINKED_ACCOUNT":
						record.Account = key
					case "REGION":
						record.Region = key
					}
				}
			}

			// Parse metrics
			if cost, ok := group.Metrics["UnblendedCost"]; ok && cost.Amount != nil {
				record.Cost = parseFloat(*cost.Amount)
			}
			if usage, ok := group.Metrics["UsageQuantity"]; ok && usage.Amount != nil {
				record.UsageQuantity = parseFloat(*usage.Amount)
				if usage.Unit != nil {
					record.UsageUnit = *usage.Unit
				}
			}

			records = append(records, record)
		}
	}

	return records
}

// GetForecast retrieves cost forecast
func (a *AWSCostExplorer) GetForecast(ctx context.Context, start, end time.Time) (float64, error) {
	input := &costexplorer.GetCostForecastInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(start.Format("2006-01-02")),
			End:   aws.String(end.Format("2006-01-02")),
		},
		Metric:      types.MetricUnblendedCost,
		Granularity: types.GranularityMonthly,
	}

	result, err := a.client.GetCostForecast(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("failed to get forecast: %w", err)
	}

	if result.Total != nil && result.Total.Amount != nil {
		return parseFloat(*result.Total.Amount), nil
	}

	return 0, nil
}

// parseFloat converts string to float64
func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

