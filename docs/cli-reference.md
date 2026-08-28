# CLI reference

Every command supports the global flags. Every command that produces a list supports every output format.

## Global flags

| Flag | Default | Description |
| --- | --- | --- |
| `-o, --output` | `table` | `table`, `wide`, `json`, `yaml`, `csv`, `tsv`, `name`, `go-template=TMPL` |
| `-q, --quiet` | off | Suppress progress. Results and errors still print |
| `-v, --verbose` | off | Diagnostic logging on stderr |
| `--color` | `auto` | `auto`, `always`, `never` |
| `--no-color` | off | Shorthand for `--color=never` |
| `--ascii` | off | ASCII symbols instead of box-drawing characters |
| `--config` | | Config file path |
| `--profile` | | Named configuration profile |

Colour resolution order: `--color` wins, then `NO_COLOR`, then `TERM=dumb`, then `FORCE_COLOR`, then TTY detection. Machine formats are never coloured regardless.

---

## state

Read-only inspection of Terraform and OpenTofu state. These commands never write to the state file.

### state inspect

```
infractl state inspect <state-file>
```

Format version, the Terraform version that wrote it, serial, lineage, managed resource count, providers, output count, and a per-type breakdown.

```bash
infractl state inspect terraform.tfstate
infractl state inspect terraform.tfstate -o json
```

### state list

```
infractl state list <state-file> [flags]
```

Every managed resource instance. Data sources are excluded: Terraform reads them but does not own them.

| Flag | Description |
| --- | --- |
| `--type` | Filter by resource type, comma-separated |
| `--provider` | Filter by provider, comma-separated |
| `--module` | Filter to modules whose path contains this string |

```bash
infractl state list terraform.tfstate
infractl state list terraform.tfstate --type aws_s3_bucket,aws_rds_instance
infractl state list terraform.tfstate --provider aws -o name
```

`-o name` prints bare addresses, which is what you want for a shell loop:

```bash
for addr in $(infractl state list terraform.tfstate -o name); do
  infractl graph blast-radius "$addr" --state terraform.tfstate -o json
done
```

### state show

```
infractl state show <state-file> <address>
```

Every attribute Terraform records for one resource. Attributes whose names indicate a secret are replaced with `(sensitive value hidden)` before the payload is built, so `-o json` does not leak what the table hides.

Detection is name-based, not provider-declared. Treat all output as sensitive.

```bash
infractl state show terraform.tfstate aws_s3_bucket.assets
infractl state show terraform.tfstate 'module.vpc.aws_subnet.private[0]'
```

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
| `--fail-on` | `none` | Exit 3 when a finding at or above this severity exists. `none`, `any`, `low`, `medium`, `high`, `critical` |
| `--show-diff` | off | Print the property-level diff for each finding |
| `--include-unmanaged` | on | Report live resources Terraform does not track |

Three kinds of finding:

| Kind | Meaning |
| --- | --- |
| `modified` | In both, attributes disagree |
| `missing_in_live` | State records it; it no longer exists |
| `unmanaged` | Live but not tracked by Terraform |

```bash
infractl drift scan --state terraform.tfstate --live live.json
infractl drift scan --state terraform.tfstate --live live.json --show-diff
infractl drift scan --state terraform.tfstate --live live.json --min-severity high
infractl drift scan --state terraform.tfstate --live live.json --fail-on high
```

A snapshot older than 24 hours produces a staleness warning on stderr, because drift found against stale data may already have been fixed.

---

## plan

### plan analyse

```
infractl plan analyse <plan.json> [flags]
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
| `--max-deletes` | `-1` | Exit 3 if more than n deletions. `-1` disables |

Without `--details` the table shows only destructive changes, because those are what a reviewer must actually look at. The summary line always covers everything.

A **replace** counts once and as destructive. Terraform encodes it as delete plus create, and the delete is real: the live resource and everything depending on it goes away, however briefly.

Two warnings are raised automatically:

- A plan that only deletes, which usually means the wrong workspace is selected or state has lost track of its resources.
- A stateful resource being deleted or replaced, since its data does not survive.

```bash
infractl plan analyse plan.json
infractl plan analyse plan.json --details
infractl plan analyse plan.json --fail-on destructive
infractl plan analyse plan.json --max-deletes 3
```

---

## risk

### risk assess

```
infractl risk assess --state <file> [flags]
```

Score every managed resource across four dimensions, then combine them.

| Dimension | Weight | Checks |
| --- | --- | --- |
| Security | 0.35 | Public exposure, missing encryption, not IaC-managed |
| Reliability | 0.30 | Single-AZ deployment, absent backups |
| Compliance | 0.20 | Missing `environment`, `owner`, `cost-center` tags |
| Cost | 0.15 | Instance sizes suggesting overprovisioning |

| Flag | Default | Description |
| --- | --- | --- |
| `--state` | required | State file |
| `--min-level` | `negligible` | Only report at or above this level |
| `--fail-on` | `none` | Exit 3 at or above this level |
| `--show-factors` | off | Print the reasons behind each score |
| `--top` | `0` | Show only the N highest-scoring. `0` shows all |

Checks are applied only where the underlying concept exists. A VPC has no encryption-at-rest setting, and a subnet is single-AZ by definition, so neither is scored against those checks. A finding nobody can act on teaches users to ignore the tool.

Scoring reads only what state records. A resource whose risk depends on something outside state, such as an IAM policy defined elsewhere, is scored only on the attributes present.

```bash
infractl risk assess --state terraform.tfstate
infractl risk assess --state terraform.tfstate --min-level high
infractl risk assess --state terraform.tfstate --top 10 --show-factors
infractl risk assess --state terraform.tfstate --fail-on critical
```

---

## graph

Dependency edges come from what Terraform recorded when it applied, capturing both explicit `depends_on` and the implicit edges from attribute references.

A dependency on something outside the state file, such as a data source, has no node to point at and is skipped rather than invented.

`--state` is required for every subcommand.

### graph stats

```
infractl graph stats --state <file>
```

Node and edge counts, roots (nothing depends on them) and leaves (they depend on nothing).

### graph blast-radius

```
infractl graph blast-radius <address> --state <file> [--max-depth N]
```

Aliased as `blast` and `impact`.

Everything that depends on a resource, directly or transitively, with the distance in hops. Distance 1 is a direct dependent.

State records dependencies at block granularity, without the `count` index. A dependency on `aws_subnet.private` fans out to every instance of that block, so the traversal does not stop short.

```bash
infractl graph blast-radius aws_vpc.main --state terraform.tfstate
infractl graph blast-radius aws_vpc.main --state terraform.tfstate --max-depth 2
```

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
infractl graph export --state terraform.tfstate --format mermaid
```

Mermaid output renders inline in GitHub Markdown.

---

## version

```
infractl version [-o json]
```

Version, commit, build date, Go toolchain, and platform. Include this output when reporting a bug.

---

## Commands requiring a server

`discover`, `policy`, `compliance`, `cost`, `security`, `remediate`, and `audit` are present in the CLI surface but have no working backend in this build. They return exit 4 with an explanation.

They do not print placeholder results. A placeholder is indistinguishable from a real answer at the point of reading, and infrastructure decisions get made on that output.
