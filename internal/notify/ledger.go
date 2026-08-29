// Package notify turns drift findings into notifications that a person will
// still be reading in six weeks.
//
// The problem it solves is not delivery. It is that an alert which arrives
// every night for the same drifted bucket stops being read by week two, and
// then the one that mattered arrives into a channel nobody looks at. The
// on-call literature is consistent on the rule: if the recipient cannot take a
// specific action, the alert should not exist.
//
// So the pipeline is:
//
//	findings -> ledger -> events -> router -> sinks
//
// with one load-bearing distinction. A *finding* is a fact about
// infrastructure: this bucket is public. An *event* is a fact about a
// finding's history: this bucket became public since the last scan, or has
// been public for a fortnight, or is no longer public. Only the second is
// worth interrupting someone for, so sinks consume events and never findings.
//
// This package is strictly a consumer of scan results. It never changes a
// finding, a severity, an exit code, or the JSON and SARIF payloads.
package notify

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// State is where a finding sits in its lifecycle.
type State string

const (
	// StateOpen means the finding was present in the most recent scan.
	StateOpen State = "open"
	// StateResolved means it was present before and is now gone.
	StateResolved State = "resolved"
	// StateSuppressed means an ignore rule now covers it.
	StateSuppressed State = "suppressed"
	// StateSnoozed means a human deferred it until SnoozedUntil.
	StateSnoozed State = "snoozed"
)

// EventKind classifies what changed about a finding since the last scan.
type EventKind string

const (
	// EventNew is a finding with no prior record. This is the only class that
	// should ever page someone.
	EventNew EventKind = "new"
	// EventResolved is a finding that was open and is now absent.
	EventResolved EventKind = "resolved"
	// EventAging is a finding open for longer than the configured threshold.
	// It fires once per threshold crossing, not on every scan.
	EventAging EventKind = "aging"
)

// Finding is the subset of a drift finding this package needs.
//
// It is defined here rather than imported so that the notification layer does
// not pull the CLI's types into its dependency graph, and so the contract
// between them is explicit.
type Finding struct {
	Address string `json:"address"`
	Kind    string `json:"kind"`
	// Severity is carried for routing but deliberately excluded from the
	// fingerprint; see Fingerprint.
	Severity string `json:"severity"`
	// Dimension is the risk dimension, used by tier rules.
	Dimension string `json:"dimension,omitempty"`
	// ChangedPaths are the attribute paths that moved. Values are never
	// carried into this package.
	ChangedPaths []string `json:"changed_paths,omitempty"`
	// ResourceType is used for grouping and ownership fallbacks.
	ResourceType string `json:"resource_type,omitempty"`
}

// Fingerprint identifies the same finding across scans.
//
// Three things are deliberately excluded:
//
//   - Values. A bucket drifting further is the same problem, worse. It should
//     age, not re-alert as though it were new.
//   - Severity. Re-scoring a rule would otherwise resurrect every finding it
//     touches as new, which is how a scoring change becomes a pager storm.
//   - Timestamps. They would make every finding new on every scan.
//
// Paths are sorted so that map iteration order cannot change the identity of a
// finding, which would produce the same storm by accident.
//
// This is the same identity SARIF uses for GitHub's fixed and still-open
// tracking, so the two agree by construction rather than by coincidence.
func Fingerprint(f Finding) string {
	paths := make([]string, len(f.ChangedPaths))
	copy(paths, f.ChangedPaths)
	sort.Strings(paths)

	h := sha256.New()
	h.Write([]byte(f.Address))
	h.Write([]byte{0})
	h.Write([]byte(f.Kind))
	h.Write([]byte{0})
	h.Write([]byte(strings.Join(paths, "\x00")))

	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Entry is one finding's record in the ledger.
type Entry struct {
	Fingerprint string `json:"fingerprint"`
	Address     string `json:"address"`
	Kind        string `json:"kind"`
	Severity    string `json:"severity"`

	FirstSeen time.Time  `json:"first_seen"`
	LastSeen  time.Time  `json:"last_seen"`
	Notified  *time.Time `json:"notified_at,omitempty"`

	State State `json:"state"`
	// AgedAt records when an aging event was emitted, so it fires once per
	// threshold crossing rather than on every subsequent scan.
	AgedAt *time.Time `json:"aged_at,omitempty"`
	// SnoozedUntil defers notification without resolving the finding.
	SnoozedUntil *time.Time `json:"snoozed_until,omitempty"`
}

// Event is a change in a finding's history, and the unit sinks consume.
type Event struct {
	Kind    EventKind `json:"kind"`
	Finding Finding   `json:"finding"`
	Entry   Entry     `json:"entry"`
	// Age is how long the finding has been open. Zero for a new finding.
	Age time.Duration `json:"age"`
}

// Ledger is the durable record of every finding's history.
//
// It is JSONL rather than a database: diffable, appendable without a driver,
// legible in a merge conflict, and adding no cgo dependency to a binary that
// currently has none. Committing it to the repository makes drift history
// reviewable and gives the scan that updates it an audit trail for free.
type Ledger struct {
	path    string
	entries map[string]Entry
}

// DefaultLedgerPath is where the ledger lives when none is configured.
const DefaultLedgerPath = ".infractl/ledger.jsonl"

// LoadLedger reads a ledger, returning an empty one when the file is absent.
//
// A missing ledger is the normal first-run state, not an error. A malformed
// one is an error: silently starting over would report every existing finding
// as new, which is precisely the pager storm this package exists to prevent.
func LoadLedger(path string) (*Ledger, error) {
	if path == "" {
		path = DefaultLedgerPath
	}
	ledger := &Ledger{path: path, entries: map[string]Entry{}}

	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return ledger, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open ledger %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	// Entries are small, but a pathological line should fail loudly rather
	// than silently truncate the history.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}

		var entry Entry
		if err := json.Unmarshal([]byte(text), &entry); err != nil {
			return nil, fmt.Errorf("ledger %s line %d does not parse: %w", path, line, err)
		}
		if entry.Fingerprint == "" {
			return nil, fmt.Errorf("ledger %s line %d has no fingerprint", path, line)
		}
		ledger.entries[entry.Fingerprint] = entry
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read ledger %s: %w", path, err)
	}

	return ledger, nil
}

