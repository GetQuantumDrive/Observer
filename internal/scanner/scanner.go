package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	".gradle": true, "target": true, "build": true,
	"dist": true, ".next": true, "__pycache__": true,
	".cache": true, "out": true,
}

// Scan walks root, applies all rules, and returns a ScanReport.
func Scan(root string, rules []Rule) (ScanReport, error) {
	start := time.Now()
	var findings []Finding
	filesScanned := 0

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		lang := DetectLanguage(path)
		if lang == LanguageUnknown {
			return nil
		}

		filesScanned++
		relPath, _ := filepath.Rel(root, path)
		ff, readErr := scanFile(relPath, path, lang, rules)
		if readErr != nil {
			return nil
		}
		findings = append(findings, ff...)
		return nil
	})
	if err != nil {
		return ScanReport{}, err
	}

	risk := computeRiskSummary(findings)
	return ScanReport{
		ID:           uuid.New().String(),
		ScannedAt:    time.Now().UTC(),
		DurationMs:   time.Since(start).Milliseconds(),
		FilesScanned: filesScanned,
		RulesApplied: len(rules),
		Findings:     findings,
		RiskSummary:  risk,
		Compliance:   computeCompliance(risk),
	}, nil
}

func scanFile(relPath, absPath string, lang Language, rules []Rule) ([]Finding, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	var findings []Finding
	for _, rule := range rules {
		if rule.Language != "" && rule.Language != string(lang) {
			continue
		}
		for i, line := range lines {
			if rule.Pattern.MatchString(line) {
				findings = append(findings, Finding{
					RuleID:     rule.ID,
					File:       relPath,
					Line:       i + 1,
					Algorithm:  rule.Algorithm,
					Severity:   rule.Severity,
					Confidence: 8,
					Snippet:    extractSnippet(lines, i),
					Message:    rule.Message,
					Migration:  rule.Migration,
				})
			}
		}
	}
	return findings, nil
}

func extractSnippet(lines []string, center int) string {
	start := center - 2
	if start < 0 {
		start = 0
	}
	end := start + 5
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}

func computeRiskSummary(findings []Finding) RiskSummary {
	r := RiskSummary{Total: len(findings)}
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			r.Critical++
		case SeverityHigh:
			r.High++
		case SeverityMedium:
			r.Medium++
		case SeverityLow:
			r.Low++
		case SeveritySafe:
			r.Safe++
		}
	}
	return r
}

func computeCompliance(r RiskSummary) ComplianceStatus {
	verdict := func() string {
		if r.Critical > 0 {
			return "NON-COMPLIANT"
		}
		if r.High > 0 {
			return "AT RISK"
		}
		return "COMPLIANT"
	}()
	return ComplianceStatus{
		NISTFIPS203: verdict,
		NISTFIPS204: verdict,
		NIS2:        verdict,
		DORA:        verdict,
	}
}
