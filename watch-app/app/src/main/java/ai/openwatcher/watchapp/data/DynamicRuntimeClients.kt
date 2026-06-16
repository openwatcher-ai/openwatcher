package ai.openwatcher.watchapp.data

import android.content.Context
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.serialization.json.Json
import okhttp3.Call
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticEventLogger
import ai.openwatcher.watchapp.data.diagnostics.NoOpDiagnosticEventLogger

class DynamicWatcherApi(
    private val baseUrlProvider: () -> String,
    private val callFactory: Call.Factory,
    private val streamCallFactory: Call.Factory = callFactory,
    private val json: Json = Json { explicitNulls = false },
    private val parser: WatcherJsonParser = WatcherJsonParser(),
    private val diagnosticEventLogger: DiagnosticEventLogger = NoOpDiagnosticEventLogger,
) : WatcherApi {
    private fun delegate(): WatcherApi {
        return HttpWatcherApi(
            baseUrl = baseUrlProvider(),
            callFactory = callFactory,
            streamCallFactory = streamCallFactory,
            json = json,
            parser = parser,
            diagnosticEventLogger = diagnosticEventLogger,
        )
    }

    override suspend fun fetchStatus(token: String): StatusFetchResult = delegate().fetchStatus(token)

    override suspend fun checkHealth(): HealthCheckResult = delegate().checkHealth()

    override suspend fun uploadScreenshot(token: String, request: ScreenshotUploadRequest): ScreenshotUploadResult {
        return delegate().uploadScreenshot(token, request)
    }

    override suspend fun uploadDiagnostics(
        token: String,
        request: DiagnosticUploadRequest,
        onProgress: (bytesUploaded: Long, totalBytes: Long) -> Unit,
    ): DiagnosticUploadResult {
        return delegate().uploadDiagnostics(token, request, onProgress)
    }

    override fun streamStatus(token: String, includeDailyTrend30d: Boolean) = delegate().streamStatus(token, includeDailyTrend30d)

    override fun streamSessionAgentMessages(token: String, threadId: String, includeMessages: Boolean) =
        delegate().streamSessionAgentMessages(token, threadId, includeMessages)

    override fun streamSessionWindow(token: String, limit: Int, preferredOrder: List<String>) =
        delegate().streamSessionWindow(token, limit, preferredOrder)

    override suspend fun reportSessionStreamClientEvent(token: String, report: SessionStreamClientEventReport) {
        delegate().reportSessionStreamClientEvent(token, report)
    }
}

class DynamicWatcherApkUpdateManager(
    private val context: Context,
    private val baseUrlProvider: () -> String,
    private val channelProvider: () -> AppUpdateChannel,
    private val deviceTokenProvider: () -> String?,
    private val currentVersionCode: Int,
    private val callFactory: Call.Factory,
    private val betaPrimaryMetadataUrl: String,
    private val betaBackupMetadataUrl: String,
    private val ioDispatcher: CoroutineDispatcher,
    private val json: Json = Json { ignoreUnknownKeys = true },
    private val diagnosticEventLogger: DiagnosticEventLogger = NoOpDiagnosticEventLogger,
) : WatcherApkUpdateManager {
    private fun delegate(): WatcherApkUpdateManager {
        return AndroidWatcherApkUpdateManager(
            context = context,
            baseUrl = baseUrlProvider(),
            channel = channelProvider(),
            deviceToken = deviceTokenProvider(),
            currentVersionCode = currentVersionCode,
            callFactory = callFactory,
            betaPrimaryMetadataUrl = betaPrimaryMetadataUrl,
            betaBackupMetadataUrl = betaBackupMetadataUrl,
            ioDispatcher = ioDispatcher,
            json = json,
            diagnosticEventLogger = diagnosticEventLogger,
        )
    }

    override fun canRequestPackageInstalls(): Boolean = delegate().canRequestPackageInstalls()

    override fun openInstallPermissionSettings(): Boolean = delegate().openInstallPermissionSettings()

    override suspend fun fetchLatestUpdate(): WatcherApkUpdateCheckResult = delegate().fetchLatestUpdate()

    override suspend fun hasCachedUpdate(update: WatcherApkUpdate): Boolean = delegate().hasCachedUpdate(update)

    override suspend fun startInstallFromCache(update: WatcherApkUpdate): WatcherApkInstallResult = delegate().startInstallFromCache(update)

    override suspend fun downloadAndStartInstall(
        update: WatcherApkUpdate,
        onProgress: (WatcherApkUpdateProgress) -> Unit,
    ): WatcherApkInstallResult {
        return delegate().downloadAndStartInstall(update, onProgress)
    }
}
