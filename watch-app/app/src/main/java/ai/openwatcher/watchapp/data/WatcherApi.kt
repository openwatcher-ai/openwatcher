package ai.openwatcher.watchapp.data

import java.io.IOException
import java.time.Instant
import java.util.Locale
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okhttp3.Call
import okhttp3.Callback
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Request
import okhttp3.RequestBody
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import okio.BufferedSink
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticEventLogger
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticLevel
import ai.openwatcher.watchapp.data.diagnostics.NoOpDiagnosticEventLogger

interface WatcherApi {
    suspend fun fetchStatus(token: String): StatusFetchResult
    suspend fun checkHealth(): HealthCheckResult
    suspend fun uploadScreenshot(token: String, request: ScreenshotUploadRequest): ScreenshotUploadResult
    suspend fun uploadDiagnostics(
        token: String,
        request: DiagnosticUploadRequest,
        onProgress: (bytesUploaded: Long, totalBytes: Long) -> Unit = { _, _ -> },
    ): DiagnosticUploadResult
    fun streamStatus(token: String, includeDailyTrend30d: Boolean = false): Flow<StatusStreamEvent>
    fun streamSessionAgentMessages(token: String, threadId: String, includeMessages: Boolean): Flow<SessionStreamEvent>
    fun streamSessionWindow(token: String, limit: Int, preferredOrder: List<String>): Flow<SessionWindowStreamEvent>
    suspend fun reportSessionStreamClientEvent(token: String, report: SessionStreamClientEventReport)
}

data class ScreenshotUploadRequest(
    val pngBytes: ByteArray,
    val deviceName: String,
    val appVersion: String,
)

data class DiagnosticUploadRequest(
    val gzipBytes: ByteArray,
    val deviceName: String,
    val appVersion: String,
    val startedAt: Instant,
    val hours: Int,
)

