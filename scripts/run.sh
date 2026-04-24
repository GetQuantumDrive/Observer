#!/usr/bin/env bash
# run.sh — forwards to scan-all.sh, which runs the full pipeline.
# Kept for backwards compatibility; prefer calling scan-all.sh directly.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec bash "$SCRIPT_DIR/scan-all.sh" "$@"
