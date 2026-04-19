package io.getquantumdrive.observer

import com.google.gson.Gson
import io.getquantumdrive.observer.model.ScanReport
import org.gradle.api.DefaultTask
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.tasks.OutputFile
import org.gradle.api.tasks.TaskAction

abstract class ObserverTask : DefaultTask() {

    lateinit var extension: ObserverExtension

    @get:OutputFile
    abstract val reportFile: RegularFileProperty

    @TaskAction
    fun scan() {
        val version = extension.version.getOrElse(PluginVersion.VERSION)
        val binary  = BinaryResolver(version).resolve()

        val cmd = mutableListOf(
            binary.absolutePath,
            "--dir",     project.rootDir.absolutePath,
            "--fail-on", extension.failOn.getOrElse("critical"),
            "--format",  extension.outputFormat.getOrElse("json"),
            "--output",  reportFile.asFile.get().also { it.parentFile.mkdirs() }.absolutePath,
            "--source",  project.rootProject.name,
            "--ref",     resolveGitRef(),
            "--sha",     resolveGitSha(),
        )

        // Remote rules repos (fetched by the binary, cached in ~/.cache/observer/rules/).
        // Layered in list order; later entries override earlier ones on duplicate rule IDs.
        extension.rulesRepos.getOrElse(emptyList()).filter { it.isNotBlank() }.forEach {
            cmd += listOf("--rules-repo", it)
        }
        extension.rulesReposToken.orNull?.takeIf { it.isNotBlank() }?.let {
            cmd += listOf("--rules-repos-token", it)
        }

        // Local rules directories (highest priority).
        extension.rulesDir.orNull?.takeIf { it.isNotBlank() }?.let { cmd += listOf("--rules-dir", it) }
        extension.extraRulesDirs.getOrElse(emptyList()).filter { it.isNotBlank() }.forEach {
            cmd += listOf("--rules-dir", it)
        }

        // Groundstate reporting.
        val gsUrl = extension.groundstateUrl.orNull
        if (!gsUrl.isNullOrBlank()) {
            cmd += listOf("--groundstate-url", gsUrl)
            extension.groundstateToken.orNull?.takeIf { it.isNotBlank() }?.let {
                cmd += listOf("--groundstate-token", it)
            }
        }

        logger.lifecycle("Observer: scanning ${project.rootDir} with binary v$version")
        project.exec { spec -> spec.commandLine(cmd) }

        // Summary parsing only works against canonical JSON; SARIF reports skip it.
        if (extension.outputFormat.getOrElse("json") == "json") {
            runCatching {
                val report = Gson().fromJson(reportFile.asFile.get().readText(), ScanReport::class.java)
                printSummary(report)
            }
        }
    }

    private fun printSummary(report: ScanReport) {
        logger.lifecycle("""
            |
            |  Observer - PQC Scan Results
            |  ───────────────────────────────────
            |  Files scanned : ${report.filesScanned}
            |  Total findings: ${report.riskSummary.total}
            |  Critical      : ${report.riskSummary.critical}
            |  High          : ${report.riskSummary.high}
            |  NIS2          : ${report.compliance.nis2}
            |  DORA          : ${report.compliance.dora}
            |  NIST FIPS 203 : ${report.compliance.nistFips203}
            |  NIST FIPS 204 : ${report.compliance.nistFips204}
            |  Report        : ${reportFile.asFile.get()}
        """.trimMargin())
    }

    private fun resolveGitRef(): String = runCatching {
        ProcessBuilder("git", "rev-parse", "--abbrev-ref", "HEAD")
            .directory(project.rootDir)
            .start().inputStream.bufferedReader().readLine() ?: ""
    }.getOrDefault("")

    private fun resolveGitSha(): String = runCatching {
        ProcessBuilder("git", "rev-parse", "HEAD")
            .directory(project.rootDir)
            .start().inputStream.bufferedReader().readLine() ?: ""
    }.getOrDefault("")
}
