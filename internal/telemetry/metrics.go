package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds the registered prometheus collectors.
type Metrics struct {
	ResourcesDiscoveredTotal *prometheus.CounterVec
	DriftEventsTotal         *prometheus.CounterVec
	PolicyEvaluationsTotal   *prometheus.CounterVec
	RemediationActionsTotal  *prometheus.CounterVec
	APIRequestDuration       *prometheus.HistogramVec
	GraphNodesTotal          prometheus.Gauge
	RiskScoreCurrent         *prometheus.GaugeVec
}

// NewMetrics initializes and returns a new Metrics struct with all collectors.
func NewMetrics() *Metrics {
	return &Metrics{
		ResourcesDiscoveredTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "infra_resources_discovered_total",
				Help: "Total number of resources discovered by provider",
			},
			[]string{"provider"},
		),
		DriftEventsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "infra_drift_events_total",
				Help: "Total number of drift events detected by severity",
			},
			[]string{"severity"},
		),
		PolicyEvaluationsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "infra_policy_evaluations_total",
				Help: "Total number of policy evaluations by result",
			},
			[]string{"result"}, // e.g. pass, fail, error
		),
		RemediationActionsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "infra_remediation_actions_total",
				Help: "Total number of remediation actions performed by status",
			},
			[]string{"status"}, // e.g. success, failed, skipped
		),
		APIRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "infra_api_request_duration_seconds",
				Help:    "Histogram of API request durations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint", "status_code"},
		),
		GraphNodesTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "infra_graph_nodes_total",
				Help: "Total number of nodes currently in the infrastructure graph",
			},
		),
		RiskScoreCurrent: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "infra_risk_score_current",
				Help: "Current risk score by resource ID",
			},
			[]string{"resource_id"},
		),
	}
}

// RegisterMetrics registers all metrics with the provided prometheus registry.
func (m *Metrics) RegisterMetrics(registry *prometheus.Registry) {
	registry.MustRegister(m.ResourcesDiscoveredTotal)
	registry.MustRegister(m.DriftEventsTotal)
	registry.MustRegister(m.PolicyEvaluationsTotal)
	registry.MustRegister(m.RemediationActionsTotal)
	registry.MustRegister(m.APIRequestDuration)
	registry.MustRegister(m.GraphNodesTotal)
	registry.MustRegister(m.RiskScoreCurrent)
}
