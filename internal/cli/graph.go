package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/ashutosh0x/infra-control/internal/graph"
	"github.com/ashutosh0x/infra-control/internal/terraform"
	"github.com/ashutosh0x/infra-control/internal/ui"
	"github.com/ashutosh0x/infra-control/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	graphStatePath string
	graphMaxDepth  int
	graphDirection string
	graphFormat    string
)

var graphCmd = &cobra.Command{
	Use:     "graph",
	Short:   "Analyse the infrastructure dependency graph",
	GroupID: "analyse",
	Long: `Build a dependency graph from Terraform or OpenTofu state and query it.

The edges come from the dependencies Terraform recorded when it applied, so they
reflect both explicit depends_on relationships and the implicit ones created by
attribute references. A dependency on something outside the state file, such as
a data source, has no node to point at and is left out rather than guessed.`,
}

var graphStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show node and edge counts for the graph",
	Example: `  infractl graph stats --state terraform.tfstate
  infractl graph stats --state terraform.tfstate -o json`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		g, err := loadGraph()
		if err != nil {
			return err
		}

		stats := g.Stats()
		payload := struct {
			Nodes       int            `json:"nodes"`
			Edges       int            `json:"edges"`
			NodesByType map[string]int `json:"nodes_by_type"`
			EdgesByType map[string]int `json:"edges_by_type"`
			Roots       int            `json:"roots"`
			Leaves      int            `json:"leaves"`
		}{
			Nodes:       stats.TotalNodes,
			Edges:       stats.TotalEdges,
			NodesByType: map[string]int{},
			EdgesByType: map[string]int{},
			Roots:       len(g.Roots()),
			Leaves:      len(g.Leaves()),
		}
		for t, n := range stats.NodesByType {
			payload.NodesByType[string(t)] = n
		}
		for t, n := range stats.EdgesByType {
			payload.EdgesByType[string(t)] = n
		}

		if rt.Format.IsMachine() {
			return rt.write(ui.View{Data: payload})
		}

		rt.UI.Raw(rt.UI.KeyValue([][2]string{
			{"nodes", fmt.Sprintf("%d", payload.Nodes)},
			{"edges", fmt.Sprintf("%d", payload.Edges)},
			{"roots", fmt.Sprintf("%d (nothing depends on these)", payload.Roots)},
			{"leaves", fmt.Sprintf("%d (these depend on nothing)", payload.Leaves)},
		}))

		if len(payload.EdgesByType) > 0 {
			rt.UI.Heading("Edges by type")
			table := ui.NewTable(
				ui.Column{Title: "TYPE", MinWidth: 12},
				ui.Column{Title: "COUNT", Align: ui.AlignRight, MinWidth: 5},
			)
			for _, entry := range sortedCounts(payload.EdgesByType) {
				table.StringRow(entry.name, fmt.Sprintf("%d", entry.count))
			}
			rt.UI.Raw(rt.UI.Render(table))
		}
		return nil
	},
}

