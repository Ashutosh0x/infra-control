package types

import (
	"time"
)

// CloudProvider represents a supported cloud provider.
type CloudProvider string

const (
	CloudProviderAWS        CloudProvider = "aws"
	CloudProviderGCP        CloudProvider = "gcp"
	CloudProviderAzure      CloudProvider = "azure"
	CloudProviderKubernetes CloudProvider = "kubernetes"
)

// String implements the fmt.Stringer interface.
func (c CloudProvider) String() string {
	return string(c)
}

// ResourceState represents the current state of a resource.
type ResourceState string

const (
	ResourceStateActive  ResourceState = "active"
	ResourceStateDeleted ResourceState = "deleted"
	ResourceStatePending ResourceState = "pending"
	ResourceStateUnknown ResourceState = "unknown"
	ResourceStateDrifted ResourceState = "drifted"
)

// String implements the fmt.Stringer interface.
func (r ResourceState) String() string {
	return string(r)
}

// Resource represents a cloud or Kubernetes infrastructure resource.
type Resource struct {
	ID             string            `json:"id"`
	ExternalID     string            `json:"external_id"` // Cloud provider resource ID (ARN, self_link, etc.)
	Name           string            `json:"name"`
	Type           string            `json:"type"` // e.g., "aws_s3_bucket", "google_compute_instance"
	Provider       CloudProvider     `json:"provider"`
	Region         string            `json:"region"`
	Account        string            `json:"account"` // AWS Account ID, GCP Project, Azure Subscription
	State          ResourceState     `json:"state"`
	Tags           map[string]string `json:"tags,omitempty"`
	Configuration  map[string]any    `json:"configuration"` // Full resource configuration
	TerraformState *TerraformRef     `json:"terraform_state,omitempty"`
	Metadata       ResourceMetadata  `json:"metadata"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	DiscoveredAt   time.Time         `json:"discovered_at"`
}

// TerraformRef links a resource to its Terraform state.
type TerraformRef struct {
	WorkspaceID string `json:"workspace_id"`
	StatePath   string `json:"state_path"`
	Address     string `json:"address"` // e.g., "module.vpc.aws_subnet.public[0]"
	Module      string `json:"module"`
	Provider    string `json:"provider"` // Terraform provider name
}

// ResourceMetadata contains additional metadata about a resource.
type ResourceMetadata struct {
	ManagedBy      string    `json:"managed_by"`       // "terraform", "manual", "crossplane", etc.
	LastModifiedBy string    `json:"last_modified_by"` // IAM principal
	LastModifiedAt time.Time `json:"last_modified_at"`
	IsPublic       bool      `json:"is_public"`
	IsEncrypted    bool      `json:"is_encrypted"`
	ComplianceTags []string  `json:"compliance_tags,omitempty"`
}

// ResourceFilter defines criteria for filtering resources.
type ResourceFilter struct {
	Providers []CloudProvider   `json:"providers,omitempty"`
	Types     []string          `json:"types,omitempty"`
	Regions   []string          `json:"regions,omitempty"`
	Accounts  []string          `json:"accounts,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
	States    []ResourceState   `json:"states,omitempty"`
	ManagedBy string            `json:"managed_by,omitempty"`
	Limit     int               `json:"limit,omitempty"`
	Offset    int               `json:"offset,omitempty"`
}
