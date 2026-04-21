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
	SeverityInfo     Severity = "INFO"
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
	case SeverityInfo, SeveritySafe:
		return 0
	default:
		return 0
	}
}

// QuantumThreat classifies a rule/finding by the threat that breaks the algorithm.
// This is the primary axis of Observer's taxonomy.
type QuantumThreat string

const (
	// ThreatClassicalBroken - algorithms already broken by classical attacks
	// (MD5, SHA-1, DES, 3DES, RC4, RSA<2048, DH<2048).
	ThreatClassicalBroken QuantumThreat = "classical-broken"

	// ThreatShorBroken - factoring/DLP-based asymmetric crypto fully broken by
	// a CRQC via Shor's algorithm (RSA, DH, ECDH, ECDSA, EdDSA, any size).
	// HNDL ("harvest now, decrypt later") applies to confidentiality uses.
	ThreatShorBroken QuantumThreat = "shor-broken"

	// ThreatGroverReduced - symmetric primitives whose effective security is
	// halved by Grover's algorithm (AES-128, SHA-256 in collision contexts).
	// Severity depends on data lifetime.
	ThreatGroverReduced QuantumThreat = "grover-reduced"

	// ThreatGroverSafe - symmetric primitives with no meaningful reduction
	// under Grover (AES-256, SHA-384/512, SHA-3).
	ThreatGroverSafe QuantumThreat = "grover-safe"

	// ThreatPQCStandardized - NIST FIPS 203/204/205 and SP 800-208 (ML-KEM,
	// ML-DSA, SLH-DSA, LMS/XMSS). Compliant.
	ThreatPQCStandardized QuantumThreat = "pqc-standardized"

	// ThreatPQCExperimental - PQC candidates in the NIST process but not yet
	// standardized (HQC, BIKE, Classic McEliece). Research / hybrid only.
	ThreatPQCExperimental QuantumThreat = "pqc-experimental"

	// ThreatPQCBroken - tried-and-broken PQC candidates (SIKE, Rainbow, GeMSS).
	// Critical regardless of context.
	ThreatPQCBroken QuantumThreat = "pqc-broken"

	// ThreatUnknown - classification cannot be determined statically (e.g. a
	// cipher name sourced from user input). Needs human review.
	ThreatUnknown QuantumThreat = "unknown"
)

// ValidQuantumThreats returns the set of valid QuantumThreat values.
// Used by rule validation.
func ValidQuantumThreats() []QuantumThreat {
	return []QuantumThreat{
		ThreatClassicalBroken, ThreatShorBroken,
		ThreatGroverReduced, ThreatGroverSafe,
		ThreatPQCStandardized, ThreatPQCExperimental, ThreatPQCBroken,
		ThreatUnknown,
	}
}

// Primitive classifies a rule/finding by the cryptographic primitive it targets.
// Orthogonal to QuantumThreat - used for reporting and filtering.
type Primitive string

const (
	PrimitiveAsymmetricEncryption Primitive = "asymmetric-encryption"
	PrimitiveKeyExchange          Primitive = "key-exchange"
	PrimitiveSignature            Primitive = "signature"
	PrimitiveSymmetricCipher      Primitive = "symmetric-cipher"
	PrimitiveHash                 Primitive = "hash"
	PrimitiveMAC                  Primitive = "mac"
	PrimitiveKDF                  Primitive = "kdf"
	PrimitiveRNG                  Primitive = "rng"
	PrimitiveAEAD                 Primitive = "aead"
	PrimitiveOther                Primitive = "other"
)

// ValidPrimitives returns the set of valid Primitive values.
func ValidPrimitives() []Primitive {
	return []Primitive{
		PrimitiveAsymmetricEncryption, PrimitiveKeyExchange, PrimitiveSignature,
		PrimitiveSymmetricCipher, PrimitiveHash, PrimitiveMAC, PrimitiveKDF,
		PrimitiveRNG, PrimitiveAEAD, PrimitiveOther,
	}
}

