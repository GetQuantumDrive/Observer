package io.getquantumdrive.observer.model

import com.google.gson.annotations.SerializedName

data class ScanReport(
    val id: String,
    val source: String,
    val ref: String,
    val sha: String,
    @SerializedName("scanned_at")    val scannedAt: String,
    @SerializedName("duration_ms")   val durationMs: Long,
    @SerializedName("files_scanned") val filesScanned: Int,
    @SerializedName("rules_applied") val rulesApplied: Int,
    val findings: List<Finding>,
    @SerializedName("risk_summary")  val riskSummary: RiskSummary,
    val compliance: ComplianceStatus,
)

data class Finding(
    @SerializedName("rule_id")   val ruleId: String,
    val file: String,
    val line: Int,
    val algorithm: String,
    val severity: String,
    val confidence: Int,
    val snippet: String,
    val message: String,
    val migration: String,
)

data class RiskSummary(
    val critical: Int,
    val high: Int,
    val medium: Int,
    val low: Int,
    val safe: Int,
    val total: Int,
)

data class ComplianceStatus(
    @SerializedName("nist_fips_203") val nistFips203: String,
    @SerializedName("nist_fips_204") val nistFips204: String,
    val nis2: String,
    val dora: String,
)
