// Package html renders Observer scan reports as self-contained HTML pages,
// similar in spirit to JaCoCo coverage reports: one file, no external deps.
package html

import (
	"html/template"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/getquantumdrive/observer/pkg/scanner"
)

// Render writes a self-contained HTML report for report to w.
// toolVersion is embedded in the page footer.
func Render(report scanner.ScanReport, toolVersion string, w io.Writer) error {
	return reportTmpl.Execute(w, buildViewModel(report, toolVersion))
}

// ── view model ────────────────────────────────────────────────────────────────

type viewModel struct {
	ToolVersion string
	Report      scanner.ScanReport
	ActiveCount int
	AlgRows     []algRow
	FileRows    []fileRow
	// max count for bar chart scaling
	MaxAlgCount int
}

type algRow struct {
	Algorithm string
	Count     int
	BarPct    int // 0–100
}

type fileRow struct {
	Path     string
	Short    string // last 3 path segments
	Findings []scanner.Finding
	Critical int
	High     int
	Medium   int
	Low      int
}

func buildViewModel(report scanner.ScanReport, ver string) viewModel {
	active := make([]scanner.Finding, 0, len(report.Findings))
	for _, f := range report.Findings {
		if f.Status == scanner.StatusActive {
			active = append(active, f)
		}
	}

	// algorithm frequency
	algCount := map[string]int{}
	for _, f := range active {
		if f.Algorithm != "" {
			algCount[f.Algorithm]++
		}
	}
	type kv struct {
		k string
		v int
	}
	var pairs []kv
	for k, v := range algCount {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].v > pairs[j].v })
	maxAlg := 1
	if len(pairs) > 0 && pairs[0].v > maxAlg {
		maxAlg = pairs[0].v
	}
	var algs []algRow
	for i, p := range pairs {
		if i == 10 {
			break
		}
		algs = append(algs, algRow{
			Algorithm: p.k,
			Count:     p.v,
			BarPct:    p.v * 100 / maxAlg,
		})
	}

	// per-file grouping
	fileMap := map[string]*fileRow{}
	fileOrder := []string{}
	for _, f := range active {
		if _, ok := fileMap[f.File]; !ok {
			segs := strings.Split(f.File, "/")
			var short string
			if len(segs) > 3 {
				short = strings.Join(segs[len(segs)-3:], "/")
			} else {
				short = f.File
			}
			fileMap[f.File] = &fileRow{Path: f.File, Short: short}
			fileOrder = append(fileOrder, f.File)
		}
		fr := fileMap[f.File]
		fr.Findings = append(fr.Findings, f)
		switch f.Severity {
		case scanner.SeverityCritical:
			fr.Critical++
		case scanner.SeverityHigh:
			fr.High++
		case scanner.SeverityMedium:
			fr.Medium++
		case scanner.SeverityLow:
			fr.Low++
		}
	}
	// sort files: most critical first
	sort.Slice(fileOrder, func(i, j int) bool {
		a, b := fileMap[fileOrder[i]], fileMap[fileOrder[j]]
		if a.Critical != b.Critical {
			return a.Critical > b.Critical
		}
		if a.High != b.High {
			return a.High > b.High
		}
		return a.Path < b.Path
	})
	files := make([]fileRow, 0, len(fileOrder))
	for _, p := range fileOrder {
		files = append(files, *fileMap[p])
	}

	return viewModel{
		ToolVersion: ver,
		Report:      report,
		ActiveCount: len(active),
		AlgRows:     algs,
		FileRows:    files,
		MaxAlgCount: maxAlg,
	}
}

// ── template functions ────────────────────────────────────────────────────────

var tmplFuncs = template.FuncMap{
	"sevClass": func(s scanner.Severity) string {
		switch s {
		case scanner.SeverityCritical:
			return "sev-critical"
		case scanner.SeverityHigh:
			return "sev-high"
		case scanner.SeverityMedium:
			return "sev-medium"
		case scanner.SeverityLow:
			return "sev-low"
		default:
			return "sev-info"
		}
	},
	"complianceClass": func(s string) string {
		switch s {
		case "NON-COMPLIANT":
			return "badge-fail"
		case "AT RISK":
			return "badge-warn"
		case "COMPLIANT":
			return "badge-pass"
		default:
			return "badge-unknown"
		}
	},
	"shortPath": func(p string) string {
		segs := strings.Split(filepath.ToSlash(p), "/")
		if len(segs) > 3 {
			return strings.Join(segs[len(segs)-3:], "/")
		}
		return p
	},
	"threatLabel": func(t scanner.QuantumThreat) string {
		switch t {
		case scanner.ThreatShorBroken:
			return "Shor-broken (CRQC)"
		case scanner.ThreatClassicalBroken:
			return "Classically broken"
		case scanner.ThreatGroverReduced:
			return "Grover-reduced"
		case scanner.ThreatGroverSafe:
			return "Grover-safe"
		case scanner.ThreatPQCStandardized:
			return "PQC standardized"
		case scanner.ThreatPQCExperimental:
			return "PQC experimental"
		case scanner.ThreatPQCBroken:
			return "PQC broken"
		default:
			return string(t)
		}
	},
	"sourceLabel": func(s string) string {
		if s == "" {
			return "local scan"
		}
		return s
	},
}

