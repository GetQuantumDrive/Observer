# Security Policy

> *Observe before you're observed.*

Observer scans codebases for quantum-relevant cryptographic vulnerabilities. We take the security of this tool itself seriously because a bug here can hide real crypto risk in our users' codebases.

## Supported versions

| Version | Supported |
|---|---|
| 0.1.x   | Yes (active) |
| < 0.1.0 | No          |

Once Observer reaches 1.0, we will support the current major version and the previous one for security fixes.

## Reporting a vulnerability

Please report security vulnerabilities **privately** via GitHub Security Advisories:

1. Go to https://github.com/GetQuantumDrive/Observer/security/advisories/new
2. Describe the issue, including:
   - Observer version (CLI `--version` or image tag)
   - Reproduction steps or a minimal PoC
   - Impact assessment (CVSS if available)
3. We will acknowledge within **2 business days** and aim to provide a fix or mitigation plan within **14 days** for critical issues.

Do **not** open public issues, PRs, or discussions for security bugs.

## Scope

In scope:
- The Observer CLI (`cmd/observer`, `cmd/action`).
- The Gradle plugin (`plugins/gradle`).
- The shipped Docker image.
- The SARIF renderer and its output (incorrect fingerprints, broken suppression propagation).
- Rule-loading or exemption-handling bugs that cause findings to be silently dropped.

Out of scope:
- Vulnerabilities in third-party rules loaded from user-supplied repos (report those to the rule repo maintainer).
- Vulnerabilities in Groundstate (report at https://github.com/GetQuantumDrive/Groundstate/security).
- Issues that require a malicious user-controlled `.observer.yml` *and* the user choosing to run Observer on untrusted code as themselves — use sandboxing for untrusted scans.

## Coordinated disclosure

We follow coordinated disclosure. After a fix is released, we publish an advisory with CVE (if applicable) and credit the reporter by name (or handle) unless you prefer to remain anonymous.
