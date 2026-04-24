# Observer bulk-scan scripts

Bash scripts for scanning multiple repositories in one pass and producing aggregated reports. Useful for quarterly PQC audits, ecosystem surveys, or compliance evidence across a portfolio of services.

## Overview

`scan-all.sh` is the single entry point. It runs the full pipeline internally:

```
scan-all.sh
├── 1. build/locate observer binary
├── 2. clone + scan each target  →  <work-dir>/results/<slug>.json
├── 3. aggregate.sh              →  <out-prefix>.{json,md}
└── 4. html-report.sh            →  <html-dir>/index.html + <html-dir>/<slug>.html
```

`aggregate.sh` and `html-report.sh` can also be called directly if you already have result files and only want to re-generate reports.

## Requirements

- `bash` 4+
- `jq` (required by `aggregate.sh` and `html-report.sh`)
- `git` (required for cloning)
- `go` or a pre-built `observer` binary on `PATH`
- `curl` (required for Groundstate posting)
- `yq` or `python3` (optional; used to parse `targets.yaml` — falls back to `awk`)

## `scan-all.sh` — full pipeline

```bash
./scripts/scan-all.sh [options]
```

| Flag | Default | Description |
|---|---|---|
| `--work-dir DIR` | `/tmp/observer-scan` | Working directory for clones, results, and reports |
| `--out PREFIX` | `<work-dir>/observer-report` | Output file prefix for aggregated reports |
| `--only SLUGS` | _(all)_ | Comma-separated subset of repo slugs (e.g. `bcgit-bc-java,google-tink`) |
| `--html-dir DIR` | `<work-dir>/html` | Directory to write HTML files into |
| `--groundstate-url URL` | _(off)_ | POST each completed scan JSON to `{url}/api/reports` |
| `--groundstate-token TOK` | _(empty)_ | Bearer token for Groundstate authentication |

**Examples:**

```bash
# Full scan of all targets
./scripts/scan-all.sh

# Custom work directory
./scripts/scan-all.sh --work-dir ~/observer-results

# Scan a subset of targets
./scripts/scan-all.sh --only bcgit-bc-java,google-tink --work-dir /tmp/test

# Full scan with Groundstate posting
./scripts/scan-all.sh \
  --groundstate-url https://app.groundstate.io \
  --groundstate-token "$GROUNDSTATE_TOKEN"
```

### What it produces

```
<work-dir>/
├── results/
│   ├── bcgit-bc-java.json
│   ├── google-tink.json
│   └── ...
├── observer-report.json    ← aggregated JSON
├── observer-report.md      ← Markdown digest
└── html/
    ├── index.html          ← project overview
    ├── bcgit-bc-java.html
    ├── google-tink.html
    └── ...
```

### `targets.yaml` format

```yaml
targets:
  - name: "Bouncy Castle"
    repo: "bcgit/bc-java"
    category: "crypto-library"
    description: "Widely-used Java crypto library"
```

`repo` must be a GitHub `owner/repo` path. `name` and `category` appear in reports. `description` is informational.

Clones are shallow (`--depth 1 --filter=blob:none`) and cached — re-running skips already-cloned repos.

## `aggregate.sh` — combine results (standalone)

Called automatically by `scan-all.sh`. Run directly to re-generate reports from existing JSON files without re-scanning.

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
    "by_quantum_threat": { "shor-broken": 2095, "classical-broken": 412 },
    "top_algorithms": [{ "algorithm": "RSA", "count": 1204 }],
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
      "risk_summary": { "critical": 142, "high": 87 },
      "compliance": { "nis2": "NON-COMPLIANT", "dora": "NON-COMPLIANT" },
      "top_findings": [],
      "top_algorithm": "RSA"
    }
  ]
}
```

## `html-report.sh` — generate HTML (standalone)

Called automatically by `scan-all.sh`. Run directly to re-generate HTML from existing JSON files.

```bash
./scripts/html-report.sh <results-dir> <combined-json> [output-dir]
```

| Argument | Default | Description |
|---|---|---|
| `results-dir` | _(required)_ | Directory containing per-project `*.json` files |
| `combined-json` | _(required)_ | Aggregated JSON produced by `aggregate.sh` |
| `output-dir` | `<results-dir>/html` | Directory to write HTML files |

**Outputs:**

- `index.html` — overview with stat cards, compliance grid, algorithm bar chart, and a projects table with links to per-project pages
- `<slug>.html` — per-project page: compliance badges, algorithm bars, full findings table

**Example (re-generate HTML only):**

```bash
./scripts/html-report.sh \
  /tmp/observer-scan/results \
  /tmp/observer-scan/observer-report.json \
  /tmp/observer-scan/html
open /tmp/observer-scan/html/index.html
```

No external CSS or JavaScript. All styles are inlined.

## Adding scan targets

Edit `scripts/targets.yaml` and add an entry:

```yaml
targets:
  - name: "My Service"
    repo: "myorg/my-service"
    category: "internal"
    description: "Core payment service"
```

The next `scan-all.sh` run picks it up automatically.

## Running in CI

```yaml
- name: Bulk PQC scan
  run: |
    ./scripts/scan-all.sh \
      --work-dir /tmp/observer \
      --groundstate-url ${{ secrets.GROUNDSTATE_URL }} \
      --groundstate-token ${{ secrets.GROUNDSTATE_TOKEN }}

- name: Upload HTML report
  uses: actions/upload-artifact@v4
  with:
    name: pqc-report
    path: /tmp/observer/html/
```
