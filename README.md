<div align="center">

# infra-control

**Read your Terraform state. Compare it to reality. Know what broke, who it belongs to, and what it takes down with it.**

[![CI](https://img.shields.io/github/actions/workflow/status/Ashutosh0x/infra-control/ci.yml?branch=main&style=for-the-badge&logo=githubactions&logoColor=white&label=CI)](https://github.com/Ashutosh0x/infra-control/actions/workflows/ci.yml)
[![Go Reference](https://img.shields.io/badge/pkg.go.dev-reference-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://pkg.go.dev/github.com/ashutosh0x/infra-control)
[![Go Report Card](https://goreportcard.com/badge/github.com/ashutosh0x/infra-control?style=for-the-badge)](https://goreportcard.com/report/github.com/ashutosh0x/infra-control)
[![Release](https://img.shields.io/github/v/release/Ashutosh0x/infra-control?style=for-the-badge&logo=github&logoColor=white)](https://github.com/Ashutosh0x/infra-control/releases)
[![License](https://img.shields.io/badge/Apache%202.0-D22128?style=for-the-badge&logo=apache&logoColor=white)](LICENSE)

[![Go](https://img.shields.io/badge/Go%201.25-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev)
[![Terraform](https://img.shields.io/badge/Terraform-844FBA?style=for-the-badge&logo=terraform&logoColor=white)](https://www.terraform.io)
[![OpenTofu](https://img.shields.io/badge/OpenTofu-FFDA18?style=for-the-badge&logo=opentofu&logoColor=black)](https://opentofu.org)
[![SARIF](https://img.shields.io/badge/SARIF%202.1.0-2088FF?style=for-the-badge&logo=github&logoColor=white)](https://sarifweb.azurewebsites.net)

**No server. No agent. No cloud credentials. No telemetry.**

State and drift detection work with any Terraform provider.<br/>Risk scoring rules are currently AWS-shaped.

[Install](#installation) · [Quick start](#quick-start) · [Features](#features) · [Commands](#command-reference) · [CI](#continuous-integration) · [Docs](docs/) · [Discussions](https://github.com/Ashutosh0x/infra-control/discussions)

</div>

---

## What this is

`infractl` answers four questions about infrastructure you already manage with Terraform or OpenTofu:

| Question | Command | Needs |
| --- | --- | --- |
| What is Terraform actually managing? | `infractl state inspect` | A state file |
| Has anything changed behind Terraform's back? | `infractl drift scan` | A state file and a live snapshot |
| If I apply this plan, what does it destroy? | `infractl plan analyse` | A JSON plan |
| Which resources are risky, and why? | `infractl risk assess` | A state file |
| If I change this, what else breaks? | `infractl graph blast-radius` | A state file |

All of it runs locally. No server, no agent, no cloud credentials, no data leaving the machine.

### Why another one

[driftctl](https://github.com/snyk/driftctl), the tool most teams reached for, has been in maintenance mode since Snyk stopped feature work on it. The alternatives that kept moving are SaaS control planes: Firefly, Spacelift, env0, Scalr. They are good products, and they all want your state file on their servers and a seat-based contract.

This sits in the gap: a single static binary that reads the files you already have, exits with a status code CI can branch on, and prints the same data as a table for a human or JSON for a pipeline.

---

## Installation

### Package managers

```bash
# Homebrew (macOS, Linux)
brew install Ashutosh0x/tap/infractl

# Scoop (Windows)
scoop bucket add ashutosh0x https://github.com/Ashutosh0x/scoop-bucket
scoop install infractl

# Go
go install github.com/ashutosh0x/infra-control/cmd/infractl@latest
```

### Docker

The image is distroless and runs as a non-root user, so a mounted working
directory cannot be modified by the container.

```bash
docker run --rm -v "$PWD:/work" ghcr.io/ashutosh0x/infractl:latest   drift scan --state terraform.tfstate --live live.json
```

> The image is published on each release. GitHub creates new packages private
> by default, so make `infractl` public under the repository's Packages
> settings before this command works for anyone else.

### Binary download

Grab a build from [Releases](https://github.com/Ashutosh0x/infra-control/releases). Binaries are static, with no runtime dependency.

```bash
# Linux, amd64
curl -sSL https://github.com/Ashutosh0x/infra-control/releases/latest/download/infra-control_Linux_x86_64.tar.gz \
  | tar xz infractl && sudo mv infractl /usr/local/bin/

# macOS, Apple silicon
curl -sSL https://github.com/Ashutosh0x/infra-control/releases/latest/download/infra-control_Darwin_arm64.tar.gz \
  | tar xz infractl && sudo mv infractl /usr/local/bin/
```

```powershell
# Windows
$url = "https://github.com/Ashutosh0x/infra-control/releases/latest/download/infra-control_Windows_x86_64.zip"
Invoke-WebRequest $url -OutFile infractl.zip
Expand-Archive infractl.zip -DestinationPath "$env:LOCALAPPDATA\Programs\infractl"
$env:Path += ";$env:LOCALAPPDATA\Programs\infractl"
```

### From source

```bash
git clone https://github.com/Ashutosh0x/infra-control.git
cd infra-control
make build          # binaries land in ./bin
```

### Verify

```bash
infractl version
```

### Shell completion

Completion covers subcommands, flags, and the closed-vocabulary flag values such as `--severity` and `--fail-on`.

```bash
infractl completion bash > /etc/bash_completion.d/infractl     # Bash
infractl completion zsh  > "${fpath[1]}/_infractl"             # Zsh
infractl completion fish > ~/.config/fish/completions/infractl.fish
infractl completion powershell | Out-String | Invoke-Expression
```

---

## Quick start

### See it work, with no setup

```bash
infractl demo
```

Runs the whole pipeline against fixtures embedded in the binary. Nothing is read from your machine, nothing leaves it, and it drives the same code a real scan does.

### 1. Capture what is actually live

```bash
infractl snapshot from-plan
```

This runs `terraform plan -refresh-only` and `terraform show -json` for you, then extracts the refreshed attribute values Terraform read back from your providers. It is a live read of your infrastructure, performed by Terraform, using credentials you already have configured. No new permissions, nothing sent anywhere.

> Terraform refreshes only what it manages, so a snapshot built this way can never contain an **unmanaged** resource. For those you need an inventory read — see [docs/live-snapshots.md](docs/live-snapshots.md).

### 2. Find the drift

```bash
infractl drift scan --state terraform.tfstate --live live.json --show-diff
```

```
SEVERITY  KIND             ADDRESS                    TYPE                CHANGES
--------  ---------------  -------------------------  ------------------  -------
CRITICAL  modified         aws_s3_bucket.assets       aws_s3_bucket             2
HIGH      missing in live  aws_db_instance.primary    aws_db_instance
MEDIUM    unmanaged        aws_security_group.orphan  aws_security_group
MEDIUM    modified         aws_subnet.private[1]      aws_subnet                2

aws_s3_bucket.assets [critical]
  - server_side_encryption_configuration.rule = {"sse_algorithm":"AES256"}
  ~ acl "private" -> "public-read"

4 finding(s) across 5 managed resources: 1 critical, 1 high, 2 medium
```

The bucket scores critical because two security-relevant attributes moved at once: encryption was removed and the ACL went public. A tag edit would have scored info.

### 3. Get the commands that fix it

```bash
infractl drift scan --state terraform.tfstate --live live.json --fix
```

Every finding has exactly one of three safe resolutions, determined by its kind, not by judgement:

| Finding | Resolutions offered |
| --- | --- |
| modified | revert with `apply -target`, or accept with `apply -refresh-only -target` |
| missing in live | recreate with `apply -target`, or record the deletion with `-refresh-only` |
| unmanaged | adopt it with a generated `import` block |

Nothing is executed. The tool proposes and prints the blast radius; a person applies. Measuring a risk and then taking it anyway would give up the reason to trust the measurement.

```bash
infractl drift scan ... --emit-import imports.tf   # just the import blocks
```

### 4. Report only what changed

```bash
infractl notify --state terraform.tfstate --live live.json
```

`drift scan` reports every finding, every time. `notify` reports what is **new**
since the last scan, what has just been **fixed**, and what has been open long
enough to need a decision. A finding that is merely still true is not news, and
repeating it nightly is how a channel stops being read.

```
EVENT     TIER     SEVERITY  ADDRESS                    OWNER
--------  -------  --------  -------------------------  ----------
new       page     CRITICAL  aws_s3_bucket.assets       platform
new       notify   HIGH      aws_db_instance.primary    unowned
new       digest   MEDIUM    aws_subnet.private[1]      unowned
```

Run it again with nothing changed and it says so, in one line, and sends
nothing.

| Tier | Default trigger | Meaning |
| --- | --- | --- |
| `page` | new + critical + security | Something changed, it matters, and it changed just now |
| `notify` | new + high | Worth a channel during working hours |
| `digest` | everything else, plus resolved and aging | Batched |

History lives in `.infractl/ledger.jsonl`. Commit it: drift history becomes
reviewable and diffable, and the scan that updates it leaves its own audit
trail. Design notes in [docs/notifications.md](docs/notifications.md).

Notifications carry attribute **paths, never values** — enforced by the type
system, not a flag. See [SECURITY.md](SECURITY.md#the-notification-path).

### If something looks wrong

```bash
infractl doctor
```

Checks the state file, snapshot freshness, ignore-rule expiry, config parsing, and the Terraform binary. Every failure names the command that fixes it.

---

## Features

### The workflow

| Step | Command | What it does |
| --- | --- | --- |
| Evaluate | `demo` | Full pipeline against embedded fixtures. No setup, no credentials, no network |
| Capture | `snapshot from-plan` | Builds the live snapshot from a Terraform refresh-only plan, using credentials you already have |
| Detect | `drift scan` | Modified, deleted, and unmanaged resources with property-level diffs |
| Understand | `graph blast-radius` | Everything that breaks when a resource changes, by distance |
| Resolve | `drift scan --fix` | The commands and import blocks that fix each finding. Runs nothing |
| Suppress | `ignore add` | Writes a suppression rule; the reason is mandatory and the expiry is enforced |
| Report | `notify` | What changed since last time, routed to whoever owns it |
| Diagnose | `doctor` | Validates every input and names the fix for each failure |

### Everything else

| Feature | Command | What it does |
| --- | --- | --- |
| State inspection | `state inspect` | Format version, serial, lineage, provider and type breakdown |
| Resource listing | `state list` | Every managed instance, filterable by type, provider, module |
| Attribute view | `state show` | Full attributes for one resource, secrets masked |
| Plan analysis | `plan analyse` | Create/update/delete/replace counts, destructive changes isolated |
| Risk scoring | `risk assess` | Security, reliability, cost, and compliance scored and weighted |
| Dependency listing | `graph deps` | Upstream or downstream neighbours of a resource |
| Graph export | `graph export` | Graphviz DOT or Mermaid, renderable inline on GitHub |
| Coverage metric | `drift scan` | How much of the observed estate Terraform actually manages |
| GitHub code scanning | `-o sarif` | Findings in the Security tab, annotated on the pull request |

### Correctness details that matter

| Behaviour | Why it exists |
| --- | --- |
| Data sources excluded from drift | Terraform reads them but does not own them; flagging them makes every upstream change look like an unauthorised edit |
| Provider bookkeeping ignored (`id`, `arn`, `tags_all`, `etag`, timestamps) | These never round-trip from a live read; comparing them reports drift on every resource every run |
| `int64(3)` equals `float64(3)` | State decodes with `UseNumber`, live reads often yield floats; a naive comparison flags every numeric attribute |
| Integers past 2^53 keep their precision | Account and snapshot IDs exceed it, and a corrupted ID shows up as permanent phantom drift |
| Replace counted once, as destructive | Terraform encodes it as delete plus create; counting two changes understates that a live resource is going away |
| Encryption checks skip resource types without encryption | A VPC has no encryption setting; a finding nobody can act on teaches users to ignore the tool |
| Single-AZ checks skip subnets | A subnet is single-AZ by definition |
| Live-only attributes are not drift | Cloud APIs return many server-assigned fields Terraform never tracks |
| Sensitive values dropped before output | Masking at render time would still leak them through `-o json` |
| Output sorted deterministically | Two runs over unchanged input produce byte-identical output, so CI can diff or checksum it |

### Terminal experience

| Behaviour | Detail |
| --- | --- |
| Results on stdout, progress on stderr | `infractl ... -o json \| jq` works with no filtering |
| Colour auto-detection | Honours [`NO_COLOR`](https://no-color.org), `FORCE_COLOR`, `TERM=dumb`, `--color`, and TTY detection |
| Machine output is never styled | `-o json` strips ANSI regardless of colour flags |
| Width-aware tables | Columns size to content, then shrink only truncatable columns to fit; identifiers stay whole |
| ANSI-correct alignment | Column widths measure display width, not byte length, so styled cells stay aligned |
| ASCII fallback | `--ascii`, or automatic on a non-UTF-8 console, swaps box-drawing for ASCII |
| Windows VT enabled | Console mode is switched so escapes render instead of printing literally |
| Spinners degrade | Animation on a TTY; one line at start on a pipe, so CI logs stay readable |
| No emoji anywhere | Status is carried by words, colour, and box-drawing, all of which survive a mono terminal and a screen reader |

### Output formats

| Format | Flag | Use |
| --- | --- | --- |
| Table | `-o table` (default) | Reading in a terminal |
| Wide | `-o wide` | Extra columns |
| JSON | `-o json` | Pipelines, `jq` |
| YAML | `-o yaml` | Config-adjacent tooling |
| CSV / TSV | `-o csv`, `-o tsv` | Spreadsheets |
| Name | `-o name` | Shell loops: `for a in $(infractl state list x -o name)` |
| SARIF | `-o sarif` | GitHub code scanning, Security tab alerts |
| Template | `-o go-template='{{.Address}}'` | Custom shaping |

---

### Configuration

A team running this in CI ends up repeating the same flags on every invocation. `.infractl.yaml` holds them instead, so the settings are reviewed like the code is and a pipeline definition stays short enough to read.

```yaml
# .infractl.yaml
state: terraform.tfstate
live: live.json
min-severity: medium

profiles:
  production:
    min-severity: high
    fail-on: high
  audit:
    min-severity: info
    output: json
```

```bash
infractl drift scan                        # uses the file
infractl drift scan --profile production   # applies the production block
```

Precedence, highest first:

| Source | Example |
| --- | --- |
| Command-line flag | `--min-severity critical` |
| Environment variable | `INFRACTL_MIN_SEVERITY=critical` |
| Config file | `min-severity: high` |
| Flag default | `info` |

The search path is `--config`, then `./.infractl.yaml`, then `$HOME/.infractl.yaml`, then `$HOME/.config/infractl/`. A missing file is not an error; a file that exists but does not parse is, because it was clearly meant to be used.

### Pre-commit

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/Ashutosh0x/infra-control
    rev: v0.2.0
    hooks:
      - id: infractl-plan          # reject destructive plans
      - id: infractl-state-check   # catch truncated or half-written state
```

### Suppressing expected drift

Some infrastructure drifts by design: an autoscaling group's capacity moves on its own, a provider assigns a load balancer's IP, a resource mid-decommission disagrees with state until it is removed. Reporting these every run is the failure mode that gets drift tooling switched off, because the real findings get lost among the ones nobody can act on.

`.infractl-ignore.yaml` suppresses them:

```yaml
version: 1
rules:
  - address: "aws_autoscaling_group.*"
    attributes: [desired_capacity, min_size]
    reason: "Capacity is managed by the autoscaling policy, not Terraform"

  - address: "aws_instance.bastion"
    reason: "Decommissioning, tracked in INFRA-4821"
    expires: "2026-12-31"
```

Three rules keep this from becoming a way to hide problems:

| Property | Effect |
| --- | --- |
| A reason is required | A rule without one is a config error, not a silent default, so the file stays reviewable |
| Expiry is enforced | Past the date the rule stops suppressing and the scan says so, which stops a temporary exception becoming permanent |
| Suppression is counted | Every scan reports how many findings were hidden and which rule hid each; `--no-ignore` shows them |

An attribute-scoped rule suppresses a finding only if it covers **every** changed attribute. A resource where an expected attribute and an unexpected one both moved is still reported, because the unexpected one is the finding.

```bash
infractl drift scan --state terraform.tfstate --live live.json            # nearest ignore file
infractl drift scan --state terraform.tfstate --live live.json --no-ignore # audit what is hidden
```

### GitHub code scanning

`-o sarif` emits SARIF 2.1.0, so drift lands in the repository's Security tab next to CodeQL, annotated on the pull request that introduced it, with GitHub tracking which findings are new, fixed, or still open across runs.

```yaml
      - name: Drift scan
        run: |
          infractl drift scan --state terraform.tfstate --live live.json             -o sarif > drift.sarif
        continue-on-error: true

      - uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: drift.sarif
          category: infractl-drift
```

A clean scan still uploads a valid document with zero results, which is how GitHub learns the previous findings are fixed rather than merely absent.

## Command reference

```
infractl
├── demo                            Full pipeline against embedded fixtures, no setup
├── doctor                          Check inputs and environment; names the fix for each failure
│
├── snapshot
│   └── from-plan [plan.json]       Build a live snapshot from a refresh-only plan
│
├── state
│   ├── inspect <file>              Format version, serial, lineage, type breakdown
│   ├── list <file>                 Every managed resource instance
│   └── show <file> <address>       One resource's attributes, secrets masked
│
├── drift
│   └── scan                        Compare state against a live snapshot
│       --show-diff                   property-level diff per finding
│       --fix                         commands and import blocks that resolve each finding
│       --emit-import <file>          write import blocks for unmanaged resources
│       --fail-on <severity>          exit 3 at or above this severity
│
├── plan
│   └── analyse <plan.json>         What a plan changes, and what it destroys
│
├── risk
│   └── assess                      Security, reliability, cost, compliance
│
├── graph
│   ├── stats                       Node and edge counts, roots and leaves
│   ├── blast-radius <address>      What breaks if this changes, by distance
│   ├── deps <address>              What this depends on
│   └── export                      Graphviz DOT or Mermaid
│
├── ignore
│   ├── add <address> --reason      Write a suppression rule; the reason is mandatory
│   └── list                        Active and expired rules
│
├── notify                          What changed since the last scan, not everything
│       --sink slack|github|webhook    where to deliver
│       --no-save                      preview without recording history
│
└── version                         Version, commit, build information
```

Seven commands for the unbuilt control plane (`discover`, `policy`, `compliance`, `cost`, `security`, `remediate`, `audit`) are **excluded from default builds**, so `--help` lists only what the tool can do. Build with `-tags preview` to see them; they return an explicit error rather than a placeholder. See [Project status](#project-status).

### The two-minute path

```bash
infractl demo                                    # see it work, no setup
infractl snapshot from-plan                      # capture your own live state
infractl drift scan --state terraform.tfstate --live live.json --fix
infractl doctor                                  # if anything looks wrong
```

### Global flags

| Flag | Default | Purpose |
| --- | --- | --- |
| `-o, --output` | `table` | `table`, `wide`, `json`, `yaml`, `csv`, `tsv`, `name`, `sarif`, `go-template=TMPL` |
| `-q, --quiet` | off | Suppress progress; results and errors still print |
| `-v, --verbose` | off | Diagnostic logging on stderr |
| `--color` | `auto` | `auto`, `always`, `never` |
| `--no-color` | off | Shorthand for `--color=never` |
| `--ascii` | off | ASCII symbols instead of box-drawing |
| `--config` | | Config file path |
| `--profile` | | Named block from the config file |

Every flag has an `INFRACTL_`-prefixed environment variable and a config-file equivalent, in that precedence order. `--min-severity` reads `INFRACTL_MIN_SEVERITY`.

---

## Continuous integration

Exit codes separate "the command failed" from "the command worked and found something":

| Code | Meaning |
| --- | --- |
| `0` | Success, nothing found |
| `1` | The command failed |
| `2` | Invalid arguments or flags |
| `3` | Success, but findings exceeded the threshold |
| `4` | Required configuration or credentials missing |
| `5` | A required backend was unreachable |

### GitHub Actions

```yaml
name: infrastructure
on: [pull_request]

jobs:
  drift:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.25' }
      - run: go install github.com/ashutosh0x/infra-control/cmd/infractl@latest

      - name: Reject plans that destroy anything
        run: |
          terraform plan -out=tfplan
          terraform show -json tfplan > plan.json
          infractl plan analyse plan.json --fail-on destructive

      - name: Fail on high-severity drift
        run: infractl drift scan --state terraform.tfstate --live live.json --fail-on high
```

Because exit 3 is distinct from exit 1, a job can treat findings and failures differently:

```bash
infractl drift scan --state terraform.tfstate --live live.json --fail-on high -o json > drift.json
case $? in
  0) echo "clean" ;;
  3) gh pr comment "$PR" --body-file <(jq -r '.findings[] | "- \(.severity): \(.address)"' drift.json) ;;
  *) echo "scan failed" >&2; exit 1 ;;
esac
```

More recipes in [docs/ci-integration.md](docs/ci-integration.md).

---

## Tech stack

### What the CLI is built from

The whole of local analysis is these. Everything below is linked into the 16 MB static binary you install.

| Purpose | Technology |
| --- | --- |
| Language | [![Go](https://img.shields.io/badge/Go%201.25-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev) |
| Commands, flags, completion | [![Cobra](https://img.shields.io/badge/Cobra-2E9E8F?style=for-the-badge&logo=go&logoColor=white)](https://cobra.dev) |
| Config files, env precedence | [![Viper](https://img.shields.io/badge/Viper-5C3EE8?style=for-the-badge&logo=go&logoColor=white)](https://github.com/spf13/viper) |
| Terminal detection and width | [![x/term](https://img.shields.io/badge/golang.org%2Fx%2Fterm-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://pkg.go.dev/golang.org/x/term) |
| Config and rule parsing | [![YAML](https://img.shields.io/badge/yaml.v3-CB171E?style=for-the-badge&logo=yaml&logoColor=white)](https://gopkg.in/yaml.v3) |
| Structured logging | [![Zap](https://img.shields.io/badge/Zap-0E76A8?style=for-the-badge&logo=go&logoColor=white)](https://github.com/uber-go/zap) |

Tables, colour, diffs, spinners, SARIF, the state and plan parsers, the dependency graph, and the notification ledger are all standard library. No TUI framework, no HTTP client library, no cloud SDK.

### What it reads and writes

| | |
| --- | --- |
| Infrastructure as code | [![Terraform](https://img.shields.io/badge/Terraform-844FBA?style=for-the-badge&logo=terraform&logoColor=white)](https://terraform.io) [![OpenTofu](https://img.shields.io/badge/OpenTofu-FFDA18?style=for-the-badge&logo=opentofu&logoColor=black)](https://opentofu.org) |
| Findings out | [![SARIF](https://img.shields.io/badge/SARIF%202.1.0-2088FF?style=for-the-badge&logo=github&logoColor=white)](https://sarifweb.azurewebsites.net) [![JSON](https://img.shields.io/badge/JSON-000000?style=for-the-badge&logo=json&logoColor=white)](https://json.org) |
| Notifications out | [![Slack](https://img.shields.io/badge/Slack-4A154B?style=for-the-badge&logo=slack&logoColor=white)](https://slack.com) [![GitHub](https://img.shields.io/badge/GitHub-181717?style=for-the-badge&logo=github&logoColor=white)](https://github.com) [![Webhook](https://img.shields.io/badge/Signed%20webhook-6E7681?style=for-the-badge&logo=webhooks&logoColor=white)](SECURITY.md#webhook-signing) |
| Diagrams | [![Graphviz](https://img.shields.io/badge/Graphviz-165C9E?style=for-the-badge&logo=graphviz&logoColor=white)](https://graphviz.org) [![Mermaid](https://img.shields.io/badge/Mermaid-FF3670?style=for-the-badge&logo=mermaid&logoColor=white)](https://mermaid.js.org) |

### How it ships

| | |
| --- | --- |
| Build and release | [![GoReleaser](https://img.shields.io/badge/GoReleaser-317F6F?style=for-the-badge&logo=goreleaser&logoColor=white)](https://goreleaser.com) [![GitHub Actions](https://img.shields.io/badge/GitHub%20Actions-2088FF?style=for-the-badge&logo=githubactions&logoColor=white)](https://github.com/features/actions) |
| Install | [![Homebrew](https://img.shields.io/badge/Homebrew-FBB040?style=for-the-badge&logo=homebrew&logoColor=black)](https://brew.sh) [![Scoop](https://img.shields.io/badge/Scoop-555555?style=for-the-badge&logo=windows&logoColor=white)](https://scoop.sh) [![Docker](https://img.shields.io/badge/Distroless-2496ED?style=for-the-badge&logo=docker&logoColor=white)](https://github.com/GoogleContainerTools/distroless) [![pkg.go.dev](https://img.shields.io/badge/pkg.go.dev-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://pkg.go.dev/github.com/ashutosh0x/infra-control) |
| Quality gates | [![golangci-lint](https://img.shields.io/badge/golangci--lint-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://golangci-lint.run) [![CodeQL](https://img.shields.io/badge/CodeQL-2088FF?style=for-the-badge&logo=github&logoColor=white)](https://codeql.github.com) |

### Not in the binary

`cmd/controller`, `cmd/worker`, and `cmd/mcp-server` are **skeletons**. They import PostgreSQL, Redis, NATS, gRPC, OpenTelemetry, and Prometheus, and none of that is linked into `infractl`. Verify it yourself:

```bash
go list -deps ./cmd/infractl | grep -E 'pgx|redis|nats|grpc|prometheus'   # no output
```

They are listed here only so the dependency list in `go.mod` is not mistaken for the CLI's.

---

## Architecture

```mermaid
flowchart LR
    subgraph inputs["Inputs, read-only"]
        direction TB
        stateFile["terraform.tfstate"]
        liveFile["live.json<br/>snapshot"]
        planFile["plan.json"]
    end

    subgraph binary["infractl, one static binary"]
        direction TB
        tfPkg["terraform<br/>state and plan parsing<br/>attribute comparison"]
        graphPkg["graph<br/>dependency edges<br/>blast radius"]
        riskPkg["risk<br/>four-dimension scoring"]
        ignorePkg["ignore<br/>suppression rules"]
        notifyPkg["notify<br/>ledger, events<br/>ownership, routing"]
        uiPkg["ui<br/>tables, diffs, formats"]

        tfPkg --> graphPkg
        tfPkg --> riskPkg
        tfPkg --> ignorePkg
        ignorePkg --> notifyPkg
        graphPkg --> uiPkg
        riskPkg --> uiPkg
        notifyPkg --> uiPkg
    end

    subgraph outputs["Outputs"]
        direction TB
        outStream["stdout<br/>table, json, yaml, csv, sarif"]
        errStream["stderr<br/>progress, warnings"]
        exitCode["exit code<br/>0 clean, 3 findings, 1 failed"]
        sinks["sinks<br/>Slack, GitHub, webhook"]
    end

    stateFile --> tfPkg
    liveFile --> tfPkg
    planFile --> tfPkg
    uiPkg --> outStream
    uiPkg --> errStream
    uiPkg --> exitCode
    notifyPkg --> sinks

    classDef inputNode fill:#e8f4f8,stroke:#0969da,color:#111
    classDef coreNode fill:#f0f0ff,stroke:#8250df,color:#111
    classDef outputNode fill:#eaf5ea,stroke:#1a7f37,color:#111
    class stateFile,liveFile,planFile inputNode
    class tfPkg,graphPkg,riskPkg,ignorePkg,notifyPkg,uiPkg coreNode
    class outStream,errStream,exitCode,sinks outputNode
```

The only outbound network traffic this binary makes is to a sink you configure. Analysis opens no sockets.

Local analysis touches no network and no server component. The optional hosted control plane is separate:

```mermaid
flowchart TB
    subgraph plane["Control plane, optional and not yet implemented"]
        direction LR
        controller[controller<br/>scheduled scans<br/>event loops]
        worker[worker<br/>queue consumers]
        mcp[mcp-server<br/>Model Context Protocol]
    end

    subgraph stores["Backing services"]
        direction LR
        pg[(PostgreSQL<br/>resources, drift, audit)]
        redis[(Redis<br/>cache, locks)]
        nats{{NATS<br/>event bus}}
    end

    controller --> pg
    controller --> nats
    worker --> nats
    worker --> pg
    mcp --> pg
    controller --> redis

    classDef svc fill:#fff4e6,stroke:#bc4c00,color:#111
    classDef store fill:#f6f8fa,stroke:#57606a,color:#111
    class controller,worker,mcp svc
    class pg,redis,nats store
```

These are skeletons, excluded from default builds. See [Project status](#project-status).

---

## Project status

Honest accounting of what works.

| Area | Status |
| --- | --- |
| State and plan parsing, drift detection, risk scoring, dependency graph | Implemented, tested |
| Snapshot capture from a refresh-only plan | Implemented, tested |
| Suppression rules, remediation output, coverage metric | Implemented, tested |
| Notifications: ledger, events, ownership, routing, Slack/GitHub/webhook | Implemented, tested |
| Terminal UI, eight output formats, exit codes, config files | Implemented, tested |
| Live cloud API reads | **Not implemented.** Drift compares against a snapshot file; `snapshot from-plan` builds one from Terraform itself |
| Unmanaged-resource detection from a refresh-only snapshot | **Structurally impossible.** Terraform refreshes only what it manages; this needs an inventory read |
| Interactive bot, slash commands | Not implemented. One-way delivery only, by design; see [docs/notifications.md](docs/notifications.md) |
| Policy engine (OPA/Rego), compliance, cost | Not implemented, and not planned. Checkov and Trivy own static policy; Infracost owns cost |
| Control-plane server, worker, MCP | Skeletons, excluded from default builds |

Commands without a working implementation return an error saying so. They never print a placeholder result, because a placeholder is indistinguishable from a real answer to whoever reads the output — and infrastructure decisions get made on that output.

---

## Contributing

Issues and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the development setup, and [Discussions](https://github.com/Ashutosh0x/infra-control/discussions) for questions and design proposals.

```bash
make build      # compile
make test       # run tests
make lint       # golangci-lint
make check      # all of the above
```

## Security

To report a vulnerability, follow [SECURITY.md](SECURITY.md). Please do not open a public issue for security reports.

## License

[Apache 2.0](LICENSE).
