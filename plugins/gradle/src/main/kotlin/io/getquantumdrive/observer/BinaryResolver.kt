package io.getquantumdrive.observer

import org.gradle.api.GradleException
import java.io.File
import java.math.BigInteger
import java.net.URL
import java.nio.file.Files
import java.security.MessageDigest
import java.util.zip.ZipInputStream

/**
 * Downloads and caches the observer CLI binary for the current platform.
 * Cache location: ~/.gradle/caches/observer/{version}/observer[.exe]
 */
class BinaryResolver(private val version: String) {

    companion object {
        private val GRADLE_USER_HOME: File
            get() = File(System.getProperty("user.home"), ".gradle")
    }

    fun resolve(): File {
        // Allow CI to inject a pre-built binary (used in PR tests before a release exists).
        val override = System.getenv("OBSERVER_BINARY_OVERRIDE")
        if (!override.isNullOrBlank()) {
            val f = File(override)
            if (!f.exists()) throw GradleException("OBSERVER_BINARY_OVERRIDE points to non-existent file: $override")
            return f
        }

        val cacheDir  = File(GRADLE_USER_HOME, "caches/observer/$version")
        val binaryName = if (os() == "windows") "observer.exe" else "observer"
        val cached    = File(cacheDir, binaryName)

        if (cached.exists() && verifyCached(cached, cacheDir)) {
            return cached
        }

        cacheDir.mkdirs()
        download(cacheDir, cached)
        return cached
    }

    private fun verifyCached(binary: File, cacheDir: File): Boolean {
        val checksumFile = File(cacheDir, "checksums.txt")
        if (!checksumFile.exists()) return false
        val archiveName = archiveName()
        val expected    = checksumFile.readLines()
            .firstOrNull { it.contains(archiveName) }
            ?.split("\\s+".toRegex())
            ?.firstOrNull()
            ?: return false
        // We stored the archive checksum; verify the extracted binary via the same checksum file entry.
        // Re-verification here just ensures the file hasn't been tampered with post-extraction.
        return sha256(binary).equals(expected, ignoreCase = true).also { ok ->
            if (!ok) binary.delete()
        }
    }

    private fun download(cacheDir: File, target: File) {
        val baseUrl      = "https://github.com/GetQuantumDrive/Observer/releases/download/v$version"
        val archiveName  = archiveName()
        val checksumUrl  = "$baseUrl/checksums.txt"
        val archiveUrl   = "$baseUrl/$archiveName"

        val checksumFile = File(cacheDir, "checksums.txt")
        checksumFile.writeBytes(URL(checksumUrl).readBytes())

        val expected = checksumFile.readLines()
            .firstOrNull { it.contains(archiveName) }
            ?.split("\\s+".toRegex())
            ?.firstOrNull()
            ?: throw GradleException("No checksum found for $archiveName in checksums.txt")

        val tmpArchive = Files.createTempFile("observer-", archiveName.substringAfterLast('.')).toFile()
        try {
            tmpArchive.writeBytes(URL(archiveUrl).readBytes())
            val actual = sha256(tmpArchive)
            if (!actual.equals(expected, ignoreCase = true)) {
                throw GradleException(
                    "Checksum mismatch for $archiveName - expected $expected, got $actual. Aborting."
                )
            }
            extract(tmpArchive, target)
        } finally {
            tmpArchive.delete()
        }

        if (os() != "windows") target.setExecutable(true)
    }

    private fun extract(archive: File, target: File) {
        when {
            archive.name.endsWith(".zip") -> {
                ZipInputStream(archive.inputStream()).use { zip ->
                    var entry = zip.nextEntry
                    while (entry != null) {
                        if (!entry.isDirectory && (entry.name.endsWith("observer") || entry.name.endsWith("observer.exe"))) {
                            target.outputStream().use { zip.copyTo(it) }
                            return
                        }
                        entry = zip.nextEntry
                    }
                }
                throw GradleException("observer binary not found inside ${archive.name}")
            }
            archive.name.endsWith(".tar.gz") -> {
                // Use ProcessBuilder - avoids requiring an Apache Commons Compress dependency.
                val result = ProcessBuilder("tar", "xzf", archive.absolutePath, "--strip-components=0", "-C", target.parent)
                    .redirectErrorStream(true)
                    .start()
                    .waitFor()
                if (result != 0) throw GradleException("Failed to extract ${archive.name}")
                // tar places binary in parent dir; rename if needed.
                val extracted = File(target.parent, "observer")
                if (extracted.exists() && extracted != target) extracted.renameTo(target)
            }
            else -> throw GradleException("Unsupported archive format: ${archive.name}")
        }
    }

    private fun archiveName(): String {
        val ext = if (os() == "windows") "zip" else "tar.gz"
        return "observer-cli_${version}_${os()}_${arch()}.$ext"
    }

    private fun os(): String = when {
        System.getProperty("os.name").lowercase().contains("win")   -> "windows"
        System.getProperty("os.name").lowercase().contains("mac")   -> "darwin"
        else                                                          -> "linux"
    }

    private fun arch(): String = when (System.getProperty("os.arch").lowercase()) {
        "aarch64", "arm64" -> "arm64"
        else               -> "amd64"
    }

    private fun sha256(file: File): String {
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { stream ->
            val buf = ByteArray(8192)
            var n: Int
            while (stream.read(buf).also { n = it } != -1) digest.update(buf, 0, n)
        }
        return BigInteger(1, digest.digest()).toString(16).padStart(64, '0')
    }
}
