package ai.openwatcher.watchapp.data

import java.io.IOException
import java.time.Instant
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import okhttp3.Call
import okhttp3.Callback
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Protocol
import okhttp3.Request
import okhttp3.Response
import okhttp3.ResponseBody
import okhttp3.ResponseBody.Companion.toResponseBody
import okio.Buffer
import okio.BufferedSource
import okio.Source
import okio.Timeout
import okio.buffer
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticEventLogger
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticLevel

@OptIn(kotlinx.coroutines.ExperimentalCoroutinesApi::class)
class WatcherScreenshotUploadApiTest {
    @Test
    fun uploadScreenshot_postsPngWithTokenAndMetadataHeaders() = runTest {
        val callFactory = CapturingCallFactory(
            responseCode = 200,
            responseBody = """{"ok":true,"filename":"watch.png"}""",
        )
        val api = HttpWatcherApi(
            baseUrl = "https://watcher.example",
            callFactory = callFactory,
        )
        val pngBytes = byteArrayOf(0x01, 0x02, 0x03)

        val result = api.uploadScreenshot(
            token = "device-token",
            request = ScreenshotUploadRequest(
                pngBytes = pngBytes,
                deviceName = "Xiaomi Watch 5",
                appVersion = "0.7.4",
            ),
        )

        assertEquals(ScreenshotUploadResult.Success("watch.png"), result)
        val request = requireNotNull(callFactory.lastRequest)
        assertEquals("POST", request.method)
        assertEquals("https://watcher.example/api/screenshots", request.url.toString())
        assertEquals("device-token", request.header("X-OpenWatcher-Token"))
        assertEquals("Xiaomi Watch 5", request.header("X-OpenWatcher-Device-Name"))
        assertEquals("0.7.4", request.header("X-OpenWatcher-App-Version"))
        assertEquals("image/png", request.body?.contentType()?.toString())

        val body = Buffer()
        request.body?.writeTo(body)
        assertTrue(body.readByteArray().contentEquals(pngBytes))
    }

    @Test
    fun uploadScreenshot_mapsUnauthorized() = runTest {
        val api = HttpWatcherApi(
            baseUrl = "https://watcher.example",
            callFactory = CapturingCallFactory(responseCode = 401, responseBody = """{"error":"unauthorized"}"""),
        )

        val result = api.uploadScreenshot(
            token = "bad-token",
            request = ScreenshotUploadRequest(
                pngBytes = byteArrayOf(0x01),
                deviceName = "watch",
                appVersion = "0.7.4",
            ),
        )

        assertEquals(ScreenshotUploadResult.Unauthorized, result)
    }

    @Test
    fun uploadDiagnostics_postsGzipWithHeadersAndProgress() = runTest {
        val callFactory = CapturingCallFactory(
            responseCode = 200,
            responseBody = """{"ok":true,"diagnosticId":"diag-20260606","receivedAt":"2026-06-06T01:20:00Z"}""",
        )
        val api = HttpWatcherApi(
            baseUrl = "https://watcher.example",
            callFactory = callFactory,
        )
        val gzipBytes = ByteArray(20 * 1024) { 0x2A }
        var lastProgress = 0L

        val result = api.uploadDiagnostics(
            token = "device-token",
            request = DiagnosticUploadRequest(
                gzipBytes = gzipBytes,
                deviceName = "Xiaomi Watch 5",
                appVersion = "0.7.4",
                startedAt = Instant.parse("2026-06-06T01:10:00Z"),
                hours = 24,
            ),
            onProgress = { bytesUploaded, _ ->
                lastProgress = bytesUploaded
            },
        )

        assertEquals(
            DiagnosticUploadResult.Success(
                diagnosticId = "diag-20260606",
                receivedAt = Instant.parse("2026-06-06T01:20:00Z"),
            ),
            result,
        )
        val request = requireNotNull(callFactory.lastRequest)
        assertEquals("POST", request.method)
        assertEquals("https://watcher.example/api/diagnostics", request.url.toString())
        assertEquals("device-token", request.header("X-OpenWatcher-Token"))
        assertEquals("Xiaomi Watch 5", request.header("X-OpenWatcher-Device-Name"))
        assertEquals("0.7.4", request.header("X-OpenWatcher-App-Version"))
        assertEquals("2026-06-06T01:10:00Z", request.header("X-OpenWatcher-Diagnostic-Started-At"))
        assertEquals("24", request.header("X-OpenWatcher-Diagnostic-Hours"))
        assertEquals("application/gzip", request.body?.contentType()?.toString())

        val body = Buffer()
        request.body?.writeTo(body)
        assertTrue(body.readByteArray().contentEquals(gzipBytes))
        assertEquals(gzipBytes.size.toLong(), lastProgress)
    }

