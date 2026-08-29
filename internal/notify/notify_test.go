package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---- Fingerprint -----------------------------------------------------------

func TestFingerprintIgnoresPathOrder(t *testing.T) {
	// Map iteration order is randomised in Go. If it reached the fingerprint,
	// every finding would look new on some runs, which is the pager storm this
	// package exists to prevent.
	a := Finding{Address: "aws_s3_bucket.assets", Kind: "modified",
		ChangedPaths: []string{"acl", "encryption", "tags.env"}}
	b := Finding{Address: "aws_s3_bucket.assets", Kind: "modified",
		ChangedPaths: []string{"tags.env", "acl", "encryption"}}

	if Fingerprint(a) != Fingerprint(b) {
		t.Error("path order changed the fingerprint")
	}
}

func TestFingerprintIgnoresSeverity(t *testing.T) {
	// Re-scoring a rule must not resurrect every finding it touches as new.
	low := Finding{Address: "a.b", Kind: "modified", Severity: "low", ChangedPaths: []string{"acl"}}
	high := Finding{Address: "a.b", Kind: "modified", Severity: "critical", ChangedPaths: []string{"acl"}}

	if Fingerprint(low) != Fingerprint(high) {
		t.Error("severity changed the fingerprint; re-scoring would re-alert everything")
	}
}

func TestFingerprintDistinguishesRealDifferences(t *testing.T) {
	base := Finding{Address: "a.b", Kind: "modified", ChangedPaths: []string{"acl"}}

	cases := map[string]Finding{
		"different address":   {Address: "a.c", Kind: "modified", ChangedPaths: []string{"acl"}},
		"different kind":      {Address: "a.b", Kind: "unmanaged", ChangedPaths: []string{"acl"}},
		"different attribute": {Address: "a.b", Kind: "modified", ChangedPaths: []string{"policy"}},
		"extra attribute":     {Address: "a.b", Kind: "modified", ChangedPaths: []string{"acl", "policy"}},
	}
	for name, other := range cases {
		if Fingerprint(base) == Fingerprint(other) {
			t.Errorf("%s produced the same fingerprint", name)
		}
	}
}

// ---- Ledger ----------------------------------------------------------------

func newLedger(t *testing.T) *Ledger {
	t.Helper()
	ledger, err := LoadLedger(filepath.Join(t.TempDir(), "ledger.jsonl"))
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	return ledger
}

func TestFirstScanMakesEverythingNew(t *testing.T) {
	ledger := newLedger(t)
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)

	events := ledger.Reconcile([]Finding{
		{Address: "a.b", Kind: "modified", Severity: "high", ChangedPaths: []string{"acl"}},
		{Address: "c.d", Kind: "unmanaged", Severity: "medium"},
	}, Options{Now: now})

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	for _, event := range events {
		if event.Kind != EventNew {
			t.Errorf("%s: kind = %s, want new", event.Finding.Address, event.Kind)
		}
	}
}

func TestUnchangedFindingProducesNoEvent(t *testing.T) {
	// This is the whole point of the ledger: a finding that is still true is
	// not news, and re-announcing it is what empties a channel of readers.
	ledger := newLedger(t)
	findings := []Finding{{Address: "a.b", Kind: "modified", Severity: "high", ChangedPaths: []string{"acl"}}}
	day := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)

	ledger.Reconcile(findings, Options{Now: day})
	events := ledger.Reconcile(findings, Options{Now: day.Add(24 * time.Hour)})

	if len(events) != 0 {
		t.Errorf("a still-true finding produced %d events: %+v", len(events), events)
	}
}

func TestDisappearedFindingResolves(t *testing.T) {
	ledger := newLedger(t)
	day := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)

	ledger.Reconcile([]Finding{{Address: "a.b", Kind: "modified", ChangedPaths: []string{"acl"}}},
		Options{Now: day})
	events := ledger.Reconcile(nil, Options{Now: day.Add(time.Hour)})

	if len(events) != 1 || events[0].Kind != EventResolved {
		t.Fatalf("expected one resolved event, got %+v", events)
	}
}

