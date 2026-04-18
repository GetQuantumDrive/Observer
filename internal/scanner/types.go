package scanner

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Severity represents the risk level of a finding.
type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
	SeveritySafe     Severity = "SAFE"
)

// SeverityRank returns a numeric rank for comparison (higher = worse).
func SeverityRank(s Severity) int {
	switch s {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	case SeveritySafe:
		return 0
	default:
		return 0
	}
}

// Language represents the programming language of a source file.
type Language string

const (
	LanguageJava       Language = "java"
	LanguagePython     Language = "python"
	LanguageJavaScript Language = "javascript"
	LanguageTypeScript Language = "typescript"
	LanguageGo         Language = "go"
	LanguageUnknown    Language = "unknown"
)

// Rule defines a single detection pattern for quantum-vulnerable cryptography.
type Rule struct {
	ID        string
	Language  string
	Pattern   *regexp.Regexp
	Algorithm string
	Severity  Severity
	Message   string
	Migration string
}

// Finding represents a single detected vulnerability in source code.
type Finding struct {
	RuleID     string   `json:"rule_id"`
	File       string   `json:"file"`
	Line       int      `json:"line"`
	Column     int      `json:"column"`
	Algorithm  string   `json:"algorithm"`
	Severity   Severity `json:"severity"`
	Confidence int      `json:"confidence"`
	Snippet    string   `json:"snippet"`
	Message    string   `json:"message"`
	Migration  string   `json:"migration"`
	AIRisk     *AIRisk  `json:"ai_risk,omitempty"`
}

// AIRisk contains the AI-generated risk assessment for a finding.
type AIRisk struct {
	Severity     string `json:"severity"`
	Reasoning    string `json:"reasoning"`
	DataLifetime string `json:"data_lifetime"`
}

// RiskSummary aggregates finding counts by severity.
type RiskSummary struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Safe     int `json:"safe"`
	Total    int `json:"total"`
}

// ComplianceStatus describes the compliance posture across frameworks.
type ComplianceStatus struct {
	NISTFIPS203 string `json:"nist_fips_203"`
	NISTFIPS204 string `json:"nist_fips_204"`
	NIS2        string `json:"nis2"`
	DORA        string `json:"dora"`
}

// ScanReport is the complete output of a scan run.
type ScanReport struct {
	ID           string           `json:"id"`
	Source       string           `json:"source"`
	Ref          string           `json:"ref"`
	SHA          string           `json:"sha"`
	ScannedAt    time.Time        `json:"scanned_at"`
	DurationMs   int64            `json:"duration_ms"`
	FilesScanned int              `json:"files_scanned"`
	RulesApplied int              `json:"rules_applied"`
	Findings     []Finding        `json:"findings"`
	RiskSummary  RiskSummary      `json:"risk_summary"`
	Compliance   ComplianceStatus `json:"compliance"`
	AIScoring    bool             `json:"ai_scoring"`
}

// extToLang maps file extensions to Language constants.
var extToLang = map[string]Language{
	".java": LanguageJava,
	".py":   LanguagePython,
	".js":   LanguageJavaScript,
	".mjs":  LanguageJavaScript,
	".cjs":  LanguageJavaScript,
	".ts":   LanguageTypeScript,
	".tsx":  LanguageTypeScript,
	".go":   LanguageGo,
}

// DetectLanguage returns the Language for a given filename based on its extension.
func DetectLanguage(filename string) Language {
	ext := strings.ToLower(filepath.Ext(filename))
	if lang, ok := extToLang[ext]; ok {
		return lang
	}
	return LanguageUnknown
}
