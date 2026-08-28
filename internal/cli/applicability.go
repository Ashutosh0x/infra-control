package cli

import "strings"

// Risk checks only mean something for resource types where the underlying
// concept exists. A VPC has no encryption setting and a subnet is single-AZ by
// definition, so scoring either against those checks produces a finding that
// cannot be acted on. A tool that reports unfixable findings gets ignored, and
// then the real findings get ignored with them.
//
// The lists below are matched as substrings against the Terraform resource type,
// which keeps them provider-agnostic: "bucket" covers aws_s3_bucket,
// google_storage_bucket, and azurerm_storage_container alike.

// encryptableTypes are resource types that store data and expose an
// encryption-at-rest setting.
var encryptableTypes = []string{
	"bucket", "s3", "blob", "storage_account", "disk", "volume",
	"db_instance", "rds", "database", "sql", "dynamodb", "table",
	"elasticache", "redis", "kafka", "msk", "efs", "filesystem",
	"secret", "parameter", "queue", "sqs", "sns", "topic",
	"snapshot", "image", "backup", "repository", "registry",
	"log_group", "stream", "kinesis", "firehose", "cluster",
}

// multiAZTypes are resource types that can span availability zones, and so can
// meaningfully be single-AZ.
var multiAZTypes = []string{
	"db_instance", "rds", "elasticache", "cluster", "instance",
	"autoscaling", "node_pool", "replication_group", "filesystem", "efs",
}

// backupableTypes are resource types that hold state worth backing up.
var backupableTypes = []string{
	"db_instance", "rds", "database", "sql", "dynamodb",
	"elasticache", "disk", "volume", "filesystem", "efs", "bucket",
}

// supportsEncryption reports whether encryption at rest is a meaningful
// property of the given resource type.
func supportsEncryption(resourceType string) bool {
	return matchesTypeList(resourceType, encryptableTypes)
}

// supportsMultiAZ reports whether the resource type can span availability zones.
func supportsMultiAZ(resourceType string) bool {
	return matchesTypeList(resourceType, multiAZTypes)
}

// supportsBackup reports whether the resource type holds state worth backing up.
func supportsBackup(resourceType string) bool {
	return matchesTypeList(resourceType, backupableTypes)
}

// matchesTypeList reports whether the resource type contains any listed token.
func matchesTypeList(resourceType string, tokens []string) bool {
	lower := strings.ToLower(resourceType)
	for _, token := range tokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}
