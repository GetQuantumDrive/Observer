package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/getquantumdrive/observer/internal/scanner"
	gha "github.com/sethvargo/go-githubactions"
)

func main() {
	a := gha.New()

	workspace := os.Getenv("GITHUB_WORKSPACE")
	if workspace == "" {
		workspace = "."
	}

	rulesDir := input(a, "rules-dir", ".pqc/rules")
	failOn := input(a, "fail-on", "critical")
	reportURL := a.GetInput("report-url")
	reportToken := a.GetInput("report-token")
	outputFile := a.GetInput("output")

	rules := scanner.BuiltInRules()
	custom, err := scanner.LoadCustomRules(rulesDir)
	if err != nil {
		a.Warningf("Could not load custom rules from %s: %v", rulesDir, err)
	}
	rules = append(rules, custom...)

	a.Infof("Scanning %s with %d rules...", workspace, len(rules))

	report, err := scanner.Scan(workspace, rules)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
		os.Exit(1)
	}

	// Enrich with GitHub context.
	if repo := os.Getenv("GITHUB_REPOSITORY"); repo != "" {
		report.Source = repo
	}
	report.Ref = os.Getenv("GITHUB_REF_NAME")
	report.SHA = os.Getenv("GITHUB_SHA")

	// Emit GitHub annotations.
	for _, f := range report.Findings {
		loc := fmt.Sprintf("file=%s,line=%d", f.File, f.Line)
		msg := fmt.Sprintf("[%s] %s", f.Algorithm, f.Message)
		switch f.Severity {
		case scanner.SeverityCritical, scanner.SeverityHigh:
			a.Errorf("::error %s::%s", loc, msg)
		case scanner.SeverityMedium:
			a.Warningf("::warning %s::%s", loc, msg)
		default:
			a.Noticef("::notice %s::%s", loc, msg)
		}
	}

	// Write JSON report file.
	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	if outputFile != "" {
		if err := os.WriteFile(outputFile, reportJSON, 0o644); err != nil {
			a.Warningf("Could not write report to %s: %v", outputFile, err)
		} else {
			a.Infof("Report written to %s", outputFile)
		}
	}

	// Set action outputs.
	a.SetOutput("findings", strconv.Itoa(report.RiskSummary.Total))
	a.SetOutput("critical", strconv.Itoa(report.RiskSummary.Critical))
	a.SetOutput("high", strconv.Itoa(report.RiskSummary.High))
	a.SetOutput("compliance", report.Compliance.NIS2)
	a.SetOutput("report-json", string(reportJSON))

	// POST to Groundstate if configured.
	if reportURL != "" {
		if err := postReport(reportURL, reportToken, reportJSON); err != nil {
			a.Warningf("Could not post report to %s: %v", reportURL, err)
		} else {
			a.Infof("Report posted to %s", reportURL)
		}
	}

	// Step summary.
	a.AddStepSummary(fmt.Sprintf(
		"## Observer — PQC Scan Results\n\n"+
			"| | |\n|---|---|\n"+
			"| Files scanned | %d |\n"+
			"| Total findings | %d |\n"+
			"| Critical | %d |\n"+
			"| High | %d |\n"+
			"| NIS2 | %s |\n"+
			"| DORA | %s |\n"+
			"| NIST FIPS 203 | %s |\n"+
			"| NIST FIPS 204 | %s |\n",
		report.FilesScanned,
		report.RiskSummary.Total,
		report.RiskSummary.Critical,
		report.RiskSummary.High,
		report.Compliance.NIS2,
		report.Compliance.DORA,
		report.Compliance.NISTFIPS203,
		report.Compliance.NISTFIPS204,
	))

	// Fail build.
	critHigh := report.RiskSummary.Critical + report.RiskSummary.High
	switch failOn {
	case "critical":
		if report.RiskSummary.Critical > 0 {
			fmt.Fprintf(os.Stderr, "Build failed: %d critical finding(s)\n", report.RiskSummary.Critical)
			os.Exit(1)
		}
	case "high":
		if critHigh > 0 {
			fmt.Fprintf(os.Stderr, "Build failed: %d critical/high finding(s)\n", critHigh)
			os.Exit(1)
		}
	case "any":
		if report.RiskSummary.Total > 0 {
			fmt.Fprintf(os.Stderr, "Build failed: %d finding(s)\n", report.RiskSummary.Total)
			os.Exit(1)
		}
	}
}

func input(a *gha.Action, name, def string) string {
	if v := a.GetInput(name); v != "" {
		return v
	}
	return def
}

func postReport(url, token string, body []byte) error {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodPost, url+"/api/reports", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}
