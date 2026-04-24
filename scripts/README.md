# Observer bulk-scan scripts

Bash scripts for scanning multiple repositories in one pass and producing aggregated reports. Useful for quarterly PQC audits, ecosystem surveys, or compliance evidence across a portfolio of services.

## Overview

```
run.sh                 ← orchestrator: runs the full pipeline in one command
├── scan-all.sh        ← clones each target and runs observer on it
│     └── results/<slug>.json  (one file per project)
├── aggregate.sh       ← combines all JSONs into a single report
│     └── observer-report.{json,md}
└── html-report.sh     ← converts aggregated JSON to HTML
      └── html/index.html + html/<slug>.html
```

## Requirements

- `bash` 4+
- `jq` (required by `aggregate.sh` and `html-report.sh`)
- `git` (required by `scan-all.sh`)
- `go` or a pre-built `observer` binary on `PATH`
- `curl` (required for Groundstate posting in `scan-all.sh`)
- `yq` or `python3` (optional; used by `scan-all.sh` to parse `targets.yaml` — falls back to `awk`)

## `run.sh` — full pipeline

Runs `scan-all.sh`, then `aggregate.sh`, then optionally `html-report.sh`.

```bash
./scripts/run.sh [options]
```

| Flag | Default | Description |
|---|---|---|
| `--work-dir DIR` | `/tmp/observer-scan` | Working directory for clones, results, and reports |
| `--out PREFIX` | `<work-dir>/observer-report` | Output file prefix (produces `{prefix}.json` and `{prefix}.md`) |
| `--only SLUGS` | _(all)_ | Comma-separated subset of repo slugs to scan (e.g. `bcgit-bc-java,google-tink`) |
| `--html` | _(off)_ | Generate HTML report in addition to JSON + Markdown |
| `--html-dir DIR` | `<work-dir>/html` | Directory to write HTML files into |
| `--groundstate-url URL` | _(off)_ | POST each scan result to a Groundstate server |
| `--groundstate-token TOK` | _(empty)_ | Bearer token for Groundstate authentication |

**Examples:**

```bash
# Full scan of all targets
./scripts/run.sh

# Scan a subset, write results under ~/observer-results, produce HTML
./scripts/run.sh \
  --only bcgit-bc-java,google-tink \
  --work-dir ~/observer-results \
  --html

# Full scan with Groundstate posting
./scripts/run.sh \
  --groundstate-url https://app.groundstate.io \
  --groundstate-token "$GROUNDSTATE_TOKEN"
```

## `scan-all.sh` — clone and scan

Reads `scripts/targets.yaml`, clones each repository, and runs `observer` on it. Writes one JSON file per project to `<work-dir>/results/<slug>.json`.

```bash
./scripts/scan-all.sh [options]
```

| Flag | Default | Description |
|---|---|---|
| `--work-dir DIR` | `/tmp/observer-scan` | Working directory |
| `--only SLUGS` | _(all)_ | Comma-separated slug filter |
| `--groundstate-url URL` | _(off)_ | POST each completed JSON to `{url}/api/reports` |
| `--groundstate-token TOK` | _(empty)_ | Bearer token for Groundstate |

The script builds the `observer` binary from source if it is not already present in the work directory or on `PATH`.

Clones are shallow (`--depth 1 --filter=blob:none`) and cached — re-running skips already-cloned repos.

### `targets.yaml` format

```yaml
targets:
  - name: "Bouncy Castle"
    repo: "bcgit/bc-java"
    category: "crypto-library"
    description: "Widely-used Java crypto library"
```

`repo` must be a GitHub `owner/repo` path. `name` and `category` are used in reports. `description` is informational.

## `aggregate.sh` — combine results

Takes a directory of per-project JSON files and produces an aggregated JSON report and a Markdown digest.

```bash
./scripts/aggregate.sh <results-dir> [output-prefix]
```

| Argument | Default | Description |
|---|---|---|
| `results-dir` | _(required)_ | Directory containing `*.json` observer scan reports |
| `output-prefix` | `<results-dir>/observer-report` | Path prefix for output files |

**Outputs:**

- `{prefix}.json` — combined JSON with global stats, per-project summaries, top algorithms, and compliance counts
- `{prefix}.md` — Markdown digest ready for blog posts or newsletters

**Example:**

```bash
./scripts/aggregate.sh /tmp/observer-scan/results /tmp/observer-scan/observer-report
```

### Combined JSON structure

```json
{
  "meta": {
    "scan_date": "2025-04-24",
    "tool": "observer",
    "projects_count": 25
  },
  "global": {
    "files_scanned": 142381,
    "total_findings": 4821,
    "critical": 1203,
    "high": 892,
    "medium": 2314,
    "low": 412,
    "by_quantum_threat": { "shor-broken": 2095, "classical-broken": 412, ... },
    "top_algorithms": [{ "algorithm": "RSA", "count": 1204 }, ...],
    "compliance_summary": {
      "non_compliant_nis2": 18,
      "at_risk_nis2": 5,
      "compliant_nis2": 2
    }
  },
  "projects": [
    {
      "source": "bcgit/bc-java",
      "files_scanned": 3821,
      "risk_summary": { "critical": 142, "high": 87, ... },
      "compliance": { "nis2": "NON-COMPLIANT", "dora": "NON-COMPLIANT", ... },
      "top_findings": [...],
      "top_algorithm": "RSA"
    }
  ]
}
```

## `html-report.sh` — generate HTML

Converts the aggregated JSON and per-project JSONs into a self-contained HTML report.

```bash
./scripts/html-report.sh <results-dir> <combined-json> [output-dir]
```

| Argument | Default | Description |
|---|---|---|
| `results-dir` | _(required)_ | Directory containing per-project `*.json` files |
| `combined-json` | _(required)_ | Aggregated JSON produced by `aggregate.sh` |
| `output-dir` | `<results-dir>/html` | Directory to write HTML files |

**Outputs:**

- `index.html` — overview with stat cards, compliance grid, algorithm bar chart, and a projects table linking to per-project pages
- `<slug>.html` — per-project page with compliance badges, algorithm bars, and a full findings table

**Example:**

```bash
./scripts/html-report.sh \
  /tmp/observer-scan/results \
  /tmp/observer-scan/observer-report.json \
  /tmp/observer-scan/html
open /tmp/observer-scan/html/index.html
```

No external CSS or JavaScript dependencies. All styles are inlined.

## Adding scan targets

Edit `scripts/targets.yaml`:

```yaml
targets:
  - name: "My Service"
    repo: "myorg/my-service"
    category: "internal"
    description: "Core payment service"
```

Push the file — the next `run.sh` invocation picks it up automatically.

## Running in CI

```yaml
- name: Bulk PQC scan
  run: |
    ./scripts/run.sh \
      --work-dir /tmp/observer \
      --html \
      --groundstate-url ${{ secrets.GROUNDSTATE_URL }} \
      --groundstate-token ${{ secrets.GROUNDSTATE_TOKEN }}

- name: Upload HTML report
  uses: actions/upload-artifact@v4
  with:
    name: pqc-report
    path: /tmp/observer/html/
```
