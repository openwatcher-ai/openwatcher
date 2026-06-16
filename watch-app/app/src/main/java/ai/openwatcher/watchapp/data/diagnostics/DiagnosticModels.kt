package ai.openwatcher.watchapp.data.diagnostics

import java.time.Instant
import java.util.Locale

data class DiagnosticDeviceInfo(
    val manufacturer: String,
    val model: String,
    val sdkInt: Int,
    val screenWidthPx: Int,
    val screenHeightPx: Int,
    val densityDpi: Int,
    val fontScale: Float,
    val isRound: Boolean,
    val smallestWidthDp: Int,
)

data class DiagnosticAppInfo(
    val versionName: String,
    val versionCode: Int,
    val buildType: String,
)

data class DiagnosticNetworkInfo(
    val baseUrl: String,
    val hasPaired: Boolean,
)

enum class DiagnosticLevel {
    Info,
    Warn,
    Error,
    ;

    fun wireValue(): String = name.lowercase(Locale.US)
}

enum class DiagnosticUploadPhase {
    Idle,
    PreparingPackage,
    Uploading,
    Failed,
}

data class PendingDiagnosticPackage(
    val fileName: String,
    val createdAt: Instant,
    val sizeBytes: Long,
)

data class DiagnosticUploadSuccess(
    val diagnosticId: String,
    val receivedAt: Instant,
    val diagnosticCreatedAt: Instant,
)

data class DiagnosticUploadStatus(
    val phase: DiagnosticUploadPhase = DiagnosticUploadPhase.Idle,
    val pendingPackage: PendingDiagnosticPackage? = null,
    val packageSizeBytes: Long? = null,
    val bytesUploaded: Long = 0L,
    val totalBytes: Long? = null,
    val uploadSpeedBytesPerSecond: Long? = null,
    val lastSuccess: DiagnosticUploadSuccess? = null,
    val lastErrorMessage: String? = null,
) {
    val hasPendingPackage: Boolean
        get() = pendingPackage != null
}
