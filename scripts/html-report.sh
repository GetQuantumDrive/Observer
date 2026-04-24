#!/usr/bin/env bash
# html-report.sh — convert observer aggregated JSON into a sleek HTML report.
#
# Usage: html-report.sh <results-dir> <combined-json> [output-dir]
#
#   results-dir    directory containing per-project *.json files (from scan-all.sh)
#   combined-json  aggregated JSON produced by aggregate.sh
#   output-dir     where to write HTML files (default: <results-dir>/html)
#
# Outputs:
#   <output-dir>/index.html           overview of all projects
#   <output-dir>/<slug>.html          per-project detail page
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <results-dir> <combined-json> [output-dir]" >&2
  exit 1
fi

RESULTS_DIR="$1"
COMBINED_JSON="$2"
OUTPUT_DIR="${3:-$RESULTS_DIR/html}"

if ! command -v jq &>/dev/null; then
  echo "ERROR: 'jq' is required but not found in PATH." >&2
  exit 1
fi

mkdir -p "$OUTPUT_DIR"

# ── shared helpers ────────────────────────────────────────────────────────────

severity_class() {
  case "$1" in
    CRITICAL) echo "sev-critical" ;;
    HIGH)     echo "sev-high"     ;;
    MEDIUM)   echo "sev-medium"   ;;
    LOW)      echo "sev-low"      ;;
    *)        echo "sev-info"     ;;
  esac
}

compliance_class() {
  case "$1" in
    "NON-COMPLIANT") echo "badge-fail"    ;;
    "AT RISK")       echo "badge-warn"    ;;
    "COMPLIANT")     echo "badge-pass"    ;;
    *)               echo "badge-unknown" ;;
  esac
}

threat_label() {
  case "$1" in
    shor-broken)        echo "Shor-broken (CRQC)"     ;;
    classical-broken)   echo "Classically broken"     ;;
    grover-reduced)     echo "Grover-reduced"          ;;
    grover-safe)        echo "Grover-safe"             ;;
    pqc-standardized)   echo "PQC standardized"        ;;
    pqc-experimental)   echo "PQC experimental"        ;;
    pqc-broken)         echo "PQC broken"              ;;
    *)                  echo "$1"                       ;;
  esac
}

slug_of() { echo "$1" | tr '/' '-'; }

# ── shared CSS + JS (inlined) ─────────────────────────────────────────────────
read -r -d '' SHARED_STYLE <<'CSS' || true
:root {
  --bg:        #0f1117;
  --surface:   #1a1d27;
  --surface2:  #242736;
  --border:    #2e3146;
  --text:      #e2e4f0;
  --muted:     #8890b0;
  --accent:    #5b6ef5;
  --accent2:   #7c8cff;
  --critical:  #f4455a;
  --high:      #f97316;
  --medium:    #f59e0b;
  --low:       #3b82f6;
  --safe:      #10b981;
  --pass:      #10b981;
  --warn:      #f59e0b;
  --fail:      #f4455a;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  background: var(--bg);
  color: var(--text);
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  font-size: 14px;
  line-height: 1.6;
  min-height: 100vh;
}
a { color: var(--accent2); text-decoration: none; }
a:hover { text-decoration: underline; }

/* ── layout ── */
.topbar {
  background: var(--surface);
  border-bottom: 1px solid var(--border);
  padding: 0 32px;
  display: flex;
  align-items: center;
  gap: 16px;
  height: 56px;
}
.topbar-logo {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.5px;
  color: var(--text);
}
.topbar-logo span { color: var(--accent2); }
.topbar-meta { font-size: 12px; color: var(--muted); margin-left: auto; }

.container { max-width: 1200px; margin: 0 auto; padding: 32px 24px; }

/* ── stat cards ── */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 16px;
  margin-bottom: 32px;
}
.stat-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 20px 18px;
}
.stat-card .label { font-size: 11px; text-transform: uppercase; letter-spacing: .08em; color: var(--muted); margin-bottom: 8px; }
.stat-card .value { font-size: 28px; font-weight: 700; }
.stat-card.critical .value { color: var(--critical); }
.stat-card.high     .value { color: var(--high);     }
.stat-card.medium   .value { color: var(--medium);   }
.stat-card.low      .value { color: var(--low);      }

