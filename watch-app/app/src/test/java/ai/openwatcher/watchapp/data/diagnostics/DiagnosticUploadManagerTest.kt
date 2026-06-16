package ai.openwatcher.watchapp.data.diagnostics

import java.time.Instant
import java.time.ZoneId
import java.util.concurrent.atomic.AtomicLong
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.emptyFlow
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import ai.openwatcher.watchapp.data.DiagnosticUploadRequest
import ai.openwatcher.watchapp.data.DiagnosticUploadResult
import ai.openwatcher.watchapp.data.HealthCheckResult
import ai.openwatcher.watchapp.data.KeyValueStore
import ai.openwatcher.watchapp.data.ScreenshotUploadRequest
import ai.openwatcher.watchapp.data.ScreenshotUploadResult
import ai.openwatcher.watchapp.data.SessionStreamClientEventReport
import ai.openwatcher.watchapp.data.SessionStreamEvent
import ai.openwatcher.watchapp.data.SessionWindowStreamEvent
import ai.openwatcher.watchapp.data.StatusFetchResult
import ai.openwatcher.watchapp.data.StatusStreamEvent
import ai.openwatcher.watchapp.data.WatcherApi

class DiagnosticUploadManagerTest {
    @Test
    fun requestUpload_successDeletesPendingPackageAndStoresResult() = runBlocking {
        val dispatcher = Dispatchers.IO
        val monotonicNowMs = AtomicLong(0L)
        val wallClockNow = AtomicLong(Instant.parse("2026-06-06T01:10:00Z").toEpochMilli())
        val api = FakeDiagnosticWatcherApi(
            nextDiagnosticResult = DiagnosticUploadResult.Success(
                diagnosticId = "diag-20260606-success",
                receivedAt = Instant.parse("2026-06-06T01:12:00Z"),
            ),
        )
        val manager = newManager(
            api = api,
            dispatcher = dispatcher,
            wallClockNow = wallClockNow,
            monotonicNowMs = monotonicNowMs,
        )
        try {
            manager.uploadNowForTesting("device-token")

            assertEquals(1, api.diagnosticRequests.size)
            assertEquals("diag-20260606-success", manager.state.value.lastSuccess?.diagnosticId)
            assertEquals(api.diagnosticRequests.first().startedAt, manager.state.value.lastSuccess?.diagnosticCreatedAt)
            assertNull(manager.state.value.pendingPackage)
            assertFalse(manager.state.value.hasPendingPackage)
        } finally {
            manager.close()
        }
    }

    @Test
    fun requestUpload_failureKeepsPendingPackageAndRetryReusesExistingPackage() = runBlocking {
        val dispatcher = Dispatchers.IO
        val monotonicNowMs = AtomicLong(0L)
        val wallClockNow = AtomicLong(Instant.parse("2026-06-06T01:10:00Z").toEpochMilli())
        val api = FakeDiagnosticWatcherApi(
            nextDiagnosticResult = DiagnosticUploadResult.NetworkFailure("服务连接失败"),
        )
        val manager = newManager(
            api = api,
            dispatcher = dispatcher,
            wallClockNow = wallClockNow,
            monotonicNowMs = monotonicNowMs,
        )
        try {
            manager.uploadNowForTesting("device-token")

            val failedState = manager.state.value
            val pending = failedState.pendingPackage
            assertNotNull(pending)
            assertEquals(DiagnosticUploadPhase.Failed, failedState.phase)
            assertTrue(failedState.packageSizeBytes ?: 0L > 0L)

            api.nextDiagnosticResult = DiagnosticUploadResult.Success(
                diagnosticId = "diag-20260606-retry",
                receivedAt = Instant.parse("2026-06-06T01:13:00Z"),
            )
            wallClockNow.set(Instant.parse("2026-06-06T01:20:00Z").toEpochMilli())

            manager.uploadNowForTesting("device-token")

            assertEquals(2, api.diagnosticRequests.size)
            assertEquals(
                api.diagnosticRequests.first().startedAt,
                api.diagnosticRequests.last().startedAt,
            )
            assertEquals("diag-20260606-retry", manager.state.value.lastSuccess?.diagnosticId)
            assertEquals(api.diagnosticRequests.first().startedAt, manager.state.value.lastSuccess?.diagnosticCreatedAt)
            assertNull(manager.state.value.pendingPackage)
        } finally {
            manager.close()
        }
    }

    @Test
    fun clearPendingPackage_removesFailedPackageAndResetsState() = runBlocking {
        val dispatcher = Dispatchers.IO
        val monotonicNowMs = AtomicLong(0L)
        val wallClockNow = AtomicLong(Instant.parse("2026-06-06T01:10:00Z").toEpochMilli())
        val api = FakeDiagnosticWatcherApi(
            nextDiagnosticResult = DiagnosticUploadResult.NetworkFailure("服务连接失败"),
        )
        val manager = newManager(
            api = api,
            dispatcher = dispatcher,
            wallClockNow = wallClockNow,
            monotonicNowMs = monotonicNowMs,
        )
        try {
            manager.uploadNowForTesting("device-token")
            assertTrue(manager.state.value.hasPendingPackage)

            manager.clearPendingPackageNowForTesting()

            assertEquals(DiagnosticUploadPhase.Idle, manager.state.value.phase)
            assertFalse(manager.state.value.hasPendingPackage)
            assertNull(manager.state.value.pendingPackage)
            assertNull(manager.state.value.packageSizeBytes)
        } finally {
            manager.close()
        }
    }

