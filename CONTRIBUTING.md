# Contributing to Observer

Thanks for your interest in improving Observer. This guide covers the essentials; for the *why* behind design choices, see the code comments and the rule authoring docs.

## Repository layout

| Path | Purpose |
|---|---|
| `cmd/observer` | Standalone CLI binary (used by the Gradle plugin) |
| `cmd/action` | GitHub Action entrypoint (adds GHA annotations, step summaries) |
| `pkg/scanner` | Core scanner, rule loader, exemption engine |
| `pkg/report/sarif` | SARIF 2.1.0 renderer |
| `pkg/groundstate` | Optional POST client for Groundstate |
| `plugins/gradle` | Kotlin DSL Gradle plugin |

Rules themselves live in [GetQuantumDrive/Observer-rules](https://github.com/GetQuantumDrive/Observer-rules). New rules go there, not here.

## Prerequisites

- Go 1.23+
- JDK 21+ (for Gradle plugin work)
- Docker (for end-to-end tests against the image)

## Local development

```bash
# Build both CLI binaries
go build ./cmd/action ./cmd/observer

# Run tests
go test ./...

# Build the Docker image with specific bundled rules
docker build -t observer:dev --build-arg OBSERVER_RULES_REF=main .

# Gradle plugin
cd plugins/gradle
./gradlew build
```

## Adding a new detection rule

1. Open an issue in [Observer-rules](https://github.com/GetQuantumDrive/Observer-rules/issues/new) with the algorithm, language, and a minimal code sample.
2. Follow the YAML schema including `quantum_threat` and `primitive`; both are required. See the [taxonomy table](README.md#taxonomy) for valid values.
3. Add a test fixture that triggers (and does not trigger when suppressed).

## Adding a new language

No scanner change is required. Any rule set can add support for a new language by declaring an `extensions` field on at least one rule for that language. Observer builds the file-extension map from loaded rules at startup, so the new language is recognized as soon as rules that declare it are loaded.

Example: to add Ruby support, publish a rules repo with at least one rule like this:

```yaml
- id: ruby-openssl-rsa
  language: ruby
  extensions: [.rb, .rake]
  pattern: 'OpenSSL::PKey::RSA\.new'
  quantum_threat: shor-broken
  primitive: signature
  algorithm: RSA
  message: "RSA key via OpenSSL is quantum-vulnerable."
  migration: "Replace with an ML-DSA signing library."
```

Every other rule for `language: ruby` in the same (or any layered) rule set will automatically apply to `.rb` and `.rake` files once the extension mapping is established by the first loaded rule.

The five built-in languages (Java, Python, JavaScript, TypeScript, Go) have their extension mappings compiled into the binary and cannot be overridden by rules - only new extensions can be added this way.

## Code style

- Run `go vet ./...` and `gofmt -s -w .` before pushing.
- Keep comments load-bearing: explain *why*, not *what*.
- Don't add speculative configuration or error handling for scenarios that can't happen.

## Pull requests

- Target the `main` branch.
- Include tests for new behavior, especially scanner / exemption / SARIF changes.
- Reference an issue in the description when one exists.
- Sign off commits with `git commit -s` (DCO); we require this for all contributions.

## Release process (maintainers)

1. Update `VERSION` and `CHANGELOG.md`.
2. Tag `vX.Y.Z` on `main`.
3. CI runs `release.yml` (GoReleaser) and `release-gradle.yml` (Plugin Portal).
4. Docker image is rebuilt with `--build-arg OBSERVER_RULES_REF=vX.Y.Z`; Observer-rules must have a matching tag.

## Code of Conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). By participating, you agree to uphold it.
