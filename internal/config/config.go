// Package config provides configuration management for FinOps Aggregator
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration
type Config struct {
	AWS      AWSConfig      `yaml:"aws"`
	Azure    AzureConfig    `yaml:"azure"`
	GCP      GCPConfig      `yaml:"gcp"`
	Budgets  []Budget       `yaml:"budgets"`
	Anomaly  AnomalyConfig  `yaml:"anomaly"`
	Alerting AlertingConfig `yaml:"alerting"`
	Reporter ReporterConfig `yaml:"reporter"`
}

// AWSConfig holds AWS-specific configuration
type AWSConfig struct {
	Enabled     bool     `yaml:"enabled"`
	RoleARN     string   `yaml:"role_arn"`
	Region      string   `yaml:"region"`
	AccountIDs  []string `yaml:"account_ids"`
	Granularity string   `yaml:"granularity"` // DAILY, MONTHLY
	GroupBy     []string `yaml:"group_by"`    // SERVICE, LINKED_ACCOUNT, etc.
}

// AzureConfig holds Azure-specific configuration
type AzureConfig struct {
	Enabled         bool     `yaml:"enabled"`
	TenantID        string   `yaml:"tenant_id"`
	SubscriptionIDs []string `yaml:"subscription_ids"`
	UseMSI          bool     `yaml:"use_msi"`
	Granularity     string   `yaml:"granularity"`
}

// GCPConfig holds GCP-specific configuration
type GCPConfig struct {
	Enabled        bool   `yaml:"enabled"`
	BillingAccount string `yaml:"billing_account"`
	ProjectID      string `yaml:"project_id"`
	WIFConfigPath  string `yaml:"wif_config_path"`
}

// Budget defines a budget threshold
type Budget struct {
	Name          string  `yaml:"name"`
	Provider      string  `yaml:"provider"` // aws, azure, gcp, or all
	Scope         string  `yaml:"scope"`    // account ID, subscription, project
	MonthlyLimit  float64 `yaml:"monthly_limit"`
	AlertAt       []int   `yaml:"alert_at"` // percentages to alert at (e.g., 50, 75, 90, 100)
	NotifyEmails  []string `yaml:"notify_emails"`
	NotifySlack   string  `yaml:"notify_slack"`
}

// AnomalyConfig configures anomaly detection
type AnomalyConfig struct {
	Enabled               bool    `yaml:"enabled"`
	LookbackDays          int     `yaml:"lookback_days"`
	DeviationThreshold    float64 `yaml:"deviation_threshold"`    // percentage (e.g., 25 = 25%)
	MinimumCostThreshold  float64 `yaml:"minimum_cost_threshold"` // ignore services below this
}

// AlertingConfig configures alerting channels
type AlertingConfig struct {
	Email EmailConfig `yaml:"email"`
	Slack SlackConfig `yaml:"slack"`
}

// EmailConfig configures email alerting
type EmailConfig struct {
	Enabled    bool     `yaml:"enabled"`
	SMTPHost   string   `yaml:"smtp_host"`
	SMTPPort   int      `yaml:"smtp_port"`
	FromAddr   string   `yaml:"from_addr"`
	Recipients []string `yaml:"recipients"`
	// Or use Microsoft Graph
	UseMSGraph bool   `yaml:"use_ms_graph"`
	TenantID   string `yaml:"ms_tenant_id"`
	ClientID   string `yaml:"ms_client_id"`
}

// SlackConfig configures Slack alerting
type SlackConfig struct {
	Enabled    bool   `yaml:"enabled"`
	WebhookURL string `yaml:"webhook_url"`
	Channel    string `yaml:"channel"`
}

// ReporterConfig configures report generation
type ReporterConfig struct {
	OutputDir   string `yaml:"output_dir"`
	HTMLTemplate string `yaml:"html_template"`
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	data = []byte(os.ExpandEnv(string(data)))

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if cfg.Anomaly.LookbackDays == 0 {
		cfg.Anomaly.LookbackDays = 30
	}
	if cfg.Anomaly.DeviationThreshold == 0 {
		cfg.Anomaly.DeviationThreshold = 25
	}
	if cfg.Reporter.OutputDir == "" {
		cfg.Reporter.OutputDir = "./reports"
	}

	return &cfg, nil
}

