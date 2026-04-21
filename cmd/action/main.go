package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/getquantumdrive/observer/pkg/groundstate"
	"github.com/getquantumdrive/observer/pkg/report/sarif"
	"github.com/getquantumdrive/observer/pkg/scanner"
	gha "github.com/sethvargo/go-githubactions"
)

// cliVersion is the Observer Action version; overridden at release time via -ldflags.
var cliVersion = "dev"

func main() {
	a := gha.New()

	workspace := os.Getenv("GITHUB_WORKSPACE")
	if workspace == "" {
		workspace = "."
	}

	rulesReposRaw   := a.GetInput("rules-repos")
	rulesReposToken := a.GetInput("rules-repos-token")
	useBundled      := input(a, "use-bundled-rules", "true") != "false"
	rulesDir        := a.GetInput("rules-dir")
	extraRulesDir   := a.GetInput("extra-rules-dir")
	failOn          := input(a, "fail-on", "critical")
	reportURL       := a.GetInput("report-url")
	reportToken     := a.GetInput("report-token")
	outputFile      := a.GetInput("output")
	outputFormat    := input(a, "output-format", "json")

	// Load order (last wins on duplicate rule IDs):
	//   1. rules embedded in the binary at compile time
	//   2. OBSERVER_BUNDLED_RULES_DIR env var (Docker hot-patch override)
	//   3. each entry in rules-repos, in order
	//   4. local rules-dir / extra-rules-dir
	var baseRules []scanner.Rule
	if useBundled {
		embedded, err := scanner.LoadBundledRules()
		if err != nil {
			a.Warningf("Could not load bundled rules: %v", err)
		} else {
			baseRules = embedded
		}
	}

	var allRulesDirs []string
	if useBundled {
		if bundled := os.Getenv("OBSERVER_BUNDLED_RULES_DIR"); bundled != "" {
			allRulesDirs = append(allRulesDirs, bundled)
		}
	}

	for _, line := range strings.Split(rulesReposRaw, "\n") {
		repo := strings.TrimSpace(line)
		if repo == "" {
			continue
		}
		if d, err := scanner.FetchRulesFromGitHub(repo, rulesReposToken); err != nil {
			a.Warningf("Could not fetch rules from %s: %v", repo, err)
		} else {
			allRulesDirs = append(allRulesDirs, d)
		}
	}

	allRulesDirs = append(allRulesDirs, rulesDir, extraRulesDir)

	extraRules, err := scanner.LoadCustomRules(allRulesDirs...)
	if err != nil {
		a.Warningf("Could not load custom rules: %v", err)
	}
	rules := scanner.MergeRules(baseRules, extraRules)

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

	// Canonical JSON is always produced; Groundstate and action outputs consume it.
	reportJSON, _ := json.MarshalIndent(report, "", "  ")

	// File output honors output-format. SARIF is the right choice for users who
	// want to upload via github/codeql-action/upload-sarif to Code Scanning.
	if outputFile != "" {
		var outputBytes []byte
		switch outputFormat {
		case "json":
			outputBytes = reportJSON
		case "sarif":
			sb, err := sarif.Render(report, cliVersion)
			if err != nil {
				a.Warningf("Could not render SARIF: %v", err)
				outputBytes = reportJSON
			} else {
				outputBytes = sb
			}
		default:
			a.Warningf("Unknown output-format %q (want: json|sarif); writing json", outputFormat)
			outputBytes = reportJSON
		}
		if err := os.WriteFile(outputFile, outputBytes, 0o644); err != nil {
			a.Warningf("Could not write report to %s: %v", outputFile, err)
		} else {
			a.Infof("Report written to %s (%s)", outputFile, outputFormat)
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
		if err := groundstate.PostReport(reportURL, reportToken, reportJSON); err != nil {
			a.Warningf("Could not post report to %s: %v", reportURL, err)
		} else {
			a.Infof("Report posted to %s", reportURL)
		}
	}

	// Step summary.
	a.AddStepSummary(fmt.Sprintf(
		"## Observer - PQC Scan Results\n\n"+
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