// Options configure how events are derived.
type Options struct {
	// AgingAfter is how long a finding must stay open before it ages.
	AgingAfter time.Duration
	// Now overrides the clock, for tests.
	Now time.Time
}

// DefaultAgingAfter matches the weekly review cadence most teams run.
const DefaultAgingAfter = 7 * 24 * time.Hour

// Reconcile folds a scan's findings into the ledger and returns the events.
//
// It does not write. The caller decides whether to persist, which keeps a
// dry run from mutating history.
func (l *Ledger) Reconcile(findings []Finding, opts Options) []Event {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	aging := opts.AgingAfter
	if aging <= 0 {
		aging = DefaultAgingAfter
	}

	var events []Event
	seen := make(map[string]struct{}, len(findings))

	for _, finding := range findings {
		fp := Fingerprint(finding)
		seen[fp] = struct{}{}

		entry, known := l.entries[fp]
		if !known {
			entry = Entry{
				Fingerprint: fp,
				Address:     finding.Address,
				Kind:        finding.Kind,
				Severity:    finding.Severity,
				FirstSeen:   now,
				LastSeen:    now,
				State:       StateOpen,
			}
			l.entries[fp] = entry

			events = append(events, Event{Kind: EventNew, Finding: finding, Entry: entry})
			continue
		}

		// A finding that was resolved and has come back is new again: the
		// intervening fix did not hold, which is worth saying out loud.
		reopened := entry.State == StateResolved

		entry.LastSeen = now
		entry.Severity = finding.Severity
		entry.State = StateOpen
		if reopened {
			entry.FirstSeen = now
			entry.AgedAt = nil
		}
		l.entries[fp] = entry

		if reopened {
			events = append(events, Event{Kind: EventNew, Finding: finding, Entry: entry})
			continue
		}

		// Aging fires once per crossing. Without AgedAt it would fire on every
		// scan after the threshold, which is the same noise in a new costume.
		age := now.Sub(entry.FirstSeen)
		if age >= aging && entry.AgedAt == nil {
			entry.AgedAt = &now
			l.entries[fp] = entry
			events = append(events, Event{Kind: EventAging, Finding: finding, Entry: entry, Age: age})
		}
	}

	// Anything open that this scan did not see has been resolved.
	for fp, entry := range l.entries {
		if _, present := seen[fp]; present {
			continue
		}
		if entry.State != StateOpen {
			continue
		}

		entry.State = StateResolved
		entry.LastSeen = now
		l.entries[fp] = entry

		events = append(events, Event{
			Kind:    EventResolved,
			Finding: Finding{Address: entry.Address, Kind: entry.Kind, Severity: entry.Severity},
			Entry:   entry,
			Age:     now.Sub(entry.FirstSeen),
		})
	}

	// Deterministic order, so two runs over identical input produce identical
	// output and the ledger diff stays reviewable.
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Kind != events[j].Kind {
			return eventRank(events[i].Kind) < eventRank(events[j].Kind)
		}
		return events[i].Entry.Fingerprint < events[j].Entry.Fingerprint
	})

	return events
}

// eventRank orders event kinds for display: what is new first, what is fixed
// last.
func eventRank(kind EventKind) int {
	switch kind {
	case EventNew:
		return 0
	case EventAging:
		return 1
	case EventResolved:
		return 2
	default:
		return 3
	}
}

// MarkNotified records that an event was delivered, so the re-notify window
// can be enforced on the next run.
func (l *Ledger) MarkNotified(fingerprint string, at time.Time) {
	entry, known := l.entries[fingerprint]
	if !known {
		return
	}
	entry.Notified = &at
	l.entries[fingerprint] = entry
}

// Entries returns every record, ordered by fingerprint.
func (l *Ledger) Entries() []Entry {
	entries := make([]Entry, 0, len(l.entries))
	for _, entry := range l.entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Fingerprint < entries[j].Fingerprint
	})
	return entries
}

// Lookup returns one entry by fingerprint.
func (l *Ledger) Lookup(fingerprint string) (Entry, bool) {
	entry, ok := l.entries[fingerprint]
	return entry, ok
}

// Len returns the number of records held.
func (l *Ledger) Len() int { return len(l.entries) }

// Save writes the ledger back to disk.
//
// The file is rewritten in fingerprint order rather than appended to, so that
// a committed ledger produces a minimal, reviewable diff instead of a growing
// append log whose history is impossible to read.
func (l *Ledger) Save() error {
	if dir := filepath.Dir(l.path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create ledger directory %s: %w", dir, err)
		}
	}

	var buf strings.Builder
	for _, entry := range l.Entries() {
		encoded, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("encode ledger entry %s: %w", entry.Fingerprint, err)
		}
		buf.Write(encoded)
		buf.WriteByte('\n')
	}

	// Written through a temporary file and renamed, so an interrupted write
	// cannot leave a truncated ledger that the next run would reject.
	tmp := l.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(buf.String()), 0o600); err != nil {
		return fmt.Errorf("write ledger %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, l.path); err != nil {
		return fmt.Errorf("replace ledger %s: %w", l.path, err)
	}
	return nil
}

// Path returns where the ledger is stored.
func (l *Ledger) Path() string { return l.path }
