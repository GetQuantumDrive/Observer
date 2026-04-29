# Observer

**Observer puts your codebase in a superposition of secure and compliant.**

> *Observe before you're observed.*

A post-quantum cryptography scanner for modern CI. Detects quantum-vulnerable crypto, classifies each finding by the threat that breaks it, and produces reports that drop into GitHub Code Scanning, SonarQube, or [Groundstate](https://github.com/GetQuantumDrive/Groundstate) - all without the source code leaving your pipeline.

[![License](https://img.shields.io/badge/license-Apache_2.0-blue.svg)](LICENSE)
[![SARIF 2.1.0](https://img.shields.io/badge/SARIF-2.1.0-green.svg)](#output-formats)

---

## Why

The harvest-now-decrypt-later (HNDL) threat is not theoretical: RSA and ECC protecting data *today* will be broken by a cryptographically relevant quantum computer (CRQC). NIS2 (Article 21), DORA (Article 9), and NIST's FIPS 203/204/205 all require inventory and a migration plan now.

Real companies have mixed crypto: internal services migrate to PQC, but SWIFT, partner APIs, and legacy tools stay classical. Observer is built for this reality: it finds every usage, classifies it by quantum threat, and lets you suppress exceptions with audit metadata instead of pretending they don't exist.

## Quickstart

```yaml
- uses: GetQuantumDrive/Observer@v0.1.0
  with:
    fail-on: critical
```

Scans every push and PR, annotates vulnerable lines, and fails the build on critical findings. `fail-on: critical` is the default — a bare `uses:` line with no `with:` block behaves identically. Default rules are bundled into the Docker image; zero network requests on the default scan path.

## Other integrations

Observer also ships a [Gradle plugin](plugins/gradle/README.md) and a [standalone CLI](cmd/observer/README.md) with the same rule set and output formats.

## Taxonomy

Every rule and finding carries three orthogonal fields, answering *what breaks it* independently of *what kind of crypto it is*.

| `quantumThreat` | Examples | Meaning |
|---|---|---|
| `classical-broken` | MD5, SHA-1, DES, 3DES, RC4, RSA<2048 | Broken today by classical attacks |
| `shor-broken` | RSA, DH, ECDH, ECDSA, EdDSA (any size) | Broken by a CRQC via Shor |
| `grover-reduced` | AES-128, SHA-256 (collision uses) | Halved margin; context-dependent |
| `grover-safe` | AES-256, SHA-384+, SHA-3 | No meaningful reduction |
| `pqc-standardized` | ML-KEM (FIPS 203), ML-DSA (FIPS 204), SLH-DSA (FIPS 205), LMS/XMSS | Compliant |
| `pqc-experimental` | HQC, BIKE, Classic McEliece | Research/hybrid only |
| `pqc-broken` | SIKE, Rainbow, GeMSS | Must not be used |
| `unknown` | Dynamic cipher selection | Needs human review |

`primitive` is one of `asymmetric-encryption`, `key-exchange`, `signature`, `symmetric-cipher`, `hash`, `mac`, `kdf`, `rng`, `aead`, `other`.

`composition: true` marks hybrid constructions (e.g. X25519+ML-KEM).

## Rules

Observer ships with a set of detection rules compiled directly into the binary using Go's embed mechanism. There are no network requests on the default scan path - no git submodule, no download at runtime. All three integrations (GitHub Action, Gradle plugin, standalone CLI) include these bundled rules automatically.

The community-maintained rule set in [GetQuantumDrive/Observer-rules](https://github.com/GetQuantumDrive/Observer-rules) is additionally baked into the Docker image at release time. Either way, you get coverage from the first run with zero extra setup.

You can layer your own rules on top via a local directory or a remote GitHub repository.

### Rule priority

Rules are merged in this order (later entries win on duplicate IDs):

1. Rules compiled into the binary
2. Rules from `rules-repos` / `--rules-repo` entries (fetched from GitHub, cached locally for 24 hours)
3. Rules from `rules-dir` / `--rules-dir` local directories

A custom rule that shares an `id` with a bundled rule replaces it. Rules with unique IDs are additive.

### Rule format

Create one or more YAML files in a directory of your choice:

`.pqc/rules/internal.yaml`:

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

Severity is derived from `quantum_threat` + `primitive`; override it with an explicit `severity:` field if needed.

### Adding rules for a language not built into Observer

Observer has built-in file-extension mappings for Java, Python, JavaScript, TypeScript, and Go. For any other language, add an `extensions` field to at least one rule for that language. Observer reads the declared extensions from the loaded rules and starts scanning matching files - no scanner change needed.

```yaml
# One rule declares the extensions; all rules sharing the same language value
# will automatically apply to those files.
- id: ruby-openssl-rsa
  language: ruby
  extensions: [.rb, .rake]
  pattern: 'OpenSSL::PKey::RSA\.new'
  algorithm: RSA
  quantum_threat: shor-broken
  primitive: signature
  message: "RSA key via OpenSSL::PKey::RSA is quantum-vulnerable."
  migration: "Replace with an ML-DSA signing library."

- id: ruby-openssl-ecdh
  language: ruby
  pattern: 'OpenSSL::PKey::EC\.new'
  algorithm: ECDH
  quantum_threat: shor-broken
  primitive: key-exchange
  message: "EC key exchange via OpenSSL::PKey::EC is quantum-vulnerable."
  migration: "Replace with ML-KEM (FIPS 203)."
```

Extensions must include the leading dot (`.rb`, not `rb`). Built-in language mappings cannot be overridden by rules; rules may only add support for new extensions.

### Adding local rules

Point the integration at the directory containing your YAML files.

**GitHub Action:**

```yaml
- uses: GetQuantumDrive/Observer@v0.1.0
  with:
    rules-dir: .pqc/rules
```

**Gradle plugin:**

```kotlin
observer {
    rulesDir.set(".pqc/rules")
}
```

**Standalone CLI:**

```bash
observer --dir . --rules-dir .pqc/rules
```

### Adding a remote rules repo

Host your rules in any GitHub repository and reference it by `owner/repo`. An optional `@ref` pins a specific branch, tag, or commit.

**GitHub Action:**

```yaml
- uses: GetQuantumDrive/Observer@v0.1.0
  with:
    rules-repos: myorg/pqc-rules@v1.0
    rules-repos-token: ${{ secrets.RULES_TOKEN }}
```

Multiple repos, one per line:

```yaml
- uses: GetQuantumDrive/Observer@v0.1.0
  with:
    rules-repos: |
      myorg/pqc-rules@v1.0
      myorg/pqc-rules-experimental
    rules-repos-token: ${{ secrets.RULES_TOKEN }}
```

**Gradle plugin:**

```kotlin
observer {
    rulesRepos.set(listOf("myorg/pqc-rules@v1.0"))
    rulesReposToken.set(providers.environmentVariable("RULES_TOKEN"))
}
```

**Standalone CLI:**

```bash
observer --dir . --rules-repo myorg/pqc-rules
observer --dir . --rules-repo myorg/pqc-rules@v1.0
```

### Cross-org with per-repo tokens

When rules repos span multiple GitHub organizations, supply a per-repo token inline using `|`:

```yaml
- uses: GetQuantumDrive/Observer@v0.1.0
  with:
    rules-repos: |
      acme/pqc-rules@v1.0|${{ secrets.ACME_TOKEN }}
      partner/shared-rules|${{ secrets.PARTNER_TOKEN }}
```

The single `rules-repos-token` applies to any entry that does not have an inline token.

## Suppressions

Partners on classical crypto, legacy interop, migrations that straddle quarterly boundaries: Observer keeps findings in the report with a disposition, it doesn't hide them.

### Inline annotations

```java
// observer:ignore rule=shor-broken.rsa-usage reason="SWIFT ISO 20022 requires RSA until Q1 2027" until=2027-03-31
Cipher c = Cipher.getInstance("RSA/ECB/OAEPWithSHA-256AndMGF1Padding");
```

Applies to the next non-blank code line. `reason="..."` is required. `rule=...` is optional (absent = suppress any Observer finding on that line). `until=YYYY-MM-DD` is optional; after the date passes, the finding re-surfaces and counts toward `fail-on`.

Works in `//`, `#`, `--`, `/* */`, `<!-- -->` comments across any language.

### Config file

`.observer.yml` at repo root:

```yaml
exemptions:
  - rule: shor-broken.rsa-usage
    path: src/main/java/com/acme/partners/swift/**
    reason: "SWIFT ISO 20022 compatibility; migration Q1 2027"
    owner: integrations-team
    until: 2027-03-31
```

Globs are doublestar-style (`**` matches across directories).

## Output formats

| Format | Flag / input | Best for |
|---|---|---|
| Observer JSON (default) | `--format json` | Groundstate, Observer tooling, custom dashboards |
| SARIF 2.1.0 | `--format sarif` | GitHub Code Scanning, SonarQube External Issues |
| HTML | `--format html` | Self-contained report, artifact uploads, local review |

### HTML report

The HTML format produces a single self-contained file — no external dependencies. Open it in any browser.

```bash
observer --dir . --format html --output report.html
```

It shows stat cards, compliance badges (NIS2 / DORA / FIPS 203 / FIPS 204), an algorithm frequency bar chart, and findings grouped by file with collapsible code snippets and a severity filter.

### SARIF

SARIF output includes:
- Stable `partialFingerprints` so reformatting doesn't re-fire issues.
- Suppressions mapped to SARIF's `suppressions[]`, honored natively by Sonar and GitHub.
- Domain fields (quantumThreat, primitive, composition) under `properties.observer.*`.

**Upload to GitHub Code Scanning:**

```yaml
- uses: GetQuantumDrive/Observer@v0.1.0
  with:
    output: observer.sarif
    output-format: sarif
- uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: observer.sarif
```

**Upload to SonarQube (10.3+ or SonarCloud):**

Generate the SARIF file, then pass it to the SonarQube scanner via `sonar.sarifReportPaths`. Observer suppressions are preserved in the SARIF and honored by SonarQube's won't-fix / false-positive workflow.

GitHub Actions:

```yaml
- uses: GetQuantumDrive/Observer@v0.1.0
  with:
    output: observer.sarif
    output-format: sarif

- uses: SonarSource/sonarqube-scan-action@v3
  env:
    SONAR_TOKEN: ${{ secrets.SONAR_TOKEN }}
    SONAR_HOST_URL: ${{ secrets.SONAR_HOST_URL }}
  with:
    args: -Dsonar.sarifReportPaths=observer.sarif
```

CLI with sonar-scanner:

```bash
observer --dir . --format sarif > observer.sarif
sonar-scanner -Dsonar.sarifReportPaths=observer.sarif
```

## Groundstate integration (optional)

```yaml
- uses: GetQuantumDrive/Observer@v0.1.0
  with:
    report-url: ${{ secrets.GROUNDSTATE_URL }}
    report-token: ${{ secrets.GROUNDSTATE_TOKEN }}
```

Groundstate aggregates reports across repositories, tracks compliance trends, and generates NIS2 Article 21 / DORA Annex II evidence packages. Only findings metadata is posted; your source code never leaves your pipeline.

## Inputs & outputs

See [action.yml](action.yml) for the full list. Key inputs: `fail-on`, `rules-repos`, `rules-repos-token`, `use-bundled-rules`, `rules-dir`, `output`, `output-format`, `report-url`, `report-token`.

Outputs: `findings`, `critical`, `high`, `compliance`, `report-json`.

## Supported languages

Java, Python, JavaScript, TypeScript, Go - built in.

Any other language can be supported by a rule set that declares `extensions` on at least one rule. No scanner changes required. See [Adding rules for a language not built into Observer](#adding-rules-for-a-language-not-built-into-observer) above.

## Architecture

```
Your CI pipeline
├── actions/checkout                  ← source stays here
├── GetQuantumDrive/Observer
│   ├── bundled rules (Docker image)  ← zero network default
│   ├── pattern scan                  ← findings + taxonomy
│   ├── suppressions                  ← inline + .observer.yml
│   └── report: JSON | SARIF
└── optional: POST findings metadata → Groundstate
```

## More documentation

| Reference | Contents |
|---|---|
| [action.yml](action.yml) | Full input/output reference |
| [cmd/observer/README.md](cmd/observer/README.md) | Standalone CLI — all flags, examples, output formats |
| [plugins/gradle/README.md](plugins/gradle/README.md) | Gradle plugin — extension DSL, tasks, Groundstate |
| [scripts/README.md](scripts/README.md) | Bulk scan scripts — scan-all, aggregate, html-report |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). New detection rules go in [Observer-rules](https://github.com/GetQuantumDrive/Observer-rules); open an issue there with the language, algorithm, and a code sample.

## Security

See [SECURITY.md](SECURITY.md) for coordinated disclosure. Report vulnerabilities privately via GitHub Security Advisories.

## License

Apache-2.0, see [LICENSE](LICENSE).
