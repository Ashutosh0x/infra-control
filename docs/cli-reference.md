# CLI reference

Every command supports the global flags. Every command producing a list supports every output format.

## Global flags

| Flag | Default | Description |
| --- | --- | --- |
| `-o, --output` | `table` | `table`, `wide`, `json`, `yaml`, `csv`, `tsv`, `name`, `sarif`, `go-template=TMPL` |
| `-q, --quiet` | off | Suppress progress. Results and errors still print |
| `-v, --verbose` | off | Diagnostic logging on stderr |
| `--color` | `auto` | `auto`, `always`, `never` |
| `--no-color` | off | Shorthand for `--color=never` |
| `--ascii` | off | ASCII symbols instead of box-drawing characters |
| `--config` | | Config file path |
| `--profile` | | Named block from the config file |

Colour resolution order: `--color` wins, then `NO_COLOR`, then `TERM=dumb`, then `FORCE_COLOR`, then TTY detection. Machine formats are never coloured regardless.

Configuration precedence, highest first: an explicit flag, then an `INFRACTL_`-prefixed environment variable, then the config file, then the flag default. `--min-severity` reads `INFRACTL_MIN_SEVERITY`.

---

## demo

```
infractl demo [--keep]
```

Runs the whole pipeline against fixtures embedded in the binary. Nothing is read from your machine and nothing leaves it.

It drives the same code a real scan does rather than printing a transcript, so if drift detection breaks the demo breaks with it. That is the only way a demo stays honest.

`--keep` writes the fixtures to `./infractl-demo` so you can experiment against them.

---

## doctor

```
infractl doctor [--state <file>] [--live <file>] [--ignore-file <file>]
```

Checks everything a scan depends on and names the command that fixes each failure. Run it first when a scan does not behave as expected, and include the output when reporting a bug.

| Check | Reports a problem when |
| --- | --- |
| terraform binary | Neither `terraform` nor `tofu` is on PATH. A warning only; scans against an existing snapshot do not need it |
| config file | A config file exists but does not parse |
| ignore rules | A rule is malformed, or one has expired and stopped suppressing |
| state file | Missing, a directory, or unparseable |
| live snapshot | Missing or unparseable |
| snapshot age | Older than 7 days fails; older than 1 day warns; a future date warns |

Exits 4 when any check fails, so a CI setup step stops rather than continuing into a confusing failure later.

A future-dated snapshot is reported as a clock skew rather than rendered as negative freshness.

---

## snapshot

### snapshot from-plan

```
infractl snapshot from-plan [plan.json] [--out live.json] [--dir .] [--keep-plan]
```

Builds the live snapshot that `drift scan` compares against.

With no argument it runs the sequence for you:

```
terraform plan -refresh-only -input=false -out=<tmp>
terraform show -json <tmp>
```

A refresh-only plan asks every provider for the real attributes of every managed resource, and Terraform records those refreshed values in the plan's prior state. That is a live read of your infrastructure, performed by Terraform, using credentials you already have configured. `-refresh-only` proposes no changes, so it cannot modify anything.

Pass a file instead to read a plan you produced yourself.

**What this cannot do.** Terraform refreshes only what it manages, so a snapshot built this way never contains an unmanaged resource. A scan against it reports modified and deleted resources but can report no unmanaged ones. The command says so on every run, not only here. For unmanaged resources you need an inventory read; see [live-snapshots.md](live-snapshots.md).

| Flag | Default | Description |
| --- | --- | --- |
| `--out` | `live.json` | Where to write the snapshot |
| `--dir` | `.` | Terraform directory to run the plan in |
| `--provider` | inferred | Provider name recorded in the snapshot |
| `--keep-plan` | off | Keep the intermediate plan JSON for inspection |

Set `INFRACTL_TERRAFORM_BINARY` to use a binary that is not on PATH.

---

## state

Read-only inspection. These commands never write to a state file.

### state inspect

```
infractl state inspect <state-file>
```

Format version, the Terraform version that wrote it, serial, lineage, managed resource count, providers, output count, and a per-type breakdown.

### state list

```
infractl state list <state-file> [--type T] [--provider P] [--module M]
```

Every managed resource instance. Data sources are excluded: Terraform reads them but does not own them.

`-o name` prints bare addresses, which is what a shell loop wants:

```bash
for addr in $(infractl state list terraform.tfstate -o name); do
  infractl graph blast-radius "$addr" --state terraform.tfstate -o json
done
```

### state show

```
infractl state show <state-file> <address>
```

Every attribute Terraform records for one resource. Attributes whose names indicate a secret are replaced before the payload is built, so `-o json` cannot leak what the table hides.