func TestReopenedFindingIsNewAgain(t *testing.T) {
	// A fix that did not hold is worth saying out loud, and its age should
	// restart rather than count from the original occurrence.
	ledger := newLedger(t)
	finding := Finding{Address: "a.b", Kind: "modified", ChangedPaths: []string{"acl"}}
	day := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)

	ledger.Reconcile([]Finding{finding}, Options{Now: day})
	ledger.Reconcile(nil, Options{Now: day.Add(time.Hour)})
	events := ledger.Reconcile([]Finding{finding}, Options{Now: day.Add(2 * time.Hour)})

	if len(events) != 1 || events[0].Kind != EventNew {
		t.Fatalf("a reopened finding should be new again, got %+v", events)
	}
	if entry, _ := ledger.Lookup(Fingerprint(finding)); !entry.FirstSeen.Equal(day.Add(2 * time.Hour)) {
		t.Errorf("FirstSeen should restart on reopen, got %s", entry.FirstSeen)
	}
}

func TestAgingFiresOnceNotEveryScan(t *testing.T) {
	// Firing on every scan past the threshold is the same noise in a new
	// costume, and is the specific bug this test exists to prevent.
	ledger := newLedger(t)
	finding := Finding{Address: "a.b", Kind: "modified", ChangedPaths: []string{"acl"}}
	day := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	opts := func(at time.Time) Options { return Options{Now: at, AgingAfter: 7 * 24 * time.Hour} }

	ledger.Reconcile([]Finding{finding}, opts(day))

	aged := ledger.Reconcile([]Finding{finding}, opts(day.Add(8*24*time.Hour)))
	if len(aged) != 1 || aged[0].Kind != EventAging {
		t.Fatalf("expected one aging event at the threshold, got %+v", aged)
	}

	again := ledger.Reconcile([]Finding{finding}, opts(day.Add(9*24*time.Hour)))
	if len(again) != 0 {
		t.Errorf("aging fired a second time: %+v", again)
	}
}

func TestLedgerRoundTripsThroughDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	finding := Finding{Address: "a.b", Kind: "modified", Severity: "high", ChangedPaths: []string{"acl"}}
	day := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)

	first, err := LoadLedger(path)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	first.Reconcile([]Finding{finding}, Options{Now: day})
	if err := first.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// A reloaded ledger must recognise the finding, or every CI run would
	// report everything as new.
	second, reloadErr := LoadLedger(path)
	if reloadErr != nil {
		t.Fatalf("reload: %v", reloadErr)
	}
	events := second.Reconcile([]Finding{finding}, Options{Now: day.Add(time.Hour)})
	if len(events) != 0 {
		t.Errorf("reloaded ledger did not recognise the finding: %+v", events)
	}
}