/* ── section ── */
.section { margin-bottom: 40px; }
.section-title {
  font-size: 13px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: .1em;
  color: var(--muted);
  margin-bottom: 16px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
}

/* ── tables ── */
.table-wrap { overflow-x: auto; }
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
th {
  text-align: left;
  padding: 10px 14px;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: .07em;
  color: var(--muted);
  background: var(--surface);
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
}
td {
  padding: 11px 14px;
  border-bottom: 1px solid var(--border);
  vertical-align: middle;
}
tr:last-child td { border-bottom: none; }
tbody tr:hover { background: var(--surface2); }
.table-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}
.num { text-align: right; font-variant-numeric: tabular-nums; }

/* ── badges ── */
.badge {
  display: inline-block;
  padding: 2px 9px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: .04em;
}
.badge-pass    { background: rgba(16,185,129,.15); color: var(--pass); }
.badge-warn    { background: rgba(245,158,11,.15);  color: var(--warn); }
.badge-fail    { background: rgba(244,69,90,.15);   color: var(--fail); }
.badge-unknown { background: var(--surface2); color: var(--muted); }
.sev-critical { color: var(--critical); font-weight: 700; }
.sev-high     { color: var(--high);     font-weight: 600; }
.sev-medium   { color: var(--medium);   font-weight: 600; }
.sev-low      { color: var(--low);      }
.sev-info     { color: var(--muted);    }

/* ── bar chart ── */
.bar-list { display: flex; flex-direction: column; gap: 10px; }
.bar-row { display: flex; align-items: center; gap: 12px; }
.bar-label { min-width: 160px; font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.bar-track { flex: 1; background: var(--surface2); border-radius: 4px; height: 8px; overflow: hidden; }
.bar-fill  { height: 100%; background: var(--accent); border-radius: 4px; transition: width .4s; }
.bar-count { min-width: 44px; text-align: right; font-size: 13px; color: var(--muted); font-variant-numeric: tabular-nums; }

/* ── compliance grid ── */
.compliance-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 16px;
}
.compliance-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 20px;
}
.compliance-card .fw-name { font-weight: 600; margin-bottom: 12px; }
.compliance-row { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; font-size: 13px; }
.compliance-row:last-child { margin-bottom: 0; }

/* ── breadcrumb ── */
.breadcrumb { font-size: 13px; color: var(--muted); margin-bottom: 24px; }
.breadcrumb a { color: var(--muted); }
.breadcrumb a:hover { color: var(--text); }
.breadcrumb .sep { margin: 0 8px; }

/* ── finding snippet ── */
.snippet {
  background: var(--surface2);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 10px 14px;
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 12px;
  overflow-x: auto;
  white-space: pre;
  color: var(--text);
  margin-top: 4px;
}
.file-path { font-family: "SFMono-Regular", Consolas, monospace; font-size: 12px; color: var(--muted); }
CSS

# ── page shell helpers ────────────────────────────────────────────────────────
html_head() {
  local title="$1"
  cat <<HTML
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${title}</title>
<style>
${SHARED_STYLE}
</style>
</head>
HTML
}

topbar() {
  local scan_date="$1"
  cat <<HTML
<body>
<header class="topbar">
  <div class="topbar-logo">Observer <span>PQC</span></div>
  <div class="topbar-meta">Scan date: ${scan_date}</div>
</header>
HTML
}

# ═════════════════════════════════════════════════════════════════════════════
# INDEX PAGE
# ═════════════════════════════════════════════════════════════════════════════

echo "Generating index.html ..."

SCAN_DATE=$(jq -r '.meta.scan_date'           "$COMBINED_JSON")
TOTAL_PROJECTS=$(jq   '.meta.projects_count'  "$COMBINED_JSON")
TOTAL_FILES=$(jq      '.global.files_scanned' "$COMBINED_JSON")
TOTAL_FINDINGS=$(jq   '.global.total_findings' "$COMBINED_JSON")
TOTAL_CRITICAL=$(jq   '.global.critical'       "$COMBINED_JSON")
TOTAL_HIGH=$(jq       '.global.high'           "$COMBINED_JSON")
TOTAL_MEDIUM=$(jq     '.global.medium'         "$COMBINED_JSON")
TOTAL_LOW=$(jq        '.global.low'            "$COMBINED_JSON")

