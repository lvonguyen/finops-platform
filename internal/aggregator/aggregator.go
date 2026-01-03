// Package aggregator provides cost aggregation and analysis
package aggregator

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/lvonguyen/finops-platform/internal/config"
)

// CostProvider defines the interface for cloud cost providers
type CostProvider interface {
	GetCosts(ctx context.Context, start, end time.Time) ([]CostEntry, error)
	GetBudgets(ctx context.Context) ([]BudgetStatus, error)
	Name() string
}

// CostEntry represents a single cost entry
type CostEntry struct {
	Provider    string            `json:"provider"`
	AccountID   string            `json:"account_id"`
	Service     string            `json:"service"`
	Region      string            `json:"region"`
	Date        time.Time         `json:"date"`
	Cost        float64           `json:"cost"`
	Currency    string            `json:"currency"`
	Tags        map[string]string `json:"tags"`
	UsageType   string            `json:"usage_type"`
	UsageAmount float64           `json:"usage_amount"`
	UsageUnit   string            `json:"usage_unit"`
}

// BudgetStatus represents budget utilization
type BudgetStatus struct {
	BudgetName    string  `json:"budget_name"`
	Provider      string  `json:"provider"`
	Scope         string  `json:"scope"`
	Limit         float64 `json:"limit"`
	CurrentSpend  float64 `json:"current_spend"`
	ForecastSpend float64 `json:"forecast_spend"`
}

// AggregationResult contains aggregated cost data
type AggregationResult struct {
	TotalCost   float64            `json:"total_cost"`
	ByProvider  map[string]float64 `json:"by_provider"`
	ByService   map[string]float64 `json:"by_service"`
	ByAccount   map[string]float64 `json:"by_account"`
	ByRegion    map[string]float64 `json:"by_region"`
	ByDate      map[string]float64 `json:"by_date"`
	Entries     []CostEntry        `json:"entries"`
}

// TopServices returns the top N services by cost
func (r *AggregationResult) TopServices(n int) []CostEntry {
	// Aggregate by service
	serviceMap := make(map[string]float64)
	for _, e := range r.Entries {
		key := fmt.Sprintf("%s:%s", e.Provider, e.Service)
		serviceMap[key] += e.Cost
	}

	// Convert to slice
	type serviceCost struct {
		Service string
		Cost    float64
	}
	services := make([]serviceCost, 0, len(serviceMap))
	for k, v := range serviceMap {
		services = append(services, serviceCost{k, v})
	}

	// Sort by cost descending
	sort.Slice(services, func(i, j int) bool {
		return services[i].Cost > services[j].Cost
	})

	// Return top N as CostEntry
	result := make([]CostEntry, 0, n)
	for i := 0; i < n && i < len(services); i++ {
		result = append(result, CostEntry{
			Service: services[i].Service,
			Cost:    services[i].Cost,
		})
	}
	return result
}

// Anomaly represents a cost anomaly
type Anomaly struct {
	Provider            string    `json:"provider"`
	Service             string    `json:"service"`
	AccountID           string    `json:"account_id"`
	Date                time.Time `json:"date"`
	ActualCost          float64   `json:"actual_cost"`
	ExpectedCost        float64   `json:"expected_cost"`
	PercentageDeviation float64   `json:"percentage_deviation"`
	Severity            string    `json:"severity"`
}

// BudgetAlert represents a budget threshold alert
type BudgetAlert struct {
	BudgetName   string    `json:"budget_name"`
	Provider     string    `json:"provider"`
	Scope        string    `json:"scope"`
	BudgetLimit  float64   `json:"budget_limit"`
	CurrentSpend float64   `json:"current_spend"`
	PercentUsed  float64   `json:"percent_used"`
	Severity     string    `json:"severity"`
	AlertedAt    time.Time `json:"alerted_at"`
}

// Aggregator orchestrates cost aggregation across providers
type Aggregator struct {
	config    *config.Config
	providers map[string]CostProvider
	mu        sync.RWMutex
}

// New creates a new Aggregator
func New(cfg *config.Config) *Aggregator {
	return &Aggregator{
		config:    cfg,
		providers: make(map[string]CostProvider),
	}
}

// RegisterProvider registers a cost provider
func (a *Aggregator) RegisterProvider(name string, provider CostProvider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.providers[name] = provider
}