class HttpWatcherApi(
    private val baseUrl: String,
    private val callFactory: Call.Factory,
    private val streamCallFactory: Call.Factory = callFactory,
    private val json: Json = Json { explicitNulls = false },
    private val parser: WatcherJsonParser = WatcherJsonParser(),
    private val diagnosticEventLogger: DiagnosticEventLogger = NoOpDiagnosticEventLogger,
) : WatcherApi {
    override suspend fun fetchStatus(token: String): StatusFetchResult = withContext(Dispatchers.IO) {
        val request = Request.Builder()
            .url(baseUrl.toHttpUrl().newBuilder().addPathSegments("api/status").build())
            .header("X-OpenWatcher-Token", token)
            .get()
            .build()
        val traceId = diagnosticEventLogger.newTraceId("status")
        val startedAt = Instant.now()
        logNetworkRequest(
            traceId = traceId,
            targetName = "status_snapshot",
            path = "/api/status",
            method = "GET",
        )

        try {
            callFactory.newCall(request).execute().use { response ->
                logNetworkResult(
                    traceId = traceId,
                    targetName = "status_snapshot",
                    path = "/api/status",
                    method = "GET",
                    startedAt = startedAt,
                    statusCode = response.code,
                )
                return@withContext when {
                    response.code == 401 -> StatusFetchResult.Unauthorized
                    !response.isSuccessful -> StatusFetchResult.HttpFailure(
                        code = response.code,
                        message = "HTTP ${response.code}",
                    )
                    else -> {
                        val body = response.body?.string().orEmpty()
                        StatusFetchResult.Success(parser.parseStatus(body))
                    }
                }
            }
        } catch (error: SerializationException) {
            logNetworkFailure(
                traceId = traceId,
                targetName = "status_snapshot",
                path = "/api/status",
                method = "GET",
                startedAt = startedAt,
                error = error,
            )
            StatusFetchResult.ParseFailure("返回数据无法解析")
        } catch (error: IOException) {
            logNetworkFailure(
                traceId = traceId,
                targetName = "status_snapshot",
                path = "/api/status",
                method = "GET",
                startedAt = startedAt,
                error = error,
            )
            StatusFetchResult.NetworkFailure("服务连接失败")
        }
    }

    override suspend fun checkHealth(): HealthCheckResult = withContext(Dispatchers.IO) {
        val request = Request.Builder()
            .url(baseUrl.toHttpUrl().newBuilder().addPathSegment("healthz").build())
            .get()
            .build()
        val traceId = diagnosticEventLogger.newTraceId("health")
        val startedAt = Instant.now()
        logNetworkRequest(
            traceId = traceId,
            targetName = "health_check",
            path = "/healthz",
            method = "GET",
        )
        try {
            callFactory.newCall(request).execute().use { response ->
                logNetworkResult(
                    traceId = traceId,
                    targetName = "health_check",
                    path = "/healthz",
                    method = "GET",
                    startedAt = startedAt,
                    statusCode = response.code,
                )
                if (!response.isSuccessful) {
                    return@withContext HealthCheckResult.Offline("健康检查失败 HTTP ${response.code}")
                }
                val body = response.body?.string().orEmpty()
                return@withContext if (parser.parseHealth(body)) {
                    HealthCheckResult.Online
                } else {
                    HealthCheckResult.Offline("服务返回异常")
                }
            }
        } catch (error: SerializationException) {
            logNetworkFailure(
                traceId = traceId,
                targetName = "health_check",
                path = "/healthz",
                method = "GET",
                startedAt = startedAt,
                error = error,
            )
            HealthCheckResult.Offline("健康检查解析失败")
        } catch (error: IOException) {
            logNetworkFailure(
                traceId = traceId,
                targetName = "health_check",
                path = "/healthz",
                method = "GET",
                startedAt = startedAt,
                error = error,
            )
            HealthCheckResult.Offline("服务不可达")
        }
    }

    override suspend fun uploadScreenshot(
        token: String,
        request: ScreenshotUploadRequest,
    ): ScreenshotUploadResult = withContext(Dispatchers.IO) {
        val requestBody = request.pngBytes.toRequestBody("image/png".toMediaType())
        val httpRequest = Request.Builder()
            .url(
                baseUrl.toHttpUrl().newBuilder()
                    .addPathSegment("api")
                    .addPathSegment("screenshots")
                    .build(),
            )
            .header("X-OpenWatcher-Token", token)
            .header("X-OpenWatcher-Device-Name", request.deviceName)
            .header("X-OpenWatcher-App-Version", request.appVersion)
            .post(requestBody)
            .build()
        val traceId = diagnosticEventLogger.newTraceId("screenshot")
        val startedAt = Instant.now()
        logNetworkRequest(
            traceId = traceId,
            targetName = "screenshot_upload",
            path = "/api/screenshots",
            method = "POST",
            bytesSent = request.pngBytes.size.toLong(),
        )
        try {
            callFactory.newCall(httpRequest).execute().use { response ->
                logNetworkResult(
                    traceId = traceId,
                    targetName = "screenshot_upload",
                    path = "/api/screenshots",
                    method = "POST",
                    startedAt = startedAt,
                    statusCode = response.code,
                    bytesSent = request.pngBytes.size.toLong(),
                )
                return@withContext when {
                    response.code == 401 -> ScreenshotUploadResult.Unauthorized
                    !response.isSuccessful -> ScreenshotUploadResult.HttpFailure(
                        code = response.code,
                        message = "HTTP ${response.code}",
                    )
                    else -> ScreenshotUploadResult.Success(
                        filename = runCatching {
                            parser.parseScreenshotUploadFilename(response.body?.string().orEmpty())
                        }.getOrElse { "" },
                    )
                }
            }
        } catch (error: IOException) {
            logNetworkFailure(
                traceId = traceId,
                targetName = "screenshot_upload",
                path = "/api/screenshots",
                method = "POST",
                startedAt = startedAt,
                bytesSent = request.pngBytes.size.toLong(),
                error = error,
            )
            ScreenshotUploadResult.NetworkFailure("服务连接失败")
        }
    }

    override suspend fun uploadDiagnostics(
        token: String,
        request: DiagnosticUploadRequest,
        onProgress: (bytesUploaded: Long, totalBytes: Long) -> Unit,
    ): DiagnosticUploadResult = withContext(Dispatchers.IO) {
        val requestBody = ProgressByteArrayRequestBody(
            bytes = request.gzipBytes,
            mediaType = "application/gzip".toMediaType(),
            onProgress = onProgress,
        )
        val httpRequest = Request.Builder()
            .url(
                baseUrl.toHttpUrl().newBuilder()
                    .addPathSegment("api")
                    .addPathSegment("diagnostics")
                    .build(),
            )
            .header("X-OpenWatcher-Token", token)
            .header("X-OpenWatcher-Device-Name", request.deviceName)
            .header("X-OpenWatcher-App-Version", request.appVersion)
            .header("X-OpenWatcher-Diagnostic-Started-At", request.startedAt.toString())
            .header("X-OpenWatcher-Diagnostic-Hours", request.hours.toString())
            .post(requestBody)
            .build()
        val traceId = diagnosticEventLogger.newTraceId("diagnostic-upload")
        val startedAt = Instant.now()
        logNetworkRequest(
            traceId = traceId,
            targetName = "diagnostic_upload",
            path = "/api/diagnostics",
            method = "POST",
            bytesSent = request.gzipBytes.size.toLong(),
        )
        try {
            callFactory.newCall(httpRequest).execute().use { response ->
                logNetworkResult(
                    traceId = traceId,
                    targetName = "diagnostic_upload",
                    path = "/api/diagnostics",
                    method = "POST",
                    startedAt = startedAt,
                    statusCode = response.code,
                    bytesSent = request.gzipBytes.size.toLong(),
                )
                return@withContext when {
                    response.code == 401 -> DiagnosticUploadResult.Unauthorized
                    !response.isSuccessful -> DiagnosticUploadResult.HttpFailure(
                        code = response.code,
                        message = "HTTP ${response.code}",
                    )
                    else -> runCatching {
                        json.decodeFromString<DiagnosticUploadResponseDto>(response.body?.string().orEmpty())
                    }.map { dto ->
                        DiagnosticUploadResult.Success(
                            diagnosticId = dto.diagnosticId.trim(),
                            receivedAt = Instant.parse(dto.receivedAt),
                        )
                    }.getOrElse {
                        DiagnosticUploadResult.HttpFailure(
                            code = response.code,
                            message = "返回数据无法解析",
                        )
                    }
                }
            }
        } catch (error: IOException) {
            logNetworkFailure(
                traceId = traceId,
                targetName = "diagnostic_upload",
                path = "/api/diagnostics",
                method = "POST",
                startedAt = startedAt,
                bytesSent = request.gzipBytes.size.toLong(),
                error = error,
            )
            DiagnosticUploadResult.NetworkFailure("服务连接失败")
        } catch (error: SerializationException) {
            logNetworkFailure(
                traceId = traceId,
                targetName = "diagnostic_upload",
                path = "/api/diagnostics",
                method = "POST",
                startedAt = startedAt,
                bytesSent = request.gzipBytes.size.toLong(),
                error = error,
            )
            DiagnosticUploadResult.HttpFailure(code = 0, message = "返回数据无法解析")
        } catch (error: IllegalArgumentException) {
            logNetworkFailure(
                traceId = traceId,
                targetName = "diagnostic_upload",
                path = "/api/diagnostics",
                method = "POST",
                startedAt = startedAt,
                bytesSent = request.gzipBytes.size.toLong(),
                error = error,
            )
            DiagnosticUploadResult.HttpFailure(code = 0, message = "返回数据无法解析")
        }
    }

    override fun streamStatus(token: String, includeDailyTrend30d: Boolean): Flow<StatusStreamEvent> = callbackFlow {
        val urlBuilder = baseUrl.toHttpUrl().newBuilder()
            .addPathSegment("api")
            .addPathSegment("status")
            .addPathSegment("stream")
        if (includeDailyTrend30d) {
            urlBuilder.addQueryParameter("includeDailyTrend30d", "1")
        }
        val request = Request.Builder()
            .url(urlBuilder.build())
            .header("X-OpenWatcher-Token", token)
            .header("Accept", "text/event-stream")
            .get()
            .build()
        val traceId = diagnosticEventLogger.newTraceId("status-stream")
        launch {
            diagnosticEventLogger.log(
                event = "stream_state",
                traceId = traceId,
                fields = mapOf(
                    "target" to mapOf(
                        "name" to "status_stream",
                        "path" to "/api/status/stream",
                        "method" to "GET",
                    ),
                    "state" to mapOf(
                        "action" to "connect_requested",
                        "includeDailyTrend30d" to includeDailyTrend30d,
                    ),
                ),
            )
        }
        val call = streamCallFactory.newCall(request)
        call.enqueue(
            object : Callback {
                override fun onFailure(call: Call, error: IOException) {
                    if (!call.isCanceled()) {
                        this@callbackFlow.launch {
                            diagnosticEventLogger.log(
                                event = "stream_state",
                                level = DiagnosticLevel.Warn,
                                traceId = traceId,
                                fields = mapOf(
                                    "target" to mapOf(
                                        "name" to "status_stream",
                                        "path" to "/api/status/stream",
                                        "method" to "GET",
                                    ),
                                    "state" to mapOf(
                                        "action" to "connect_failed",
                                        "errorType" to error::class.java.simpleName,
                                        "message" to error.message,
                                    ),
                                ),
                            )
                        }
                        trySend(error.asStatusStreamFailure())
                    }
                    close()
                }

                override fun onResponse(call: Call, response: Response) {
                    response.use {
                        readStatusStreamResponse(call, it, traceId)
                    }
                }
            },
        )
        awaitClose { call.cancel() }
    }

    private fun kotlinx.coroutines.channels.ProducerScope<StatusStreamEvent>.readStatusStreamResponse(
        call: Call,
        response: Response,
        traceId: String,
    ) {
        try {
            when {
                response.code == 401 -> {
                    launch {
                        diagnosticEventLogger.log(
                            event = "stream_state",
                            level = DiagnosticLevel.Warn,
                            traceId = traceId,
                            fields = mapOf(
                                "target" to mapOf(
                                    "name" to "status_stream",
                                    "path" to "/api/status/stream",
                                    "method" to "GET",
                                ),
                                "state" to mapOf(
                                    "action" to "unauthorized",
                                    "statusCode" to response.code,
                                ),
                            ),
                        )
                    }
                    trySend(
                        StatusStreamEvent.Failure(
                            message = "需要重新配对",
                            reason = StatusStreamFailureReason.Unauthorized,
                            retryable = false,
                            terminal = true,
                            statusCode = response.code,
                        ),
                    )
                    close()
                    return
                }
                !response.isSuccessful -> {
                    launch {
                        diagnosticEventLogger.log(
                            event = "stream_state",
                            level = DiagnosticLevel.Warn,
                            traceId = traceId,
                            fields = mapOf(
                                "target" to mapOf(
                                    "name" to "status_stream",
                                    "path" to "/api/status/stream",
                                    "method" to "GET",
                                ),
                                "state" to mapOf(
                                    "action" to "http_failure",
                                    "statusCode" to response.code,
                                ),
                            ),
                        )
                    }
                    trySend(statusHttpFailure(response.code))
                    close()
                    return
                }
            }
            launch {
                diagnosticEventLogger.log(
                    event = "stream_state",
                    traceId = traceId,
                    fields = mapOf(
                        "target" to mapOf(
                            "name" to "status_stream",
                            "path" to "/api/status/stream",
                            "method" to "GET",
                        ),
                        "state" to mapOf(
                            "action" to "connected",
                            "statusCode" to response.code,
                        ),
                    ),
                )
            }
            val body = response.body ?: run {
                trySend(
                    StatusStreamEvent.Failure(
                        message = "状态流连接失败",
                        reason = StatusStreamFailureReason.StreamClosed,
                        retryable = true,
                        terminal = true,
                        detail = "empty_body",
                    ),
                )
                close()
                return
            }
            val parser = SessionSseParser()
            body.charStream().buffered().use { reader ->
                while (!call.isCanceled()) {
                    val line = reader.readLine() ?: break
                    parser.accept(line)?.let { event ->
                        mapStatusSseEvent(event)?.let { trySend(it) }
                    }
                }
            }
            if (!call.isCanceled()) {
                parser.finish()?.let { event ->
                    mapStatusSseEvent(event)?.let { trySend(it) }
                }
                launch {
                    diagnosticEventLogger.log(
                        event = "stream_state",
                        level = DiagnosticLevel.Warn,
                        traceId = traceId,
                        fields = mapOf(
                            "target" to mapOf(
                                "name" to "status_stream",
                                "path" to "/api/status/stream",
                                "method" to "GET",
                            ),
                            "state" to mapOf(
                                "action" to "stream_closed",
                                "detail" to "eof",
                            ),
                        ),
                    )
                }
                trySend(
                    StatusStreamEvent.Failure(
                        message = "状态流连接失败",
                        reason = StatusStreamFailureReason.StreamClosed,
                        retryable = true,
                        terminal = true,
                        detail = "eof",
                    ),
                )
            }
            close()
        } catch (error: IOException) {
            if (!call.isCanceled()) {
                launch {
                    diagnosticEventLogger.log(
                        event = "stream_state",
                        level = DiagnosticLevel.Warn,
                        traceId = traceId,
                        fields = mapOf(
                            "target" to mapOf(
                                "name" to "status_stream",
                                "path" to "/api/status/stream",
                                "method" to "GET",
                            ),
                            "state" to mapOf(
                                "action" to "read_failed",
                                "errorType" to error::class.java.simpleName,
                                "message" to error.message,
                            ),
                        ),
                    )
                }
                trySend(error.asStatusStreamFailure())
            }
            close()
        }
    }

    private fun mapStatusSseEvent(event: SseEvent): StatusStreamEvent? {
        val eventName = event.eventName.orEmpty()
        return when (eventName) {
            "heartbeat" -> StatusStreamEvent.Heartbeat
            "error" -> StatusStreamEvent.Failure(
                message = runCatching { parser.parseStatusStreamError(event.data) }
                    .getOrElse { "状态流连接失败" },
                reason = StatusStreamFailureReason.ServerError,
                retryable = true,
                terminal = true,
            )
            "status_snapshot" -> runCatching {
                StatusStreamEvent.Snapshot(parser.parseStatusStreamSnapshot(event.data))
            }.getOrElse { statusParseFailure(it) }
            "status_quota" -> runCatching {
                val update = parser.parseStatusQuotaUpdate(event.data)
                StatusStreamEvent.Quota(update.observedAt, update.quota)
            }.getOrElse { statusParseFailure(it) }
            "status_heatmap24h" -> runCatching {
                val update = parser.parseStatusHeatmapUpdate(event.data)
                StatusStreamEvent.Heatmap24h(update.observedAt, update.heatmap24h, update.heatmap7d, update.dailyUsage)
            }.getOrElse { statusParseFailure(it) }
            "status_sessions" -> runCatching {
                val update = parser.parseStatusSessionsUpdate(event.data)
                StatusStreamEvent.Sessions(update.observedAt, update.sessions)
            }.getOrElse { statusParseFailure(it) }
            "status_errors" -> runCatching {
                val update = parser.parseStatusErrorsUpdate(event.data)
                StatusStreamEvent.Errors(update.observedAt, update.errors)
            }.getOrElse { statusParseFailure(it) }
            else -> null
        }
    }

    override fun streamSessionAgentMessages(
        token: String,
        threadId: String,
        includeMessages: Boolean,
    ): Flow<SessionStreamEvent> = callbackFlow {
        val sessionStreamPath = "/api/sessions/$threadId/stream"
        val request = Request.Builder()
            .url(
                baseUrl.toHttpUrl().newBuilder()
                    .addPathSegment("api")
                    .addPathSegment("sessions")
                    .addPathSegment(threadId)
                    .addPathSegment("stream")
                    .addQueryParameter("includeMessages", if (includeMessages) "1" else "0")
                    .build(),
            )
            .header("X-OpenWatcher-Token", token)
            .header("Accept", "text/event-stream")
            .get()
            .build()
        val traceId = diagnosticEventLogger.newTraceId("session-stream")
        launch {
            diagnosticEventLogger.log(
                event = "stream_state",
                traceId = traceId,
                fields = mapOf(
                    "target" to mapOf(
                        "name" to "session_stream",
                        "path" to sessionStreamPath,
                        "method" to "GET",
                    ),
                    "state" to mapOf(
                        "action" to "connect_requested",
                        "includeMessages" to includeMessages,
                    ),
                ),
            )
        }
        val call = streamCallFactory.newCall(request)
        call.enqueue(
            object : Callback {
                override fun onFailure(call: Call, error: IOException) {
                    this@callbackFlow.launch {
                        if (!call.isCanceled()) {
                            diagnosticEventLogger.log(
                                event = "stream_state",
                                level = DiagnosticLevel.Warn,
                                traceId = traceId,
                                fields = mapOf(
                                    "target" to mapOf(
                                        "name" to "session_stream",
                                        "path" to sessionStreamPath,
                                        "method" to "GET",
                                    ),
                                    "state" to mapOf(
                                        "action" to "connect_failed",
                                        "errorType" to error::class.java.simpleName,
                                        "message" to error.message,
                                    ),
                                ),
                            )
                            trySend(error.asRetryableFailure())
                        }
                        close()
                    }
                }

                override fun onResponse(call: Call, response: Response) {
                    this@callbackFlow.launch {
                        response.use {
                            readSessionStreamResponse(
                                call = call,
                                response = it,
                                traceId = traceId,
                                sessionStreamPath = sessionStreamPath,
                            )
                        }
                    }
                }
            },
        )
        awaitClose { call.cancel() }
    }

    override fun streamSessionWindow(
        token: String,
        limit: Int,
        preferredOrder: List<String>,
    ): Flow<SessionWindowStreamEvent> = callbackFlow {
        val sessionWindowPath = "/api/sessions/stream"
        val request = Request.Builder()
            .url(
                baseUrl.toHttpUrl().newBuilder()
                    .addPathSegment("api")
                    .addPathSegment("sessions")
                    .addPathSegment("stream")
                    .addQueryParameter("limit", limit.coerceAtLeast(1).toString())
                    .apply {
                        if (preferredOrder.isNotEmpty()) {
                            addQueryParameter("preferredOrder", preferredOrder.joinToString(","))
                        }
                    }
                    .build(),
            )
            .header("X-OpenWatcher-Token", token)
            .header("Accept", "text/event-stream")
            .get()
            .build()
        val traceId = diagnosticEventLogger.newTraceId("session-window-stream")
        launch {
            diagnosticEventLogger.log(
                event = "stream_state",
                traceId = traceId,
                fields = mapOf(
                    "target" to mapOf(
                        "name" to "session_window_stream",
                        "path" to sessionWindowPath,
                        "method" to "GET",
                    ),
                    "state" to mapOf(
                        "action" to "connect_requested",
                        "limit" to limit.coerceAtLeast(1),
                        "preferredOrderCount" to preferredOrder.size,
                    ),
                ),
            )
        }
        val call = streamCallFactory.newCall(request)
        call.enqueue(
            object : Callback {
                override fun onFailure(call: Call, error: IOException) {
                    this@callbackFlow.launch {
                        if (!call.isCanceled()) {
                            diagnosticEventLogger.log(
                                event = "stream_state",
                                level = DiagnosticLevel.Warn,
                                traceId = traceId,
                                fields = mapOf(
                                    "target" to mapOf(
                                        "name" to "session_window_stream",
                                        "path" to sessionWindowPath,
                                        "method" to "GET",
                                    ),
                                    "state" to mapOf(
                                        "action" to "connect_failed",
                                        "errorType" to error::class.java.simpleName,
                                        "message" to error.message,
                                    ),
                                ),
                            )
                            trySend(error.asRetryableWindowFailure())
                        }
                        close()
                    }
                }

                override fun onResponse(call: Call, response: Response) {
                    this@callbackFlow.launch {
                        response.use {
                            readSessionWindowStreamResponse(
                                call = call,
                                response = it,
                                traceId = traceId,
                                sessionWindowPath = sessionWindowPath,
                            )
                        }
                    }
                }
            },
        )
        awaitClose { call.cancel() }
    }

    private suspend fun kotlinx.coroutines.channels.ProducerScope<SessionStreamEvent>.readSessionStreamResponse(
        call: Call,
        response: Response,
        traceId: String,
        sessionStreamPath: String,
    ) {
        try {
            when {
                response.code == 401 -> {
                    diagnosticEventLogger.log(
                        event = "stream_state",
                        level = DiagnosticLevel.Warn,
                        traceId = traceId,
                        fields = mapOf(
                            "target" to mapOf(
                                "name" to "session_stream",
                                "path" to sessionStreamPath,
                                "method" to "GET",
                            ),
                            "state" to mapOf(
                                "action" to "unauthorized",
                                "statusCode" to response.code,
                            ),
                        ),
                    )
                    trySend(
                        SessionStreamEvent.Failure(
                            message = "需要重新配对",
                            reason = SessionStreamFailureReason.Unauthorized,
                            retryable = false,
                            terminal = true,
                            statusCode = response.code,
                        ),
                    )
                    close()
                    return
                }
                !response.isSuccessful -> {
                    diagnosticEventLogger.log(
                        event = "stream_state",
                        level = DiagnosticLevel.Warn,
                        traceId = traceId,
                        fields = mapOf(
                            "target" to mapOf(
                                "name" to "session_stream",
                                "path" to sessionStreamPath,
                                "method" to "GET",
                            ),
                            "state" to mapOf(
                                "action" to "http_failure",
                                "statusCode" to response.code,
                            ),
                        ),
                    )
                    trySend(httpFailure(response.code))
                    close()
                    return
                }
            }
            diagnosticEventLogger.log(
                event = "stream_state",
                traceId = traceId,
                fields = mapOf(
                    "target" to mapOf(
                        "name" to "session_stream",
                        "path" to sessionStreamPath,
                        "method" to "GET",
                    ),
                    "state" to mapOf(
                        "action" to "connected",
                        "statusCode" to response.code,
                    ),
                ),
            )
            val body = response.body ?: run {
                trySend(
                    SessionStreamEvent.Failure(
                        message = "会话流连接失败",
                        reason = SessionStreamFailureReason.StreamClosed,
                        retryable = true,
                        terminal = true,
                        detail = "empty_body",
                    ),
                )
                close()
                return
            }
            val parser = SessionSseParser()
            body.charStream().buffered().use { reader ->
                while (!call.isCanceled()) {
                    val line = reader.readLine() ?: break
                    parser.accept(line)?.let { event ->
                        mapSseEvent(event)?.let { trySend(it) }
                    }
                }
            }
            if (!call.isCanceled()) {
                parser.finish()?.let { event ->
                    mapSseEvent(event)?.let { trySend(it) }
                }
                diagnosticEventLogger.log(
                    event = "stream_state",
                    level = DiagnosticLevel.Warn,
                    traceId = traceId,
                    fields = mapOf(
                        "target" to mapOf(
                            "name" to "session_stream",
                            "path" to sessionStreamPath,
                            "method" to "GET",
                        ),
                        "state" to mapOf(
                            "action" to "stream_closed",
                            "detail" to "eof",
                        ),
                    ),
                )
                trySend(
                    SessionStreamEvent.Failure(
                        message = "会话流连接失败",
                        reason = SessionStreamFailureReason.StreamClosed,
                        retryable = true,
                        terminal = true,
                        detail = "eof",
                    ),
                )
            }
            close()
        } catch (error: IOException) {
            if (!call.isCanceled()) {
                diagnosticEventLogger.log(
                    event = "stream_state",
                    level = DiagnosticLevel.Warn,
                    traceId = traceId,
                    fields = mapOf(
                        "target" to mapOf(
                            "name" to "session_stream",
                            "path" to sessionStreamPath,
                            "method" to "GET",
                        ),
                        "state" to mapOf(
                            "action" to "read_failed",
                            "errorType" to error::class.java.simpleName,
                            "message" to error.message,
                        ),
                    ),
                )
                trySend(error.asRetryableFailure())
            }
            close()
        }
    }

    private suspend fun kotlinx.coroutines.channels.ProducerScope<SessionWindowStreamEvent>.readSessionWindowStreamResponse(
        call: Call,
        response: Response,
        traceId: String,
        sessionWindowPath: String,
    ) {
        try {
            when {
                response.code == 401 -> {
                    diagnosticEventLogger.log(
                        event = "stream_state",
                        level = DiagnosticLevel.Warn,
                        traceId = traceId,
                        fields = mapOf(
                            "target" to mapOf(
                                "name" to "session_window_stream",
                                "path" to sessionWindowPath,
                                "method" to "GET",
                            ),
                            "state" to mapOf(
                                "action" to "unauthorized",
                                "statusCode" to response.code,
                            ),
                        ),
                    )
                    trySend(
                        SessionWindowStreamEvent.Failure(
                            message = "需要重新配对",
                            reason = SessionStreamFailureReason.Unauthorized,
                            retryable = false,
                            terminal = true,
                            statusCode = response.code,
                        ),
                    )
                    close()
                    return
                }
                !response.isSuccessful -> {
                    diagnosticEventLogger.log(
                        event = "stream_state",
                        level = DiagnosticLevel.Warn,
                        traceId = traceId,
                        fields = mapOf(
                            "target" to mapOf(
                                "name" to "session_window_stream",
                                "path" to sessionWindowPath,
                                "method" to "GET",
                            ),
                            "state" to mapOf(
                                "action" to "http_failure",
                                "statusCode" to response.code,
                            ),
                        ),
                    )
                    trySend(windowHttpFailure(response.code))
                    close()
                    return
                }
            }
            diagnosticEventLogger.log(
                event = "stream_state",
                traceId = traceId,
                fields = mapOf(
                    "target" to mapOf(
                        "name" to "session_window_stream",
                        "path" to sessionWindowPath,
                        "method" to "GET",
                    ),
                    "state" to mapOf(
                        "action" to "connected",
                        "statusCode" to response.code,
                    ),
                ),
            )
            val body = response.body ?: run {
                trySend(
                    SessionWindowStreamEvent.Failure(
                        message = "会话窗口流连接失败",
                        reason = SessionStreamFailureReason.StreamClosed,
                        retryable = true,
                        terminal = true,
                        detail = "empty_body",
                    ),
                )
                close()
                return
            }
            val parser = SessionSseParser()
            body.charStream().buffered().use { reader ->
                while (!call.isCanceled()) {
                    val line = reader.readLine() ?: break
                    parser.accept(line)?.let { event ->
                        mapSessionWindowSseEvent(event)?.let { trySend(it) }
                    }
                }
            }
            if (!call.isCanceled()) {
                parser.finish()?.let { event ->
                    mapSessionWindowSseEvent(event)?.let { trySend(it) }
                }
                diagnosticEventLogger.log(
                    event = "stream_state",
                    level = DiagnosticLevel.Warn,
                    traceId = traceId,
                    fields = mapOf(
                        "target" to mapOf(
                            "name" to "session_window_stream",
                            "path" to sessionWindowPath,
                            "method" to "GET",
                        ),
                        "state" to mapOf(
                            "action" to "stream_closed",
                            "detail" to "eof",
                        ),
                    ),
                )
                trySend(
                    SessionWindowStreamEvent.Failure(
                        message = "会话窗口流连接失败",
                        reason = SessionStreamFailureReason.StreamClosed,
                        retryable = true,
                        terminal = true,
                        detail = "eof",
                    ),
                )
            }
            close()
        } catch (error: IOException) {
            if (!call.isCanceled()) {
                diagnosticEventLogger.log(
                    event = "stream_state",
                    level = DiagnosticLevel.Warn,
                    traceId = traceId,
                    fields = mapOf(
                        "target" to mapOf(
                            "name" to "session_window_stream",
                            "path" to sessionWindowPath,
                            "method" to "GET",
                        ),
                        "state" to mapOf(
                            "action" to "read_failed",
                            "errorType" to error::class.java.simpleName,
                            "message" to error.message,
                        ),
                    ),
                )
                trySend(error.asRetryableWindowFailure())
            }
            close()
        }
    }

    private fun mapSseEvent(event: SseEvent): SessionStreamEvent? {
        val eventName = event.eventName.orEmpty()
        return when {
            eventName == "heartbeat" -> SessionStreamEvent.Heartbeat
            eventName == "error" -> SessionStreamEvent.Failure(
                message = runCatching { parser.parseSessionStreamError(event.data) }
                    .getOrElse { "会话流连接失败" },
                reason = SessionStreamFailureReason.ServerError,
                retryable = true,
                terminal = true,
            )
            eventName == "agent_message" -> runCatching {
                SessionStreamEvent.AgentMessage(parser.parseSessionAgentMessage(event.id, event.data))
            }.getOrElse {
                SessionStreamEvent.Failure(
                    message = "消息解析失败",
                    reason = SessionStreamFailureReason.ParseError,
                    retryable = false,
                    terminal = false,
                    detail = it.message,
                )
            }
            eventName == "runtime_state" -> runCatching {
                SessionStreamEvent.RuntimeState(parser.parseSessionRuntimeState(event.data))
            }.getOrElse {
                SessionStreamEvent.Failure(
                    message = "运行态解析失败",
                    reason = SessionStreamFailureReason.ParseError,
                    retryable = false,
                    terminal = false,
                    detail = it.message,
                )
            }
            eventName.isBlank() -> runCatching {
                val message = parser.parseSessionAgentMessage(event.id, event.data)
                SessionStreamEvent.AgentMessage(message)
            }.getOrNull()
            else -> null
        }
    }

    private fun mapSessionWindowSseEvent(event: SseEvent): SessionWindowStreamEvent? {
        val eventName = event.eventName.orEmpty()
        return when {
            eventName == "heartbeat" -> SessionWindowStreamEvent.Heartbeat
            eventName == "error" -> SessionWindowStreamEvent.Failure(
                message = runCatching { parser.parseSessionStreamError(event.data) }
                    .getOrElse { "会话窗口流连接失败" },
                reason = SessionStreamFailureReason.ServerError,
                retryable = true,
                terminal = true,
            )
            eventName == "sessions_window" -> runCatching {
                SessionWindowStreamEvent.Window(parser.parseSessionWindowSnapshot(event.data))
            }.getOrElse {
                SessionWindowStreamEvent.Failure(
                    message = "会话窗口解析失败",
                    reason = SessionStreamFailureReason.ParseError,
                    retryable = false,
                    terminal = false,
                    detail = it.message,
                )
            }
            eventName == "session_runtime_state" -> runCatching {
                SessionWindowStreamEvent.RuntimeState(parser.parseSessionWindowRuntimeState(event.data))
            }.getOrElse {
                SessionWindowStreamEvent.Failure(
                    message = "会话窗口运行态解析失败",
                    reason = SessionStreamFailureReason.ParseError,
                    retryable = false,
                    terminal = false,
                    detail = it.message,
                )
            }
            eventName == "session_agent_message" -> runCatching {
                SessionWindowStreamEvent.AgentMessage(parser.parseSessionWindowAgentMessage(event.data))
            }.getOrElse {
                SessionWindowStreamEvent.Failure(
                    message = "会话窗口消息解析失败",
                    reason = SessionStreamFailureReason.ParseError,
                    retryable = false,
                    terminal = false,
                    detail = it.message,
                )
            }
            else -> null
        }
    }

    override suspend fun reportSessionStreamClientEvent(
        token: String,
        report: SessionStreamClientEventReport,
    ) = withContext(Dispatchers.IO) {
        val traceId = diagnosticEventLogger.newTraceId("session-stream-client-event")
        val startedAt = Instant.now()
        val requestBody = json.encodeToString(
            SessionStreamClientEventReportDto(
                eventType = report.eventType.name.lowercase(Locale.US),
                threadId = report.threadId,
                deviceName = report.deviceName,
                appVersion = report.appVersion,
                reconnectAttempt = report.reconnectAttempt,
                reason = report.reason?.name?.lowercase(Locale.US),
                detail = report.detail,
                statusCode = report.statusCode,
                retryable = report.retryable,
                connectedMs = report.connectedMs,
                nextRetryDelayMs = report.nextRetryDelayMs,
                receivedAgentMessage = report.receivedAgentMessage,
                firstEventType = report.firstEventType,
            ),
        ).toRequestBody("application/json; charset=utf-8".toMediaType())
        val request = Request.Builder()
            .url(
                baseUrl.toHttpUrl().newBuilder()
                    .addPathSegment("api")
                    .addPathSegment("session-stream-events")
                    .build(),
            )
            .header("X-OpenWatcher-Token", token)
            .post(requestBody)
            .build()
        logNetworkRequest(
            traceId = traceId,
            targetName = "session_stream_client_event",
            path = "/api/session-stream-events",
            method = "POST",
        )
        try {
            callFactory.newCall(request).execute().use { response ->
                logNetworkResult(
                    traceId = traceId,
                    targetName = "session_stream_client_event",
                    path = "/api/session-stream-events",
                    method = "POST",
                    startedAt = startedAt,
                    statusCode = response.code,
                )
            }
        } catch (error: IOException) {
            logNetworkFailure(
                traceId = traceId,
                targetName = "session_stream_client_event",
                path = "/api/session-stream-events",
                method = "POST",
                startedAt = startedAt,
                error = error,
            )
        }
    }

    private fun httpFailure(code: Int): SessionStreamEvent.Failure {
        val retryable = code >= 500 || code == 408 || code == 429
        return SessionStreamEvent.Failure(
            message = "会话流连接失败",
            reason = SessionStreamFailureReason.HttpError,
            retryable = retryable,
            terminal = true,
            statusCode = code,
            detail = "http_$code",
        )
    }

    private fun windowHttpFailure(code: Int): SessionWindowStreamEvent.Failure {
        val retryable = code >= 500 || code == 408 || code == 429
        return SessionWindowStreamEvent.Failure(
            message = "会话窗口流连接失败",
            reason = SessionStreamFailureReason.HttpError,
            retryable = retryable,
            terminal = true,
            statusCode = code,
            detail = "http_$code",
        )
    }

    private fun statusHttpFailure(code: Int): StatusStreamEvent.Failure {
        val retryable = code >= 500 || code == 408 || code == 429
        return StatusStreamEvent.Failure(
            message = "状态流连接失败",
            reason = StatusStreamFailureReason.HttpError,
            retryable = retryable,
            terminal = true,
            statusCode = code,
            detail = "http_$code",
        )
    }

    private fun statusParseFailure(error: Throwable): StatusStreamEvent.Failure {
        return StatusStreamEvent.Failure(
            message = "状态流解析失败",
            reason = StatusStreamFailureReason.ParseError,
            retryable = false,
            terminal = false,
            detail = error.message,
        )
    }

    private fun IOException.asRetryableFailure(): SessionStreamEvent.Failure {
        val detail = buildString {
            append(this@asRetryableFailure::class.java.simpleName)
            val suffix = message?.trim().orEmpty()
            if (suffix.isNotBlank()) {
                append(": ")
                append(suffix)
            }
        }
        return SessionStreamEvent.Failure(
            message = "会话流连接失败",
            reason = SessionStreamFailureReason.NetworkError,
            retryable = true,
            terminal = true,
            detail = detail,
        )
    }

    private fun IOException.asRetryableWindowFailure(): SessionWindowStreamEvent.Failure {
        val detail = buildString {
            append(this@asRetryableWindowFailure::class.java.simpleName)
            val suffix = message?.trim().orEmpty()
            if (suffix.isNotBlank()) {
                append(": ")
                append(suffix)
            }
        }
        return SessionWindowStreamEvent.Failure(
            message = "会话窗口流连接失败",
            reason = SessionStreamFailureReason.NetworkError,
            retryable = true,
            terminal = true,
            detail = detail,
        )
    }

    private fun IOException.asStatusStreamFailure(): StatusStreamEvent.Failure {
        val detail = buildString {
            append(this@asStatusStreamFailure::class.java.simpleName)
            val suffix = message?.trim().orEmpty()
            if (suffix.isNotBlank()) {
                append(": ")
                append(suffix)
            }
        }
        return StatusStreamEvent.Failure(
            message = "状态流连接失败",
            reason = StatusStreamFailureReason.NetworkError,
            retryable = true,
            terminal = true,
            detail = detail,
        )
    }

    private suspend fun logNetworkRequest(
        traceId: String,
        targetName: String,
        path: String,
        method: String,
        bytesSent: Long? = null,
    ) {
        diagnosticEventLogger.log(
            event = "network_request",
            traceId = traceId,
            fields = mapOf(
                "target" to mapOf(
                    "name" to targetName,
                    "path" to path,
                    "method" to method,
                ),
                "request" to mapOf(
                    "bytesSent" to bytesSent,
                ),
            ),
        )
    }

    private suspend fun logNetworkResult(
        traceId: String,
        targetName: String,
        path: String,
        method: String,
        startedAt: Instant,
        statusCode: Int,
        bytesSent: Long? = null,
    ) {
        diagnosticEventLogger.log(
            event = "network_result",
            traceId = traceId,
            fields = mapOf(
                "target" to mapOf(
                    "name" to targetName,
                    "path" to path,
                    "method" to method,
                ),
                "result" to mapOf(
                    "statusCode" to statusCode,
                    "durationMs" to (Instant.now().toEpochMilli() - startedAt.toEpochMilli()).coerceAtLeast(0L),
                    "bytesSent" to bytesSent,
                ),
            ),
        )
    }

    private suspend fun logNetworkFailure(
        traceId: String,
        targetName: String,
        path: String,
        method: String,
        startedAt: Instant,
        error: Throwable,
        bytesSent: Long? = null,
    ) {
        diagnosticEventLogger.log(
            event = "network_result",
            level = DiagnosticLevel.Warn,
            traceId = traceId,
            fields = mapOf(
                "target" to mapOf(
                    "name" to targetName,
                    "path" to path,
                    "method" to method,
                ),
                "result" to mapOf(
                    "durationMs" to (Instant.now().toEpochMilli() - startedAt.toEpochMilli()).coerceAtLeast(0L),
                    "bytesSent" to bytesSent,
                    "errorType" to error::class.java.simpleName,
                    "message" to error.message,
                ),
            ),
        )
    }
}

