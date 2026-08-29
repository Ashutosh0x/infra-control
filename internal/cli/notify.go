package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ashutosh0x/infra-control/internal/notify"
	"github.com/ashutosh0x/infra-control/internal/ui"
	"github.com/spf13/cobra"
)

var (
	notifyStatePath string
	notifyLivePath  string
	notifyLedger    string
	notifySink      string
	notifyChannel   string
	notifyWebhook   string
	notifyOwnerTags string
	notifyOwnerDflt string
	notifyCodeowner string
	notifyAgingDays int
	notifyRenotifyD int
	notifyNoSave    bool
)

var notifyCmd = &cobra.Command{
	Use:     "notify",
	Short:   "Turn drift findings into notifications people will still read",
	GroupID: "analyse",
	Long: `Compare a scan against the recorded history of previous scans and report
only what changed.

The difference from ` + "`drift scan`" + ` is what gets said. A scan reports every
finding, every time. This reports what is new since last time, what has just
been fixed, and what has been open long enough to need a decision. A finding
that is merely still true is not news, and repeating it nightly is how a
channel stops being read.

State lives in a ledger, by default .infractl/ledger.jsonl. Commit it: drift
history becomes reviewable and diffable, and the scan that updates it leaves
its own audit trail.`,
	Example: `  # See what changed, without sending anything
  infractl notify --state terraform.tfstate --live live.json

  # Post to Slack
  infractl notify --state terraform.tfstate --live live.json \
    --sink slack --channel '#infra-drift'

  # In CI, recording history for the next run
  infractl notify --state terraform.tfstate --live live.json --sink webhook \
    --webhook-url "$DRIFT_WEBHOOK"`,
	Args: cobra.NoArgs,
	RunE: runNotify,
}

// notifyReport is the machine-readable result.
type notifyReport struct {
	LedgerPath string            `json:"ledger"`
	Scanned    int               `json:"scanned"`
	Events     []notifyEventView `json:"events"`
	Delivered  int               `json:"delivered"`
	Suppressed int               `json:"suppressed"`
	Unowned    int               `json:"unowned"`
	Groups     []notifyGroupView `json:"groups"`
}

type notifyEventView struct {
	Kind       string `json:"kind"`
	Address    string `json:"address"`
	Severity   string `json:"severity"`
	Tier       string `json:"tier"`
	Owner      string `json:"owner,omitempty"`
	OwnerVia   string `json:"owner_via,omitempty"`
	Suppressed string `json:"suppressed,omitempty"`
}

type notifyGroupView struct {
	Tier  string `json:"tier"`
	Title string `json:"title"`
	Count int    `json:"count"`
	Owner string `json:"owner,omitempty"`
}

