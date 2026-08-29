package notify

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Sinks are the egress boundary, and are treated as one.
//
// A notification leaves the machine and lands somewhere with a different
// audience from a terminal: a channel with contractors in it, a webhook whose
// far end is somebody else's log aggregator. The scan output being safe does
// not make the notification safe, so redaction happens again here.

// Notification is what a sink delivers.
type Notification struct {
	// Title is one line stating what happened.
	Title string `json:"title"`
	// Summary is a short paragraph of context.
	Summary string `json:"summary"`
	// Tier is the urgency, which sinks may map onto their own concepts.
	Tier Tier `json:"tier"`
	// Owner is who should act, when known.
	Owner Owner `json:"owner"`
	// Items are the findings this notification covers.
	Items []NotificationItem `json:"items"`
	// Actions are commands the recipient can run. Never executed here.
	Actions []Action `json:"actions,omitempty"`
}

// NotificationItem is one finding, already redacted.
type NotificationItem struct {
	Address  string `json:"address"`
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	// Paths are the attribute paths that changed. Values are absent by
	// design; see Redact.
	Paths []string `json:"paths,omitempty"`
	// Age is how long the finding has been open, rendered for humans.
	Age string `json:"age,omitempty"`
	// Module is the declaring module, when it could be determined.
	Module string `json:"module,omitempty"`
}

// Action is a suggested next step.
type Action struct {
	Label   string `json:"label"`
	Command string `json:"command"`
}

// Sink delivers notifications.
type Sink interface {
	// Name identifies the sink in configuration and audit output.
	Name() string
	// Send delivers a notification. It must not retry indefinitely; the caller
	// decides retry policy.
	Send(ctx context.Context, n Notification) error
}

// Redact prepares a group for delivery.
//
// There is deliberately no option to include attribute values. The Finding
// type this package consumes carries attribute *paths* only, so a value cannot
// reach a notification even by mistake: leaking one would be a compile error
// rather than a review question. A flag permitting values would be a promise
// the type system already refuses to keep.
//
// What remains is sanitising every cloud-derived string. Resource names and
// tag values come from an account that may hold resources an attacker created,
// so a tag reading "<!channel> urgent", or one containing a link, must never
// render as one. This is the same shape of problem as prompt injection and has
// the same defence: treat the data as data.
func Redact(group Group) Notification {
	items := make([]NotificationItem, 0, len(group.Decisions))

	for _, decision := range group.Decisions {
		finding := decision.Event.Finding

		item := NotificationItem{
			Address:  Sanitise(finding.Address),
			Kind:     Sanitise(finding.Kind),
			Severity: Sanitise(finding.Severity),
			Module:   Sanitise(ModulePath(finding.Address)),
		}

		// Paths are structural, not user data, but they still pass through
		// sanitisation because a provider may permit arbitrary map keys.
		for _, path := range finding.ChangedPaths {
			item.Paths = append(item.Paths, Sanitise(path))
		}

		if decision.Event.Age > 0 {
			item.Age = humanAge(decision.Event.Age)
		}
		items = append(items, item)
	}

	kind := "finding"
	if len(group.Decisions) > 0 {
		kind = string(group.Decisions[0].Event.Kind)
	}

	return Notification{
		Title:   titleFor(kind, group),
		Summary: summaryFor(kind, group),
		Tier:    group.Tier,
		Owner:   Owner{Name: Sanitise(group.Owner.Name), Source: group.Owner.Source},
		Items:   items,
		Actions: actionsFor(group),
	}
}

// Sanitise makes a cloud-derived string safe to render.
//
// Control characters are removed because they can rewrite a terminal line or
// break a JSON payload. The characters platforms treat as markup are escaped
// so that a resource named after a mention does not become one.
func Sanitise(s string) string {
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteByte(' ')
		case unicode.IsControl(r):
			// Dropped entirely: there is no legitimate resource name
			// containing an escape sequence.
		case r == '<':
			b.WriteString("&lt;")
		case r == '>':
			b.WriteString("&gt;")
		case r == '&':
			b.WriteString("&amp;")
		default:
			b.WriteRune(r)
		}
	}

	// A very long name is a denial-of-service on a channel, not information.
	const maxLength = 256
	out := b.String()
	if len(out) > maxLength {
		return out[:maxLength] + "..."
	}
	return out
}

// titleFor writes the one-line headline.
func titleFor(kind string, group Group) string {
	count := len(group.Decisions)
	noun := "finding"
	if count != 1 {
		noun = "findings"
	}

	switch EventKind(kind) {
	case EventNew:
		return fmt.Sprintf("%d new drift %s", count, noun)
	case EventResolved:
		return fmt.Sprintf("%d drift %s resolved", count, noun)
	case EventAging:
		return fmt.Sprintf("%d drift %s still open", count, noun)
	default:
		return fmt.Sprintf("%d drift %s", count, noun)
	}
}

// summaryFor writes the context paragraph.
func summaryFor(kind string, group Group) string {
	var b strings.Builder

	switch EventKind(kind) {
	case EventNew:
		b.WriteString("Infrastructure changed outside Terraform since the last scan.")
	case EventResolved:
		b.WriteString("These no longer differ from Terraform state.")
	case EventAging:
		b.WriteString("These have been open long enough to need a decision: fix, accept, or suppress with a reason.")
	}

	if group.Owner.Known() {
		fmt.Fprintf(&b, " Owner: %s (%s).", Sanitise(group.Owner.Name), group.Owner.Source)
	} else {
		b.WriteString(" No owner could be determined; add an owner tag or a CODEOWNERS entry.")
	}
	return b.String()
}

// actionsFor suggests next steps.
//
// These are commands to run, never buttons that act. The tool proposes and a
// human applies; the blast radius is printed beside the proposal.
func actionsFor(group Group) []Action {
	if len(group.Decisions) == 0 {
		return nil
	}
	first := group.Decisions[0].Event.Finding

	if EventKind(group.Decisions[0].Event.Kind) == EventResolved {
		return nil
	}

	return []Action{
		{Label: "See the diff", Command: "infractl drift scan --show-diff"},
		{Label: "What breaks", Command: fmt.Sprintf("infractl graph blast-radius %q", first.Address)},
		{Label: "How to resolve", Command: "infractl drift scan --fix"},
		{Label: "If expected", Command: fmt.Sprintf(
			"infractl ignore add %q --reason \"...\"", first.Address)},
	}
}

// humanAge renders a duration for a notification.
func humanAge(d time.Duration) string {
	days := int(d.Hours() / 24)
	switch {
	case days >= 30:
		return fmt.Sprintf("%d months", days/30)
	case days >= 7:
		return fmt.Sprintf("%d weeks", days/7)
	case days >= 1:
		return fmt.Sprintf("%d days", days)
	default:
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
}