// ── HTML template ─────────────────────────────────────────────────────────────

var reportTmpl = template.Must(template.New("report").Funcs(tmplFuncs).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Observer PQC — {{.Report.Source | sourceLabel}}</title>
<style>
:root {
  --bg:       #0f1117;
  --surface:  #1a1d27;
  --surface2: #242736;
  --border:   #2e3146;
  --text:     #e2e4f0;
  --muted:    #8890b0;
  --accent:   #5b6ef5;
  --accent2:  #7c8cff;
  --critical: #f4455a;
  --high:     #f97316;
  --medium:   #f59e0b;
  --low:      #3b82f6;
  --pass:     #10b981;
  --warn:     #f59e0b;
  --fail:     #f4455a;
}
*{box-sizing:border-box;margin:0;padding:0}
body{background:var(--bg);color:var(--text);font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;font-size:14px;line-height:1.6;min-height:100vh}
a{color:var(--accent2);text-decoration:none}
a:hover{text-decoration:underline}
code{font-family:"SFMono-Regular",Consolas,monospace;font-size:12px}

/* topbar */
.topbar{background:var(--surface);border-bottom:1px solid var(--border);padding:0 32px;display:flex;align-items:center;gap:16px;height:56px}
.topbar-logo{font-size:18px;font-weight:700;letter-spacing:-.5px}
.topbar-logo span{color:var(--accent2)}
.topbar-source{font-size:13px;color:var(--muted)}
.topbar-meta{font-size:12px;color:var(--muted);margin-left:auto}

/* layout */
.container{max-width:1200px;margin:0 auto;padding:32px 24px}

/* stat cards */
.stat-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(150px,1fr));gap:16px;margin-bottom:32px}
.stat-card{background:var(--surface);border:1px solid var(--border);border-radius:10px;padding:20px 18px}
.stat-card .label{font-size:11px;text-transform:uppercase;letter-spacing:.08em;color:var(--muted);margin-bottom:8px}
.stat-card .value{font-size:28px;font-weight:700}
.stat-card.critical .value{color:var(--critical)}
.stat-card.high     .value{color:var(--high)}
.stat-card.medium   .value{color:var(--medium)}
.stat-card.low      .value{color:var(--low)}

/* section */
.section{margin-bottom:40px}
.section-title{font-size:13px;font-weight:600;text-transform:uppercase;letter-spacing:.1em;color:var(--muted);margin-bottom:16px;padding-bottom:8px;border-bottom:1px solid var(--border)}

/* table */
.table-wrap{overflow-x:auto}
.table-card{background:var(--surface);border:1px solid var(--border);border-radius:10px;overflow:hidden}
table{width:100%;border-collapse:collapse;font-size:13px}
th{text-align:left;padding:10px 14px;font-size:11px;text-transform:uppercase;letter-spacing:.07em;color:var(--muted);background:var(--surface);border-bottom:1px solid var(--border);white-space:nowrap}
td{padding:11px 14px;border-bottom:1px solid var(--border);vertical-align:top}
tr:last-child td{border-bottom:none}
tbody tr:hover{background:var(--surface2)}
.num{text-align:right;font-variant-numeric:tabular-nums}

/* badges */
.badge{display:inline-block;padding:2px 9px;border-radius:4px;font-size:11px;font-weight:600;letter-spacing:.04em}
.badge-pass   {background:rgba(16,185,129,.15);color:var(--pass)}
.badge-warn   {background:rgba(245,158,11,.15);color:var(--warn)}
.badge-fail   {background:rgba(244,69,90,.15);color:var(--fail)}
.badge-unknown{background:var(--surface2);color:var(--muted)}

.sev-critical{color:var(--critical);font-weight:700}
.sev-high    {color:var(--high);font-weight:600}
.sev-medium  {color:var(--medium);font-weight:600}
.sev-low     {color:var(--low)}
.sev-info    {color:var(--muted)}

/* compliance grid */
.compliance-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:16px}
.compliance-card{background:var(--surface);border:1px solid var(--border);border-radius:10px;padding:20px}
.compliance-card .fw-name{font-weight:600;margin-bottom:10px;font-size:13px}

