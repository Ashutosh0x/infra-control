# CI integration

## Exit codes

The design point is that **finding something is not the same as failing**. A scan that runs correctly and detects drift exits 3; a scan that could not run exits 1. A pipeline can react to those differently.

| Code | Meaning | Typical response |
| --- | --- | --- |
| 0 | Ran, found nothing | Continue |
| 1 | Failed | Fail the build, page whoever owns the pipeline |
| 2 | Bad arguments | Fail the build, fix the invocation |
| 3 | Ran, found something over the threshold | Block the merge, comment on the PR |
| 4 | Missing configuration or credentials | Fail the build, fix the environment |
| 5 | Backend unreachable | Retry, or fail the build |

Without this split, a broken scanner and a clean scan both look like success, or a real finding and a crashed binary both look like failure. Neither is acceptable for a check that gates a deploy.

## Gating flags

| Flag | Command | Exits 3 when |
| --- | --- | --- |
| `--fail-on <severity>` | `drift scan` | A finding at or above that severity exists |
| `--fail-on any` | `drift scan` | Any finding exists |
| `--fail-on destructive` | `plan analyse` | The plan deletes or replaces anything |
| `--fail-on delete` | `plan analyse` | The plan deletes anything |
| `--fail-on any` | `plan analyse` | The plan changes anything |
| `--max-deletes <n>` | `plan analyse` | More than n resources would be deleted |
| `--fail-on <level>` | `risk assess` | A resource at or above that risk level exists |

## GitHub Actions

### Block destructive plans

```yaml
name: terraform
on: pull_request

jobs:
  plan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: hashicorp/setup-terraform@v3
      - uses: actions/setup-go@v5
        with: { go-version: '1.25' }
      - run: go install github.com/ashutosh0x/infra-control/cmd/infractl@latest

      - name: Plan
        run: |
          terraform init
          terraform plan -out=tfplan
          terraform show -json tfplan > plan.json

      - name: Reject destructive changes
        run: infractl plan analyse plan.json --fail-on destructive
```

A plan that only creates and updates passes. One that deletes or replaces exits 3 and fails the job with the offending resources listed.

### Comment findings on the pull request

Distinguishing exit 3 from exit 1 is what makes this safe: a crashed scan does not post an empty "all clear" comment.

```yaml
      - name: Drift scan
        id: drift
        continue-on-error: true
        run: |
          infractl drift scan \
            --state terraform.tfstate \
            --live live.json \
            --fail-on medium \
            --quiet -o json > drift.json
          echo "code=$?" >> "$GITHUB_OUTPUT"

      - name: Report
        env:
          CODE: ${{ steps.drift.outputs.code }}
        run: |
          case "$CODE" in
            0) echo "No drift." ;;
            3)
              jq -r '"### Drift detected\n\n| Severity | Resource | Kind |\n|---|---|---|\n" +
                     (.findings | map("| \(.severity) | `\(.address)` | \(.kind) |") | join("\n"))' \
                 drift.json > comment.md
              gh pr comment "${{ github.event.number }}" --body-file comment.md
              exit 1
              ;;
            *) echo "Scan failed with $CODE" >&2; exit 1 ;;
          esac
```

### Scheduled drift check

```yaml
name: drift
on:
  schedule:
    - cron: '0 6 * * *'
  workflow_dispatch:

jobs:
  scan:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      issues: write
      id-token: write
    steps:
      - uses: actions/checkout@v4
      - uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_READONLY_ROLE }}
          aws-region: us-east-1
      - uses: actions/setup-go@v5
        with: { go-version: '1.25' }
      - run: go install github.com/ashutosh0x/infra-control/cmd/infractl@latest

      - run: ./scripts/snapshot.sh > live.json

      - name: Scan
        id: scan
        continue-on-error: true
        run: |
          infractl drift scan --state terraform.tfstate --live live.json \
            --fail-on high --quiet -o json > drift.json
          echo "code=$?" >> "$GITHUB_OUTPUT"

      - name: Open an issue for high-severity drift
        if: steps.scan.outputs.code == '3'
        run: |
          gh issue create \
            --title "Drift detected $(date -u +%Y-%m-%d)" \
            --label drift \
            --body "$(jq -r '.findings[] | "- **\(.severity)** `\(.address)` (\(.kind))"' drift.json)"
```

Note the snapshot job holds the read-only cloud role. The scan itself needs no credentials.

## GitLab CI

```yaml
stages: [validate]

.infractl:
  image: golang:1.25
  before_script:
    - go install github.com/ashutosh0x/infra-control/cmd/infractl@latest

plan-check:
  stage: validate
  extends: .infractl
  script:
    - terraform show -json tfplan > plan.json
    - infractl plan analyse plan.json --fail-on destructive
  allow_failure:
    exit_codes: 3   # a finding blocks without marking the pipeline broken

drift-check:
  stage: validate
  extends: .infractl
  script:
    - infractl drift scan --state terraform.tfstate --live live.json --fail-on high -o json
  artifacts:
    when: always
    paths: [drift.json]
```

`allow_failure.exit_codes: 3` is the GitLab equivalent of the split above: exit 3 surfaces as a warning rather than a broken pipeline, while exit 1 still fails hard.

## Pre-commit

```yaml
# .pre-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: infractl-plan
        name: reject destructive terraform plans
        entry: infractl plan analyse
        language: system
        files: '^plan\.json$'
        args: ['--fail-on', 'destructive']
```

## Jenkins

```groovy
pipeline {
  agent any
  stages {
    stage('Plan analysis') {
      steps {
        script {
          def code = sh(
            script: 'infractl plan analyse plan.json --fail-on destructive -o json > plan-report.json',
            returnStatus: true
          )
          if (code == 3) {
            unstable('Plan contains destructive changes')
          } else if (code != 0) {
            error("infractl failed with exit ${code}")
          }
        }
      }
    }
  }
  post {
    always { archiveArtifacts artifacts: 'plan-report.json', allowEmptyArchive: true }
  }
}
```

## Notes on machine output

`-o json` suppresses progress output automatically, so stdout carries only the payload. `--quiet` additionally silences warnings on stderr.

Output is sorted deterministically. Two runs over unchanged input produce byte-identical bytes, so a checksum is a valid way to detect that anything moved:

```bash
infractl drift scan --state terraform.tfstate --live live.json -o json \
  | jq -S . | sha256sum > drift.sha
```

Colour is stripped from every machine format regardless of the colour flags, so the payload is always parseable.