class DebugAwareWatcherApi(
    private val delegate: WatcherApi,
    private val demoStore: DebugDemoStore,
) : WatcherApi {
    private fun demoSessionAgentMessage(threadId: String, eventId: String): SessionAgentMessage {
        return SessionAgentMessage(
            threadId = threadId,
            eventId = eventId,
            createdAt = Instant.parse("2026-06-03T03:30:20Z"),
            text = "这张演示图专门用来验详情页中间区。上面的 token、模型和 effort 需要和标题分开，中间正文要足够长，才能看出布局有没有互相压住。\n\n这里故意放一段更长的 agent 回复，包含多行中文、不同长度的句子和一个较长的收尾段落。目标不是测试排版美观，而是确认详情页在长消息场景下，顶部元数据、中心正文和底部 context 读数还能同时成立。\n\n如果这段文字还能继续完整显示几行，说明中间区域没有被上方挤扁；如果第一屏只剩一两行，或者长消息一出现就把标题和元数据顶乱，那就说明布局还不够稳。",
            truncated = false,
        )
    }

    override suspend fun fetchStatus(token: String): StatusFetchResult {
        return when (demoStore.current()) {
            DebugDemoScenario.NONE -> delegate.fetchStatus(token)
            DebugDemoScenario.DASHBOARD -> StatusFetchResult.Success(sampleSnapshot())
            DebugDemoScenario.QUOTA_STALE -> StatusFetchResult.Success(sampleSnapshot(quotaStatus = QuotaStatus.Stale))
            DebugDemoScenario.UNAUTHORIZED -> StatusFetchResult.Unauthorized
            DebugDemoScenario.OFFLINE -> StatusFetchResult.NetworkFailure("服务连接失败")
        }
    }

    override suspend fun checkHealth(): HealthCheckResult {
        return when (demoStore.current()) {
            DebugDemoScenario.OFFLINE -> HealthCheckResult.Offline("服务不可达")
            DebugDemoScenario.DASHBOARD,
            DebugDemoScenario.QUOTA_STALE,
            DebugDemoScenario.UNAUTHORIZED,
            -> HealthCheckResult.Online
            DebugDemoScenario.NONE -> delegate.checkHealth()
        }
    }

    override suspend fun uploadScreenshot(
        token: String,
        request: ScreenshotUploadRequest,
    ): ScreenshotUploadResult {
        return when (demoStore.current()) {
            DebugDemoScenario.UNAUTHORIZED -> ScreenshotUploadResult.Unauthorized
            DebugDemoScenario.OFFLINE -> ScreenshotUploadResult.NetworkFailure("服务连接失败")
            else -> delegate.uploadScreenshot(token, request)
        }
    }

    override suspend fun uploadDiagnostics(
        token: String,
        request: DiagnosticUploadRequest,
        onProgress: (bytesUploaded: Long, totalBytes: Long) -> Unit,
    ): DiagnosticUploadResult {
        return when (demoStore.current()) {
            DebugDemoScenario.UNAUTHORIZED -> DiagnosticUploadResult.Unauthorized
            DebugDemoScenario.OFFLINE -> DiagnosticUploadResult.NetworkFailure("服务连接失败")
            else -> delegate.uploadDiagnostics(token, request, onProgress)
        }
    }

    override fun streamStatus(token: String, includeDailyTrend30d: Boolean): Flow<StatusStreamEvent> {
        return when (demoStore.current()) {
            DebugDemoScenario.NONE -> delegate.streamStatus(token, includeDailyTrend30d)
            DebugDemoScenario.DASHBOARD -> flow { emit(StatusStreamEvent.Snapshot(sampleSnapshot())) }
            DebugDemoScenario.QUOTA_STALE -> flow { emit(StatusStreamEvent.Snapshot(sampleSnapshot(quotaStatus = QuotaStatus.Stale))) }
            DebugDemoScenario.UNAUTHORIZED -> flow {
                emit(
                    StatusStreamEvent.Failure(
                        message = "需要重新配对",
                        reason = StatusStreamFailureReason.Unauthorized,
                        retryable = false,
                        terminal = true,
                        statusCode = 401,
                    ),
                )
            }
            DebugDemoScenario.OFFLINE -> flow {
                emit(
                    StatusStreamEvent.Failure(
                        message = "状态流连接失败",
                        reason = StatusStreamFailureReason.NetworkError,
                        retryable = true,
                        terminal = true,
                    ),
                )
            }
        }
    }

    override fun streamSessionAgentMessages(
        token: String,
        threadId: String,
        includeMessages: Boolean,
    ): Flow<SessionStreamEvent> {
        return when (demoStore.current()) {
            DebugDemoScenario.NONE -> delegate.streamSessionAgentMessages(token, threadId, includeMessages)
            DebugDemoScenario.DASHBOARD,
            DebugDemoScenario.QUOTA_STALE,
            -> flow {
                emit(
                    SessionStreamEvent.AgentMessage(
                        demoSessionAgentMessage(threadId, "demo-agent-message"),
                    ),
                )
                while (true) {
                    delay(15_000)
                    emit(SessionStreamEvent.Heartbeat)
                }
            }
            DebugDemoScenario.UNAUTHORIZED,
            DebugDemoScenario.OFFLINE,
            -> flow { emit(SessionStreamEvent.Heartbeat) }
        }
    }

    override fun streamSessionWindow(
        token: String,
        limit: Int,
        preferredOrder: List<String>,
    ): Flow<SessionWindowStreamEvent> {
        return when (demoStore.current()) {
            DebugDemoScenario.NONE -> delegate.streamSessionWindow(token, limit, preferredOrder)
            DebugDemoScenario.DASHBOARD,
            DebugDemoScenario.QUOTA_STALE,
            -> flow {
                emit(
                    SessionWindowStreamEvent.Window(
                        SessionWindowSnapshot(
                            observedAt = Instant.parse("2026-06-03T03:30:00Z"),
                            limit = limit,
                            threadOrder = sampleSnapshot().sessions.take(limit).map { it.threadId },
                            sessions = sampleSnapshot().sessions.take(limit).map {
                                SessionWindowEntry(
                                    session = it,
                                    runtimeState = SessionRuntimeState(
                                        threadId = it.threadId,
                                        turnId = null,
                                        startedAt = null,
                                        running = false,
                                        lifecycle = SessionRuntimeLifecycle.Idle,
                                        phase = SessionRuntimePhase.Unknown,
                                        updatedAt = it.updatedAt,
                                        sequence = 0,
                                    ),
                                    latestAgentMessage = demoSessionAgentMessage(
                                        threadId = it.threadId,
                                        eventId = "demo-window-message-${it.threadId}",
                                    ),
                                )
                            },
                        ),
                    ),
                )
                while (true) {
                    delay(15_000)
                    emit(SessionWindowStreamEvent.Heartbeat)
                }
            }
            DebugDemoScenario.UNAUTHORIZED -> flow {
                emit(
                    SessionWindowStreamEvent.Failure(
                        message = "需要重新配对",
                        reason = SessionStreamFailureReason.Unauthorized,
                        retryable = false,
                        terminal = true,
                    ),
                )
            }
            DebugDemoScenario.OFFLINE -> flow {
                emit(
                    SessionWindowStreamEvent.Failure(
                        message = "会话窗口流连接失败",
                        reason = SessionStreamFailureReason.NetworkError,
                        retryable = true,
                        terminal = true,
                    ),
                )
            }
        }
    }

    override suspend fun reportSessionStreamClientEvent(token: String, report: SessionStreamClientEventReport) {
        if (demoStore.current() == DebugDemoScenario.NONE) {
            delegate.reportSessionStreamClientEvent(token, report)
        }
    }

    private fun sampleSnapshot(quotaStatus: QuotaStatus = QuotaStatus.Ok): WatcherStatusSnapshot {
        val bucketTotals = listOf(
            6_000L, 8_000L, 10_000L, 7_000L, 6_000L, 5_000L,
            8_000L, 14_000L, 22_000L, 38_000L, 64_000L, 88_000L,
            122_000L, 148_000L, 132_000L, 94_000L, 58_000L, 42_000L,
            28_000L, 18_000L, 12_000L, 10_000L, 16_000L, 24_000L,
        )
        val bucketStarts = List(bucketTotals.size) { index ->
            Instant.parse("2026-06-02T04:00:00Z").plusSeconds(index * 3600L)
        }
        val heatmap7dRows = listOf(
            listOf(0L, 0L, 4_000L, 8_000L, 16_000L, 26_000L, 22_000L, 12_000L, 4_000L, 0L, 0L, 0L, 0L, 6_000L, 14_000L, 9_000L, 3_000L, 0L, 0L, 0L, 0L, 0L, 0L, 0L),
            listOf(0L, 2_000L, 12_000L, 24_000L, 42_000L, 58_000L, 46_000L, 20_000L, 4_000L, 0L, 0L, 16_000L, 38_000L, 32_000L, 14_000L, 0L, 0L, 0L, 0L, 0L, 0L, 6_000L, 12_000L, 0L),
            listOf(0L, 5_000L, 18_000L, 36_000L, 64_000L, 78_000L, 70_000L, 32_000L, 6_000L, 0L, 0L, 22_000L, 56_000L, 48_000L, 20_000L, 0L, 0L, 0L, 0L, 0L, 12_000L, 24_000L, 0L, 0L),
            listOf(0L, 3_000L, 15_000L, 32_000L, 56_000L, 66_000L, 60_000L, 24_000L, 5_000L, 0L, 0L, 12_000L, 28_000L, 34_000L, 18_000L, 0L, 0L, 0L, 0L, 0L, 18_000L, 34_000L, 8_000L, 0L),
            listOf(0L, 0L, 8_000L, 16_000L, 24_000L, 30_000L, 22_000L, 10_000L, 2_000L, 0L, 0L, 0L, 0L, 8_000L, 16_000L, 10_000L, 2_000L, 0L, 0L, 0L, 10_000L, 18_000L, 0L, 0L),
            listOf(0L, 0L, 0L, 6_000L, 12_000L, 14_000L, 10_000L, 4_000L, 0L, 0L, 0L, 0L, 0L, 0L, 8_000L, 12_000L, 8_000L, 0L, 0L, 0L, 0L, 8_000L, 12_000L, 0L),
            listOf(0L, 0L, 0L, 0L, 0L, 4_000L, 10_000L, 18_000L, 12_000L, 0L, 0L, 0L, 0L, 0L, 0L, 6_000L, 10_000L, 4_000L, 0L, 0L, 0L, 0L, 0L, 0L),
        )
        val heatmap7dPeak = heatmap7dRows.flatten().maxOrNull() ?: 0L

        return WatcherStatusSnapshot(
            observedAt = Instant.parse("2026-06-03T03:30:00Z"),
            quota = QuotaSnapshot(
                source = "oauth-api",
                fresh = quotaStatus == QuotaStatus.Ok,
                status = quotaStatus,
                planType = "pro",
                fiveHour = QuotaWindow(
                    usedPercent = 42f,
                    remainingPercent = 58f,
                    resetAt = Instant.parse("2026-06-03T06:32:00Z"),
                ),
                weekly = QuotaWindow(
                    usedPercent = 68f,
                    remainingPercent = 32f,
                    resetAt = Instant.parse("2026-06-07T16:00:00Z"),
                ),
            ),
            heatmap24h = Heatmap24hSnapshot(
                timezone = "Asia/Shanghai",
                generatedAt = Instant.parse("2026-06-03T03:30:00Z"),
                peakHourStart = Instant.parse("2026-06-03T02:00:00Z"),
                buckets = bucketTotals.mapIndexed { index, total ->
                    val input = (total * 0.69).toLong()
                    val cached = (total * 0.20).toLong()
                    val output = total - input - cached
                    HeatmapBucket(
                        hourStart = bucketStarts[index],
                        inputTokens = input,
                        cachedInputTokens = cached,
                        outputTokens = output,
                        reasoningOutputTokens = (output * 0.35).toLong(),
                        totalTokens = total,
                        activeThreads = if (total > 90_000) 4 else 2,
                    )
                },
            ),
            heatmap7d = Heatmap7dSnapshot(
                timezone = "Asia/Shanghai",
                generatedAt = Instant.parse("2026-06-03T03:30:00Z"),
                startDate = "2026-05-28",
                endDate = "2026-06-03",
                peakTokens = heatmap7dPeak,
                days = heatmap7dRows.mapIndexed { index, row ->
                    Heatmap7dDay(
                        date = Instant.parse("2026-05-28T00:00:00Z").plusSeconds(index * 86_400L)
                            .atZone(java.time.ZoneId.of("Asia/Shanghai"))
                            .toLocalDate()
                            .toString(),
                        totalTokens = row.sum(),
                        hours = row,
                    )
                },
            ),
            dailyUsage = DailyUsageSnapshot(
                generatedAt = Instant.parse("2026-06-03T03:30:00Z"),
                totalTokens = bucketTotals.sum(),
                inputTokens = bucketTotals.sumOf { (it * 0.69).toLong() },
                cachedInputTokens = bucketTotals.sumOf { (it * 0.20).toLong() },
                outputTokens = bucketTotals.sumOf { total ->
                    val input = (total * 0.69).toLong()
                    val cached = (total * 0.20).toLong()
                    total - input - cached
                },
                reasoningOutputTokens = bucketTotals.sumOf { total ->
                    val input = (total * 0.69).toLong()
                    val cached = (total * 0.20).toLong()
                    ((total - input - cached) * 0.35).toLong()
                },
                activeSessions = 5,
                estimatedValueUsd = 5.28,
                estimatedValueLabel = "\$5.28",
                pricingDate = "2026-06-04",
                pricingSourceUrl = "https://developers.openai.com/api/docs/pricing",
                pricingUnavailableReason = null,
                modelShares = listOf(
                    DailyUsageModelShare(model = "gpt-5.5", tokens = 612_000L, sharePercent = 42.8),
                    DailyUsageModelShare(model = "gpt-5", tokens = 488_000L, sharePercent = 34.1),
                    DailyUsageModelShare(model = "gpt-5-mini", tokens = 330_000L, sharePercent = 23.1),
                ),
            ),
            dailyTrend30d = DailyTrend30dSnapshot(
                timezone = "Asia/Shanghai",
                generatedAt = Instant.parse("2026-06-03T03:30:00Z"),
                startDate = "2026-05-04",
                endDate = "2026-06-02",
                totalTokens = 7_920_000,
                averageTokens = 248_000,
                peakTokens = 612_000,
                estimatedValueUsd = 31.46,
                estimatedValueLabel = "\$31.46",
                days = List(30) { index ->
                    val total = listOf(
                        88_000L, 96_000L, 102_000L, 110_000L, 118_000L, 142_000L,
                        156_000L, 165_000L, 174_000L, 182_000L, 190_000L, 205_000L,
                        198_000L, 212_000L, 224_000L, 238_000L, 254_000L, 271_000L,
                        263_000L, 281_000L, 295_000L, 320_000L, 348_000L, 366_000L,
                        392_000L, 418_000L, 447_000L, 510_000L, 612_000L, 544_000L,
                    )[index]
                    DailyTrendDay(
                        date = Instant.parse("2026-05-04T00:00:00Z").plusSeconds(index * 86_400L)
                            .atZone(java.time.ZoneId.of("Asia/Shanghai"))
                            .toLocalDate()
                            .toString(),
                        totalTokens = total,
                    )
                },
            ),
            sessions = listOf(
                SessionSnapshot(
                    threadId = "019e8943-36f6-73b2-8a7a-c30c3ecc0ef2",
                    title = "AnyChat",
                    updatedAt = Instant.parse("2026-06-03T03:29:20Z"),
                    model = "gpt-5.5",
                    reasoningEffort = "high",
                    tokensUsedTotal = 50_000_000,
                    contextUsedTokens = 120_800,
                    contextWindow = 920_000,
                    contextPressurePercent = 13,
                    contextCompactThresholdTokens = 300_000,
                    contextCompactThresholdPercent = 32,
                    lastActiveAgoMinutes = 7,
                ),
                SessionSnapshot(
                    threadId = "019e8943-1111-73b2-8a7a-c30c3ecc0abc",
                    title = "Android Build",
                    updatedAt = Instant.parse("2026-06-03T03:27:10Z"),
                    model = "gpt-5",
                    reasoningEffort = "medium",
                    tokensUsedTotal = 200_000_000,
                    contextUsedTokens = 91_000,
                    contextWindow = 192_000,
                    contextPressurePercent = 47,
                    contextCompactThresholdTokens = 160_000,
                    contextCompactThresholdPercent = 82,
                    lastActiveAgoMinutes = 3,
                ),
                SessionSnapshot(
                    threadId = "019e8943-2222-73b2-8a7a-c30c3ecc0def",
                    title = "UI Refactor",
                    updatedAt = Instant.parse("2026-06-03T03:25:00Z"),
                    model = "gpt-5.5",
                    reasoningEffort = "xhigh",
                    tokensUsedTotal = 100_000_000,
                    contextUsedTokens = 120_000,
                    contextWindow = 256_000,
                    contextPressurePercent = 46,
                    contextCompactThresholdTokens = 128_000,
                    contextCompactThresholdPercent = 49,
                    lastActiveAgoMinutes = 5,
                ),
                SessionSnapshot(
                    threadId = "019e8943-3333-73b2-8a7a-c30c3ecc0123",
                    title = "Docs Sync",
                    updatedAt = Instant.parse("2026-06-03T03:18:00Z"),
                    model = "gpt-5.4-mini",
                    reasoningEffort = "low",
                    tokensUsedTotal = 30_000_000,
                    contextUsedTokens = 22_000,
                    contextWindow = 128_000,
                    contextPressurePercent = 9,
                    contextCompactThresholdTokens = null,
                    contextCompactThresholdPercent = null,
                    lastActiveAgoMinutes = 12,
                ),
                SessionSnapshot(
                    threadId = "019e8943-4444-73b2-8a7a-c30c3ecc0456",
                    title = "Long Context Strategy Review For Round Watch Session Title Marquee Stress Test",
                    updatedAt = Instant.parse("2026-06-03T03:02:00Z"),
                    model = "gpt-5.5-mini",
                    reasoningEffort = "xhigh",
                    tokensUsedTotal = 5_000_000,
                    contextUsedTokens = 120_800,
                    contextWindow = 920_000,
                    contextPressurePercent = 13,
                    contextCompactThresholdTokens = null,
                    contextCompactThresholdPercent = null,
                    lastActiveAgoMinutes = 28,
                ),
            ),
            errors = emptyList(),
        )
    }
}