    @Test
    fun streamSessionAgentMessages_logsLocalStreamStateOnConnectAndClose() = runTest {
        val logger = RecordingDiagnosticEventLogger()
        val api = HttpWatcherApi(
            baseUrl = "https://watcher.example",
            callFactory = CapturingCallFactory(responseCode = 200, responseBody = """{"ok":true}"""),
            streamCallFactory = ProvidedCallFactory(
                StaticResponseCall(
                    request = Request.Builder().url("https://watcher.example").build(),
                    responseCode = 200,
                    responseBody = "event: heartbeat\n\n",
                    contentType = "text/event-stream",
                ),
            ),
            diagnosticEventLogger = logger,
        )

        val events = api.streamSessionAgentMessages(
            token = "device-token",
            threadId = "session-1",
            includeMessages = true,
        ).toList()
        advanceUntilIdle()

        assertEquals(2, events.size)
        assertEquals(listOf("connect_requested", "connected", "stream_closed"), logger.actions())
    }

    @Test
    fun streamSessionAgentMessages_logsUnauthorizedAndHttpFailure() = runTest {
        val unauthorizedLogger = RecordingDiagnosticEventLogger()
        val unauthorizedApi = HttpWatcherApi(
            baseUrl = "https://watcher.example",
            callFactory = CapturingCallFactory(responseCode = 200, responseBody = """{"ok":true}"""),
            streamCallFactory = ProvidedCallFactory(
                StaticResponseCall(
                    request = Request.Builder().url("https://watcher.example").build(),
                    responseCode = 401,
                    responseBody = """{"error":"unauthorized"}""",
                ),
            ),
            diagnosticEventLogger = unauthorizedLogger,
        )
        unauthorizedApi.streamSessionAgentMessages("device-token", "session-1", false).toList()
        advanceUntilIdle()
        assertEquals(listOf("connect_requested", "unauthorized"), unauthorizedLogger.actions())

        val httpFailureLogger = RecordingDiagnosticEventLogger()
        val httpFailureApi = HttpWatcherApi(
            baseUrl = "https://watcher.example",
            callFactory = CapturingCallFactory(responseCode = 200, responseBody = """{"ok":true}"""),
            streamCallFactory = ProvidedCallFactory(
                StaticResponseCall(
                    request = Request.Builder().url("https://watcher.example").build(),
                    responseCode = 503,
                    responseBody = """{"error":"busy"}""",
                ),
            ),
            diagnosticEventLogger = httpFailureLogger,
        )
        httpFailureApi.streamSessionAgentMessages("device-token", "session-1", false).toList()
        advanceUntilIdle()
        assertEquals(listOf("connect_requested", "http_failure"), httpFailureLogger.actions())
    }

