# Live snapshots

`infractl drift scan` compares a Terraform state file against a **live snapshot**: a JSON file describing what the resources actually look like right now.

## Why a file

Every other drift tool calls cloud APIs itself. This one takes a file, for three reasons.

**Credentials stay where they already are.** Drift detection runs in a pipeline that needs no cloud access of its own. The snapshot is produced by whatever already holds a read-only role.

**The snapshot is auditable.** It is a file you can inspect, commit, diff, and re-run against. When a finding is disputed, the exact input that produced it still exists.

**Any source works.** A read-only role, an existing CMDB export, [Steampipe](https://steampipe.io), AWS Config, `gcloud asset`, or a script someone already wrote. The tool does not need to grow a provider integration for each one.

The trade is that you supply the snapshot. The rest of this page covers how.

## Format

```json
{
  "captured_at": "2026-08-29T09:00:00Z",
  "provider": "aws",
  "resources": {
    "<terraform address>": { "<attribute>": "<live value>" }
  }
}
```

| Field | Required | Meaning |
| --- | --- | --- |
| `captured_at` | No | RFC 3339 timestamp. A snapshot over 24 hours old triggers a staleness warning, because drift found against stale data may already be fixed |
| `provider` | No | Recorded for reporting |
| `resources` | Yes | Map of Terraform address to observed attributes |

### Addresses are the join key

The key must be the Terraform address exactly as Terraform prints it, because that is what the state side is matched on.

| Resource shape | Address |
| --- | --- |
| Singleton | `aws_s3_bucket.assets` |
| `count` | `aws_subnet.private[0]` |
| `for_each` | `aws_instance.web["primary"]` |
| In a module | `module.vpc.aws_route_table.rt` |

Get the exact list from the state file itself:

```bash
infractl state list terraform.tfstate -o name
```

An address in the snapshot that is not in state is reported as an **unmanaged** resource. An address in state that is not in the snapshot is reported as **missing in live**.

### Attributes

Include only the attributes you want compared. The comparison walks the state side, so:

- An attribute in state but absent from the snapshot is reported as removed.
- An attribute in the snapshot but not in state is ignored. Cloud APIs return many server-assigned fields Terraform never tracks, and reporting them would bury the real changes.
- Provider bookkeeping (`id`, `arn`, `tags_all`, `etag`, `self_link`, timestamps) is excluded from comparison regardless. These never round-trip from a live read and would otherwise report drift on every resource on every run.

Start narrow. A snapshot carrying only the security-relevant attributes produces a shorter, more actionable report than one carrying everything.

```json
{
  "resources": {
    "aws_s3_bucket.assets": {
      "acl": "public-read",
      "server_side_encryption_configuration": {}
    }
  }
}
```

## Producing a snapshot

### From AWS CLI

```bash
#!/usr/bin/env bash
set -euo pipefail

echo '{"captured_at":"'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'","provider":"aws","resources":{'

first=true
for bucket in $(aws s3api list-buckets --query 'Buckets[].Name' --output text); do
  $first || echo ','
  first=false

  acl=$(aws s3api get-bucket-acl --bucket "$bucket" \
    --query "Grants[?Grantee.URI=='http://acs.amazonaws.com/groups/global/AllUsers']" \
    --output json)
  visibility=$([ "$acl" = "[]" ] && echo private || echo public-read)

  printf '"aws_s3_bucket.%s":{"bucket":"%s","acl":"%s"}' "$bucket" "$bucket" "$visibility"
done

echo '}}'
```

The address must match the Terraform resource name, not the bucket name, so a real script maps between them. `infractl state list --type aws_s3_bucket -o json` gives you both sides.

### From Steampipe

Steampipe exposes cloud inventory as SQL, which makes the mapping straightforward.

```sql
select
  'aws_s3_bucket.' || replace(name, '-', '_') as address,
  jsonb_build_object(
    'bucket', name,
    'acl', case when bucket_policy_is_public then 'public-read' else 'private' end
  ) as attributes
from aws_s3_bucket;
```

```bash
steampipe query --output json snapshot.sql \
  | jq '{captured_at: now|todate, provider: "aws",
         resources: (map({(.address): .attributes}) | add)}' \
  > live.json
```

### From `terraform refresh`

The simplest source, if you can run Terraform with credentials: refresh a copy of state and treat the refreshed copy as the live side.

```bash
cp terraform.tfstate before.tfstate
terraform refresh -state=after.tfstate

infractl state list after.tfstate -o json \
  | jq '{captured_at: now|todate,
         resources: (map({(.Address): .Attributes}) | add)}' \
  > live.json

infractl drift scan --state before.tfstate --live live.json
```

This detects everything Terraform's own refresh detects, which is a lower bar than a full inventory read: it will not find unmanaged resources, because refresh only looks at what state already knows about. Pass `--include-unmanaged=false` to suppress the unmanaged section when using this method, since it cannot populate it.

## Severity

Each modified resource is scored by which attributes changed.

| Weight | Attribute contains | Rationale |
| --- | --- | --- |
| 40 | `public`, `acl`, `encryption`, `kms_key`, `policy`, `security_group`, `ingress`, `egress`, `password`, `secret`, `principal` | A security boundary moved |
| 25 | `instance_type`, `size`, `vpc`, `subnet`, `route`, `firewall`, `port`, `cidr`, `deletion_protection`, `multi_az` | Availability or capacity |
| 10 | `backup`, `retention`, `logging`, `monitoring`, `versioning`, `lifecycle` | Operational posture |
| 2 | anything else | Bookkeeping |

Scores sum and cap at 100:

| Score | Severity |
| --- | --- |
| 80 and above | critical |
| 50 to 79 | high |
| 25 to 49 | medium |
| 10 to 24 | low |
| below 10 | info |

A bucket that loses encryption and goes public scores 80 and lands critical. A tag edit scores 2 and lands info.

Filter with `--min-severity`, and gate CI with `--fail-on`, which exits 3.

## Secrets

Attributes whose names indicate a secret are dropped during comparison, before the output payload is built. The diff records that the path changed but carries neither value.

Dropping rather than masking at render time is deliberate: masking later would leave the real value in the structured payload, so `-o json` would leak what the table hid.

Detection is name-based. If your provider uses an unusual name for a secret, the value will not be recognised. Review a snapshot before committing it anywhere.
