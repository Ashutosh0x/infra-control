package types

import ()

// NodeType represents the kind of graph node.
type NodeType string

const (
	NodeTypeResource  NodeType = "resource"
	NodeTypeModule    NodeType = "module"
	NodeTypeStack     NodeType = "stack"
	NodeTypeNamespace NodeType = "namespace"
	NodeTypeAccount   NodeType = "account"
)

// String implements fmt.Stringer
func (n NodeType) String() string { return string(n) }

// EdgeType represents the relationship between nodes.
type EdgeType string

const (
	EdgeTypeDependsOn  EdgeType = "depends_on"
	EdgeTypeReferences EdgeType = "references"
	EdgeTypeContains   EdgeType = "contains"
	EdgeTypeNetworkTo  EdgeType = "network_to"
	EdgeTypeIAMTo      EdgeType = "iam_to"
)

// String implements fmt.Stringer
func (e EdgeType) String() string { return string(e) }

// Node represents an entity in the infrastructure graph.
type Node struct {
	ID         string         `json:"id"`
	Type       NodeType       `json:"type"`
	ResourceID string         `json:"resource_id"`
	Label      string         `json:"label"`
	Provider   string         `json:"provider"`
	Properties map[string]any `json:"properties"`
}

// Edge represents a directed relationship between two nodes.
type Edge struct {
	ID         string         `json:"id"`
	Source     string         `json:"source"`
	Target     string         `json:"target"`
	Type       EdgeType       `json:"type"`
	Weight     float64        `json:"weight"`
	Properties map[string]any `json:"properties"`
}

// AffectedNode describes a node impacted by a change or drift.
type AffectedNode struct {
	NodeID      string  `json:"node_id"`
	Label       string  `json:"label"`
	Distance    int     `json:"distance"`
	ImpactType  string  `json:"impact_type"` // e.g., "direct", "transitive"
	Probability float64 `json:"probability"`
}

// BlastRadius estimates the impact of a change to a specific origin node.
type BlastRadius struct {
	OriginNodeID  string         `json:"origin_node_id"`
	AffectedNodes []AffectedNode `json:"affected_nodes"`
	TotalAffected int            `json:"total_affected"`
	MaxDepth      int            `json:"max_depth"`
	RiskScore     float64        `json:"risk_score"`
}

// GraphQuery specifies parameters for traversing the graph.
type GraphQuery struct {
	StartNodeID string     `json:"start_node_id"`
	Direction   string     `json:"direction"` // "upstream", "downstream", "both"
	MaxDepth    int        `json:"max_depth"`
	EdgeTypes   []EdgeType `json:"edge_types,omitempty"`
	NodeTypes   []NodeType `json:"node_types,omitempty"`
}

// GraphStats holds aggregate metrics about the graph.
type GraphStats struct {
	TotalNodes  int              `json:"total_nodes"`
	TotalEdges  int              `json:"total_edges"`
	NodesByType map[NodeType]int `json:"nodes_by_type"`
	EdgesByType map[EdgeType]int `json:"edges_by_type"`
}
