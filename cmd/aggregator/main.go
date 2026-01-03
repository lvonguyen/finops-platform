// Package main provides the FinOps cost aggregation CLI.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/lvonguyen/finops-platform/internal/anomaly"
	"github.com/lvonguyen/finops-platform/internal/chargeback"
	"github.com/lvonguyen/finops-platform/internal/normalizer"
	"github.com/lvonguyen/finops-platform/internal/providers"
)

// Config holds application configuration
type Config struct {
	Mode       string // aggregate, chargeback, anomaly, forecast, budget
	ConfigPath string
	Month      string // YYYY-MM for chargeback
	Days       int    // Days for anomaly detection
	OutputDir  string
	Verbose    bool
}

func main() {
	cfg := parseFlags()

	// Initialize logger
	var logger *zap.Logger
	var err error
	if cfg.Verbose {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting FinOps Cost Aggregator",
		zap.String("mode", cfg.Mode),
		zap.String("config", cfg.ConfigPath),
	)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("Received shutdown signal")
		cancel()
	}()

	// Execute based on mode
	var execErr error
	switch cfg.Mode {
	case "aggregate":
		execErr = runAggregate(ctx, cfg, logger)
	case "chargeback":
		execErr = runChargeback(ctx, cfg, logger)
	case "anomaly":
		execErr = runAnomaly(ctx, cfg, logger)
	case "forecast":
		execErr = runForecast(ctx, cfg, logger)
	case "budget":
		execErr = runBudgetCheck(ctx, cfg, logger)
	default:
		logger.Fatal("Unknown mode", zap.String("mode", cfg.Mode))
	}

	if execErr != nil {
		logger.Error("Execution failed", zap.Error(execErr))
		os.Exit(1)
	}

	logger.Info("FinOps Cost Aggregator complete")
}

func parseFlags() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.Mode, "mode", "aggregate", "Mode: aggregate, chargeback, anomaly, forecast, budget")
	flag.StringVar(&cfg.ConfigPath, "config", "configs/config.yaml", "Path to config file")
	flag.StringVar(&cfg.Month, "month", "", "Month for chargeback (YYYY-MM)")
	flag.IntVar(&cfg.Days, "days", 7, "Days for anomaly detection")
	flag.StringVar(&cfg.OutputDir, "output", "reports", "Output directory for reports")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Enable verbose logging")
	flag.Parse()

	return cfg
}

// runAggregate aggregates costs from all cloud providers
func runAggregate(ctx context.Context, cfg *Config, logger *zap.Logger) error {
	logger.Info("Running cost aggregation")

	// Initialize providers
	costProviders := []providers.CostProvider{}

	// AWS Cost Explorer
	if os.Getenv("AWS_REGION") != "" || os.Getenv("AWS_PROFILE") != "" {
		aws, err := providers.NewAWSCostExplorer(ctx, providers.AWSConfig{
			Region:      os.Getenv("AWS_REGION"),
			Granularity: "DAILY",
		})
		if err != nil {
			logger.Warn("Failed to initialize AWS provider", zap.Error(err))
		} else {
			costProviders = append(costProviders, aws)
		}
	}

	// Azure Cost Management
	if os.Getenv("AZURE_SUBSCRIPTION_ID") != "" {
		azure, err := providers.NewAzureCostManagement(ctx, providers.AzureConfig{
			SubscriptionID: os.Getenv("AZURE_SUBSCRIPTION_ID"),
		})
		if err != nil {
			logger.Warn("Failed to initialize Azure provider", zap.Error(err))
		} else {
			costProviders = append(costProviders, azure)
		}
	}

	// GCP Billing
	if os.Getenv("GCP_PROJECT_ID") != "" {
		gcp, err := providers.NewGCPBilling(ctx, providers.GCPConfig{
			ProjectID: os.Getenv("GCP_PROJECT_ID"),
			Dataset:   os.Getenv("GCP_BILLING_DATASET"),
		})
		if err != nil {
			logger.Warn("Failed to initialize GCP provider", zap.Error(err))
		} else {
			costProviders = append(costProviders, gcp)
		}
	}

	if len(costProviders) == 0 {
		return fmt.Errorf("no cost providers configured")
	}

	// Aggregate costs
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30) // Last 30 days

	var allCosts []normalizer.CostRecord
	for _, provider := range costProviders {
		costs, err := provider.GetCosts(ctx, startDate, endDate)
		if err != nil {
			logger.Error("Failed to get costs", zap.String("provider", provider.Name()), zap.Error(err))
			continue
		}
		allCosts = append(allCosts, costs...)
		logger.Info("Costs retrieved", zap.String("provider", provider.Name()), zap.Int("records", len(costs)))
	}

	// Normalize and summarize
	summary := normalizer.Summarize(allCosts)
	printSummary(summary)

	return nil
}

