package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ashutosh0x/infra-control/internal/terraform"
	"github.com/ashutosh0x/infra-control/internal/ui"
	"github.com/spf13/cobra"
)

var (
	snapshotOut      string
	snapshotProvider string
	snapshotDir      string
	snapshotKeepPlan bool
)

var snapshotCmd = &cobra.Command{
	Use:     "snapshot",
	Short:   "Capture a live snapshot of infrastructure for drift scanning",
	GroupID: "analyse",
	Long: `Produce the live snapshot that ` + "`drift scan`" + ` compares Terraform state against.

Without a snapshot, drift detection has nothing to compare to, and building one
by hand is the hardest part of adopting this tool. These commands build it from
sources you already have.`,
}

var snapshotFromPlanCmd = &cobra.Command{
	Use:   "from-plan [plan.json]",
	Short: "Build a snapshot from a Terraform refresh-only plan",
	Long: `Build a live snapshot from a Terraform or OpenTofu refresh-only plan.

A refresh-only plan asks every provider for the real attributes of every
managed resource. Terraform records those refreshed values in the plan's prior
state, which is a live read of your infrastructure performed by Terraform
itself, using credentials you have already configured.

Run with no argument and it does the whole sequence for you:

  terraform plan -refresh-only -out=<tmp>
  terraform show -json <tmp>

Pass a file and it reads a plan you produced yourself.

What this cannot do: Terraform refreshes only what it manages, so a snapshot
built this way never contains an unmanaged resource. ` + "`drift scan`" + ` against it
will report modified and deleted resources but can report no unmanaged ones.
For those you need an inventory read; see docs/live-snapshots.md.`,
	Example: `  # Capture and scan, from a Terraform directory
  infractl snapshot from-plan
  infractl drift scan --state terraform.tfstate --live live.json

  # From a plan you already produced
  terraform plan -refresh-only -out=tfplan
  terraform show -json tfplan > plan.json
  infractl snapshot from-plan plan.json

  # Write somewhere other than ./live.json
  infractl snapshot from-plan --out snapshots/prod.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSnapshotFromPlan,
}

func runSnapshotFromPlan(cmd *cobra.Command, args []string) error {
	var planJSON string

	if len(args) == 1 {
		planJSON = args[0]
		if err := requireFile(planJSON, "plan file"); err != nil {
			return err
		}
	} else {
		generated, cleanup, err := generateRefreshPlan()
		if err != nil {
			return err
		}
		defer cleanup()
		planJSON = generated
	}

	spinner := rt.UI.Spin("Reading refreshed values")
	extracted, err := terraform.ExtractPriorStateFile(planJSON)
	if err != nil {
		spinner.Fail("Could not read the plan")
		return failf(ExitError, "%w", err)
	}
	spinner.Stop()

	provider := snapshotProvider
	if provider == "" && len(extracted.Providers) == 1 {
		provider = extracted.Providers[0]
	}

	snapshot := liveSnapshot{
		CapturedAt: time.Now().UTC(),
		Provider:   provider,
		Resources:  extracted.Resources,
	}

	if err := writeSnapshot(snapshotOut, snapshot); err != nil {
		return err
	}

	rt.UI.Success("Captured %d resources to %s", len(snapshot.Resources), snapshotOut)

	if len(extracted.DriftedAddresses) > 0 {
		rt.UI.Detail("Terraform's own refresh flagged %d resource(s) as changed.",
			len(extracted.DriftedAddresses))
	}

	// The blind spot is stated every time, not just in the help text. A user who
	// runs a scan and sees no unmanaged resources should know whether that means
	// there are none or that this snapshot cannot see them.
	rt.UI.Detail("This snapshot covers only Terraform-managed resources; unmanaged ones cannot appear.")

	if !rt.Format.IsMachine() {
		rt.UI.Println()
		hint(cmd, [][2]string{
			{"Scan for drift", fmt.Sprintf("infractl drift scan --state %s --live %s",
				guessStatePath(), snapshotOut)},
		})
	}
	return nil
}

// generateRefreshPlan runs Terraform to produce a refresh-only plan as JSON.
//
// The plan file is binary and has no public format, so it is converted with
// `terraform show -json` and the binary discarded.
func generateRefreshPlan() (path string, cleanup func(), err error) {
	binary, err := terraformBinary()
	if err != nil {
		return "", func() {}, err
	}

	dir := snapshotDir
	if dir == "" {
		dir = "."
	}
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		return "", func() {}, failf(ExitUsage, "not a directory: %s", dir)
	}

	tmp, err := os.MkdirTemp("", "infractl-plan-")
	if err != nil {
		return "", func() {}, failf(ExitError, "create temp directory: %w", err)
	}
	cleanup = func() {
		if !snapshotKeepPlan {
			_ = os.RemoveAll(tmp)
		}
	}

	planPath := filepath.Join(tmp, "refresh.tfplan")
	jsonPath := filepath.Join(tmp, "refresh.json")

	spinner := rt.UI.Spin("Refreshing state against live infrastructure")

	// -refresh-only reads every managed resource and proposes no changes, so
	// this cannot modify infrastructure. -input=false stops Terraform blocking
	// on a prompt when a variable is missing, which would otherwise hang CI.
	plan := exec.Command(binary, "plan", "-refresh-only", "-input=false", "-out="+planPath) //nolint:gosec // binary is resolved from PATH or an explicit user-supplied path
	plan.Dir = dir
	plan.Stderr = rt.UI.Err()

	if output, planErr := plan.Output(); planErr != nil {
		spinner.Fail("terraform plan failed")
		cleanup()
		return "", func() {}, failf(ExitError,
			"terraform plan -refresh-only failed in %s: %w\n"+
				"  Run it directly to see why. A missing `terraform init` is the usual cause.\n%s",
			dir, planErr, strings.TrimSpace(string(output)))
	}

	spinner.Update("Converting the plan to JSON")

	show := exec.Command(binary, "show", "-json", planPath) //nolint:gosec // as above
	show.Dir = dir

	jsonBytes, showErr := show.Output()
	if showErr != nil {
		spinner.Fail("terraform show failed")
		cleanup()
		return "", func() {}, failf(ExitError, "terraform show -json failed: %w", showErr)
	}

	if err := os.WriteFile(jsonPath, jsonBytes, 0o600); err != nil {
		spinner.Fail("Could not write the plan JSON")
		cleanup()
		return "", func() {}, failf(ExitError, "write plan json: %w", err)
	}

	spinner.Stop()

	if snapshotKeepPlan {
		rt.UI.Detail("Kept the intermediate plan at %s", jsonPath)
	}
	return jsonPath, cleanup, nil
}

// terraformBinary finds terraform or tofu on PATH.
func terraformBinary() (string, error) {
	if explicit := os.Getenv("INFRACTL_TERRAFORM_BINARY"); explicit != "" {
		return explicit, nil
	}
	for _, candidate := range []string{"terraform", "tofu"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", failf(ExitConfig,
		"neither `terraform` nor `tofu` is on PATH.\n"+
			"  Install one, or set INFRACTL_TERRAFORM_BINARY to its path.\n"+
			"  To skip this step, produce the plan yourself and pass it:\n"+
			"    terraform plan -refresh-only -out=tfplan\n"+
			"    terraform show -json tfplan > plan.json\n"+
			"    infractl snapshot from-plan plan.json")
}

// writeSnapshot serialises the snapshot to disk.
func writeSnapshot(path string, snapshot liveSnapshot) error {
	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return failf(ExitError, "encode snapshot: %w", err)
	}

	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return failf(ExitError, "create directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		return failf(ExitError, "write snapshot %s: %w", path, err)
	}
	return nil
}

// guessStatePath returns the state file a follow-up command would most likely
// use, so the suggested next step is copy-pasteable rather than a placeholder.
func guessStatePath() string {
	for _, candidate := range []string{"terraform.tfstate", ".terraform/terraform.tfstate"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "terraform.tfstate"
}

// hint prints suggested next commands after a result.
//
// A result that ends without saying what to do next makes the user go back to
// the help text to make progress. Suppressed for machine output, where it would
// corrupt the payload, and in quiet mode.
func hint(_ *cobra.Command, steps [][2]string) {
	if rt.Format.IsMachine() || rt.UI.Quiet() || len(steps) == 0 {
		return
	}

	width := 0
	for _, step := range steps {
		if len(step[0]) > width {
			width = len(step[0])
		}
	}

	for _, step := range steps {
		rt.UI.Printf("  %s  %s\n",
			rt.UI.Apply(ui.StyleMuted, pad(step[0], width)),
			rt.UI.Apply(ui.StyleInfo, step[1]))
	}
}

// pad right-pads a label for hint alignment.
func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
	snapshotCmd.AddCommand(snapshotFromPlanCmd)

	f := snapshotFromPlanCmd.Flags()
	f.StringVar(&snapshotOut, "out", "live.json", "where to write the snapshot")
	f.StringVar(&snapshotProvider, "provider", "", "provider name to record (inferred when the plan uses only one)")
	f.StringVar(&snapshotDir, "dir", ".", "Terraform directory to run the refresh plan in")
	f.BoolVar(&snapshotKeepPlan, "keep-plan", false, "keep the intermediate plan JSON for inspection")

	_ = snapshotFromPlanCmd.MarkFlagFilename("out", "json")
	_ = snapshotFromPlanCmd.MarkFlagDirname("dir")
}