private class ProgressByteArrayRequestBody(
    private val bytes: ByteArray,
    private val mediaType: okhttp3.MediaType,
    private val onProgress: (bytesUploaded: Long, totalBytes: Long) -> Unit,
) : RequestBody() {
    override fun contentType() = mediaType

    override fun contentLength(): Long = bytes.size.toLong()

    override fun writeTo(sink: BufferedSink) {
        if (bytes.isEmpty()) {
            onProgress(0L, 0L)
            return
        }
        var offset = 0
        while (offset < bytes.size) {
            val nextCount = minOf(CHUNK_SIZE, bytes.size - offset)
            sink.write(bytes, offset, nextCount)
            offset += nextCount
            onProgress(offset.toLong(), bytes.size.toLong())
        }
    }

    companion object {
        private const val CHUNK_SIZE = 8 * 1024
    }
}

class WatcherJsonParser(
    private val json: Json = Json {
        ignoreUnknownKeys = true
        explicitNulls = false
    },
) {
    fun parseHealth(body: String): Boolean {
        return json.decodeFromString<HealthResponseDto>(body).ok
    }

    fun parseScreenshotUploadFilename(body: String): String {
        return json.decodeFromString<ScreenshotUploadResponseDto>(body).filename.trim()
    }

    fun parseStatus(body: String): WatcherStatusSnapshot {
        val dto = json.decodeFromString<StatusResponseDto>(body)
        return WatcherStatusSnapshot(
            observedAt = dto.observedAt?.toInstantOrNull(),
            quota = dto.quota?.toDomain(),
            heatmap24h = dto.heatmap24h?.toDomain(),
            heatmap7d = dto.heatmap7d?.toDomain(),
            dailyUsage = dto.dailyUsage?.toDomain(),
            dailyTrend30d = dto.dailyTrend30d?.toDomain(),
            sessions = dto.sessions.orEmpty().map { it.toDomain() },
            errors = dto.errors.orEmpty(),
        )
    }

    fun parseStatusStreamSnapshot(data: String): WatcherStatusSnapshot {
        val dto = json.decodeFromString<StatusResponseDto>(data)
        require(dto.type == null || dto.type == "status_snapshot") { "unexpected status stream event type" }
        return WatcherStatusSnapshot(
            observedAt = dto.observedAt?.toInstantOrNull(),
            quota = dto.quota?.toDomain(),
            heatmap24h = dto.heatmap24h?.toDomain(),
            heatmap7d = dto.heatmap7d?.toDomain(),
            dailyUsage = dto.dailyUsage?.toDomain(),
            dailyTrend30d = dto.dailyTrend30d?.toDomain(),
            sessions = dto.sessions.orEmpty().map { it.toDomain() },
            errors = dto.errors.orEmpty(),
        )
    }

    fun parseStatusQuotaUpdate(data: String): StatusQuotaUpdate {
        val dto = json.decodeFromString<StatusQuotaUpdateDto>(data)
        require(dto.type == "status_quota") { "unexpected status stream event type" }
        return StatusQuotaUpdate(
            observedAt = dto.observedAt?.toInstantOrNull(),
            quota = dto.quota?.toDomain(),
        )
    }

    fun parseStatusHeatmapUpdate(data: String): StatusHeatmapUpdate {
        val dto = json.decodeFromString<StatusHeatmapUpdateDto>(data)
        require(dto.type == "status_heatmap24h") { "unexpected status stream event type" }
        return StatusHeatmapUpdate(
            observedAt = dto.observedAt?.toInstantOrNull(),
            heatmap24h = dto.heatmap24h?.toDomain(),
            heatmap7d = dto.heatmap7d?.toDomain(),
            dailyUsage = dto.dailyUsage?.toDomain(),
        )
    }

    fun parseStatusSessionsUpdate(data: String): StatusSessionsUpdate {
        val dto = json.decodeFromString<StatusSessionsUpdateDto>(data)
        require(dto.type == "status_sessions") { "unexpected status stream event type" }
        return StatusSessionsUpdate(
            observedAt = dto.observedAt?.toInstantOrNull(),
            sessions = dto.sessions.orEmpty().map { it.toDomain() },
        )
    }

    fun parseStatusErrorsUpdate(data: String): StatusErrorsUpdate {
        val dto = json.decodeFromString<StatusErrorsUpdateDto>(data)
        require(dto.type == "status_errors") { "unexpected status stream event type" }
        return StatusErrorsUpdate(
            observedAt = dto.observedAt?.toInstantOrNull(),
            errors = dto.errors.orEmpty(),
        )
    }

    fun parseSessionAgentMessage(eventId: String?, data: String): SessionAgentMessage {
        val dto = json.decodeFromString<SessionAgentMessageDto>(data)
        require(dto.type == "agent_message") { "unexpected stream event type" }
        return SessionAgentMessage(
            threadId = dto.threadId,
            eventId = dto.eventId ?: eventId.orEmpty(),
            createdAt = dto.createdAt?.toInstantOrNull(),
            text = dto.text.trim(),
            truncated = dto.truncated,
        )
    }

    fun parseSessionStreamError(data: String): String {
        val dto = json.decodeFromString<SessionStreamErrorDto>(data)
        return dto.message.trim().ifBlank { "会话流连接失败" }
    }

    fun parseStatusStreamError(data: String): String {
        val dto = json.decodeFromString<SessionStreamErrorDto>(data)
        return dto.message.trim().ifBlank { "状态流连接失败" }
    }

    fun parseSessionRuntimeState(data: String): SessionRuntimeState {
        val dto = json.decodeFromString<SessionRuntimeStateDto>(data)
        require(dto.type == "runtime_state") { "unexpected stream event type" }
        return SessionRuntimeState(
            threadId = dto.threadId,
            turnId = dto.turnId?.takeIf { it.isNotBlank() },
            startedAt = dto.startedAt?.toInstantOrNull(),
            running = dto.running,
            lifecycle = dto.lifecycle.toRuntimeLifecycle(),
            phase = dto.phase.toRuntimePhase(),
            updatedAt = dto.updatedAt?.toInstantOrNull(),
            sequence = dto.sequence,
        )
    }

    fun parseSessionWindowSnapshot(data: String): SessionWindowSnapshot {
        val dto = json.decodeFromString<SessionWindowDto>(data)
        require(dto.type == "sessions_window") { "unexpected stream event type" }
        return SessionWindowSnapshot(
            observedAt = dto.observedAt?.toInstantOrNull(),
            limit = dto.limit,
            threadOrder = dto.threadOrder.orEmpty(),
            sessions = dto.sessions.orEmpty().map { it.toDomain() },
        )
    }

    fun parseSessionWindowRuntimeState(data: String): SessionRuntimeState {
        val dto = json.decodeFromString<SessionWindowRuntimeEventDto>(data)
        require(dto.type == "session_runtime_state") { "unexpected stream event type" }
        val runtime = dto.runtimeState ?: error("missing runtimeState")
        return SessionRuntimeState(
            threadId = runtime.threadId,
            turnId = runtime.turnId?.takeIf { it.isNotBlank() },
            startedAt = runtime.startedAt?.toInstantOrNull(),
            running = runtime.running,
            lifecycle = runtime.lifecycle.toRuntimeLifecycle(),
            phase = runtime.phase.toRuntimePhase(),
            updatedAt = runtime.updatedAt?.toInstantOrNull(),
            sequence = runtime.sequence,
        )
    }

    fun parseSessionWindowAgentMessage(data: String): SessionAgentMessage {
        val dto = json.decodeFromString<SessionWindowAgentMessageEventDto>(data)
        require(dto.type == "session_agent_message") { "unexpected stream event type" }
        val message = dto.agentMessage ?: error("missing agentMessage")
        return SessionAgentMessage(
            threadId = message.threadId,
            eventId = message.eventId.orEmpty(),
            createdAt = message.createdAt?.toInstantOrNull(),
            text = message.text.trim(),
            truncated = message.truncated,
        )
    }

    private fun QuotaDto.toDomain(): QuotaSnapshot {
        return QuotaSnapshot(
            source = source,
            fresh = fresh,
            status = status.toQuotaStatus(
                fresh = fresh,
                hasCachedWindow = fiveHour != null || weekly != null || !planType.isNullOrBlank(),
            ),
            planType = planType,
            fiveHour = fiveHour?.toDomain(),
            weekly = weekly?.toDomain(),
        )
    }

    private fun WindowDto.toDomain(): QuotaWindow {
        return QuotaWindow(
            usedPercent = usedPercent,
            remainingPercent = remainingPercent,
            resetAt = resetAt?.takeIf { it > 0 }?.let(Instant::ofEpochSecond),
        )
    }

    private fun HeatmapDto.toDomain(): Heatmap24hSnapshot {
        return Heatmap24hSnapshot(
            timezone = timezone,
            generatedAt = generatedAt?.toInstantOrNull(),
            peakHourStart = peakHourStart?.toInstantOrNull(),
            buckets = buckets.orEmpty().map { it.toDomain() },
        )
    }

    private fun HeatmapBucketDto.toDomain(): HeatmapBucket {
        return HeatmapBucket(
            hourStart = hourStart?.toInstantOrNull(),
            inputTokens = inputTokens,
            cachedInputTokens = cachedInputTokens,
            outputTokens = outputTokens,
            reasoningOutputTokens = reasoningOutputTokens,
            totalTokens = totalTokens,
            activeThreads = activeThreads,
        )
    }

    private fun Heatmap7dDto.toDomain(): Heatmap7dSnapshot {
        return Heatmap7dSnapshot(
            timezone = timezone,
            generatedAt = generatedAt?.toInstantOrNull(),
            startDate = startDate,
            endDate = endDate,
            peakTokens = peakTokens,
            days = days.orEmpty().map { it.toDomain() },
        )
    }

    private fun Heatmap7dDayDto.toDomain(): Heatmap7dDay {
        return Heatmap7dDay(
            date = date,
            totalTokens = totalTokens,
            hours = hours.orEmpty(),
        )
    }

    private fun DailyUsageDto.toDomain(): DailyUsageSnapshot {
        return DailyUsageSnapshot(
            generatedAt = generatedAt?.toInstantOrNull(),
            totalTokens = totalTokens,
            inputTokens = inputTokens,
            cachedInputTokens = cachedInputTokens,
            outputTokens = outputTokens,
            reasoningOutputTokens = reasoningOutputTokens,
            activeSessions = activeSessions,
            estimatedValueUsd = estimatedValueUsd,
            estimatedValueLabel = estimatedValueLabel,
            pricingDate = pricingDate,
            pricingSourceUrl = pricingSourceUrl,
            pricingUnavailableReason = pricingUnavailableReason,
            modelShares = modelShares.orEmpty().map { it.toDomain() },
        )
    }

    private fun DailyUsageModelShareDto.toDomain(): DailyUsageModelShare {
        return DailyUsageModelShare(
            model = model,
            tokens = tokens,
            sharePercent = sharePercent,
        )
    }

    private fun DailyTrend30dDto.toDomain(): DailyTrend30dSnapshot {
        return DailyTrend30dSnapshot(
            timezone = timezone,
            generatedAt = generatedAt?.toInstantOrNull(),
            startDate = startDate,
            endDate = endDate,
            totalTokens = totalTokens,
            averageTokens = averageTokens,
            peakTokens = peakTokens,
            estimatedValueUsd = estimatedValueUsd,
            estimatedValueLabel = estimatedValueLabel,
            days = days.orEmpty().map {
                DailyTrendDay(date = it.date, totalTokens = it.totalTokens)
            },
        )
    }

    private fun SessionDto.toDomain(): SessionSnapshot {
        return SessionSnapshot(
            threadId = threadId,
            title = title,
            updatedAt = updatedAt?.toInstantOrNull(),
            model = model,
            reasoningEffort = reasoningEffort,
            tokensUsedTotal = tokensUsedTotal,
            contextUsedTokens = contextUsedTokens,
            contextWindow = contextWindow,
            contextPressurePercent = contextPressurePercent,
            contextCompactThresholdTokens = contextCompactThresholdTokens?.takeIf { it > 0L },
            contextCompactThresholdPercent = contextCompactThresholdPercent?.takeIf { it in 1..100 },
            contextCompaction = contextCompaction?.toDomain(),
            lastActiveAgoMinutes = lastActiveAgoMinutes,
        )
    }

    private fun SessionWindowSessionDto.toDomain(): SessionWindowEntry {
        val runtime = runtimeState ?: error("missing runtimeState")
        return SessionWindowEntry(
            session = SessionSnapshot(
                threadId = threadId,
                title = title,
                updatedAt = updatedAt?.toInstantOrNull(),
                model = model,
                reasoningEffort = reasoningEffort,
                tokensUsedTotal = tokensUsedTotal,
                contextUsedTokens = contextUsedTokens,
                contextWindow = contextWindow,
                contextPressurePercent = contextPressurePercent,
                contextCompactThresholdTokens = contextCompactThresholdTokens?.takeIf { it > 0L },
                contextCompactThresholdPercent = contextCompactThresholdPercent?.takeIf { it in 1..100 },
                contextCompaction = contextCompaction?.toDomain(),
                lastActiveAgoMinutes = lastActiveAgoMinutes,
            ),
            runtimeState = SessionRuntimeState(
                threadId = runtime.threadId,
                turnId = runtime.turnId?.takeIf { it.isNotBlank() },
                startedAt = runtime.startedAt?.toInstantOrNull(),
                running = runtime.running,
                lifecycle = runtime.lifecycle.toRuntimeLifecycle(),
                phase = runtime.phase.toRuntimePhase(),
                updatedAt = runtime.updatedAt?.toInstantOrNull(),
                sequence = runtime.sequence,
            ),
            latestAgentMessage = latestAgentMessage?.let {
                SessionAgentMessage(
                    threadId = it.threadId,
                    eventId = it.eventId.orEmpty(),
                    createdAt = it.createdAt?.toInstantOrNull(),
                    text = it.text.trim(),
                    truncated = it.truncated,
                )
            },
        )
    }

    private fun ContextCompactionDto.toDomain(): ContextCompactionSnapshot {
        return ContextCompactionSnapshot(
            trigger = trigger.trim(),
            startedAt = startedAt?.toInstantOrNull(),
            updatedAt = updatedAt?.toInstantOrNull(),
            turnId = turnId?.takeIf { it.isNotBlank() },
        )
    }

    private fun String.toInstantOrNull(): Instant? = runCatching { Instant.parse(this) }.getOrNull()

    private fun String.toRuntimeLifecycle(): SessionRuntimeLifecycle {
        return when (lowercase(Locale.US)) {
            "running" -> SessionRuntimeLifecycle.Running
            "completed" -> SessionRuntimeLifecycle.Completed
            "aborted" -> SessionRuntimeLifecycle.Aborted
            else -> SessionRuntimeLifecycle.Idle
        }
    }

    private fun String.toRuntimePhase(): SessionRuntimePhase {
        return when (lowercase(Locale.US)) {
            "reasoning" -> SessionRuntimePhase.Reasoning
            "tool_running" -> SessionRuntimePhase.ToolRunning
            "agent_commentary" -> SessionRuntimePhase.AgentCommentary
            "agent_final" -> SessionRuntimePhase.AgentFinal
            else -> SessionRuntimePhase.Unknown
        }
    }
}

