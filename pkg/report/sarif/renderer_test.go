package sarif

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/getquantumdrive/observer/pkg/scanner"
)

func TestRenderShape(t *testing.T) {
	report := scanner.ScanReport{
		ID:        "test-id",
		Source:    "getquantumdrive/observer",
		Ref:       "main",
		SHA:       "abc123",
		ScannedAt: time.Unix(0, 0).UTC(),
		Findings: []scanner.Finding{
			{
				RuleID:        "shor-broken.rsa-usage",
				File:          "src/Main.java",
				Line:          42,
				Algorithm:     "RSA",
				Severity:      scanner.SeverityHigh,
				Snippet:       "KeyPairGenerator.getInstance(\"RSA\")",
				Message:       "RSA usage detected.",
				Migration:     "Switch to ML-KEM.",
				QuantumThreat: scanner.ThreatShorBroken,
				Primitive:     scanner.PrimitiveKeyExchange,
				Status:        scanner.StatusActive,
			},
			{
				RuleID:        "classical-broken.md5",
				File:          "src/Hash.java",
				Line:          10,
				Algorithm:     "MD5",
				Severity:      scanner.SeverityHigh,
				Snippet:       "MessageDigest.getInstance(\"MD5\")",
				Message:       "MD5 is broken.",
				QuantumThreat: scanner.ThreatClassicalBroken,
				Primitive:     scanner.PrimitiveHash,
				Status:        scanner.StatusExempted,
				Exemption: &scanner.Exemption{
					Reason: "legacy interop",
					Source: scanner.ExemptionSourceInline,
				},
			},
		},
		Compliance: scanner.ComplianceStatus{NIS2: "AT RISK"},
	}

	out, err := Render(report, "0.1.0")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}

	if parsed["version"] != "2.1.0" {
		t.Errorf("version = %v, want 2.1.0", parsed["version"])
	}
	runs, ok := parsed["runs"].([]any)
	if !ok || len(runs) != 1 {
		t.Fatalf("runs: want 1, got %v", parsed["runs"])
	}
	run := runs[0].(map[string]any)

	// driver.rules should contain exactly two deduplicated rules.
	tool := run["tool"].(map[string]any)
	driver := tool["driver"].(map[string]any)
	if driver["name"] != ToolName {
		t.Errorf("driver.name = %v, want %s", driver["name"], ToolName)
	}
	rules := driver["rules"].([]any)
	if len(rules) != 2 {
		t.Errorf("rules count = %d, want 2", len(rules))
	}

	// results should contain 2 entries; the second has suppressions[].
	results := run["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("results count = %d, want 2", len(results))
	}
	second := results[1].(map[string]any)
	supp, ok := second["suppressions"].([]any)
	if !ok || len(supp) != 1 {
		t.Fatalf("second result must have suppressions[]; got %v", second["suppressions"])
	}
	s := supp[0].(map[string]any)
	if s["kind"] != "inSource" {
		t.Errorf("suppression kind = %v, want inSource", s["kind"])
	}
	if s["justification"] != "legacy interop" {
		t.Errorf("suppression justification = %v", s["justification"])
	}

	// Fingerprints should be present and stable.
	first := results[0].(map[string]any)
	fps := first["partialFingerprints"].(map[string]any)
	fp := fps["observer/v1"].(string)
	if len(fp) != 64 {
		t.Errorf("fingerprint length = %d, want 64 (sha256 hex)", len(fp))
	}

	// Domain properties are namespaced under properties.observer.
	props := first["properties"].(map[string]any)
	obs := props["observer"].(map[string]any)
	if obs["quantumThreat"] != "shor-broken" {
		t.Errorf("quantumThreat = %v, want shor-broken", obs["quantumThreat"])
	}
}

func TestFingerprintStableAcrossLineShifts(t *testing.T) {
	a := scanner.Finding{
		RuleID: "r", File: "a.go", Line: 5,
		Snippet: "x := \"RSA\"",
	}
	b := a
	b.Line = 99
	if fingerprint(a) != fingerprint(b) {
		t.Errorf("fingerprint must not depend on line number")
	}
}

func TestSeverityMapping(t *testing.T) {
	cases := map[scanner.Severity]string{
		scanner.SeverityCritical: "error",
		scanner.SeverityHigh:     "error",
		scanner.SeverityMedium:   "warning",
		scanner.SeverityLow:      "note",
		scanner.SeverityInfo:     "note",
		scanner.SeveritySafe:     "note",
	}
	for in, want := range cases {
		got := severityToLevel(in)
		if got != want {
			t.Errorf("severityToLevel(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestRenderEmpty(t *testing.T) {
	report := scanner.ScanReport{Findings: nil}
	out, err := Render(report, "0.1.0")
	if err != nil {
		t.Fatalf("Render empty: %v", err)
	}
	if !strings.Contains(string(out), `"version": "2.1.0"`) {
		t.Errorf("empty report must still produce valid SARIF")
	}
}
