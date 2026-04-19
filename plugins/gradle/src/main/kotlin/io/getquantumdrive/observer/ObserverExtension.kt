package io.getquantumdrive.observer

import org.gradle.api.provider.ListProperty
import org.gradle.api.provider.Property
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.Optional

abstract class ObserverExtension {
    /** Observer CLI version to download (defaults to the version baked into this plugin). */
    @get:Input
    abstract val version: Property<String>

    /** Fail the build when findings of this level or worse are found. One of: critical, high, any, never. */
    @get:Input
    abstract val failOn: Property<String>

    /** Report output format written to the report file. One of: json (Observer canonical) | sarif. */
    @get:Input
    abstract val outputFormat: Property<String>

    /**
     * GitHub rules repos (owner/repo[@ref][:path][|token]) applied in list order.
     * Defaults to ["GetQuantumDrive/Observer-rules"]. Set to an empty list to use only local rules.
     * Entries can embed a per-repo token with "|<token>" suffix for cross-org auth.
     */
    @get:Input
    abstract val rulesRepos: ListProperty<String>

    /** Default bearer token applied to rulesRepos entries that do not carry an inline token. */
    @get:Input
    @get:Optional
    abstract val rulesReposToken: Property<String>

    /** Local directory containing custom YAML rules checked into this repo. */
    @get:Input
    @get:Optional
    abstract val rulesDir: Property<String>

    /** Additional local rule directories (highest priority — override repo rules with the same ID). */
    @get:Input
    abstract val extraRulesDirs: ListProperty<String>

    /** Groundstate server base URL (e.g. https://app.groundstate.io). Optional. */
    @get:Input
    @get:Optional
    abstract val groundstateUrl: Property<String>

    /** Bearer token for Groundstate authentication. Optional — prefer env var injection. */
    @get:Input
    @get:Optional
    abstract val groundstateToken: Property<String>
}