/* bar chart */
.bar-list{display:flex;flex-direction:column;gap:10px}
.bar-row{display:flex;align-items:center;gap:12px}
.bar-label{min-width:160px;font-size:13px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.bar-track{flex:1;background:var(--surface2);border-radius:4px;height:8px;overflow:hidden}
.bar-fill{height:100%;background:var(--accent);border-radius:4px}
.bar-count{min-width:44px;text-align:right;font-size:13px;color:var(--muted);font-variant-numeric:tabular-nums}

/* snippet */
.snippet{background:var(--surface2);border:1px solid var(--border);border-radius:6px;padding:10px 14px;font-size:12px;overflow-x:auto;white-space:pre;margin-top:6px;color:var(--text)}

/* file group */
.file-group{margin-bottom:4px}
.file-header{background:var(--surface2);padding:10px 14px;display:flex;align-items:center;gap:8px;cursor:pointer;border-radius:6px;user-select:none}
.file-header:hover{background:var(--border)}
.file-path-label{font-family:"SFMono-Regular",Consolas,monospace;font-size:12px;flex:1}
.file-counts{display:flex;gap:6px}
.file-findings{padding-left:20px}

/* filter bar */
.filter-bar{display:flex;gap:8px;margin-bottom:16px;flex-wrap:wrap}
.filter-btn{padding:5px 14px;border-radius:6px;border:1px solid var(--border);background:var(--surface);color:var(--muted);cursor:pointer;font-size:12px;font-weight:500;transition:all .15s}
.filter-btn:hover{border-color:var(--accent);color:var(--text)}
.filter-btn.active{background:var(--accent);border-color:var(--accent);color:#fff}

/* footer */
.footer{border-top:1px solid var(--border);padding:24px;text-align:center;font-size:12px;color:var(--muted);margin-top:40px}
</style>
</head>
<body>
<header class="topbar">
  <div class="topbar-logo">Observer <span>PQC</span></div>
  {{if .Report.Source}}<div class="topbar-source">/ {{.Report.Source}}</div>{{end}}
  <div class="topbar-meta">
    {{if .Report.Ref}}{{.Report.Ref}} · {{end}}
    {{.Report.FilesScanned}} files · {{.Report.RulesApplied}} rules · {{.Report.ScannedAt.Format "2006-01-02"}}
  </div>
</header>

<div class="container">

  <!-- stat cards -->
  <div class="stat-grid">
    <div class="stat-card">
      <div class="label">Files scanned</div>
      <div class="value">{{.Report.FilesScanned}}</div>
    </div>
    <div class="stat-card">
      <div class="label">Active findings</div>
      <div class="value">{{.ActiveCount}}</div>
    </div>
    <div class="stat-card critical">
      <div class="label">Critical</div>
      <div class="value">{{.Report.RiskSummary.Critical}}</div>
    </div>
    <div class="stat-card high">
      <div class="label">High</div>
      <div class="value">{{.Report.RiskSummary.High}}</div>
    </div>
    <div class="stat-card medium">
      <div class="label">Medium</div>
      <div class="value">{{.Report.RiskSummary.Medium}}</div>
    </div>
    <div class="stat-card low">
      <div class="label">Low</div>
      <div class="value">{{.Report.RiskSummary.Low}}</div>
    </div>
    {{if gt .Report.RiskSummary.Exempted 0}}
    <div class="stat-card">
      <div class="label">Exempted</div>
      <div class="value">{{.Report.RiskSummary.Exempted}}</div>
    </div>
    {{end}}
  </div>

  <!-- compliance -->
  <div class="section">
    <div class="section-title">Compliance</div>
    <div class="compliance-grid">
      <div class="compliance-card">
        <div class="fw-name">NIS2 — Article 21</div>
        <span class="badge {{.Report.Compliance.NIS2 | complianceClass}}">{{.Report.Compliance.NIS2}}</span>
      </div>
      <div class="compliance-card">
        <div class="fw-name">DORA — Article 9</div>
        <span class="badge {{.Report.Compliance.DORA | complianceClass}}">{{.Report.Compliance.DORA}}</span>
      </div>
      <div class="compliance-card">
        <div class="fw-name">NIST FIPS 203 (ML-KEM)</div>
        <span class="badge {{.Report.Compliance.NISTFIPS203 | complianceClass}}">{{.Report.Compliance.NISTFIPS203}}</span>
      </div>
      <div class="compliance-card">
        <div class="fw-name">NIST FIPS 204 (ML-DSA)</div>
        <span class="badge {{.Report.Compliance.NISTFIPS204 | complianceClass}}">{{.Report.Compliance.NISTFIPS204}}</span>
      </div>
    </div>
  </div>

  <!-- algorithm breakdown -->
  {{if .AlgRows}}
  <div class="section">
    <div class="section-title">Top algorithms</div>
    <div class="bar-list">
      {{range .AlgRows}}
      <div class="bar-row">
        <span class="bar-label">{{.Algorithm}}</span>
        <div class="bar-track"><div class="bar-fill" style="width:{{.BarPct}}%"></div></div>
        <span class="bar-count">{{.Count}}</span>
      </div>
      {{end}}
    </div>
  </div>
  {{end}}

  <!-- findings by file -->
  {{if .FileRows}}
  <div class="section">
    <div class="section-title">Findings by file</div>
    <div class="filter-bar">
      <button class="filter-btn active" onclick="setFilter('all',this)">All</button>
      <button class="filter-btn" onclick="setFilter('CRITICAL',this)">Critical</button>
      <button class="filter-btn" onclick="setFilter('HIGH',this)">High</button>
      <button class="filter-btn" onclick="setFilter('MEDIUM',this)">Medium</button>
      <button class="filter-btn" onclick="setFilter('LOW',this)">Low</button>
    </div>
    {{range .FileRows}}
    <div class="file-group" data-file="{{.Path}}">
      <div class="file-header" onclick="toggleFile(this)">
        <code class="file-path-label">{{.Short}}</code>
        <div class="file-counts">
          {{if gt .Critical 0}}<span class="badge badge-fail">{{.Critical}} C</span>{{end}}
          {{if gt .High 0}}<span class="badge badge-warn">{{.High}} H</span>{{end}}
          {{if gt .Medium 0}}<span class="badge" style="background:rgba(245,158,11,.1);color:var(--medium)">{{.Medium}} M</span>{{end}}
          {{if gt .Low 0}}<span class="badge" style="background:rgba(59,130,246,.1);color:var(--low)">{{.Low}} L</span>{{end}}
        </div>
        <span style="color:var(--muted);font-size:12px;margin-left:8px">▾</span>
      </div>
      <div class="file-findings">
        <div class="table-card" style="margin:8px 0 16px">
          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Sev</th>
                  <th>Algorithm</th>
                  <th class="num">Line</th>
                  <th>Quantum threat</th>
                  <th>Message</th>
                </tr>
              </thead>
              <tbody>
                {{range .Findings}}
                <tr class="finding-row" data-sev="{{.Severity}}" onclick="toggleSnippet(this)">
                  <td><span class="{{.Severity | sevClass}}">{{.Severity}}</span></td>
                  <td>{{if .Algorithm}}{{.Algorithm}}{{else}}—{{end}}</td>
                  <td class="num">{{.Line}}</td>
                  <td>{{.QuantumThreat | threatLabel}}</td>
                  <td>{{.Message}}</td>
                </tr>
                {{if .Snippet}}
                <tr class="snippet-row" style="display:none">
                  <td colspan="5"><pre class="snippet">{{.Snippet}}</pre>
                  {{if .Migration}}<div style="margin-top:8px;font-size:12px;color:var(--muted)">Migration: {{.Migration}}</div>{{end}}
                  </td>
                </tr>
                {{end}}
                {{end}}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
    {{end}}
  </div>
  {{end}}

</div>

<div class="footer">
  Generated by <a href="https://github.com/GetQuantumDrive/Observer">Observer</a> {{.ToolVersion}}
</div>

<script>
function toggleFile(header) {
  var body = header.nextElementSibling;
  var arrow = header.querySelector('span:last-child');
  var hidden = body.style.display === 'none';
  body.style.display = hidden ? '' : 'none';
  arrow.textContent = hidden ? '▾' : '▸';
}

function toggleSnippet(row) {
  var next = row.nextElementSibling;
  if (next && next.classList.contains('snippet-row')) {
    next.style.display = next.style.display === 'none' ? '' : 'none';
  }
}

var currentFilter = 'all';
function setFilter(sev, btn) {
  currentFilter = sev;
  document.querySelectorAll('.filter-btn').forEach(function(b) { b.classList.remove('active'); });
  btn.classList.add('active');
  document.querySelectorAll('.finding-row').forEach(function(row) {
    var show = sev === 'all' || row.dataset.sev === sev;
    row.style.display = show ? '' : 'none';
    var snip = row.nextElementSibling;
    if (snip && snip.classList.contains('snippet-row')) {
      if (!show) snip.style.display = 'none';
    }
  });
  // hide file groups where all rows are hidden
  document.querySelectorAll('.file-group').forEach(function(group) {
    var visible = group.querySelectorAll('.finding-row:not([style*="display: none"])').length;
    group.style.display = visible ? '' : 'none';
  });
}
</script>
</body>
</html>
`))