internal data class SseEvent(
    val eventName: String?,
    val id: String?,
    val data: String,
)

internal class SessionSseParser {
    private var eventName: String? = null
    private var id: String? = null
    private val dataLines = mutableListOf<String>()

    fun accept(line: String): SseEvent? {
        if (line.isEmpty()) {
            return dispatch()
        }
        if (line.startsWith(":")) {
            return null
        }
        val separator = line.indexOf(':')
        val field = if (separator >= 0) line.substring(0, separator) else line
        val rawValue = if (separator >= 0) line.substring(separator + 1) else ""
        val value = rawValue.removePrefix(" ")
        when (field) {
            "event" -> eventName = value
            "id" -> id = value
            "data" -> dataLines += value
        }
        return null
    }

    fun finish(): SseEvent? = dispatch()

    private fun dispatch(): SseEvent? {
        if (eventName == null && id == null && dataLines.isEmpty()) {
            return null
        }
        val event = SseEvent(
            eventName = eventName,
            id = id,
            data = dataLines.joinToString(separator = "\n"),
        )
        eventName = null
        id = null
        dataLines.clear()
        return event
    }
}

@Serializable
private data class HealthResponseDto(
    val ok: Boolean,
)

@Serializable
private data class ScreenshotUploadResponseDto(
    val ok: Boolean,
    val filename: String = "",
)

