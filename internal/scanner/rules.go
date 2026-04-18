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
	ID        string `yaml:"id"`
	Language  string `yaml:"language"`
	Pattern   string `yaml:"pattern"`
	Algorithm string `yaml:"algorithm"`
	Severity  string `yaml:"severity"`
	Message   string `yaml:"message"`
	Migration string `yaml:"migration"`
}

// LoadCustomRules loads all *.yaml rule files from dir.
// Returns nil, nil if dir does not exist.
func LoadCustomRules(dir string) ([]Rule, error) {
	var rules []Rule

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
		rules = append(rules, loaded...)
		return nil
	})

	if os.IsNotExist(err) {
		return nil, nil
	}
	return rules, err
}

func loadRuleFile(path string) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

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
		rules = append(rules, Rule{
			ID:        yr.ID,
			Language:  yr.Language,
			Pattern:   pat,
			Algorithm: yr.Algorithm,
			Severity:  Severity(yr.Severity),
			Message:   yr.Message,
			Migration: yr.Migration,
		})
	}
	return rules, nil
}
