plugins {
    `kotlin-dsl`
    id("com.gradle.plugin-publish") version "1.2.1"
}

repositories {
    mavenCentral()
}

dependencies {
    implementation("com.google.code.gson:gson:2.10.1")
    testImplementation(gradleTestKit())
    testImplementation("org.junit.jupiter:junit-jupiter:5.10.2")
    testImplementation("org.assertj:assertj-core:3.25.3")
}

gradlePlugin {
    website = "https://github.com/GetQuantumDrive/Observer"
    vcsUrl  = "https://github.com/GetQuantumDrive/Observer"

    plugins {
        create("observer") {
            id                  = "io.getquantumdrive.observer"
            implementationClass = "io.getquantumdrive.observer.ObserverPlugin"
            displayName         = "Observer - PQC Compliance Scanner"
            description         = "Detect quantum-vulnerable cryptography (RSA, ECDSA, ECDH, DH, DSA) and enforce NIS2/DORA/NIST FIPS 203/204 compliance in Gradle builds."
            tags                = listOf("security", "cryptography", "post-quantum", "compliance", "pqc", "nis2", "dora", "sarif")
        }
    }
}

tasks.withType<Test> {
    useJUnitPlatform()
}

// Source sets for integration tests (require a published or locally-built binary)
sourceSets {
    create("integrationTest") {
        compileClasspath += sourceSets["main"].output + configurations["testRuntimeClasspath"]
        runtimeClasspath += output + compileClasspath
    }
}

val integrationTest by tasks.registering(Test::class) {
    description = "Runs integration tests that invoke the real observer binary."
    group       = "verification"
    testClassesDirs = sourceSets["integrationTest"].output.classesDirs
    classpath       = sourceSets["integrationTest"].runtimeClasspath
    useJUnitPlatform()
    // Allow overriding the binary path in CI before a release exists
    environment("OBSERVER_BINARY_OVERRIDE", System.getenv("OBSERVER_BINARY_OVERRIDE") ?: "")
}