NC_NIS2=$(jq '.global.compliance_summary.non_compliant_nis2' "$COMBINED_JSON")
AR_NIS2=$(jq '.global.compliance_summary.at_risk_nis2'       "$COMBINED_JSON")
OK_NIS2=$(jq '.global.compliance_summary.compliant_nis2'     "$COMBINED_JSON")
NC_DORA=$(jq '.global.compliance_summary.non_compliant_dora' "$COMBINED_JSON")
AR_DORA=$(jq '.global.compliance_summary.at_risk_dora'       "$COMBINED_JSON")
OK_DORA=$(jq '.global.compliance_summary.compliant_dora'     "$COMBINED_JSON")

MAX_ALG_COUNT=$(jq '[.global.top_algorithms[].count] | max // 1' "$COMBINED_JSON")

{
  html_head "Observer PQC Report — ${SCAN_DATE}"
  topbar "$SCAN_DATE"

  cat <<HTML
<div class="container">

  <div class="stat-grid">
    <div class="stat-card">
      <div class="label">Projects</div>
      <div class="value">${TOTAL_PROJECTS}</div>
    </div>
    <div class="stat-card">
      <div class="label">Files scanned</div>
      <div class="value">${TOTAL_FILES}</div>
    </div>
    <div class="stat-card">
      <div class="label">Total findings</div>
      <div class="value">${TOTAL_FINDINGS}</div>
    </div>
    <div class="stat-card critical">
      <div class="label">Critical</div>
      <div class="value">${TOTAL_CRITICAL}</div>
    </div>
    <div class="stat-card high">
      <div class="label">High</div>
      <div class="value">${TOTAL_HIGH}</div>
    </div>
    <div class="stat-card medium">
      <div class="label">Medium</div>
      <div class="value">${TOTAL_MEDIUM}</div>
    </div>
    <div class="stat-card low">
      <div class="label">Low</div>
      <div class="value">${TOTAL_LOW}</div>
    </div>
  </div>

  <div class="section">
    <div class="section-title">Compliance</div>
    <div class="compliance-grid">
      <div class="compliance-card">
        <div class="fw-name">NIS2 — Article 21</div>
        <div class="compliance-row"><span>Non-compliant</span><span class="badge badge-fail">${NC_NIS2}</span></div>
        <div class="compliance-row"><span>At risk</span><span class="badge badge-warn">${AR_NIS2}</span></div>
        <div class="compliance-row"><span>Compliant</span><span class="badge badge-pass">${OK_NIS2}</span></div>
      </div>
      <div class="compliance-card">
        <div class="fw-name">DORA — Article 9</div>
        <div class="compliance-row"><span>Non-compliant</span><span class="badge badge-fail">${NC_DORA}</span></div>
        <div class="compliance-row"><span>At risk</span><span class="badge badge-warn">${AR_DORA}</span></div>
        <div class="compliance-row"><span>Compliant</span><span class="badge badge-pass">${OK_DORA}</span></div>
      </div>
    </div>
  </div>

  <div class="section">
    <div class="section-title">Top algorithms</div>
    <div class="bar-list">
HTML

  jq -r --argjson max "$MAX_ALG_COUNT" '
    .global.top_algorithms[] |
    @json | . as $row | (fromjson | .algorithm) as $alg | (fromjson | .count) as $cnt |
    ($cnt * 100 / $max | floor) as $pct |
    "<div class=\"bar-row\"><span class=\"bar-label\">\($alg)</span><div class=\"bar-track\"><div class=\"bar-fill\" style=\"width:\($pct)%\"></div></div><span class=\"bar-count\">\($cnt)</span></div>"
  ' "$COMBINED_JSON"

  cat <<HTML
    </div>
  </div>

  <div class="section">
    <div class="section-title">Projects</div>
    <div class="table-card">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Project</th>
              <th class="num">Files</th>
              <th class="num">Critical</th>
              <th class="num">High</th>
              <th class="num">Medium</th>
              <th class="num">Low</th>
              <th class="num">Total</th>
              <th>Top algorithm</th>
              <th>NIS2</th>
              <th>DORA</th>
            </tr>
          </thead>
          <tbody>
HTML

  jq -r '
    .projects[] |
    .source as $src |
    ($src | gsub("/"; "-")) as $slug |
    "<tr>" +
    "<td><a href=\"\($slug).html\">\($src)</a></td>" +
    "<td class=\"num\">\(.files_scanned)</td>" +
    "<td class=\"num sev-critical\">\(.risk_summary.critical)</td>" +
    "<td class=\"num sev-high\">\(.risk_summary.high)</td>" +
    "<td class=\"num sev-medium\">\(.risk_summary.medium)</td>" +
    "<td class=\"num sev-low\">\(.risk_summary.low)</td>" +
    "<td class=\"num\">\(.risk_summary.total)</td>" +
    "<td>\(.top_algorithm // "—")</td>" +
    "<td><span class=\"badge \(if .compliance.nis2 == "NON-COMPLIANT" then "badge-fail" elif .compliance.nis2 == "AT RISK" then "badge-warn" else "badge-pass" end)\">\(.compliance.nis2 // "—")</span></td>" +
    "<td><span class=\"badge \(if .compliance.dora == "NON-COMPLIANT" then "badge-fail" elif .compliance.dora == "AT RISK" then "badge-warn" else "badge-pass" end)\">\(.compliance.dora // "—")</span></td>" +
    "</tr>"
  ' "$COMBINED_JSON"

  cat <<HTML
          </tbody>
        </table>
      </div>
    </div>
  </div>

</div>
</body>
</html>
HTML

} > "$OUTPUT_DIR/index.html"