func TestMalformedLedgerIsAnError(t *testing.T) {
	// Silently starting over would report every existing finding as new.
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	if err := os.WriteFile(path, []byte("{not json\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadLedger(path); err == nil {
		t.Error("a malformed ledger should error rather than silently reset")
	}
}

func TestMissingLedgerIsNotAnError(t *testing.T) {
	ledger, err := LoadLedger(filepath.Join(t.TempDir(), "absent.jsonl"))
	if err != nil {
		t.Fatalf("a missing ledger is the normal first run: %v", err)
	}
	if ledger.Len() != 0 {
		t.Errorf("expected an empty ledger, got %d entries", ledger.Len())
	}
}

// ---- Router ----------------------------------------------------------------

func TestOnlyNewCriticalSecurityPages(t *testing.T) {
	router := NewRouter(DefaultRouterConfig(), nil)
	now := time.Now().UTC()

	cases := []struct {
		name  string
		event Event
		want  Tier
	}{
		{"new critical security", Event{Kind: EventNew, Finding: Finding{
			Severity: "critical", Dimension: "security"}}, TierPage},
		{"new critical cost", Event{Kind: EventNew, Finding: Finding{
			Severity: "critical", Dimension: "cost"}}, TierNotify},
		{"new high security", Event{Kind: EventNew, Finding: Finding{
			Severity: "high", Dimension: "security"}}, TierNotify},
		{"new low", Event{Kind: EventNew, Finding: Finding{Severity: "low"}}, TierDigest},
		{"aging critical security", Event{Kind: EventAging, Finding: Finding{
			Severity: "critical", Dimension: "security"}}, TierDigest},
		{"resolved critical", Event{Kind: EventResolved, Finding: Finding{
			Severity: "critical", Dimension: "security"}}, TierDigest},
	}

	for _, tc := range cases {
		decisions := router.Route([]Event{tc.event}, now)
		if decisions[0].Tier != tc.want {
			t.Errorf("%s: tier = %s, want %s", tc.name, decisions[0].Tier, tc.want)
		}
	}
}

func TestRenotifyWindowSuppressesRepeats(t *testing.T) {
	router := NewRouter(RouterConfig{
		RenotifyAfter: 7 * 24 * time.Hour,
		Tiers:         DefaultRouterConfig().Tiers,
	}, nil)

	now := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	recent := now.Add(-24 * time.Hour)

	event := Event{
		Kind:    EventAging,
		Finding: Finding{Address: "a.b", Severity: "high"},
		Entry:   Entry{Fingerprint: "f1", State: StateOpen, Notified: &recent},
	}

	decision := router.Route([]Event{event}, now)[0]
	if decision.Delivered() {
		t.Error("an event notified a day ago should be inside the seven-day window")
	}
	if !strings.Contains(decision.Suppressed, "re-notify window") {
		t.Errorf("suppression should name the reason, got %q", decision.Suppressed)
	}
}

func TestNewEventIgnoresRenotifyWindow(t *testing.T) {
	// A new finding has not been said before, so the window cannot apply.
	router := NewRouter(DefaultRouterConfig(), nil)
	now := time.Now().UTC()
	recent := now.Add(-time.Hour)

	event := Event{
		Kind:    EventNew,
		Finding: Finding{Address: "a.b", Severity: "critical", Dimension: "security"},
		Entry:   Entry{Fingerprint: "f1", State: StateOpen, Notified: &recent},
	}

	if !router.Route([]Event{event}, now)[0].Delivered() {
		t.Error("a new finding must always be delivered")
	}
}

func TestResolvedIsNeverSuppressed(t *testing.T) {
	// It is the message that closes a loop someone is waiting on.
	router := NewRouter(DefaultRouterConfig(), nil)
	now := time.Now().UTC()
	recent := now.Add(-time.Minute)

	event := Event{
		Kind:    EventResolved,
		Finding: Finding{Address: "a.b", Severity: "high"},
		Entry:   Entry{Fingerprint: "f1", Notified: &recent},
	}
	if !router.Route([]Event{event}, now)[0].Delivered() {
		t.Error("a resolved event must not be suppressed by the re-notify window")
	}
}

func TestGroupingCollapsesLikeFindings(t *testing.T) {
	// Twelve buckets that all lost encryption is one alert about a policy
	// change, not twelve alerts.
	addresses := []string{"b.one", "b.two", "b.three"}
	decisions := make([]Decision, 0, len(addresses))
	for _, address := range addresses {
		decisions = append(decisions, Decision{
			Tier: TierNotify,
			Event: Event{
				Kind:    EventNew,
				Finding: Finding{Address: address, Kind: "modified", Severity: "high"},
				Entry:   Entry{Fingerprint: address},
			},
			Owner: Owner{Name: "@platform"},
		})
	}

	groups := GroupDecisions(decisions)
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if len(groups[0].Decisions) != 3 {
		t.Errorf("group holds %d members, want 3", len(groups[0].Decisions))
	}
	if groups[0].Owner.Name != "@platform" {
		t.Errorf("a unanimous owner should carry onto the group, got %q", groups[0].Owner.Name)
	}
}

func TestGroupWithMixedOwnersIsUnowned(t *testing.T) {
	// Addressing a group to one of several owners would tell the wrong team
	// it owns something it does not.
	decisions := []Decision{
		{Tier: TierNotify, Owner: Owner{Name: "@a"}, Event: Event{
			Kind: EventNew, Finding: Finding{Kind: "modified"}, Entry: Entry{Fingerprint: "1"}}},
		{Tier: TierNotify, Owner: Owner{Name: "@b"}, Event: Event{
			Kind: EventNew, Finding: Finding{Kind: "modified"}, Entry: Entry{Fingerprint: "2"}}},
	}
	if groups := GroupDecisions(decisions); groups[0].Owner.Known() {
		t.Errorf("a mixed-owner group should be unowned, got %q", groups[0].Owner.Name)
	}
}

// ---- Ownership -------------------------------------------------------------

func TestOwnershipPrefersTagOverEverything(t *testing.T) {
	resolver, err := NewResolver(
		OwnershipConfig{
			TagKeys:  []string{"owner"},
			Fallback: map[string]string{"aws_s3_bucket.*": "@fallback"},
			Default:  "#default",
		},
		map[string]map[string]string{"aws_s3_bucket.assets": {"owner": "@storage"}},
	)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}

	owner := resolver.Resolve(Finding{Address: "aws_s3_bucket.assets"})
	if owner.Name != "@storage" {
		t.Errorf("owner = %q, want @storage (the tag is the most specific signal)", owner.Name)
	}
}

