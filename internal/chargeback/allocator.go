// Package chargeback provides cost allocation and showback functionality.
package chargeback

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/lvonguyen/finops-platform/internal/normalizer"
)

// AllocatorConfig holds configuration for cost allocation
type AllocatorConfig struct {
	PrimaryTag     string // Primary tag for allocation (e.g., cost_center)
	FallbackTag    string // Fallback tag if primary missing
	UntaggedPool   string // Where to allocate untagged costs
	SharedCostSplit []SharedCostRule
}

// SharedCostRule defines how to split shared costs
type SharedCostRule struct {
	CostCenter string
	Percentage float64
}

// Allocation represents allocated costs for a cost center
type Allocation struct {
	CostCenter   string             `json:"cost_center"`
	TotalCost    float64            `json:"total_cost"`
	DirectCost   float64            `json:"direct_cost"`   // Directly tagged
	AllocatedCost float64           `json:"allocated_cost"` // Allocated from shared
	ByCloud      map[string]float64 `json:"by_cloud"`
	ByService    map[string]float64 `json:"by_service"`
	Records      []normalizer.CostRecord `json:"-"`
}

// Allocator performs tag-based cost allocation
type Allocator struct {
	config AllocatorConfig
}

// NewAllocator creates a new cost allocator
func NewAllocator(cfg AllocatorConfig) *Allocator {
	return &Allocator{config: cfg}
}

// Allocate distributes costs to cost centers based on tags
func (a *Allocator) Allocate(records []normalizer.CostRecord) map[string]*Allocation {
	allocations := make(map[string]*Allocation)
	var untaggedCosts []normalizer.CostRecord

	for _, r := range records {
		costCenter := a.getCostCenter(r)

		if costCenter == "" {
			untaggedCosts = append(untaggedCosts, r)
			continue
		}

		if _, exists := allocations[costCenter]; !exists {
			allocations[costCenter] = &Allocation{
				CostCenter: costCenter,
				ByCloud:    make(map[string]float64),
				ByService:  make(map[string]float64),
			}
		}

		alloc := allocations[costCenter]
		alloc.TotalCost += r.Cost
		alloc.DirectCost += r.Cost
		alloc.ByCloud[r.Cloud] += r.Cost
		alloc.ByService[r.Service] += r.Cost
		alloc.Records = append(alloc.Records, r)
	}

	// Handle untagged costs
	a.allocateUntagged(allocations, untaggedCosts)

	return allocations
}

// getCostCenter extracts the cost center from a record's tags
func (a *Allocator) getCostCenter(r normalizer.CostRecord) string {
	// Try primary tag
	if cc, ok := r.Tags[a.config.PrimaryTag]; ok && cc != "" {
		return cc
	}

	// Try fallback tag
	if cc, ok := r.Tags[a.config.FallbackTag]; ok && cc != "" {
		return cc
	}

	return ""
}

// allocateUntagged distributes untagged costs
func (a *Allocator) allocateUntagged(allocations map[string]*Allocation, untagged []normalizer.CostRecord) {
	if len(untagged) == 0 {
		return
	}

	// Calculate total untagged cost
	var totalUntagged float64
	for _, r := range untagged {
		totalUntagged += r.Cost
	}

	// If we have shared cost rules, use them
	if len(a.config.SharedCostSplit) > 0 {
		remainingPct := 100.0

		for _, rule := range a.config.SharedCostSplit {
			if _, exists := allocations[rule.CostCenter]; !exists {
				allocations[rule.CostCenter] = &Allocation{
					CostCenter: rule.CostCenter,
					ByCloud:    make(map[string]float64),
					ByService:  make(map[string]float64),
				}
			}

			allocated := totalUntagged * (rule.Percentage / 100)
			allocations[rule.CostCenter].AllocatedCost += allocated
			allocations[rule.CostCenter].TotalCost += allocated
			remainingPct -= rule.Percentage
		}

		// Distribute remaining proportionally
		if remainingPct > 0 {
			a.distributeProportionally(allocations, totalUntagged*(remainingPct/100))
		}
	} else if a.config.UntaggedPool != "" {
		// Allocate all to untagged pool
		if _, exists := allocations[a.config.UntaggedPool]; !exists {
			allocations[a.config.UntaggedPool] = &Allocation{
				CostCenter: a.config.UntaggedPool,
				ByCloud:    make(map[string]float64),
				ByService:  make(map[string]float64),
			}
		}
		allocations[a.config.UntaggedPool].TotalCost += totalUntagged
		allocations[a.config.UntaggedPool].AllocatedCost += totalUntagged

		for _, r := range untagged {
			allocations[a.config.UntaggedPool].ByCloud[r.Cloud] += r.Cost
			allocations[a.config.UntaggedPool].ByService[r.Service] += r.Cost
		}
	} else {
		// Distribute proportionally to existing cost centers
		a.distributeProportionally(allocations, totalUntagged)
	}
}

// distributeProportionally allocates costs based on existing spend
func (a *Allocator) distributeProportionally(allocations map[string]*Allocation, amount float64) {
	var totalDirect float64
	for _, alloc := range allocations {
		totalDirect += alloc.DirectCost
	}

	if totalDirect == 0 {
		return
	}

	for _, alloc := range allocations {
		proportion := alloc.DirectCost / totalDirect
		allocated := amount * proportion
		alloc.AllocatedCost += allocated
		alloc.TotalCost += allocated
	}
}

// Report holds a generated chargeback report
type Report struct {
	Month       string
	Allocations []*Allocation
	TotalCost   float64
	Generated   time.Time
}

// GenerateReport creates a chargeback report from allocations
func GenerateReport(allocations map[string]*Allocation, month string) *Report {
	report := &Report{
		Month:     month,
		Generated: time.Now(),
	}

	for _, alloc := range allocations {
		report.Allocations = append(report.Allocations, alloc)
		report.TotalCost += alloc.TotalCost
	}

	// Sort by cost descending
	sort.Slice(report.Allocations, func(i, j int) bool {
		return report.Allocations[i].TotalCost > report.Allocations[j].TotalCost
	})

	return report
}

// SaveCSV saves the report as a CSV file
func (r *Report) SaveCSV(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Header
	header := []string{"Cost Center", "Total Cost", "Direct Cost", "Allocated Cost", "AWS", "Azure", "GCP", "% of Total"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Data rows
	for _, alloc := range r.Allocations {
		pct := (alloc.TotalCost / r.TotalCost) * 100
		row := []string{
			alloc.CostCenter,
			fmt.Sprintf("%.2f", alloc.TotalCost),
			fmt.Sprintf("%.2f", alloc.DirectCost),
			fmt.Sprintf("%.2f", alloc.AllocatedCost),
			fmt.Sprintf("%.2f", alloc.ByCloud["aws"]),
			fmt.Sprintf("%.2f", alloc.ByCloud["azure"]),
			fmt.Sprintf("%.2f", alloc.ByCloud["gcp"]),
			fmt.Sprintf("%.1f%%", pct),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	// Total row
	totalRow := []string{
		"TOTAL",
		fmt.Sprintf("%.2f", r.TotalCost),
		"", "", "", "", "",
		"100.0%",
	}
	return writer.Write(totalRow)
}

