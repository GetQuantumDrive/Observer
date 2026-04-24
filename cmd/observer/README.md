# Observer CLI

Standalone binary for scanning a local directory for quantum-vulnerable cryptography.

## Installation

**Go install:**

```bash
go install github.com/getquantumdrive/observer/cmd/observer@v0.1.0
```

**Docker (bundled rules, no setup):**

```bash
docker run --rm -v $PWD:/src ghcr.io/getquantumdrive/observer:0.1.0 --dir /src
```

**Build from source:**

```bash
git clone https://github.com/GetQuantumDrive/Observer.git
cd Observer
go build -o observer ./cmd/observer/
```

## Basic usage

```bash
# Scan the current directory, print JSON to stdout
observer --dir .

# Write JSON report to a file
observer --dir . --output report.json

# Self-contained HTML report (open in any browser)
observer --dir . --format html --output report.html

# SARIF for GitHub Code Scanning / SonarQube
observer --dir . --format sarif --output observer.sarif

# Fail the build on any critical finding
observer --dir . --fail-on critical
echo $?   # 1 if critical findings exist, 0 otherwise
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--dir` | `.` | Root directory to scan (recursive) |
| `--format` | `json` | Output format: `json` \| `sarif` \| `html` |
| `--output` | stdout | File path to write the report to |
| `--fail-on` | `critical` | Exit 1 when findings at this level or worse are found: `critical` \| `high` \| `any` \| `never` |
| `--source` | _(empty)_ | Source identifier embedded in the report (e.g. `org/repo`) |
| `--ref` | _(empty)_ | Git ref embedded in the report (e.g. `main`) |
| `--sha` | _(empty)_ | Git commit SHA embedded in the report |
| `--rules-dir` | _(none)_ | Local directory containing custom YAML rules (repeatable) |
| `--rules-repo` | _(none)_ | GitHub rules repo `owner/repo[@ref][:path][|token]` (repeatable) |
| `--rules-repos-token` | _(empty)_ | Default bearer token for `--rules-repo` entries without an inline token |
| `--use-bundled-rules` | `true` | Load rules embedded in the binary (and from `OBSERVER_BUNDLED_RULES_DIR` if set) |
| `--groundstate-url` | _(empty)_ | Groundstate server base URL — POSTs the canonical JSON report to `{url}/api/reports` |
| `--groundstate-token` | _(empty)_ | Bearer token for Groundstate authentication |
| `--version` | — | Print version and exit |

## Output formats

### JSON (default)

Observer's canonical format. Contains the full `ScanReport` struct: every finding with its file, line, algorithm, severity, quantum threat, primitive, snippet, migration guidance, and compliance status.

```bash
observer --dir . --format json --output report.json
```

Use this format to feed results to Groundstate or to build custom dashboards.

### HTML

A self-contained, dependency-free HTML file. No server needed — open it directly in a browser.

```bash
observer --dir . --format html --output report.html
```

The report includes:

- Stat cards: files scanned, active findings, counts by severity
- Compliance badges: NIS2, DORA, NIST FIPS 203, NIST FIPS 204
- Algorithm frequency bar chart (top 10)
- Findings grouped by file — click a finding row to expand the code snippet
- Severity filter bar (All / Critical / High / Medium / Low)

### SARIF 2.1.0

For GitHub Code Scanning, SonarQube, or any SARIF-aware tool.

```bash
observer --dir . --format sarif --output observer.sarif
```

SARIF output carries:
- Stable `partialFingerprints` (hash of rule + file + snippet) — reformatting doesn't re-open issues.
- Suppressions mapped to SARIF `suppressions[]` — Sonar and GitHub honor them natively.
- Observer domain fields under `result.properties.observer.*` (quantumThreat, primitive, composition).

## Rules

Rules are loaded in priority order (later entries win on duplicate IDs):

1. Rules compiled into the binary (`--use-bundled-rules true`, the default)
2. `OBSERVER_BUNDLED_RULES_DIR` env var (Docker hot-patch override)
3. Each `--rules-repo` entry in order
4. Each `--rules-dir` entry in order

### Custom rules from a local directory

```bash
observer --dir . --rules-dir .pqc/rules
```

### Custom rules from a GitHub repository

```bash
# Latest default branch
observer --dir . --rules-repo myorg/pqc-rules

# Pin a tag
observer --dir . --rules-repo myorg/pqc-rules@v1.0

# Private repo with a token
observer --dir . --rules-repo myorg/pqc-rules@v1.0|$RULES_TOKEN

# Multiple repos
observer --dir . \
  --rules-repo GetQuantumDrive/Observer-rules \
  --rules-repo myorg/pqc-rules@v1.0
```

### Rule file format

```yaml
- id: internal-rsa-wrapper
  language: java
  pattern: 'InternalCrypto\.rsaSign\('
  algorithm: RSA
  quantum_threat: shor-broken
  primitive: signature
  message: "Internal RSA signing wrapper is quantum-vulnerable."
  migration: "Replace with InternalCrypto.mlDsaSign() (same interface, ML-DSA)."
```

`severity` is derived from `quantum_threat` if omitted. For languages beyond the built-in five (Java, Python, JavaScript, TypeScript, Go), add an `extensions` field on at least one rule for that language.

## Suppressions

### Inline annotation

```java
// observer:ignore rule=shor-broken.rsa-usage reason="SWIFT ISO 20022 requires RSA until Q1 2027" until=2027-03-31
Cipher c = Cipher.getInstance("RSA/ECB/OAEPWithSHA-256AndMGF1Padding");
```

- `reason="..."` is required.
- `rule=...` is optional — omit to suppress any finding on that line.
- `until=YYYY-MM-DD` is optional — after this date the finding re-surfaces and counts toward `--fail-on`.

Works in `//`, `#`, `--`, `/* */`, and `<!-- -->` comments.

### Config file (`.observer.yml` at repo root)

```yaml
exemptions:
  - rule: shor-broken.rsa-usage
    path: src/main/java/com/acme/partners/swift/**
    reason: "SWIFT ISO 20022 compatibility; migration Q1 2027"
    owner: integrations-team
    until: 2027-03-31
```

Globs use doublestar syntax — `**` matches across directory boundaries.

## Groundstate integration

```bash
observer --dir . \
  --groundstate-url https://app.groundstate.io \
  --groundstate-token "$GROUNDSTATE_TOKEN"
```

The canonical JSON report is POSTed to `{url}/api/reports` after the scan. The selected `--format` still controls what is written to `--output`; Groundstate always receives the canonical JSON regardless of format.

## Environment variables

| Variable | Description |
|---|---|
| `OBSERVER_BUNDLED_RULES_DIR` | Directory of additional bundled rules loaded before `--rules-repo` entries (used by the Docker image) |

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Scan complete, no findings at or above `--fail-on` threshold |
| `1` | Findings at or above `--fail-on` threshold found |
| `2` | Scan or output error (directory not found, bad format flag, etc.) |
