// Package normalizer provides common schema for multi-cloud cost data.
package normalizer

import (
	"time"
)

// CostRecord represents a normalized cost record from any cloud provider
type CostRecord struct {
	// Identification
	ID       string `json:"id"`
	Cloud    string `json:"cloud"`     // aws, azure, gcp
	Account  string `json:"account"`   // Account/Subscription/Project ID
	Region   string `json:"region"`
	Service  string `json:"service"`   // Normalized service name
	Resource string `json:"resource"`  // Resource identifier

	// Cost
	Cost          float64 `json:"cost"`
	Currency      string  `json:"currency"`       // USD
	UsageQuantity float64 `json:"usage_quantity"`
	UsageUnit     string  `json:"usage_unit"`
	PricingModel  string  `json:"pricing_model"`  // on_demand, reserved, spot, savings_plan

	// Time
	Date       time.Time `json:"date"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`

	// Tags for chargeback
	Tags map[string]string `json:"tags"`

	// Metadata
	CloudService     string `json:"cloud_service"`      // Original cloud service name
	CloudServiceType string `json:"cloud_service_type"` // E.g., EC2-Instance, Lambda
}

// CostSummary holds aggregated cost data
type CostSummary struct {
	TotalCost     float64            `json:"total_cost"`
	Currency      string             `json:"currency"`
	StartDate     time.Time          `json:"start_date"`
	EndDate       time.Time          `json:"end_date"`
	ByCloud       map[string]float64 `json:"by_cloud"`
	ByService     map[string]float64 `json:"by_service"`
	ByAccount     map[string]float64 `json:"by_account"`
	ByRegion      map[string]float64 `json:"by_region"`
	ByCostCenter  map[string]float64 `json:"by_cost_center"`
	DailyCosts    []DailyCost        `json:"daily_costs"`
}

// DailyCost holds daily cost breakdown
type DailyCost struct {
	Date    time.Time          `json:"date"`
	Total   float64            `json:"total"`
	ByCloud map[string]float64 `json:"by_cloud"`
}

// Summarize aggregates cost records into a summary
func Summarize(records []CostRecord) CostSummary {
	summary := CostSummary{
		Currency:     "USD",
		ByCloud:      make(map[string]float64),
		ByService:    make(map[string]float64),
		ByAccount:    make(map[string]float64),
		ByRegion:     make(map[string]float64),
		ByCostCenter: make(map[string]float64),
	}

	if len(records) == 0 {
		return summary
	}

	// Track date range
	summary.StartDate = records[0].Date
	summary.EndDate = records[0].Date

	// Daily aggregation
	dailyMap := make(map[string]*DailyCost)

	for _, r := range records {
		summary.TotalCost += r.Cost
		summary.ByCloud[r.Cloud] += r.Cost
		summary.ByService[r.Service] += r.Cost
		summary.ByAccount[r.Account] += r.Cost
		summary.ByRegion[r.Region] += r.Cost

		// Cost center from tags
		if cc, ok := r.Tags["cost_center"]; ok {
			summary.ByCostCenter[cc] += r.Cost
		} else {
			summary.ByCostCenter["UNTAGGED"] += r.Cost
		}

		// Track date range
		if r.Date.Before(summary.StartDate) {
			summary.StartDate = r.Date
		}
		if r.Date.After(summary.EndDate) {
			summary.EndDate = r.Date
		}

		// Daily aggregation
		dateKey := r.Date.Format("2006-01-02")
		if _, exists := dailyMap[dateKey]; !exists {
			dailyMap[dateKey] = &DailyCost{
				Date:    r.Date,
				ByCloud: make(map[string]float64),
			}
		}
		dailyMap[dateKey].Total += r.Cost
		dailyMap[dateKey].ByCloud[r.Cloud] += r.Cost
	}

	// Convert daily map to slice
	for _, dc := range dailyMap {
		summary.DailyCosts = append(summary.DailyCosts, *dc)
	}

	return summary
}

// ServiceMapping maps cloud-specific services to normalized names
var ServiceMapping = map[string]map[string]string{
	"aws": {
		"Amazon Elastic Compute Cloud - Compute": "Compute",
		"Amazon Relational Database Service":     "Database",
		"Amazon Simple Storage Service":          "Storage",
		"AWS Lambda":                              "Serverless",
		"Amazon Virtual Private Cloud":           "Networking",
		"Amazon CloudWatch":                      "Monitoring",
	},
	"azure": {
		"Virtual Machines":          "Compute",
		"Azure SQL Database":        "Database",
		"Storage":                   "Storage",
		"Azure Functions":           "Serverless",
		"Virtual Network":           "Networking",
		"Azure Monitor":             "Monitoring",
	},
	"gcp": {
		"Compute Engine":            "Compute",
		"Cloud SQL":                 "Database",
		"Cloud Storage":             "Storage",
		"Cloud Functions":           "Serverless",
		"Virtual Private Cloud":     "Networking",
		"Cloud Monitoring":          "Monitoring",
	},
}

// NormalizeService converts cloud-specific service names to normalized names
func NormalizeService(cloud, cloudService string) string {
	if mapping, ok := ServiceMapping[cloud]; ok {
		if normalized, ok := mapping[cloudService]; ok {
			return normalized
		}
	}
	return cloudService // Return original if no mapping found
}

