// Package cloud provides a unified interface for discovering and managing
// resources across multiple cloud providers (AWS, GCP, Azure, Kubernetes).
package cloud

import (
	"context"
	"fmt"
	"sync"

	"github.com/ashutosh0x/infra-control/pkg/types"
	"go.uber.org/zap"
)

// Provider defines the interface that all cloud provider implementations must satisfy.
// Each provider is responsible for discovering, reading, and watching resources
// in its respective cloud environment.
type Provider interface {
	// Name returns the provider identifier (e.g., "aws", "gcp", "azure", "kubernetes").
	Name() types.CloudProvider

	// Discover performs a full resource discovery scan and returns all discovered resources.
	Discover(ctx context.Context, filter types.ResourceFilter) ([]*types.Resource, error)

	// Get retrieves a single resource by its external ID.
	Get(ctx context.Context, externalID string) (*types.Resource, error)

	// List returns resources matching the given filter with pagination support.
	List(ctx context.Context, filter types.ResourceFilter) ([]*types.Resource, int, error)

	// Watch starts a real-time resource change stream. Changes are sent to the returned channel.
	// The caller must cancel the context to stop watching.
	Watch(ctx context.Context) (<-chan ResourceEvent, error)

	// GetAuditEvents retrieves audit events (CloudTrail, Activity Log, etc.) for attribution.
	GetAuditEvents(ctx context.Context, resourceID string, opts AuditEventOptions) ([]AuditEvent, error)

	// Validate checks if the provider configuration and credentials are valid.
	Validate(ctx context.Context) error

	// Close releases any resources held by the provider.
	Close() error
}

// ResourceEventType represents the type of resource change event.
type ResourceEventType string

const (
	ResourceEventCreated ResourceEventType = "created"
	ResourceEventUpdated ResourceEventType = "updated"
	ResourceEventDeleted ResourceEventType = "deleted"
)

// ResourceEvent represents a real-time resource change event from a cloud provider.
type ResourceEvent struct {
	Type     ResourceEventType `json:"type"`
	Resource *types.Resource   `json:"resource"`
	Previous *types.Resource   `json:"previous,omitempty"`
}

// AuditEventOptions configures audit event retrieval.
type AuditEventOptions struct {
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
	MaxEvents int    `json:"max_events,omitempty"`
}

// AuditEvent represents a cloud provider audit event for drift attribution.
type AuditEvent struct {
	EventID     string         `json:"event_id"`
	EventTime   string         `json:"event_time"`
	EventSource string         `json:"event_source"`
	EventName   string         `json:"event_name"`
	Principal   string         `json:"principal"`
	SourceIP    string         `json:"source_ip"`
	UserAgent   string         `json:"user_agent"`
	ResourceID  string         `json:"resource_id"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	RawEvent    string         `json:"raw_event,omitempty"`
}

// Registry manages the registration and retrieval of cloud providers.
type Registry struct {
	mu        sync.RWMutex
	providers map[types.CloudProvider]Provider
	logger    *zap.Logger
}

// NewRegistry creates a new provider registry.
func NewRegistry(logger *zap.Logger) *Registry {
	return &Registry{
		providers: make(map[types.CloudProvider]Provider),
		logger:    logger,
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(provider Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := provider.Name()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %s is already registered", name)
	}

	r.providers[name] = provider
	r.logger.Info("registered cloud provider", zap.String("provider", string(name)))
	return nil
}

// Get retrieves a registered provider by name.
func (r *Registry) Get(name types.CloudProvider) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s is not registered", name)
	}
	return provider, nil
}

// List returns all registered provider names.
func (r *Registry) List() []types.CloudProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]types.CloudProvider, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// DiscoverAll runs discovery across all registered providers concurrently.
func (r *Registry) DiscoverAll(ctx context.Context, filter types.ResourceFilter) ([]*types.Resource, error) {
	r.mu.RLock()
	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	r.mu.RUnlock()

	type result struct {
		resources []*types.Resource
		err       error
		provider  types.CloudProvider
	}

	results := make(chan result, len(providers))
	var wg sync.WaitGroup

	for _, p := range providers {
		wg.Add(1)
		go func(provider Provider) {
			defer wg.Done()
			resources, err := provider.Discover(ctx, filter)
			results <- result{
				resources: resources,
				err:       err,
				provider:  provider.Name(),
			}
		}(p)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allResources []*types.Resource
	var errs []error

	for res := range results {
		if res.err != nil {
			r.logger.Error("discovery failed for provider",
				zap.String("provider", string(res.provider)),
				zap.Error(res.err),
			)
			errs = append(errs, fmt.Errorf("provider %s: %w", res.provider, res.err))
			continue
		}
		r.logger.Info("discovery completed",
			zap.String("provider", string(res.provider)),
			zap.Int("resources", len(res.resources)),
		)
		allResources = append(allResources, res.resources...)
	}

	if len(errs) > 0 && len(allResources) == 0 {
		return nil, fmt.Errorf("all providers failed: %v", errs)
	}

	return allResources, nil
}

// Close shuts down all registered providers.
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for name, provider := range r.providers {
		if err := provider.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing provider %s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing providers: %v", errs)
	}
	return nil
}
