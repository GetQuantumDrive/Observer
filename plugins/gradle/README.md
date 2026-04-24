# Observer Gradle Plugin

Gradle plugin that runs the Observer PQC scanner as part of your build. Downloads the Observer binary on first use (checksum-verified, cached under `~/.gradle/caches/observer/`) — no manual installation required.

## Requirements

- Gradle 7.6+
- Java 11+

## Setup

**`settings.gradle.kts`:**

```kotlin
pluginManagement {
    repositories {
        gradlePluginPortal()
    }
}
```

**`build.gradle.kts`:**

```kotlin
plugins {
    id("io.getquantumdrive.observer") version "0.1.0"
}
```

## Running

```bash
./gradlew observerScan
```

The task scans the project root, prints a summary to the build log, and writes a report to `build/observer/report.json` (or `.html` / `.sarif` depending on `outputFormat`).

## Configuration

All properties are optional — the defaults work without any `observer {}` block.

```kotlin
observer {
    // Observer CLI version to download.
    // Defaults to the version baked into this plugin release.
    version.set("0.1.0")

    // Fail the build when findings at this severity or worse are found.
    // One of: critical (default) | high | any | never
    failOn.set("critical")

    // Report format written to the output file.
    // One of: json (default) | sarif | html
    outputFormat.set("json")

    // Remote GitHub rules repositories loaded in order.
    // Later entries override earlier ones on duplicate rule IDs.
    // Each entry: "owner/repo[@ref][:path][|token]"
    rulesRepos.set(listOf("GetQuantumDrive/Observer-rules"))

    // Default bearer token for rulesRepos entries without an inline token.
    rulesReposToken.set(providers.environmentVariable("RULES_TOKEN"))

    // Local directory containing custom YAML rules checked into this repo.
    rulesDir.set(".pqc/rules")

    // Additional local rule directories (highest priority).
    extraRulesDirs.set(listOf(".pqc/rules-experimental"))

    // Groundstate server base URL. When set, the JSON report is POSTed
    // to {groundstateUrl}/api/reports after a successful scan.
    groundstateUrl.set(providers.environmentVariable("GROUNDSTATE_URL"))

    // Bearer token for Groundstate authentication.
    groundstateToken.set(providers.environmentVariable("GROUNDSTATE_TOKEN"))
}
```

## Output formats

### JSON (default)

```kotlin
observer {
    outputFormat.set("json")
}
```

Full `ScanReport` written to `build/observer/report.json`. Use this for Groundstate or custom dashboards.

### HTML

```kotlin
observer {
    outputFormat.set("html")
}
```

Self-contained HTML written to `build/observer/report.html`. Open in any browser — no server needed. Includes stat cards, compliance badges, algorithm frequency bar chart, and findings grouped by file with collapsible code snippets.

### SARIF

```kotlin
observer {
    outputFormat.set("sarif")
}
```

SARIF 2.1.0 written to `build/observer/report.sarif`. Pass to `sonar.sarifReportPaths` or upload to GitHub Code Scanning.

## Groundstate integration

When `groundstateUrl` is set the plugin passes it through to the Observer binary, which POSTs the canonical JSON report to `{url}/api/reports` after scanning. This happens regardless of `outputFormat`.

```kotlin
observer {
    groundstateUrl.set(providers.environmentVariable("GROUNDSTATE_URL"))
    groundstateToken.set(providers.environmentVariable("GROUNDSTATE_TOKEN"))
}
```

Keep the token out of `build.gradle.kts` — inject it via environment variable in CI.

## Custom rules

### From a local directory

Check your rules into the repo and point the plugin at the directory:

```kotlin
observer {
    rulesDir.set(".pqc/rules")
}
```

### From a remote GitHub repository

```kotlin
observer {
    rulesRepos.set(listOf(
        "GetQuantumDrive/Observer-rules",  // community rules
        "myorg/pqc-rules@v1.0",           // internal rules, pinned tag
    ))
    rulesReposToken.set(providers.environmentVariable("RULES_TOKEN"))
}
```

For repos spanning multiple organizations, embed a per-repo token inline with `|`:

```kotlin
observer {
    rulesRepos.set(listOf(
        "acme/pqc-rules@v1.0|${System.getenv("ACME_TOKEN")}",
        "partner/shared-rules|${System.getenv("PARTNER_TOKEN")}",
    ))
}
```

## CI example (GitHub Actions)

```yaml
- name: Run Observer PQC scan
  run: ./gradlew observerScan
  env:
    GROUNDSTATE_URL: ${{ secrets.GROUNDSTATE_URL }}
    GROUNDSTATE_TOKEN: ${{ secrets.GROUNDSTATE_TOKEN }}

- name: Upload HTML report
  uses: actions/upload-artifact@v4
  if: always()
  with:
    name: observer-report
    path: build/observer/report.html
```

## Multi-module projects

Apply the plugin to the root project. The scan covers the entire `rootDir` tree regardless of submodule structure. If you need per-module scans, apply and configure the plugin in each submodule's `build.gradle.kts` and set a distinct output path (the plugin always writes to `build/observer/` relative to the applying project).

## Binary caching

The Observer binary is downloaded once and cached at `~/.gradle/caches/observer/<version>/`. Subsequent runs reuse the cached binary without any network access. The cache is keyed on version, so upgrading the plugin version automatically fetches the new binary.

Set `OBSERVER_BINARY_OVERRIDE` to an absolute path to skip downloading and use a local binary instead (useful in CI environments with no outbound network access to GitHub releases).
