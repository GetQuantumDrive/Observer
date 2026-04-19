package io.getquantumdrive.observer

import org.gradle.api.Plugin
import org.gradle.api.Project

class ObserverPlugin : Plugin<Project> {
    override fun apply(project: Project) {
        val extension = project.extensions.create("observer", ObserverExtension::class.java)
        extension.rulesRepos.convention(listOf("GetQuantumDrive/Observer-rules"))
        extension.failOn.convention("critical")
        extension.outputFormat.convention("json")

        project.tasks.register("observerScan", ObserverTask::class.java) { task ->
            task.group       = "verification"
            task.description = "Scan for quantum-vulnerable cryptography using Observer."
            task.extension   = extension
            task.reportFile  = project.layout.buildDirectory.file("reports/observer/report.json")
        }
    }
}
