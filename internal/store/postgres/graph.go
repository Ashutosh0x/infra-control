package postgres

import (
	"context"
	"fmt"

	"github.com/ashutosh0x/infra-control/pkg/types"
	"go.uber.org/zap"
)

// GraphStore provides data access for graph nodes and edges.
type GraphStore struct {
	db     *DB
	logger *zap.Logger
}

// NewGraphStore creates a new GraphStore.
func NewGraphStore(db *DB, logger *zap.Logger) *GraphStore {
	return &GraphStore{
		db:     db,
		logger: logger,
	}
}

func (s *GraphStore) UpsertNode(ctx context.Context, node *types.Node) error {
	return fmt.Errorf("not implemented")
}

func (s *GraphStore) UpsertEdge(ctx context.Context, edge *types.Edge) error {
	return fmt.Errorf("not implemented")
}

func (s *GraphStore) GetNode(ctx context.Context, id string) (*types.Node, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *GraphStore) GetEdges(ctx context.Context, nodeID string, direction string) ([]*types.Edge, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *GraphStore) GetNeighbors(ctx context.Context, nodeID string, maxDepth int) ([]*types.Node, []*types.Edge, error) {
	return nil, nil, fmt.Errorf("not implemented")
}

func (s *GraphStore) DeleteNode(ctx context.Context, id string) error {
	return fmt.Errorf("not implemented")
}

func (s *GraphStore) DeleteEdge(ctx context.Context, id string) error {
	return fmt.Errorf("not implemented")
}

func (s *GraphStore) GetStats(ctx context.Context) (*types.GraphStats, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *GraphStore) BulkUpsertNodes(ctx context.Context, nodes []*types.Node) error {
	return fmt.Errorf("not implemented")
}

func (s *GraphStore) BulkUpsertEdges(ctx context.Context, edges []*types.Edge) error {
	return fmt.Errorf("not implemented")
}
