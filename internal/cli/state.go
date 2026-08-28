package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ashutosh0x/infra-control/internal/terraform"
	"github.com/ashutosh0x/infra-control/internal/ui"
	"github.com/spf13/cobra"
)

var (
	stateFilterType     string
	stateFilterProvider string
	stateFilterModule   string
)

var stateCmd = &cobra.Command{
	Use:     "state",
	Short:   "Inspect Terraform and OpenTofu state files",
	GroupID: "analyse",
	Long: `Read a Terraform or OpenTofu state file and report what it manages.

These commands are read-only and never write to the state file, so they are safe
to run against a copy of production state.`,
}

// stateSummary is the payload of `state inspect`.
type stateSummary struct {
	Path             string         `json:"path"`
	FormatVersion    int            `json:"format_version"`
	TerraformVersion string         `json:"terraform_version"`
	Serial           uint64         `json:"serial"`
	Lineage          string         `json:"lineage"`
	ManagedResources int            `json:"managed_resources"`
	Providers        []string       `json:"providers"`
	ResourceTypes    map[string]int `json:"resource_types"`
	Outputs          int            `json:"outputs"`
	SensitiveOutputs int            `json:"sensitive_outputs"`
}

var stateInspectCmd = &cobra.Command{
	Use:   "inspect <state-file>",
	Short: "Summarise what a state file manages",
	Long: `Summarise a state file: the Terraform version that wrote it, its serial
and lineage, how many resources it manages, and the breakdown by provider and
resource type.

This is the fastest way to answer "what is in this state file" without loading
it into Terraform.`,
	Example: `  infractl state inspect terraform.tfstate
  infractl state inspect terraform.tfstate -o json`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		path := args[0]
		if err := requireFile(path, "state file"); err != nil {
			return err
		}

		state, err := terraform.ParseStateFile(path)
		if err != nil {
			return failf(ExitError, "%w", err)
		}

		sensitive := 0
		for _, output := range state.Outputs {
			if output.Sensitive {
				sensitive++
			}
		}

		summary := stateSummary{
			Path:             path,
			FormatVersion:    state.Version,
			TerraformVersion: state.TerraformVersion,
			Serial:           state.Serial,
			Lineage:          state.Lineage,
			ManagedResources: state.ResourceCount(),
			Providers:        state.Providers(),
			ResourceTypes:    state.ResourceTypes(),
			Outputs:          len(state.Outputs),
			SensitiveOutputs: sensitive,
		}

		// Machine formats get the payload and nothing else.
		if rt.Format.IsMachine() {
			return rt.write(ui.View{Data: summary, Names: []string{path}})
		}

		rt.UI.Raw(rt.UI.KeyValue([][2]string{
			{"state file", path},
			{"format version", fmt.Sprintf("%d", summary.FormatVersion)},
			{"written by", "Terraform " + summary.TerraformVersion},
			{"serial", fmt.Sprintf("%d", summary.Serial)},
			{"lineage", summary.Lineage},
			{"managed resources", fmt.Sprintf("%d", summary.ManagedResources)},
			{"providers", strings.Join(summary.Providers, ", ")},
			{"outputs", fmt.Sprintf("%d (%d sensitive)", summary.Outputs, summary.SensitiveOutputs)},
		}))

		if len(summary.ResourceTypes) == 0 {
			return nil
		}

		rt.UI.Heading("Resource types")

		table := ui.NewTable(
			ui.Column{Title: "TYPE", MinWidth: 20, Truncatable: true},
			ui.Column{Title: "COUNT", Align: ui.AlignRight, MinWidth: 5},
		)
		for _, entry := range sortedCounts(summary.ResourceTypes) {
			table.StringRow(entry.name, fmt.Sprintf("%d", entry.count))
		}
		rt.UI.Raw(rt.UI.Render(table))
		return nil
	},
}

var stateListCmd = &cobra.Command{
	Use:     "list <state-file>",
	Aliases: []string{"ls"},
	Short:   "List the resources a state file manages",
	Long: `List every managed resource instance in a state file, with its Terraform
address, type, provider, and module.

Data sources are excluded: Terraform reads them but does not own them.`,
	Example: `  infractl state list terraform.tfstate
  infractl state list terraform.tfstate --type aws_s3_bucket
  infractl state list terraform.tfstate --provider aws -o name`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		path := args[0]
		if err := requireFile(path, "state file"); err != nil {
			return err
		}

		state, err := terraform.ParseStateFile(path)
		if err != nil {
			return failf(ExitError, "%w", err)
		}

		types := splitList(stateFilterType)
		providers := splitList(stateFilterProvider)

		var matched []terraform.StateResource
		for _, resource := range state.ManagedResources() {
			if len(types) > 0 && !containsFold(types, resource.Type) {
				continue
			}
			if len(providers) > 0 && !containsFold(providers, resource.Provider) {
				continue
			}
			if stateFilterModule != "" && !strings.Contains(resource.Module, stateFilterModule) {
				continue
			}
			matched = append(matched, resource)
		}

		table := ui.NewTable(
			ui.Column{Title: "ADDRESS", MinWidth: 24, Truncatable: true},
			ui.Column{Title: "TYPE", MinWidth: 16, Truncatable: true},
			ui.Column{Title: "PROVIDER", MinWidth: 8},
			ui.Column{Title: "MODULE", MinWidth: 6, Truncatable: true},
		)

		names := make([]string, 0, len(matched))
		for _, resource := range matched {
			module := resource.Module
			if module == "" {
				module = "root"
			}
			table.StringRow(resource.Address, resource.Type, resource.Provider, module)
			names = append(names, resource.Address)
		}

		return rt.write(ui.View{
			Data:  matched,
			Table: table,
			Names: names,
			Empty: "No managed resources match those filters.",
		})
	},
}

