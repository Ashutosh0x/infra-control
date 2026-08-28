package graph

import (
	"context"
	"strings"
	"testing"

	"github.com/ashutosh0x/infra-control/internal/terraform"
	"github.com/ashutosh0x/infra-control/pkg/types"
	"go.uber.org/zap"
)

// stateWithDependencies exercises the cases the graph builder has to get right:
// a plain singleton, a counted resource, and a dependency recorded at block
// granularity that must fan out to both instances.
const stateWithDependencies = `{
  "version": 4,
  "resources": [
    {"mode": "managed", "type": "aws_vpc", "name": "main", "provider": "p",
     "instances": [{"attributes": {}}]},
    {"mode": "managed", "type": "aws_subnet", "name": "private", "provider": "p",
     "instances": [
       {"index_key": 0, "attributes": {}, "dependencies": ["aws_vpc.main"]},
       {"index_key": 1, "attributes": {}, "dependencies": ["aws_vpc.main"]}
     ]},
    {"mode": "managed", "type": "aws_db_instance", "name": "primary", "provider": "p",
     "instances": [{"attributes": {}, "dependencies": ["aws_subnet.private"]}]}
  ]
}`

func buildTestGraph(t *testing.T, stateJSON string) *Graph {
	t.Helper()

	state, err := terraform.ParseState(strings.NewReader(stateJSON))
	if err != nil {
		t.Fatalf("ParseState: %v", err)
	}
	g, err := FromState(state, zap.NewNop())
	if err != nil {
		t.Fatalf("FromState: %v", err)
	}
	return g
}

func TestFromStateBuildsNodesForEveryInstance(t *testing.T) {
	g := buildTestGraph(t, stateWithDependencies)

	for _, address := range []string{
		"aws_vpc.main",
		"aws_subnet.private[0]",
		"aws_subnet.private[1]",
		"aws_db_instance.primary",
	} {
		if _, found := g.GetNode(address); !found {
			t.Errorf("missing node for %s", address)
		}
	}

	if stats := g.Stats(); stats.TotalNodes != 4 {
		t.Errorf("TotalNodes = %d, want 4", stats.TotalNodes)
	}
}

func TestFromStateFansOutBlockLevelDependencies(t *testing.T) {
	// State records the dependency as `aws_subnet.private`, with no index. Both
	// instances of that block must become edges, or the blast radius of the VPC
	// stops short of the database.
	g := buildTestGraph(t, stateWithDependencies)

	dependencies := g.Neighbors("aws_db_instance.primary", "upstream")
	if len(dependencies) != 2 {
		t.Fatalf("got %d upstream dependencies, want 2 (both subnet instances): %v",
			len(dependencies), nodeIDs(dependencies))
	}
}

func TestBlastRadiusReachesTransitiveDependents(t *testing.T) {
	g := buildTestGraph(t, stateWithDependencies)

	radius, err := g.BlastRadius(context.Background(), "aws_vpc.main", 0)
	if err != nil {
		t.Fatalf("BlastRadius: %v", err)
	}

	// Two subnets at distance 1, the database at distance 2.
	if radius.TotalAffected != 3 {
		t.Fatalf("TotalAffected = %d, want 3: %+v", radius.TotalAffected, radius.AffectedNodes)
	}

	distances := map[string]int{}
	for _, node := range radius.AffectedNodes {
		distances[node.NodeID] = node.Distance
	}
	if distances["aws_subnet.private[0]"] != 1 {
		t.Errorf("subnet should be a direct dependent, got distance %d", distances["aws_subnet.private[0]"])
	}
	if distances["aws_db_instance.primary"] != 2 {
		t.Errorf("database should be two hops away, got distance %d", distances["aws_db_instance.primary"])
	}
}

func TestBlastRadiusRespectsMaxDepth(t *testing.T) {
	g := buildTestGraph(t, stateWithDependencies)

	radius, err := g.BlastRadius(context.Background(), "aws_vpc.main", 1)
	if err != nil {
		t.Fatalf("BlastRadius: %v", err)
	}

	for _, node := range radius.AffectedNodes {
		if node.Distance > 1 {
			t.Errorf("max-depth 1 should exclude %s at distance %d", node.NodeID, node.Distance)
		}
	}
}

func TestFromStateSkipsDependenciesOutsideState(t *testing.T) {
	// A dependency on a data source or a resource in another state file has no
	// node to point at. It must be skipped, never invented, or the graph would
	// assert a relationship state does not support.
	g := buildTestGraph(t, `{
      "version": 4,
      "resources": [
        {"mode": "managed", "type": "aws_instance", "name": "web", "provider": "p",
         "instances": [{"attributes": {},
           "dependencies": ["data.aws_ami.ubuntu", "aws_vpc.elsewhere"]}]}
      ]
    }`)

	stats := g.Stats()
	if stats.TotalNodes != 1 {
		t.Errorf("TotalNodes = %d, want 1", stats.TotalNodes)
	}
	if stats.TotalEdges != 0 {
		t.Errorf("TotalEdges = %d, want 0 (unresolvable dependencies must not create edges)", stats.TotalEdges)
	}
}

func TestRootsAndLeaves(t *testing.T) {
	g := buildTestGraph(t, stateWithDependencies)

	roots := nodeIDs(g.Roots())
	if len(roots) != 1 || roots[0] != "aws_vpc.main" {
		t.Errorf("Roots = %v, want [aws_vpc.main] (nothing depends on the VPC)", roots)
	}

	leaves := nodeIDs(g.Leaves())
	if len(leaves) != 1 || leaves[0] != "aws_db_instance.primary" {
		t.Errorf("Leaves = %v, want [aws_db_instance.primary]", leaves)
	}
}

func TestFromStateCreatesModuleContainment(t *testing.T) {
	g := buildTestGraph(t, `{
      "version": 4,
      "resources": [
        {"mode": "managed", "module": "module.vpc", "type": "aws_subnet", "name": "a",
         "provider": "p", "instances": [{"attributes": {}}]}
      ]
    }`)

	if _, found := g.GetNode("module.vpc"); !found {
		t.Fatal("expected a node for module.vpc")
	}

	contained := g.Neighbors("module.vpc", "downstream")
	if len(contained) != 1 || contained[0].ID != "module.vpc.aws_subnet.a" {
		t.Errorf("module should contain its resource, got %v", nodeIDs(contained))
	}
}

func TestFromStateRejectsNilState(t *testing.T) {
	if _, err := FromState(nil, zap.NewNop()); err == nil {
		t.Error("FromState(nil) should return an error")
	}
}

func nodeIDs(nodes []*types.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.ID)
	}
	return out
}
