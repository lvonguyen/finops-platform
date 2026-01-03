// Package main provides the entrypoint for the FinOps Cost Aggregator
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lvonguyen/finops-platform/internal/aggregator"
	"github.com/lvonguyen/finops-platform/internal/config"
	"github.com/lvonguyen/finops-platform/internal/providers/aws"
	"github.com/lvonguyen/finops-platform/internal/providers/azure"
	"github.com/lvonguyen/finops-platform/internal/providers/gcp"
	"github.com/lvonguyen/finops-platform/internal/reporter"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "configs/config.yaml", "Path to configuration file")
	dryRun := flag.Bool("dry-run", false, "Dry run mode - don't send alerts")
	cloud := flag.String("cloud", "all", "Cloud provider to query: aws, azure, gcp, or all")
	startDate := flag.String("start", "", "Start date (YYYY-MM-DD), defaults to first of current month")
	endDate := flag.String("end", "", "End date (YYYY-MM-DD), defaults to today")
	outputFormat := flag.String("format", "html", "Output format: html, csv, json")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Parse dates
	start, end := parseDates(*startDate, *endDate)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal, cancelling...")
		cancel()
	}()

	// Initialize aggregator
	agg := aggregator.New(cfg)

	// Register cloud providers
	if *cloud == "all" || *cloud == "aws" {
		awsProvider, err := aws.NewCostProvider(ctx, cfg.AWS)
		if err != nil {
			log.Printf("Warning: Failed to initialize AWS provider: %v", err)
		} else {
			agg.RegisterProvider("aws", awsProvider)
		}
	}

	if *cloud == "all" || *cloud == "azure" {
		azureProvider, err := azure.NewCostProvider(ctx, cfg.Azure)
		if err != nil {
			log.Printf("Warning: Failed to initialize Azure provider: %v", err)
		} else {
			agg.RegisterProvider("azure", azureProvider)
		}
	}

	if *cloud == "all" || *cloud == "gcp" {
		gcpProvider, err := gcp.NewCostProvider(ctx, cfg.GCP)
		if err != nil {
			log.Printf("Warning: Failed to initialize GCP provider: %v", err)
		} else {
			agg.RegisterProvider("gcp", gcpProvider)
		}
	}

	// Aggregate costs
	log.Printf("Aggregating costs from %s to %s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	
	results, err := agg.Aggregate(ctx, start, end)
	if err != nil {
		log.Fatalf("Failed to aggregate costs: %v", err)
	}

	log.Printf("Retrieved %d cost entries across %d providers", len(results.Entries), len(results.ByProvider))

	// Detect anomalies
	anomalies := agg.DetectAnomalies(results)
	if len(anomalies) > 0 {
		log.Printf("Detected %d cost anomalies", len(anomalies))
	}

	// Check budgets
	budgetAlerts := agg.CheckBudgets(results)
	if len(budgetAlerts) > 0 {
		log.Printf("Detected %d budget alerts", len(budgetAlerts))
	}

	// Generate report
	rep := reporter.New(cfg.Reporter)
	
	reportData := reporter.ReportData{
		Period:       fmt.Sprintf("%s to %s", start.Format("2006-01-02"), end.Format("2006-01-02")),
		Results:      results,
		Anomalies:    anomalies,
		BudgetAlerts: budgetAlerts,
		GeneratedAt:  time.Now(),
	}

	var outputPath string
	switch *outputFormat {
	case "html":
		outputPath, err = rep.GenerateHTML(reportData)
	case "csv":
		outputPath, err = rep.GenerateCSV(reportData)
	case "json":
		outputPath, err = rep.GenerateJSON(reportData)
	default:
		log.Fatalf("Unknown output format: %s", *outputFormat)
	}

	if err != nil {
		log.Fatalf("Failed to generate report: %v", err)
	}

	log.Printf("Report generated: %s", outputPath)

	// Send alerts (unless dry-run)
	if !*dryRun && (len(anomalies) > 0 || len(budgetAlerts) > 0) {
		if err := agg.SendAlerts(ctx, anomalies, budgetAlerts); err != nil {
			log.Printf("Warning: Failed to send some alerts: %v", err)
		}
	}

	// Print summary
	printSummary(results, anomalies, budgetAlerts)
}

func parseDates(startStr, endStr string) (time.Time, time.Time) {
	now := time.Now()
	
	var start, end time.Time
	var err error

	if startStr == "" {
		// Default to first of current month
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	} else {
		start, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			log.Fatalf("Invalid start date format: %v", err)
		}
	}

	if endStr == "" {
		// Default to today
		end = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	} else {
		end, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			log.Fatalf("Invalid end date format: %v", err)
		}
	}

	return start, end
}

func printSummary(results *aggregator.AggregationResult, anomalies []aggregator.Anomaly, budgetAlerts []aggregator.BudgetAlert) {
	separator := strings.Repeat("=", 60)
	fmt.Println("\n" + separator)
	fmt.Println("COST AGGREGATION SUMMARY")
	fmt.Println(separator)

	fmt.Printf("\nTotal Cost: $%.2f\n", results.TotalCost)
	fmt.Println("\nBy Provider:")
	for provider, cost := range results.ByProvider {
		fmt.Printf("  %-10s: $%.2f\n", provider, cost)
	}

	fmt.Println("\nTop 5 Services:")
	for i, entry := range results.TopServices(5) {
		fmt.Printf("  %d. %-30s: $%.2f\n", i+1, entry.Service, entry.Cost)
	}

	if len(anomalies) > 0 {
		fmt.Printf("\nAnomalies Detected: %d\n", len(anomalies))
		for _, a := range anomalies {
			fmt.Printf("  - %s: %.1f%% above expected ($%.2f vs $%.2f expected)\n",
				a.Service, a.PercentageDeviation, a.ActualCost, a.ExpectedCost)
		}
	}

	if len(budgetAlerts) > 0 {
		fmt.Printf("\nBudget Alerts: %d\n", len(budgetAlerts))
		for _, b := range budgetAlerts {
			fmt.Printf("  - %s: $%.2f / $%.2f (%.1f%%)\n",
				b.BudgetName, b.CurrentSpend, b.BudgetLimit, b.PercentUsed)
		}
	}

	fmt.Println("\n" + separator)
}

