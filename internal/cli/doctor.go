package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ashutosh0x/infra-control/internal/ignore"
	"github.com/ashutosh0x/infra-control/internal/terraform"
	"github.com/ashutosh0x/infra-control/internal/ui"
	"github.com/ashutosh0x/infra-control/pkg/version"
	"github.com/spf13/cobra"
)

// Doctor exists because "your tool is broken" is almost always "my inputs are
// not what I think they are": a stale snapshot, an ignore rule that expired, a
// state file that is actually a directory, no terraform on PATH.
//
// Every check states what it looked at and, on failure, the exact command that
// fixes it. This is the output people paste into an issue, so it has to be
// worth reading on its own.

// checkStatus is the outcome of one diagnostic.
type checkStatus string

const (
	checkPass checkStatus = "pass"
	checkWarn checkStatus = "warn"
	checkFail checkStatus = "fail"
	checkSkip checkStatus = "skip"
)

// checkResult is one diagnostic's finding.
type checkResult struct {
	Name   string      `json:"name"`
	Status checkStatus `json:"status"`
	Detail string      `json:"detail"`
	Fix    string      `json:"fix,omitempty"`
}

// doctorReport is the full diagnostic payload.
type doctorReport struct {
	Version string        `json:"version"`
	Checks  []checkResult `json:"checks"`
	Passed  int           `json:"passed"`
	Warned  int           `json:"warned"`
	Failed  int           `json:"failed"`
}

var (
	doctorState string
	doctorLive  string
)

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Short:   "Check that the environment and inputs are usable",
	GroupID: "system",
	Long: `Check everything a scan depends on and report what is wrong.

Each failing check names the command that fixes it. Run this first when a scan
does not behave the way you expect, and include the output when reporting a bug.`,
	Example: `  infractl doctor
  infractl doctor --state terraform.tfstate --live live.json
  infractl doctor -o json`,
	Args: cobra.NoArgs,
	RunE: runDoctor,
}

func runDoctor(_ *cobra.Command, _ []string) error {
	report := doctorReport{Version: version.Get().Version}

	report.Checks = append(report.Checks,
		checkTerraformBinary(),
		checkConfigFile(),
		checkIgnoreFile(),
	)
	report.Checks = append(report.Checks, checkStateFile()...)
	report.Checks = append(report.Checks, checkLiveSnapshot()...)

	for _, check := range report.Checks {
		switch check.Status {
		case checkPass:
			report.Passed++
		case checkWarn:
			report.Warned++
		case checkFail:
			report.Failed++
		}
	}

	if rt.Format.IsMachine() {
		if err := rt.write(ui.View{Data: report}); err != nil {
			return err
		}
	} else {
		printDoctor(report)
	}

	// A failing check means a scan would not work, which is worth a non-zero
	// exit so that a setup step in CI stops rather than continuing into a
	// confusing failure later.
	if report.Failed > 0 {
		return failf(ExitConfig, "%d check(s) failed", report.Failed)
	}
	return nil
}

// checkTerraformBinary reports whether a refresh-only snapshot can be taken.
func checkTerraformBinary() checkResult {
	for _, candidate := range []string{"terraform", "tofu"} {
		path, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}

		out, versionErr := exec.Command(path, "version").Output() //nolint:gosec // resolved from PATH
		detail := path
		if versionErr == nil {
			if first, _, ok := strings.Cut(string(out), "\n"); ok {
				detail = fmt.Sprintf("%s (%s)", strings.TrimSpace(first), path)
			}
		}
		return checkResult{Name: "terraform binary", Status: checkPass, Detail: detail}
	}

	return checkResult{
		Name:   "terraform binary",
		Status: checkWarn,
		Detail: "neither terraform nor tofu is on PATH",
		Fix:    "only needed for `infractl snapshot from-plan`; scans against an existing snapshot work without it",
	}
}

