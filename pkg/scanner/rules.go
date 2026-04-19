package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

type yamlRule struct {
	ID            string `yaml:"id"`
	Language      string `yaml:"language"`
	Pattern       string `yaml:"pattern"`
	Algorithm     string `yaml:"algorithm"`
	Severity      string `yaml:"severity"`
	Message       string `yaml:"message"`
	Migration     string `yaml:"migration"`
	QuantumThreat string `yaml:"quantum_threat"`
	Primitive     string `yaml:"primitive"`
	Composition   bool   `yaml:"composition"`
}

// LoadCustomRules loads all *.yaml rule files from one or more directories.
// Directories that do not exist are silently skipped.
// Rules from later directories override rules with the same ID from earlier ones.
func LoadCustomRules(dirs ...string) ([]Rule, error) {
	seen := map[string]int{} // rule ID → index in rules slice
	var rules []Rule

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml" {
				return nil
			}
			loaded, loadErr := loadRuleFile(path)
			if loadErr != nil {
				return fmt.Errorf("load %s: %w", path, loadErr)
			}
			for _, r := range loaded {
				if idx, exists := seen[r.ID]; exists {
					rules[idx] = r // override
				} else {
					seen[r.ID] = len(rules)
					rules = append(rules, r)
				}
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	return rules, nil
}

func loadRuleFile(path string) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseRuleBytes(data)
}

func parseRuleBytes(data []byte) ([]Rule, error) {
	var yrules []yamlRule
	if err := yaml.Unmarshal(data, &yrules); err != nil {
		return nil, err
	}

	var rules []Rule
	for _, yr := range yrules {
		pat, err := regexp.Compile(yr.Pattern)
		if err != nil {
			return nil, fmt.Errorf("rule %s: invalid pattern: %w", yr.ID, err)
		}

		threat, err := parseQuantumThreat(yr.QuantumThreat)
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", yr.ID, err)
		}
		prim, err := parsePrimitive(yr.Primitive)
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", yr.ID, err)
		}

		sev := Severity(yr.Severity)
		if sev == "" {
			sev = DeriveSeverity(threat, prim)
		}

		rules = append(rules, Rule{
			ID:            yr.ID,
			Language:      yr.Language,
			Pattern:       pat,
			Algorithm:     yr.Algorithm,
			Severity:      sev,
			Message:       yr.Message,
			Migration:     yr.Migration,
			QuantumThreat: threat,
			Primitive:     prim,
			Composition:   yr.Composition,
		})
	}
	return rules, nil
}

func parseQuantumThreat(s string) (QuantumThreat, error) {
	if s == "" {
		return "", fmt.Errorf("quantum_threat is required")
	}
	t := QuantumThreat(s)
	for _, v := range ValidQuantumThreats() {
		if v == t {
			return t, nil
		}
	}
	return "", fmt.Errorf("invalid quantum_threat %q (valid: %v)", s, ValidQuantumThreats())
}

func parsePrimitive(s string) (Primitive, error) {
	if s == "" {
		return "", fmt.Errorf("primitive is required")
	}
	p := Primitive(s)
	for _, v := range ValidPrimitives() {
		if v == p {
			return p, nil
		}
	}
	return "", fmt.Errorf("invalid primitive %q (valid: %v)", s, ValidPrimitives())
}
