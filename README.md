<div align="center">

# infra-control

**Read your Terraform state. Compare it to reality. Know what broke, what it costs you, and what it takes down with it.**

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![Terraform](https://img.shields.io/badge/Terraform-844FBA?style=flat&logo=terraform&logoColor=white)](https://www.terraform.io)
[![OpenTofu](https://img.shields.io/badge/OpenTofu-FFDA18?style=flat&logo=opentofu&logoColor=black)](https://opentofu.org)
[![License](https://img.shields.io/badge/License-Apache_2.0-D22128?style=flat&logo=apache&logoColor=white)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/Ashutosh0x/infra-control/ci.yml?branch=main&style=flat&logo=githubactions&logoColor=white&label=CI)](https://github.com/Ashutosh0x/infra-control/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ashutosh0x/infra-control?style=flat)](https://goreportcard.com/report/github.com/ashutosh0x/infra-control)

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

### Go install

```bash
go install github.com/ashutosh0x/infra-control/cmd/infractl@latest
```

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

### 1. See what state holds

```bash
infractl state inspect terraform.tfstate
```

```
  state file:         terraform.tfstate
  format version:     4
  written by:         Terraform 1.9.5
  serial:             42
  managed resources:  5
  providers:          aws
  outputs:            2 (1 sensitive)

Resource types
--------------
TYPE                  COUNT
--------------------  -----
aws_subnet                2
aws_db_instance           1
aws_s3_bucket             1
aws_vpc                   1
```

### 2. Capture what is actually live

Drift detection compares state against a **live snapshot**: a JSON file mapping Terraform addresses to the attributes observed on the real resource.

```json
{
  "captured_at": "2026-08-29T09:00:00Z",
  "provider": "aws",
  "resources": {
    "aws_s3_bucket.assets": { "bucket": "prod-assets", "acl": "public-read" }
  }
}
```

Keeping this a file rather than a built-in cloud call is deliberate. It means drift detection runs in a pipeline with no cloud credentials, and the snapshot can be produced by whatever already has read access — a read-only role, an existing inventory export, or Steampipe. See [docs/live-snapshots.md](docs/live-snapshots.md).

### 3. Find the drift

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

aws_subnet.private[1] [medium]
  ~ cidr_block "10.0.2.0/24" -> "10.0.99.0/24"
  ~ tags.environment "production" -> "staging"

4 finding(s) across 5 managed resources: 1 critical, 1 high, 2 medium
```

The bucket scores critical because two security-relevant attributes moved at once: encryption was removed and the ACL went public. A tag edit on the same resource would have scored info.

---

## Features

### Analysis, no server required

| Feature | Command | What it does |
| --- | --- | --- |
| State inspection | `state inspect` | Format version, serial, lineage, provider and type breakdown |
| Resource listing | `state list` | Every managed instance, filterable by type, provider, module |
| Attribute view | `state show` | Full attributes for one resource, secrets masked |
| Drift detection | `drift scan` | Modified, deleted, and unmanaged resources with property-level diffs |
| Plan analysis | `plan analyse` | Create/update/delete/replace counts, destructive-change isolation |
| Risk scoring | `risk assess` | Security, reliability, cost, and compliance scored and weighted |
| Blast radius | `graph blast-radius` | Everything that breaks when a resource changes, by distance |
| Dependency listing | `graph deps` | Upstream or downstream neighbours of a resource |
| Graph export | `graph export` | Graphviz DOT or Mermaid, renderable inline on GitHub |

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
| Template | `-o go-template='{{.Address}}'` | Custom shaping |

---

## Command reference

```
infractl
├── state
│   ├── inspect <file>              Summarise a state file
│   ├── list <file>                 List managed resources
│   └── show <file> <address>       Show one resource's attributes
├── drift
│   └── scan                        Compare state against a live snapshot
├── plan
│   └── analyse <plan.json>         Report what a plan changes and destroys
├── risk
│   └── assess                      Score resources across four dimensions
├── graph
│   ├── stats                       Node and edge counts
│   ├── blast-radius <address>      What breaks if this changes
│   ├── deps <address>              What this depends on
│   └── export                      DOT or Mermaid
└── version                         Version, commit, build info
```

Commands for the hosted control plane (`discover`, `policy`, `compliance`, `cost`, `security`, `remediate`, `audit`) are present in the CLI surface but return an explicit error stating that the server is not configured. They do not print placeholder results. See [Project status](#project-status).

### Global flags

| Flag | Default | Purpose |
| --- | --- | --- |
| `-o, --output` | `table` | Output format |
| `-q, --quiet` | off | Suppress progress; results and errors still print |
| `-v, --verbose` | off | Diagnostic logging on stderr |
| `--color` | `auto` | `auto`, `always`, `never` |
| `--no-color` | off | Shorthand for `--color=never` |
| `--ascii` | off | ASCII symbols instead of box-drawing |
| `--config` | | Config file path |

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

| Layer | Technology |
| --- | --- |
| Language | [![Go](https://img.shields.io/badge/Go_1.25-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev) |
| CLI | [![Cobra](https://img.shields.io/badge/Cobra-38B2AC?style=flat&logo=go&logoColor=white)](https://cobra.dev) [![Viper](https://img.shields.io/badge/Viper-4B32C3?style=flat&logo=go&logoColor=white)](https://github.com/spf13/viper) |
| IaC | [![Terraform](https://img.shields.io/badge/Terraform-844FBA?style=flat&logo=terraform&logoColor=white)](https://terraform.io) [![OpenTofu](https://img.shields.io/badge/OpenTofu-FFDA18?style=flat&logo=opentofu&logoColor=black)](https://opentofu.org) |
| Storage | [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-4169E1?style=flat&logo=postgresql&logoColor=white)](https://postgresql.org) [![Redis](https://img.shields.io/badge/Redis-FF4438?style=flat&logo=redis&logoColor=white)](https://redis.io) |
| Messaging | [![NATS](https://img.shields.io/badge/NATS-27AAE1?style=flat&logo=natsdotio&logoColor=white)](https://nats.io) |
| Observability | [![OpenTelemetry](https://img.shields.io/badge/OpenTelemetry-425CC7?style=flat&logo=opentelemetry&logoColor=white)](https://opentelemetry.io) [![Prometheus](https://img.shields.io/badge/Prometheus-E6522C?style=flat&logo=prometheus&logoColor=white)](https://prometheus.io) [![Zap](https://img.shields.io/badge/Zap-0E76A8?style=flat&logo=go&logoColor=white)](https://github.com/uber-go/zap) |
| Transport | [![gRPC](https://img.shields.io/badge/gRPC-244C5A?style=flat&logo=grpc&logoColor=white)](https://grpc.io) |
| Packaging | [![Docker](https://img.shields.io/badge/Docker-2496ED?style=flat&logo=docker&logoColor=white)](https://docker.com) [![GoReleaser](https://img.shields.io/badge/GoReleaser-317F6F?style=flat&logo=goreleaser&logoColor=white)](https://goreleaser.com) |
| CI | [![GitHub Actions](https://img.shields.io/badge/GitHub_Actions-2088FF?style=flat&logo=githubactions&logoColor=white)](https://github.com/features/actions) [![golangci-lint](https://img.shields.io/badge/golangci--lint-00ADD8?style=flat&logo=go&logoColor=white)](https://golangci-lint.run) |

The CLI itself depends on Cobra, Viper, `golang.org/x/term`, and the standard library. Postgres, Redis, NATS, and gRPC belong to the server components and are not linked into local analysis paths.

---

## Architecture

```
                      ┌────────────────────────────────────┐
    terraform.tfstate │  infractl (single static binary)   │
    live.json ───────>│                                    │
    plan.json         │  internal/terraform  state, plan   │
                      │  internal/graph      dependencies  │
                      │  internal/risk       scoring       │
                      │  internal/ui         presentation  │
                      └──────────────┬─────────────────────┘
                                     │
                     stdout: results (table, json, yaml, csv)
                     stderr: progress, warnings, errors
                     exit:   0 clean · 3 findings · 1 failed

    ── optional, for the hosted control plane ──────────────
    controller  scheduled scans, event loops
    worker      queue consumers
    mcp-server  Model Context Protocol interface
    Postgres · Redis · NATS
```

Local analysis touches none of the server components. Details in [docs/architecture.md](docs/architecture.md).

---

## Project status

Honest accounting of what works.

| Area | Status |
| --- | --- |
| State parsing, drift detection, plan analysis, risk scoring, graph | Implemented and tested |
| Terminal UI, output formats, exit codes | Implemented and tested |
| Cloud provider discovery (live API reads) | Not implemented; drift takes a snapshot file instead |
| Policy engine (OPA/Rego), compliance, cost, remediation | Data model and CLI surface exist; no working backend |
| Control-plane server, controller, worker, MCP | Skeletons |

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
