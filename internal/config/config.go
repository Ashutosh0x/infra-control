// Package config defines the configuration schema and its defaults.
package config

import "time"

// Config is the root configuration for infra-control.
type Config struct {
	Server      ServerConfig      `yaml:"server"      mapstructure:"server"`
	Database    DatabaseConfig    `yaml:"database"    mapstructure:"database"`
	Cache       CacheConfig       `yaml:"cache"       mapstructure:"cache"`
	Events      EventsConfig      `yaml:"events"      mapstructure:"events"`
	Cloud       CloudConfig       `yaml:"cloud"       mapstructure:"cloud"`
	Terraform   TerraformConfig   `yaml:"terraform"   mapstructure:"terraform"`
	Drift       DriftConfig       `yaml:"drift"       mapstructure:"drift"`
	Policy      PolicyConfig      `yaml:"policy"      mapstructure:"policy"`
	AI          AIConfig          `yaml:"ai"          mapstructure:"ai"`
	Remediation RemediationConfig `yaml:"remediation" mapstructure:"remediation"`
	Audit       AuditConfig       `yaml:"audit"       mapstructure:"audit"`
	Telemetry   TelemetryConfig   `yaml:"telemetry"   mapstructure:"telemetry"`
	MCP         MCPConfig         `yaml:"mcp"         mapstructure:"mcp"`
}

// ServerConfig configures HTTP and GRPC servers.
type ServerConfig struct {
	HTTPPort            int           `yaml:"http_port"           mapstructure:"http_port"`
	GRPCPort            int           `yaml:"grpc_port"           mapstructure:"grpc_port"`
	Host                string        `yaml:"host"                mapstructure:"host"`
	TLSCert             string        `yaml:"tls_cert"            mapstructure:"tls_cert"`
	TLSKey              string        `yaml:"tls_key"             mapstructure:"tls_key"`
	ReadTimeout         time.Duration `yaml:"read_timeout"        mapstructure:"read_timeout"`
	WriteTimeout        time.Duration `yaml:"write_timeout"       mapstructure:"write_timeout"`
	ShutdownGracePeriod time.Duration `yaml:"shutdown_grace_period" mapstructure:"shutdown_grace_period"`
}

// DatabaseConfig configures PostgreSQL/database connection.
type DatabaseConfig struct {
	Host            string        `yaml:"host"              mapstructure:"host"`
	Port            int           `yaml:"port"              mapstructure:"port"`
	User            string        `yaml:"user"              mapstructure:"user"`
	Password        string        `yaml:"password"          mapstructure:"password"`
	DBName          string        `yaml:"dbname"            mapstructure:"dbname"`
	SSLMode         string        `yaml:"sslmode"           mapstructure:"sslmode"`
	MaxOpenConns    int           `yaml:"max_open_conns"    mapstructure:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"    mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" mapstructure:"conn_max_lifetime"`
}

// CacheConfig configures Redis/caching setup.
type CacheConfig struct {
	Host         string `yaml:"host"           mapstructure:"host"`
	Port         int    `yaml:"port"           mapstructure:"port"`
	Password     string `yaml:"password"       mapstructure:"password"`
	DB           int    `yaml:"db"             mapstructure:"db"`
	PoolSize     int    `yaml:"pool_size"      mapstructure:"pool_size"`
	MinIdleConns int    `yaml:"min_idle_conns" mapstructure:"min_idle_conns"`
}

// EventsConfig configures NATS/event bus.
type EventsConfig struct {
	URL           string        `yaml:"url"             mapstructure:"url"`
	ClusterID     string        `yaml:"cluster_id"      mapstructure:"cluster_id"`
	ClientID      string        `yaml:"client_id"       mapstructure:"client_id"`
	MaxReconnects int           `yaml:"max_reconnects"  mapstructure:"max_reconnects"`
	ReconnectWait time.Duration `yaml:"reconnect_wait"  mapstructure:"reconnect_wait"`
}

// CloudConfig configures multicloud integrations.
type CloudConfig struct {
	AWS        AWSConfig        `yaml:"aws"        mapstructure:"aws"`
	GCP        GCPConfig        `yaml:"gcp"        mapstructure:"gcp"`
	Azure      AzureConfig      `yaml:"azure"      mapstructure:"azure"`
	Kubernetes KubernetesConfig `yaml:"kubernetes" mapstructure:"kubernetes"`
}

// AWSConfig holds the AWS account, region, and credential settings.
type AWSConfig struct {
	Enabled       bool     `yaml:"enabled"         mapstructure:"enabled"`
	Region        string   `yaml:"region"          mapstructure:"region"`
	Profile       string   `yaml:"profile"         mapstructure:"profile"`
	AssumeRoleARN string   `yaml:"assume_role_arn" mapstructure:"assume_role_arn"`
	Regions       []string `yaml:"regions"         mapstructure:"regions"`
}

// GCPConfig holds the GCP project, region, and credential settings.
type GCPConfig struct {
	Enabled   bool     `yaml:"enabled"    mapstructure:"enabled"`
	ProjectID string   `yaml:"project_id" mapstructure:"project_id"`
	Projects  []string `yaml:"projects"   mapstructure:"projects"`
}

// AzureConfig holds the Azure subscription, tenant, and credential settings.
type AzureConfig struct {
	Enabled        bool     `yaml:"enabled"         mapstructure:"enabled"`
	SubscriptionID string   `yaml:"subscription_id" mapstructure:"subscription_id"`
	TenantID       string   `yaml:"tenant_id"       mapstructure:"tenant_id"`
	Subscriptions  []string `yaml:"subscriptions"   mapstructure:"subscriptions"`
}

