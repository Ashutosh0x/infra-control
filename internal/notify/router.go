package notify

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Routing decides who hears about an event, how loudly, and whether they have
// heard it recently enough that saying it again would be noise.
//
// The calibration target is the SRE guidance of two to three actionable pages
// per shift. A default policy that cannot hold that against a real estate is
// the wrong default, not a configuration problem for the user to solve.

// Tier is how urgently an event should reach someone.
type Tier string

const (
	// TierPage interrupts a human immediately. Reserved for a new,
	// security-relevant, critical finding: something changed, it matters, and
	// it changed just now.
	TierPage Tier = "page"
	// TierNotify reaches a team channel during working hours.
	TierNotify Tier = "notify"
	// TierDigest is batched into a periodic summary.
	TierDigest Tier = "digest"
	// TierNone is not delivered at all.
	TierNone Tier = "none"
)

// TierRule matches events to a tier.
type TierRule struct {
	// Events limits the rule to these event kinds. Empty matches any.
	Events []EventKind `yaml:"events" json:"events"`
	// Severities limits the rule to these severities. Empty matches any.
	Severities []string `yaml:"severities" json:"severities"`
	// Dimensions limits the rule to these risk dimensions. Empty matches any.
	Dimensions []string `yaml:"dimensions" json:"dimensions"`
	// Sink names the destination.
	Sink string `yaml:"sink" json:"sink"`
}

// RouterConfig declares the routing policy.
type RouterConfig struct {
	// Tiers maps a tier to the rules that select it. Evaluated page, then
	// notify, then digest, so the loudest matching rule wins.
	Tiers map[Tier]TierRule `yaml:"tiers" json:"tiers"`
	// RenotifyAfter is how long before the same fingerprint may be notified
	// again. Without it, a finding that stays open re-alerts on every scan,
	// which is the single fastest way to make a channel unreadable.
	RenotifyAfter time.Duration `yaml:"renotify_after" json:"renotify_after"`
}

// DefaultRenotifyAfter matches the default aging threshold, so a finding that
// ages is also eligible to be mentioned again.
const DefaultRenotifyAfter = 7 * 24 * time.Hour

// DefaultRouterConfig is the policy applied when none is configured.
//
// Only a new, critical, security finding pages. Everything that is merely
// still true, or merely resolved, is batched. That is the whole anti-fatigue
// posture in three rules.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		RenotifyAfter: DefaultRenotifyAfter,
		Tiers: map[Tier]TierRule{
			TierPage: {
				Events:     []EventKind{EventNew},
				Severities: []string{"critical"},
				Dimensions: []string{"security"},
			},
			TierNotify: {
				Events:     []EventKind{EventNew},
				Severities: []string{"critical", "high"},
			},
			TierDigest: {},
		},
	}
}

// Decision is the routing outcome for one event.
type Decision struct {
	Event Event  `json:"event"`
	Tier  Tier   `json:"tier"`
	Owner Owner  `json:"owner"`
	Sink  string `json:"sink,omitempty"`
	// Suppressed explains why an event was not delivered, so a missing alert
	// can be accounted for rather than assumed lost.
	Suppressed string `json:"suppressed,omitempty"`
}

// Delivered reports whether the decision results in a notification.
func (d Decision) Delivered() bool { return d.Tier != TierNone && d.Suppressed == "" }

// Router applies a policy to events.
type Router struct {
	config   RouterConfig
	resolver *Resolver
}

// NewRouter builds a router.
func NewRouter(config RouterConfig, resolver *Resolver) *Router {
	if config.RenotifyAfter <= 0 {
		config.RenotifyAfter = DefaultRenotifyAfter
	}
	if len(config.Tiers) == 0 {
		config.Tiers = DefaultRouterConfig().Tiers
	}
	return &Router{config: config, resolver: resolver}
}

// Route assigns a tier and owner to every event.
func (r *Router) Route(events []Event, now time.Time) []Decision {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	decisions := make([]Decision, 0, len(events))
	for _, event := range events {
		decision := Decision{Event: event, Tier: r.tierFor(event)}

		if r.resolver != nil {
			decision.Owner = r.resolver.Resolve(event.Finding)
		}
		if rule, ok := r.config.Tiers[decision.Tier]; ok {
			decision.Sink = rule.Sink
		}

		// A resolved finding is always worth recording, and never worth
		// suppressing: it is the message that closes a loop someone is
		// waiting on.
		if event.Kind != EventResolved {
			if reason := r.suppressionReason(event, now); reason != "" {
				decision.Suppressed = reason
			}
		}

		decisions = append(decisions, decision)
	}
	return decisions
}