    private fun newManager(
        api: FakeDiagnosticWatcherApi,
        dispatcher: kotlinx.coroutines.CoroutineDispatcher,
        wallClockNow: AtomicLong,
        monotonicNowMs: AtomicLong,
    ): DiagnosticUploadManager {
        val eventDirectory = java.nio.file.Files.createTempDirectory("diagnostic-events").toFile()
        val pendingDirectory = java.nio.file.Files.createTempDirectory("diagnostic-pending").toFile()
        val uiStateHolder = DiagnosticUiStateHolder()
        val runtimeContext = DiagnosticRuntimeContext(baseUrl = "https://watcher.example")
        runtimeContext.updateHasPaired(true)
        val eventStore = DiagnosticEventStore(
            directory = eventDirectory,
            ioDispatcher = dispatcher,
            clock = { Instant.ofEpochMilli(wallClockNow.get()) },
            zoneId = ZoneId.of("UTC"),
        )
        val eventLogger = StructuredDiagnosticEventLogger(
            store = eventStore,
            runtimeContext = runtimeContext,
            uiStateProvider = uiStateHolder::current,
            deviceInfo = DiagnosticDeviceInfo(
                manufacturer = "Xiaomi",
                model = "Watch 5",
                sdkInt = 34,
                screenWidthPx = 466,
                screenHeightPx = 466,
                densityDpi = 320,
                fontScale = 1f,
                isRound = true,
                smallestWidthDp = 192,
            ),
            appInfo = DiagnosticAppInfo(
                versionName = "0.13.2",
                versionCode = 53,
                buildType = "debug",
            ),
            clock = { Instant.ofEpochMilli(wallClockNow.get()) },
        )
        return DiagnosticUploadManager(
            api = api,
            eventStore = eventStore,
            eventLogger = eventLogger,
            snapshotCollector = DiagnosticSnapshotCollector(
                uiStateProvider = uiStateHolder::current,
                eventLogger = eventLogger,
            ),
            stateStore = DiagnosticUploadPreferenceStore(FakeKeyValueStore()),
            pendingDirectory = pendingDirectory,
            deviceName = "Xiaomi Watch 5",
            appVersion = "0.13.2",
            ioDispatcher = dispatcher,
            clock = { Instant.ofEpochMilli(wallClockNow.get()) },
            monotonicNowMs = { monotonicNowMs.getAndAdd(300L) },
        )
    }

    private class FakeDiagnosticWatcherApi(
        var nextDiagnosticResult: DiagnosticUploadResult,
    ) : WatcherApi {
        val diagnosticRequests = mutableListOf<DiagnosticUploadRequest>()

        override suspend fun fetchStatus(token: String): StatusFetchResult {
            return StatusFetchResult.NetworkFailure("unused")
        }

        override suspend fun checkHealth(): HealthCheckResult {
            return HealthCheckResult.Offline("unused")
        }

        override suspend fun uploadScreenshot(
            token: String,
            request: ScreenshotUploadRequest,
        ): ScreenshotUploadResult {
            return ScreenshotUploadResult.NetworkFailure("unused")
        }

        override suspend fun uploadDiagnostics(
            token: String,
            request: DiagnosticUploadRequest,
            onProgress: (bytesUploaded: Long, totalBytes: Long) -> Unit,
        ): DiagnosticUploadResult {
            diagnosticRequests += request
            onProgress(request.gzipBytes.size.toLong() / 2L, request.gzipBytes.size.toLong())
            onProgress(request.gzipBytes.size.toLong(), request.gzipBytes.size.toLong())
            return nextDiagnosticResult
        }

        override fun streamStatus(token: String, includeDailyTrend30d: Boolean): Flow<StatusStreamEvent> {
            return emptyFlow()
        }

        override fun streamSessionAgentMessages(
            token: String,
            threadId: String,
            includeMessages: Boolean,
        ): Flow<SessionStreamEvent> {
            return emptyFlow()
        }

        override fun streamSessionWindow(
            token: String,
            limit: Int,
            preferredOrder: List<String>,
        ): Flow<SessionWindowStreamEvent> {
            return emptyFlow()
        }

        override suspend fun reportSessionStreamClientEvent(token: String, report: SessionStreamClientEventReport) = Unit
    }

    private class FakeKeyValueStore : KeyValueStore {
        private val values = mutableMapOf<String, String>()

        override fun getString(key: String): String? = values[key]

        override fun putString(key: String, value: String) {
            values[key] = value
        }

        override fun remove(key: String) {
            values.remove(key)
        }
    }
}
