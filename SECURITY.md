# Security Policy

## Reporting a vulnerability

Do not open a public issue for a security report.

Use GitHub's private reporting: **[Report a vulnerability](https://github.com/Ashutosh0x/infra-control/security/advisories/new)**. It creates a draft advisory visible only to you and the maintainers.

If private reporting is unavailable, email **ashutoshkumarsingh951@gmail.com** with `[infra-control security]` in the subject.

### What to include

- The version, from `infractl version`
- The affected component: CLI, a specific package, or a server component
- Steps to reproduce, ideally the smallest input that triggers it
- What an attacker gains
- Any state or plan file needed to reproduce, **with real identifiers and secrets removed**

### What to expect

| Stage | Target |
| --- | --- |
| Acknowledgement | 48 hours |
| Initial assessment | 5 working days |
| Fix or mitigation plan | 30 days for high and critical |
| Public advisory | After a fix ships, coordinated with you |

You will be credited in the advisory unless you ask otherwise.

## Supported versions

| Version | Supported |
| --- | --- |
| `main` | Yes |
| Latest release | Yes |
| Older releases | No |

This project is pre-1.0. Fixes land on `main` and in the next release rather than being backported.

## Scope

### In scope

- Disclosure of secrets read from state, plan, or snapshot files
- Path traversal or arbitrary file read through a file argument
- Code execution triggered by parsing a malicious state, plan, or snapshot
- Denial of service from a crafted input file, including memory exhaustion
- Privilege or authentication flaws in the server components
- Credential leakage into logs, telemetry, or error messages
- Dependency vulnerabilities reachable from a supported code path

### Out of scope

- Vulnerabilities in Terraform, OpenTofu, or a cloud provider's own tooling
- Findings that require an attacker to already control the machine running `infractl`
- The absence of a hardening measure with no demonstrated exploit
- Results from an automated scanner with no working proof of concept
- Social engineering, physical access, or attacks on project infrastructure
- Reports against unimplemented commands, which return an error and do nothing

## Handling of sensitive data

These properties are enforced in code and covered by tests. A regression in any of them is a security bug.

### Secrets are dropped, not masked at render time

Attributes whose names indicate a secret — `password`, `secret`, `token`, `private_key`, `access_key`, `client_secret`, `connection_string`, and similar — never reach the output payload. The value is discarded during comparison, before any encoder runs.

This ordering is deliberate. Masking at render time would leave the real value in the structured payload, so `-o json` would still leak it while the table looked safe.

```bash
infractl state show terraform.tfstate aws_db_instance.primary -o json
# "password": "(sensitive value hidden)"
```

Detection is name-based, not provider-declared. Treat all output from `state show` as sensitive regardless.

### No network egress from local analysis

`state`, `drift`, `plan`, `risk`, and `graph` read local files and write to local streams. They open no sockets. Nothing is sent for telemetry, analytics, or update checks. Verify with a network monitor or by running with the network disabled.

### Input parsing is bounded

State, plan, and snapshot files are parsed with the standard library JSON decoder. Unknown state format versions are rejected rather than parsed on a best-effort basis, because a partially understood state produces a resource list wrong in ways drift detection cannot detect.

### Files are read, never written

No analysis command writes to a state file. Running against a copy of production state is safe, and running against production state itself does not modify it.

## Building from source

```bash
git clone https://github.com/Ashutosh0x/infra-control.git
cd infra-control
go build ./cmd/infractl
```

Released binaries are built by GoReleaser in GitHub Actions from a tagged commit. Checksums are published with each release:

```bash
sha256sum -c checksums.txt --ignore-missing
```

## Hardening notes for operators

- Run `infractl` with read-only access to state files.
- State files contain every attribute of every resource, secrets included. Treat a state file as a credential store: the risk of it leaking is not created by this tool, but this tool reads it.
- In CI, prefer `-o json` with `--quiet` so that only the intended payload reaches the log.
- The server components under `cmd/controller`, `cmd/worker`, and `cmd/mcp-server` are skeletons. Do not deploy them to an environment handling real data.
