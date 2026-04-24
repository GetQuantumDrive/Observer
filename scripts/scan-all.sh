#!/usr/bin/env bash
# scan-all.sh — scan every target in targets.yaml and produce a full report.
#
# Usage: scan-all.sh [options]
#
#   --work-dir DIR          working directory for clones, results, and reports
#                           (default: /tmp/observer-scan)
#   --out PREFIX            output file prefix for aggregated reports
#                           (default: <work-dir>/observer-report)
#   --only SLUGS            comma-separated subset of repo slugs to scan
#                           e.g. --only bcgit-bc-java,google-tink
#   --html                  generate HTML report after aggregation (always on)
#   --html-dir DIR          where to write HTML files (default: <work-dir>/html)
#   --groundstate-url URL   POST each completed scan JSON to a Groundstate server
#   --groundstate-token TOK bearer token for Groundstate authentication
#
# Pipeline:
#   1. Build or locate the observer binary
#   2. Clone + scan each target  →  <work-dir>/results/<slug>.json
#   3. aggregate.sh              →  <out-prefix>.{json,md}
#   4. html-report.sh            →  <html-dir>/index.html + <html-dir>/<slug>.html
#
# Examples:
#   ./scripts/scan-all.sh
#   ./scripts/scan-all.sh --work-dir ~/observer-results
#   ./scripts/scan-all.sh --only bcgit-bc-java,google-tink --work-dir /tmp/test
#   ./scripts/scan-all.sh --groundstate-url https://app.groundstate.io \
#                         --groundstate-token "$TOKEN"
set -euo pipefail

# ── paths ─────────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ── defaults ──────────────────────────────────────────────────────────────────
WORK_DIR="${SCAN_WORK_DIR:-/tmp/observer-scan}"
OUT_PREFIX=""
ONLY=""
HTML_DIR=""
GS_URL=""
GS_TOKEN=""

# ── flag parsing ──────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --work-dir)          WORK_DIR="$2";   shift 2 ;;
    --out)               OUT_PREFIX="$2"; shift 2 ;;
    --only)              ONLY="$2";       shift 2 ;;
    --html)              shift ;; # kept for backwards compat; HTML is always generated
    --html-dir)          HTML_DIR="$2";   shift 2 ;;
    --groundstate-url)   GS_URL="$2";    shift 2 ;;
    --groundstate-token) GS_TOKEN="$2";  shift 2 ;;
    -h|--help)
      sed -n '2,30p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) echo "Unknown flag: $1  (try --help)" >&2; exit 1 ;;
  esac
done

[[ -z "$OUT_PREFIX" ]] && OUT_PREFIX="$WORK_DIR/observer-report"
[[ -z "$HTML_DIR"   ]] && HTML_DIR="$WORK_DIR/html"

RESULTS_DIR="$WORK_DIR/results"
CLONES_DIR="$WORK_DIR/clones"
TARGETS_YAML="$SCRIPT_DIR/targets.yaml"
OBSERVER_BIN="$WORK_DIR/observer"

mkdir -p "$RESULTS_DIR" "$CLONES_DIR"

# ── dependency checks ─────────────────────────────────────────────────────────
require() {
  if ! command -v "$1" &>/dev/null; then
    echo "ERROR: '$1' is required but not found in PATH." >&2
    echo "       Install it and re-run, or check your PATH." >&2
    exit 1
  fi
}
require git
require jq

# ── banner ────────────────────────────────────────────────────────────────────
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║          Observer — PQC scan of Java crypto projects         ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "  Work dir : $WORK_DIR"
echo "  Report   : $OUT_PREFIX.{json,md}"
echo "  HTML     : $HTML_DIR/index.html"
[[ -n "$ONLY"   ]] && echo "  Filter   : $ONLY"
[[ -n "$GS_URL" ]] && echo "  Groundstate : $GS_URL"
echo ""

# ── build observer ────────────────────────────────────────────────────────────
if [[ -x "$OBSERVER_BIN" ]]; then
  echo "observer binary already present at $OBSERVER_BIN"
elif command -v observer &>/dev/null; then
  OBSERVER_BIN="$(command -v observer)"
  echo "Using observer from PATH: $OBSERVER_BIN"
else
  require go
  echo "Building observer binary..."
  (cd "$REPO_ROOT" && go build -o "$OBSERVER_BIN" ./cmd/observer/)
  echo "Built: $OBSERVER_BIN"
fi