// KubernetesConfig holds the kubeconfig path, context, and namespace.
type KubernetesConfig struct {
	Enabled    bool     `yaml:"enabled"    mapstructure:"enabled"`
	Kubeconfig string   `yaml:"kubeconfig" mapstructure:"kubeconfig"`
	Contexts   []string `yaml:"contexts"   mapstructure:"contexts"`
	InCluster  bool     `yaml:"in_cluster" mapstructure:"in_cluster"`
}

// TerraformConfig configures Terraform CLI integration.
type TerraformConfig struct {
	BinaryPath   string        `yaml:"binary_path"   mapstructure:"binary_path"`
	WorkspaceDir string        `yaml:"workspace_dir" mapstructure:"workspace_dir"`
	Parallelism  int           `yaml:"parallelism"   mapstructure:"parallelism"`
	PlanTimeout  time.Duration `yaml:"plan_timeout"  mapstructure:"plan_timeout"`
	ApplyTimeout time.Duration `yaml:"apply_timeout" mapstructure:"apply_timeout"`
	StateBackend string        `yaml:"state_backend" mapstructure:"state_backend"`
}

// DriftConfig configures drift detection scheduling and actions.
type DriftConfig struct {
	ScanInterval         time.Duration `yaml:"scan_interval"           mapstructure:"scan_interval"`
	EnableRealtime       bool          `yaml:"enable_realtime"         mapstructure:"enable_realtime"`
	SeverityThresholds   []string      `yaml:"severity_thresholds"     mapstructure:"severity_thresholds"`
	AutoRemediate        bool          `yaml:"auto_remediate"          mapstructure:"auto_remediate"`
	AutoRemediateMaxRisk string        `yaml:"auto_remediate_max_risk" mapstructure:"auto_remediate_max_risk"`
}

// PolicyConfig configures OPA or similar policy engines.
type PolicyConfig struct {
	BundlePath        string `yaml:"bundle_path"         mapstructure:"bundle_path"`
	EnableBuiltin     bool   `yaml:"enable_builtin"      mapstructure:"enable_builtin"`
	CustomPoliciesDir string `yaml:"custom_policies_dir" mapstructure:"custom_policies_dir"`
}

// AIConfig configures LLM providers for remediation/analysis.
type AIConfig struct {
	Provider    string        `yaml:"provider"     mapstructure:"provider"` // openai, gemini, anthropic
	Model       string        `yaml:"model"        mapstructure:"model"`
	APIKey      string        `yaml:"api_key"      mapstructure:"api_key"`
	Temperature float32       `yaml:"temperature"  mapstructure:"temperature"`
	MaxTokens   int           `yaml:"max_tokens"   mapstructure:"max_tokens"`
	Timeout     time.Duration `yaml:"timeout"      mapstructure:"timeout"`
	MaxRetries  int           `yaml:"max_retries"  mapstructure:"max_retries"`
}

// RemediationConfig configures automated and manual remediation workflows.
type RemediationConfig struct {
	AutoApplyEnabled    bool     `yaml:"auto_apply_enabled"     mapstructure:"auto_apply_enabled"`
	MaxRiskForAutoApply string   `yaml:"max_risk_for_auto_apply" mapstructure:"max_risk_for_auto_apply"`
	PREnabled           bool     `yaml:"pr_enabled"             mapstructure:"pr_enabled"`
	PRBaseBranch        string   `yaml:"pr_base_branch"         mapstructure:"pr_base_branch"`
	PRLabels            []string `yaml:"pr_labels"              mapstructure:"pr_labels"`
	RequiredApprovers   int      `yaml:"required_approvers"     mapstructure:"required_approvers"`
}

// AuditConfig configures audit logging retention and storage.
type AuditConfig struct {
	RetentionDays  int    `yaml:"retention_days"  mapstructure:"retention_days"`
	ImmutableStore bool   `yaml:"immutable_store" mapstructure:"immutable_store"`
	ExportFormat   string `yaml:"export_format"   mapstructure:"export_format"` // json, csv
}

// TelemetryConfig configures observability (metrics, tracing, logging).
type TelemetryConfig struct {
	Metrics MetricsConfig `yaml:"metrics" mapstructure:"metrics"`
	Tracing TracingConfig `yaml:"tracing" mapstructure:"tracing"`
	Logging LoggingConfig `yaml:"logging" mapstructure:"logging"`
}

// MetricsConfig configures the Prometheus metrics endpoint.
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	Port    int    `yaml:"port"    mapstructure:"port"`
	Path    string `yaml:"path"    mapstructure:"path"`
}

// TracingConfig configures the OpenTelemetry trace exporter.
type TracingConfig struct {
	Enabled    bool    `yaml:"enabled"     mapstructure:"enabled"`
	Endpoint   string  `yaml:"endpoint"    mapstructure:"endpoint"`
	SampleRate float64 `yaml:"sample_rate" mapstructure:"sample_rate"`
}

// LoggingConfig configures the log level, format, and destination.
type LoggingConfig struct {
	Level       string   `yaml:"level"        mapstructure:"level"`
	Format      string   `yaml:"format"       mapstructure:"format"`
	OutputPaths []string `yaml:"output_paths" mapstructure:"output_paths"`
}

// MCPConfig configures Model Context Protocol features.
type MCPConfig struct {
	Enabled      bool     `yaml:"enabled"       mapstructure:"enabled"`
	Port         int      `yaml:"port"          mapstructure:"port"`
	AuthToken    string   `yaml:"auth_token"    mapstructure:"auth_token"`
	AllowedTools []string `yaml:"allowed_tools" mapstructure:"allowed_tools"`
}