@Serializable
private data class DiagnosticUploadResponseDto(
    val ok: Boolean,
    val diagnosticId: String = "",
    val receivedAt: String,
)

@Serializable
private data class StatusResponseDto(
    val type: String? = null,
    val ok: Boolean,
    val observedAt: String? = null,
    val quota: QuotaDto? = null,
    val heatmap24h: HeatmapDto? = null,
    val heatmap7d: Heatmap7dDto? = null,
    val dailyUsage: DailyUsageDto? = null,
    val dailyTrend30d: DailyTrend30dDto? = null,
    val sessions: List<SessionDto>? = null,
    val errors: List<String>? = null,
)

data class StatusQuotaUpdate(
    val observedAt: Instant?,
    val quota: QuotaSnapshot?,
)

data class StatusHeatmapUpdate(
    val observedAt: Instant?,
    val heatmap24h: Heatmap24hSnapshot?,
    val heatmap7d: Heatmap7dSnapshot?,
    val dailyUsage: DailyUsageSnapshot?,
)

data class StatusSessionsUpdate(
    val observedAt: Instant?,
    val sessions: List<SessionSnapshot>,
)

data class StatusErrorsUpdate(
    val observedAt: Instant?,
    val errors: List<String>,
)

@Serializable
private data class StatusQuotaUpdateDto(
    val type: String,
    val observedAt: String? = null,
    val quota: QuotaDto? = null,
)