func TestOwnershipFallbackPrefersLongestPrefix(t *testing.T) {
	resolver, err := NewResolver(OwnershipConfig{
		Fallback: map[string]string{
			"module.*":          "@platform",
			"module.payments.*": "@payments",
		},
	}, nil)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}

	owner := resolver.Resolve(Finding{Address: "module.payments.aws_kms_key.main"})
	if owner.Name != "@payments" {
		t.Errorf("owner = %q, want @payments; a specific rule must beat a general one", owner.Name)
	}
}

func TestOwnershipFallsBackToDefault(t *testing.T) {
	resolver, _ := NewResolver(OwnershipConfig{Default: "#infra-drift"}, nil)
	owner := resolver.Resolve(Finding{Address: "aws_vpc.main"})

	if owner.Name != "#infra-drift" || owner.Source != "default" {
		t.Errorf("owner = %+v, want the default", owner)
	}
}

func TestModulePath(t *testing.T) {
	cases := map[string]string{
		"aws_s3_bucket.assets":                "",
		"module.vpc.aws_subnet.private":       "module.vpc",
		"module.vpc.module.inner.aws_eip.nat": "module.vpc.module.inner",
		"module.payments.aws_kms_key.main":    "module.payments",
	}
	for address, want := range cases {
		if got := ModulePath(address); got != want {
			t.Errorf("ModulePath(%q) = %q, want %q", address, got, want)
		}
	}
}

func TestCodeownersMatching(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CODEOWNERS")
	content := "# comment\n*           @everyone\nmodules/vpc  @network\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	resolver, err := NewResolver(OwnershipConfig{CodeownersPath: path}, nil)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}

	// The last matching rule wins, which is how git resolves CODEOWNERS.
	owner := resolver.Resolve(Finding{Address: "module.modules/vpc.aws_subnet.a"})
	if owner.Name != "@network" {
		t.Errorf("owner = %q, want @network", owner.Name)
	}
}

func TestMissingCodeownersIsAnError(t *testing.T) {
	// The user asked for it; silently ignoring it would leave them wondering
	// why nothing is owned.
	_, err := NewResolver(OwnershipConfig{CodeownersPath: "/nonexistent/CODEOWNERS"}, nil)
	if err == nil {
		t.Error("a configured but absent CODEOWNERS should error")
	}
}

// ---- Redaction -------------------------------------------------------------

func TestNotificationsCarryNoValues(t *testing.T) {
	// A notification lands somewhere with a wider audience than the terminal
	// the scan ran in. Paths and severities only.
	group := Group{
		Tier: TierNotify,
		Decisions: []Decision{{
			Tier: TierNotify,
			Event: Event{
				Kind: EventNew,
				Finding: Finding{
					Address:      "aws_db_instance.primary",
					Kind:         "modified",
					Severity:     "critical",
					ChangedPaths: []string{"password", "acl"},
				},
			},
		}},
	}

	encoded, err := json.Marshal(Redact(group))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// The path may appear; a value never can. The Finding type carries no
	// values at all, which is the structural guarantee behind this test.
	if strings.Contains(string(encoded), "correct-horse") {
		t.Errorf("a value leaked into a notification: %s", encoded)
	}
	if !strings.Contains(string(encoded), "password") {
		t.Error("the changed path should still be reported")
	}
}

