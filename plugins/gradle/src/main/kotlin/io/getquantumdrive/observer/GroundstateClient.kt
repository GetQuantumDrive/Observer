package io.getquantumdrive.observer

import org.gradle.api.GradleException
import java.net.HttpURLConnection
import java.net.URL

object GroundstateClient {
    fun post(baseUrl: String, token: String?, body: ByteArray) {
        val url = URL("$baseUrl/api/reports")
        val conn = url.openConnection() as HttpURLConnection
        try {
            conn.requestMethod = "POST"
            conn.doOutput      = true
            conn.connectTimeout = 15_000
            conn.readTimeout    = 30_000
            conn.setRequestProperty("Content-Type", "application/json")
            if (!token.isNullOrBlank()) {
                conn.setRequestProperty("Authorization", "Bearer $token")
            }
            conn.outputStream.use { it.write(body) }
            val status = conn.responseCode
            if (status >= 400) {
                throw GradleException("Groundstate returned HTTP $status")
            }
        } finally {
            conn.disconnect()
        }
    }
}
