#!/usr/bin/env bash
# scan-all.sh — clone every target in targets.yaml and run observer on it.
#
# Usage: scan-all.sh [--work-dir DIR] [--only SLUG,SLUG,...]
#
# Outputs one JSON file per project to $WORK_DIR/results/<slug>.json
# Reads target list from scripts/targets.yaml (next to this script).
set -euo pipefail

# ── paths ─────────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ── defaults (overridable via env or flags) ────────────────────────────────────
WORK_DIR="${SCAN_WORK_DIR:-/tmp/observer-scan}"
ONLY=""        # comma-separated slug filter, empty = scan all

# ── flag parsing ──────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --work-dir) WORK_DIR="$2"; shift 2 ;;
    --only)     ONLY="$2";     shift 2 ;;
    *) echo "Unknown flag: $1" >&2; exit 1 ;;
  esac
done

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

# ── build observer ─────────────────────────────────────────────────────────────
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
# We need repo slugs and names.  Prefer yq > python3 > awk.
parse_targets() {
  if command -v yq &>/dev/null; then
    yq -r '.targets[] | "\(.repo)|\(.name)|\(.category)"' "$TARGETS_YAML"
  elif command -v python3 &>/dev/null; then
    python3 - "$TARGETS_YAML" <<'PYEOF'
import sys, re

path = sys.argv[1]
with open(path) as f:
    content = f.read()

# Extract targets block entries naively via regex
entries = re.findall(
    r'- name:\s+"([^"]+)".*?repo:\s+"([^"]+)".*?category:\s+"([^"]+)"',
    content, re.DOTALL
)
for name, repo, cat in entries:
    print(f"{repo}|{name}|{cat}")
PYEOF
  else
    # Minimal awk fallback: assumes strict formatting from our YAML
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

# ── slug helper ───────────────────────────────────────────────────────────────
slug_of() {
  # bcgit/bc-java  →  bcgit-bc-java
  echo "$1" | tr '/' '-'
}

# ── filter helper ─────────────────────────────────────────────────────────────
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
