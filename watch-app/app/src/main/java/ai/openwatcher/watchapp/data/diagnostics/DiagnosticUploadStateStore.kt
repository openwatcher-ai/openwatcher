package ai.openwatcher.watchapp.data.diagnostics

import java.time.Instant
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import ai.openwatcher.watchapp.data.KeyValueStore

interface DiagnosticUploadStateStore {
    fun read(): DiagnosticUploadStatus
    fun write(status: DiagnosticUploadStatus)
}

class DiagnosticUploadPreferenceStore(
    private val store: KeyValueStore,
    private val json: Json = Json { ignoreUnknownKeys = true; explicitNulls = false },
) : DiagnosticUploadStateStore {
    override fun read(): DiagnosticUploadStatus {
        val raw = store.getString(KEY_DIAGNOSTIC_UPLOAD_STATE).orEmpty().trim()
        if (raw.isBlank()) {
            return DiagnosticUploadStatus()
        }
        return runCatching {
            json.decodeFromString(DiagnosticUploadStateDto.serializer(), raw).toDomain()
        }.getOrElse {
            DiagnosticUploadStatus()
        }
    }

    override fun write(status: DiagnosticUploadStatus) {
        store.putString(
            KEY_DIAGNOSTIC_UPLOAD_STATE,
            json.encodeToString(DiagnosticUploadStateDto.fromDomain(status)),
        )
    }

    companion object {
        private const val KEY_DIAGNOSTIC_UPLOAD_STATE = "diagnostic_upload_state"
    }
}

@Serializable
private data class DiagnosticUploadStateDto(
    val phase: String = DiagnosticUploadPhase.Idle.name,
    val pendingFileName: String? = null,
    val pendingCreatedAt: String? = null,
    val pendingSizeBytes: Long? = null,
    val packageSizeBytes: Long? = null,
    val bytesUploaded: Long = 0L,
    val totalBytes: Long? = null,
    val uploadSpeedBytesPerSecond: Long? = null,
    val lastDiagnosticId: String? = null,
    val lastReceivedAt: String? = null,
    val lastDiagnosticCreatedAt: String? = null,
    val lastErrorMessage: String? = null,
) {
    fun toDomain(): DiagnosticUploadStatus {
        val pending = if (
            pendingFileName.isNullOrBlank() ||
            pendingCreatedAt.isNullOrBlank() ||
            pendingSizeBytes == null
        ) {
            null
        } else {
            PendingDiagnosticPackage(
                fileName = pendingFileName,
                createdAt = Instant.parse(pendingCreatedAt),
                sizeBytes = pendingSizeBytes,
            )
        }
        val success = if (lastDiagnosticId.isNullOrBlank() || lastReceivedAt.isNullOrBlank()) {
            null
        } else {
            DiagnosticUploadSuccess(
                diagnosticId = lastDiagnosticId,
                receivedAt = Instant.parse(lastReceivedAt),
                diagnosticCreatedAt = lastDiagnosticCreatedAt
                    ?.takeIf { it.isNotBlank() }
                    ?.let(Instant::parse)
                    ?: Instant.parse(lastReceivedAt),
            )
        }
        return DiagnosticUploadStatus(
            phase = runCatching { DiagnosticUploadPhase.valueOf(phase) }.getOrDefault(DiagnosticUploadPhase.Idle),
            pendingPackage = pending,
            packageSizeBytes = packageSizeBytes,
            bytesUploaded = bytesUploaded,
            totalBytes = totalBytes,
            uploadSpeedBytesPerSecond = uploadSpeedBytesPerSecond,
            lastSuccess = success,
            lastErrorMessage = lastErrorMessage,
        )
    }

    companion object {
        fun fromDomain(status: DiagnosticUploadStatus): DiagnosticUploadStateDto {
            return DiagnosticUploadStateDto(
                phase = status.phase.name,
                pendingFileName = status.pendingPackage?.fileName,
                pendingCreatedAt = status.pendingPackage?.createdAt?.toString(),
                pendingSizeBytes = status.pendingPackage?.sizeBytes,
                packageSizeBytes = status.packageSizeBytes,
                bytesUploaded = status.bytesUploaded,
                totalBytes = status.totalBytes,
                uploadSpeedBytesPerSecond = status.uploadSpeedBytesPerSecond,
                lastDiagnosticId = status.lastSuccess?.diagnosticId,
                lastReceivedAt = status.lastSuccess?.receivedAt?.toString(),
                lastDiagnosticCreatedAt = status.lastSuccess?.diagnosticCreatedAt?.toString(),
                lastErrorMessage = status.lastErrorMessage,
            )
        }
    }
}
