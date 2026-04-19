package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigExemption is a suppression declared in `.observer.yml` at the repo root.
type ConfigExemption struct {
	Rule   string `yaml:"rule"`             // optional; empty = match any rule ID
	Path   string `yaml:"path"`             // doublestar glob against finding's file path (repo-relative)
	Reason string `yaml:"reason"`           // required
	Owner  string `yaml:"owner,omitempty"`  // optional
	Until  string `yaml:"until,omitempty"`  // optional ISO 8601 date (YYYY-MM-DD)
}

type configFile struct {
	Exemptions []ConfigExemption `yaml:"exemptions"`
}

// LoadConfigExemptions reads `.observer.yml` from the scan root.
// Returns (nil, nil) if no config file is present.
// Returns error only for malformed YAML or invalid exemptions (e.g. missing reason).
func LoadConfigExemptions(root string) ([]ConfigExemption, error) {
	path := filepath.Join(root, ".observer.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cf configFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf(".observer.yml: %w", err)
	}
	for i, ex := range cf.Exemptions {
		if strings.TrimSpace(ex.Reason) == "" {
			return nil, fmt.Errorf(".observer.yml: exemption #%d missing required 'reason'", i+1)
		}
	}
	return cf.Exemptions, nil
}

// annotationRE matches `observer:ignore` anywhere in a line. The remaining kv-pairs
// are parsed independently by keyRE/reasonRE/untilRE below so order doesn't matter.
var (
	annotationRE = regexp.MustCompile(`observer:ignore\b`)
	ruleRE       = regexp.MustCompile(`\brule=([A-Za-z0-9._:/-]+)`)
	reasonRE     = regexp.MustCompile(`\breason=(?:"([^"]*)"|'([^']*)'|(\S+))`)
	untilRE      = regexp.MustCompile(`\buntil=(\d{4}-\d{2}-\d{2})`)
)

// inlineAnnotation is a parsed observer:ignore comment.
type inlineAnnotation struct {
	RuleID string // empty = apply to any rule on the target line
	Reason string
	Until  string // ISO date
	// Target line number (1-based) the annotation applies to — the next non-blank line.
	TargetLine int
}

// scanInlineAnnotations walks file lines and returns a map from target-line
// number (1-based) to the parsed annotation that should suppress findings there.
// If multiple annotations target the same line, the last one wins.
func scanInlineAnnotations(lines []string) map[int]inlineAnnotation {
	out := map[int]inlineAnnotation{}
	for i, line := range lines {
		if !annotationRE.MatchString(line) {
			continue
		}
		ann, ok := parseInlineAnnotation(line)
		if !ok {
			// observer:ignore present but reason missing — emit a warning but do not suppress.
			fmt.Fprintf(os.Stderr, "warning: observer:ignore on line %d is missing reason=\"...\" (suppression skipped)\n", i+1)
			continue
		}
		// Target = next non-blank line after the annotation.
		target := -1
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) != "" {
				target = j + 1
				break
			}
		}
		if target > 0 {
			ann.TargetLine = target
			out[target] = ann
		}
	}
	return out
}

func parseInlineAnnotation(line string) (inlineAnnotation, bool) {
	ann := inlineAnnotation{}
	if m := ruleRE.FindStringSubmatch(line); m != nil {
		ann.RuleID = m[1]
	}
	if m := reasonRE.FindStringSubmatch(line); m != nil {
		// Capture group order: double-quoted | single-quoted | bareword
		for _, g := range m[1:] {
			if g != "" {
				ann.Reason = g
				break
			}
		}
	}
	if m := untilRE.FindStringSubmatch(line); m != nil {
		ann.Until = m[1]
	}
	if strings.TrimSpace(ann.Reason) == "" {
		return ann, false
	}
	return ann, true
}

// applyInline marks each finding as exempted if an inline annotation on the
// finding's line applies to it. Returns the (possibly updated) finding slice.
// Expired exemptions result in StatusExpiredExemption and do NOT suppress.
func applyInline(findings []Finding, annotations map[int]inlineAnnotation, today time.Time) []Finding {
	for i := range findings {
		ann, ok := annotations[findings[i].Line]
		if !ok {
			continue
		}
		if ann.RuleID != "" && ann.RuleID != findings[i].RuleID {
			continue
		}
		setStatus(&findings[i], Exemption{
			Reason: ann.Reason,
			Until:  ann.Until,
			Source: ExemptionSourceInline,
		}, today)
	}
	return findings
}

// applyConfig matches each finding against the config exemption list.
// First-match wins (order in `.observer.yml` determines precedence).
func applyConfig(findings []Finding, exemptions []ConfigExemption, today time.Time) []Finding {
	for i := range findings {
		if findings[i].Status == StatusExempted || findings[i].Status == StatusExpiredExemption {
			continue // already handled by inline
		}
		for _, ex := range exemptions {
			if ex.Rule != "" && ex.Rule != findings[i].RuleID {
				continue
			}
			if !matchGlob(ex.Path, findings[i].File) {
				continue
			}
			setStatus(&findings[i], Exemption{
				Reason: ex.Reason,
				Owner:  ex.Owner,
				Until:  ex.Until,
				Source: ExemptionSourceConfig,
			}, today)
			break
		}
	}
	return findings
}

// setStatus applies an exemption to a finding, marking it expired if until < today.
func setStatus(f *Finding, ex Exemption, today time.Time) {
	if ex.Until != "" {
		if u, err := time.Parse("2006-01-02", ex.Until); err == nil {
			if today.After(u) {
				f.Status = StatusExpiredExemption
				f.Exemption = &ex
				return
			}
		}
	}
	f.Status = StatusExempted
	f.Exemption = &ex
}

// matchGlob reports whether path matches pattern using doublestar-style globs
// (** matches zero or more path segments). An empty pattern matches everything —
// useful for repo-wide rule-only exemptions.
func matchGlob(pattern, path string) bool {
	if pattern == "" {
		return true
	}
	// Normalize to forward slashes so Windows-style paths match POSIX globs.
	path = filepath.ToSlash(path)
	return globMatch(pattern, path)
}

// globMatch is a minimal doublestar matcher supporting **, *, ?.
// Avoids external dep for a hot path with small pattern count.
func globMatch(pattern, path string) bool {
	pSegs := strings.Split(pattern, "/")
	tSegs := strings.Split(path, "/")
	return matchSegs(pSegs, tSegs)
}

func matchSegs(p, t []string) bool {
	for len(p) > 0 {
		if p[0] == "**" {
			// ** matches zero or more path segments.
			// Try consuming 0..len(t) segments.
			for i := 0; i <= len(t); i++ {
				if matchSegs(p[1:], t[i:]) {
					return true
				}
			}
			return false
		}
		if len(t) == 0 {
			return false
		}
		if !matchSeg(p[0], t[0]) {
			return false
		}
		p = p[1:]
		t = t[1:]
	}
	return len(t) == 0
}

// matchSeg matches a single path segment against a pattern supporting * and ?.
func matchSeg(pattern, s string) bool {
	// Fast path: no wildcards.
	if !strings.ContainsAny(pattern, "*?") {
		return pattern == s
	}
	// Dynamic match.
	pi, si := 0, 0
	starPi, starSi := -1, 0
	for si < len(s) {
		if pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == s[si]) {
			pi++
			si++
		} else if pi < len(pattern) && pattern[pi] == '*' {
			starPi = pi
			starSi = si
			pi++
		} else if starPi != -1 {
			pi = starPi + 1
			starSi++
			si = starSi
		} else {
			return false
		}
	}
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}
