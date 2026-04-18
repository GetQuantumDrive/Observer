# Observer — PQC Compliance Scanner

GitHub Action that detects quantum-vulnerable cryptography in your codebase and
enforces NIS2 / DORA / NIST FIPS 203 / 204 compliance in CI.

**Narrative:** Observer shows where you are. [Groundstate](https://github.com/GetQuantumDrive/Groundstate) is where you need to be.

---

## Quick start

Add this to `.github/workflows/pqc.yml` in your repository:

```yaml
- uses: GetQuantumDrive/Observer@main
  with:
    fail-on: critical
```

That's it. Observer will scan every push and pull request, annotate vulnerable lines,
and fail the build if CRITICAL findings are present.

---

## What it detects

| Algorithm | Severity | Why |
|-----------|----------|-----|
| RSA (any key size) | HIGH | Broken by Shor's algorithm |
| ECDSA / ECDH | HIGH | Broken by Shor's algorithm |
| Diffie-Hellman / DHE | HIGH | Broken by Shor's algorithm |
| DSA | HIGH | Deprecated by NIST, quantum-vulnerable |
| SHA-1 in signing context | CRITICAL | Cryptographically broken + quantum-vulnerable |

**Supported languages:** Java, Python, JavaScript, TypeScript, Go

**Not flagged (quantum-safe):** AES-256, ChaCha20, SHA-2, SHA-3, ML-KEM (FIPS 203),
ML-DSA (FIPS 204), SLH-DSA (FIPS 205)

---

## Inputs

| Input | Default | Description |
|-------|---------|-------------|
| `fail-on` | `critical` | Fail build on: `critical`, `high`, `any`, `never` |
| `rules-dir` | `.pqc/rules` | Directory with custom YAML rules |
| `report-url` | — | Groundstate server URL to POST report to |
| `report-token` | — | Bearer token for Groundstate authentication |
| `output` | — | Write JSON report to this file path |

## Outputs

| Output | Description |
|--------|-------------|
| `findings` | Total finding count |
| `critical` | Critical finding count |
| `high` | High finding count |
| `compliance` | NIS2 status: `COMPLIANT` \| `AT RISK` \| `NON-COMPLIANT` |
| `report-json` | Full report as JSON string |

---

## Posting reports to Groundstate

```yaml
- uses: GetQuantumDrive/Observer@main
  with:
    fail-on: high
    report-url: ${{ secrets.GROUNDSTATE_URL }}
    report-token: ${{ secrets.GROUNDSTATE_TOKEN }}
```

Groundstate aggregates reports across all your repos, tracks compliance trends,
and generates NIS2 Article 21 / DORA Annex II evidence packages.

---

## Custom detection rules

Create `.pqc/rules/internal.yaml` in your repo:

```yaml
- id: internal-rsa-wrapper
  language: java
  pattern: 'InternalCrypto\.rsaSign\('
  algorithm: RSA
  severity: HIGH
  message: "Internal RSA signing wrapper is quantum-vulnerable."
  migration: "Replace with InternalCrypto.mlDsaSign() — same interface."
```

Rules use Go regex syntax. All built-in rules remain active alongside custom rules.
See `.pqc/rules/example.yaml` in this repo for more examples.

---

## Compliance coverage

| Regulation | Article | What Observer covers |
|------------|---------|---------------------|
| NIS2 | Article 21(2)(h) | Cryptographic policy enforcement |
| DORA | Article 9(4)(c) | Encryption and cryptographic controls |
| NIST FIPS 203 | Full | ML-KEM migration readiness |
| NIST FIPS 204 | Full | ML-DSA migration readiness |

---

## Architecture

Code never leaves your infrastructure. Observer runs entirely inside your CI pipeline.
Only the compliance report (findings metadata — no source code) is sent to Groundstate.

```
Your CI pipeline (GitHub Actions)
├── actions/checkout  ← your code stays here
├── GetQuantumDrive/Observer
│   ├── regex scan: finds vulnerable patterns in-process
│   └── POST /api/reports → Groundstate (findings only, no source code)
└── build continues / fails based on fail-on setting
```