func runNotify(cmd *cobra.Command, _ []string) error {
	if err := requireFile(notifyStatePath, "state file (--state)"); err != nil {
		return err
	}
	if err := requireFile(notifyLivePath, "live snapshot (--live)"); err != nil {
		return err
	}

	// The scan is run through the same code path a plain `drift scan` uses, so
	// notifications can never describe findings the scan would not produce.
	findings, tags, err := scanForNotify()
	if err != nil {
		return err
	}

	ledgerPath := notifyLedger
	if ledgerPath == "" {
		ledgerPath = notify.DefaultLedgerPath
	}

	ledger, err := notify.LoadLedger(ledgerPath)
	if err != nil {
		return failf(ExitError, "%w", err)
	}

	now := time.Now().UTC()
	events := ledger.Reconcile(findings, notify.Options{
		Now:        now,
		AgingAfter: time.Duration(notifyAgingDays) * 24 * time.Hour,
	})

	resolver, err := notify.NewResolver(notify.OwnershipConfig{
		TagKeys:        splitList(notifyOwnerTags),
		CodeownersPath: notifyCodeowner,
		Default:        notifyOwnerDflt,
	}, tags)
	if err != nil {
		return failf(ExitConfig, "%w", err)
	}

	router := notify.NewRouter(notify.RouterConfig{
		RenotifyAfter: time.Duration(notifyRenotifyD) * 24 * time.Hour,
		Tiers:         notify.DefaultRouterConfig().Tiers,
	}, resolver)

	decisions := router.Route(events, now)
	groups := notify.GroupDecisions(decisions)

	report := buildNotifyReport(ledgerPath, len(findings), decisions, groups)

	sink, err := buildSink()
	if err != nil {
		return err
	}

	// Delivery happens before the ledger is written, so a send that fails
	// leaves the event unrecorded and the next run will try again rather than
	// silently dropping it.
	var delivered int
	if sink != nil {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		for _, group := range groups {
			notification := notify.Redact(group)
			if err := sink.Send(ctx, notification); err != nil {
				return failf(ExitUnavailable, "deliver to %s: %w", sink.Name(), err)
			}
			for _, decision := range group.Decisions {
				ledger.MarkNotified(decision.Event.Entry.Fingerprint, now)
				delivered++
			}
		}
	}

	if !notifyNoSave {
		if err := ledger.Save(); err != nil {
			return failf(ExitError, "%w", err)
		}
	}

	if rt.Format.IsMachine() {
		return rt.write(ui.View{Data: report})
	}

	printNotifyReport(report, sink != nil, delivered)

	if !rt.Format.IsMachine() && report.Unowned > 0 {
		rt.UI.Warn("%d finding(s) have no owner; an alert with no owner has no responder.", report.Unowned)
		rt.UI.Detail("Add an owner tag, a CODEOWNERS entry, or set --owner-default.")
	}
	return nil
}

// scanForNotify runs a drift scan and returns findings in the notify shape.
//
// Values never cross this boundary: the notify package's Finding type carries
// attribute paths only, which makes leaking a value into a notification a
// compile error rather than a review question.
func scanForNotify() ([]notify.Finding, map[string]map[string]string, error) {
	previousState, previousLive := driftStatePath, driftLivePath
	driftStatePath, driftLivePath = notifyStatePath, notifyLivePath
	defer func() { driftStatePath, driftLivePath = previousState, previousLive }()

	state, snapshot, findings, err := collectDrift()
	if err != nil {
		return nil, nil, err
	}

	out := make([]notify.Finding, 0, len(findings))
	for _, finding := range findings {
		paths := make([]string, 0, len(finding.Changes))
		for _, change := range finding.Changes {
			paths = append(paths, change.Path)
		}
		sort.Strings(paths)

		out = append(out, notify.Finding{
			Address:      finding.Address,
			Kind:         finding.Kind,
			Severity:     string(finding.Severity),
			Dimension:    dimensionFor(finding),
			ChangedPaths: paths,
			ResourceType: finding.Type,
		})
	}

	// Tags come from state rather than the snapshot, because state is what the
	// team declared and is therefore the ownership they intended.
	tags := map[string]map[string]string{}
	for _, resource := range state.ManagedResources() {
		if extracted := extractTags(resource.Attributes); len(extracted) > 0 {
			tags[resource.Address] = extracted
		}
	}
	_ = snapshot

	return out, tags, nil
}

// dimensionFor classifies a finding for tier routing.
//
// Anything touching a security boundary is security; the rest is reliability.
// This is coarse on purpose: the tier rules only distinguish security from
// everything else, and inventing finer categories the router cannot use would
// be a false precision.
func dimensionFor(finding driftFinding) string {
	for _, change := range finding.Changes {
		if matchesAny(strings.ToLower(change.Path), criticalPatterns) {
			return "security"
		}
	}
	if finding.Kind == "unmanaged" {
		return "security"
	}
	return "reliability"
}