@Serializable
private data class StatusHeatmapUpdateDto(
    val type: String,
    val observedAt: String? = null,
    val heatmap24h: HeatmapDto? = null,
    val heatmap7d: Heatmap7dDto? = null,
    val dailyUsage: DailyUsageDto? = null,
)

@Serializable
private data class StatusSessionsUpdateDto(
    val type: String,
    val observedAt: String? = null,
    val sessions: List<SessionDto>? = null,
)

@Serializable
private data class StatusErrorsUpdateDto(
    val type: String,
    val observedAt: String? = null,
    val errors: List<String>? = null,
)

@Serializable
private data class SessionAgentMessageDto(
    val type: String,
    val threadId: String,
    val eventId: String? = null,
    val createdAt: String? = null,
    val text: String = "",
    val truncated: Boolean = false,
)

@Serializable
private data class SessionWindowDto(
    val type: String,
    val observedAt: String? = null,
    val limit: Int = 0,
    val threadOrder: List<String>? = null,
    val sessions: List<SessionWindowSessionDto>? = null,
)

@Serializable
private data class SessionWindowSessionDto(
    val threadId: String,
    val title: String,
    val updatedAt: String? = null,
    val model: String,
    val reasoningEffort: String = "",
    val tokensUsedTotal: Long = 0,
    val contextUsedTokens: Long = 0,
    val contextWindow: Long = 0,
    val contextPressurePercent: Int = 0,
    val contextCompactThresholdTokens: Long? = null,
    val contextCompactThresholdPercent: Int? = null,
    val contextCompaction: ContextCompactionDto? = null,
    val lastActiveAgoMinutes: Int = 0,
    val runtimeState: SessionRuntimeStateDto? = null,
    val latestAgentMessage: SessionAgentMessageDto? = null,
)

