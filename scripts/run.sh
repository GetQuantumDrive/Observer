#!/usr/bin/env bash
# run.sh — one command to scan all Java crypto targets and produce the report.
#
# Usage: ./scripts/run.sh [options]
#
#   --work-dir DIR          working directory for clones and results
#                           (default: /tmp/observer-scan)
#   --out PREFIX            output file prefix for the report
#                           (default: <work-dir>/observer-report)
#   --only SLUGS            comma-separated subset of repo slugs to scan
#                           e.g. --only bcgit-bc-java,google-tink
#   --html                  generate HTML report in addition to MD/JSON
#   --html-dir DIR          where to write HTML (default: <work-dir>/html)
#   --groundstate-url URL   POST each scan report to a Groundstate server
#   --groundstate-token TOK bearer token for Groundstate (optional)
#
# Examples:
#   ./scripts/run.sh
#   ./scripts/run.sh --work-dir ~/observer-results --html
#   ./scripts/run.sh --only bcgit-bc-java,google-tink --work-dir /tmp/test
#   ./scripts/run.sh --groundstate-url https://app.groundstate.io --groundstate-token $TOKEN
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── defaults ──────────────────────────────────────────────────────────────────
WORK_DIR="${SCAN_WORK_DIR:-/tmp/observer-scan}"
OUT_PREFIX=""
ONLY=""
HTML=false
HTML_DIR=""
GS_URL=""
GS_TOKEN=""

# ── flag parsing ──────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --work-dir)          WORK_DIR="$2";   shift 2 ;;
    --out)               OUT_PREFIX="$2"; shift 2 ;;
    --only)              ONLY="$2";       shift 2 ;;
    --html)              HTML=true;       shift   ;;
    --html-dir)          HTML_DIR="$2";   shift 2 ;;
    --groundstate-url)   GS_URL="$2";    shift 2 ;;
    --groundstate-token) GS_TOKEN="$2";  shift 2 ;;
    -h|--help)
      sed -n '2,22p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) echo "Unknown flag: $1  (try --help)" >&2; exit 1 ;;
  esac
done

[[ -z "$OUT_PREFIX" ]] && OUT_PREFIX="$WORK_DIR/observer-report"
[[ -z "$HTML_DIR"   ]] && HTML_DIR="$WORK_DIR/html"

# ── run scan ──────────────────────────────────────────────────────────────────
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║          Observer — PQC scan of Java crypto projects         ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "  Work dir : $WORK_DIR"
echo "  Report   : $OUT_PREFIX.{md,json}"
[[ -n "$ONLY"    ]] && echo "  Filter   : $ONLY"
[[ -n "$GS_URL"  ]] && echo "  Groundstate : $GS_URL"
$HTML              && echo "  HTML     : $HTML_DIR"
echo ""

SCAN_ARGS=(--work-dir "$WORK_DIR")
[[ -n "$ONLY"     ]] && SCAN_ARGS+=(--only "$ONLY")
[[ -n "$GS_URL"   ]] && SCAN_ARGS+=(--groundstate-url "$GS_URL")
[[ -n "$GS_TOKEN" ]] && SCAN_ARGS+=(--groundstate-token "$GS_TOKEN")

export SCAN_WORK_DIR="$WORK_DIR"
bash "$SCRIPT_DIR/scan-all.sh" "${SCAN_ARGS[@]}"

# ── run aggregation ───────────────────────────────────────────────────────────
echo ""
bash "$SCRIPT_DIR/aggregate.sh" "$WORK_DIR/results" "$OUT_PREFIX"

# ── optional HTML generation ──────────────────────────────────────────────────
if $HTML; then
  echo ""
  bash "$SCRIPT_DIR/html-report.sh" "$WORK_DIR/results" "${OUT_PREFIX}.json" "$HTML_DIR"
fi

echo ""
echo "Done. Reports:"
echo "  $OUT_PREFIX.md"
echo "  $OUT_PREFIX.json"
$HTML && echo "  $HTML_DIR/index.html"
