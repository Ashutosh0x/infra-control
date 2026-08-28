package routes

import (
	"net/http"

	"github.com/ashutosh0x/infra-control/internal/api/handlers"
)

// Handlers contains all the handlers needed for the API routes.
type Handlers struct {
	Health      *handlers.HealthHandler
	Drift       *handlers.DriftHandler
	Resources   *handlers.ResourcesHandler
	Policies    *handlers.PoliciesHandler
	Graph       *handlers.GraphHandler
	Audit       *handlers.AuditHandler
	Remediation *handlers.RemediationHandler
	Compliance  *handlers.ComplianceHandler
	Risk        *handlers.RiskHandler
}

// RegisterV1Routes sets up the API v1 routing.
func RegisterV1Routes(mux *http.ServeMux, h *Handlers) {
	// Health
	mux.HandleFunc("GET /healthz", h.Health.Liveness)
	mux.HandleFunc("GET /readyz", h.Health.Readiness)
	mux.HandleFunc("GET /version", h.Health.Version)

	// Drift
	mux.HandleFunc("GET /api/v1/drift", h.Drift.List)
	mux.HandleFunc("GET /api/v1/drift/{id}", h.Drift.Get)
	mux.HandleFunc("POST /api/v1/drift/scan", h.Drift.Scan)
	mux.HandleFunc("PUT /api/v1/drift/{id}/resolve", h.Drift.Resolve)

	// Resources
	mux.HandleFunc("GET /api/v1/resources", h.Resources.List)
	mux.HandleFunc("GET /api/v1/resources/{id}", h.Resources.Get)
	mux.HandleFunc("POST /api/v1/resources/discover", h.Resources.Discover)
	mux.HandleFunc("GET /api/v1/resources/{id}/risk", h.Resources.Risk)

	// Policies
	mux.HandleFunc("GET /api/v1/policies", h.Policies.List)
	mux.HandleFunc("POST /api/v1/policies", h.Policies.Create)
	mux.HandleFunc("GET /api/v1/policies/{id}", h.Policies.Get)
	mux.HandleFunc("PUT /api/v1/policies/{id}", h.Policies.Update)
	mux.HandleFunc("DELETE /api/v1/policies/{id}", h.Policies.Delete)
	mux.HandleFunc("POST /api/v1/policies/evaluate", h.Policies.Evaluate)

	// Graph
	mux.HandleFunc("GET /api/v1/graph", h.Graph.GetFull)
	mux.HandleFunc("GET /api/v1/graph/node/{id}", h.Graph.GetNode)
	mux.HandleFunc("GET /api/v1/graph/blast-radius/{id}", h.Graph.BlastRadius)
	mux.HandleFunc("POST /api/v1/graph/query", h.Graph.Query)

	// Audit
	mux.HandleFunc("GET /api/v1/audit", h.Audit.List)
	mux.HandleFunc("GET /api/v1/audit/{id}", h.Audit.Get)
	mux.HandleFunc("GET /api/v1/audit/export", h.Audit.Export)

	// Remediation
	mux.HandleFunc("GET /api/v1/remediations", h.Remediation.List)
	mux.HandleFunc("GET /api/v1/remediations/{id}", h.Remediation.Get)
	mux.HandleFunc("POST /api/v1/remediations/{id}/approve", h.Remediation.Approve)
	mux.HandleFunc("POST /api/v1/remediations/{id}/reject", h.Remediation.Reject)
	mux.HandleFunc("POST /api/v1/remediations/{id}/execute", h.Remediation.Execute)

	// Compliance
	mux.HandleFunc("GET /api/v1/compliance/frameworks", h.Compliance.Frameworks)
	mux.HandleFunc("GET /api/v1/compliance/{framework}", h.Compliance.Status)
	mux.HandleFunc("GET /api/v1/compliance/{framework}/report", h.Compliance.Report)

	// Risk
	mux.HandleFunc("GET /api/v1/risk/summary", h.Risk.Summary)
	mux.HandleFunc("GET /api/v1/risk/resources/{id}", h.Risk.ResourceRisk)
	mux.HandleFunc("GET /api/v1/risk/trends", h.Risk.Trends)
}