@Serializable
private data class SessionRuntimeStateDto(
    val type: String,
    val threadId: String,
    val turnId: String? = null,
    val startedAt: String? = null,
    val running: Boolean = false,
    val lifecycle: String = "idle",
    val phase: String = "unknown",
    val updatedAt: String? = null,
    val sequence: Long = 0,
)

@Serializable
private data class SessionWindowRuntimeEventDto(
    val type: String,
    val runtimeState: SessionRuntimeStateDto? = null,
)

@Serializable
private data class SessionWindowAgentMessageEventDto(
    val type: String,
    val agentMessage: SessionAgentMessageDto? = null,
)

@Serializable
private data class SessionStreamErrorDto(
    val type: String? = null,
    val threadId: String? = null,
    val message: String = "",
)

@Serializable
private data class SessionStreamClientEventReportDto(
    val eventType: String,
    val threadId: String,
    val deviceName: String,
    val appVersion: String,
    val reconnectAttempt: Int = 0,
    val reason: String? = null,
    val detail: String? = null,
    val statusCode: Int? = null,
    val retryable: Boolean? = null,
    val connectedMs: Long? = null,
    val nextRetryDelayMs: Long? = null,
    val receivedAgentMessage: Boolean = false,
    val firstEventType: String? = null,
)

@Serializable
private data class QuotaDto(
    val source: String,
    val fresh: Boolean,
    val status: String? = null,
    val planType: String? = null,
    val fiveHour: WindowDto? = null,
    val weekly: WindowDto? = null,
)

@Serializable
private data class WindowDto(
    val usedPercent: Float,
    val remainingPercent: Float,
    val resetAt: Long? = null,
)

private fun String?.toQuotaStatus(fresh: Boolean, hasCachedWindow: Boolean): QuotaStatus {
    return when (this?.trim()?.lowercase(Locale.US)) {
        "ok" -> QuotaStatus.Ok
        "stale" -> QuotaStatus.Stale
        "unavailable" -> QuotaStatus.Unavailable
        else -> when {
            fresh -> QuotaStatus.Ok
            hasCachedWindow -> QuotaStatus.Stale
            else -> QuotaStatus.Unavailable
        }
    }
}

@Serializable
private data class HeatmapDto(
    val timezone: String = "",
    val generatedAt: String? = null,
    val peakHourStart: String? = null,
    val buckets: List<HeatmapBucketDto>? = null,
)

@Serializable
private data class HeatmapBucketDto(
    val hourStart: String? = null,
    val inputTokens: Long = 0,
    val cachedInputTokens: Long = 0,
    val outputTokens: Long = 0,
    val reasoningOutputTokens: Long = 0,
    val totalTokens: Long = 0,
    val activeThreads: Int = 0,
)

@Serializable
private data class Heatmap7dDto(
    val timezone: String = "",
    val generatedAt: String? = null,
    val startDate: String = "",
    val endDate: String = "",
    val peakTokens: Long = 0,
    val days: List<Heatmap7dDayDto>? = null,
)

@Serializable
private data class Heatmap7dDayDto(
    val date: String = "",
    val totalTokens: Long = 0,
    val hours: List<Long>? = null,
)

@Serializable
private data class DailyUsageDto(
    val generatedAt: String? = null,
    val totalTokens: Long = 0,
    val inputTokens: Long = 0,
    val cachedInputTokens: Long = 0,
    val outputTokens: Long = 0,
    val reasoningOutputTokens: Long = 0,
    val activeSessions: Int = 0,
    val estimatedValueUsd: Double? = null,
    val estimatedValueLabel: String? = null,
    val pricingDate: String? = null,
    val pricingSourceUrl: String? = null,
    val pricingUnavailableReason: String? = null,
    val modelShares: List<DailyUsageModelShareDto>? = null,
)

@Serializable
private data class DailyUsageModelShareDto(
    val model: String = "",
    val tokens: Long = 0,
    val sharePercent: Double = 0.0,
)

@Serializable
private data class DailyTrend30dDto(
    val timezone: String = "",
    val generatedAt: String? = null,
    val startDate: String = "",
    val endDate: String = "",
    val totalTokens: Long = 0,
    val averageTokens: Long = 0,
    val peakTokens: Long = 0,
    val estimatedValueUsd: Double? = null,
    val estimatedValueLabel: String? = null,
    val days: List<DailyTrendDayDto>? = null,
)

@Serializable
private data class DailyTrendDayDto(
    val date: String = "",
    val totalTokens: Long = 0,
)

@Serializable
private data class SessionDto(
    val threadId: String,
    val title: String = "",
    val updatedAt: String? = null,
    val model: String = "",
    val reasoningEffort: String = "",
    val tokensUsedTotal: Long = 0,
    val contextUsedTokens: Long = 0,
    val contextWindow: Long = 0,
    val contextPressurePercent: Int = 0,
    val contextCompactThresholdTokens: Long? = null,
    val contextCompactThresholdPercent: Int? = null,
    val contextCompaction: ContextCompactionDto? = null,
    val lastActiveAgoMinutes: Int = 0,
)

@Serializable
private data class ContextCompactionDto(
    val trigger: String = "",
    val startedAt: String? = null,
    val updatedAt: String? = null,
    val turnId: String? = null,
)