Detection is name-based, not provider-declared. Treat all output as sensitive.

Quote addresses containing brackets, or the shell will glob them.

---

## drift

### drift scan

```
infractl drift scan --state <file> --live <file> [flags]
```

Compare state against a live snapshot. See [live-snapshots.md](live-snapshots.md) for the snapshot format.

| Flag | Default | Description |
| --- | --- | --- |
| `--state` | required | State file |
| `--live` | required | Live snapshot JSON |
| `--min-severity` | `info` | Only report findings at or above this severity |
| `--fail-on` | `none` | Exit 3 at or above this severity. `none`, `any`, `low`, `medium`, `high`, `critical` |
| `--show-diff` | off | Print the property-level diff for each finding |
| `--include-unmanaged` | on | Report live resources Terraform does not track |
| `--fix` | off | Print the commands and import blocks that resolve each finding. Runs nothing |
| `--emit-import` | | Write Terraform import blocks for unmanaged resources to this file |
| `--ignore-file` | nearest | Suppression rules to apply |
| `--no-ignore` | off | Report every finding, for auditing what suppression hides |

Three kinds of finding:

| Kind | Meaning |
| --- | --- |
| `modified` | In both, attributes disagree |
| `missing_in_live` | State records it; it no longer exists |
| `unmanaged` | Live but not tracked by Terraform |

A snapshot older than 24 hours produces a staleness warning, because drift found against stale data may already have been fixed.

### Resolving findings

`--fix` prints what would resolve each finding. Which of the three safe resolutions applies is determined by the kind of finding, not by judgement:

| Finding | Resolutions offered |
| --- | --- |
| `modified` | Revert with `apply -target`, or accept into state with `apply -refresh-only -target` |
| `missing_in_live` | Recreate with `apply -target`, or record the deletion with `-refresh-only` |
| `unmanaged` | Adopt it with a generated `import` block |

Nothing is executed. Import identifiers are provider-specific and sometimes composite, so each generated block records which attribute its id came from, and says plainly when it could not find one.

### Coverage

Every scan reports how much of the observed estate Terraform manages. One untracked security group is a curiosity; eighty against four hundred managed resources is a different conversation.

A snapshot that structurally cannot see unmanaged resources is marked `partial`, so the figure is never read as full coverage when it means "could not look".

---

## ignore

### ignore add

```
infractl ignore add <address> --reason "<why>" [--attributes a,b] [--expires YYYY-MM-DD] [--dry-run]
```

Writes a suppression rule so expected drift stops being reported.

The reason is **required**. On a terminal it is prompted for; with nothing attached to read the prompt the command fails naming the flag, because a pipeline that hangs waiting for input is worse than one that stops.

Scope the rule to specific attributes when only part of a resource's drift is expected. An attribute-scoped rule suppresses a finding only when it covers **every** changed attribute, so a resource where an expected attribute and an unexpected one both moved is still reported.

The file is decoded and re-encoded rather than appended to as text, so a malformed result is impossible, an unparseable existing file is refused rather than overwritten, and a duplicate rule is rejected. The written file is then re-read through the loader scans use, so a rule that would be rejected tomorrow fails today.

### ignore list

```
infractl ignore list
```

Active and expired rules. A lapsed rule is usually why findings reappeared.

---

## notify

```
infractl notify --state <file> --live <file> [--sink <name>] [flags]
```

Reports what changed since the last scan rather than everything, every time.

`drift scan` reports every finding on every run. Sent to a channel nightly, that is how a channel stops being read. `notify` reports what is **new**, what has just been **resolved**, and what has been open long enough to be **aging**. Run it twice over unchanged infrastructure and the second run says so in one line and sends nothing.

### Tiers

| Tier | Default trigger | Meaning |
| --- | --- | --- |
| `page` | new, critical, security | Something changed, it matters, and it changed just now |
| `notify` | new, high | Worth a channel during working hours |
| `digest` | everything else, plus resolved and aging | Batched |

### Ownership

Resolved in order, first match wins: an `owner` or `team` tag on the resource, a CODEOWNERS match on the module path, then the configured default. Findings nothing matched are counted and reported, because a high unowned rate is itself a finding and an alert with no owner has no responder.

### Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--sink` | none | `stdout`, `slack`, `webhook`, `github`. Omit to report without sending |
| `--channel` | | Slack channel |
| `--webhook-url` | | Webhook endpoint |
| `--ledger` | `.infractl/ledger.jsonl` | Where scan history is recorded |
| `--owner-tags` | `owner,team` | Tag keys checked for an owner, in order |
| `--owner-default` | | Owner for findings nothing else matches |
| `--codeowners` | | CODEOWNERS file to match module paths against |
| `--aging-days` | `7` | Days open before a finding is reported as aging |
| `--renotify-days` | `7` | Days before the same finding may be reported again |
| `--no-save` | off | Preview without recording history |