// tierFor selects the loudest tier whose rule matches.
func (r *Router) tierFor(event Event) Tier {
	for _, tier := range []Tier{TierPage, TierNotify, TierDigest} {
		rule, configured := r.config.Tiers[tier]
		if !configured {
			continue
		}
		if matchesRule(rule, event) {
			return tier
		}
	}
	return TierNone
}

// matchesRule reports whether an event satisfies every constraint on a rule.
// An empty constraint matches anything, so the zero rule is a catch-all.
func matchesRule(rule TierRule, event Event) bool {
	if len(rule.Events) > 0 && !containsEventKind(rule.Events, event.Kind) {
		return false
	}
	if len(rule.Severities) > 0 && !containsFold(rule.Severities, event.Finding.Severity) {
		return false
	}
	if len(rule.Dimensions) > 0 && !containsFold(rule.Dimensions, event.Finding.Dimension) {
		return false
	}
	return true
}

// suppressionReason returns why an event should not be delivered, or "".
func (r *Router) suppressionReason(event Event, now time.Time) string {
	entry := event.Entry

	if entry.SnoozedUntil != nil && now.Before(*entry.SnoozedUntil) {
		return fmt.Sprintf("snoozed until %s", entry.SnoozedUntil.Format(time.RFC3339))
	}
	if entry.State == StateSuppressed {
		return "an ignore rule covers this finding"
	}

	// A new finding is always delivered: it has not been said before, so the
	// re-notify window cannot apply to it.
	if event.Kind == EventNew {
		return ""
	}

	if entry.Notified != nil {
		since := now.Sub(*entry.Notified)
		if since < r.config.RenotifyAfter {
			return fmt.Sprintf("notified %s ago, within the %s re-notify window",
				since.Round(time.Hour), r.config.RenotifyAfter)
		}
	}
	return ""
}

// Group collapses related decisions into one notification.
//
// Twelve buckets that all lost encryption is one alert about a policy change,
// not twelve alerts. Grouping by rule shape rather than by resource is what
// makes that distinction, and it is the difference between a channel someone
// reads and one they mute.
type Group struct {
	// Key identifies what the members share.
	Key string `json:"key"`
	// Tier is the loudest tier among the members.
	Tier Tier `json:"tier"`
	// Owner is shared when every member agrees, and unowned otherwise.
	Owner Owner `json:"owner"`
	// Decisions are the members, ordered deterministically.
	Decisions []Decision `json:"decisions"`
}

// GroupDecisions batches deliverable decisions by event kind and finding kind.
func GroupDecisions(decisions []Decision) []Group {
	buckets := map[string][]Decision{}

	for _, decision := range decisions {
		if !decision.Delivered() {
			continue
		}
		key := fmt.Sprintf("%s/%s/%s",
			decision.Tier, decision.Event.Kind, decision.Event.Finding.Kind)
		buckets[key] = append(buckets[key], decision)
	}

	groups := make([]Group, 0, len(buckets))
	for key, members := range buckets {
		sort.SliceStable(members, func(i, j int) bool {
			return members[i].Event.Entry.Fingerprint < members[j].Event.Entry.Fingerprint
		})

		group := Group{Key: key, Tier: members[0].Tier, Decisions: members}

		// An owner is only carried onto the group when every member agrees;
		// otherwise the group would be addressed to someone who owns part of it.
		owner := members[0].Owner
		for _, member := range members[1:] {
			if member.Owner.Name != owner.Name {
				owner = Unowned
				break
			}
		}
		group.Owner = owner

		groups = append(groups, group)
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Tier != groups[j].Tier {
			return tierRank(groups[i].Tier) < tierRank(groups[j].Tier)
		}
		return groups[i].Key < groups[j].Key
	})
	return groups
}

// tierRank orders tiers loudest first.
func tierRank(tier Tier) int {
	switch tier {
	case TierPage:
		return 0
	case TierNotify:
		return 1
	case TierDigest:
		return 2
	default:
		return 3
	}
}

func containsEventKind(kinds []EventKind, kind EventKind) bool {
	for _, candidate := range kinds {
		if candidate == kind {
			return true
		}
	}
	return false
}

func containsFold(haystack []string, needle string) bool {
	for _, candidate := range haystack {
		if strings.EqualFold(candidate, needle) {
			return true
		}
	}
	return false
}
