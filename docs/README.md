# Documentation

| Document | Read it when |
| --- | --- |
| [cli-reference.md](cli-reference.md) | You want every command, flag, and environment variable |
| [live-snapshots.md](live-snapshots.md) | You need to produce the snapshot drift detection compares against |
| [ci-integration.md](ci-integration.md) | You are wiring this into GitHub Actions, GitLab, Jenkins, or pre-commit |
| [notifications.md](notifications.md) | You want to understand the event model, or argue with it |
| [../SECURITY.md](../SECURITY.md) | You are reviewing what leaves the machine, or reporting a vulnerability |
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | You are sending a pull request |

## The short version

```bash
infractl demo                 # see it work against embedded fixtures
infractl snapshot from-plan   # capture your own live state
infractl drift scan --state terraform.tfstate --live live.json --fix
infractl doctor               # when something looks wrong
```

## Three things worth knowing before you start

**The snapshot is the input you have to supply.** Drift detection compares a state file against a live snapshot. `snapshot from-plan` builds one from a Terraform refresh-only plan, which needs no permissions beyond the ones Terraform already has. It cannot see unmanaged resources; nothing that reads only Terraform can. [live-snapshots.md](live-snapshots.md) covers the alternatives.

**Exit 3 is not failure.** A scan that runs correctly and finds drift exits 3. A scan that could not run exits 1. Keeping those apart is what lets a pipeline block a merge on drift without treating a crashed scanner as a clean run. [ci-integration.md](ci-integration.md).

**Nothing is applied.** Every remediation path is printed for a human to run, with the blast radius beside it. A tool that measures a risk and then takes it anyway has given up the reason to trust the measurement.
