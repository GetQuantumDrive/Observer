#!/usr/bin/env bash
# aggregate.sh — read observer result JSONs and produce a markdown report + combined JSON.
#
# Usage: aggregate.sh <results-dir> [output-prefix]
#   results-dir    directory containing *.json observer scan reports
#   output-prefix  path prefix for output files (default: <results-dir>/observer-report)
#
# Outputs:
#   {output-prefix}.md    markdown report (blog / monthly digest ready)
#   {output-prefix}.json  combined JSON with per-project summaries + global stats
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <results-dir> [output-prefix]" >&2
  exit 1
fi

RESULTS_DIR="$1"
OUTPUT_PREFIX="${2:-$RESULTS_DIR/observer-report}"

# ── dependency checks ─────────────────────────────────────────────────────────
if ! command -v jq &>/dev/null; then
  echo "ERROR: 'jq' is required but not found in PATH." >&2
  echo "       Install with: apt-get install jq  OR  brew install jq" >&2
  exit 1
fi

shopt -s nullglob
JSON_FILES=("$RESULTS_DIR"/*.json)

if [[ ${#JSON_FILES[@]} -eq 0 ]]; then
  echo "ERROR: no *.json files found in $RESULTS_DIR" >&2
  exit 1
fi

SCAN_DATE="$(date -u '+%Y-%m-%d')"

echo "Aggregating ${#JSON_FILES[@]} result file(s) from $RESULTS_DIR ..."

# ── build combined JSON ───────────────────────────────────────────────────────
# Each report → extract summary fields, tag with slug from filename
COMBINED_JSON="${OUTPUT_PREFIX}.json"

jq -n \
  --arg scan_date "$SCAN_DATE" \
  --argjson reports "$(
    jq -s '
      map({
        source:        .source,
        scanned_at:    .scanned_at,
        files_scanned: .files_scanned,
        rules_applied: .rules_applied,
        duration_ms:   .duration_ms,
        risk_summary:  .risk_summary,
        compliance:    .compliance,
        top_findings:  (
          .findings
          | map(select(.status == "active"))
          | sort_by(
              if   .severity == "CRITICAL" then 0
              elif .severity == "HIGH"     then 1
              elif .severity == "MEDIUM"   then 2
              elif .severity == "LOW"      then 3
              else                              4 end
            )
          | .[0:5]
          | map({ rule_id, file, line, algorithm, severity, quantum_threat, message })
        ),
        top_algorithm: (
          .findings
          | map(select(.status == "active" and .algorithm != null and .algorithm != ""))
          | group_by(.algorithm)
          | map({ algorithm: .[0].algorithm, count: length })
          | sort_by(-.count)
          | .[0].algorithm // "n/a"
        )
      })
    ' "${JSON_FILES[@]}"
  )" \
  '{
    meta: {
      scan_date:      $scan_date,
      tool:           "observer",
      projects_count: ($reports | length)
    },
    global: {
      files_scanned: ($reports | map(.files_scanned) | add // 0),
      total_findings: ($reports | map(.risk_summary.total) | add // 0),
      critical:       ($reports | map(.risk_summary.critical) | add // 0),
      high:           ($reports | map(.risk_summary.high) | add // 0),
      medium:         ($reports | map(.risk_summary.medium) | add // 0),
      low:            ($reports | map(.risk_summary.low) | add // 0),
      by_quantum_threat: (
        $reports
        | map(.risk_summary.by_quantum_threat // {})
        | reduce .[] as $item (
            {};
            . as $acc |
            ($item | keys[]) as $k |
            $acc + { ($k): (($acc[$k] // 0) + $item[$k]) }
          )
      ),
      top_algorithms: (
        $reports
        | map(
            .source as $src |
            .risk_summary |
            . as $rs |
            []  # placeholder — top algorithms computed below across all findings
          )
        | add // []
      ),
      compliance_summary: {
        non_compliant_nis2: ($reports | map(select(.compliance.nis2 == "NON-COMPLIANT")) | length),
        at_risk_nis2:       ($reports | map(select(.compliance.nis2 == "AT RISK"))       | length),
        compliant_nis2:     ($reports | map(select(.compliance.nis2 == "COMPLIANT"))     | length),
        non_compliant_dora: ($reports | map(select(.compliance.dora == "NON-COMPLIANT")) | length),
        at_risk_dora:       ($reports | map(select(.compliance.dora == "AT RISK"))       | length),
        compliant_dora:     ($reports | map(select(.compliance.dora == "COMPLIANT"))     | length)
      }
    },
    projects: $reports
  }' > "$COMBINED_JSON"

echo "Combined JSON → $COMBINED_JSON"

# ── top algorithms across all projects (post-process into combined JSON) ───────
# Re-read all findings to get global top-algorithm list
ALL_ALGORITHMS_JSON="$(
  jq -s '
    [.[].findings[]
     | select(.status == "active" and (.algorithm // "") != "")
     | .algorithm]
    | group_by(.)
    | map({ algorithm: .[0], count: length })
    | sort_by(-.count)
    | .[0:10]
  ' "${JSON_FILES[@]}"
)"

# Inject global top_algorithms back into combined JSON
COMBINED_JSON_TMP="${COMBINED_JSON}.tmp"
jq --argjson top "$ALL_ALGORITHMS_JSON" \
  '.global.top_algorithms = $top' \
  "$COMBINED_JSON" > "$COMBINED_JSON_TMP" && mv "$COMBINED_JSON_TMP" "$COMBINED_JSON"

# ── top 10 critical findings across all projects ──────────────────────────────
TOP10_JSON="$(
  jq -s '
    [.[].findings[]
     | select(.status == "active")
     | {source: null, rule_id, file, line, algorithm, severity, quantum_threat, message}]
    | sort_by(
        if   .severity == "CRITICAL" then 0
        elif .severity == "HIGH"     then 1
        elif .severity == "MEDIUM"   then 2
        elif .severity == "LOW"      then 3
        else                              4 end
      )
    | .[0:10]
  ' "${JSON_FILES[@]}"
)"

# ── generate markdown report ──────────────────────────────────────────────────
MD_FILE="${OUTPUT_PREFIX}.md"

{
  # Header
  echo "# Observer PQC Scan Report"
  echo ""
  echo "> Scan date: **${SCAN_DATE}**  |  Tool: [Observer](https://github.com/getquantumdrive/observer)"
  echo ""

  # Global stats
  TOTAL_PROJECTS=$(jq '.meta.projects_count'           "$COMBINED_JSON")
  TOTAL_FILES=$(   jq '.global.files_scanned'          "$COMBINED_JSON")
  TOTAL_FINDINGS=$(jq '.global.total_findings'         "$COMBINED_JSON")
  TOTAL_CRITICAL=$(jq '.global.critical'               "$COMBINED_JSON")
  TOTAL_HIGH=$(    jq '.global.high'                   "$COMBINED_JSON")
  TOTAL_MEDIUM=$(  jq '.global.medium'                 "$COMBINED_JSON")
  TOTAL_LOW=$(     jq '.global.low'                    "$COMBINED_JSON")

  echo "## Overview"
  echo ""
  echo "| Metric | Value |"
  echo "|--------|-------|"
  echo "| Projects scanned | $TOTAL_PROJECTS |"
  echo "| Total files scanned | $TOTAL_FILES |"
  echo "| Total findings | $TOTAL_FINDINGS |"
  echo "| Critical | $TOTAL_CRITICAL |"
  echo "| High | $TOTAL_HIGH |"
  echo "| Medium | $TOTAL_MEDIUM |"
  echo "| Low | $TOTAL_LOW |"
  echo ""

  # Compliance summary
  NC_NIS2=$(jq '.global.compliance_summary.non_compliant_nis2' "$COMBINED_JSON")
  AR_NIS2=$(jq '.global.compliance_summary.at_risk_nis2'       "$COMBINED_JSON")
  OK_NIS2=$(jq '.global.compliance_summary.compliant_nis2'     "$COMBINED_JSON")
  NC_DORA=$(jq '.global.compliance_summary.non_compliant_dora' "$COMBINED_JSON")
  AR_DORA=$(jq '.global.compliance_summary.at_risk_dora'       "$COMBINED_JSON")
  OK_DORA=$(jq '.global.compliance_summary.compliant_dora'     "$COMBINED_JSON")

  echo "## Compliance Summary"
  echo ""
  echo "| Framework | NON-COMPLIANT | AT RISK | COMPLIANT |"
  echo "|-----------|:---:|:---:|:---:|"
  echo "| NIS2 (Article 21) | $NC_NIS2 | $AR_NIS2 | $OK_NIS2 |"
  echo "| DORA (Article 9)  | $NC_DORA | $AR_DORA | $OK_DORA |"
  echo ""

  # Top algorithms
  echo "## Most Common Quantum-Vulnerable Algorithms"
  echo ""
  echo "| Algorithm | Occurrences (active findings) |"
  echo "|-----------|:---:|"
  jq -r '.global.top_algorithms[] | "| \(.algorithm) | \(.count) |"' "$COMBINED_JSON"
  echo ""

  # Quantum threat breakdown
  echo "## Quantum Threat Breakdown"
  echo ""
  echo "| Threat Category | Findings |"
  echo "|-----------------|:---:|"
  jq -r '
    .global.by_quantum_threat
    | to_entries
    | sort_by(-.value)[]
    | "| \(.key) | \(.value) |"
  ' "$COMBINED_JSON"
  echo ""

  # Per-project table
  echo "## Per-Project Results"
  echo ""
  echo "| Project | Files | Critical | High | Med | Low | Total | Top Algorithm | NIS2 | DORA |"
  echo "|---------|------:|:---:|:---:|:---:|:---:|:---:|---------------|------|------|"

  jq -r '
    .projects[]
    | [
        .source,
        (.files_scanned | tostring),
        (.risk_summary.critical | tostring),
        (.risk_summary.high     | tostring),
        (.risk_summary.medium   | tostring),
        (.risk_summary.low      | tostring),
        (.risk_summary.total    | tostring),
        (.top_algorithm // "n/a"),
        (.compliance.nis2 // "—"),
        (.compliance.dora // "—")
      ]
    | "| " + join(" | ") + " |"
  ' "$COMBINED_JSON"
  echo ""

  # Top 10 findings
  echo "## Top 10 Most Critical Findings Across All Projects"
  echo ""
  echo "| Project | File | Line | Algorithm | Severity | Quantum Threat |"
  echo "|---------|------|-----:|-----------|----------|----------------|"

  # Re-extract with source attached (we need to join back to source)
  jq -rs '
    [
      . as $reports |
      range(length) as $i |
      $reports[$i].findings[] |
      select(.status == "active") |
      {
        source: $reports[$i].source,
        file,
        line,
        algorithm,
        severity,
        quantum_threat
      }
    ]
    | sort_by(
        if   .severity == "CRITICAL" then 0
        elif .severity == "HIGH"     then 1
        elif .severity == "MEDIUM"   then 2
        elif .severity == "LOW"      then 3
        else                              4 end
      )
    | .[0:10][]
    | "| \(.source) | `\(.file | split("/") | last)` | \(.line) | \(.algorithm) | \(.severity) | \(.quantum_threat) |"
  ' "${JSON_FILES[@]}"
  echo ""

} > "$MD_FILE"

echo "Markdown report → $MD_FILE"

# ── summary ───────────────────────────────────────────────────────────────────
echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  Aggregation complete"
echo "  Projects : $TOTAL_PROJECTS"
echo "  Findings : $TOTAL_FINDINGS  (critical: $TOTAL_CRITICAL  high: $TOTAL_HIGH)"
echo "  Reports  :"
echo "    $MD_FILE"
echo "    $COMBINED_JSON"
echo "═══════════════════════════════════════════════════════════════"