echo "  → $OUTPUT_DIR/index.html"

# ═════════════════════════════════════════════════════════════════════════════
# PER-PROJECT PAGES
# ═════════════════════════════════════════════════════════════════════════════

shopt -s nullglob
for PROJECT_JSON in "$RESULTS_DIR"/*.json; do
  # skip the combined JSON if it landed in results dir
  [[ "$PROJECT_JSON" == "$COMBINED_JSON" ]] && continue

  SLUG="$(basename "$PROJECT_JSON" .json)"
  SOURCE="$(jq -r '.source' "$PROJECT_JSON")"
  PROJ_DATE="$(jq -r '.scanned_at // ""' "$PROJECT_JSON" | cut -c1-10)"
  [[ -z "$PROJ_DATE" ]] && PROJ_DATE="$SCAN_DATE"

  FILES=$(jq '.files_scanned'        "$PROJECT_JSON")
  RULES=$(jq '.rules_applied'        "$PROJECT_JSON")
  DUR=$(  jq '.duration_ms // 0'     "$PROJECT_JSON")
  P_CRITICAL=$(jq '.risk_summary.critical' "$PROJECT_JSON")
  P_HIGH=$(    jq '.risk_summary.high'     "$PROJECT_JSON")
  P_MEDIUM=$(  jq '.risk_summary.medium'   "$PROJECT_JSON")
  P_LOW=$(     jq '.risk_summary.low'      "$PROJECT_JSON")
  P_TOTAL=$(   jq '.risk_summary.total'    "$PROJECT_JSON")
  P_EXEMPTED=$(jq '.risk_summary.exempted // 0' "$PROJECT_JSON")

  P_NIS2=$(jq -r '.compliance.nis2 // "—"'        "$PROJECT_JSON")
  P_DORA=$(jq -r '.compliance.dora // "—"'         "$PROJECT_JSON")
  P_FIPS203=$(jq -r '.compliance.nist_fips203 // "—"' "$PROJECT_JSON")
  P_FIPS204=$(jq -r '.compliance.nist_fips204 // "—"' "$PROJECT_JSON")

  # top algorithms for this project
  MAX_PROJ_ALG=$(jq '
    [.findings[]
     | select(.status == "active" and (.algorithm // "") != "")
     | .algorithm]
    | group_by(.) | map(length) | max // 1
  ' "$PROJECT_JSON")

  echo "Generating ${SLUG}.html ..."

  {
    html_head "${SOURCE} — Observer PQC"
    topbar "$SCAN_DATE"

    cat <<HTML
<div class="container">
  <div class="breadcrumb">
    <a href="index.html">All projects</a>
    <span class="sep">›</span>
    ${SOURCE}
  </div>

  <div class="stat-grid">
    <div class="stat-card">
      <div class="label">Files scanned</div>
      <div class="value">${FILES}</div>
    </div>
    <div class="stat-card">
      <div class="label">Rules applied</div>
      <div class="value">${RULES}</div>
    </div>
    <div class="stat-card critical">
      <div class="label">Critical</div>
      <div class="value">${P_CRITICAL}</div>
    </div>
    <div class="stat-card high">
      <div class="label">High</div>
      <div class="value">${P_HIGH}</div>
    </div>
    <div class="stat-card medium">
      <div class="label">Medium</div>
      <div class="value">${P_MEDIUM}</div>
    </div>
    <div class="stat-card low">
      <div class="label">Low</div>
      <div class="value">${P_LOW}</div>
    </div>
    <div class="stat-card">
      <div class="label">Exempted</div>
      <div class="value">${P_EXEMPTED}</div>
    </div>
  </div>

  <div class="section">
    <div class="section-title">Compliance</div>
    <div class="compliance-grid">
      <div class="compliance-card">
        <div class="fw-name">NIS2</div>
        <div class="compliance-row"><span class="badge $(compliance_class "$P_NIS2")">${P_NIS2}</span></div>
      </div>
      <div class="compliance-card">
        <div class="fw-name">DORA</div>
        <div class="compliance-row"><span class="badge $(compliance_class "$P_DORA")">${P_DORA}</span></div>
      </div>
      <div class="compliance-card">
        <div class="fw-name">NIST FIPS 203</div>
        <div class="compliance-row"><span class="badge $(compliance_class "$P_FIPS203")">${P_FIPS203}</span></div>
      </div>
      <div class="compliance-card">
        <div class="fw-name">NIST FIPS 204</div>
        <div class="compliance-row"><span class="badge $(compliance_class "$P_FIPS204")">${P_FIPS204}</span></div>
      </div>
    </div>
  </div>

  <div class="section">
    <div class="section-title">Top algorithms</div>
    <div class="bar-list">
HTML

    jq -r --argjson max "$MAX_PROJ_ALG" '
      [.findings[]
       | select(.status == "active" and (.algorithm // "") != "")
       | .algorithm]
      | group_by(.)
      | map({ alg: .[0], cnt: length })
      | sort_by(-.cnt)
      | .[0:10][]
      | ($cnt * 100 / $max | floor) as $pct
      | "<div class=\"bar-row\"><span class=\"bar-label\">\(.alg)</span><div class=\"bar-track\"><div class=\"bar-fill\" style=\"width:\($pct)%\"></div></div><span class=\"bar-count\">\(.cnt)</span></div>"
    ' "$PROJECT_JSON" 2>/dev/null || true

    cat <<HTML
    </div>
  </div>

  <div class="section">
    <div class="section-title">Findings</div>
    <div class="table-card">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Severity</th>
              <th>Algorithm</th>
              <th>File</th>
              <th class="num">Line</th>
              <th>Quantum threat</th>
              <th>Message</th>
            </tr>
          </thead>
          <tbody>
HTML

    jq -r '
      .findings[]
      | select(.status == "active")
      | . as $f
      | ($f.severity | ascii_downcase) as $sevlc
      | "<tr>" +
        "<td><span class=\"sev-\($sevlc)\">\($f.severity)</span></td>" +
        "<td>\($f.algorithm // "—")</td>" +
        "<td class=\"file-path\">\($f.file | split("/") | .[-3:] | join("/"))</td>" +
        "<td class=\"num\">\($f.line)</td>" +
        "<td>\($f.quantum_threat // "—")</td>" +
        "<td>\($f.message // "")</td>" +
        "</tr>"
    ' "$PROJECT_JSON" 2>/dev/null || true

    cat <<HTML
          </tbody>
        </table>
      </div>
    </div>
  </div>

</div>
</body>
</html>
HTML

  } > "$OUTPUT_DIR/${SLUG}.html"

  echo "  → $OUTPUT_DIR/${SLUG}.html"
done

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  HTML report complete → $OUTPUT_DIR/index.html"
echo "═══════════════════════════════════════════════════════════════"
