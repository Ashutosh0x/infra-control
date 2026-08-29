# Notification layer — design note

Status: **proposed**, not implemented. This document exists to be argued with
before any of it is built.

## The problem this solves

Not "infractl has no Slack integration." The problem is that a drift alert
which arrives every night for six weeks stops being read in week two.

The numbers people cite for this are consistent: teams take on the order of
thousands of alerts a week, a low single-digit percentage of which need
immediate action, and outages get attributed to alerts that were ignored
because everything was an alert. The operative rule from the on-call literature
is blunt — if the person receiving an alert cannot take a specific action, the
alert should not exist.

So the design goal is not delivery. It is that **a finding reaches one person,
once, with enough context to act, and stops arriving when it stops mattering.**

Three questions every notification must answer:

| Question | Answered by |
| --- | --- |
| Who owns this? | Ownership resolution (§3) |
| How urgently? | Tier routing (§4) |
| What do I do? | The remediation output that already exists (`--fix`) |

A finding that cannot answer all three goes in a digest, not a notification.

---

## 1. Pipeline

```
drift scan  →  findings[]  →  ledger  →  events[]  →  router  →  sinks
                              (dedupe,   (NEW,       (owner,     (slack,
                               state)     RESOLVED,   tier)       github,
                                          AGING)                  webhook)
```

The important property: **sinks consume events, never findings.** A finding is
a fact about infrastructure. An event is a fact about a finding's history —
that it is new, or gone, or has been open for a fortnight. Only the second is
worth interrupting someone for.

The layer is strictly a consumer. It never changes a finding, a severity, an
exit code, or the JSON and SARIF payloads. `drift scan` behaves identically
whether or not notifications are configured.

---

## 2. The ledger

The piece most tools skip, and the one that does the actual work.

**Location:** `.infractl/ledger.jsonl`, overridable. JSONL rather than SQLite:
it is diffable, appendable without a driver, survives a merge conflict
legibly, and adds no cgo dependency to a binary that currently has none.
Compaction rewrites the file when it exceeds a size threshold.

**Record:**

```json
{
  "fingerprint": "a3f2c1...",
  "address": "aws_s3_bucket.assets",
  "kind": "modified",
  "first_seen": "2026-08-01T09:14:22Z",
  "last_seen": "2026-08-29T06:00:11Z",
  "notified_at": "2026-08-01T09:15:03Z",
  "state": "open"
}
```

**States:** `new` → `open` → (`resolved` | `suppressed` | `snoozed`).

### Fingerprint

```
sha256(address + "\0" + kind + "\0" + sorted(changed attribute paths))
```

Deliberately **excluding** values, timestamps, and severity.

- **Values excluded** so that a bucket drifting further does not read as a new
  finding. It is the same problem, worse. It should age, not re-alert.
- **Severity excluded** so that a re-scored rule does not resurrect every
  finding it touches as new.
- **Paths sorted** so map iteration order cannot change the fingerprint.

This is the same fingerprint SARIF already uses for GitHub's fixed/still-open
tracking, so the two agree by construction rather than by coincidence.

### Events

| Event | Condition |
| --- | --- |
| `NEW` | Fingerprint absent from the ledger |
| `RESOLVED` | In the ledger as `open`, absent from this scan |
| `AGING` | `open`, and `first_seen` older than `aging_after` (default 7d) |

`AGING` fires **once** per threshold crossing, not every scan.

---

## 3. Ownership

An alert with no owner has no responder. Resolution order, first match wins:

1. An `owner` or `team` tag on the resource.
2. A CODEOWNERS match against the resource's module source path.
3. A fallback map in config, keyed by address prefix or resource type.
4. Unowned — routed to a default channel and **counted**, because a high
   unowned rate is itself the finding.

### Source position: stubbed on purpose

Mapping a state address to the HCL file and line that declared it is the
enrichment that turns `aws_s3_bucket.assets` into
`modules/storage/main.tf:47, owned by @data-platform`. It is also genuinely
fiddly: state records a module *address*, not a source path, and recovering
the path needs the configuration block from `terraform show -json` plus
module-source resolution, with more edge cases than are worth guessing at.

**Decision: define the interface, implement the module-address half, and
return "unknown" for line numbers rather than approximating them.**

```go
type SourceResolver interface {
    // Resolve returns the file and line declaring a resource. found is false
    // when the position cannot be determined; callers must degrade rather
    // than display a guess.
    Resolve(address string) (file string, line int, found bool)
}
```

A wrong line number in an alert sends someone to the wrong code, which is
worse than sending them to the right module with no line.

---

## 4. Router and tiers

Declared in config, not hardcoded.

| Tier | Default trigger | Sink | Timing |
| --- | --- | --- | --- |
| `page` | `NEW` + critical + security dimension | On-call, user group not `@channel` | Immediate |
| `notify` | `NEW` + high | Team channel, threaded | Business hours |
| `digest` | Everything else, plus `RESOLVED` and `AGING` | One scheduled message | Batched |

Two suppression rules on top:

- **Re-notify window.** The same fingerprint is never notified twice inside
  `renotify_after` (default 7d).
- **Grouping.** Findings sharing a resource, or a rule across many resources,
  become one message. Twelve buckets that all lost encryption is one alert
  about a policy change, not twelve alerts.

