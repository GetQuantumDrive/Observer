package scanner

import (
	"testing"
	"time"
)

func TestParseInlineAnnotation(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		wantRule string
		wantRea  string
		wantUnt  string
		wantOK   bool
	}{
		{
			name:     "full form",
			line:     `// observer:ignore rule=shor-broken.rsa reason="SWIFT partner" until=2027-03-31`,
			wantRule: "shor-broken.rsa",
			wantRea:  "SWIFT partner",
			wantUnt:  "2027-03-31",
			wantOK:   true,
		},
		{
			name:    "reason only",
			line:    `# observer:ignore reason='legacy interop'`,
			wantRea: "legacy interop",
			wantOK:  true,
		},
		{
			name:   "missing reason is rejected",
			line:   `// observer:ignore rule=foo`,
			wantOK: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ann, ok := parseInlineAnnotation(c.line)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if !ok {
				return // when rejected, other field values are don't-cares
			}
			if ann.RuleID != c.wantRule {
				t.Errorf("rule = %q, want %q", ann.RuleID, c.wantRule)
			}
			if ann.Reason != c.wantRea {
				t.Errorf("reason = %q, want %q", ann.Reason, c.wantRea)
			}
			if ann.Until != c.wantUnt {
				t.Errorf("until = %q, want %q", ann.Until, c.wantUnt)
			}
		})
	}
}

func TestScanInlineAnnotations(t *testing.T) {
	lines := []string{
		`package x`,
		``,
		`// observer:ignore reason="legacy"`,
		``, // blank skipped
		`rsa.New()`, // target = line 5 (1-based)
	}
	got := scanInlineAnnotations(lines)
	ann, ok := got[5]
	if !ok {
		t.Fatalf("expected annotation targeting line 5, got %v", got)
	}
	if ann.Reason != "legacy" {
		t.Errorf("reason = %q", ann.Reason)
	}
}

func TestApplyInlineExempts(t *testing.T) {
	findings := []Finding{
		{RuleID: "r", File: "a.go", Line: 5, Status: StatusActive},
	}
	ann := map[int]inlineAnnotation{
		5: {Reason: "x"},
	}
	out := applyInline(findings, ann, time.Now())
	if out[0].Status != StatusExempted {
		t.Errorf("status = %v, want exempted", out[0].Status)
	}
	if out[0].Exemption == nil || out[0].Exemption.Source != ExemptionSourceInline {
		t.Errorf("exemption source = %+v", out[0].Exemption)
	}
}

func TestExpiredExemption(t *testing.T) {
	findings := []Finding{
		{RuleID: "r", File: "a.go", Line: 5, Status: StatusActive},
	}
	ann := map[int]inlineAnnotation{
		5: {Reason: "x", Until: "2020-01-01"},
	}
	today, _ := time.Parse("2006-01-02", "2026-04-19")
	out := applyInline(findings, ann, today)
	if out[0].Status != StatusExpiredExemption {
		t.Errorf("status = %v, want expired-exemption", out[0].Status)
	}
}

func TestRuleScopedAnnotation(t *testing.T) {
	findings := []Finding{
		{RuleID: "matches", File: "a.go", Line: 5, Status: StatusActive},
		{RuleID: "other", File: "a.go", Line: 5, Status: StatusActive},
	}
	ann := map[int]inlineAnnotation{
		5: {RuleID: "matches", Reason: "x"},
	}
	out := applyInline(findings, ann, time.Now())
	if out[0].Status != StatusExempted {
		t.Errorf("matching rule: status = %v", out[0].Status)
	}
	if out[1].Status != StatusActive {
		t.Errorf("non-matching rule: status = %v, must remain active", out[1].Status)
	}
}

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"src/**/partners/**", "src/main/java/partners/swift/Foo.java", true},
		{"src/**/partners/**", "src/main/java/other/Foo.java", false},
		{"**/*.go", "cmd/observer/main.go", true},
		{"**/*.go", "cmd/observer/main.js", false},
		{"", "anything", true},
		{"exact.txt", "exact.txt", true},
		{"exact.txt", "other.txt", false},
	}
	for _, c := range cases {
		got := matchGlob(c.pattern, c.path)
		if got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}