// runChargeback generates chargeback reports
func runChargeback(ctx context.Context, cfg *Config, logger *zap.Logger) error {
	logger.Info("Generating chargeback report", zap.String("month", cfg.Month))

	// Parse month
	month := cfg.Month
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	// Initialize allocator
	allocator := chargeback.NewAllocator(chargeback.AllocatorConfig{
		PrimaryTag:    "cost_center",
		FallbackTag:   "team",
		UntaggedPool:  "IT-SHARED",
	})

	// Get costs for month (stub)
	costs := []normalizer.CostRecord{} // Would fetch from providers

	// Allocate costs
	allocations := allocator.Allocate(costs)

	// Generate report
	report := chargeback.GenerateReport(allocations, month)
	
	// Save report
	outputPath := fmt.Sprintf("%s/chargeback-%s.csv", cfg.OutputDir, month)
	if err := report.SaveCSV(outputPath); err != nil {
		return fmt.Errorf("failed to save report: %w", err)
	}

	logger.Info("Chargeback report generated", zap.String("path", outputPath))
	return nil
}

// runAnomaly runs anomaly detection
func runAnomaly(ctx context.Context, cfg *Config, logger *zap.Logger) error {
	logger.Info("Running anomaly detection", zap.Int("days", cfg.Days))

	detector := anomaly.NewDetector(anomaly.DetectorConfig{
		Sensitivity:  anomaly.SensitivityMedium,
		BaselineDays: 30,
		MinSpend:     100.0,
	})

	// Get recent costs (stub)
	costs := []normalizer.CostRecord{}

	// Detect anomalies
	anomalies := detector.Detect(costs)

	if len(anomalies) == 0 {
		logger.Info("No anomalies detected")
		return nil
	}

	logger.Warn("Anomalies detected", zap.Int("count", len(anomalies)))
	for _, a := range anomalies {
		fmt.Printf("  - %s: %s (%.1f%% change)\n", a.Service, a.Reason, a.PercentChange)
	}

	return nil
}

// runForecast generates spend forecasts
func runForecast(ctx context.Context, cfg *Config, logger *zap.Logger) error {
	logger.Info("Generating spend forecast")
	// Implementation would use time-series forecasting
	fmt.Println("Forecast generation not yet implemented")
	return nil
}

// runBudgetCheck checks budget status
func runBudgetCheck(ctx context.Context, cfg *Config, logger *zap.Logger) error {
	logger.Info("Checking budget status")
	// Implementation would check against configured budgets
	fmt.Println("Budget check not yet implemented")
	return nil
}

// printSummary prints the cost summary
func printSummary(summary normalizer.CostSummary) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                Multi-Cloud Cost Summary                          ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Total Spend: $%.2f                                        \n", summary.TotalCost)
	fmt.Printf("║  Period: %s to %s                             \n", summary.StartDate.Format("2006-01-02"), summary.EndDate.Format("2006-01-02"))
	fmt.Println("║")
	fmt.Println("║  By Cloud:")
	for cloud, cost := range summary.ByCloud {
		pct := (cost / summary.TotalCost) * 100
		fmt.Printf("║    %-8s $%.2f (%.1f%%)\n", cloud+":", cost, pct)
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