The Google SRE guidance of two to three actionable pages per shift is the
calibration target. If a default policy cannot hold that on a real estate, the
default is wrong.

---

## 5. Sinks

```go
type Sink interface {
    Name() string
    Send(ctx context.Context, n Notification) error
}
```

Shipping first: `slack`, `github`, `webhook`. `teams`, `discord`, and
`pagerduty` get the interface and an explicit unimplemented error — the same
rule the rest of the CLI follows.

---

## 6. Interactivity — `infractl bot serve`

Slack buttons: **Suppress**, **Accept into state**, **Open fix PR**,
**Snooze 7d**, **Show blast radius**.

Suppress opens a modal that **requires a reason** and records the clicker's
handle, writing a rule to `.infractl-ignore.yaml`. This preserves the
required-reason property that makes suppression trustworthy, while removing
the friction of hand-writing YAML — which is the reason people currently skip
suppression and let noise accumulate instead.

Note for implementation: Slack's `trigger_id` expires in three seconds, so the
modal must open before any other work.

Slash commands: `/infractl drift [env]`, `/infractl blast <address>`,
`/infractl why <finding-id>`.

---

## 7. Security

The notification path is a **new egress boundary** and is treated as one.

| Control | Rule |
| --- | --- |
| Redaction | Notifications carry attribute **paths and severities, never values**, unless `--include-values`. This is a second allowlist *after* the existing sensitive-value drop, because Slack is a different audience from a terminal |
| Untrusted input | Resource names and tag values come from a cloud account that may hold attacker-created resources. Control characters and Slack mrkdwn are stripped before rendering; a tag value can never become a link, a mention, or a command |
| Inbound auth | Slack signing secret verified on every request; unsigned requests rejected. Timestamp checked against replay |
| Outbound auth | Generic webhooks signed; the scheme documented |
| Scopes | `chat:write` and `commands`. Nothing else. Explicitly **not** `channels:history` |
| Binding | `bot serve` binds localhost by default; remote binding needs an explicit flag and prints a warning |
| Telemetry | None. Not opt-out — absent |

A `SECURITY.md` section covers the notification threat model, including the
prompt-injection-shaped risk of cloud-derived strings reaching a renderer.

---

## 8. `infractl notify audit`

The feedback loop that keeps the policy honest. Per rule over a window: fired,
acted on, suppressed, and the **action rate**. A rule with a zero action rate
over thirty days is flagged as a noise candidate.

This is the part that makes tuning possible instead of aspirational.

---

## 9. Config

```yaml
notify:
  aging_after: 7d
  renotify_after: 7d
  ledger: .infractl/ledger.jsonl

  ownership:
    tag_keys: [owner, team]
    codeowners: .github/CODEOWNERS
    fallback:
      "module.payments.*": "@payments"
    default: "#infra-drift"

  tiers:
    page:
      when: { event: new, severity: [critical], dimension: [security] }
      sink: pagerduty
    notify:
      when: { event: new, severity: [high] }
      sink: slack
    digest:
      when: { event: [any] }
      sink: slack
      schedule: "0 9 * * 1"

  sinks:
    slack:
      channel: "#infra-drift"
      token_env: SLACK_BOT_TOKEN
      signing_secret_env: SLACK_SIGNING_SECRET
```

Every setting has an `INFRACTL_`-prefixed environment variable, following the
existing precedence.

---

## 10. Non-interactive behaviour

`bot serve` is a server and never prompts. Every other new surface follows the
existing rule: **when stdout is not a TTY, or `CI` is set, nothing prompts.**
A missing required value fails with a message naming the flag to pass. A hung
pipeline is worse than an error.

---

## 11. What this deliberately is not

- No SaaS backend, hosted service, or account system.
- No auto-remediation. The bot proposes; a human applies. The blast radius is
  printed beside the proposal, and a tool that measures a risk then takes it
  anyway has given up the reason to trust the measurement.
- No cloud API calls in the scan path.
- No LLM calls anywhere in this layer.
- No new dependency in the local-analysis path. `bot serve` may pull
  dependencies; `drift scan` must not get slower or larger for it.

---

## 12. Open questions

Each carries a recommendation rather than only a question.

**Q1. Is the long-running `bot serve` in scope now, or one-way notifications first?**
A server is a real commitment for a tool that is currently a single static
binary with no runtime. **Recommendation: ship one-way first** — ledger,
events, router, and the three sinks, driven by `infractl notify` in CI. That
delivers the entire anti-fatigue benefit with no service to operate. Buttons
follow once the routing defaults are proven against a real estate.

**Q2. Where does the ledger live in CI, where the working directory is discarded?**
The ledger is worthless without persistence, and this is the case that decides
its shape. **Recommendation: commit it.** `.infractl/ledger.jsonl` in the repo
makes drift history reviewable, diffable, and durable with no external store,
and a scan that updates it produces a commit that is itself the audit trail.
The alternative — a CI cache or an object-store URI — is available later
behind the same interface.

**Q3. Does `AGING` belong at all in v1?**
It is the one event class requiring a durable clock rather than set
comparison. **Recommendation: yes, but digest-only**, never paging. "This has
been open a fortnight" is exactly the thing a weekly review should surface and
exactly the thing that should never wake someone.