### Environment

| Variable | Used by |
| --- | --- |
| `SLACK_BOT_TOKEN` | slack sink. The app needs `chat:write` and nothing else |
| `INFRACTL_WEBHOOK_SECRET` | webhook sink. Absent means unsigned, and the run warns |
| `GITHUB_TOKEN`, `GITHUB_REPOSITORY`, `INFRACTL_GITHUB_PR` | github sink |

Notifications carry attribute **paths and severities, never values** — enforced by the type system, not a flag. See [SECURITY.md](../SECURITY.md#the-notification-path) and [notifications.md](notifications.md).

---

## plan

### plan analyse

```
infractl plan analyse <plan.json> [--details] [--fail-on X] [--max-deletes N]
```

Aliased as `analyze`.

Input must be `terraform show -json` output, not the binary plan file, which has no public format:

```bash
terraform plan -out=tfplan
terraform show -json tfplan > plan.json
```

| Flag | Default | Description |
| --- | --- | --- |
| `--fail-on` | `none` | Exit 3 on match. `none`, `any`, `destructive`, `delete` |
| `--details` | off | List every change, not only the destructive ones |
| `--max-deletes` | `-1` | Exit 3 above this many deletions. `-1` disables |

Without `--details` the table shows only destructive changes, because those are what a reviewer must actually look at. The summary line always covers everything.

A **replace** counts once and as destructive. Terraform encodes it as delete plus create, and the delete is real: the live resource and everything depending on it goes away, however briefly.

Two warnings are raised automatically: a plan that only deletes, which usually means the wrong workspace is selected; and a stateful resource being deleted or replaced, since its data does not survive.

---

## risk

### risk assess

```
infractl risk assess --state <file> [--min-level L] [--top N] [--show-factors]
```

Scores every managed resource across four dimensions, then combines them.

| Dimension | Weight | Checks |
| --- | --- | --- |
| Security | 0.35 | Public exposure, missing encryption, not IaC-managed |
| Reliability | 0.30 | Single-AZ deployment, absent backups |
| Compliance | 0.20 | Missing `environment`, `owner`, `cost-center` tags |
| Cost | 0.15 | Instance sizes suggesting overprovisioning |

Checks apply only where the underlying concept exists. A VPC has no encryption-at-rest setting and a subnet is single-AZ by definition, so neither is scored against those checks. A finding nobody can act on teaches users to ignore the tool.

Scoring reads only what state records. A resource whose risk depends on something outside state, such as an IAM policy defined elsewhere, is scored only on the attributes present.

Rules are currently AWS-shaped. State parsing and drift detection are provider-agnostic.

---

## graph

Dependency edges come from what Terraform recorded when it applied, capturing both explicit `depends_on` and the implicit edges from attribute references. A dependency on something outside the state file, such as a data source, has no node to point at and is skipped rather than invented.

`--state` is required for every subcommand.

### graph stats

Node and edge counts, roots (nothing depends on them) and leaves (they depend on nothing).

### graph blast-radius

```
infractl graph blast-radius <address> --state <file> [--max-depth N]
```

Aliased as `blast` and `impact`.

Everything that depends on a resource, directly or transitively, with the distance in hops.

State records dependencies at block granularity, without the `count` index, so a dependency on `aws_subnet.private` fans out to every instance of that block and the traversal does not stop short.

### graph deps

```
infractl graph deps <address> --state <file> [--direction upstream|downstream|both]
```

The inverse of blast radius: what must exist before this resource can be created.

### graph export

```
infractl graph export --state <file> --format dot|mermaid
```

```bash
infractl graph export --state terraform.tfstate --format dot | dot -Tsvg > graph.svg
```

Mermaid output renders inline in GitHub Markdown.

---

## version

```
infractl version [-o json]
```

Version, commit, build date, Go toolchain, and platform. Include this output when reporting a bug.

A binary installed with `go install` reports its module version; one built from a checkout reports the commit, marked `-dirty` when the tree had uncommitted changes.

---

## Commands excluded from default builds

`discover`, `policy`, `compliance`, `cost`, `security`, `remediate`, and `audit` have no implementation and are compiled out, so `--help` lists only what the tool can do.

```bash
go build -tags preview ./cmd/infractl
```

They return an explicit error rather than a placeholder result. A placeholder is indistinguishable from a real answer at the point of reading, and infrastructure decisions get made on that output.
