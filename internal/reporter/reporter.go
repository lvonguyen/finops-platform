// Package reporter generates cost reports
package reporter

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"

	"github.com/lvonguyen/finops-platform/internal/aggregator"
	"github.com/lvonguyen/finops-platform/internal/config"
)

// ReportData contains all data for report generation
type ReportData struct {
	Period       string
	Results      *aggregator.AggregationResult
	Anomalies    []aggregator.Anomaly
	BudgetAlerts []aggregator.BudgetAlert
	GeneratedAt  time.Time
}

// Reporter generates cost reports
type Reporter struct {
	config config.ReporterConfig
}

// New creates a new Reporter
func New(cfg config.ReporterConfig) *Reporter {
	return &Reporter{config: cfg}
}

// GenerateHTML generates an HTML report
func (r *Reporter) GenerateHTML(data ReportData) (string, error) {
	// Ensure output directory exists
	if err := os.MkdirAll(r.config.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := fmt.Sprintf("cost-report-%s.html", time.Now().Format("20060102-150405"))
	outputPath := filepath.Join(r.config.OutputDir, filename)

	f, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	tmpl := template.Must(template.New("report").Parse(htmlTemplate))
	if err := tmpl.Execute(f, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return outputPath, nil
}

// GenerateCSV generates a CSV report
func (r *Reporter) GenerateCSV(data ReportData) (string, error) {
	if err := os.MkdirAll(r.config.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := fmt.Sprintf("cost-report-%s.csv", time.Now().Format("20060102-150405"))
	outputPath := filepath.Join(r.config.OutputDir, filename)

	f, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	// Header
	writer.Write([]string{"Provider", "AccountID", "Service", "Region", "Date", "Cost", "Currency"})

	// Data rows
	for _, entry := range data.Results.Entries {
		writer.Write([]string{
			entry.Provider,
			entry.AccountID,
			entry.Service,
			entry.Region,
			entry.Date.Format("2006-01-02"),
			fmt.Sprintf("%.2f", entry.Cost),
			entry.Currency,
		})
	}

	return outputPath, nil
}

// GenerateJSON generates a JSON report
func (r *Reporter) GenerateJSON(data ReportData) (string, error) {
	if err := os.MkdirAll(r.config.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := fmt.Sprintf("cost-report-%s.json", time.Now().Format("20060102-150405"))
	outputPath := filepath.Join(r.config.OutputDir, filename)

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return outputPath, nil
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Cloud Cost Report - {{.Period}}</title>
    <style>
        :root {
            --bg-dark: #0f172a;
            --bg-card: #1e293b;
            --text-primary: #f1f5f9;
            --text-secondary: #94a3b8;
            --accent-blue: #3b82f6;
            --accent-green: #22c55e;
            --accent-yellow: #eab308;
            --accent-red: #ef4444;
            --border: #334155;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
            background: var(--bg-dark);
            color: var(--text-primary);
            line-height: 1.6;
            padding: 2rem;
        }
        .container { max-width: 1400px; margin: 0 auto; }
        h1 {
            font-size: 2rem;
            margin-bottom: 0.5rem;
            background: linear-gradient(135deg, var(--accent-blue), #8b5cf6);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }
        .subtitle { color: var(--text-secondary); margin-bottom: 2rem; }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }
        .stat-card {
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: 12px;
            padding: 1.5rem;
        }
        .stat-label { color: var(--text-secondary); font-size: 0.875rem; }
        .stat-value { font-size: 2rem; font-weight: 700; }
        .stat-value.green { color: var(--accent-green); }
        .stat-value.yellow { color: var(--accent-yellow); }
        .stat-value.red { color: var(--accent-red); }
        .section { margin-bottom: 2rem; }
        .section-title {
            font-size: 1.25rem;
            margin-bottom: 1rem;
            padding-bottom: 0.5rem;
            border-bottom: 1px solid var(--border);
        }
        table {
            width: 100%;
            border-collapse: collapse;
            background: var(--bg-card);
            border-radius: 12px;
            overflow: hidden;
        }
        th, td { padding: 1rem; text-align: left; }
        th {
            background: rgba(59, 130, 246, 0.1);
            font-weight: 600;
            color: var(--accent-blue);
        }
        tr:not(:last-child) { border-bottom: 1px solid var(--border); }
        .badge {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            border-radius: 9999px;
            font-size: 0.75rem;
            font-weight: 600;
        }
        .badge.low { background: rgba(34, 197, 94, 0.2); color: var(--accent-green); }
        .badge.medium { background: rgba(234, 179, 8, 0.2); color: var(--accent-yellow); }
        .badge.high { background: rgba(239, 68, 68, 0.2); color: var(--accent-red); }
        .provider-breakdown {
            display: flex;
            gap: 1rem;
            flex-wrap: wrap;
        }
        .provider-item {
            flex: 1;
            min-width: 200px;
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: 8px;
            padding: 1rem;
        }
        .footer {
            margin-top: 3rem;
            padding-top: 1rem;
            border-top: 1px solid var(--border);
            color: var(--text-secondary);
            font-size: 0.875rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Multi-Cloud Cost Report</h1>
        <p class="subtitle">{{.Period}} | Generated: {{.GeneratedAt.Format "2006-01-02 15:04:05 MST"}}</p>

        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-label">Total Cost</div>
                <div class="stat-value">${{printf "%.2f" .Results.TotalCost}}</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Providers</div>
                <div class="stat-value">{{len .Results.ByProvider}}</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Anomalies</div>
                <div class="stat-value {{if gt (len .Anomalies) 0}}red{{else}}green{{end}}">{{len .Anomalies}}</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Budget Alerts</div>
                <div class="stat-value {{if gt (len .BudgetAlerts) 0}}yellow{{else}}green{{end}}">{{len .BudgetAlerts}}</div>
            </div>
        </div>

        <div class="section">
            <h2 class="section-title">Cost by Provider</h2>
            <div class="provider-breakdown">
                {{range $provider, $cost := .Results.ByProvider}}
                <div class="provider-item">
                    <div class="stat-label">{{$provider}}</div>
                    <div class="stat-value">${{printf "%.2f" $cost}}</div>
                </div>
                {{end}}
            </div>
        </div>

        {{if .Anomalies}}
        <div class="section">
            <h2 class="section-title">Cost Anomalies</h2>
            <table>
                <thead>
                    <tr>
                        <th>Service</th>
                        <th>Actual Cost</th>
                        <th>Expected</th>
                        <th>Deviation</th>
                        <th>Severity</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Anomalies}}
                    <tr>
                        <td>{{.Service}}</td>
                        <td>${{printf "%.2f" .ActualCost}}</td>
                        <td>${{printf "%.2f" .ExpectedCost}}</td>
                        <td>+{{printf "%.1f" .PercentageDeviation}}%</td>
                        <td><span class="badge {{.Severity}}">{{.Severity}}</span></td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
        {{end}}

        {{if .BudgetAlerts}}
        <div class="section">
            <h2 class="section-title">Budget Alerts</h2>
            <table>
                <thead>
                    <tr>
                        <th>Budget</th>
                        <th>Provider</th>
                        <th>Current Spend</th>
                        <th>Limit</th>
                        <th>Usage</th>
                        <th>Severity</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .BudgetAlerts}}
                    <tr>
                        <td>{{.BudgetName}}</td>
                        <td>{{.Provider}}</td>
                        <td>${{printf "%.2f" .CurrentSpend}}</td>
                        <td>${{printf "%.2f" .BudgetLimit}}</td>
                        <td>{{printf "%.1f" .PercentUsed}}%</td>
                        <td><span class="badge {{.Severity}}">{{.Severity}}</span></td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
        {{end}}

        <div class="section">
            <h2 class="section-title">Top Services by Cost</h2>
            <table>
                <thead>
                    <tr>
                        <th>Service</th>
                        <th>Cost</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Results.TopServices 10}}
                    <tr>
                        <td>{{.Service}}</td>
                        <td>${{printf "%.2f" .Cost}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>

        <div class="footer">
            <p>Generated by FinOps Cost Aggregator | github.com/lvonguyen/finops-platform</p>
        </div>
    </div>
</body>
</html>`