// buildSink constructs the configured destination.
func buildSink() (notify.Sink, error) {
	switch strings.ToLower(strings.TrimSpace(notifySink)) {
	case "", "none":
		// No sink is the default. `notify` then reports what it would send,
		// which is the safe thing for a first run and for a dry run in CI.
		return nil, nil

	case "stdout":
		return notify.StdoutSink{Out: rt.UI.Out()}, nil

	case "slack":
		token := os.Getenv("SLACK_BOT_TOKEN")
		if token == "" {
			return nil, failf(ExitConfig,
				"the slack sink needs SLACK_BOT_TOKEN.\n"+
					"  The app needs the chat:write scope and nothing else.")
		}
		if notifyChannel == "" {
			return nil, failf(ExitConfig, "the slack sink needs --channel")
		}
		return notify.SlackSink{Token: token, Channel: notifyChannel}, nil

	case "webhook":
		if notifyWebhook == "" {
			return nil, failf(ExitConfig, "the webhook sink needs --webhook-url")
		}
		// The secret is optional but its absence is worth saying: an
		// unsigned webhook gives the receiver no way to tell this tool from
		// anyone who learned the URL.
		secret := os.Getenv("INFRACTL_WEBHOOK_SECRET")
		if secret == "" {
			rt.UI.Warn("INFRACTL_WEBHOOK_SECRET is not set; the payload will be unsigned.")
		}
		return notify.WebhookSink{URL: notifyWebhook, Secret: secret}, nil

	case "github":
		token := os.Getenv("GITHUB_TOKEN")
		repo := os.Getenv("GITHUB_REPOSITORY")
		if token == "" || repo == "" {
			return nil, failf(ExitConfig,
				"the github sink needs GITHUB_TOKEN and GITHUB_REPOSITORY")
		}
		pr := githubPRNumber()
		if pr == 0 {
			return nil, failf(ExitConfig,
				"the github sink needs a pull request number; set INFRACTL_GITHUB_PR")
		}
		return notify.GitHubSink{Token: token, Repo: repo, PR: pr}, nil

	case "teams", "discord", "pagerduty":
		return nil, failf(ExitError,
			"the %s sink is not implemented in this build.\n"+
				"  The interface exists; the delivery does not, and shipping an unwired\n"+
				"  sink would report success for a notification nobody received.", notifySink)

	default:
		return nil, failf(ExitUsage,
			"unknown sink %q (want stdout, slack, webhook, or github)", notifySink)
	}
}

// githubPRNumber reads the pull request number from the environment.
func githubPRNumber() int {
	var pr int
	if _, err := fmt.Sscanf(os.Getenv("INFRACTL_GITHUB_PR"), "%d", &pr); err == nil {
		return pr
	}
	return 0
}

// buildNotifyReport assembles the machine-readable payload.
func buildNotifyReport(ledgerPath string, scanned int, decisions []notify.Decision, groups []notify.Group) notifyReport {
	report := notifyReport{LedgerPath: ledgerPath, Scanned: scanned}

	for _, decision := range decisions {
		view := notifyEventView{
			Kind:       string(decision.Event.Kind),
			Address:    decision.Event.Finding.Address,
			Severity:   decision.Event.Finding.Severity,
			Tier:       string(decision.Tier),
			Owner:      decision.Owner.Name,
			OwnerVia:   decision.Owner.Source,
			Suppressed: decision.Suppressed,
		}
		report.Events = append(report.Events, view)

		if decision.Delivered() {
			report.Delivered++
		} else {
			report.Suppressed++
		}
		if !decision.Owner.Known() {
			report.Unowned++
		}
	}

	for _, group := range groups {
		notification := notify.Redact(group)
		report.Groups = append(report.Groups, notifyGroupView{
			Tier:  string(group.Tier),
			Title: notification.Title,
			Count: len(group.Decisions),
			Owner: group.Owner.Name,
		})
	}
	return report
}

