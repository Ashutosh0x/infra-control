// Package graph implements the infrastructure dependency graph.
// It constructs a directed acyclic graph (DAG) of all infrastructure resources
// and their relationships, enabling blast radius calculation and impact analysis.
package graph

import (
	"context"
	"fmt"
	"sync"

	"github.com/ashutosh0x/infra-control/pkg/types"
	"go.uber.org/zap"
)

// Graph is the infrastructure dependency graph. It models resources as nodes
// and their relationships as directed edges, enabling traversal, querying,
// and blast radius calculations.
type Graph struct {
	mu     sync.RWMutex
	nodes  map[string]*types.Node
	edges  map[string]*types.Edge
	adj    map[string][]string // node ID -> outgoing edge IDs
	radj   map[string][]string // node ID -> incoming edge IDs
	logger *zap.Logger
}

// New creates a new empty infrastructure graph.
func New(logger *zap.Logger) *Graph {
	return &Graph{
		nodes:  make(map[string]*types.Node),
		edges:  make(map[string]*types.Edge),
		adj:    make(map[string][]string),
		radj:   make(map[string][]string),
		logger: logger,
	}
}

// AddNode adds a node to the graph. If a node with the same ID exists, it is updated.
func (g *Graph) AddNode(node *types.Node) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes[node.ID] = node
	if _, exists := g.adj[node.ID]; !exists {
		g.adj[node.ID] = []string{}
	}
	if _, exists := g.radj[node.ID]; !exists {
		g.radj[node.ID] = []string{}
	}
}

// AddEdge adds a directed edge between two nodes. Both nodes must already exist.
func (g *Graph) AddEdge(edge *types.Edge) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[edge.Source]; !exists {
		return fmt.Errorf("source node %s does not exist", edge.Source)
	}
	if _, exists := g.nodes[edge.Target]; !exists {
		return fmt.Errorf("target node %s does not exist", edge.Target)
	}

	g.edges[edge.ID] = edge
	g.adj[edge.Source] = append(g.adj[edge.Source], edge.ID)
	g.radj[edge.Target] = append(g.radj[edge.Target], edge.ID)
	return nil
}

// GetNode returns a node by its ID.
func (g *Graph) GetNode(id string) (*types.Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	node, exists := g.nodes[id]
	return node, exists
}

// GetEdge returns an edge by its ID.
func (g *Graph) GetEdge(id string) (*types.Edge, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	edge, exists := g.edges[id]
	return edge, exists
}

// Neighbors returns all nodes directly connected to the given node.
func (g *Graph) Neighbors(nodeID string, direction string) []*types.Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var neighbors []*types.Node
	seen := make(map[string]bool)

	switch direction {
	case "downstream", "both":
		for _, edgeID := range g.adj[nodeID] {
			edge := g.edges[edgeID]
			if !seen[edge.Target] {
				seen[edge.Target] = true
				if node, exists := g.nodes[edge.Target]; exists {
					neighbors = append(neighbors, node)
				}
			}
		}
	}

	switch direction {
	case "upstream", "both":
		for _, edgeID := range g.radj[nodeID] {
			edge := g.edges[edgeID]
			if !seen[edge.Source] {
				seen[edge.Source] = true
				if node, exists := g.nodes[edge.Source]; exists {
					neighbors = append(neighbors, node)
				}
			}
		}
	}

	return neighbors
}

// BlastRadius calculates the blast radius for a given node — the set of all
// resources that could be affected if this node changes. Uses BFS traversal
// with weighted depth tracking.
func (g *Graph) BlastRadius(ctx context.Context, nodeID string, maxDepth int) (*types.BlastRadius, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[nodeID]; !exists {
		return nil, fmt.Errorf("node %s does not exist in graph", nodeID)
	}

	if maxDepth <= 0 {
		maxDepth = 10
	}

	result := &types.BlastRadius{
		OriginNodeID:  nodeID,
		AffectedNodes: []types.AffectedNode{},
	}

	// BFS traversal
	type queueItem struct {
		nodeID   string
		depth    int
		indirect bool
	}

	visited := make(map[string]bool)
	visited[nodeID] = true
	queue := []queueItem{}

	// Seed with direct downstream dependencies
	for _, edgeID := range g.adj[nodeID] {
		edge := g.edges[edgeID]
		if !visited[edge.Target] {
			queue = append(queue, queueItem{edge.Target, 1, false})
			visited[edge.Target] = true
		}
	}

	maxFoundDepth := 0

	for len(queue) > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		item := queue[0]
		queue = queue[1:]

		if item.depth > maxDepth {
			continue
		}

		if item.depth > maxFoundDepth {
			maxFoundDepth = item.depth
		}

		node := g.nodes[item.nodeID]
		if node == nil {
			continue
		}

		impactType := "direct"
		probability := 1.0
		if item.depth > 1 || item.indirect {
			impactType = "transitive"
			probability = 1.0 / float64(item.depth) // Decreasing probability with depth
		}

		result.AffectedNodes = append(result.AffectedNodes, types.AffectedNode{
			NodeID:      item.nodeID,
			Label:       node.Label,
			Distance:    item.depth,
			ImpactType:  impactType,
			Probability: probability,
		})

		// Continue BFS to downstream nodes
		for _, edgeID := range g.adj[item.nodeID] {
			edge := g.edges[edgeID]
			if !visited[edge.Target] {
				visited[edge.Target] = true
				queue = append(queue, queueItem{edge.Target, item.depth + 1, true})
			}
		}
	}

	result.TotalAffected = len(result.AffectedNodes)
	result.MaxDepth = maxFoundDepth

	// Calculate composite risk score based on affected nodes
	result.RiskScore = g.calculateBlastRiskScore(result)

	g.logger.Info("calculated blast radius",
		zap.String("origin", nodeID),
		zap.Int("affected", result.TotalAffected),
		zap.Int("max_depth", result.MaxDepth),
		zap.Float64("risk_score", result.RiskScore),
	)

	return result, nil
}