func TestSanitiseNeutralisesHostileNames(t *testing.T) {
	// Resource names and tags come from an account that may hold resources an
	// attacker created. A tag must never become a mention or a link.
	cases := map[string]string{
		"<!channel> urgent":    "&lt;!channel&gt; urgent",
		"a\x1b[31mred":         "a[31mred",
		"line\nbreak":          "line break",
		"tom & jerry":          "tom &amp; jerry",
		"<https://evil|click>": "&lt;https://evil|click&gt;",
	}
	for input, want := range cases {
		if got := Sanitise(input); got != want {
			t.Errorf("Sanitise(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSanitiseBoundsLength(t *testing.T) {
	// A very long name is a denial of service on a channel, not information.
	long := strings.Repeat("a", 5000)
	if got := Sanitise(long); len(got) > 300 {
		t.Errorf("Sanitise did not bound length: got %d characters", len(got))
	}
}

// ---- Signing ---------------------------------------------------------------

func TestWebhookSignatureRoundTrips(t *testing.T) {
	body := []byte(`{"title":"drift"}`)
	timestamp := "1756400000"
	signature := Sign("s3cret", timestamp, body)

	if !Verify("s3cret", timestamp, signature, body) {
		t.Error("a signature this package produced did not verify")
	}
	if Verify("wrong", timestamp, signature, body) {
		t.Error("verification accepted the wrong secret")
	}
	if Verify("s3cret", timestamp, signature, []byte(`{"title":"tampered"}`)) {
		t.Error("verification accepted a tampered body")
	}
	if Verify("s3cret", "1756499999", signature, body) {
		t.Error("verification accepted a different timestamp; replay is possible")
	}
}

// ---- Sinks -----------------------------------------------------------------

func TestWebhookSinkSignsWhatItSends(t *testing.T) {
	var gotSignature, gotTimestamp string
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSignature = r.Header.Get(SignatureHeader)
		gotTimestamp = r.Header.Get(TimestampHeader)
		gotBody, _ = readAll(r)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sink := WebhookSink{URL: server.URL, Secret: "s3cret", Client: server.Client()}
	if err := sink.Send(context.Background(), Notification{Title: "drift"}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if !Verify("s3cret", gotTimestamp, gotSignature, gotBody) {
		t.Error("the receiver could not verify the signature this sink sent")
	}
}

func TestSlackSinkTreatsOkFalseAsFailure(t *testing.T) {
	// Slack answers 200 with ok:false for application errors, so the status
	// code alone does not say whether the message arrived.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error":"channel_not_found"}`))
	}))
	defer server.Close()

	sink := SlackSink{Token: "xoxb-test", Channel: "#nope", Client: server.Client()}
	// Point the sink at the test server by overriding the transport.
	sink.Client = &http.Client{Transport: rewriteTransport{server.URL}}

	err := sink.Send(context.Background(), Notification{Title: "drift"})
	if err == nil {
		t.Fatal("expected an error when Slack replies ok:false")
	}
	if !strings.Contains(err.Error(), "channel_not_found") {
		t.Errorf("the error should carry Slack's reason, got: %v", err)
	}
}

func TestStdoutSinkRendersWithoutValues(t *testing.T) {
	var out strings.Builder
	sink := StdoutSink{Out: &out}

	err := sink.Send(context.Background(), Notification{
		Title:   "1 new drift finding",
		Summary: "Infrastructure changed outside Terraform.",
		Tier:    TierNotify,
		Items: []NotificationItem{{
			Address: "aws_s3_bucket.assets", Severity: "critical", Paths: []string{"acl"},
		}},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	for _, want := range []string{"NOTIFY", "aws_s3_bucket.assets", "CRITICAL", "acl"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output omitted %q:\n%s", want, out.String())
		}
	}
}

// rewriteTransport redirects every request to a test server.
type rewriteTransport struct{ base string }

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	target := t.base + req.URL.Path
	proxied, err := http.NewRequestWithContext(req.Context(), req.Method, target, req.Body)
	if err != nil {
		return nil, err
	}
	proxied.Header = req.Header
	return http.DefaultTransport.RoundTrip(proxied)
}

// readAll drains a request body.
func readAll(r *http.Request) ([]byte, error) {
	defer func() { _ = r.Body.Close() }()
	buf := make([]byte, 0, 1024)
	tmp := make([]byte, 512)
	for {
		n, err := r.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			return buf, nil
		}
	}
}