// printNotifyReport renders the result for a terminal.
func printNotifyReport(report notifyReport, hasSink bool, delivered int) {
	if len(report.Events) == 0 {
		rt.UI.Printf("%s\n", rt.UI.Apply(ui.StyleMuted,
			fmt.Sprintf("Nothing changed since the last scan. %d findings, all already recorded.",
				report.Scanned)))
		return
	}

	table := ui.NewTable(
		ui.Column{Title: "EVENT", MinWidth: 8},
		ui.Column{Title: "TIER", MinWidth: 7},
		ui.Column{Title: "SEVERITY", MinWidth: 8},
		ui.Column{Title: "ADDRESS", MinWidth: 22, Truncatable: true},
		ui.Column{Title: "OWNER", MinWidth: 10, Truncatable: true},
	)

	for _, event := range report.Events {
		owner := event.Owner
		if owner == "" {
			owner = "unowned"
		}
		row := []ui.Cell{
			ui.Styled(event.Kind, eventStyle(event.Kind)),
			ui.Text(event.Tier),
			ui.Styled(strings.ToUpper(event.Severity), ui.SeverityStyle(event.Severity)),
			ui.Text(event.Address),
			ui.Text(owner),
		}
		if event.Suppressed != "" {
			row[0] = ui.Styled(event.Kind+" (held)", ui.StyleMuted)
		}
		table.Row(row...)
	}

	rt.UI.Raw(rt.UI.Render(table))
	rt.UI.Println()

	if hasSink {
		rt.UI.Success("Delivered %d finding(s) in %d message(s)", delivered, len(report.Groups))
	} else {
		rt.UI.Printf("%s\n", rt.UI.Apply(ui.StyleMuted,
			fmt.Sprintf("%d would be delivered in %d message(s); no sink configured.",
				report.Delivered, len(report.Groups))))
	}
	if report.Suppressed > 0 {
		rt.UI.Printf("%s\n", rt.UI.Apply(ui.StyleMuted,
			fmt.Sprintf("%d held back by the re-notify window or a snooze.", report.Suppressed)))
	}
}

// eventStyle colours an event kind by what it means.
func eventStyle(kind string) ui.Style {
	switch notify.EventKind(kind) {
	case notify.EventNew:
		return ui.StyleError
	case notify.EventAging:
		return ui.StyleWarning
	case notify.EventResolved:
		return ui.StyleSuccess
	default:
		return ui.StyleNone
	}
}

func init() {
	rootCmd.AddCommand(notifyCmd)

	f := notifyCmd.Flags()
	f.StringVar(&notifyStatePath, "state", "", "path to the state file (required)")
	f.StringVar(&notifyLivePath, "live", "", "path to the live snapshot (required)")
	f.StringVar(&notifyLedger, "ledger", "", "where scan history is recorded (default .infractl/ledger.jsonl)")
	f.StringVar(&notifySink, "sink", "", "where to deliver: stdout, slack, webhook, github (default: report only)")
	f.StringVar(&notifyChannel, "channel", "", "Slack channel for the slack sink")
	f.StringVar(&notifyWebhook, "webhook-url", "", "endpoint for the webhook sink")
	f.StringVar(&notifyOwnerTags, "owner-tags", "owner,team", "resource tag keys checked for an owner, in order")
	f.StringVar(&notifyOwnerDflt, "owner-default", "", "owner for findings nothing else matches")
	f.StringVar(&notifyCodeowner, "codeowners", "", "CODEOWNERS file to match module paths against")
	f.IntVar(&notifyAgingDays, "aging-days", 7, "days a finding stays open before it is reported as aging")
	f.IntVar(&notifyRenotifyD, "renotify-days", 7, "days before the same finding may be reported again")
	f.BoolVar(&notifyNoSave, "no-save", false, "do not write the ledger; use to preview without recording history")

	_ = notifyCmd.MarkFlagRequired("state")
	_ = notifyCmd.MarkFlagRequired("live")
	_ = notifyCmd.RegisterFlagCompletionFunc("sink",
		cobra.FixedCompletions([]string{"stdout", "slack", "webhook", "github"}, cobra.ShellCompDirectiveNoFileComp))
}