// checkConfigFile reports whether a config file was found and parses.
func checkConfigFile() checkResult {
	path := cfgFile
	if path == "" {
		for _, candidate := range []string{".infractl.yaml", ".infractl.yml"} {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
	}
	if path == "" {
		return checkResult{
			Name:   "config file",
			Status: checkSkip,
			Detail: "none found; flags and defaults are in use",
		}
	}

	// loadConfig on a throwaway command surfaces a parse error without
	// disturbing the flags of the running one.
	probe := &cobra.Command{Use: "probe"}
	probe.Flags().String("state", "", "")

	previous := cfgFile
	cfgFile = path
	err := loadConfig(probe)
	cfgFile = previous

	if err != nil {
		return checkResult{
			Name:   "config file",
			Status: checkFail,
			Detail: fmt.Sprintf("%s does not parse: %v", path, err),
			Fix:    "fix the YAML, or move the file aside to fall back to defaults",
		}
	}
	return checkResult{Name: "config file", Status: checkPass, Detail: path}
}

// checkIgnoreFile reports on suppression rules, including expired ones.
func checkIgnoreFile() checkResult {
	path := driftIgnorePath
	if path == "" {
		cwd, err := os.Getwd()
		if err == nil {
			path = ignore.FindDefault(cwd)
		}
	}
	if path == "" {
		return checkResult{Name: "ignore rules", Status: checkSkip, Detail: "no ignore file"}
	}

	rules, err := ignore.Load(path)
	if err != nil {
		return checkResult{
			Name:   "ignore rules",
			Status: checkFail,
			Detail: err.Error(),
			Fix:    "every rule needs an address and a reason",
		}
	}

	if expired := rules.Expired(); len(expired) > 0 {
		return checkResult{
			Name:   "ignore rules",
			Status: checkWarn,
			Detail: fmt.Sprintf("%d active, %d expired and no longer suppressing", rules.Len(), len(expired)),
			Fix:    "renew or delete the expired rules; findings they hid are now reported again",
		}
	}
	return checkResult{
		Name:   "ignore rules",
		Status: checkPass,
		Detail: fmt.Sprintf("%d active rule(s) in %s", rules.Len(), path),
	}
}

// checkStateFile validates the configured state file.
func checkStateFile() []checkResult {
	path := doctorState
	if path == "" {
		path = driftStatePath
	}
	if path == "" {
		path = guessStatePath()
		if _, err := os.Stat(path); err != nil {
			return []checkResult{{
				Name:   "state file",
				Status: checkSkip,
				Detail: "none configured and none found in the working directory",
				Fix:    "pass --state, or set `state:` in .infractl.yaml",
			}}
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		return []checkResult{{
			Name: "state file", Status: checkFail,
			Detail: fmt.Sprintf("%s cannot be read: %v", path, err),
		}}
	}
	if info.IsDir() {
		return []checkResult{{
			Name: "state file", Status: checkFail,
			Detail: fmt.Sprintf("%s is a directory", path),
			Fix:    "point --state at a .tfstate file",
		}}
	}

	state, err := terraform.ParseStateFile(path)
	if err != nil {
		return []checkResult{{
			Name: "state file", Status: checkFail,
			Detail: err.Error(),
			Fix:    "a truncated or half-written state file is the usual cause",
		}}
	}

	results := []checkResult{{
		Name:   "state file",
		Status: checkPass,
		Detail: fmt.Sprintf("%s: %d managed resources, Terraform %s, serial %d",
			path, state.ResourceCount(), state.TerraformVersion, state.Serial),
	}}

	if state.ResourceCount() == 0 {
		results = append(results, checkResult{
			Name: "state contents", Status: checkWarn,
			Detail: "the state file manages no resources",
			Fix:    "check you are pointing at the right workspace's state",
		})
	}
	return results
}

// checkLiveSnapshot validates the snapshot and, importantly, its age.
func checkLiveSnapshot() []checkResult {
	path := doctorLive
	if path == "" {
		path = driftLivePath
	}
	if path == "" {
		if _, err := os.Stat("live.json"); err == nil {
			path = "live.json"
		}
	}
	if path == "" {
		return []checkResult{{
			Name:   "live snapshot",
			Status: checkSkip,
			Detail: "none configured and no live.json in the working directory",
			Fix:    "infractl snapshot from-plan",
		}}
	}

	snapshot, err := readLiveSnapshot(path)
	if err != nil {
		return []checkResult{{
			Name: "live snapshot", Status: checkFail,
			Detail: err.Error(),
			Fix:    "infractl snapshot from-plan --out " + path,
		}}
	}

	results := []checkResult{{
		Name:   "live snapshot",
		Status: checkPass,
		Detail: fmt.Sprintf("%s: %d resources", path, len(snapshot.Resources)),
	}}

	// Snapshot age is the single most common cause of a confusing scan: drift
	// found against week-old data may have been reconciled days ago.
	if snapshot.CapturedAt.IsZero() {
		results = append(results, checkResult{
			Name: "snapshot age", Status: checkWarn,
			Detail: "no captured_at timestamp, so staleness cannot be judged",
			Fix:    "add captured_at, or regenerate with `infractl snapshot from-plan`",
		})
		return results
	}

	age := time.Since(snapshot.CapturedAt)
	switch {
	case age < 0:
		// A snapshot dated in the future is either a clock skew between the
		// machine that captured it and this one, or a hand-edited timestamp.
		// Either way the age cannot be judged, and reporting a negative
		// duration as though it were freshness would be worse than saying so.
		results = append(results, checkResult{
			Name: "snapshot age", Status: checkWarn,
			Detail: fmt.Sprintf("captured_at is %s in the future (%s)",
				ui.HumanDuration(age), snapshot.CapturedAt.Format(time.RFC3339)),
			Fix: "check the clock on the machine that captured it; staleness cannot be judged",
		})

	case age > 7*24*time.Hour:
		results = append(results, checkResult{
			Name: "snapshot age", Status: checkFail,
			Detail: fmt.Sprintf("captured %s ago", ui.HumanDuration(age)),
			Fix:    "infractl snapshot from-plan --out " + path,
		})
	case age > 24*time.Hour:
		results = append(results, checkResult{
			Name: "snapshot age", Status: checkWarn,
			Detail: fmt.Sprintf("captured %s ago; findings may already be reconciled", ui.HumanDuration(age)),
			Fix:    "infractl snapshot from-plan --out " + path,
		})
	default:
		results = append(results, checkResult{
			Name: "snapshot age", Status: checkPass,
			Detail: fmt.Sprintf("captured %s ago", ui.HumanDuration(age)),
		})
	}
	return results
}

// printDoctor renders the report for a terminal.
func printDoctor(report doctorReport) {
	symbols := rt.UI.Symbols()

	for _, check := range report.Checks {
		var mark string
		var style ui.Style

		switch check.Status {
		case checkPass:
			mark, style = symbols.Success, ui.StyleSuccess
		case checkWarn:
			mark, style = symbols.Warning, ui.StyleWarning
		case checkFail:
			mark, style = symbols.Failure, ui.StyleError
		default:
			mark, style = symbols.Pending, ui.StyleMuted
		}

		rt.UI.Printf("%s %-16s %s\n",
			rt.UI.Apply(style, mark), check.Name,
			rt.UI.Apply(ui.StyleMuted, check.Detail))

		if check.Fix != "" && check.Status != checkPass {
			rt.UI.Printf("  %s %s\n",
				rt.UI.Apply(ui.StyleMuted, symbols.Arrow),
				rt.UI.Apply(ui.StyleInfo, check.Fix))
		}
	}

	rt.UI.Println()
	if report.Failed == 0 && report.Warned == 0 {
		rt.UI.Printf("%s\n", rt.UI.Apply(ui.StyleSuccess, "Everything checks out."))
		return
	}
	rt.UI.Printf("%d passed, %d warning(s), %d failure(s)\n",
		report.Passed, report.Warned, report.Failed)
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().StringVar(&doctorState, "state", "", "state file to check")
	doctorCmd.Flags().StringVar(&doctorLive, "live", "", "live snapshot to check")
	doctorCmd.Flags().StringVar(&driftIgnorePath, "ignore-file", "", "ignore file to check")
}
