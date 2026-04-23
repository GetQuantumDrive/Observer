package io.getquantumdrive.observer

import org.gradle.api.Plugin
import org.gradle.api.Project

class ObserverPlugin : Plugin<Project> {
    override fun apply(project: Project) {
        val ext = project.extensions.create("observer", ObserverExtension::class.java)
        ext.rulesRepos.convention(listOf("GetQuantumDrive/Observer-rules"))
        ext.failOn.convention("critical")
        ext.outputFormat.convention("json")

        project.tasks.register("observerScan", ObserverTask::class.java) {
            group       = "verification"
            description = "Scan for quantum-vulnerable cryptography using Observer."
            extension   = ext
            reportFile.set(project.layout.buildDirectory.file("reports/observer/report.json"))
        }
    }
}