// Aggregate fetches and aggregates costs from all providers
func (a *Aggregator) Aggregate(ctx context.Context, start, end time.Time) (*AggregationResult, error) {
	a.mu.RLock()
	providers := make(map[string]CostProvider)
	for k, v := range a.providers {
		providers[k] = v
	}
	a.mu.RUnlock()

	result := &AggregationResult{
		ByProvider: make(map[string]float64),
		ByService:  make(map[string]float64),
		ByAccount:  make(map[string]float64),
		ByRegion:   make(map[string]float64),
		ByDate:     make(map[string]float64),
		Entries:    make([]CostEntry, 0),
	}

	// Fetch from all providers concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	errCh := make(chan error, len(providers))

	for name, provider := range providers {
		wg.Add(1)
		go func(name string, provider CostProvider) {
			defer wg.Done()

			entries, err := provider.GetCosts(ctx, start, end)
			if err != nil {
				errCh <- fmt.Errorf("%s: %w", name, err)
				return
			}

			mu.Lock()
			defer mu.Unlock()

			for _, entry := range entries {
				result.Entries = append(result.Entries, entry)
				result.TotalCost += entry.Cost
				result.ByProvider[entry.Provider] += entry.Cost
				result.ByService[entry.Service] += entry.Cost
				result.ByAccount[entry.AccountID] += entry.Cost
				result.ByRegion[entry.Region] += entry.Cost
				result.ByDate[entry.Date.Format("2006-01-02")] += entry.Cost
			}
		}(name, provider)
	}

	wg.Wait()
	close(errCh)

	// Collect errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 && len(errs) == len(providers) {
		// All providers failed
		return nil, fmt.Errorf("all providers failed: %v", errs)
	}

	return result, nil
}

// DetectAnomalies identifies cost anomalies
func (a *Aggregator) DetectAnomalies(result *AggregationResult) []Anomaly {
	if !a.config.Anomaly.Enabled {
		return nil
	}

	anomalies := make([]Anomaly, 0)
	threshold := a.config.Anomaly.DeviationThreshold
	minCost := a.config.Anomaly.MinimumCostThreshold

	// Group by service for comparison
	serviceDaily := make(map[string][]float64)
	for _, entry := range result.Entries {
		key := fmt.Sprintf("%s:%s:%s", entry.Provider, entry.AccountID, entry.Service)
		serviceDaily[key] = append(serviceDaily[key], entry.Cost)
	}

	// Calculate statistics and detect anomalies
	for key, costs := range serviceDaily {
		if len(costs) < 7 {
			continue // Need enough data points
		}

		mean, stdDev := calculateStats(costs)
		if mean < minCost {
			continue // Below minimum threshold
		}

		// Check most recent cost
		recent := costs[len(costs)-1]
		deviation := ((recent - mean) / mean) * 100

		if deviation > threshold {
			severity := "low"
			if deviation > threshold*2 {
				severity = "medium"
			}
			if deviation > threshold*3 {
				severity = "high"
			}

			anomalies = append(anomalies, Anomaly{
				Service:             key,
				ActualCost:          recent,
				ExpectedCost:        mean,
				PercentageDeviation: deviation,
				Severity:            severity,
			})
		}

		// Also check using z-score
		if stdDev > 0 {
			zScore := (recent - mean) / stdDev
			if zScore > 2.5 {
				// Already added above, just log
			}
		}
	}

	return anomalies
}

// CheckBudgets checks budget thresholds
func (a *Aggregator) CheckBudgets(result *AggregationResult) []BudgetAlert {
	alerts := make([]BudgetAlert, 0)

	for _, budget := range a.config.Budgets {
		var currentSpend float64

		if budget.Provider == "all" {
			currentSpend = result.TotalCost
		} else {
			currentSpend = result.ByProvider[budget.Provider]
		}

		if budget.Scope != "" {
			currentSpend = result.ByAccount[budget.Scope]
		}

		percentUsed := (currentSpend / budget.MonthlyLimit) * 100

		// Check each alert threshold
		for _, alertAt := range budget.AlertAt {
			if percentUsed >= float64(alertAt) {
				severity := "info"
				if alertAt >= 90 {
					severity = "high"
				} else if alertAt >= 75 {
					severity = "medium"
				} else if alertAt >= 50 {
					severity = "low"
				}

				alerts = append(alerts, BudgetAlert{
					BudgetName:   budget.Name,
					Provider:     budget.Provider,
					Scope:        budget.Scope,
					BudgetLimit:  budget.MonthlyLimit,
					CurrentSpend: currentSpend,
					PercentUsed:  percentUsed,
					Severity:     severity,
					AlertedAt:    time.Now(),
				})
				break // Only alert once per budget
			}
		}
	}

	return alerts
}

// SendAlerts sends alerts for anomalies and budget issues
func (a *Aggregator) SendAlerts(ctx context.Context, anomalies []Anomaly, budgetAlerts []BudgetAlert) error {
	// Implementation would send to Slack, email, etc.
	// For now, just log
	return nil
}

func calculateStats(values []float64) (mean, stdDev float64) {
	n := float64(len(values))
	if n == 0 {
		return 0, 0
	}

	// Calculate mean
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean = sum / n

	// Calculate standard deviation
	var sumSquares float64
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}
	stdDev = math.Sqrt(sumSquares / n)

	return mean, stdDev
}