// calculateBlastRiskScore computes a risk score (0-100) based on the blast radius.
func (g *Graph) calculateBlastRiskScore(br *types.BlastRadius) float64 {
	if br.TotalAffected == 0 {
		return 0
	}

	// Score based on total affected nodes (logarithmic scale)
	score := float64(br.TotalAffected) * 5.0
	if score > 100 {
		score = 100
	}

	// Boost for deep dependency chains
	depthBoost := float64(br.MaxDepth) * 3.0
	score += depthBoost
	if score > 100 {
		score = 100
	}

	return score
}

// TopologicalSort returns nodes in topological order for safe execution.
func (g *Graph) TopologicalSort() ([]*types.Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	inDegree := make(map[string]int)
	for id := range g.nodes {
		inDegree[id] = 0
	}
	for _, edgeIDs := range g.adj {
		for _, edgeID := range edgeIDs {
			edge := g.edges[edgeID]
			inDegree[edge.Target]++
		}
	}

	// Kahn's algorithm
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var sorted []*types.Node
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		sorted = append(sorted, g.nodes[nodeID])

		for _, edgeID := range g.adj[nodeID] {
			edge := g.edges[edgeID]
			inDegree[edge.Target]--
			if inDegree[edge.Target] == 0 {
				queue = append(queue, edge.Target)
			}
		}
	}

	if len(sorted) != len(g.nodes) {
		return nil, fmt.Errorf("cycle detected in dependency graph")
	}

	return sorted, nil
}

// Query traverses the graph from a starting node based on query parameters.
func (g *Graph) Query(ctx context.Context, query types.GraphQuery) ([]*types.Node, []*types.Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[query.StartNodeID]; !exists {
		return nil, nil, fmt.Errorf("start node %s does not exist", query.StartNodeID)
	}

	visited := make(map[string]bool)
	visited[query.StartNodeID] = true

	var resultNodes []*types.Node
	var resultEdges []*types.Edge

	g.traverseQuery(query.StartNodeID, 0, query, visited, &resultNodes, &resultEdges)

	return resultNodes, resultEdges, nil
}

// traverseQuery recursively traverses the graph for a query.
func (g *Graph) traverseQuery(nodeID string, depth int, query types.GraphQuery, visited map[string]bool, nodes *[]*types.Node, edges *[]*types.Edge) {
	if depth >= query.MaxDepth {
		return
	}

	// Traverse downstream edges
	if query.Direction == "downstream" || query.Direction == "both" {
		for _, edgeID := range g.adj[nodeID] {
			edge := g.edges[edgeID]
			if !matchesEdgeTypes(edge.Type, query.EdgeTypes) {
				continue
			}
			*edges = append(*edges, edge)
			if !visited[edge.Target] {
				visited[edge.Target] = true
				node := g.nodes[edge.Target]
				if matchesNodeTypes(node.Type, query.NodeTypes) {
					*nodes = append(*nodes, node)
				}
				g.traverseQuery(edge.Target, depth+1, query, visited, nodes, edges)
			}
		}
	}

	// Traverse upstream edges
	if query.Direction == "upstream" || query.Direction == "both" {
		for _, edgeID := range g.radj[nodeID] {
			edge := g.edges[edgeID]
			if !matchesEdgeTypes(edge.Type, query.EdgeTypes) {
				continue
			}
			*edges = append(*edges, edge)
			if !visited[edge.Source] {
				visited[edge.Source] = true
				node := g.nodes[edge.Source]
				if matchesNodeTypes(node.Type, query.NodeTypes) {
					*nodes = append(*nodes, node)
				}
				g.traverseQuery(edge.Source, depth+1, query, visited, nodes, edges)
			}
		}
	}
}

// Stats returns graph statistics.
func (g *Graph) Stats() types.GraphStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodesByType := make(map[types.NodeType]int)
	for _, node := range g.nodes {
		nodesByType[node.Type]++
	}

	edgesByType := make(map[types.EdgeType]int)
	for _, edge := range g.edges {
		edgesByType[edge.Type]++
	}

	return types.GraphStats{
		TotalNodes:  len(g.nodes),
		TotalEdges:  len(g.edges),
		NodesByType: nodesByType,
		EdgesByType: edgesByType,
	}
}

// matchesEdgeTypes checks if an edge type is in the allowed list (empty = all allowed).
func matchesEdgeTypes(edgeType types.EdgeType, allowed []types.EdgeType) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, t := range allowed {
		if t == edgeType {
			return true
		}
	}
	return false
}

// matchesNodeTypes checks if a node type is in the allowed list (empty = all allowed).
func matchesNodeTypes(nodeType types.NodeType, allowed []types.NodeType) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, t := range allowed {
		if t == nodeType {
			return true
		}
	}
	return false
}
