package ai.openwatcher.watchapp.data.diagnostics

import java.io.File
import java.time.Instant
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import java.util.UUID
import java.util.zip.GZIPOutputStream
import kotlin.math.max
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import ai.openwatcher.watchapp.data.DiagnosticUploadRequest
import ai.openwatcher.watchapp.data.DiagnosticUploadResult
import ai.openwatcher.watchapp.data.WatcherApi

class DiagnosticUploadManager(
    private val api: WatcherApi,
    private val eventStore: DiagnosticEventStore,
    private val eventLogger: DiagnosticEventLogger,
    private val snapshotCollector: DiagnosticSnapshotCollector,
    private val stateStore: DiagnosticUploadStateStore,
    private val pendingDirectory: File,
    private val deviceName: String,
    private val appVersion: String,
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
    private val clock: () -> Instant = { Instant.now() },
    private val monotonicNowMs: () -> Long = { System.nanoTime() / 1_000_000L },
) {
    private val scope = CoroutineScope(SupervisorJob() + ioDispatcher)
    private val stateLock = Any()
    private val uploadJobLock = Any()
    private var uploadJob: Job? = null
    private val _state = MutableStateFlow(loadInitialState())
    val state: StateFlow<DiagnosticUploadStatus> = _state.asStateFlow()

    fun requestUpload(token: String) {
        if (token.isBlank()) {
            return
        }
        synchronized(uploadJobLock) {
            if (uploadJob?.isActive == true) {
                return
            }
            uploadJob = scope.launch {
                try {
                    performUpload(token)
                } finally {
                    synchronized(uploadJobLock) {
                        uploadJob = null
                    }
                }
            }
        }
    }

    fun clearPendingPackage() {
        synchronized(uploadJobLock) {
            if (uploadJob?.isActive == true) {
                return
            }
        }
        scope.launch {
            clearPendingPackageInternal()
        }
    }

    suspend fun awaitIdleForTesting() {
        while (true) {
            val waitFor = synchronized(uploadJobLock) { uploadJob } ?: return
            waitFor.join()
        }
    }

    suspend fun uploadNowForTesting(token: String) {
        if (token.isBlank()) {
            return
        }
        performUpload(token)
    }

    suspend fun clearPendingPackageNowForTesting() {
        clearPendingPackageInternal()
    }

    fun close() {
        scope.cancel()
    }

    private suspend fun performUpload(token: String) {
        val pending = state.value.pendingPackage
        if (pending != null) {
            retryExistingPackage(token, pending)
        } else {
            createAndUploadPackage(token)
        }
    }

    private suspend fun createAndUploadPackage(token: String) {
        val traceId = eventLogger.newTraceId("diagnostic")
        eventLogger.log(
            event = "diagnostic_upload_requested",
            traceId = traceId,
            fields = mapOf("hours" to WINDOW_HOURS),
        )
        updateState {
            it.copy(
                phase = DiagnosticUploadPhase.PreparingPackage,
                bytesUploaded = 0L,
                totalBytes = null,
                uploadSpeedBytesPerSecond = null,
                lastErrorMessage = null,
            )
        }
        snapshotCollector.capture(traceId)
        val pending = preparePendingPackage(traceId)
        if (pending == null) {
            failUpload(
                traceId = traceId,
                pending = null,
                message = "诊断包生成失败",
            )
            return
        }
        uploadPendingPackage(
            token = token,
            pending = pending,
            traceId = traceId,
        )
    }

    private suspend fun retryExistingPackage(token: String, pending: PendingDiagnosticPackage) {
        val traceId = eventLogger.newTraceId("diagnostic-retry")
        eventLogger.log(
            event = "diagnostic_upload_retry_requested",
            traceId = traceId,
            fields = mapOf(
                "fileName" to pending.fileName,
                "compressedBytes" to pending.sizeBytes,
            ),
        )
        uploadPendingPackage(token, pending, traceId)
    }

    private suspend fun preparePendingPackage(traceId: String): PendingDiagnosticPackage? = withContext(ioDispatcher) {
        runCatching {
            if (!pendingDirectory.exists() && !pendingDirectory.mkdirs()) {
                return@withContext null
            }
            cleanupPendingFiles()
            val startedAt = clock()
            val outputFile = File(pendingDirectory, buildPackageFileName(startedAt))
            val lines = eventStore.readRecentLines(hours = WINDOW_HOURS, now = startedAt)
            GZIPOutputStream(outputFile.outputStream().buffered()).bufferedWriter(Charsets.UTF_8).use { writer ->
                lines.forEach { line ->
                    writer.append(line)
                    writer.newLine()
                }
            }
            val pending = PendingDiagnosticPackage(
                fileName = outputFile.name,
                createdAt = startedAt,
                sizeBytes = outputFile.length().coerceAtLeast(0L),
            )
            updateState {
                it.copy(
                    phase = DiagnosticUploadPhase.PreparingPackage,
                    pendingPackage = pending,
                    packageSizeBytes = pending.sizeBytes,
                    bytesUploaded = 0L,
                    totalBytes = pending.sizeBytes,
                    uploadSpeedBytesPerSecond = null,
                    lastErrorMessage = null,
                )
            }
            eventLogger.log(
                event = "diagnostic_package_created",
                traceId = traceId,
                fields = mapOf(
                    "fileName" to pending.fileName,
                    "compressedBytes" to pending.sizeBytes,
                    "hours" to WINDOW_HOURS,
                    "lineCount" to lines.size,
                ),
            )
            pending
        }.getOrNull()
    }

    private suspend fun uploadPendingPackage(
        token: String,
        pending: PendingDiagnosticPackage,
        traceId: String,
    ) {
        val file = File(pendingDirectory, pending.fileName)
        if (!file.isFile) {
            failUpload(traceId, pending = null, message = "待上传诊断包不存在")
            return
        }
        val gzipBytes = withContext(ioDispatcher) {
            runCatching { file.readBytes() }.getOrNull()
        }
        if (gzipBytes == null) {
            failUpload(traceId, pending = pending, message = "读取诊断包失败")
            return
        }
        val tracker = UploadSpeedTracker(monotonicNowMs)
        updateState {
            it.copy(
                phase = DiagnosticUploadPhase.Uploading,
                pendingPackage = pending.copy(sizeBytes = file.length()),
                packageSizeBytes = file.length(),
                bytesUploaded = 0L,
                totalBytes = max(file.length(), 0L),
                uploadSpeedBytesPerSecond = null,
                lastErrorMessage = null,
            )
        }
        eventLogger.log(
            event = "diagnostic_upload_started",
            traceId = traceId,
            fields = mapOf(
                "fileName" to pending.fileName,
                "compressedBytes" to file.length(),
                "hours" to WINDOW_HOURS,
            ),
        )
        val result = api.uploadDiagnostics(
            token = token,
            request = DiagnosticUploadRequest(
                gzipBytes = gzipBytes,
                deviceName = deviceName,
                appVersion = appVersion,
                startedAt = pending.createdAt,
                hours = WINDOW_HOURS,
            ),
            onProgress = { bytesUploaded, totalBytes ->
                maybePublishProgress(
                    pending = pending.copy(sizeBytes = file.length()),
                    bytesUploaded = bytesUploaded,
                    totalBytes = totalBytes,
                    speedTracker = tracker,
                )
            },
        )
        when (result) {
            is DiagnosticUploadResult.Success -> {
                eventLogger.log(
                    event = "diagnostic_upload_succeeded",
                    traceId = traceId,
                    fields = mapOf(
                        "diagnosticId" to result.diagnosticId,
                        "receivedAt" to result.receivedAt,
                        "compressedBytes" to file.length(),
                    ),
                )
                withContext(ioDispatcher) {
                    file.delete()
                    cleanupPendingFiles()
                }
                updateState {
                    it.copy(
                        phase = DiagnosticUploadPhase.Idle,
                        pendingPackage = null,
                        packageSizeBytes = null,
                        bytesUploaded = 0L,
                        totalBytes = null,
                        uploadSpeedBytesPerSecond = null,
                        lastSuccess = DiagnosticUploadSuccess(
                            diagnosticId = result.diagnosticId,
                            receivedAt = result.receivedAt,
                            diagnosticCreatedAt = pending.createdAt,
                        ),
                        lastErrorMessage = null,
                    )
                }
            }

            DiagnosticUploadResult.Unauthorized -> {
                failUpload(traceId, pending, "需要重新配对")
            }

            is DiagnosticUploadResult.HttpFailure -> {
                failUpload(traceId, pending, "上传失败 HTTP ${result.code}")
            }

            is DiagnosticUploadResult.NetworkFailure -> {
                failUpload(traceId, pending, result.message)
            }
        }
    }

    private fun maybePublishProgress(
        pending: PendingDiagnosticPackage,
        bytesUploaded: Long,
        totalBytes: Long,
        speedTracker: UploadSpeedTracker,
    ) {
        val sample = speedTracker.record(bytesUploaded, totalBytes)
        if (!sample.publish) {
            return
        }
        updateState {
            it.copy(
                phase = DiagnosticUploadPhase.Uploading,
                pendingPackage = pending,
                packageSizeBytes = totalBytes,
                bytesUploaded = sample.bytesUploaded,
                totalBytes = sample.totalBytes,
                uploadSpeedBytesPerSecond = sample.speedBytesPerSecond,
                lastErrorMessage = null,
            )
        }
    }

    private suspend fun failUpload(
        traceId: String,
        pending: PendingDiagnosticPackage?,
        message: String,
    ) {
        eventLogger.log(
            event = "diagnostic_upload_failed",
            level = DiagnosticLevel.Warn,
            traceId = traceId,
            fields = mapOf(
                "message" to message,
                "fileName" to pending?.fileName,
                "compressedBytes" to pending?.sizeBytes,
            ),
        )
        updateState {
            it.copy(
                phase = if (pending == null) DiagnosticUploadPhase.Idle else DiagnosticUploadPhase.Failed,
                pendingPackage = pending,
                packageSizeBytes = pending?.sizeBytes ?: it.packageSizeBytes,
                bytesUploaded = it.bytesUploaded,
                totalBytes = pending?.sizeBytes ?: it.totalBytes,
                uploadSpeedBytesPerSecond = null,
                lastErrorMessage = message,
            )
        }
    }

    private fun updateState(transform: (DiagnosticUploadStatus) -> DiagnosticUploadStatus) {
        synchronized(stateLock) {
            val next = transform(_state.value)
            _state.value = next
            stateStore.write(next)
        }
    }

    private suspend fun clearPendingPackageInternal() {
        val pending = state.value.pendingPackage
        if (pending == null) {
            return
        }
        withContext(ioDispatcher) {
            File(pendingDirectory, pending.fileName).delete()
            cleanupPendingFiles()
        }
        eventLogger.log(
            event = "diagnostic_pending_package_cleared",
            fields = mapOf(
                "fileName" to pending.fileName,
                "compressedBytes" to pending.sizeBytes,
            ),
        )
        updateState {
            it.copy(
                phase = DiagnosticUploadPhase.Idle,
                pendingPackage = null,
                packageSizeBytes = null,
                bytesUploaded = 0L,
                totalBytes = null,
                uploadSpeedBytesPerSecond = null,
                lastErrorMessage = null,
            )
        }
    }

    private fun loadInitialState(): DiagnosticUploadStatus {
        val persisted = stateStore.read()
        val pending = persisted.pendingPackage
            ?.let { candidate ->
                val file = File(pendingDirectory, candidate.fileName)
                if (!file.isFile) {
                    null
                } else {
                    candidate.copy(sizeBytes = file.length())
                }
            }
        return persisted.copy(
            phase = when {
                pending == null -> DiagnosticUploadPhase.Idle
                persisted.phase == DiagnosticUploadPhase.Uploading ||
                    persisted.phase == DiagnosticUploadPhase.PreparingPackage -> DiagnosticUploadPhase.Failed
                else -> persisted.phase
            },
            pendingPackage = pending,
            packageSizeBytes = pending?.sizeBytes ?: persisted.packageSizeBytes,
            totalBytes = pending?.sizeBytes ?: persisted.totalBytes,
            uploadSpeedBytesPerSecond = null,
        )
    }

    private fun cleanupPendingFiles(keepFileName: String? = null) {
        pendingDirectory
            .listFiles { file -> file.isFile && file.extension == "gz" }
            .orEmpty()
            .forEach { file ->
                if (keepFileName == null || file.name != keepFileName) {
                    file.delete()
                }
            }
    }

    private fun buildPackageFileName(startedAt: Instant): String {
        val timestamp = FILE_NAME_FORMATTER.format(startedAt.atZone(ZoneOffset.UTC))
        val shortId = UUID.randomUUID().toString().replace("-", "").take(8)
        return "diagnostic-$timestamp-$shortId.jsonl.gz"
    }

    private class UploadSpeedTracker(
        private val monotonicNowMs: () -> Long,
    ) {
        private val samples = ArrayDeque<Sample>()
        private var lastPublishedAtMs = 0L

        fun record(bytesUploaded: Long, totalBytes: Long): UploadSample {
            val now = monotonicNowMs()
            samples.addLast(Sample(now, bytesUploaded))
            while (samples.size > 1 && now - samples.first().timeMs > WINDOW_MS) {
                samples.removeFirst()
            }
            val earliest = samples.firstOrNull() ?: Sample(now, bytesUploaded)
            val elapsedMs = max(now - earliest.timeMs, 1L)
            val speed = if (samples.size <= 1) {
                null
            } else {
                ((bytesUploaded - earliest.bytesUploaded).coerceAtLeast(0L) * 1000L) / elapsedMs
            }
            val publish = bytesUploaded >= totalBytes || now - lastPublishedAtMs >= PUBLISH_INTERVAL_MS
            if (publish) {
                lastPublishedAtMs = now
            }
            return UploadSample(
                publish = publish,
                bytesUploaded = bytesUploaded,
                totalBytes = totalBytes,
                speedBytesPerSecond = speed,
            )
        }

        private data class Sample(
            val timeMs: Long,
            val bytesUploaded: Long,
        )

        companion object {
            private const val WINDOW_MS = 1_000L
            private const val PUBLISH_INTERVAL_MS = 250L
        }
    }

    private data class UploadSample(
        val publish: Boolean,
        val bytesUploaded: Long,
        val totalBytes: Long,
        val speedBytesPerSecond: Long?,
    )

    companion object {
        private const val WINDOW_HOURS = 24
        private val FILE_NAME_FORMATTER =
            DateTimeFormatter.ofPattern("yyyyMMdd'T'HHmmss'Z'")
    }
}
