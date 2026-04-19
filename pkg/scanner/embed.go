package scanner

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed rules/*.yaml
var bundledRulesFS embed.FS

// LoadBundledRules returns the core PQC detection rules embedded at compile time.
// These rules are sourced from GetQuantumDrive/Observer-rules and bundled into
// every released binary so the tool works without network access.
func LoadBundledRules() ([]Rule, error) {
	seen := map[string]int{}
	var rules []Rule

	entries, err := fs.ReadDir(bundledRulesFS, "rules")
	if err != nil {
		return nil, fmt.Errorf("read bundled rules: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		data, err := bundledRulesFS.ReadFile("rules/" + name)
		if err != nil {
			return nil, fmt.Errorf("read bundled rules/%s: %w", name, err)
		}
		loaded, err := parseRuleBytes(data)
		if err != nil {
			return nil, fmt.Errorf("parse bundled rules/%s: %w", name, err)
		}
		for _, r := range loaded {
			if idx, exists := seen[r.ID]; exists {
				rules[idx] = r
			} else {
				seen[r.ID] = len(rules)
				rules = append(rules, r)
			}
		}
	}
	return rules, nil
}

// MergeRules merges override rules on top of base rules, with override winning on duplicate IDs.
func MergeRules(base, override []Rule) []Rule {
	if len(override) == 0 {
		return base
	}
	seen := make(map[string]int, len(base))
	result := make([]Rule, len(base))
	copy(result, base)
	for i, r := range result {
		seen[r.ID] = i
	}
	for _, r := range override {
		if idx, exists := seen[r.ID]; exists {
			result[idx] = r
		} else {
			seen[r.ID] = len(result)
			result = append(result, r)
		}
	}
	return result
}