# ── parse targets.yaml ────────────────────────────────────────────────────────
# Prefer yq > python3 > awk.
parse_targets() {
  if command -v yq &>/dev/null; then
    yq -r '.targets[] | "\(.repo)|\(.name)|\(.category)"' "$TARGETS_YAML"
  elif command -v python3 &>/dev/null; then
    python3 - "$TARGETS_YAML" <<'PYEOF'
import sys, re

path = sys.argv[1]
with open(path) as f:
    content = f.read()

entries = re.findall(
    r'- name:\s+"([^"]+)".*?repo:\s+"([^"]+)".*?category:\s+"([^"]+)"',
    content, re.DOTALL
)
for name, repo, cat in entries:
    print(f"{repo}|{name}|{cat}")
PYEOF
  else
    awk '
      /^  - name:/ { name=$0; sub(/.*"/, "", name); sub(/".*/, "", name) }
      /^    repo:/  { repo=$0; sub(/.*"/, "", repo); sub(/".*/, "", repo) }
      /^    category:/ {
        cat=$0; sub(/.*"/, "", cat); sub(/".*/, "", cat)
        print repo "|" name "|" cat
      }
    ' "$TARGETS_YAML"
  fi
}

slug_of() { echo "$1" | tr '/' '-'; }

in_filter() {
  local slug="$1"
  [[ -z "$ONLY" ]] && return 0
  IFS=',' read -ra ITEMS <<< "$ONLY"
  for item in "${ITEMS[@]}"; do
    [[ "$item" == "$slug" ]] && return 0
  done
  return 1
}

# ── scan loop ─────────────────────────────────────────────────────────────────
TOTAL=0
SKIPPED=0
FAILED=0

while IFS='|' read -r repo name category; do
  slug="$(slug_of "$repo")"

  if ! in_filter "$slug"; then
    (( SKIPPED++ )) || true
    continue
  fi

  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  [$((TOTAL+1))] $name  ($repo)"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

  clone_dir="$CLONES_DIR/$slug"
  result_file="$RESULTS_DIR/$slug.json"

  # Clone (skip if already present)
  if [[ -d "$clone_dir/.git" ]]; then
    echo "  clone already present, skipping git clone"
  else
    echo "  cloning $repo ..."
    git clone --depth 1 --filter=blob:none \
      "https://github.com/$repo.git" "$clone_dir" 2>&1 | \
      sed 's/^/    /'
  fi

  # Scan
  echo "  scanning with observer..."
  if "$OBSERVER_BIN" \
      --dir "$clone_dir" \
      --format json \
      --fail-on never \
      --source "$repo" \
      > "$result_file" 2>&1; then
    findings=$(python3 -c "import json,sys; d=json.load(open('$result_file')); print(d['risk_summary']['total'])" 2>/dev/null || echo "?")
    echo "  done — $findings total findings → $result_file"
    # POST to Groundstate if configured
    if [[ -n "$GS_URL" ]]; then
      GS_ARGS=(-s -o /dev/null -w "%{http_code}" \
               -X POST "${GS_URL}/api/reports" \
               -H "Content-Type: application/json" \
               --data-binary "@${result_file}")
      [[ -n "$GS_TOKEN" ]] && GS_ARGS+=(-H "Authorization: Bearer ${GS_TOKEN}")
      GS_STATUS=$(curl "${GS_ARGS[@]}" 2>/dev/null || echo "000")
      if [[ "$GS_STATUS" -ge 200 && "$GS_STATUS" -lt 300 ]]; then
        echo "  groundstate → posted (HTTP $GS_STATUS)"
      else
        echo "  groundstate → warning: server returned HTTP $GS_STATUS" >&2
      fi
    fi
    (( TOTAL++ )) || true
  else
    echo "  ERROR: observer exited non-zero for $repo" >&2
    (( FAILED++ )) || true
    (( TOTAL++ )) || true
  fi

done < <(parse_targets)

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  Scan complete"
echo "  Scanned : $((TOTAL - FAILED)) / $TOTAL  (skipped: $SKIPPED, failed: $FAILED)"
echo "  Results : $RESULTS_DIR"
echo "═══════════════════════════════════════════════════════════════"

# ── aggregate ─────────────────────────────────────────────────────────────────
echo ""
bash "$SCRIPT_DIR/aggregate.sh" "$RESULTS_DIR" "$OUT_PREFIX"

# ── HTML report ───────────────────────────────────────────────────────────────
echo ""
bash "$SCRIPT_DIR/html-report.sh" "$RESULTS_DIR" "${OUT_PREFIX}.json" "$HTML_DIR"

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                        Done                                  ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "  $OUT_PREFIX.json"
echo "  $OUT_PREFIX.md"
echo "  $HTML_DIR/index.html"