var stateShowCmd = &cobra.Command{
	Use:   "show <state-file> <address>",
	Short: "Show the attributes of one resource in state",
	Long: `Print every attribute Terraform records for a single resource.

Attributes whose names indicate a secret are masked. The masking is based on the
attribute name, not on the provider's own sensitivity marking, so treat the
output as sensitive regardless.`,
	Example: `  infractl state show terraform.tfstate aws_s3_bucket.assets
  infractl state show terraform.tfstate 'module.vpc.aws_subnet.private[0]'`,
	Args: cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		path, address := args[0], args[1]
		if err := requireFile(path, "state file"); err != nil {
			return err
		}

		state, err := terraform.ParseStateFile(path)
		if err != nil {
			return failf(ExitError, "%w", err)
		}

		resource, found := state.FindByAddress(address)
		if !found {
			return failf(ExitUsage,
				"no managed resource at address %q.\n"+
					"  Run `infractl state list %s` to see the addresses in this state file.",
				address, path)
		}

		// Mask before the payload is built, so `-o json` cannot leak a secret.
		masked := make(map[string]any, len(resource.Attributes))
		for key, value := range resource.Attributes {
			if terraform.IsSensitivePath(key) {
				masked[key] = "(sensitive value hidden)"
				continue
			}
			masked[key] = value
		}

		payload := struct {
			Address      string         `json:"address"`
			Type         string         `json:"type"`
			Provider     string         `json:"provider"`
			Module       string         `json:"module"`
			Dependencies []string       `json:"dependencies,omitempty"`
			Attributes   map[string]any `json:"attributes"`
		}{
			Address:      resource.Address,
			Type:         resource.Type,
			Provider:     resource.Provider,
			Module:       resource.Module,
			Dependencies: resource.Dependencies,
			Attributes:   masked,
		}

		if rt.Format.IsMachine() {
			return rt.write(ui.View{Data: payload, Names: []string{resource.Address}})
		}

		rt.UI.Raw(rt.UI.KeyValue([][2]string{
			{"address", resource.Address},
			{"type", resource.Type},
			{"provider", resource.Provider},
			{"dependencies", fmt.Sprintf("%d", len(resource.Dependencies))},
		}))

		rt.UI.Heading("Attributes")

		table := ui.NewTable(
			ui.Column{Title: "ATTRIBUTE", MinWidth: 16, Truncatable: true},
			ui.Column{Title: "VALUE", MinWidth: 20, Truncatable: true},
		)

		keys := make([]string, 0, len(masked))
		for key := range masked {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			cell := ui.Text(formatAttr(masked[key]))
			if terraform.IsSensitivePath(key) {
				cell = ui.Styled(formatAttr(masked[key]), ui.StyleMuted)
			}
			table.Row(ui.Text(key), cell)
		}
		rt.UI.Raw(rt.UI.Render(table))
		return nil
	},
}

// nameCount pairs a name with its count for sorted display.
type nameCount struct {
	name  string
	count int
}

// sortedCounts orders a count map by descending count, then name, so the
// output is both useful and stable.
func sortedCounts(counts map[string]int) []nameCount {
	entries := make([]nameCount, 0, len(counts))
	for name, count := range counts {
		entries = append(entries, nameCount{name: name, count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].name < entries[j].name
	})
	return entries
}

// containsFold reports case-insensitive membership.
func containsFold(haystack []string, needle string) bool {
	for _, item := range haystack {
		if strings.EqualFold(item, needle) {
			return true
		}
	}
	return false
}

// formatAttr renders an attribute value compactly for a table cell.
func formatAttr(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return t
	case bool:
		return fmt.Sprintf("%t", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case []any:
		return fmt.Sprintf("[%d items]", len(t))
	case map[string]any:
		return fmt.Sprintf("{%d keys}", len(t))
	default:
		return fmt.Sprintf("%v", t)
	}
}

func init() {
	rootCmd.AddCommand(stateCmd)
	stateCmd.AddCommand(stateInspectCmd, stateListCmd, stateShowCmd)

	f := stateListCmd.Flags()
	f.StringVar(&stateFilterType, "type", "", "only list these resource types (comma-separated)")
	f.StringVar(&stateFilterProvider, "provider", "", "only list resources from these providers (comma-separated)")
	f.StringVar(&stateFilterModule, "module", "", "only list resources whose module path contains this string")
}
