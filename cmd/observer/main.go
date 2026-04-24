package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/getquantumdrive/observer/pkg/groundstate"
	htmlreport "github.com/getquantumdrive/observer/pkg/report/html"
	"github.com/getquantumdrive/observer/pkg/report/sarif"
	"github.com/getquantumdrive/observer/pkg/scanner"
)

// cliVersion is the Observer CLI version; overridden at release time via -ldflags.
var cliVersion = "dev"

// multiFlag allows a flag to be specified multiple times.
type multiFlag []string

func (m *multiFlag) String() string        { return fmt.Sprint([]string(*m)) }
func (m *multiFlag) Set(v string) error    { *m = append(*m, v); return nil }

func main() {
	var rulesDirs multiFlag
	var rulesRepos multiFlag

	version         := flag.Bool("version", false, "Print version and exit")
	dir             := flag.String("dir", ".", "Root directory to scan")
	failOn          := flag.String("fail-on", "critical", "Fail with exit code 1 when findings at this level or worse are found (critical|high|any|never)")
	output          := flag.String("output", "", "Write JSON report to this file path (default: stdout)")
	source          := flag.String("source", "", "Source identifier written into the report (e.g. org/repo)")
	ref             := flag.String("ref", "", "Git ref written into the report (e.g. main)")
	sha             := flag.String("sha", "", "Git commit SHA written into the report")
	gsURL           := flag.String("groundstate-url", "", "Groundstate server base URL to POST the report to")
	gsToken         := flag.String("groundstate-token", "", "Bearer token for the Groundstate server")
	rulesReposToken := flag.String("rules-repos-token", "", "Default bearer token applied to --rules-repo entries without an inline token")
	useBundled      := flag.Bool("use-bundled-rules", true, "If OBSERVER_BUNDLED_RULES_DIR is set, load rules from it before any --rules-repo entries")
	format          := flag.String("format", "json", "Report output format: json (Observer canonical) | sarif (SARIF 2.1.0) | html (self-contained HTML report)")
	flag.Var(&rulesDirs, "rules-dir", "Local directory containing custom YAML rules (repeatable)")
	flag.Var(&rulesRepos, "rules-repo", "GitHub rules repo (owner/repo[@ref][:path][|token]); repeatable, later entries override earlier ones")

	flag.Parse()

	if *version {
		fmt.Println(cliVersion)
		os.Exit(0)
	}

	// Load order (last wins on duplicate rule IDs):
	//   1. rules embedded in the binary at compile time
	//   2. OBSERVER_BUNDLED_RULES_DIR env var (Docker hot-patch override)
	//   3. each --rules-repo in order
	//   4. local --rules-dir entries
	var baseRules []scanner.Rule
	if *useBundled {
		embedded, err := scanner.LoadBundledRules()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load bundled rules: %v\n", err)
		} else {
			baseRules = embedded
		}
	}

	var allRulesDirs []string
	if *useBundled {
		if bundled := os.Getenv("OBSERVER_BUNDLED_RULES_DIR"); bundled != "" {
			allRulesDirs = append(allRulesDirs, bundled)
		}
	}

	for _, repo := range rulesRepos {
		if d, err := scanner.FetchRulesFromGitHub(repo, *rulesReposToken); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not fetch rules from %s: %v\n", repo, err)
		} else {
			allRulesDirs = append(allRulesDirs, d)
		}
	}

	allRulesDirs = append(allRulesDirs, rulesDirs...)

	extraRules, err := scanner.LoadCustomRules(allRulesDirs...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load custom rules: %v\n", err)
	}
	rules := scanner.MergeRules(baseRules, extraRules)

	report, err := scanner.Scan(*dir, rules)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
		os.Exit(2)
	}

	report.Source = *source
	report.Ref    = *ref
	report.SHA    = *sha

	// Canonical JSON (always produced; used for Groundstate + fail-on logic).
	reportJSON, _ := json.MarshalIndent(report, "", "  ")

	// Output bytes: either canonical JSON or SARIF, based on --format.
	var outputBytes []byte
	switch *format {
	case "json":
		outputBytes = reportJSON
	case "sarif":
		sb, err := sarif.Render(report, cliVersion)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not render SARIF: %v\n", err)
			os.Exit(2)
		}
		outputBytes = sb
	case "html":
		var buf bytes.Buffer
		if err := htmlreport.Render(report, cliVersion, &buf); err != nil {
			fmt.Fprintf(os.Stderr, "could not render HTML: %v\n", err)
			os.Exit(2)
		}
		outputBytes = buf.Bytes()
	default:
		fmt.Fprintf(os.Stderr, "unknown --format %q (want: json|sarif|html)\n", *format)
		os.Exit(2)
	}

	if *output != "" {
		if err := os.WriteFile(*output, outputBytes, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "could not write report: %v\n", err)
			os.Exit(2)
		}
	} else {
		fmt.Println(string(outputBytes))
	}

	// Groundstate only accepts Observer canonical JSON; the SARIF output is for
	// third-party tooling, not a replacement for the domain wire format.
	if *gsURL != "" {
		if err := groundstate.PostReport(*gsURL, *gsToken, reportJSON); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not post to groundstate: %v\n", err)
		}
	}

	critHigh := report.RiskSummary.Critical + report.RiskSummary.High
	switch *failOn {
	case "critical":
		if report.RiskSummary.Critical > 0 {
			fmt.Fprintf(os.Stderr, "failed: %d critical finding(s)\n", report.RiskSummary.Critical)
			os.Exit(1)
		}
	case "high":
		if critHigh > 0 {
			fmt.Fprintf(os.Stderr, "failed: %d critical/high finding(s)\n", critHigh)
			os.Exit(1)
		}
	case "any":
		if report.RiskSummary.Total > 0 {
			fmt.Fprintf(os.Stderr, "failed: %d finding(s)\n", report.RiskSummary.Total)
			os.Exit(1)
		}
	}
}