// Status describes the disposition of a finding after exemption processing.
type Status string

const (
	StatusActive           Status = "active"
	StatusExempted         Status = "exempted"
	StatusExpiredExemption Status = "expired-exemption"
)

// ExemptionSource identifies how a finding was exempted.
type ExemptionSource string

const (
	ExemptionSourceInline ExemptionSource = "inline"
	ExemptionSourceConfig ExemptionSource = "config"
)

// Exemption records an accepted suppression applied to a finding.
type Exemption struct {
	Reason string          `json:"reason"`
	Owner  string          `json:"owner,omitempty"`
	Until  string          `json:"until,omitempty"` // ISO 8601 date (YYYY-MM-DD), empty if no expiry
	Source ExemptionSource `json:"source"`
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

// Rule defines a single detection pattern for quantum-relevant cryptography.
type Rule struct {
	ID            string
	Language      string
	Pattern       *regexp.Regexp
	Algorithm     string
	Severity      Severity      // optional - derived from QuantumThreat if empty
	Message       string
	Migration     string
	QuantumThreat QuantumThreat // required
	Primitive     Primitive     // required
	Composition   bool          // true for hybrid constructions (e.g. X25519+ML-KEM)
}

// Finding represents a single detected crypto usage in source code.
type Finding struct {
	RuleID        string        `json:"rule_id"`
	File          string        `json:"file"`
	Line          int           `json:"line"`
	Column        int           `json:"column"`
	Algorithm     string        `json:"algorithm"`
	Severity      Severity      `json:"severity"`
	Confidence    int           `json:"confidence"`
	Snippet       string        `json:"snippet"`
	Message       string        `json:"message"`
	Migration     string        `json:"migration"`
	QuantumThreat QuantumThreat `json:"quantum_threat,omitempty"`
	Primitive     Primitive     `json:"primitive,omitempty"`
	Composition   bool          `json:"composition,omitempty"`
	Status        Status        `json:"status"`
	Exemption     *Exemption    `json:"exemption,omitempty"`
	AIRisk        *AIRisk       `json:"ai_risk,omitempty"`
}

// AIRisk contains the AI-generated risk assessment for a finding.
type AIRisk struct {
	Severity     string `json:"severity"`
	Reasoning    string `json:"reasoning"`
	DataLifetime string `json:"data_lifetime"`
}

// RiskSummary aggregates finding counts by severity and taxonomy axes.
type RiskSummary struct {
	Critical        int                   `json:"critical"`
	High            int                   `json:"high"`
	Medium          int                   `json:"medium"`
	Low             int                   `json:"low"`
	Info            int                   `json:"info"`
	Safe            int                   `json:"safe"`
	Total           int                   `json:"total"`
	Exempted        int                   `json:"exempted"`
	ByQuantumThreat map[QuantumThreat]int `json:"by_quantum_threat,omitempty"`
	ByPrimitive     map[Primitive]int     `json:"by_primitive,omitempty"`
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

// DeriveSeverity computes a default severity from a rule's quantum taxonomy.
// Used when a rule does not pin severity explicitly.
func DeriveSeverity(threat QuantumThreat, prim Primitive) Severity {
	switch threat {
	case ThreatPQCBroken:
		return SeverityCritical
	case ThreatShorBroken:
		// Confidentiality uses (HNDL applies) are worse than signatures,
		// but without per-finding context we default to HIGH and let
		// rules override for long-lived-data cases.
		if prim == PrimitiveAsymmetricEncryption || prim == PrimitiveKeyExchange {
			return SeverityCritical
		}
		return SeverityHigh
	case ThreatClassicalBroken:
		return SeverityHigh
	case ThreatGroverReduced:
		return SeverityMedium
	case ThreatGroverSafe, ThreatPQCStandardized:
		return SeverityInfo
	case ThreatPQCExperimental:
		return SeverityLow
	case ThreatUnknown:
		return SeverityMedium
	default:
		return SeverityMedium
	}
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
