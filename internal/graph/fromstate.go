package graph

import (
	"fmt"
	"strings"

	"github.com/ashutosh0x/infra-control/internal/terraform"
	"github.com/ashutosh0x/infra-control/pkg/types"
	"go.uber.org/zap"
)

// FromState builds a dependency graph from Terraform or OpenTofu state.
//
// Terraform records, for each resource instance, the addresses it depends on.
// Those recorded dependencies are authoritative: Terraform computed them from
// the configuration graph it used to apply, so they capture both explicit
// depends_on and the implicit edges created by attribute references.
//
// Edges point from the dependency to the dependent, meaning an edge A -> B
// reads "B depends on A". A downstream traversal from A therefore reaches
// everything that breaks when A changes, which is what blast radius needs.
func FromState(state *terraform.State, logger *zap.Logger) (*Graph, error) {
	if state == nil {
		return nil, fmt.Errorf("no state supplied")
	}

	g := New(logger)
	resources := state.ManagedResources()

	// Every resource becomes a node before any edge is added, because AddEdge
	// rejects an edge whose endpoints do not yet exist.
	for _, resource := range resources {
		g.AddNode(&types.Node{
			ID:         resource.Address,
			Type:       types.NodeTypeResource,
			ResourceID: resource.Address,
			Label:      resource.Address,
			Provider:   resource.Provider,
			Properties: map[string]any{
				"resource_type": resource.Type,
				"module":        resource.Module,
			},
		})
	}

	// Modules become container nodes so that a blast radius can be reported at
	// module granularity as well as per resource.
	modules := map[string]struct{}{}
	for _, resource := range resources {
		if resource.Module == "" {
			continue
		}
		if _, seen := modules[resource.Module]; seen {
			continue
		}
		modules[resource.Module] = struct{}{}
		g.AddNode(&types.Node{
			ID:    resource.Module,
			Type:  types.NodeTypeModule,
			Label: resource.Module,
		})
	}

	var skipped int
	for _, resource := range resources {
		if resource.Module != "" {
			if err := g.AddEdge(&types.Edge{
				ID:     resource.Module + " contains " + resource.Address,
				Source: resource.Module,
				Target: resource.Address,
				Type:   types.EdgeTypeContains,
				Weight: 1,
			}); err != nil {
				return nil, fmt.Errorf("link module %s: %w", resource.Module, err)
			}
		}

		for _, dependency := range resource.Dependencies {
			// State records dependencies at resource-block granularity, without
			// the count or for_each index. Resolve each to the concrete instance
			// addresses it refers to.
			targets := resolveDependency(dependency, resources)
			if len(targets) == 0 {
				// A dependency on a data source or a resource outside this state
				// file has no node to point at. It is skipped rather than
				// invented, so the graph never claims an edge that state does
				// not support.
				skipped++
				continue
			}

			for _, target := range targets {
				if target == resource.Address {
					continue
				}
				edge := &types.Edge{
					ID:     target + " -> " + resource.Address,
					Source: target,
					Target: resource.Address,
					Type:   types.EdgeTypeDependsOn,
					Weight: 1,
				}
				// A duplicate edge is not an error: two instances of the same
				// block can declare the same dependency.
				if _, exists := g.GetEdge(edge.ID); exists {
					continue
				}
				if err := g.AddEdge(edge); err != nil {
					return nil, fmt.Errorf("link %s to %s: %w", target, resource.Address, err)
				}
			}
		}
	}

	if logger != nil {
		stats := g.Stats()
		logger.Info("built dependency graph from state",
			zap.Int("nodes", stats.TotalNodes),
			zap.Int("edges", stats.TotalEdges),
			zap.Int("unresolved_dependencies", skipped),
		)
	}

	return g, nil
}

// resolveDependency maps a state dependency reference onto concrete instance
// addresses.
//
// A dependency is recorded as a resource block address such as
// `aws_subnet.private`. When that block uses count or for_each, the instances
// carry index suffixes, so an exact match finds nothing and every instance of
// the block must be returned instead.
func resolveDependency(dependency string, resources []terraform.StateResource) []string {
	var matches []string

	for _, resource := range resources {
		if resource.Address == dependency {
			return []string{resource.Address}
		}
		// An indexed instance of the referenced block.
		if strings.HasPrefix(resource.Address, dependency+"[") {
			matches = append(matches, resource.Address)
		}
	}

	return matches
}

// Roots returns the nodes nothing depends on, which are the entry points of
// the graph and usually the resources safest to change.
func (g *Graph) Roots() []*types.Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var roots []*types.Node
	for id, node := range g.nodes {
		if len(g.radj[id]) == 0 {
			roots = append(roots, node)
		}
	}
	return roots
}

// Leaves returns the nodes that depend on nothing.
func (g *Graph) Leaves() []*types.Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var leaves []*types.Node
	for id, node := range g.nodes {
		if len(g.adj[id]) == 0 {
			leaves = append(leaves, node)
		}
	}
	return leaves
}

// NodeIDs returns every node ID in the graph.
func (g *Graph) NodeIDs() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	ids := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		ids = append(ids, id)
	}
	return ids
}