    @Test
    fun streamSessionAgentMessages_logsReadFailed() = runTest {
        val logger = RecordingDiagnosticEventLogger()
        val api = HttpWatcherApi(
            baseUrl = "https://watcher.example",
            callFactory = CapturingCallFactory(responseCode = 200, responseBody = """{"ok":true}"""),
            streamCallFactory = ProvidedCallFactory(
                object : Call {
                    private var canceled = false

                    override fun request(): Request = Request.Builder().url("https://watcher.example").build()

                    override fun execute(): Response {
                        return Response.Builder()
                            .request(request())
                            .protocol(Protocol.HTTP_1_1)
                            .code(200)
                            .message("OK")
                            .body(ThrowingResponseBody("text/event-stream", "broken stream"))
                            .build()
                    }

                    override fun enqueue(responseCallback: Callback) {
                        responseCallback.onResponse(this, execute())
                    }

                    override fun cancel() {
                        canceled = true
                    }

                    override fun isExecuted(): Boolean = true

                    override fun isCanceled(): Boolean = canceled

                    override fun clone(): Call = this

                    override fun timeout(): Timeout = Timeout.NONE
                },
            ),
            diagnosticEventLogger = logger,
        )

        api.streamSessionAgentMessages("device-token", "session-1", true).toList()
        advanceUntilIdle()

        assertEquals(listOf("connect_requested", "connected", "read_failed"), logger.actions())
    }

    @Test
    fun streamSessionWindow_logsLocalStreamStateOnConnectAndClose() = runTest {
        val logger = RecordingDiagnosticEventLogger()
        val api = HttpWatcherApi(
            baseUrl = "https://watcher.example",
            callFactory = CapturingCallFactory(responseCode = 200, responseBody = """{"ok":true}"""),
            streamCallFactory = ProvidedCallFactory(
                StaticResponseCall(
                    request = Request.Builder().url("https://watcher.example").build(),
                    responseCode = 200,
                    responseBody = "event: heartbeat\n\n",
                    contentType = "text/event-stream",
                ),
            ),
            diagnosticEventLogger = logger,
        )

        val events = api.streamSessionWindow(
            token = "device-token",
            limit = 5,
            preferredOrder = listOf("thread-a", "thread-b"),
        ).toList()
        advanceUntilIdle()

        assertEquals(2, events.size)
        assertEquals(
            listOf("connect_requested", "connected", "stream_closed"),
            logger.actions("session_window_stream"),
        )
    }

    @Test
    fun streamSessionWindow_logsUnauthorizedHttpFailureAndReadFailed() = runTest {
        val unauthorizedLogger = RecordingDiagnosticEventLogger()
        val unauthorizedApi = HttpWatcherApi(
            baseUrl = "https://watcher.example",
            callFactory = CapturingCallFactory(responseCode = 200, responseBody = """{"ok":true}"""),
            streamCallFactory = ProvidedCallFactory(
                StaticResponseCall(
                    request = Request.Builder().url("https://watcher.example").build(),
                    responseCode = 401,
                    responseBody = """{"error":"unauthorized"}""",
                ),
            ),
            diagnosticEventLogger = unauthorizedLogger,
        )
        unauthorizedApi.streamSessionWindow("device-token", 5, emptyList()).toList()
        advanceUntilIdle()
        assertEquals(listOf("connect_requested", "unauthorized"), unauthorizedLogger.actions("session_window_stream"))

        val httpFailureLogger = RecordingDiagnosticEventLogger()
        val httpFailureApi = HttpWatcherApi(
            baseUrl = "https://watcher.example",
            callFactory = CapturingCallFactory(responseCode = 200, responseBody = """{"ok":true}"""),
            streamCallFactory = ProvidedCallFactory(
                StaticResponseCall(
                    request = Request.Builder().url("https://watcher.example").build(),
                    responseCode = 503,
                    responseBody = """{"error":"busy"}""",
                ),
            ),
            diagnosticEventLogger = httpFailureLogger,
        )
        httpFailureApi.streamSessionWindow("device-token", 5, emptyList()).toList()
        advanceUntilIdle()
        assertEquals(listOf("connect_requested", "http_failure"), httpFailureLogger.actions("session_window_stream"))

        val readFailedLogger = RecordingDiagnosticEventLogger()
        val readFailedApi = HttpWatcherApi(
            baseUrl = "https://watcher.example",
            callFactory = CapturingCallFactory(responseCode = 200, responseBody = """{"ok":true}"""),
            streamCallFactory = ProvidedCallFactory(
                object : Call {
                    private var canceled = false

                    override fun request(): Request = Request.Builder().url("https://watcher.example").build()

                    override fun execute(): Response {
                        return Response.Builder()
                            .request(request())
                            .protocol(Protocol.HTTP_1_1)
                            .code(200)
                            .message("OK")
                            .body(ThrowingResponseBody("text/event-stream", "broken stream"))
                            .build()
                    }

                    override fun enqueue(responseCallback: Callback) {
                        responseCallback.onResponse(this, execute())
                    }

                    override fun cancel() {
                        canceled = true
                    }

                    override fun isExecuted(): Boolean = true

                    override fun isCanceled(): Boolean = canceled

                    override fun clone(): Call = this

                    override fun timeout(): Timeout = Timeout.NONE
                },
            ),
            diagnosticEventLogger = readFailedLogger,
        )
        readFailedApi.streamSessionWindow("device-token", 5, emptyList()).toList()
        advanceUntilIdle()
        assertEquals(listOf("connect_requested", "connected", "read_failed"), readFailedLogger.actions("session_window_stream"))
    }

