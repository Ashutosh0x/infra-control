package cli

import (
	"embed"
	"os"
	"path/filepath"

	"github.com/ashutosh0x/infra-control/internal/ui"
	"github.com/spf13/cobra"
)

// Demo exists to answer "what does this actually do" without asking for a
// state file first.
//
// Someone evaluating a tool at the end of a working day will not go and build a
// live snapshot to find out whether it is worth their time. The fixtures are
// embedded so the binary is self-contained: no clone, no network, no cloud
// account, no Terraform installation.

//go:embed all:demodata
var demoFS embed.FS

var demoKeep bool

var demoCmd = &cobra.Command{
	Use:     "demo",
	Short:   "Run a full scan against bundled fixtures, with no setup",
	GroupID: "system",
	Long: `Run the full pipeline against fixtures embedded in this binary.

Nothing is read from your machine and nothing is sent anywhere. It exists so
you can see real output before deciding whether to point the tool at your own
infrastructure.

The fixtures contain deliberate drift: a bucket that went public and lost
encryption, a database deleted outside Terraform, a subnet re-addressed, and a
security group nobody tracked.`,
	Example: `  infractl demo
  infractl demo --keep    # write the fixtures out so you can experiment`,
	Args: cobra.NoArgs,
	RunE: runDemo,
}

func runDemo(cmd *cobra.Command, _ []string) error {
	dir, cleanup, err := materialiseDemo()
	if err != nil {
		return err
	}
	defer cleanup()

	statePath := filepath.Join(dir, "terraform.tfstate")
	livePath := filepath.Join(dir, "live.json")

	if !rt.Format.IsMachine() {
		rt.UI.Raw(rt.UI.Panel("Demo", []string{
			"Fixtures embedded in this binary. Nothing is read from your machine",
			"and nothing leaves it.",
		}, ui.StyleInfo))
	}

	// The demo drives the same code path a real scan does, rather than printing
	// a canned transcript. If drift detection breaks, the demo breaks with it,
	// which is the only way a demo stays honest.
	previousState, previousLive, previousIgnore := driftStatePath, driftLivePath, driftNoIgnore
	driftStatePath, driftLivePath, driftNoIgnore = statePath, livePath, true
	defer func() {
		driftStatePath, driftLivePath, driftNoIgnore = previousState, previousLive, previousIgnore
	}()

	if !rt.Format.IsMachine() {
		rt.UI.Heading("Drift")
	}
	// The findings exit is swallowed: the demo is meant to show findings, so
	// exiting 3 for having done its job would be wrong.
	if err := runDriftScan(cmd, nil); err != nil && codeOf(err) != ExitFindings {
		return err
	}

	if rt.Format.IsMachine() {
		return nil
	}

	rt.UI.Heading("What breaks if the VPC changes")
	previousGraph := graphStatePath
	graphStatePath = statePath
	defer func() { graphStatePath = previousGraph }()

	if err := graphBlastRadiusCmd.RunE(cmd, []string{"aws_vpc.main"}); err != nil {
		return err
	}

	rt.UI.Println()
	rt.UI.Raw(rt.UI.KeyValue([][2]string{
		{"next", "point it at your own state"},
	}))
	hint(cmd, [][2]string{
		{"Capture live state", "infractl snapshot from-plan"},
		{"Scan it", "infractl drift scan --state terraform.tfstate --live live.json"},
		{"Check your setup", "infractl doctor"},
	})
	return nil
}

// materialiseDemo writes the embedded fixtures to a directory.
func materialiseDemo() (dir string, cleanup func(), err error) {
	if demoKeep {
		dir = "infractl-demo"
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return "", func() {}, failf(ExitError, "create %s: %w", dir, mkErr)
		}
		cleanup = func() {
			rt.UI.Detail("Fixtures kept in %s", dir)
		}
	} else {
		dir, err = os.MkdirTemp("", "infractl-demo-")
		if err != nil {
			return "", func() {}, failf(ExitError, "create temp directory: %w", err)
		}
		cleanup = func() { _ = os.RemoveAll(dir) }
	}

	entries, err := demoFS.ReadDir("demodata")
	if err != nil {
		cleanup()
		return "", func() {}, failf(ExitError, "read embedded fixtures: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, readErr := demoFS.ReadFile("demodata/" + entry.Name())
		if readErr != nil {
			cleanup()
			return "", func() {}, failf(ExitError, "read fixture %s: %w", entry.Name(), readErr)
		}
		if writeErr := os.WriteFile(filepath.Join(dir, entry.Name()), content, 0o600); writeErr != nil {
			cleanup()
			return "", func() {}, failf(ExitError, "write fixture %s: %w", entry.Name(), writeErr)
		}
	}

	return dir, cleanup, nil
}

func init() {
	rootCmd.AddCommand(demoCmd)
	demoCmd.Flags().BoolVar(&demoKeep, "keep", false,
		"write the fixtures to ./infractl-demo instead of a temporary directory")
}
