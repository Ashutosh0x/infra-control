package postgres

import (
	"context"
	"fmt"

	"github.com/ashutosh0x/infra-control/pkg/types"
	"go.uber.org/zap"
)

// ResourceStore provides resource data access operations.
type ResourceStore struct {
	db     *DB
	logger *zap.Logger
}

// NewResourceStore creates a new ResourceStore.
func NewResourceStore(db *DB, logger *zap.Logger) *ResourceStore {
	return &ResourceStore{
		db:     db,
		logger: logger,
	}
}

// Create inserts a new resource.
func (s *ResourceStore) Create(ctx context.Context, resource *types.Resource) error {
	q := `INSERT INTO resources (id, external_id, name, type, provider, region, account, state, tags, configuration, terraform_state, metadata) 
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	_, err := s.db.Pool.Exec(ctx, q, resource.ID, resource.ExternalID, resource.Name, resource.Type, resource.Provider, resource.Region, resource.Account, resource.State, resource.Tags, resource.Configuration, resource.TerraformState, resource.Metadata)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}
	return nil
}

// Get returns the resource.
func (s *ResourceStore) Get(ctx context.Context, id string) (*types.Resource, error) {
	q := `SELECT id, external_id, name, type, provider, region, account, state, tags, configuration, terraform_state, metadata, created_at, updated_at, discovered_at FROM resources WHERE id = $1`
	var r types.Resource
	err := s.db.Pool.QueryRow(ctx, q, id).Scan(&r.ID, &r.ExternalID, &r.Name, &r.Type, &r.Provider, &r.Region, &r.Account, &r.State, &r.Tags, &r.Configuration, &r.TerraformState, &r.Metadata, &r.CreatedAt, &r.UpdatedAt, &r.DiscoveredAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}
	return &r, nil
}

// GetByExternalID returns the resource.
func (s *ResourceStore) GetByExternalID(ctx context.Context, externalID string) (*types.Resource, error) {
	q := `SELECT id, external_id, name, type, provider, region, account, state, tags, configuration, terraform_state, metadata, created_at, updated_at, discovered_at FROM resources WHERE external_id = $1`
	var r types.Resource
	err := s.db.Pool.QueryRow(ctx, q, externalID).Scan(&r.ID, &r.ExternalID, &r.Name, &r.Type, &r.Provider, &r.Region, &r.Account, &r.State, &r.Tags, &r.Configuration, &r.TerraformState, &r.Metadata, &r.CreatedAt, &r.UpdatedAt, &r.DiscoveredAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource by external ID: %w", err)
	}
	return &r, nil
}

// List returns resource.
func (s *ResourceStore) List(_ context.Context, _ types.ResourceFilter) ([]*types.Resource, int, error) {
	// Dummy implementation for list
	return nil, 0, fmt.Errorf("not implemented")
}

// Update rewrites the resource.
func (s *ResourceStore) Update(ctx context.Context, resource *types.Resource) error {
	q := `UPDATE resources SET name=$1, type=$2, provider=$3, region=$4, account=$5, state=$6, tags=$7, configuration=$8, terraform_state=$9, metadata=$10, updated_at=NOW() WHERE id=$11`
	_, err := s.db.Pool.Exec(ctx, q, resource.Name, resource.Type, resource.Provider, resource.Region, resource.Account, resource.State, resource.Tags, resource.Configuration, resource.TerraformState, resource.Metadata, resource.ID)
	if err != nil {
		return fmt.Errorf("failed to update resource: %w", err)
	}
	return nil
}

// Delete removes the resource.
func (s *ResourceStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.Pool.Exec(ctx, `DELETE FROM resources WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete resource: %w", err)
	}
	return nil
}

// Upsert inserts or updates the resource.
func (s *ResourceStore) Upsert(_ context.Context, _ *types.Resource) error {
	return fmt.Errorf("not implemented")
}

// BulkUpsert inserts or updates many resource.
func (s *ResourceStore) BulkUpsert(_ context.Context, _ []*types.Resource) error {
	return fmt.Errorf("not implemented")
}

// Count returns the number of resource.
func (s *ResourceStore) Count(_ context.Context, _ types.ResourceFilter) (int, error) {
	return 0, fmt.Errorf("not implemented")
}