    private class CapturingCallFactory(
        private val responseCode: Int,
        private val responseBody: String,
    ) : Call.Factory {
        var lastRequest: Request? = null
            private set

        override fun newCall(request: Request): Call {
            lastRequest = request
            return StaticResponseCall(request, responseCode, responseBody)
        }
    }

    private class ProvidedCallFactory(
        private val call: Call,
    ) : Call.Factory {
        override fun newCall(request: Request): Call = call
    }

    private class StaticResponseCall(
        private val request: Request,
        private val responseCode: Int,
        private val responseBody: String,
        private val contentType: String = "application/json",
    ) : Call {
        private var executed = false
        private var canceled = false

        override fun request(): Request = request

        override fun execute(): Response {
            if (canceled) {
                throw IOException("canceled")
            }
            executed = true
            return Response.Builder()
                .request(request)
                .protocol(Protocol.HTTP_1_1)
                .code(responseCode)
                .message("OK")
                .body(responseBody.toResponseBody(contentType.toMediaType()))
                .build()
        }

        override fun enqueue(responseCallback: Callback) {
            executed = true
            responseCallback.onResponse(this, execute())
        }

        override fun cancel() {
            canceled = true
        }

        override fun isExecuted(): Boolean = executed

        override fun isCanceled(): Boolean = canceled

        override fun clone(): Call = StaticResponseCall(request, responseCode, responseBody)

        override fun timeout(): Timeout = Timeout.NONE
    }

    private class ThrowingResponseBody(
        private val contentTypeValue: String,
        private val message: String,
    ) : ResponseBody() {
        override fun contentType() = contentTypeValue.toMediaType()

        override fun contentLength(): Long = -1L

        override fun source(): BufferedSource {
            return object : Source {
                override fun read(sink: Buffer, byteCount: Long): Long {
                    throw IOException(message)
                }

                override fun timeout(): Timeout = Timeout.NONE

                override fun close() = Unit
            }.buffer()
        }
    }

    private class RecordingDiagnosticEventLogger : DiagnosticEventLogger {
        private val entries = mutableListOf<Map<String, Any?>>()

        override fun newTraceId(prefix: String): String = "$prefix-test"

        override suspend fun log(
            event: String,
            level: DiagnosticLevel,
            traceId: String?,
            screen: String?,
            fields: Map<String, Any?>,
        ) {
            if (event == "stream_state") {
                synchronized(entries) {
                    entries += fields
                }
            }
        }

        fun actions(targetName: String? = null): List<String> {
            return synchronized(entries) {
                entries.mapNotNull { entry ->
                    @Suppress("UNCHECKED_CAST")
                    val target = (entry["target"] as? Map<String, Any?>)?.get("name") as? String
                    if (targetName != null && target != targetName) {
                        return@mapNotNull null
                    }
                    @Suppress("UNCHECKED_CAST")
                    (entry["state"] as? Map<String, Any?>)?.get("action") as? String
                }
            }
        }
    }
}