var graphBlastRadiusCmd = &cobra.Command{
	Use:     "blast-radius <address>",
	Aliases: []string{"blast", "impact"},
	Short:   "Show what breaks if a resource changes",
	Long: `Traverse the dependency graph downstream from a resource and list
everything that depends on it, directly or transitively.

This answers the question that matters before a risky change: if this resource
is replaced or removed, what else is affected. Distance 1 is a direct dependent;
higher distances are reached through intermediate resources.`,
	Example: `  infractl graph blast-radius aws_vpc.main --state terraform.tfstate
  infractl graph blast-radius aws_vpc.main --state terraform.tfstate --max-depth 3
  infractl graph blast-radius aws_vpc.main --state terraform.tfstate -o json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		address := args[0]

		g, err := loadGraph()
		if err != nil {
			return err
		}

		if _, found := g.GetNode(address); !found {
			return failf(ExitUsage,
				"no resource at address %q in this state file.\n"+
					"  Run `infractl state list %s` to see the available addresses.",
				address, graphStatePath)
		}

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		radius, err := g.BlastRadius(ctx, address, graphMaxDepth)
		if err != nil {
			return failf(ExitError, "compute blast radius: %w", err)
		}

		sort.SliceStable(radius.AffectedNodes, func(i, j int) bool {
			if radius.AffectedNodes[i].Distance != radius.AffectedNodes[j].Distance {
				return radius.AffectedNodes[i].Distance < radius.AffectedNodes[j].Distance
			}
			return radius.AffectedNodes[i].NodeID < radius.AffectedNodes[j].NodeID
		})

		table := ui.NewTable(
			ui.Column{Title: "DISTANCE", Align: ui.AlignRight, MinWidth: 8},
			ui.Column{Title: "IMPACT", MinWidth: 10},
			ui.Column{Title: "RESOURCE", MinWidth: 24, Truncatable: true},
		)
		names := make([]string, 0, len(radius.AffectedNodes))
		for _, node := range radius.AffectedNodes {
			style := ui.StyleWarning
			if node.Distance > 1 {
				style = ui.StyleMuted
			}
			table.Row(
				ui.Text(fmt.Sprintf("%d", node.Distance)),
				ui.Styled(node.ImpactType, style),
				ui.Text(node.NodeID),
			)
			names = append(names, node.NodeID)
		}

		if err := rt.write(ui.View{
			Data:  radius,
			Table: table,
			Names: names,
			Empty: fmt.Sprintf("Nothing depends on %s. Changing it affects no other resource in this state.", address),
		}); err != nil {
			return err
		}

		if !rt.Format.IsMachine() && radius.TotalAffected > 0 {
			rt.UI.Println()
			rt.UI.Printf("Changing %s affects %d resources, up to %d hops away.\n",
				rt.UI.Apply(ui.StyleBold, address), radius.TotalAffected, radius.MaxDepth)
		}
		return nil
	},
}

var graphDepsCmd = &cobra.Command{
	Use:   "deps <address>",
	Short: "List what a resource depends on",
	Long: `List the resources a given resource depends on.

This is the inverse of blast-radius: it walks upstream, showing what must exist
before this resource can be created.`,
	Example: `  infractl graph deps aws_instance.web --state terraform.tfstate`,
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		address := args[0]

		g, err := loadGraph()
		if err != nil {
			return err
		}
		if _, found := g.GetNode(address); !found {
			return failf(ExitUsage,
				"no resource at address %q in this state file.\n"+
					"  Run `infractl state list %s` to see the available addresses.",
				address, graphStatePath)
		}

		neighbors := g.Neighbors(address, graphDirection)
		sort.Slice(neighbors, func(i, j int) bool { return neighbors[i].ID < neighbors[j].ID })

		table := ui.NewTable(
			ui.Column{Title: "RESOURCE", MinWidth: 24, Truncatable: true},
			ui.Column{Title: "TYPE", MinWidth: 10},
			ui.Column{Title: "PROVIDER", MinWidth: 8},
		)
		names := make([]string, 0, len(neighbors))
		for _, node := range neighbors {
			resourceType, _ := node.Properties["resource_type"].(string)
			table.StringRow(node.ID, resourceType, node.Provider)
			names = append(names, node.ID)
		}

		return rt.write(ui.View{
			Data:  neighbors,
			Table: table,
			Names: names,
			Empty: fmt.Sprintf("No %s dependencies recorded for %s.", graphDirection, address),
		})
	},
}

var graphExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the graph as DOT or Mermaid",
	Long: `Export the dependency graph for rendering elsewhere.

  dot      Graphviz source; render with ` + "`dot -Tsvg`" + `
  mermaid  Mermaid flowchart, which GitHub renders inline in Markdown`,
	Example: `  infractl graph export --state terraform.tfstate --format dot > graph.dot
  infractl graph export --state terraform.tfstate --format mermaid`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		g, err := loadGraph()
		if err != nil {
			return err
		}

		switch strings.ToLower(graphFormat) {
		case "dot":
			rt.UI.Raw(exportDOT(g))
		case "mermaid":
			rt.UI.Raw(exportMermaid(g))
		default:
			return failf(ExitUsage, "invalid --format %q (want dot or mermaid)", graphFormat)
		}
		return nil
	},
}

// loadGraph reads the configured state file and builds the dependency graph.
func loadGraph() (*graph.Graph, error) {
	if err := requireFile(graphStatePath, "state file (--state)"); err != nil {
		return nil, err
	}

	state, err := terraform.ParseStateFile(graphStatePath)
	if err != nil {
		return nil, failf(ExitError, "%w", err)
	}

	logger := zap.NewNop()
	if verbose {
		logger, _ = zap.NewDevelopment()
	}

	g, err := graph.FromState(state, logger)
	if err != nil {
		return nil, failf(ExitError, "build dependency graph: %w", err)
	}
	return g, nil
}

// exportDOT renders the graph as Graphviz DOT source.
func exportDOT(g *graph.Graph) string {
	var b strings.Builder
	b.WriteString("digraph infrastructure {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [shape=box, style=rounded, fontname=\"Helvetica\"];\n")

	ids := g.NodeIDs()
	sort.Strings(ids)

	for _, id := range ids {
		node, _ := g.GetNode(id)
		if node == nil {
			continue
		}
		shape := "box"
		if node.Type == types.NodeTypeModule {
			shape = "folder"
		}
		fmt.Fprintf(&b, "  %q [label=%q, shape=%s];\n", id, node.Label, shape)
	}

	for _, id := range ids {
		for _, neighbor := range g.Neighbors(id, "downstream") {
			fmt.Fprintf(&b, "  %q -> %q;\n", id, neighbor.ID)
		}
	}

	b.WriteString("}\n")
	return b.String()
}

// exportMermaid renders the graph as a Mermaid flowchart.
func exportMermaid(g *graph.Graph) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")

	ids := g.NodeIDs()
	sort.Strings(ids)

	// Mermaid node IDs cannot contain dots or brackets, so each address is given
	// a stable synthetic ID and the real address becomes the label.
	alias := make(map[string]string, len(ids))
	for i, id := range ids {
		alias[id] = fmt.Sprintf("n%d", i)
	}

	for _, id := range ids {
		node, _ := g.GetNode(id)
		if node == nil {
			continue
		}
		fmt.Fprintf(&b, "  %s[%q]\n", alias[id], node.Label)
	}

	for _, id := range ids {
		for _, neighbor := range g.Neighbors(id, "downstream") {
			fmt.Fprintf(&b, "  %s --> %s\n", alias[id], alias[neighbor.ID])
		}
	}

	return b.String()
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.AddCommand(graphStatsCmd, graphBlastRadiusCmd, graphDepsCmd, graphExportCmd)

	// --state applies to every graph subcommand.
	graphCmd.PersistentFlags().StringVar(&graphStatePath, "state", "",
		"path to the Terraform or OpenTofu state file (required)")
	_ = graphCmd.MarkPersistentFlagRequired("state")
	_ = graphCmd.MarkPersistentFlagFilename("state", "tfstate", "json")

	graphBlastRadiusCmd.Flags().IntVar(&graphMaxDepth, "max-depth", 0,
		"stop traversing after this many hops (0 means no limit)")
	graphDepsCmd.Flags().StringVar(&graphDirection, "direction", "upstream",
		"which way to walk: upstream, downstream, both")
	graphExportCmd.Flags().StringVar(&graphFormat, "format", "dot",
		"export format: dot, mermaid")

	_ = graphDepsCmd.RegisterFlagCompletionFunc("direction",
		cobra.FixedCompletions([]string{"upstream", "downstream", "both"}, cobra.ShellCompDirectiveNoFileComp))
	_ = graphExportCmd.RegisterFlagCompletionFunc("format",
		cobra.FixedCompletions([]string{"dot", "mermaid"}, cobra.ShellCompDirectiveNoFileComp))
}
