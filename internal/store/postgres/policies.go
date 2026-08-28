package postgres

import (
	"context"
	"fmt"

	"github.com/ashutosh0x/infra-control/pkg/types"
	"go.uber.org/zap"
)

// PolicyStore provides data access for policies.
type PolicyStore struct {
	db     *DB
	logger *zap.Logger
}

// NewPolicyStore creates a new PolicyStore.
func NewPolicyStore(db *DB, logger *zap.Logger) *PolicyStore {
	return &PolicyStore{
		db:     db,
		logger: logger,
	}
}

// Create inserts a new policy.
func (s *PolicyStore) Create(_ context.Context, _ *types.Policy) error {
	return fmt.Errorf("not implemented")
}

// Get returns the policy.
func (s *PolicyStore) Get(_ context.Context, _ string) (*types.Policy, error) {
	return nil, fmt.Errorf("not implemented")
}

// List returns policy.
func (s *PolicyStore) List(_ context.Context) ([]*types.Policy, error) {
	return nil, fmt.Errorf("not implemented")
}

// Update rewrites the policy.
func (s *PolicyStore) Update(_ context.Context, _ *types.Policy) error {
	return fmt.Errorf("not implemented")
}

// Delete removes the policy.
func (s *PolicyStore) Delete(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}

// GetByType returns policies of a given type policy.
func (s *PolicyStore) GetByType(_ context.Context, _ types.PolicyType) ([]*types.Policy, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetEnabled returns the enabled policies policy.
func (s *PolicyStore) GetEnabled(_ context.Context) ([]*types.Policy, error) {
	return nil, fmt.Errorf("not implemented")
}
