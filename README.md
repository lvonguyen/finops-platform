# FinOps Cost Management Platform

**Multi-Cloud Cost Aggregation, Anomaly Detection, and Chargeback**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![AWS](https://img.shields.io/badge/AWS-Cost%20Explorer-FF9900?style=flat&logo=amazon-aws)](https://aws.amazon.com/aws-cost-management/aws-cost-explorer/)
[![Azure](https://img.shields.io/badge/Azure-Cost%20Management-0078D4?style=flat&logo=microsoft-azure)](https://azure.microsoft.com/en-us/services/cost-management/)
[![GCP](https://img.shields.io/badge/GCP-Billing-4285F4?style=flat&logo=google-cloud)](https://cloud.google.com/billing)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A production-ready FinOps platform that aggregates cloud costs across AWS, Azure, and GCP with anomaly detection, budget alerting, and automated chargeback reporting.

## What This Solves

**The Problem:** Multi-cloud cost management is fragmented and reactive:
- Cost data scattered across AWS Cost Explorer, Azure Cost Management, GCP Billing
- No unified view of total cloud spend by team, project, or application
- Surprise bills from untagged resources or runaway spend
- Manual chargeback processes that take days to complete

**The Solution:** Unified FinOps platform with:
- Automated cost aggregation from all major cloud providers
- Real-time anomaly detection with configurable thresholds
- Tag-based chargeback allocation with showback reports
- Budget tracking with proactive alerting
- Optimization recommendations based on usage patterns

## Architecture

```
                              FinOps Cost Management Platform

    AWS Cost Explorer        Azure Cost Management        GCP Cloud Billing
          |                         |                            |
          v                         v                            v
    +----------+              +----------+               +----------+
    | Cost API |              | Cost API |               |BigQuery  |
    | Reader   |              | Reader   |               |Export    |
    +----+-----+              +----+-----+               +----+-----+
         |                         |                          |
         +------------+------------+------------+-------------+
                      |                         |
                      v                         v
         +------------------------+   +------------------------+
         |   Cost Normalizer      |   |   Tag Enrichment       |
         |   (Common Schema)      |   |   (CMDB Integration)   |
         +------------------------+   +------------------------+
                      |                         |
                      +------------+------------+
                                   |
                      +------------+------------+
                      |                         |
                      v                         v
         +------------------------+   +------------------------+
         |   Anomaly Detection    |   |   Chargeback Engine    |
         |   (ML-based)           |   |   (Allocation Rules)   |
         +------------------------+   +------------------------+
                      |                         |
         +------------+------------+------------+
         |                         |            |
         v                         v            v
    +---------+            +----------+   +---------+
    | Alerts  |            | Reports  |   | Budget  |
    | Slack/  |            | CSV/PDF  |   | Tracking|
    | Email   |            | Dashboard|   |         |
    +---------+            +----------+   +---------+
```

## Features

### Cost Aggregation
| Cloud | API | Data Granularity |
|-------|-----|------------------|
| AWS | Cost Explorer API | Daily/Hourly |
| Azure | Cost Management API | Daily |
| GCP | BigQuery Billing Export | Daily/Hourly |

### Anomaly Detection
- Statistical anomaly detection (Z-score, IQR)
- ML-based forecasting with Prophet
- Configurable sensitivity thresholds
- Per-service and per-account baselines

### Chargeback & Showback
- Tag-based cost allocation rules
- Split costs by percentage or usage
- Untagged cost handling strategies
- CSV/PDF report generation
- Integration with billing systems

### Budget Management
- Multi-cloud budget tracking
- Forecasted spend vs budget
- Proactive threshold alerts
- Slack/Email/PagerDuty notifications

## Project Structure

```
finops-platform/
├── cmd/
│   └── aggregator/
│       └── main.go              # CLI entrypoint
├── internal/
│   ├── aggregator/
│   │   └── aggregator.go        # Core aggregation engine
│   ├── config/
│   │   └── config.go            # Configuration management
│   ├── providers/
│   │   ├── aws/
│   │   │   └── cost.go          # AWS Cost Explorer client
│   │   ├── azure/
│   │   │   └── cost.go          # Azure Cost Management client
│   │   └── gcp/
│   │       └── cost.go          # GCP BigQuery Billing client
│   ├── normalizer/
│   │   └── schema.go            # Common cost schema
│   ├── anomaly/
│   │   └── detector.go          # Statistical anomaly detection
│   ├── chargeback/
│   │   └── allocator.go         # Cost allocation engine
│   ├── reporter/
│   │   └── reporter.go          # HTML/CSV report generation
│   └── alerts/                  # Alerting integrations
├── configs/
│   └── config.yaml              # Configuration template
├── reports/                     # Generated reports
├── go.mod
└── README.md
```

## Quick Start

### Prerequisites

- Go 1.21+
- Cloud credentials with billing read access
- (Optional) Slack webhook for alerts

### Cloud Permissions

| Cloud | Required Permissions |
|-------|---------------------|
| AWS | ce:GetCostAndUsage, ce:GetCostForecast |
| Azure | Cost Management Reader role |
| GCP | BigQuery Data Viewer on billing export dataset |

### Run the Aggregator

```bash
# Clone repository
git clone https://github.com/lvonguyen/finops-platform.git
cd finops-platform

# Build
go build -o bin/aggregator ./cmd/aggregator

# Set credentials
export AWS_PROFILE=finops-readonly
export AZURE_SUBSCRIPTION_ID=your-subscription
export GCP_PROJECT_ID=your-project
export GCP_BILLING_DATASET=billing_export

# Run cost aggregation
./bin/aggregator --config configs/config.yaml

# Generate chargeback report
./bin/aggregator --mode chargeback --month 2024-01

# Check for anomalies
./bin/aggregator --mode anomaly --days 7
```

### CLI Commands

| Command | Description |
|---------|-------------|
| `--mode aggregate` | Aggregate costs from all clouds (default) |
| `--mode chargeback` | Generate chargeback reports |
| `--mode anomaly` | Run anomaly detection |
| `--mode forecast` | Generate spend forecasts |
| `--mode budget` | Check budget status |

## Configuration

```yaml
# configs/config.yaml
providers:
  aws:
    enabled: true
    regions:
      - us-east-1
      - us-west-2
    granularity: DAILY  # DAILY or HOURLY
    
  azure:
    enabled: true
    subscriptions:
      - subscription-id-1
      - subscription-id-2
      
  gcp:
    enabled: true
    billing_account: "01234-ABCDE-56789"
    dataset: "billing_export"

# Cost allocation rules
chargeback:
  rules:
    - tag: cost_center
      priority: 1
    - tag: team
      priority: 2
    - tag: application
      priority: 3
      
  untagged_handling: "shared"  # shared, default_cc, exclude
  default_cost_center: "IT-SHARED"
  
  # Split shared costs
  shared_cost_split:
    - cost_center: "PLATFORM"
      percentage: 30
    - cost_center: "SECURITY" 
      percentage: 10
    # Remaining 60% split by usage

# Anomaly detection
anomaly:
  sensitivity: medium  # low, medium, high
  baseline_days: 30
  min_daily_spend: 100  # Ignore services under this threshold
  
# Budget tracking
budgets:
  - name: "Total Cloud Spend"
    amount: 500000
    period: monthly
    alerts:
      - threshold: 80
        channel: slack
      - threshold: 100
        channel: pagerduty
        
  - name: "Engineering Team"
    filter:
      tag: team
      value: engineering
    amount: 150000
    period: monthly

# Alerting
alerts:
  slack:
    webhook_url: ${SLACK_WEBHOOK_URL}
    channel: "#finops-alerts"
    
  email:
    smtp_server: smtp.example.com
    recipients:
      - finops@example.com
```

## Sample Output

### Cost Summary Report

```
╔══════════════════════════════════════════════════════════════════════════════╗
║                Multi-Cloud Cost Summary - January 2024                       ║
╠══════════════════════════════════════════════════════════════════════════════╣
║                                                                              ║
║  Total Spend: $347,892.45                                                    ║
║  vs Last Month: +$12,341 (+3.7%)                                             ║
║  vs Budget: 92.5% ($376,000)                                                 ║
║  Forecast (EOM): $352,100                                                    ║
║                                                                              ║
║  By Cloud:                                                                   ║
║    AWS:   $198,234  (57.0%)  [+2.1%]                                         ║
║    Azure: $112,456  (32.3%)  [+5.8%]                                         ║
║    GCP:   $37,202   (10.7%)  [+4.2%]                                         ║
║                                                                              ║
║  Top 5 Services:                                                             ║
║    1. EC2            $78,234  (22.5%)                                        ║
║    2. RDS            $45,123  (13.0%)                                        ║
║    3. Azure VMs      $38,456  (11.1%)                                        ║
║    4. S3             $23,890   (6.9%)                                        ║
║    5. Lambda         $18,234   (5.2%)                                        ║
║                                                                              ║
║  Anomalies Detected: 3                                                       ║
║    - Lambda costs +234% (training job)                                       ║
║    - NAT Gateway costs +45% (new egress)                                     ║
║    - Azure Storage +28% (backup retention)                                   ║
║                                                                              ║
║  Optimization Opportunities: $12,340/month                                   ║
║    - 23 idle EC2 instances                                                   ║
║    - 15 unattached EBS volumes                                               ║
║    - 8 oversized RDS instances                                               ║
║                                                                              ║
╚══════════════════════════════════════════════════════════════════════════════╝
```

### Chargeback Report

| Cost Center | AWS | Azure | GCP | Total | % of Total |
|-------------|-----|-------|-----|-------|------------|
| Engineering | $89,234 | $45,678 | $18,234 | $153,146 | 44.0% |
| Data | $56,234 | $23,456 | $12,345 | $92,035 | 26.5% |
| Platform | $34,567 | $28,901 | $4,567 | $68,035 | 19.6% |
| Security | $12,199 | $9,421 | $2,056 | $23,676 | 6.8% |
| Untagged | $6,000 | $5,000 | $0 | $11,000 | 3.2% |
| **Total** | **$198,234** | **$112,456** | **$37,202** | **$347,892** | **100%** |

## Interview Talking Points

This project demonstrates:

1. **FinOps Expertise**
   - Multi-cloud cost aggregation and normalization
   - Tag-based showback and chargeback strategies
   - Budget management and forecasting

2. **Data Engineering**
   - ETL pipeline for billing data
   - Time-series analysis for anomaly detection
   - Report generation at scale

3. **Cloud Financial Management**
   - Understanding of cloud pricing models
   - Cost optimization recommendations
   - Reserved instance / Savings Plan analysis

4. **Platform Engineering**
   - Self-service cost visibility for teams
   - Integration with enterprise billing systems
   - Automated compliance with tagging policies

## Related Projects

- [cspm-aggregator](https://github.com/lvonguyen/cspm-aggregator) - Security findings aggregation
- [CloudForge](https://github.com/lvonguyen/cloudforge) - Policy enforcement (cost policies)
- [multicloud-observability](https://github.com/lvonguyen/multicloud-observability) - Correlate cost with performance

## License

MIT License - See [LICENSE](LICENSE)

## Author

**Liem Vo-Nguyen**
- LinkedIn: [linkedin.com/in/liemvn](https://linkedin.com/in/liemvn)
- Email: liem@vonguyen.io

---

*FinOps Platform demonstrates cloud financial management expertise for Staff/Principal Cloud Architect roles.*

