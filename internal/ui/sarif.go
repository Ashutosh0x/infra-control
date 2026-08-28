package ui

import (
	"encoding/json"
	"fmt"
)

// SARIF is the Static Analysis Results Interchange Format, the schema GitHub
// code scanning ingests.
//
// Emitting it means drift and risk findings land in a repository's Security tab
// alongside CodeQL and dependency alerts, annotated on the pull request that
// introduced them, with GitHub tracking which findings are new, fixed, or still
// open across runs. That history is something a scan printing a table cannot
// provide, and it is why this format is worth carrying rather than leaving
// users to convert JSON themselves.
//
// This implements SARIF 2.1.0, the version GitHub requires.
const (
	sarifVersion = "2.1.0"
	sarifSchema  = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json"
)

// SarifLog is the root of a SARIF document.
type SarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []SarifRun `json:"runs"`
}

// SarifRun is one execution of one tool.
type SarifRun struct {
	Tool    SarifTool     `json:"tool"`
	Results []SarifResult `json:"results"`
}

// SarifTool identifies the analysis tool and declares its rules.
type SarifTool struct {
	Driver SarifDriver `json:"driver"`
}

// SarifDriver describes the tool itself.
type SarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version,omitempty"`
	InformationURI string      `json:"informationUri,omitempty"`
	Rules          []SarifRule `json:"rules"`
}

// SarifRule declares one kind of finding the tool can report.
//
// Declaring rules up front is what lets GitHub group findings, show a
// description on each alert, and keep a stable identity for a finding across
// runs so it can be marked fixed rather than merely absent.
type SarifRule struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	ShortDescription SarifText             `json:"shortDescription"`
	FullDescription  SarifText             `json:"fullDescription,omitempty"`
	Help             SarifText             `json:"help,omitempty"`
	Properties       map[string]any        `json:"properties,omitempty"`
	DefaultConfig    *SarifReportingConfig `json:"defaultConfiguration,omitempty"`
}

// SarifReportingConfig carries a rule's default severity.
type SarifReportingConfig struct {
	Level string `json:"level"`
}

// SarifText is SARIF's string wrapper.
type SarifText struct {
	Text string `json:"text"`
}

// SarifResult is a single finding.
type SarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   SarifText       `json:"message"`
	Locations []SarifLocation `json:"locations"`
	// PartialFingerprints let GitHub match a finding to the same finding in a
	// later run even if line numbers move, which is what makes "fixed" and
	// "still open" meaningful rather than guesses.
	PartialFingerprints map[string]string `json:"partialFingerprints,omitempty"`
	Properties          map[string]any    `json:"properties,omitempty"`
}

// SarifLocation points at where a finding lives.
type SarifLocation struct {
	PhysicalLocation SarifPhysicalLocation `json:"physicalLocation"`
}

// SarifPhysicalLocation is a file and optional region.
type SarifPhysicalLocation struct {
	ArtifactLocation SarifArtifactLocation `json:"artifactLocation"`
	Region           *SarifRegion          `json:"region,omitempty"`
}

// SarifArtifactLocation names the file.
type SarifArtifactLocation struct {
	URI string `json:"uri"`
}

// SarifRegion names a position within the file.
type SarifRegion struct {
	StartLine int `json:"startLine"`
}

// SarifFinding is the tool-neutral input this package converts into SARIF, so
// that drift, risk, and plan findings can all reach the same encoder.
type SarifFinding struct {
	// RuleID groups like findings, for example "drift-modified".
	RuleID string
	// RuleName is the human name of that group.
	RuleName string
	// RuleDescription explains what the rule detects.
	RuleDescription string
	// Level is the SARIF severity: error, warning, note, or none.
	Level string
	// Message is the finding, phrased so it reads on its own in an alert.
	Message string
	// File is the artifact the finding is attributed to, usually the state file.
	File string
	// Fingerprint identifies this finding stably across runs.
	Fingerprint string
	// Properties carries the structured detail for anyone reading the raw SARIF.
	Properties map[string]any
}

// SeverityToSarifLevel maps a severity or risk level onto a SARIF level.
//
// SARIF has four levels where this tool has five severities, so critical and
// high both map to error. Collapsing them here rather than inventing a level
// keeps the document valid; the original severity survives in properties and in
// the rule ID, so nothing is lost for a reader who wants it.
func SeverityToSarifLevel(severity string) string {
	switch severity {
	case "critical", "high":
		return "error"
	case "medium":
		return "warning"
	case "low", "info", "negligible":
		return "note"
	default:
		return "none"
	}
}

// BuildSarif converts findings into a SARIF document.
func BuildSarif(toolName, version, infoURI string, findings []SarifFinding) SarifLog {
	// Collect the distinct rules, preserving first-seen order so the output is
	// deterministic for a deterministic input.
	var rules []SarifRule
	seen := map[string]bool{}

	for _, f := range findings {
		if seen[f.RuleID] {
			continue
		}
		seen[f.RuleID] = true

		rules = append(rules, SarifRule{
			ID:               f.RuleID,
			Name:             f.RuleName,
			ShortDescription: SarifText{Text: f.RuleName},
			FullDescription:  SarifText{Text: f.RuleDescription},
			Help:             SarifText{Text: f.RuleDescription},
			DefaultConfig:    &SarifReportingConfig{Level: f.Level},
		})
	}
	// GitHub rejects a run with a null rules array, so an empty scan still
	// needs an empty slice rather than nil.
	if rules == nil {
		rules = []SarifRule{}
	}

	results := make([]SarifResult, 0, len(findings))
	for _, f := range findings {
		result := SarifResult{
			RuleID:  f.RuleID,
			Level:   f.Level,
			Message: SarifText{Text: f.Message},
			Locations: []SarifLocation{{
				PhysicalLocation: SarifPhysicalLocation{
					ArtifactLocation: SarifArtifactLocation{URI: f.File},
					// SARIF requires a region for GitHub to anchor an alert.
					// A state file has no meaningful line for a resource, so
					// line 1 stands for the file as a whole.
					Region: &SarifRegion{StartLine: 1},
				},
			}},
			Properties: f.Properties,
		}
		if f.Fingerprint != "" {
			result.PartialFingerprints = map[string]string{"infractlFingerprint/v1": f.Fingerprint}
		}
		results = append(results, result)
	}

	return SarifLog{
		Schema:  sarifSchema,
		Version: sarifVersion,
		Runs: []SarifRun{{
			Tool:    SarifTool{Driver: SarifDriver{Name: toolName, Version: version, InformationURI: infoURI, Rules: rules}},
			Results: results,
		}},
	}
}

// WriteSarif encodes a SARIF document to stdout.
func (r *Renderer) WriteSarif(log SarifLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	enc := json.NewEncoder(r.out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(log); err != nil {
		return fmt.Errorf("encode sarif output: %w", err)
	}
	return nil
}
