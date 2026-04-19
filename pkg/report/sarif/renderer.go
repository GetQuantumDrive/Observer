// Package sarif renders Observer scan reports into SARIF 2.1.0 for consumption
// by GitHub Code Scanning, SonarQube External Issues, and other SARIF-aware tools.
//
// Observer's bespoke JSON remains the canonical format; SARIF is a projection.
// Domain-specific fields (quantum_threat, primitive, composition, exemption)
// land under result.properties.observer.* with a schemaVersion for
// forward compatibility.
package sarif

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/getquantumdrive/observer/pkg/scanner"
)

const (
	// ToolName is the SARIF toolComponent name; Sonar keys suppressions against this.
	ToolName = "Observer"
	// ToolURI is the canonical tool information URI.
	ToolURI = "https://github.com/GetQuantumDrive/Observer"
	// ObserverSchemaVersion is bumped whenever the layout of
	// result.properties.observer.* changes. Consumers should tolerate unknown fields.
	ObserverSchemaVersion = "1.0"
)

// Render converts an Observer ScanReport to SARIF 2.1.0 JSON bytes.
// Output is indented for readability and diffability in review.
func Render(report scanner.ScanReport, toolVersion string) ([]byte, error) {
	rules := collectRules(report.Findings)
	ruleIndex := make(map[string]int, len(rules))
	for i, r := range rules {
		ruleIndex[r["id"].(string)] = i
	}

	results := make([]map[string]any, 0, len(report.Findings))
	for _, f := range report.Findings {
		results = append(results, renderResult(f, ruleIndex))
	}

	out := map[string]any{
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
		"version": "2.1.0",
		"runs": []map[string]any{
			{
				"tool": map[string]any{
					"driver": map[string]any{
						"name":           ToolName,
						"informationUri": ToolURI,
						"version":        toolVersion,
						"rules":          rules,
					},
				},
				"results": results,
				"properties": map[string]any{
					"observer": map[string]any{
						"schemaVersion": ObserverSchemaVersion,
						"compliance":    report.Compliance,
						"riskSummary":   report.RiskSummary,
						"source":        report.Source,
						"ref":           report.Ref,
						"sha":           report.SHA,
					},
				},
			},
		},
	}
	return json.MarshalIndent(out, "", "  ")
}

// collectRules deduplicates the set of rule IDs referenced by findings and
// emits SARIF reportingDescriptor entries in stable (sorted-insertion) order.
func collectRules(findings []scanner.Finding) []map[string]any {
	seen := map[string]bool{}
	var ids []string
	byID := map[string]scanner.Finding{}
	for _, f := range findings {
		if !seen[f.RuleID] {
			seen[f.RuleID] = true
			ids = append(ids, f.RuleID)
			byID[f.RuleID] = f
		}
	}
	rules := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		f := byID[id]
		rules = append(rules, map[string]any{
			"id":               id,
			"name":             id,
			"shortDescription": map[string]any{"text": f.Algorithm},
			"fullDescription":  map[string]any{"text": f.Message},
			"help": map[string]any{
				"text":     f.Migration,
				"markdown": "**Migration:** " + f.Migration,
			},
			"defaultConfiguration": map[string]any{
				"level": severityToLevel(f.Severity),
			},
			"properties": map[string]any{
				"observer": map[string]any{
					"quantumThreat": f.QuantumThreat,
					"primitive":     f.Primitive,
					"composition":   f.Composition,
				},
				"tags": ruleTags(f),
			},
		})
	}
	return rules
}

func renderResult(f scanner.Finding, ruleIndex map[string]int) map[string]any {
	result := map[string]any{
		"ruleId":    f.RuleID,
		"ruleIndex": ruleIndex[f.RuleID],
		"level":     severityToLevel(f.Severity),
		"message":   map[string]any{"text": f.Message},
		"locations": []map[string]any{
			{
				"physicalLocation": map[string]any{
					"artifactLocation": map[string]any{
						"uri": f.File,
					},
					"region": map[string]any{
						"startLine":   max1(f.Line),
						"startColumn": max1(f.Column),
						"snippet":     map[string]any{"text": f.Snippet},
					},
				},
			},
		},
		"partialFingerprints": map[string]any{
			"observer/v1": fingerprint(f),
		},
		"properties": map[string]any{
			"observer": map[string]any{
				"quantumThreat": f.QuantumThreat,
				"primitive":     f.Primitive,
				"composition":   f.Composition,
				"algorithm":     f.Algorithm,
				"confidence":    f.Confidence,
				"migration":     f.Migration,
				"status":        f.Status,
			},
		},
	}
	if f.Exemption != nil {
		result["suppressions"] = []map[string]any{
			{
				"kind":          exemptionKind(f.Exemption.Source),
				"justification": f.Exemption.Reason,
				"status":        suppressionStatus(f.Status),
			},
		}
	}
	return result
}

// severityToLevel maps Observer severity → SARIF level.
// SARIF has only error/warning/note/none, so we collapse:
//
//	CRITICAL, HIGH → error
//	MEDIUM          → warning
//	LOW, INFO, SAFE → note
func severityToLevel(s scanner.Severity) string {
	switch s {
	case scanner.SeverityCritical, scanner.SeverityHigh:
		return "error"
	case scanner.SeverityMedium:
		return "warning"
	default:
		return "note"
	}
}

// fingerprint produces a stable hash that identifies this finding across
// reformats and line-number changes. Components: rule ID + file path +
// normalized code snippet (whitespace-collapsed). Deliberately omits line number.
func fingerprint(f scanner.Finding) string {
	snippet := strings.Join(strings.Fields(f.Snippet), " ")
	h := sha256.Sum256([]byte(f.RuleID + "|" + f.File + "|" + snippet))
	return hex.EncodeToString(h[:])
}

func ruleTags(f scanner.Finding) []string {
	tags := []string{"security", "cryptography", "post-quantum"}
	if f.QuantumThreat != "" {
		tags = append(tags, string(f.QuantumThreat))
	}
	if f.Primitive != "" {
		tags = append(tags, string(f.Primitive))
	}
	return tags
}

func exemptionKind(src scanner.ExemptionSource) string {
	if src == scanner.ExemptionSourceConfig {
		return "external"
	}
	return "inSource"
}

func suppressionStatus(s scanner.Status) string {
	if s == scanner.StatusExpiredExemption {
		// SARIF "rejected" = suppression was evaluated and declined.
		return "rejected"
	}
	return "accepted"
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
