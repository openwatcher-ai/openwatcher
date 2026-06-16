package ai.openwatcher.watchapp.ui

import androidx.compose.ui.graphics.Color
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewModelScope
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.Locale
import kotlin.math.abs
import kotlin.math.min
import kotlin.math.roundToInt
import kotlin.random.Random
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.catch
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
import ai.openwatcher.watchapp.data.AppUpdateChannel
import ai.openwatcher.watchapp.data.AppUpdateNote
import ai.openwatcher.watchapp.data.AppUpdatePreferenceStore
import ai.openwatcher.watchapp.data.AppUpdatePreferences
import ai.openwatcher.watchapp.data.AppUpdateVersionNotes
import ai.openwatcher.watchapp.data.ContextCompactionSnapshot
import ai.openwatcher.watchapp.data.DebugDemoPreferenceStore
import ai.openwatcher.watchapp.data.DebugDemoScenario
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticEventLogger
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticLevel
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticRuntimeContext
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticUploadManager
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticUploadPhase
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticUploadStatus
import ai.openwatcher.watchapp.data.diagnostics.NoOpDiagnosticEventLogger
import ai.openwatcher.watchapp.data.DailyTrend30dSnapshot
import ai.openwatcher.watchapp.data.DailyTrendStore
import ai.openwatcher.watchapp.data.DeviceTokenRepository
import ai.openwatcher.watchapp.data.DailyUsageSnapshot
import ai.openwatcher.watchapp.data.EndpointSelection
import ai.openwatcher.watchapp.data.EndpointSelector
import ai.openwatcher.watchapp.data.Heatmap24hSnapshot
import ai.openwatcher.watchapp.data.Heatmap7dSnapshot
import ai.openwatcher.watchapp.data.HealthCheckResult
import ai.openwatcher.watchapp.data.NoOpWatcherApkUpdateManager
import ai.openwatcher.watchapp.data.PairingStateStore
import ai.openwatcher.watchapp.data.BootstrapRequest
import ai.openwatcher.watchapp.data.NoOpScreenshotUploadQueue
import ai.openwatcher.watchapp.data.QuotaStatus
import ai.openwatcher.watchapp.data.QuotaSnapshot
import ai.openwatcher.watchapp.data.QuotaWindow
import ai.openwatcher.watchapp.data.SessionAgentMessage
import ai.openwatcher.watchapp.data.SessionRuntimeLifecycle
import ai.openwatcher.watchapp.data.SessionRuntimePhase
import ai.openwatcher.watchapp.data.SessionRuntimeState
import ai.openwatcher.watchapp.data.SessionStreamClientEventReport
import ai.openwatcher.watchapp.data.SessionStreamClientEventType
import ai.openwatcher.watchapp.data.SessionSnapshot
import ai.openwatcher.watchapp.data.SessionWindowStreamEvent
import ai.openwatcher.watchapp.data.SessionWindowEntry
import ai.openwatcher.watchapp.data.SessionWindowSnapshot
import ai.openwatcher.watchapp.data.SessionStreamEvent
import ai.openwatcher.watchapp.data.SessionStreamFailureReason
import ai.openwatcher.watchapp.data.ScreenshotUploadRequest
import ai.openwatcher.watchapp.data.ScreenshotUploadQueue
import ai.openwatcher.watchapp.data.ScreenshotUploadResult
import ai.openwatcher.watchapp.data.ServerConfig
import ai.openwatcher.watchapp.data.ServerConfigRepository
import ai.openwatcher.watchapp.data.ServerConfigSource
import ai.openwatcher.watchapp.data.ServerEndpoint
import ai.openwatcher.watchapp.data.SessionDetailsWindowCacheSnapshot
import ai.openwatcher.watchapp.data.SessionDetailsWindowStore
import ai.openwatcher.watchapp.data.StatusFetchResult
import ai.openwatcher.watchapp.data.StatusSnapshotStore
import ai.openwatcher.watchapp.data.StatusStreamEvent
import ai.openwatcher.watchapp.data.StatusStreamFailureReason
import ai.openwatcher.watchapp.data.WatcherApi
import ai.openwatcher.watchapp.data.WatcherApkInstallResult
import ai.openwatcher.watchapp.data.WatcherApkUpdate
import ai.openwatcher.watchapp.data.WatcherApkUpdateCheckResult
import ai.openwatcher.watchapp.data.WatcherApkUpdateManager
import ai.openwatcher.watchapp.data.WatcherApkUpdateProgress
import ai.openwatcher.watchapp.data.WatchBootstrapCodeStore
import ai.openwatcher.watchapp.data.WatchBootstrapException
import ai.openwatcher.watchapp.data.WatchBootstrapGateway
import ai.openwatcher.watchapp.data.WatchBootstrapPollResult
import ai.openwatcher.watchapp.data.WatcherStatusSnapshot
import ai.openwatcher.watchapp.ui.home.HomeQuotaEasterEggCopyConfig
import ai.openwatcher.watchapp.ui.home.HomeQuotaTipPool

fun interface WatcherHapticController {
    fun vibrateSessionCompleted()

    fun vibrateScreenshotCaptured() {}
}

private const val HEATMAP_CURSOR_SETTLE_DELAY_MS = 500L
private const val HEATMAP_CURSOR_DEGREES_PER_FULL_ROTATION = 120f
private const val HEATMAP_ROTARY_UNITS_PER_FULL_ROTATION = 360f
private const val HEATMAP_ROTARY_DETENTS_PER_FULL_ROTATION = 6f
private const val HEATMAP_RING_SEGMENT_COUNT = 24f
private const val HEATMAP_TREND_TIP_HIDE_DURATION_MS = 3_000L
private const val HEATMAP_TREND_ROTARY_DEGREES_PER_BAR = 24f
private const val HEATMAP_TREND_ROTARY_DEGREES_PER_UNIT = 50f
private const val SCREENSHOT_UPLOAD_FEEDBACK_HIDE_MS = 1_800L
private const val SESSION_RUNTIME_DURATION_TICK_MS = 1_000L
private const val MAX_SESSION_RUNTIME_DURATION_SECONDS = 99L * 60L * 60L + 59L * 60L + 59L
private const val APP_UPDATE_CHECK_MIN_DURATION_MS = 1_000L
private const val APP_UPDATE_CHECK_TIMEOUT_MS = 10_000L
private const val APP_UPDATE_CHANNEL_TAP_TARGET = 5
private const val APP_UPDATE_CHANNEL_TAP_WINDOW_MS = 4_000L

internal fun heatmapRotaryDeltaToCursorDelta(rawDelta: Float): Float {
    if (rawDelta == 0f) {
        return 0f
    }
    if (abs(rawDelta) <= 1.5f) {
        val cursorDegrees = rawDelta * (HEATMAP_CURSOR_DEGREES_PER_FULL_ROTATION / HEATMAP_ROTARY_DETENTS_PER_FULL_ROTATION)
        return cursorDegrees / (360f / 24f)
    }
    val cursorDegrees = rawDelta * (HEATMAP_CURSOR_DEGREES_PER_FULL_ROTATION / HEATMAP_ROTARY_UNITS_PER_FULL_ROTATION)
    return cursorDegrees / (360f / 24f)
}

internal fun wrapHeatmapCursorPosition(position: Float, size: Int): Float {
    if (size <= 0) {
        return 0f
    }
    val raw = position % size.toFloat()
    return if (raw < 0f) raw + size.toFloat() else raw
}

internal fun roundHeatmapCursorPosition(position: Float, size: Int): Int {
    if (size <= 0) {
        return 0
    }
    val raw = wrapHeatmapCursorPosition(position, size).roundToInt() % size
    return if (raw < 0) raw + size else raw
}

internal fun heatmapTrendRotaryDeltaToDegrees(rawDelta: Float): Float {
    return rawDelta * HEATMAP_TREND_ROTARY_DEGREES_PER_UNIT
}

class WatcherViewModel(
    private val api: WatcherApi,
    private val appUpdateManager: WatcherApkUpdateManager = NoOpWatcherApkUpdateManager,
    private val appUpdatePreferenceStore: AppUpdatePreferenceStore,
    private val tokenRepository: DeviceTokenRepository,
    private val serverConfigRepository: ServerConfigRepository,
    private val endpointSelector: EndpointSelector,
    private val config: Config,
    private val pairingStateStore: PairingStateStore,
    private val dailyTrendStore: DailyTrendStore,
    private val statusSnapshotStore: StatusSnapshotStore,
    private val sessionDetailsWindowStore: SessionDetailsWindowStore,
    private val watchBootstrapClient: WatchBootstrapGateway? = null,
    private val watchBootstrapCodeStore: WatchBootstrapCodeStore? = null,
    private val screenshotUploadQueue: ScreenshotUploadQueue = NoOpScreenshotUploadQueue,
    private val diagnosticEventLogger: DiagnosticEventLogger = NoOpDiagnosticEventLogger,
    private val diagnosticUploadManager: DiagnosticUploadManager? = null,
    private val diagnosticUiStateSink: (AppUiState) -> Unit = {},
    private val diagnosticRuntimeContext: DiagnosticRuntimeContext? = null,
    private val debugStore: DebugDemoPreferenceStore? = null,
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
    private val autoPolling: Boolean = true,
    private val enableBootstrap: Boolean = true,
    private val appUpdateCheckMinDurationMs: Long = APP_UPDATE_CHECK_MIN_DURATION_MS,
    private val appUpdateCheckTimeoutMs: Long = APP_UPDATE_CHECK_TIMEOUT_MS,
    private val sessionStreamReconnectDelaysMs: List<Long> = DEFAULT_SESSION_STREAM_RECONNECT_DELAYS_MS,
    private val monotonicNowMs: () -> Long = { System.nanoTime() / 1_000_000L },
    private val wallClockNow: () -> Instant = { Instant.now() },
    private val homeQuotaTipRandomIndex: (Int) -> Int = { bound -> Random.Default.nextInt(bound) },
    private val homeQuotaTipHideDurationMs: Long = HOME_QUOTA_TIP_HIDE_DURATION_MS,
    private val hapticController: WatcherHapticController = WatcherHapticController {},
) : ViewModel() {
    data class Config(
        val baseUrl: String,
        val deviceName: String,
        val appVersion: String,
        val appVersionCode: Int,
        val debugToolsEnabled: Boolean,
    )

    private var appUpdatePreferences: AppUpdatePreferences = appUpdatePreferenceStore.read()
    private var currentServerConfig = serverConfigRepository.current(activeEnvironment())
    private var currentBaseUrl = currentServerConfig.activeEndpoint().url
    private var serviceHostLabel = extractHostLabel(currentBaseUrl)
    private var activeEndpointLabel = currentServerConfig.activeEndpoint().label
    private var savedEndpointSummary = currentServerConfig.endpointSummary()
    private var appUpdateChannelTapCount: Int = 0
    private var lastAppUpdateChannelTapAtMs: Long = 0L
    private val _uiState = MutableStateFlow(
        AppUiState(
            screen = AppScreen.Splash,
            pairing = PairingUiState(
                serviceHostLabel = serviceHostLabel,
                serviceBaseUrl = currentBaseUrl,
                environmentLabel = currentPairingEnvironmentLabel(),
            ),
            settings = SettingsUiState(
                baseUrl = currentBaseUrl,
                activeEndpointLabel = activeEndpointLabel,
                savedEndpointCountLabel = "${currentServerConfig.endpoints.size} 项",
                savedEndpointSummary = savedEndpointSummary,
                serviceHostLabel = serviceHostLabel,
                debugToolsVisible = config.debugToolsEnabled,
                selectedScenario = debugStore?.current() ?: DebugDemoScenario.NONE,
                update = AppUpdateUiState(
                    currentVersionLabel = formatVersionLabel(config.appVersion, config.appVersionCode),
                    channelLabel = currentAppUpdateChannelLabel(),
                    installPermissionEnabled = appUpdateManager.canRequestPackageInstalls(),
                    installPermissionLabel = formatInstallPermissionLabel(appUpdateManager.canRequestPackageInstalls()),
                    autoCheckEnabled = appUpdatePreferences.autoCheckEnabled,
                    currentVersionNotes = AppUpdateVersionNotesUiState(
                        versionLabel = currentVersionNotesVersionLabel(),
                        emptyLabel = "当前版本暂无更新说明",
                    ),
                ),
            ),
            offline = OfflineUiState(serviceHostLabel = serviceHostLabel),
        ),
    )
    private val _sessionMessageRotaryScrollDeltas = MutableSharedFlow<Float>(
        extraBufferCapacity = 8,
    )
    val uiState: StateFlow<AppUiState> = _uiState.asStateFlow()
    val sessionMessageRotaryScrollDeltas: SharedFlow<Float> = _sessionMessageRotaryScrollDeltas.asSharedFlow()

    private val refreshMutex = Mutex()
    private var screenBeforeSettings = AppScreen.Pairing
    private var currentToken: String = ""
    private var pendingBootstrapRequest: BootstrapRequest? = null
    private var pendingBootstrapContinuation = false
    private var hasPaired = false
    private var isBootstrapping = false
    private var isServiceDegraded = false
    private var latestSnapshot: WatcherStatusSnapshot? = null
    private var observedContextCompactions: Map<String, String> = emptyMap()
    private var cachedDailyTrend30d: DailyTrend30dSnapshot? = dailyTrendStore.read()
    private var selectedHeatmapHourStart: Instant? = null
    private var heatmapCursorPosition: Float? = null
    private var heatmapCursorSettleJob: Job? = null
    private var heatmapRotaryMode = HeatmapRotaryMode.HourRing
    private var selectedTrendDayIndex: Int? = null
    private var heatmapTrendCursorPosition: Float? = null
    private var heatmapTrendPendingDegrees = 0f
    private var heatmapTrendTipVisible = false
    private var heatmapTrendTipHideJob: Job? = null
    private var selectedSessionThreadId: String? = null
    private var selectedSessionSlotIndex: Int? = null
    private var sessionSelectionCursorPosition: Float? = null
    private var pollJob: Job? = null
    private var statusStreamJob: Job? = null
    private var localRefreshJob: Job? = null
    private var sessionStreamJob: Job? = null
    private var sessionRuntimeDurationTickerJob: Job? = null
    private var detailWindowStreamJob: Job? = null
    private var sessionStreamThreadId: String? = null
    private var sessionStreamIncludeMessages = false
    private var isForegroundVisible = true
    private var homeQuotaTipHideJob: Job? = null
    private var screenshotUploadJob: Job? = null
    private var pendingScreenshotUploadJob: Job? = null
    private var screenshotFeedbackHideJob: Job? = null
    private var sessionAgentState = SessionAgentState()
    private var sessionTurnDurationByThreadId = emptyMap<String, SessionTurnDuration>()
    private var detailWindowState = DetailWindowState()
    private val homeQuotaTipLastIndexByPool = mutableMapOf<HomeQuotaTipPool, Int>()
    private var pendingAppUpdate: WatcherApkUpdate? = null
    private var watchBootstrapJob: Job? = null

    init {
        reconcileInstalledAppUpdatePreferences()
        refreshAppUpdatePreferenceState()
        viewModelScope.launch {
            uiState.collect { state ->
                diagnosticUiStateSink(state)
            }
        }
        diagnosticUploadManager?.let { manager ->
            viewModelScope.launch {
                manager.state.collect { status ->
                    applyDiagnosticUploadStatus(status)
                }
            }
        }
        if (enableBootstrap) {
            bootstrap()
        } else {
            initializeWithoutBootstrap()
        }
    }

    fun openSettings() {
        logUserAction("open_settings")
        resetSettingsDestination()
        if (hasPaired) {
            _uiState.value = _uiState.value.copy(
                screen = AppScreen.Dashboard,
                dashboard = _uiState.value.dashboard.copy(pagerPage = DashboardPage.Settings),
            )
            updateSessionStreamForCurrentScreen(clearMessage = true)
            runHealthCheck()
            return
        }
        screenBeforeSettings = _uiState.value.screen
        _uiState.value = _uiState.value.copy(screen = AppScreen.Settings)
        runHealthCheck()
    }

    fun closeSettings() {
        if (_uiState.value.settings.destination != SettingsDestination.Root) {
            resetSettingsDestination()
            return
        }
        _uiState.value = _uiState.value.copy(
            screen = screenBeforeSettings,
            dashboard = _uiState.value.dashboard.copy(pagerPage = DashboardPage.Home),
        )
        updateSessionStreamForCurrentScreen(clearMessage = true)
    }

    fun setDashboardPage(page: DashboardPage) {
        if (_uiState.value.dashboard.pagerPage == page) {
            return
        }
        val previousPage = _uiState.value.dashboard.pagerPage
        logUserAction("set_dashboard_page", mapOf("page" to page.name))
        if (page != DashboardPage.Settings) {
            resetSettingsDestination()
        }
        _uiState.value = _uiState.value.copy(
            dashboard = _uiState.value.dashboard.copy(pagerPage = page),
        )
        updateSessionStreamForCurrentScreen(clearMessage = page != DashboardPage.Home)
        if (page == DashboardPage.Settings && previousPage != DashboardPage.Settings) {
            runHealthCheck()
        }
    }

    fun navigateBackFromSettings() {
        when (_uiState.value.settings.destination) {
            SettingsDestination.About,
            -> {
                resetSettingsDestination()
                return
            }

            SettingsDestination.UpdateCheck,
            SettingsDestination.UpdateLatest,
            -> {
                _uiState.value = _uiState.value.copy(
                    settings = _uiState.value.settings.copy(destination = SettingsDestination.About),
                )
                return
            }

            SettingsDestination.CurrentVersionNotes,
            -> {
                _uiState.value = _uiState.value.copy(
                    settings = _uiState.value.settings.copy(destination = SettingsDestination.UpdateLatest),
                )
                return
            }

            SettingsDestination.UpdateNotes,
            -> {
                _uiState.value = _uiState.value.copy(
                    settings = _uiState.value.settings.copy(destination = SettingsDestination.About),
                )
                return
            }

            SettingsDestination.Root -> Unit
        }
        if (_uiState.value.settings.destination != SettingsDestination.Root) {
            return
        }
        if (_uiState.value.screen == AppScreen.Settings) {
            closeSettings()
            return
        }
        setDashboardPage(DashboardPage.Home)
    }

    fun openSettingsDestination(destination: SettingsDestination) {
        if (_uiState.value.settings.destination == destination) {
            when (destination) {
                SettingsDestination.About -> syncInstallPermissionState()
                SettingsDestination.UpdateCheck -> checkForAppUpdate()
                SettingsDestination.UpdateLatest -> refreshAppUpdatePreferenceState()
                SettingsDestination.CurrentVersionNotes -> syncCurrentVersionNotesState(persistIfMissing = true)
                SettingsDestination.UpdateNotes -> syncInstallPermissionState()
                SettingsDestination.Root -> Unit
            }
            return
        }
        logUserAction("open_settings_destination", mapOf("destination" to destination.name))
        _uiState.value = _uiState.value.copy(
            settings = _uiState.value.settings.copy(destination = destination),
        )
        when (destination) {
            SettingsDestination.Root -> Unit
            SettingsDestination.About -> syncInstallPermissionState()
            SettingsDestination.UpdateCheck -> {
                syncInstallPermissionState()
                checkForAppUpdate()
            }
            SettingsDestination.UpdateLatest -> refreshAppUpdatePreferenceState()
            SettingsDestination.CurrentVersionNotes -> syncCurrentVersionNotesState(persistIfMissing = true)
            SettingsDestination.UpdateNotes -> syncInstallPermissionState()
        }
    }

    fun openHeatmap() {
        if (!hasPaired) {
            return
        }
        logUserAction("open_heatmap24h")
        clearHeatmapTrendSelection(refreshUi = false)
        _uiState.value = _uiState.value.copy(
            screen = AppScreen.Heatmap24h,
            dashboard = _uiState.value.dashboard.copy(pagerPage = DashboardPage.Home),
        )
        updateSessionStreamForCurrentScreen(clearMessage = true)
    }

    fun openSessionDetails() {
        if (!hasPaired) {
            return
        }
        logUserAction("open_session_details")
        _uiState.value = _uiState.value.copy(
            screen = AppScreen.SessionDetails,
            dashboard = _uiState.value.dashboard.copy(pagerPage = DashboardPage.Home),
        )
        restorePersistedDetailWindowState()
        updateSessionStreamForCurrentScreen(clearMessage = false)
    }

    fun showHomeQuotaEasterEgg() {
        val snapshot = latestSnapshot ?: return
        val pool = classifyHomeQuotaTipPool(snapshot) ?: return
        val text = pickHomeQuotaTip(pool) ?: return
        homeQuotaTipHideJob?.cancel()
        updateHomeQuotaEasterEggState(
            HomeQuotaEasterEggUiState(
                visible = true,
                text = text,
                pool = pool,
            ),
        )
        homeQuotaTipHideJob = viewModelScope.launch {
            delay(homeQuotaTipHideDurationMs)
            homeQuotaTipHideJob = null
            updateHomeQuotaEasterEggState(HomeQuotaEasterEggUiState())
        }
    }

    private fun showScreenshotFeedback(
        message: String,
        inProgress: Boolean,
        autoHide: Boolean = true,
    ) {
        showTransientNotice(
            message = message,
            inProgress = inProgress,
            autoHide = autoHide,
        )
    }

    private fun showTransientNotice(
        message: String,
        inProgress: Boolean = false,
        autoHide: Boolean = true,
    ) {
        screenshotFeedbackHideJob?.cancel()
        screenshotFeedbackHideJob = null
        _uiState.value = _uiState.value.copy(
            screenshotUpload = ScreenshotUploadUiState(
                visible = true,
                message = message,
                inProgress = inProgress,
            ),
        )
        if (autoHide) {
            screenshotFeedbackHideJob = viewModelScope.launch {
                delay(SCREENSHOT_UPLOAD_FEEDBACK_HIDE_MS)
                screenshotFeedbackHideJob = null
                _uiState.value = _uiState.value.copy(screenshotUpload = ScreenshotUploadUiState())
            }
        }
    }

    fun openAppUpdateFromAbout() {
        syncInstallPermissionState()
        if (!appUpdateManager.canRequestPackageInstalls()) {
            showTransientNotice("请允许安装未知来源才可更新应用")
            return
        }
        openSettingsDestination(SettingsDestination.UpdateCheck)
    }

    fun registerSecretAppUpdateChannelTap() {
        val nowMs = monotonicNowMs()
        val withinWindow = nowMs - lastAppUpdateChannelTapAtMs <= APP_UPDATE_CHANNEL_TAP_WINDOW_MS
        appUpdateChannelTapCount = if (withinWindow) {
            appUpdateChannelTapCount + 1
        } else {
            1
        }
        lastAppUpdateChannelTapAtMs = nowMs
        if (appUpdateChannelTapCount < APP_UPDATE_CHANNEL_TAP_TARGET) {
            return
        }
        appUpdateChannelTapCount = 0
        lastAppUpdateChannelTapAtMs = 0L
        if (appUpdatePreferences.selectedChannel == AppUpdateChannel.Beta &&
            !serverConfigRepository.hasStoredConfig(AppUpdateChannel.Dev)
        ) {
            showTransientNotice("尚未收到开发环境配置")
            return
        }
        val nextChannel = when (appUpdatePreferences.selectedChannel) {
            AppUpdateChannel.Beta -> AppUpdateChannel.Dev
            AppUpdateChannel.Dev -> AppUpdateChannel.Beta
        }
        persistAppUpdatePreferences(
            appUpdatePreferences.copy(
                selectedChannel = nextChannel,
                ignoredVersionCodes = emptySet(),
            ),
        )
        pendingAppUpdate = null
        switchEnvironmentRuntime()
        showTransientNotice(
            when (nextChannel) {
                AppUpdateChannel.Beta -> "已切换到 beta 通道"
                AppUpdateChannel.Dev -> "已切换到 dev 通道"
            },
        )
    }

    fun closeDetailScreen() {
        clearHeatmapTrendSelection(refreshUi = false)
        _uiState.value = _uiState.value.copy(
            screen = AppScreen.Dashboard,
            dashboard = _uiState.value.dashboard.copy(pagerPage = DashboardPage.Home),
        )
        updateSessionStreamForCurrentScreen(clearMessage = true)
    }

    fun selectHeatmapHour(index: Int) {
        val buckets = latestSnapshot?.heatmap24h?.buckets.orEmpty()
        if (buckets.isEmpty()) {
            return
        }
        clearHeatmapTrendSelection(refreshUi = false)
        heatmapCursorSettleJob?.cancel()
        heatmapCursorSettleJob = null
        val safeIndex = index.coerceIn(0, buckets.lastIndex)
        selectedHeatmapHourStart = buckets[safeIndex].hourStart
        heatmapCursorPosition = safeIndex.toFloat()
        refreshDerivedState()
    }

    fun selectHeatmapTrendDay(index: Int) {
        val trendDays = latestSnapshot?.dailyTrend30d?.days.orEmpty()
        if (trendDays.isEmpty()) {
            return
        }
        heatmapCursorSettleJob?.cancel()
        heatmapCursorSettleJob = null
        val safeIndex = index.coerceIn(0, trendDays.lastIndex)
        heatmapRotaryMode = HeatmapRotaryMode.Trend30d
        selectedTrendDayIndex = safeIndex
        heatmapTrendCursorPosition = safeIndex.toFloat()
        heatmapTrendPendingDegrees = 0f
        showHeatmapTrendTip()
    }

    fun clearHeatmapTrendSelection() {
        clearHeatmapTrendSelection(refreshUi = true)
    }

    fun stepHeatmapSelection(step: Int) {
        val buckets = latestSnapshot?.heatmap24h?.buckets.orEmpty()
        if (buckets.isEmpty()) {
            return
        }
        val currentIndex = _uiState.value.dashboard.heatmap24h.selectedIndex
            .takeIf { it >= 0 }
            ?: buckets.lastIndex
        selectHeatmapHour(wrapIndex(currentIndex + step, buckets.size))
    }

    fun rotateHeatmapCursor(rawDelta: Float) {
        if (rotateHeatmapTrendSelection(rawDelta)) {
            return
        }
        val buckets = latestSnapshot?.heatmap24h?.buckets.orEmpty()
        if (buckets.isEmpty() || rawDelta == 0f) {
            return
        }
        val currentIndex = _uiState.value.dashboard.heatmap24h.selectedIndex
            .takeIf { it >= 0 }
            ?: buckets.lastIndex
        val currentCursor = heatmapCursorPosition ?: currentIndex.toFloat()
        heatmapCursorPosition = wrapHeatmapCursorPosition(
            currentCursor + heatmapRotaryDeltaToCursorDelta(rawDelta),
            buckets.size,
        )
        refreshDerivedState()
        scheduleHeatmapCursorSettle()
    }

    fun scrollSessionMessage(rawDelta: Float) {
        if (rawDelta == 0f || _uiState.value.screen != AppScreen.SessionDetails) {
            return
        }
        _sessionMessageRotaryScrollDeltas.tryEmit(rawDelta)
    }

    private fun rotateHeatmapTrendSelection(rawDelta: Float): Boolean {
        if (heatmapRotaryMode != HeatmapRotaryMode.Trend30d) {
            return false
        }
        val trendDays = latestSnapshot?.dailyTrend30d?.days.orEmpty()
        if (trendDays.isEmpty() || rawDelta == 0f) {
            return false
        }
        heatmapTrendPendingDegrees += heatmapTrendRotaryDeltaToDegrees(rawDelta)
        val step = if (heatmapTrendPendingDegrees > 0f) {
            kotlin.math.floor(heatmapTrendPendingDegrees / HEATMAP_TREND_ROTARY_DEGREES_PER_BAR).toInt()
        } else {
            kotlin.math.ceil(heatmapTrendPendingDegrees / HEATMAP_TREND_ROTARY_DEGREES_PER_BAR).toInt()
        }
        if (step == 0) {
            return true
        }
        heatmapTrendPendingDegrees -= step * HEATMAP_TREND_ROTARY_DEGREES_PER_BAR
        val currentIndex = selectedTrendDayIndex?.coerceIn(0, trendDays.lastIndex) ?: 0
        val nextIndex = (currentIndex + step).coerceIn(0, trendDays.lastIndex)
        if (nextIndex == currentIndex) {
            heatmapTrendPendingDegrees = 0f
            return true
        }
        heatmapTrendCursorPosition = nextIndex.toFloat()
        selectedTrendDayIndex = nextIndex
        showHeatmapTrendTip()
        return true
    }

    private fun scheduleHeatmapCursorSettle() {
        heatmapCursorSettleJob?.cancel()
        heatmapCursorSettleJob = viewModelScope.launch {
            delay(HEATMAP_CURSOR_SETTLE_DELAY_MS)
            settleHeatmapCursor()
        }
    }

    private fun settleHeatmapCursor() {
        val buckets = latestSnapshot?.heatmap24h?.buckets.orEmpty()
        val cursor = heatmapCursorPosition
        if (buckets.isEmpty() || cursor == null) {
            return
        }
        val settledIndex = roundHeatmapCursorPosition(cursor, buckets.size)
        selectedHeatmapHourStart = buckets[settledIndex].hourStart
        heatmapCursorPosition = settledIndex.toFloat()
        heatmapCursorSettleJob = null
        refreshDerivedState()
    }

    private fun showHeatmapTrendTip() {
        heatmapTrendTipHideJob?.cancel()
        heatmapTrendTipVisible = true
        refreshDerivedState()
        heatmapTrendTipHideJob = viewModelScope.launch {
            delay(HEATMAP_TREND_TIP_HIDE_DURATION_MS)
            heatmapTrendTipHideJob = null
            heatmapTrendTipVisible = false
            refreshDerivedState()
        }
    }

    private fun clearHeatmapTrendSelection(refreshUi: Boolean) {
        val changed = heatmapRotaryMode != HeatmapRotaryMode.HourRing ||
            selectedTrendDayIndex != null ||
            heatmapTrendCursorPosition != null ||
            heatmapTrendTipVisible ||
            heatmapTrendTipHideJob != null
        heatmapRotaryMode = HeatmapRotaryMode.HourRing
        selectedTrendDayIndex = null
        heatmapTrendCursorPosition = null
        heatmapTrendPendingDegrees = 0f
        heatmapTrendTipVisible = false
        heatmapTrendTipHideJob?.cancel()
        heatmapTrendTipHideJob = null
        if (changed && refreshUi) {
            refreshDerivedState()
        }
    }

    fun selectSession(index: Int) {
        val threadOrder = detailWindowState.orderedThreadIds.takeIf {
            _uiState.value.screen == AppScreen.SessionDetails && it.isNotEmpty()
        } ?: sessionDetailDisplaySessions(latestSnapshot?.sessions.orEmpty()).map { it.threadId }
        if (threadOrder.isEmpty()) {
            return
        }
        val safeIndex = index.coerceIn(0, threadOrder.lastIndex)
        val nextThreadId = threadOrder[safeIndex]
        val changed = selectedSessionThreadId != nextThreadId
        selectedSessionThreadId = nextThreadId
        selectedSessionSlotIndex = safeIndex
        sessionSelectionCursorPosition = safeIndex.toFloat()
        if (_uiState.value.screen == AppScreen.SessionDetails && changed && detailWindowState.orderedThreadIds.isEmpty()) {
            prepareSessionStreamState(nextThreadId, AgentMessageStreamStatus.Connecting)
        }
        refreshDerivedState()
        if (detailWindowState.entriesByThreadId.isNotEmpty()) {
            persistDetailWindowSnapshot()
        }
        if (changed && _uiState.value.screen != AppScreen.SessionDetails) {
            updateSessionStreamForCurrentScreen(clearMessage = _uiState.value.screen == AppScreen.SessionDetails)
        }
    }

    fun regenerateToken() {
        logUserAction("regenerate_token")
        restartPairing(resetToken = true)
    }

    fun repair() {
        logUserAction("repair_pairing")
        restartPairing(resetToken = true)
    }

    fun clearTokenAndRegenerate() {
        restartPairing(resetToken = true)
    }

    fun refreshNow() {
        if (shouldStartWatchBootstrapFlow()) {
            logUserAction("refresh_watch_bootstrap")
            watchBootstrapJob?.cancel()
            startWatchBootstrapFlow()
            return
        }
        if (currentToken.isBlank()) {
            return
        }
        logUserAction("refresh_now")
        viewModelScope.launch {
            fetchStatus(showRefreshing = true, fromPairingLoop = false)
        }
    }

    fun captureAndUploadScreenshot(capturePng: () -> ByteArray?) {
        if (screenshotUploadJob?.isActive == true) {
            showScreenshotFeedback("截图上传中", inProgress = true, autoHide = false)
            return
        }
        if (currentToken.isBlank() || !hasPaired) {
            showScreenshotFeedback("未配对，无法上传截图", inProgress = false)
            return
        }
        val token = currentToken
        logUserAction("upload_screenshot")

        screenshotUploadJob = viewModelScope.launch {
            val pngBytes = runCatching { capturePng() }.getOrNull()
            if (pngBytes == null || pngBytes.isEmpty()) {
                showScreenshotFeedback("截图失败", inProgress = false)
                diagnosticEventLogger.log(
                    event = "user_action",
                    level = DiagnosticLevel.Warn,
                    fields = mapOf(
                        "action" to mapOf(
                            "name" to "upload_screenshot_failed",
                            "reason" to "capture_failed",
                        ),
                    ),
                )
                return@launch
            }

            hapticController.vibrateScreenshotCaptured()
            if (hasPendingScreenshots()) {
                if (enqueuePendingScreenshot(pngBytes)) {
                    showScreenshotFeedback("截图已暂存", inProgress = false)
                    schedulePendingScreenshotUpload()
                } else {
                    showScreenshotFeedback("截图暂存失败", inProgress = false)
                }
                return@launch
            }

            showScreenshotFeedback("截图上传中", inProgress = true, autoHide = false)
            val result = uploadScreenshotBytes(token, pngBytes)
            when (result) {
                is ScreenshotUploadResult.Success -> {
                    showScreenshotFeedback("截图已上传", inProgress = false)
                    schedulePendingScreenshotUpload()
                }
                ScreenshotUploadResult.Unauthorized -> {
                    showScreenshotFeedback("需要重新配对", inProgress = false)
                    handleUnauthorized(fromPairingLoop = false)
                }
                is ScreenshotUploadResult.HttpFailure -> showScreenshotFeedback("截图上传失败", inProgress = false)
                is ScreenshotUploadResult.NetworkFailure -> {
                    if (enqueuePendingScreenshot(pngBytes)) {
                        showScreenshotFeedback("网络离线，截图已暂存", inProgress = false)
                    } else {
                        showScreenshotFeedback("服务连接失败", inProgress = false)
                    }
                }
            }
        }
    }

    fun uploadDiagnostics() {
        val manager = diagnosticUploadManager ?: return
        dismissDiagnosticPrompt()
        if (currentToken.isBlank() || !hasPaired) {
            _uiState.value = _uiState.value.copy(
                settings = _uiState.value.settings.copy(
                    diagnosticUpload = _uiState.value.settings.diagnosticUpload.copy(
                        statusLabel = "未配对，无法上传",
                    ),
                ),
            )
            return
        }
        logUserAction(
            name = if (manager.state.value.hasPendingPackage) "retry_diagnostic_upload" else "upload_diagnostics",
            fields = mapOf("hasPendingPackage" to manager.state.value.hasPendingPackage),
        )
        manager.requestUpload(currentToken)
    }

    fun onDiagnosticEntryClick() {
        val diagnostic = _uiState.value.settings.diagnosticUpload
        if (!diagnostic.entryEnabled) {
            return
        }
        val prompt = if (diagnostic.hasPendingPackage) {
            DiagnosticPromptUiState(
                visible = true,
                title = "应用诊断",
                message = "本地已有未上传诊断包",
                primaryLabel = "上传",
                primaryAction = DiagnosticPromptAction.UploadPending,
                secondaryLabel = "清除",
                secondaryAction = DiagnosticPromptAction.ClearPending,
            )
        } else {
            DiagnosticPromptUiState(
                visible = true,
                title = "应用诊断",
                message = "确认上传新的诊断信息",
                primaryLabel = "确认上传",
                primaryAction = DiagnosticPromptAction.ConfirmUpload,
            )
        }
        _uiState.value = _uiState.value.copy(
            settings = _uiState.value.settings.copy(
                diagnosticPrompt = prompt,
            ),
        )
    }

    fun dismissDiagnosticPrompt() {
        if (!_uiState.value.settings.diagnosticPrompt.visible) {
            return
        }
        _uiState.value = _uiState.value.copy(
            settings = _uiState.value.settings.copy(
                diagnosticPrompt = DiagnosticPromptUiState(),
            ),
        )
    }

    fun confirmDiagnosticPromptPrimary() {
        executeDiagnosticPromptAction(_uiState.value.settings.diagnosticPrompt.primaryAction)
    }

    fun confirmDiagnosticPromptSecondary() {
        executeDiagnosticPromptAction(_uiState.value.settings.diagnosticPrompt.secondaryAction)
    }

    private suspend fun hasPendingScreenshots(): Boolean {
        return screenshotUploadQueue.pending().isNotEmpty()
    }

    private suspend fun enqueuePendingScreenshot(pngBytes: ByteArray): Boolean {
        return screenshotUploadQueue.enqueue(pngBytes, wallClockNow())
    }

    private suspend fun uploadScreenshotBytes(token: String, pngBytes: ByteArray): ScreenshotUploadResult {
        return withContext(ioDispatcher) {
            api.uploadScreenshot(
                token = token,
                request = ScreenshotUploadRequest(
                    pngBytes = pngBytes,
                    deviceName = config.deviceName,
                    appVersion = config.appVersion,
                ),
            )
        }
    }

    private fun schedulePendingScreenshotUpload() {
        if (pendingScreenshotUploadJob?.isActive == true || currentToken.isBlank() || !hasPaired) {
            return
        }
        val token = currentToken
        pendingScreenshotUploadJob = viewModelScope.launch {
            uploadPendingScreenshots(token)
        }
    }

    private suspend fun uploadPendingScreenshots(token: String) {
        var uploadedCount = 0
        while (hasPaired && currentToken == token) {
            val pending = screenshotUploadQueue.pending()
            if (pending.isEmpty()) {
                break
            }
            var shouldStop = false
            for (item in pending) {
                val pngBytes = screenshotUploadQueue.read(item)
                if (pngBytes == null || pngBytes.isEmpty()) {
                    screenshotUploadQueue.delete(item)
                    continue
                }

                when (uploadScreenshotBytes(token, pngBytes)) {
                    is ScreenshotUploadResult.Success -> {
                        screenshotUploadQueue.delete(item)
                        uploadedCount += 1
                    }
                    ScreenshotUploadResult.Unauthorized -> {
                        showScreenshotFeedback("需要重新配对", inProgress = false)
                        handleUnauthorized(fromPairingLoop = false)
                        shouldStop = true
                    }
                    is ScreenshotUploadResult.HttpFailure,
                    is ScreenshotUploadResult.NetworkFailure,
                    -> shouldStop = true
                }
                if (shouldStop) {
                    break
                }
            }
            if (shouldStop) {
                break
            }
        }

        if (uploadedCount > 0 && hasPaired) {
            val message = if (uploadedCount == 1) {
                "暂存截图已补传"
            } else {
                "${uploadedCount} 张截图已补传"
            }
            showScreenshotFeedback(message, inProgress = false)
        }
    }

    fun runHealthCheck() {
        if (_uiState.value.settings.healthCheck.status == ServiceHealthStatus.Checking) {
            return
        }
        _uiState.value = _uiState.value.copy(
            settings = _uiState.value.settings.copy(
                healthCheck = ServiceHealthUiState(
                    status = ServiceHealthStatus.Checking,
                    resultLabel = "正在检查",
                    detailLabel = "正在检查服务可达性",
                    errorLabel = "检查完成后会在这里显示结果",
                    resultColor = Color(0xFF58A6FF),
                ),
            ),
        )
        viewModelScope.launch {
            val next = when (val result = api.checkHealth()) {
                HealthCheckResult.Online -> ServiceHealthUiState(
                    status = ServiceHealthStatus.Online,
                    resultLabel = "服务在线",
                    detailLabel = "服务响应正常",
                    errorLabel = "暂无",
                    resultColor = Color(0xFF65D46E),
                )

                is HealthCheckResult.Offline -> ServiceHealthUiState(
                    status = ServiceHealthStatus.Offline,
                    resultLabel = "服务不可达",
                    detailLabel = "当前无法连接到服务",
                    errorLabel = result.message,
                    resultColor = Color(0xFFFF7070),
                )
            }
            _uiState.value = _uiState.value.copy(
                settings = _uiState.value.settings.copy(
                    healthCheck = next,
                ),
            )
        }
    }

    fun checkForAppUpdate() {
        val current = _uiState.value.settings.update.status
        if (current == AppUpdateStatus.Checking || current == AppUpdateStatus.Downloading) {
            return
        }
        if (_uiState.value.settings.destination != SettingsDestination.UpdateCheck) {
            _uiState.value = _uiState.value.copy(
                settings = _uiState.value.settings.copy(destination = SettingsDestination.UpdateCheck),
            )
        }
        pendingAppUpdate = null
        syncInstallPermissionState()
        updateAppUpdateState {
            it.copy(
                status = AppUpdateStatus.Checking,
                isExpanded = false,
                latestVersionLabel = null,
                comparisonLabel = null,
                progressPercent = null,
                progressDetailLabel = null,
                downloadSpeedLabel = null,
                hasPendingUpdate = false,
                detailLabel = "正在检查更新…",
                latestVersionNotes = AppUpdateVersionNotesUiState(),
                downloadOverlay = AppUpdateDownloadOverlayUiState(),
            )
        }
        viewModelScope.launch {
            when (val result = performTimedAppUpdateCheck()) {
                is WatcherApkUpdateCheckResult.Failure -> {
                    updateAppUpdateState {
                        it.copy(
                            status = AppUpdateStatus.Failed,
                            progressPercent = null,
                            progressDetailLabel = null,
                            downloadSpeedLabel = null,
                            detailLabel = result.message,
                            latestVersionNotes = AppUpdateVersionNotesUiState(),
                            downloadOverlay = AppUpdateDownloadOverlayUiState(),
                        )
                    }
                }

                is WatcherApkUpdateCheckResult.Success -> {
                    val update = result.update
                    val latestLabel = formatVersionLabel(update.versionName, update.versionCode)
                    if (update.versionCode <= config.appVersionCode) {
                        pendingAppUpdate = null
                        updateAppUpdateState {
                            it.copy(
                                status = AppUpdateStatus.UpToDate,
                                latestVersionLabel = latestLabel,
                                comparisonLabel = "当前版本 ${it.currentVersionLabel}",
                                progressPercent = null,
                                progressDetailLabel = null,
                                downloadSpeedLabel = null,
                                hasPendingUpdate = false,
                                detailLabel = "当前已是所选通道最新版本",
                                latestVersionNotes = AppUpdateVersionNotesUiState(),
                                downloadOverlay = AppUpdateDownloadOverlayUiState(),
                            )
                        }
                        _uiState.value = _uiState.value.copy(
                            dashboard = _uiState.value.dashboard.copy(pagerPage = DashboardPage.Settings),
                            settings = _uiState.value.settings.copy(destination = SettingsDestination.UpdateLatest),
                        )
                    } else {
                        pendingAppUpdate = update
                        val comparisonLabel = "当前 ${formatVersionShort(config.appVersion)} -> 最新 ${update.versionName}"
                        val hasCachedUpdate = appUpdateManager.hasCachedUpdate(update)
                        if (hasCachedUpdate) {
                            showCachedUpdateNotesAndResumeInstall(update, latestLabel, comparisonLabel)
                        } else {
                            updateAppUpdateState {
                                it.copy(
                                    status = AppUpdateStatus.Available,
                                    latestVersionLabel = latestLabel,
                                    comparisonLabel = comparisonLabel,
                                    progressPercent = null,
                                    progressDetailLabel = null,
                                    downloadSpeedLabel = null,
                                    hasPendingUpdate = true,
                                    detailLabel = "发现新版本",
                                    latestVersionNotes = buildLatestVersionNotesUiState(update),
                                    downloadOverlay = AppUpdateDownloadOverlayUiState(),
                                )
                            }
                            _uiState.value = _uiState.value.copy(
                                dashboard = _uiState.value.dashboard.copy(pagerPage = DashboardPage.Settings),
                                settings = _uiState.value.settings.copy(destination = SettingsDestination.UpdateNotes),
                            )
                        }
                    }
                }
            }
        }
    }

    fun openInstallPermissionSettings() {
        val opened = appUpdateManager.openInstallPermissionSettings()
        syncInstallPermissionState(
            detailLabel = if (opened) "已打开系统设置" else "无法打开系统设置",
        )
    }

    fun ignoreAppUpdate() {
        val ignored = pendingAppUpdate ?: return
        persistAppUpdatePreferences(
            appUpdatePreferences.copy(
                ignoredVersionCodes = appUpdatePreferences.ignoredVersionCodes + ignored.versionCode,
            ),
        )
        pendingAppUpdate = null
        updateAppUpdateState {
            it.copy(
                status = AppUpdateStatus.UpToDate,
                isExpanded = false,
                latestVersionLabel = formatVersionLabel(ignored.versionName, ignored.versionCode),
                comparisonLabel = null,
                progressPercent = null,
                progressDetailLabel = null,
                downloadSpeedLabel = null,
                hasPendingUpdate = false,
                detailLabel = "已忽略本次更新",
                latestVersionNotes = AppUpdateVersionNotesUiState(),
                downloadOverlay = AppUpdateDownloadOverlayUiState(),
            )
        }
        _uiState.value = _uiState.value.copy(
            settings = _uiState.value.settings.copy(destination = SettingsDestination.UpdateCheck),
        )
    }

    fun downloadAndInstallAppUpdate() {
        val update = pendingAppUpdate ?: return
        viewModelScope.launch {
            if (appUpdateManager.hasCachedUpdate(update)) {
                cachePendingInstalledVersionNotes(update)
                updateAppUpdateState {
                    it.copy(
                        status = AppUpdateStatus.ReadyToInstall,
                        isExpanded = false,
                        latestVersionLabel = formatVersionLabel(update.versionName, update.versionCode),
                        comparisonLabel = "当前 ${formatVersionShort(config.appVersion)} -> 最新 ${update.versionName}",
                        progressPercent = 100,
                        progressDetailLabel = null,
                        downloadSpeedLabel = null,
                        hasPendingUpdate = true,
                        detailLabel = "已下载更新，准备安装",
                        downloadOverlay = AppUpdateDownloadOverlayUiState(),
                    )
                }
                applyAppUpdateInstallResult(
                    appUpdateManager.startInstallFromCache(update),
                    successDetail = "已打开系统安装器",
                )
                return@launch
            }

            updateAppUpdateState {
                it.copy(
                    status = AppUpdateStatus.Downloading,
                    isExpanded = false,
                    latestVersionLabel = formatVersionLabel(update.versionName, update.versionCode),
                    comparisonLabel = "当前 ${formatVersionShort(config.appVersion)} -> 最新 ${update.versionName}",
                    progressPercent = 0,
                    progressDetailLabel = "0 B / --",
                    downloadSpeedLabel = "0 B/s",
                    hasPendingUpdate = true,
                    detailLabel = "正在下载更新",
                    downloadOverlay = AppUpdateDownloadOverlayUiState(
                        visible = true,
                        statusLabel = "准备下载更新",
                        fileSizeLabel = update.apkSizeBytes?.let(::formatBinaryBytes) ?: "待确认",
                        progressLabel = "0%",
                        transferredLabel = "0 B / --",
                        speedLabel = "0 B/s",
                    ),
                )
            }
            val result = appUpdateManager.downloadAndStartInstall(update) { progress ->
                when (progress) {
                    is WatcherApkUpdateProgress.Downloading -> updateAppUpdateState {
                        val progressPercent = progress.totalBytes?.takeIf { total -> total > 0L }?.let { total ->
                            ((progress.bytesDownloaded.toDouble() / total.toDouble()) * 100.0)
                                .roundToInt()
                                .coerceIn(0, 100)
                        }
                        val transferredLabel = progress.totalBytes?.takeIf { total -> total > 0L }?.let { total ->
                            "${formatBinaryBytes(progress.bytesDownloaded)} / ${formatBinaryBytes(total)}"
                        } ?: "${formatBinaryBytes(progress.bytesDownloaded)} / --"
                        val speedLabel = progress.speedBytesPerSecond
                            ?.takeIf { speed -> speed > 0L }
                            ?.let { speed -> "${formatBinaryBytes(speed)}/s" }
                        it.copy(
                            status = AppUpdateStatus.Downloading,
                            progressPercent = progressPercent,
                            progressDetailLabel = transferredLabel,
                            downloadSpeedLabel = speedLabel,
                            detailLabel = "正在下载更新",
                            downloadOverlay = AppUpdateDownloadOverlayUiState(
                                visible = true,
                                statusLabel = "正在下载更新",
                                fileSizeLabel = progress.totalBytes
                                    ?.takeIf { total -> total > 0L }
                                    ?.let(::formatBinaryBytes)
                                    ?: it.downloadOverlay.fileSizeLabel,
                                progressLabel = progressPercent?.let { percent -> "$percent%" } ?: "下载中",
                                transferredLabel = transferredLabel,
                                speedLabel = speedLabel,
                            ),
                        )
                    }

                    WatcherApkUpdateProgress.PreparingInstall -> updateAppUpdateState {
                        cachePendingInstalledVersionNotes(update)
                        it.copy(
                            status = AppUpdateStatus.ReadyToInstall,
                            progressPercent = 100,
                            progressDetailLabel = null,
                            downloadSpeedLabel = null,
                            detailLabel = "下载完成，准备安装",
                            downloadOverlay = it.downloadOverlay.copy(
                                visible = true,
                                statusLabel = "下载完成，准备安装",
                                progressLabel = "100%",
                            ),
                        )
                    }
                }
            }
            applyAppUpdateInstallResult(result, successDetail = "已打开系统安装器")
        }
    }

    fun setAutoCheckAppUpdateEnabled(enabled: Boolean) {
        if (appUpdatePreferences.autoCheckEnabled == enabled) {
            return
        }
        persistAppUpdatePreferences(appUpdatePreferences.copy(autoCheckEnabled = enabled))
        refreshAppUpdatePreferenceState()
    }

    fun setDebugScenario(scenario: DebugDemoScenario) {
        debugStore?.set(scenario) ?: return
        _uiState.value = _uiState.value.copy(
            settings = _uiState.value.settings.copy(selectedScenario = scenario),
        )
        viewModelScope.launch {
            fetchStatus(showRefreshing = false, fromPairingLoop = !hasPaired)
        }
    }

    fun applyDebugSettingsPreview(
        destination: SettingsDestination,
        updatePreview: String?,
        installPermissionEnabledOverride: Boolean?,
    ) {
        if (!config.debugToolsEnabled) {
            return
        }
        val installPermissionEnabled = installPermissionEnabledOverride
            ?: _uiState.value.settings.update.installPermissionEnabled
        val previewUpdate = buildDebugPreviewUpdateState(
            updatePreview = updatePreview,
            installPermissionEnabled = installPermissionEnabled,
        )
        pendingAppUpdate = if (previewUpdate.hasPendingUpdate) {
            buildDebugPreviewUpdate(previewUpdate.latestVersionLabel)
        } else {
            null
        }
        _uiState.value = _uiState.value.copy(
            screen = AppScreen.Settings,
            settings = _uiState.value.settings.copy(
                destination = destination,
                serviceTitle = "服务在线",
                serviceSubtitle = "连接正常",
                serviceColor = Color(0xFF65D46E),
                healthCheck = ServiceHealthUiState(
                    status = ServiceHealthStatus.Online,
                    resultLabel = "服务在线",
                    detailLabel = "服务响应正常",
                    errorLabel = "暂无",
                    resultColor = Color(0xFF65D46E),
                ),
                update = previewUpdate,
            ),
        )
    }

    fun stopPolling() {
        pollJob?.cancel()
        pollJob = null
        watchBootstrapJob?.cancel()
        watchBootstrapJob = null
        stopStatusStream()
        stopLocalRefreshTicker()
        stopSessionStream(clearMessage = true)
        stopDetailWindowStream(clearState = true)
        stopSessionRuntimeDurationTicker()
    }

    fun setForegroundVisible(visible: Boolean) {
        if (isForegroundVisible == visible) {
            return
        }
        isForegroundVisible = visible
        viewModelScope.launch {
            diagnosticEventLogger.log(
                event = "app_lifecycle",
                fields = mapOf(
                    "lifecycle" to mapOf(
                        "action" to if (visible) "foreground" else "background",
                    ),
                ),
            )
        }
        if (visible) {
            syncInstallPermissionState()
            if (hasPaired && autoPolling && !isBootstrapping) {
                startStatusStream()
                startLocalRefreshTicker()
            }
            updateSessionStreamForCurrentScreen(clearMessage = false)
        } else {
            stopStatusStream()
            stopLocalRefreshTicker()
            stopSessionStream(clearMessage = false)
            stopDetailWindowStream(clearState = false)
            stopSessionRuntimeDurationTicker()
        }
    }

    suspend fun fetchForTesting(fromPairingLoop: Boolean) {
        fetchStatus(showRefreshing = false, fromPairingLoop = fromPairingLoop)
    }

    fun presentBootstrapRequest(request: BootstrapRequest) {
        pollJob?.cancel()
        pollJob = null
        stopStatusStream()
        stopLocalRefreshTicker()
        stopSessionStream(clearMessage = false)
        stopDetailWindowStream(clearState = false)
        pendingBootstrapRequest = null
        pendingBootstrapContinuation = false
        isBootstrapping = true
        applyIncomingBootstrapProgress(request)
        viewModelScope.launch {
            applyIncomingBootstrapRequest(request)
        }
    }

    fun showBootstrapError(message: String) {
        pendingBootstrapRequest = null
        pendingBootstrapContinuation = false
        _uiState.value = _uiState.value.copy(
            screen = AppScreen.BootstrapConfirm,
            bootstrap = BootstrapUiState(
                title = "配置链接无效",
                detailLabel = message,
                warningLabel = "配置不会写入手表，请返回电脑端重新生成链接。",
                resultLabel = message,
                canConfirm = false,
            ),
        )
    }

    fun cancelBootstrapRequest() {
        pendingBootstrapRequest = null
        pendingBootstrapContinuation = false
        isBootstrapping = false
        initializeWithoutBootstrap()
    }

    fun confirmBootstrapRequest() {
        if (pendingBootstrapRequest == null) {
            if (pendingBootstrapContinuation) {
                pendingBootstrapContinuation = false
                initializeWithoutBootstrap()
                resumeCurrentEnvironmentAfterConfigChange()
            }
            return
        }
        val request = requireNotNull(pendingBootstrapRequest)
        pendingBootstrapContinuation = false
        viewModelScope.launch {
            applyIncomingBootstrapRequest(request)
        }
    }

    private fun applyIncomingBootstrapProgress(request: BootstrapRequest) {
        val primaryEndpoint = request.endpoints.minByOrNull { it.priority } ?: request.endpoints.first()
        _uiState.value = _uiState.value.copy(
            screen = AppScreen.Pairing,
            pairing = PairingUiState(
                statusLabel = "正在保存服务配置",
                hintLabel = "正在写入服务地址并选择可用入口",
                serviceLabel = "配置写入中",
                serviceHostLabel = extractHostLabel(primaryEndpoint.url),
                serviceBaseUrl = primaryEndpoint.url,
                environmentLabel = request.channel.toPairingEnvironmentLabel(),
                serviceColor = Color(0xFF58A6FF),
                tokenFingerprint = tokenRepository.tokenFingerprint(request.deviceToken),
            ),
        )
    }

    private suspend fun applyIncomingBootstrapRequest(request: BootstrapRequest) {
        pendingBootstrapRequest = null
        pendingBootstrapContinuation = false
        val savedConfig = serverConfigRepository.save(
            channel = request.channel,
            endpoints = request.endpoints,
            source = ServerConfigSource.DesktopBootstrap,
            deviceToken = request.deviceToken,
            deviceName = request.deviceName,
        )
        if (request.channel == AppUpdateChannel.Beta) {
            tokenRepository.setToken(request.deviceToken)
            pairingStateStore.markPaired()
        }
        persistAppUpdatePreferences(appUpdatePreferences.copy(selectedChannel = request.channel))
        applySelectedEnvironmentRuntimeContext()
        refreshAppUpdatePreferenceState()
        syncCurrentVersionNotesState(persistIfMissing = true)
        statusSnapshotStore.clear()
        sessionDetailsWindowStore.clear()
        latestSnapshot = null
        isServiceDegraded = false
        updateDiagnosticPairedState()
        val selection = selectAndApplyEndpoint(savedConfig)

        if (selection.hasReachableEndpoint) {
            isBootstrapping = true
            applyBootstrapResult(api.fetchStatus(currentToken))
        } else {
            isBootstrapping = false
            val failureLabel = endpointFailureSummary(selection)
            _uiState.value = _uiState.value.copy(
                screen = AppScreen.Pairing,
                pairing = buildPairingState(
                    status = "服务不可达",
                    hint = "检查网络后再试",
                    service = failureLabel,
                    color = Color(0xFFFF7070),
                    scanStepCompleted = false,
                    confirmStepActive = false,
                    authStepCompleted = false,
                ),
                settings = _uiState.value.settings.copy(
                    baseUrl = currentBaseUrl,
                    activeEndpointLabel = activeEndpointLabel,
                    savedEndpointCountLabel = endpointCountLabel(),
                    savedEndpointSummary = savedEndpointSummary,
                    serviceTitle = "服务不可达",
                    serviceSubtitle = failureLabel,
                    serviceColor = Color(0xFFFF7070),
                    serviceHostLabel = serviceHostLabel,
                    syncStatusLabel = "等待服务恢复",
                ),
                offline = buildOfflineState(failureLabel),
            )
            if (autoPolling) {
                startPairingLoop()
            }
        }
    }

    private fun bootstrap() {
        pollJob?.cancel()
        watchBootstrapJob?.cancel()
        stopStatusStream()
        stopLocalRefreshTicker()
        stopSessionStream(clearMessage = true)
        stopDetailWindowStream(clearState = true)
        homeQuotaTipHideJob?.cancel()
        homeQuotaTipHideJob = null
        pendingBootstrapContinuation = false
        resetSelectionAndTransientState()
        if (shouldStartWatchBootstrapFlow()) {
            startWatchBootstrapFlow()
            return
        }
        startConfiguredBootstrapFlow()
    }

    private fun startConfiguredBootstrapFlow() {
        applyRuntimeServerConfig(serverConfigRepository.current(activeEnvironment()))
        currentToken = currentRuntimeToken()
        hasPaired = hasCurrentRuntimeConfig()
        updateDiagnosticPairedState()
        isBootstrapping = hasPaired
        isServiceDegraded = hasPaired
        latestSnapshot = statusSnapshotStore.read()?.let(::mergeCachedDailyTrend)
        if (!hasPaired) {
            _uiState.value = _uiState.value.copy(
                screen = AppScreen.Pairing,
                pairing = buildPairingState(
                    status = "等待手机扫码",
                    hint = "请在手机端扫码配对",
                    service = "等待服务确认",
                    color = Color(0xFFFFC857),
                    scanStepCompleted = false,
                    confirmStepActive = false,
                    authStepCompleted = false,
                ),
                dashboard = DashboardUiState(serviceHostLabel = serviceHostLabel),
                settings = _uiState.value.settings.copy(
                    baseUrl = currentBaseUrl,
                    activeEndpointLabel = activeEndpointLabel,
                    savedEndpointCountLabel = endpointCountLabel(),
                    savedEndpointSummary = savedEndpointSummary,
                    destination = SettingsDestination.Root,
                    serviceTitle = "等待配对",
                    serviceSubtitle = "等待服务确认",
                    serviceColor = Color(0xFFFFC857),
                    serviceHostLabel = serviceHostLabel,
                    updatedAtLabel = latestSnapshot?.let { buildUpdatedLabel(it.observedAt, fresh = false) } ?: "尚未刷新",
                    syncStatusLabel = "等待数据",
                    healthCheck = ServiceHealthUiState(),
                ),
                offline = buildOfflineState("无法连接到 OpenWatcher"),
            )
            if (autoPolling) {
                startPairingLoop()
            }
            return
        }
        _uiState.value = _uiState.value.copy(
            screen = AppScreen.Splash,
            pairing = buildPairingState(
                status = "等待手机扫码",
                hint = "请在手机端扫码配对",
                service = "等待服务确认",
                color = Color(0xFFFFC857),
                scanStepCompleted = false,
                confirmStepActive = false,
                authStepCompleted = false,
            ),
            dashboard = bootstrapDashboardState(),
            settings = _uiState.value.settings.copy(
                baseUrl = currentBaseUrl,
                activeEndpointLabel = activeEndpointLabel,
                savedEndpointCountLabel = endpointCountLabel(),
                savedEndpointSummary = savedEndpointSummary,
                destination = SettingsDestination.Root,
                serviceTitle = if (hasPaired) "等待连接" else "等待配对",
                serviceSubtitle = if (hasPaired) "准备恢复历史数据" else "等待服务确认",
                serviceColor = if (hasPaired) Color(0xFFADB9CC) else Color(0xFFFFC857),
                serviceHostLabel = serviceHostLabel,
                updatedAtLabel = latestSnapshot?.let { buildUpdatedLabel(it.observedAt, fresh = false) } ?: "尚未刷新",
                syncStatusLabel = if (hasPaired) "启动恢复中" else "等待数据",
                healthCheck = ServiceHealthUiState(),
            ),
            offline = buildOfflineState("无法连接到 OpenWatcher"),
        )
        viewModelScope.launch {
            selectAndApplyEndpoint(currentServerConfig)
            val fetchDeferred = async(ioDispatcher) {
                api.fetchStatus(currentToken)
            }
            val autoUpdateDeferred = if (shouldAutoCheckAppUpdateOnColdStart()) {
                async(ioDispatcher) { fetchAutoAppUpdate() }
            } else {
                null
            }
            delay(SPLASH_MIN_DURATION_MS)
            val autoUpdate = autoUpdateDeferred?.await()
            if (autoUpdate != null) {
                prepareAutoAppUpdateDestination(autoUpdate)
            }
            applyBootstrapResult(fetchDeferred.await())
        }
    }

    private fun initializeWithoutBootstrap() {
        pendingBootstrapContinuation = false
        watchBootstrapJob?.cancel()
        resetSelectionAndTransientState()
        applyRuntimeServerConfig(serverConfigRepository.current(activeEnvironment()))
        currentToken = currentRuntimeToken()
        hasPaired = hasCurrentRuntimeConfig()
        updateDiagnosticPairedState()
        isBootstrapping = false
        isServiceDegraded = hasPaired
        latestSnapshot = statusSnapshotStore.read()?.let(::mergeCachedDailyTrend)
        _uiState.value = _uiState.value.copy(
            screen = if (hasPaired) AppScreen.Dashboard else AppScreen.Pairing,
            pairing = buildPairingState(
                status = if (hasPaired) "已配对" else "等待手机扫码",
                hint = if (hasPaired) "服务恢复后会自动刷新" else "请在手机端扫码配对",
                service = if (hasPaired) "等待连接" else "等待服务确认",
                color = if (hasPaired) Color(0xFF8E98A7) else Color(0xFFFFC857),
                scanStepCompleted = hasPaired,
                confirmStepActive = false,
                authStepCompleted = hasPaired,
            ),
            dashboard = if (hasPaired) bootstrapDashboardState() else DashboardUiState(serviceHostLabel = serviceHostLabel),
            settings = _uiState.value.settings.copy(
                baseUrl = currentBaseUrl,
                activeEndpointLabel = activeEndpointLabel,
                savedEndpointCountLabel = endpointCountLabel(),
                savedEndpointSummary = savedEndpointSummary,
                destination = SettingsDestination.Root,
                serviceTitle = if (hasPaired) "等待连接" else "等待配对",
                serviceSubtitle = if (hasPaired) "准备恢复历史数据" else "等待服务确认",
                serviceColor = if (hasPaired) Color(0xFF8E98A7) else Color(0xFFFFC857),
                serviceHostLabel = serviceHostLabel,
                updatedAtLabel = latestSnapshot?.let { buildUpdatedLabel(it.observedAt, fresh = false) } ?: "尚未刷新",
                syncStatusLabel = if (hasPaired) "等待连接" else "等待数据",
                healthCheck = ServiceHealthUiState(),
            ),
            offline = buildOfflineState("无法连接到 OpenWatcher"),
        )
    }

    private fun shouldStartWatchBootstrapFlow(): Boolean {
        return watchBootstrapClient != null &&
            watchBootstrapCodeStore != null &&
            !serverConfigRepository.hasAnyStoredConfig()
    }

    private fun startWatchBootstrapFlow() {
        isBootstrapping = false
        isServiceDegraded = false
        hasPaired = false
        currentToken = ""
        latestSnapshot = null
        val initialCode = watchBootstrapCodeStore?.currentCode().orEmpty()
        _uiState.value = _uiState.value.copy(
            screen = AppScreen.Pairing,
            pairing = buildWatchBootstrapPairingState(
                bootstrapCode = initialCode,
                status = if (initialCode.isBlank()) "正在申请临时配置码" else "临时配置码",
                detail = WATCH_BOOTSTRAP_SETUP_HINT,
            ),
            dashboard = DashboardUiState(serviceHostLabel = ""),
            settings = _uiState.value.settings.copy(
                destination = SettingsDestination.Root,
                serviceTitle = "等待远程配置",
                serviceSubtitle = "手表尚未保存 API 基址",
                serviceColor = Color(0xFF58A6FF),
                serviceHostLabel = "",
                updatedAtLabel = "尚未刷新",
                syncStatusLabel = "等待配置",
                healthCheck = ServiceHealthUiState(),
            ),
            offline = buildOfflineState("等待远程配置"),
        )

        watchBootstrapJob = viewModelScope.launch(ioDispatcher) {
            runWatchBootstrapLoop()
        }
    }

    private suspend fun runWatchBootstrapLoop() {
        val client = watchBootstrapClient ?: return
        val store = watchBootstrapCodeStore ?: return
        var bootstrapCode = store.currentCode().orEmpty()
        if (bootstrapCode.isBlank()) {
            bootstrapCode = runCatching {
                client.register(config.deviceName, formatVersionLabel(config.appVersion, config.appVersionCode)).bootstrapCode
            }.getOrElse {
                applyWatchBootstrapError(watchBootstrapUnavailableMessage())
                waitForStoredConfigAfterWatchBootstrapError()
                return
            }
            store.save(bootstrapCode)
        }
        applyWatchBootstrapWaitingState(bootstrapCode)

        var attempt = 0
        while (!serverConfigRepository.hasAnyStoredConfig()) {
            val pollResult = runCatching { client.poll(bootstrapCode) }
            if (pollResult.isFailure && shouldRenewWatchBootstrapCode(pollResult.exceptionOrNull())) {
                store.clear()
                bootstrapCode = runCatching {
                    client.register(config.deviceName, formatVersionLabel(config.appVersion, config.appVersionCode)).bootstrapCode
                }.getOrElse {
                    applyWatchBootstrapError(watchBootstrapUnavailableMessage())
                    waitForStoredConfigAfterWatchBootstrapError()
                    return
                }
                store.save(bootstrapCode)
                attempt = 0
            }
            when (val result = pollResult.getOrNull()) {
                is WatchBootstrapPollResult.Ready -> {
                    applyRemoteBootstrapConfig(result.config)
                    store.clear()
                    initializeRemoteBootstrapPairing()
                    return
                }

                WatchBootstrapPollResult.Pending,
                null,
                -> {
                    applyWatchBootstrapWaitingState(bootstrapCode)
                    delay(watchBootstrapPollDelayMs(attempt))
                    attempt += 1
                }
            }
        }
        initializeStoredConfigAfterWatchBootstrap()
    }

    private fun applyWatchBootstrapWaitingState(bootstrapCode: String) {
        _uiState.value = _uiState.value.copy(
            screen = AppScreen.Pairing,
            pairing = buildWatchBootstrapPairingState(
                bootstrapCode = bootstrapCode,
                status = "临时配置码",
                detail = WATCH_BOOTSTRAP_SETUP_HINT,
            ),
        )
    }

    private fun applyWatchBootstrapError(message: String) {
        _uiState.value = _uiState.value.copy(
            screen = AppScreen.Pairing,
            pairing = buildWatchBootstrapPairingState(
                bootstrapCode = watchBootstrapCodeStore?.currentCode().orEmpty(),
                status = "临时配置不可用",
                detail = message,
            ),
        )
    }

    private suspend fun waitForStoredConfigAfterWatchBootstrapError() {
        while (!serverConfigRepository.hasAnyStoredConfig()) {
            delay(WATCH_BOOTSTRAP_CONFIG_CHECK_INTERVAL_MS)
        }
        initializeStoredConfigAfterWatchBootstrap()
    }

    private fun initializeStoredConfigAfterWatchBootstrap() {
        watchBootstrapCodeStore?.clear()
        if (!serverConfigRepository.hasAnyStoredConfig()) {
            return
        }
        startConfiguredBootstrapFlow()
    }

    private fun shouldRenewWatchBootstrapCode(error: Throwable?): Boolean {
        return (error as? WatchBootstrapException)?.code in RENEWABLE_WATCH_BOOTSTRAP_ERROR_CODES
    }

    private fun watchBootstrapUnavailableMessage(): String {
        return "官方初始配置通道异常，请检查手表网络。$WATCH_BOOTSTRAP_SETUP_HINT"
    }

    private fun applyRemoteBootstrapConfig(config: ai.openwatcher.watchapp.data.WatchBootstrapConfig) {
        val token = tokenRepository.ensureToken()
        serverConfigRepository.save(
            channel = config.environment,
            endpoints = listOf(
                ServerEndpoint(
                    id = "remote-bootstrap",
                    label = if (config.environment == AppUpdateChannel.Dev) "远程 dev" else "远程 beta",
                    url = config.apiBase,
                    priority = 0,
                ),
            ),
            source = ServerConfigSource.RemoteBootstrap,
            deviceToken = token,
            deviceName = this.config.deviceName,
        )
        persistAppUpdatePreferences(appUpdatePreferences.copy(selectedChannel = config.environment))
        applyRuntimeServerConfig(serverConfigRepository.current(config.environment))
        currentToken = currentRuntimeToken()
        hasPaired = false
        updateDiagnosticPairedState()
    }

    private fun promoteRemoteBootstrapConfigIfNeeded() {
        val channel = activeEnvironment()
        val profile = serverConfigRepository.profile(channel) ?: return
        if (profile.source != ServerConfigSource.RemoteBootstrap) {
            return
        }
        serverConfigRepository.save(
            channel = channel,
            endpoints = profile.endpoints,
            source = ServerConfigSource.DesktopBootstrap,
            deviceToken = profile.deviceToken,
            deviceName = profile.deviceName,
            activeEndpointId = profile.activeEndpointId,
            configuredAt = profile.configuredAt,
        )
        applyRuntimeServerConfig(serverConfigRepository.current(channel))
    }

    private fun initializeRemoteBootstrapPairing() {
        pollJob?.cancel()
        resetSelectionAndTransientState()
        applyRuntimeServerConfig(serverConfigRepository.current(activeEnvironment()))
        currentToken = currentRuntimeToken()
        hasPaired = false
        isBootstrapping = false
        isServiceDegraded = false
        updateDiagnosticPairedState()
        _uiState.value = _uiState.value.copy(
            screen = AppScreen.Pairing,
            pairing = buildPairingState(
                status = "等待手机扫码",
                hint = "API 基址已保存，请扫码完成配对",
                service = "等待服务确认",
                color = Color(0xFFFFC857),
                scanStepCompleted = false,
                confirmStepActive = false,
                authStepCompleted = false,
            ),
            dashboard = DashboardUiState(serviceHostLabel = serviceHostLabel),
            settings = _uiState.value.settings.copy(
                baseUrl = currentBaseUrl,
                activeEndpointLabel = activeEndpointLabel,
                savedEndpointCountLabel = endpointCountLabel(),
                savedEndpointSummary = savedEndpointSummary,
                destination = SettingsDestination.Root,
                serviceTitle = "等待配对",
                serviceSubtitle = "API 基址已保存",
                serviceColor = Color(0xFFFFC857),
                serviceHostLabel = serviceHostLabel,
                updatedAtLabel = "尚未刷新",
                syncStatusLabel = "等待数据",
                healthCheck = ServiceHealthUiState(),
            ),
            offline = buildOfflineState("无法连接到 OpenWatcher"),
        )
        if (autoPolling) {
            startPairingLoop()
        }
    }

    private fun restartPairing(resetToken: Boolean) {
        pollJob?.cancel()
        stopStatusStream()
        stopLocalRefreshTicker()
        stopSessionStream(clearMessage = true)
        stopDetailWindowStream(clearState = true)
        homeQuotaTipHideJob?.cancel()
        homeQuotaTipHideJob = null
        pendingBootstrapContinuation = false
        isBootstrapping = false
        isServiceDegraded = false
        hasPaired = false
        updateDiagnosticPairedState()
        pairingStateStore.clear()
        statusSnapshotStore.clear()
        sessionDetailsWindowStore.clear()
        latestSnapshot = null
        resetSelectionAndTransientState()
        persistAppUpdatePreferences(appUpdatePreferences.copy(selectedChannel = AppUpdateChannel.Beta))
        applyRuntimeServerConfig(serverConfigRepository.current(AppUpdateChannel.Beta))
        currentToken = if (resetToken) tokenRepository.regenerate() else currentRuntimeToken()

        _uiState.value = _uiState.value.copy(
            screen = AppScreen.Pairing,
            pairing = buildPairingState(
                status = "等待手机扫码",
                hint = "请在手机端扫码配对",
                service = "等待服务确认",
                color = Color(0xFFFFC857),
                scanStepCompleted = false,
                confirmStepActive = false,
                authStepCompleted = false,
            ),
            dashboard = DashboardUiState(serviceHostLabel = serviceHostLabel),
            settings = _uiState.value.settings.copy(
                baseUrl = currentBaseUrl,
                activeEndpointLabel = activeEndpointLabel,
                savedEndpointCountLabel = endpointCountLabel(),
                savedEndpointSummary = savedEndpointSummary,
                destination = SettingsDestination.Root,
                serviceTitle = "等待配对",
                serviceSubtitle = "等待服务确认",
                serviceColor = Color(0xFFFFC857),
                serviceHostLabel = serviceHostLabel,
                updatedAtLabel = "尚未刷新",
                syncStatusLabel = "等待数据",
                healthCheck = ServiceHealthUiState(),
            ),
            offline = buildOfflineState("无法连接到 OpenWatcher"),
        )
        if (autoPolling) {
            startPairingLoop()
        }
    }

    private fun startPairingLoop() {
        pollJob?.cancel()
        pollJob = viewModelScope.launch(ioDispatcher) {
            while (true) {
                fetchStatus(showRefreshing = false, fromPairingLoop = true)
                delay(3_000)
            }
        }
    }

    private suspend fun applyBootstrapResult(result: StatusFetchResult) {
        if (pendingBootstrapRequest != null) {
            return
        }
        when (result) {
            is StatusFetchResult.Success -> handleSuccess(result.snapshot)
            StatusFetchResult.Unauthorized -> handleUnauthorized(fromPairingLoop = true)
            is StatusFetchResult.NetworkFailure -> {
                if (!trySwitchEndpointAfterTransportFailure(fromPairingLoop = false)) {
                    handleNetworkFailure(fromPairingLoop = false, message = result.message)
                }
            }
            is StatusFetchResult.ParseFailure -> handleParseFailure(fromPairingLoop = false, message = result.message)
            is StatusFetchResult.HttpFailure -> {
                if (!trySwitchEndpointAfterTransportFailure(fromPairingLoop = false)) {
                    handleHttpFailure(fromPairingLoop = false, message = result.message)
                }
            }
        }
        isBootstrapping = false
        if (!hasPaired && autoPolling) {
            startPairingLoop()
        } else if (hasPaired && autoPolling && isForegroundVisible) {
            startStatusStream()
            startLocalRefreshTicker()
            updateSessionStreamForCurrentScreen(clearMessage = false)
        }
    }

    private suspend fun fetchStatus(
        showRefreshing: Boolean,
        fromPairingLoop: Boolean,
    ) {
        refreshMutex.withLock {
            if (pendingBootstrapRequest != null) {
                return
            }
            if (showRefreshing) {
                _uiState.value = _uiState.value.copy(
                    dashboard = _uiState.value.dashboard.copy(
                        serviceStatus = ServiceStatus.Refreshing,
                        serviceLabel = "刷新中",
                        serviceColor = Color(0xFF58A6FF),
                    ),
                )
            }

            when (val result = api.fetchStatus(currentToken)) {
                is StatusFetchResult.Success -> handleSuccess(result.snapshot)
                StatusFetchResult.Unauthorized -> handleUnauthorized(fromPairingLoop)
                is StatusFetchResult.NetworkFailure -> {
                    if (!trySwitchEndpointAfterTransportFailure(fromPairingLoop)) {
                        handleNetworkFailure(fromPairingLoop, result.message)
                    }
                }
                is StatusFetchResult.ParseFailure -> handleParseFailure(fromPairingLoop, result.message)
                is StatusFetchResult.HttpFailure -> {
                    if (!trySwitchEndpointAfterTransportFailure(fromPairingLoop)) {
                        handleHttpFailure(fromPairingLoop, result.message)
                    }
                }
            }
        }
    }

    private fun startStatusStream() {
        if (statusStreamJob?.isActive == true || currentToken.isBlank() || isBootstrapping) {
            return
        }
        statusStreamJob = viewModelScope.launch {
            runStatusStreamLoop()
        }
    }

    private suspend fun runStatusStreamLoop() {
        var reconnectAttempt = 0
        while (shouldKeepStatusStream()) {
            var terminalFailure: StatusStreamEvent.Failure? = null
            api.streamStatus(
                token = currentToken,
                includeDailyTrend30d = shouldRequestDailyTrend30d(),
            )
                .flowOn(ioDispatcher)
                .catch { error ->
                    if (error is CancellationException) {
                        throw error
                    }
                    emit(unexpectedStatusStreamFailure(error))
                }
                .collect { event ->
                    if (event !is StatusStreamEvent.Failure || !event.terminal) {
                        reconnectAttempt = 0
                    }
                    if (event is StatusStreamEvent.Failure && event.terminal) {
                        terminalFailure = event
                    }
                    handleStatusStreamEvent(event)
                }

            if (!shouldKeepStatusStream()) {
                return
            }

            val failure = terminalFailure ?: StatusStreamEvent.Failure(
                message = "状态流连接失败",
                reason = StatusStreamFailureReason.StreamClosed,
                retryable = true,
                terminal = true,
                detail = "collector_completed",
            )
            if (!failure.retryable) {
                return
            }
            reconnectAttempt += 1
            delay(reconnectDelayForAttempt(reconnectAttempt))
        }
    }

    private fun shouldKeepStatusStream(): Boolean {
        return hasPaired && isForegroundVisible && currentToken.isNotBlank() && !isBootstrapping
    }

    private fun stopStatusStream() {
        statusStreamJob?.cancel()
        statusStreamJob = null
    }

    private fun startLocalRefreshTicker() {
        if (localRefreshJob?.isActive == true || isBootstrapping) {
            return
        }
        localRefreshJob = viewModelScope.launch {
            while (hasPaired && isForegroundVisible && !isBootstrapping) {
                delay(60_000)
                refreshDerivedState()
            }
        }
    }

    private fun stopLocalRefreshTicker() {
        localRefreshJob?.cancel()
        localRefreshJob = null
    }

    private fun stopSessionRuntimeDurationTicker() {
        sessionRuntimeDurationTickerJob?.cancel()
        sessionRuntimeDurationTickerJob = null
    }

    private fun updateSessionRuntimeDurationTicker() {
        if (!isForegroundVisible || !hasActiveRuntimeDuration()) {
            stopSessionRuntimeDurationTicker()
            return
        }
        if (sessionRuntimeDurationTickerJob?.isActive == true) {
            return
        }
        val job = viewModelScope.launch {
            while (isForegroundVisible && hasActiveRuntimeDuration()) {
                delay(SESSION_RUNTIME_DURATION_TICK_MS)
                if (!isForegroundVisible || !hasActiveRuntimeDuration()) {
                    break
                }
                refreshDerivedState()
            }
        }
        sessionRuntimeDurationTickerJob = job
    }

    private fun hasActiveRuntimeDuration(): Boolean {
        if (latestSnapshot?.sessions.orEmpty().any { it.contextCompaction?.startedAt != null }) {
            return true
        }
        if (detailWindowState.entriesByThreadId.values.any { it.session.contextCompaction?.startedAt != null }) {
            return true
        }
        val selectedThreadId = selectedSessionThreadId ?: return false
        val runtimeState = detailWindowState.entriesByThreadId[selectedThreadId]?.runtimeState
            ?: runtimeStateForThread(selectedThreadId)
            ?: return false
        val turnDuration = sessionTurnDurationForThread(selectedThreadId) ?: return false
        return runtimeState.running && turnDuration.startedAt != null
    }

    private fun handleStatusStreamEvent(event: StatusStreamEvent) {
        when (event) {
            is StatusStreamEvent.Snapshot -> handleSuccess(event.snapshot)
            is StatusStreamEvent.Quota -> mergeStatusSnapshot(
                observedAt = event.observedAt,
                quota = event.quota,
            )
            is StatusStreamEvent.Heatmap24h -> mergeStatusSnapshot(
                observedAt = event.observedAt,
                heatmap24h = event.heatmap24h,
                heatmap7d = event.heatmap7d,
                dailyUsage = event.dailyUsage,
            )
            is StatusStreamEvent.Sessions -> mergeStatusSnapshot(
                observedAt = event.observedAt,
                sessions = event.sessions,
            )
            is StatusStreamEvent.Errors -> mergeStatusSnapshot(
                observedAt = event.observedAt,
                errors = event.errors,
            )
            StatusStreamEvent.Heartbeat -> Unit
            is StatusStreamEvent.Failure -> handleStatusStreamFailure(event)
        }
        if (event !is StatusStreamEvent.Failure) {
            schedulePendingScreenshotUpload()
        }
    }

    private fun mergeStatusSnapshot(
        observedAt: Instant?,
        quota: QuotaSnapshot? = latestSnapshot?.quota,
        heatmap24h: Heatmap24hSnapshot? = latestSnapshot?.heatmap24h,
        heatmap7d: Heatmap7dSnapshot? = latestSnapshot?.heatmap7d,
        dailyUsage: DailyUsageSnapshot? = latestSnapshot?.dailyUsage,
        dailyTrend30d: DailyTrend30dSnapshot? = latestSnapshot?.dailyTrend30d,
        sessions: List<SessionSnapshot> = latestSnapshot?.sessions.orEmpty(),
        errors: List<String> = latestSnapshot?.errors.orEmpty(),
    ) {
        val previous = latestSnapshot ?: return
        val nextSnapshot = previous.copy(
            observedAt = observedAt ?: previous.observedAt,
            quota = quota,
            heatmap24h = heatmap24h,
            heatmap7d = heatmap7d,
            dailyUsage = dailyUsage,
            dailyTrend30d = dailyTrend30d,
            sessions = sessions,
            errors = errors,
        )
        logContextCompactionTransitions(nextSnapshot.sessions, source = "status_update")
        latestSnapshot = nextSnapshot
        statusSnapshotStore.write(requireNotNull(latestSnapshot))
        refreshDerivedState()
        updateSessionStreamForCurrentScreen(clearMessage = false)
    }

    private fun logContextCompactionTransitions(sessions: List<SessionSnapshot>, source: String) {
        val next = sessions.mapNotNull { session ->
            val compaction = session.contextCompaction ?: return@mapNotNull null
            session.threadId to contextCompactionSignature(compaction)
        }.toMap()
        val previous = observedContextCompactions
        observedContextCompactions = next

        sessions.forEach { session ->
            val compaction = session.contextCompaction ?: return@forEach
            val signature = next[session.threadId] ?: return@forEach
            if (previous[session.threadId] == signature) {
                return@forEach
            }
            viewModelScope.launch {
                diagnosticEventLogger.log(
                    event = "context_compaction_detected",
                    fields = mapOf(
                        "compaction" to mapOf(
                            "source" to source,
                            "threadId" to session.threadId,
                            "trigger" to compaction.trigger,
                            "turnId" to compaction.turnId,
                            "startedAt" to compaction.startedAt?.toString(),
                            "updatedAt" to compaction.updatedAt?.toString(),
                        ),
                    ),
                )
            }
        }

        previous.keys
            .filter { it !in next }
            .forEach { threadId ->
                viewModelScope.launch {
                    diagnosticEventLogger.log(
                        event = "context_compaction_cleared",
                        fields = mapOf(
                            "compaction" to mapOf(
                                "source" to source,
                                "threadId" to threadId,
                                "previous" to previous[threadId],
                            ),
                        ),
                    )
                }
            }
    }

    private fun contextCompactionSignature(compaction: ContextCompactionSnapshot): String {
        return listOf(
            compaction.trigger,
            compaction.turnId.orEmpty(),
            compaction.startedAt?.toString().orEmpty(),
            compaction.updatedAt?.toString().orEmpty(),
        ).joinToString("|")
    }

    private fun handleStatusStreamFailure(event: StatusStreamEvent.Failure) {
        when (event.reason) {
            StatusStreamFailureReason.Unauthorized -> handleUnauthorized(fromPairingLoop = false)
            StatusStreamFailureReason.ParseError -> handleParseFailure(fromPairingLoop = false, message = event.message)
            StatusStreamFailureReason.HttpError,
            StatusStreamFailureReason.ServerError,
            -> handleHttpFailure(fromPairingLoop = false, message = event.message)
            StatusStreamFailureReason.NetworkError,
            StatusStreamFailureReason.StreamClosed,
            -> handleNetworkFailure(fromPairingLoop = false, message = event.message)
        }
    }

    private fun handleSuccess(snapshot: WatcherStatusSnapshot) {
        hasPaired = true
        updateDiagnosticPairedState()
        if (activeEnvironment() == AppUpdateChannel.Beta) {
            pairingStateStore.markPaired()
        }
        promoteRemoteBootstrapConfigIfNeeded()
        isServiceDegraded = false
        val nextSnapshot = mergeCachedDailyTrend(snapshot)
        logContextCompactionTransitions(nextSnapshot.sessions, source = "status_snapshot")
        latestSnapshot = nextSnapshot
        statusSnapshotStore.write(requireNotNull(latestSnapshot))
        val currentScreen = _uiState.value.screen
        val currentPagerPage = _uiState.value.dashboard.pagerPage
        val dashboardState = buildDashboardState(requireNotNull(latestSnapshot), currentPagerPage)
        _uiState.value = _uiState.value.copy(
            screen = when (currentScreen) {
                AppScreen.Splash -> AppScreen.Dashboard
                AppScreen.Settings -> AppScreen.Settings
                AppScreen.Heatmap24h -> AppScreen.Heatmap24h
                AppScreen.SessionDetails -> AppScreen.SessionDetails
                else -> AppScreen.Dashboard
            },
            pairing = buildPairingState(
                status = "已配对",
                hint = "手机确认后已建立连接",
                service = "服务在线",
                color = Color(0xFF65D46E),
                scanStepCompleted = true,
                confirmStepActive = false,
                authStepCompleted = true,
            ),
            dashboard = dashboardState,
            settings = buildSettingsState(dashboardState),
            offline = buildOfflineState("无法连接到 OpenWatcher"),
        )
        if (_uiState.value.screen !in listOf(AppScreen.Settings, AppScreen.Heatmap24h, AppScreen.SessionDetails)) {
            _uiState.value = _uiState.value.copy(screen = AppScreen.Dashboard)
        }
        pollJob?.cancel()
        pollJob = null
        if (autoPolling && isForegroundVisible && !isBootstrapping) {
            startStatusStream()
            startLocalRefreshTicker()
        }
        updateSessionStreamForCurrentScreen(clearMessage = false)
        schedulePendingScreenshotUpload()
    }

    private fun handleUnauthorized(fromPairingLoop: Boolean) {
        if (activeEnvironment() == AppUpdateChannel.Dev) {
            if (serverConfigRepository.profile(AppUpdateChannel.Dev)?.source == ServerConfigSource.RemoteBootstrap) {
                hasPaired = false
                updateDiagnosticPairedState()
                _uiState.value = _uiState.value.copy(
                    screen = AppScreen.Pairing,
                    pairing = buildPairingState(
                        status = "等待服务确认",
                        hint = "等待手机侧确认 dev 配对",
                        service = "服务已连接",
                        color = Color(0xFF65D46E),
                        scanStepCompleted = true,
                        confirmStepActive = true,
                        authStepCompleted = false,
                    ),
                )
                if (!fromPairingLoop && autoPolling && !isBootstrapping) {
                    startPairingLoop()
                }
                return
            }
            applyPairedServiceFailure(
                serviceTitle = "开发环境 token 错误",
                serviceLabel = "开发环境异常",
                serviceColor = Color(0xFFFF7070),
                syncStatusLabel = "请重新发送开发环境",
                detailMessage = "当前 dev 环境授权无效，请重新发送开发环境，或手动切回 beta。",
                status = ServiceStatus.TokenError,
            )
            return
        }
        stopSessionStream(clearMessage = true)
        stopDetailWindowStream(clearState = true)
        val statusText = if (hasPaired) "token 错误" else "等待服务确认"
        val serviceText = if (hasPaired) "需要重新配对" else "服务已连接"
        pairingStateStore.clear()
        statusSnapshotStore.clear()
        sessionDetailsWindowStore.clear()
        isServiceDegraded = false
        latestSnapshot = null
        _uiState.value = _uiState.value.copy(
            screen = AppScreen.Pairing,
            pairing = buildPairingState(
                status = statusText,
                hint = if (hasPaired) "本地 token 已失效，请重新配对" else "等待手机侧确认配对",
                service = serviceText,
                color = if (hasPaired) Color(0xFFFF7070) else Color(0xFF65D46E),
                scanStepCompleted = !hasPaired,
                confirmStepActive = !hasPaired,
                authStepCompleted = false,
            ),
            dashboard = _uiState.value.dashboard.copy(
                serviceStatus = if (hasPaired) ServiceStatus.TokenError else ServiceStatus.WaitingPairing,
                serviceLabel = if (hasPaired) "token 错误" else "等待配对",
                serviceColor = if (hasPaired) Color(0xFFFF7070) else Color(0xFFFFC857),
                syncStatusLabel = if (hasPaired) "需要重新配对" else "等待数据",
                errors = emptyList(),
            ),
            settings = _uiState.value.settings.copy(
                activeEndpointLabel = activeEndpointLabel,
                savedEndpointCountLabel = endpointCountLabel(),
                savedEndpointSummary = savedEndpointSummary,
                serviceTitle = if (hasPaired) "需要重新配对" else "等待配对",
                serviceSubtitle = serviceText,
                serviceColor = if (hasPaired) Color(0xFFFF7070) else Color(0xFFFFC857),
                syncStatusLabel = if (hasPaired) "需要重新配对" else "等待数据",
            ),
        )
        hasPaired = false
        updateDiagnosticPairedState()
        if (!fromPairingLoop && autoPolling && !isBootstrapping) {
            startPairingLoop()
        }
    }

    private fun handleNetworkFailure(fromPairingLoop: Boolean, message: String) {
        if (fromPairingLoop || !hasPaired) {
            _uiState.value = _uiState.value.copy(
                screen = AppScreen.Pairing,
                pairing = buildPairingState(
                    status = "服务不可达",
                    hint = "检查网络后再试",
                    service = message,
                    color = Color(0xFFFF7070),
                    scanStepCompleted = false,
                    confirmStepActive = false,
                    authStepCompleted = false,
                ),
                settings = _uiState.value.settings.copy(
                    activeEndpointLabel = activeEndpointLabel,
                    savedEndpointCountLabel = endpointCountLabel(),
                    savedEndpointSummary = savedEndpointSummary,
                    serviceTitle = "服务不可达",
                    serviceSubtitle = message,
                    serviceColor = Color(0xFFFF7070),
                    syncStatusLabel = "连接失败",
                ),
            )
        } else {
            applyPairedServiceFailure(
                serviceTitle = "服务不可达",
                serviceLabel = "离线",
                serviceColor = Color(0xFFFF7070),
                syncStatusLabel = "连接失败",
                detailMessage = message,
                status = ServiceStatus.Offline,
            )
        }
    }

    private fun handleParseFailure(fromPairingLoop: Boolean, message: String) {
        if (fromPairingLoop || !hasPaired) {
            _uiState.value = _uiState.value.copy(
                screen = AppScreen.Pairing,
                pairing = buildPairingState(
                    status = "返回异常",
                    hint = "服务返回格式异常",
                    service = message,
                    color = Color(0xFFFF7070),
                    scanStepCompleted = false,
                    confirmStepActive = false,
                    authStepCompleted = false,
                ),
                settings = _uiState.value.settings.copy(
                    activeEndpointLabel = activeEndpointLabel,
                    savedEndpointCountLabel = endpointCountLabel(),
                    savedEndpointSummary = savedEndpointSummary,
                    serviceTitle = "返回异常",
                    serviceSubtitle = message,
                    serviceColor = Color(0xFFFF7070),
                    syncStatusLabel = "返回异常",
                ),
            )
        } else {
            applyPairedServiceFailure(
                serviceTitle = "解析失败",
                serviceLabel = "解析失败",
                serviceColor = Color(0xFFFF7070),
                syncStatusLabel = "返回异常",
                detailMessage = message,
                status = ServiceStatus.ParseFailure,
            )
        }
    }

    private fun handleHttpFailure(fromPairingLoop: Boolean, message: String) {
        if (fromPairingLoop || !hasPaired) {
            _uiState.value = _uiState.value.copy(
                screen = AppScreen.Pairing,
                pairing = buildPairingState(
                    status = "等待服务连接",
                    hint = "服务暂时未响应",
                    service = message,
                    color = Color(0xFFFFC857),
                    scanStepCompleted = false,
                    confirmStepActive = false,
                    authStepCompleted = false,
                ),
                settings = _uiState.value.settings.copy(
                    activeEndpointLabel = activeEndpointLabel,
                    savedEndpointCountLabel = endpointCountLabel(),
                    savedEndpointSummary = savedEndpointSummary,
                    serviceTitle = "服务异常",
                    serviceSubtitle = message,
                    serviceColor = Color(0xFFFFC857),
                    syncStatusLabel = "服务异常",
                ),
            )
        } else {
            applyPairedServiceFailure(
                serviceTitle = "服务异常",
                serviceLabel = "服务异常",
                serviceColor = Color(0xFFFFC857),
                syncStatusLabel = "服务异常",
                detailMessage = message,
                status = ServiceStatus.Offline,
            )
        }
    }

    private fun buildPairingState(
        status: String,
        hint: String,
        service: String,
        color: Color,
        scanStepCompleted: Boolean,
        confirmStepActive: Boolean,
        authStepCompleted: Boolean,
    ): PairingUiState {
        val token = pairingToken()
        return PairingUiState(
            statusLabel = status,
            hintLabel = hint,
            serviceLabel = service,
            serviceHostLabel = serviceHostLabel,
            serviceBaseUrl = pairingBaseUrl(),
            environmentLabel = currentPairingEnvironmentLabel(),
            serviceColor = color,
            tokenFingerprint = tokenRepository.tokenFingerprint(token),
            qrPayload = tokenRepository.buildPairingPayload(
                baseUrl = pairingBaseUrl(),
                token = token,
                deviceName = config.deviceName,
            ),
            scanStepCompleted = scanStepCompleted,
            confirmStepActive = confirmStepActive,
            authStepCompleted = authStepCompleted,
        )
    }

    private fun buildWatchBootstrapPairingState(
        bootstrapCode: String,
        status: String,
        detail: String,
    ): PairingUiState {
        return PairingUiState(
            statusLabel = status,
            hintLabel = detail,
            serviceLabel = "等待远程配置",
            serviceHostLabel = "openwatcher.ai",
            serviceBaseUrl = "",
            environmentLabel = "",
            serviceColor = Color(0xFF58A6FF),
            tokenFingerprint = "",
            qrPayload = "",
            bootstrapCode = bootstrapCode,
            bootstrapDetailLabel = detail,
            scanStepCompleted = false,
            confirmStepActive = false,
            authStepCompleted = false,
        )
    }

    private fun buildDashboardState(
        snapshot: WatcherStatusSnapshot,
        pagerPage: DashboardPage = DashboardPage.Home,
    ): DashboardUiState {
        val currentQuotaEasterEgg = _uiState.value.dashboard.home.quotaEasterEgg
        val quotaStatus = snapshot.quota?.status ?: QuotaStatus.Unavailable
        val quotaDimmed = isServiceDegraded || quotaStatus != QuotaStatus.Ok
        val heatmapState = buildHeatmapState(
            heatmap = snapshot.heatmap24h,
            dailyUsage = snapshot.dailyUsage,
            dailyTrend30d = snapshot.dailyTrend30d,
            observedAt = snapshot.observedAt,
        )
        val sessionDetailsState = buildSessionDetailsState(snapshot.sessions)
        val selectedSession = snapshot.sessions.firstOrNull { it.threadId == selectedSessionThreadId }
            ?: snapshot.sessions.firstOrNull()
        val selectedRuntime = selectedSession?.threadId?.let(::runtimeStateForThread)
        val selectedTurnDuration = selectedSession?.threadId?.let(::sessionTurnDurationForThread)
        val selectedCompaction = selectedSession?.contextCompaction
        val selectedSessionPressure = selectedSession?.contextPressurePercent?.toFloat() ?: 0f
        val selectedSessionCompactThreshold = selectedSession?.compactThresholdPercent()
        val selectedLastActiveLabel = selectedSession?.let(::lastActiveLabel) ?: "--"
        val selectedActiveLabel = formatHomeActivityLabel(
            isRunning = selectedRuntime?.running == true,
            isCompacting = selectedCompaction != null,
            recentLabel = selectedLastActiveLabel,
            durationLabel = selectedCompaction?.durationLabel() ?: selectedTurnDuration?.durationLabel(),
        )
        return DashboardUiState(
            pagerPage = pagerPage,
            serviceStatus = if (isServiceDegraded) ServiceStatus.Offline else ServiceStatus.Online,
            serviceLabel = if (isServiceDegraded) "离线" else "在线",
            serviceColor = if (isServiceDegraded) Color(0xFF8E98A7) else Color(0xFF65D46E),
            updatedAtLabel = buildUpdatedLabel(
                snapshot.observedAt,
                fresh = !isServiceDegraded && quotaStatus == QuotaStatus.Ok,
            ),
            serviceHostLabel = serviceHostLabel,
            syncStatusLabel = when {
                isServiceDegraded -> "使用缓存"
                quotaStatus == QuotaStatus.Stale -> "额度缓存"
                quotaStatus == QuotaStatus.Unavailable -> "额度不可用"
                else -> "数据已同步"
            },
            isServiceDegraded = isServiceDegraded,
            home = HomeDashboardUiState(
                fiveHour = snapshot.quota?.fiveHour.toRingState("5h", snapshot.observedAt, FIVE_HOUR_WINDOW_SECONDS, quotaDimmed) ?: QuotaRingUiState(
                    title = "5h",
                    remainingLabel = "--",
                    isDimmed = quotaDimmed,
                ),
                weekly = snapshot.quota?.weekly.toRingState("weekly", snapshot.observedAt, WEEKLY_WINDOW_SECONDS, quotaDimmed) ?: QuotaRingUiState(
                    title = "weekly",
                    remainingLabel = "--",
                    isDimmed = quotaDimmed,
                ),
                miniBars = heatmapState.segments.map { MiniBarUiState(it.hourLabel, it.intensity.coerceIn(0f, 1f)) },
                weeklyHeatmap = buildHomeWeeklyHeatmapState(snapshot.heatmap7d),
                selectedSessionTitle = selectedSession?.title ?: "暂无活跃会话",
                totalTokensLabel = selectedSession?.let { formatCompactTokens(it.tokensUsedTotal) } ?: "0",
                selectedSessionModel = selectedSession?.model?.let(::formatModelTitle) ?: "--",
                selectedSessionReasoning = selectedSession?.reasoningEffort?.let(::formatReasoningTitle) ?: "--",
                selectedSessionContextLabel = selectedSession?.let {
                    formatContextLabel(it.contextUsedTokens, it.contextWindow)
                } ?: "-- / --",
                selectedSessionPressurePercent = selectedSessionPressure,
                selectedSessionCompactThresholdPercent = selectedSessionCompactThreshold,
                selectedSessionCompactWarning = isNearCompactThreshold(
                    pressurePercent = selectedSessionPressure,
                    compactThresholdPercent = selectedSessionCompactThreshold,
                ),
                selectedSessionActiveLabel = selectedActiveLabel,
                selectedSessionIsActiveNow = selectedRuntime?.running == true || selectedCompaction != null,
                selectedSessionRuntimePhaseLabel = formatRuntimeStatusLine(
                    state = selectedRuntime,
                    contextCompaction = selectedCompaction,
                    lastActiveLabel = selectedLastActiveLabel,
                    runtimeTime = selectedRuntime?.let(::runtimeTimeForSelectedSession),
                    turnDuration = selectedTurnDuration,
                ),
                sessionAvailable = selectedSession != null,
                isServiceDegraded = isServiceDegraded,
                quotaEasterEgg = currentQuotaEasterEgg,
            ),
            heatmap24h = heatmapState.copy(isServiceDegraded = isServiceDegraded),
            sessionDetails = sessionDetailsState.copy(isServiceDegraded = isServiceDegraded),
            errors = snapshot.errors,
        )
    }

    private fun buildHeatmapState(
        heatmap: Heatmap24hSnapshot?,
        dailyUsage: DailyUsageSnapshot?,
        dailyTrend30d: DailyTrend30dSnapshot?,
        observedAt: Instant?,
    ): Heatmap24hUiState {
        val buckets = heatmap?.buckets.orEmpty()
        val anchor = heatmap?.generatedAt ?: observedAt ?: Instant.now()
        if (buckets.isEmpty()) {
            selectedHeatmapHourStart = null
            heatmapCursorPosition = null
            val emptyBars = buildEmptyHeatmapSegments(anchor).map {
                HeatmapBarUiState(
                    hourLabel = it.hourLabel,
                    intensity = it.intensity,
                )
            }
            return Heatmap24hUiState(
                bars = emptyBars,
                segments = buildEmptyHeatmapSegments(anchor),
                dailyUsage = buildDailyUsageState(dailyUsage, dailyTrend30d, emptyBars = true),
                isServiceDegraded = isServiceDegraded,
            )
        }

        val total24h = buckets.sumOf { it.totalTokens }
        val inputTotal = buckets.sumOf { it.inputTokens }
        val cachedTotal = buckets.sumOf { it.cachedInputTokens }
        val outputTotal = buckets.sumOf { it.outputTokens }
        val peakTotal = buckets.maxOfOrNull { it.totalTokens } ?: 0L
        val peakHour = heatmap?.peakHourStart
        val selectedBucket = buckets.firstOrNull { it.hourStart == selectedHeatmapHourStart }
            ?: buckets.lastOrNull { it.totalTokens > 0 }
            ?: buckets.last()
        val selectedIndex = buckets.indexOf(selectedBucket).coerceAtLeast(0)
        selectedHeatmapHourStart = selectedBucket.hourStart
        val segments = buckets.mapIndexed { index, bucket ->
            HeatmapSegmentUiState(
                hourLabel = bucket.hourStart?.let(HOUR_FORMATTER::format) ?: "--",
                timeRangeLabel = bucket.hourStart?.let(::formatHourRange) ?: "--",
                intensity = if (peakTotal <= 0) 0f else bucket.totalTokens.toFloat() / peakTotal.toFloat(),
                totalTokensLabel = formatCompactTokens(bucket.totalTokens),
                totalTokens = bucket.totalTokens,
                inputTokensLabel = formatCompactTokens(bucket.inputTokens),
                cachedInputTokensLabel = formatCompactTokens(bucket.cachedInputTokens),
                outputTokensLabel = formatCompactTokens(bucket.outputTokens),
                cacheHitRateLabel = formatPercent(
                    if (bucket.inputTokens <= 0L) {
                        0.0
                    } else {
                        bucket.cachedInputTokens.toDouble() / bucket.inputTokens.toDouble()
                    },
                ),
                isPeak = peakHour != null && bucket.hourStart == peakHour,
                isSelected = index == selectedIndex,
                isNonEmpty = bucket.totalTokens > 0L,
            )
        }
        val cacheHitRatio = if (inputTotal <= 0L) 0.0 else cachedTotal.toDouble() / inputTotal.toDouble()
        val cursorIndex = heatmapCursorPosition
            ?.let { wrapHeatmapCursorPosition(it, buckets.size) }
            ?: selectedIndex.toFloat()
        heatmapCursorPosition = cursorIndex

        return Heatmap24hUiState(
            totalTokensLabel = formatCompactTokens(total24h),
            peakHourLabel = peakHour?.let(::formatHourRange) ?: "--",
            selectedHourRangeLabel = segments[selectedIndex].timeRangeLabel,
            selectedHourTokensLabel = formatCompactTokens(selectedBucket.totalTokens),
            selectedIndex = selectedIndex,
            selectionCursorIndex = cursorIndex,
            bars = segments.map {
                HeatmapBarUiState(
                    hourLabel = it.hourLabel,
                    intensity = it.intensity,
                    isPeak = it.isPeak,
                )
            },
            segments = segments,
            inputLabel = formatCompactTokens(inputTotal),
            cachedInputLabel = formatCompactTokens(cachedTotal),
            outputLabel = formatCompactTokens(outputTotal),
            cacheHitRateLabel = formatPercent(cacheHitRatio),
            dailyUsage = buildDailyUsageState(dailyUsage, dailyTrend30d, fallbackBuckets = buckets),
            rotaryMode = heatmapRotaryMode,
            emptyMessage = "暂无 24h 数据",
            isServiceDegraded = isServiceDegraded,
        )
    }

    private fun buildHomeWeeklyHeatmapState(heatmap7d: Heatmap7dSnapshot?): HomeWeeklyHeatmapUiState {
        val days = heatmap7d?.days.orEmpty().takeLast(7).asReversed()

        val peakTokens = maxOf(
            heatmap7d?.peakTokens ?: 0L,
            days.maxOfOrNull { day -> day.hours.maxOrNull() ?: 0L } ?: 0L,
        )
        val rows = List(7) { rowIndex ->
            val day = days.getOrNull(rowIndex)
            val paddedHours = List(24) { index -> day?.hours?.getOrElse(index) { 0L } ?: 0L }
            HomeWeeklyHeatmapRowUiState(
                dateLabel = formatHomeWeeklyHeatmapDate(day?.date),
                cells = paddedHours.map { totalTokens ->
                    HomeWeeklyHeatmapCellUiState(
                        intensity = if (peakTokens <= 0L) 0f else {
                            (totalTokens.toFloat() / peakTokens.toFloat()).coerceIn(0f, 1f)
                        },
                        totalTokens = totalTokens,
                    )
                },
            )
        }
        return HomeWeeklyHeatmapUiState(
            available = rows.any { row -> row.cells.any { it.totalTokens > 0L } },
            rows = rows,
        )
    }

    private fun formatHomeWeeklyHeatmapDate(date: String?): String {
        if (date.isNullOrBlank()) {
            return "--.--"
        }
        return runCatching {
            LocalDate.parse(date).format(HOME_HEATMAP_DATE_FORMATTER)
        }.getOrDefault("--.--")
    }

    private fun buildDailyUsageState(
        dailyUsage: DailyUsageSnapshot?,
        dailyTrend30d: DailyTrend30dSnapshot?,
        fallbackBuckets: List<ai.openwatcher.watchapp.data.HeatmapBucket> = emptyList(),
        emptyBars: Boolean = false,
    ): DailyUsageUiState {
        val input = dailyUsage?.inputTokens ?: fallbackBuckets.sumOf { it.inputTokens }
        val cached = dailyUsage?.cachedInputTokens ?: fallbackBuckets.sumOf { it.cachedInputTokens }
        val output = dailyUsage?.outputTokens ?: fallbackBuckets.sumOf { it.outputTokens }
        val reasoning = dailyUsage?.reasoningOutputTokens ?: fallbackBuckets.sumOf { it.reasoningOutputTokens }
        val total = dailyUsage?.totalTokens ?: fallbackBuckets.sumOf { it.totalTokens }
        val activeSessions = dailyUsage?.activeSessions ?: 0
        val valueLabel = dailyUsage?.estimatedValueLabel?.takeIf { it.isNotBlank() }
            ?: dailyUsage?.pricingUnavailableReason?.takeIf { it.isNotBlank() }?.let { "--" }
            ?: "--"
        val denominator = (input + output + reasoning).coerceAtLeast(1L)
        val cacheHitRatio = if (input <= 0L) 0.0 else cached.toDouble() / input.toDouble()
        val segments = if (emptyBars || total <= 0L) {
            emptyList()
        } else {
            listOf(
                DailyUsageBarSegmentUiState(DailyUsageSegmentKind.Input, ((input - cached).coerceAtLeast(0L)).toFloat() / denominator.toFloat()),
                DailyUsageBarSegmentUiState(DailyUsageSegmentKind.CachedInput, cached.toFloat() / denominator.toFloat()),
                DailyUsageBarSegmentUiState(DailyUsageSegmentKind.Output, output.toFloat() / denominator.toFloat()),
                DailyUsageBarSegmentUiState(DailyUsageSegmentKind.ReasoningOutput, reasoning.toFloat() / denominator.toFloat()),
            ).filter { it.fraction > 0f }
        }
        return DailyUsageUiState(
            totalTokensLabel = formatCompactTokens(total),
            inputLabel = formatCompactTokens(input),
            cachedInputLabel = formatCompactTokens(cached),
            outputLabel = formatCompactTokens(output + reasoning),
            cacheHitRateLabel = formatPercent(cacheHitRatio),
            reasoningOutputLabel = formatCompactTokens(reasoning),
            activeSessionsLabel = "${activeSessions.coerceAtLeast(0)} 会话",
            estimatedValueLabel = valueLabel,
            valueCaption = if (dailyUsage?.estimatedValueLabel.isNullOrBlank()) "今日价值不可用" else "今日价值",
            segments = segments,
            modelShares = dailyUsage?.modelShares.orEmpty().map {
                DailyUsageModelShareUiState(
                    model = it.model,
                    shareLabel = "${trimZero(String.format(Locale.US, "%.1f", it.sharePercent))}%",
                    fraction = (it.sharePercent.toFloat() / 100f).coerceIn(0f, 1f),
                )
            },
            dailyTrend30d = buildDailyTrend30dState(dailyTrend30d),
        )
    }

    private fun buildDailyTrend30dState(trend: DailyTrend30dSnapshot?): DailyTrend30dUiState {
        val days = trend?.days.orEmpty()
        if (days.isEmpty()) {
            return DailyTrend30dUiState()
        }
        val minTokens = days.minOfOrNull { it.totalTokens } ?: 0L
        val maxTokens = days.maxOfOrNull { it.totalTokens }?.coerceAtLeast(1L) ?: 1L
        val range = (maxTokens - minTokens).coerceAtLeast(1L)
        val selectedIndex = selectedTrendDayIndex?.coerceIn(0, days.lastIndex) ?: -1
        val selectedDay = days.getOrNull(selectedIndex)
        return DailyTrend30dUiState(
            available = true,
            totalLabel = formatCompactTokens(trend?.totalTokens ?: days.sumOf { it.totalTokens }),
            averageLabel = formatCompactTokens(trend?.averageTokens ?: 0L),
            dayDates = days.map { it.date },
            dayLabels = days.map { formatTrendDateLabel(it.date) },
            dayTokenLabels = days.map { formatCompactTokens(it.totalTokens) },
            peakLabel = formatCompactTokens(maxTokens),
            valueLabel = trend?.estimatedValueLabel?.takeIf { it.isNotBlank() } ?: "--",
            selectedIndex = selectedIndex,
            selectedDateLabel = selectedDay?.let { formatTrendDateLabel(it.date) } ?: "--",
            selectedTokenLabel = selectedDay?.let { formatCompactTokens(it.totalTokens) } ?: "0",
            tipVisible = heatmapTrendTipVisible && selectedDay != null,
            dayFractions = if (maxTokens == minTokens) {
                List(days.size) { 0.55f }
            } else {
                days.map { ((it.totalTokens - minTokens).toFloat() / range.toFloat()).coerceIn(0f, 1f) }
            },
        )
    }

    private fun mergeCachedDailyTrend(snapshot: WatcherStatusSnapshot): WatcherStatusSnapshot {
        snapshot.dailyTrend30d?.let {
            cachedDailyTrend30d = it
            dailyTrendStore.write(it)
        }
        val expectedEndDate = expectedTrendEndDate(wallClockNow())
        val fallbackTrend = cachedDailyTrend30d?.takeIf { it.endDate == expectedEndDate }
        return snapshot.copy(
            dailyTrend30d = snapshot.dailyTrend30d ?: fallbackTrend,
        )
    }

    private fun shouldRequestDailyTrend30d(): Boolean {
        val expectedEndDate = expectedTrendEndDate(wallClockNow())
        val cachedEndDate = latestSnapshot?.dailyTrend30d?.endDate ?: cachedDailyTrend30d?.endDate
        return cachedEndDate != expectedEndDate
    }

    private fun expectedTrendEndDate(now: Instant): String {
        return now.atZone(ZoneId.of("Asia/Shanghai")).toLocalDate().minusDays(1).toString()
    }

    private fun buildSessionDetailsState(sessions: List<SessionSnapshot>): SessionDetailsUiState {
        if (detailWindowState.orderedThreadIds.isNotEmpty() && detailWindowState.entriesByThreadId.isNotEmpty()) {
            return buildSessionDetailsWindowState()
        }
        val detailSessions = sessionDetailDisplaySessions(sessions)
        if (detailSessions.isEmpty()) {
            selectedSessionThreadId = null
            selectedSessionSlotIndex = null
            sessionSelectionCursorPosition = null
            return SessionDetailsUiState(
                agentMessageStreamStatus = detailWindowState.status,
                agentMessageError = detailWindowState.error,
                isServiceDegraded = isServiceDegraded,
            )
        }
        val requestedThreadId = selectedSessionThreadId
        val selected = detailSessions.firstOrNull { it.threadId == requestedThreadId } ?: detailSessions.first()
        selectedSessionThreadId = selected.threadId
        selectedSessionSlotIndex = detailSessions.indexOfFirst { it.threadId == selected.threadId }
        val selectedRuntime = runtimeStateForThread(selected.threadId)
        val selectedCompaction = selected.contextCompaction
        val selectedTurnDuration = sessionTurnDurationForThread(selected.threadId)
        val selectedPressure = selected.contextPressurePercent.toFloat()
        val selectedCompactThreshold = selected.compactThresholdPercent()
        val selectedIndex = detailSessions.indexOfFirst { it.threadId == selected.threadId }
        val cursorIndex = selectedIndex.toFloat()
        sessionSelectionCursorPosition = cursorIndex
        val baseState = SessionDetailsUiState(
            selectedSessionTitle = selected.title,
            selectedSessionTitleMarquee = selected.title,
            selectedSessionModel = formatModelTitle(selected.model),
            selectedSessionReasoning = formatReasoningTitle(selected.reasoningEffort),
            selectedSessionActiveLabel = when {
                selectedCompaction != null -> contextCompactionTitle(selectedCompaction)
                selectedRuntime?.running == true -> "运行中"
                else -> lastActiveLabel(selected)
            },
            selectedSessionIsActiveNow = selectedRuntime?.running == true || selectedCompaction != null,
            selectedSessionRuntimePhaseLabel = formatRuntimeStatusLine(
                state = selectedRuntime,
                contextCompaction = selectedCompaction,
                lastActiveLabel = lastActiveLabel(selected),
                runtimeTime = selectedRuntime?.let(::runtimeTimeForSelectedSession),
                turnDuration = selectedTurnDuration,
            ),
            selectedSessionContextLabel = formatContextLabel(selected.contextUsedTokens, selected.contextWindow),
            selectedSessionPressurePercent = selectedPressure,
            selectedSessionCompactThresholdPercent = selectedCompactThreshold,
            selectedSessionCompactWarning = isNearCompactThreshold(
                pressurePercent = selectedPressure,
                compactThresholdPercent = selectedCompactThreshold,
            ),
            selectedSessionTokensLabel = formatCompactTokens(selected.tokensUsedTotal),
            selectedIndex = selectedIndex,
            selectionCursorIndex = cursorIndex,
            rows = detailSessions.map { session ->
                val pressure = session.contextPressurePercent.toFloat()
                val compactThreshold = session.compactThresholdPercent()
                val runtime = runtimeStateForThread(session.threadId)
                val compaction = session.contextCompaction
                val turnDuration = sessionTurnDurationForThread(session.threadId)
                val lastActiveLabel = lastActiveLabel(session)
                val activeMinutes = lastActiveAgoMinutes(session)
                SessionRowUiState(
                    sessionId = session.threadId,
                    title = session.title,
                    tokensLabel = formatCompactTokens(session.tokensUsedTotal),
                    model = formatModelTitle(session.model),
                    reasoningLabel = formatReasoningTitle(session.reasoningEffort),
                    lastActiveLabel = lastActiveLabel,
                    lastActiveAgoMinutes = activeMinutes,
                    runtimePhaseLabel = formatRuntimeStatusLine(
                        state = runtime,
                        contextCompaction = compaction,
                        lastActiveLabel = lastActiveLabel,
                        runtimeTime = runtime?.let(::runtimeTimeForSelectedSession),
                        turnDuration = turnDuration,
                    ),
                    agentStatusLine = formatAgentStatusLine(
                        state = runtime,
                        contextCompaction = compaction,
                        runtimeTime = runtime?.let(::runtimeTimeForAgentStatus),
                        activeMinutes = activeMinutes,
                        turnDuration = turnDuration,
                    ),
                    contextLabel = formatContextLabel(session.contextUsedTokens, session.contextWindow),
                    contextPressurePercent = pressure,
                    contextCompactThresholdPercent = compactThreshold,
                    contextCompactWarning = isNearCompactThreshold(
                        pressurePercent = pressure,
                        compactThresholdPercent = compactThreshold,
                    ),
                    isActiveNow = runtime?.running == true || compaction != null,
                    isSelected = session.threadId == selected.threadId,
                )
            },
            segments = detailSessions.mapIndexed { index, session ->
                SessionSegmentUiState(
                    threadId = session.threadId,
                    activeLabel = lastActiveLabel(session),
                    intensity = sessionRelativeIntensity(index),
                    isSelected = session.threadId == selected.threadId,
                )
            },
            sessionAvailable = true,
            isServiceDegraded = isServiceDegraded,
        ).withSessionAgentState(selected.threadId)
            .withDebugSessionAgentPreview(selected.threadId)
        return baseState.copy(
            agentMessageStreamStatus = when {
                _uiState.value.screen != AppScreen.SessionDetails -> AgentMessageStreamStatus.Disconnected
                !baseState.latestAgentMessage.isNullOrBlank() -> AgentMessageStreamStatus.Live
                else -> detailWindowState.status
            },
            agentMessageError = if (
                _uiState.value.screen == AppScreen.SessionDetails &&
                baseState.latestAgentMessage.isNullOrBlank()
            ) {
                detailWindowState.error
            } else {
                null
            },
        )
    }

    private fun buildSessionDetailsWindowState(): SessionDetailsUiState {
        val orderedEntries = detailWindowState.orderedThreadIds.mapNotNull(detailWindowState.entriesByThreadId::get)
        if (orderedEntries.isEmpty()) {
            selectedSessionThreadId = null
            selectedSessionSlotIndex = null
            sessionSelectionCursorPosition = null
            return SessionDetailsUiState(
                agentMessageStreamStatus = detailWindowState.status,
                agentMessageError = detailWindowState.error,
                isServiceDegraded = isServiceDegraded,
            )
        }

        val requestedThreadId = selectedSessionThreadId
        val selectedIndex = orderedEntries.indexOfFirst { it.session.threadId == requestedThreadId }
            .takeIf { it >= 0 }
            ?: selectedSessionSlotIndex?.coerceIn(0, orderedEntries.lastIndex)
            ?: 0
        val selectedEntry = orderedEntries[selectedIndex]
        val selectedSession = selectedEntry.session
        val selectedRuntime = selectedEntry.runtimeState
        val selectedCompaction = selectedSession.contextCompaction
        val selectedTurnDuration = sessionTurnDurationForThread(selectedSession.threadId)
        val selectedMessage = selectedEntry.latestMessage

        selectedSessionThreadId = selectedSession.threadId
        selectedSessionSlotIndex = selectedIndex
        sessionSelectionCursorPosition = selectedIndex.toFloat()

        val selectedPressure = selectedSession.contextPressurePercent.toFloat()
        val selectedCompactThreshold = selectedSession.compactThresholdPercent()
        val selectedLastActiveLabel = lastActiveLabel(selectedSession)

        return SessionDetailsUiState(
            selectedSessionTitle = selectedSession.title,
            selectedSessionTitleMarquee = selectedSession.title,
            selectedSessionModel = formatModelTitle(selectedSession.model),
            selectedSessionReasoning = formatReasoningTitle(selectedSession.reasoningEffort),
            selectedSessionActiveLabel = when {
                selectedCompaction != null -> contextCompactionTitle(selectedCompaction)
                selectedRuntime.running -> "运行中"
                else -> selectedLastActiveLabel
            },
            selectedSessionIsActiveNow = selectedRuntime.running || selectedCompaction != null,
            selectedSessionRuntimePhaseLabel = formatRuntimeStatusLine(
                state = selectedRuntime,
                contextCompaction = selectedCompaction,
                lastActiveLabel = selectedLastActiveLabel,
                runtimeTime = runtimeTimeForDetailWindowThread(selectedEntry),
                turnDuration = selectedTurnDuration,
            ),
            selectedSessionContextLabel = formatContextLabel(selectedSession.contextUsedTokens, selectedSession.contextWindow),
            selectedSessionPressurePercent = selectedPressure,
            selectedSessionCompactThresholdPercent = selectedCompactThreshold,
            selectedSessionCompactWarning = isNearCompactThreshold(
                pressurePercent = selectedPressure,
                compactThresholdPercent = selectedCompactThreshold,
            ),
            selectedSessionTokensLabel = formatCompactTokens(selectedSession.tokensUsedTotal),
            selectedIndex = selectedIndex,
            selectionCursorIndex = selectedIndex.toFloat(),
            rows = orderedEntries.mapIndexed { index, entry ->
                val session = entry.session
                val runtime = entry.runtimeState
                val compaction = session.contextCompaction
                val turnDuration = sessionTurnDurationForThread(session.threadId)
                val lastActiveLabel = lastActiveLabel(session)
                val compactThreshold = session.compactThresholdPercent()
                val activeMinutes = lastActiveAgoMinutes(session)
                SessionRowUiState(
                    sessionId = session.threadId,
                    title = session.title,
                    tokensLabel = formatCompactTokens(session.tokensUsedTotal),
                    model = formatModelTitle(session.model),
                    reasoningLabel = formatReasoningTitle(session.reasoningEffort),
                    lastActiveLabel = lastActiveLabel,
                    lastActiveAgoMinutes = activeMinutes,
                    runtimePhaseLabel = formatRuntimeStatusLine(
                        state = runtime,
                        contextCompaction = compaction,
                        lastActiveLabel = lastActiveLabel,
                        runtimeTime = runtimeTimeForDetailWindowThread(entry),
                        turnDuration = turnDuration,
                    ),
                    agentStatusLine = formatAgentStatusLine(
                        state = runtime,
                        contextCompaction = compaction,
                        runtimeTime = runtimeTimeForDetailWindowThread(entry),
                        activeMinutes = activeMinutes,
                        turnDuration = turnDuration,
                    ),
                    contextLabel = formatContextLabel(session.contextUsedTokens, session.contextWindow),
                    contextPressurePercent = session.contextPressurePercent.toFloat(),
                    contextCompactThresholdPercent = compactThreshold,
                    contextCompactWarning = isNearCompactThreshold(
                        pressurePercent = session.contextPressurePercent.toFloat(),
                        compactThresholdPercent = compactThreshold,
                    ),
                    isActiveNow = runtime.running || compaction != null,
                    isSelected = index == selectedIndex,
                )
            },
            segments = orderedEntries.mapIndexed { index, entry ->
                SessionSegmentUiState(
                    threadId = entry.session.threadId,
                    activeLabel = lastActiveLabel(entry.session),
                    intensity = sessionRelativeIntensity(index),
                    isSelected = index == selectedIndex,
                )
            },
            sessionAvailable = true,
            latestAgentMessage = selectedMessage?.text?.let(::cleanAgentMessageText)?.takeIf { it.isNotBlank() },
            latestAgentMessageAtLabel = selectedMessage?.createdAt?.let(TIME_FORMATTER::format),
            agentMessageStreamStatus = detailWindowDisplayStatus(
                selectedThreadId = selectedSession.threadId,
                state = detailWindowState,
            ),
            agentMessageError = detailWindowState.error,
            isServiceDegraded = isServiceDegraded,
        ).withDebugSessionAgentPreview(selectedSession.threadId)
    }

    private fun SessionDetailsUiState.withDebugSessionAgentPreview(threadId: String): SessionDetailsUiState {
        if (!latestAgentMessage.isNullOrBlank()) {
            return this
        }
        val debugScenario = debugStore?.current() ?: return this
        if (debugScenario != DebugDemoScenario.DASHBOARD && debugScenario != DebugDemoScenario.QUOTA_STALE) {
            return this
        }
        val preview = SessionAgentMessage(
            threadId = threadId,
            eventId = "debug-session-preview",
            createdAt = Instant.parse("2026-06-03T03:30:20Z"),
            text = "这张演示图专门用来验详情页中间区。上面的 token、模型和 effort 要和标题分开，中间正文要足够长，才能看出布局有没有互相压住。\n\n这里故意放一段更长的 agent 回复，包含多行中文、不同长度的句子和一个偏长的收尾段落。目标不是测试文案，而是确认详情页在长消息场景下，顶部元数据、中心正文和底部 context 读数还能同时成立。\n\n如果第一屏还能稳定显示几行正文，而且不把标题或元数据顶坏，就说明这次布局调整是有效的。",
            truncated = false,
        )
        return copy(
            latestAgentMessage = preview.text,
            latestAgentMessageAtLabel = preview.createdAt?.let(TIME_FORMATTER::format),
            agentMessageStreamStatus = AgentMessageStreamStatus.Live,
            agentMessageError = null,
        )
    }

    private fun sessionDetailDisplaySessions(sessions: List<SessionSnapshot>): List<SessionSnapshot> {
        return sessions.sortedWith(
            compareBy<SessionSnapshot> { it.lastActiveAgoMinutes }
                .thenBy { it.threadId },
        )
    }

    private fun buildSettingsState(dashboard: DashboardUiState): SettingsUiState {
        return _uiState.value.settings.copy(
            baseUrl = currentBaseUrl,
            activeEndpointLabel = activeEndpointLabel,
            savedEndpointCountLabel = endpointCountLabel(),
            savedEndpointSummary = savedEndpointSummary,
            serviceTitle = when (dashboard.serviceStatus) {
                ServiceStatus.Online -> "服务在线"
                ServiceStatus.Offline -> "服务不可达"
                ServiceStatus.WaitingPairing -> "等待配对"
                ServiceStatus.TokenError -> "需要重新配对"
                ServiceStatus.Refreshing -> "正在刷新"
                ServiceStatus.ParseFailure -> "解析失败"
            },
            serviceSubtitle = dashboard.serviceLabel,
            serviceColor = dashboard.serviceColor,
            serviceHostLabel = serviceHostLabel,
            updatedAtLabel = dashboard.updatedAtLabel,
            syncStatusLabel = dashboard.syncStatusLabel,
        )
    }

    private fun applyDiagnosticUploadStatus(status: DiagnosticUploadStatus) {
        val recentDiagnosticAt = status.pendingPackage?.createdAt ?: status.lastSuccess?.diagnosticCreatedAt
        val recentDiagnosticAtLabel = recentDiagnosticAt?.let(DIAGNOSTIC_TIME_FORMATTER::format)
        val entrySubtitle = when {
            recentDiagnosticAtLabel == null -> "最近诊断：暂无"
            status.pendingPackage != null -> "最近诊断：${recentDiagnosticAtLabel}未上传"
            else -> "最近诊断：${recentDiagnosticAtLabel}已上传"
        }
        _uiState.value = _uiState.value.copy(
            settings = _uiState.value.settings.copy(
                diagnosticUpload = DiagnosticUploadUiState(
                    entrySubtitle = entrySubtitle,
                    entryEnabled = status.phase != DiagnosticUploadPhase.PreparingPackage &&
                        status.phase != DiagnosticUploadPhase.Uploading,
                    actionLabel = when (status.phase) {
                        DiagnosticUploadPhase.PreparingPackage,
                        DiagnosticUploadPhase.Uploading,
                        -> "上传诊断中"
                        DiagnosticUploadPhase.Failed -> "重新尝试上传诊断"
                        DiagnosticUploadPhase.Idle -> if (status.hasPendingPackage) {
                            "重新尝试上传诊断"
                        } else {
                            "上传诊断信息"
                        }
                    },
                    actionEnabled = status.phase != DiagnosticUploadPhase.PreparingPackage &&
                        status.phase != DiagnosticUploadPhase.Uploading,
                    statusLabel = when (status.phase) {
                        DiagnosticUploadPhase.PreparingPackage -> "正在生成诊断包"
                        DiagnosticUploadPhase.Uploading -> "上传诊断中"
                        DiagnosticUploadPhase.Failed -> "上传失败，可重试"
                        DiagnosticUploadPhase.Idle -> when {
                            status.lastSuccess != null -> "上传完成"
                            status.hasPendingPackage -> "上传失败，可重试"
                            else -> "尚未上传"
                        }
                    },
                    hasPendingPackage = status.hasPendingPackage,
                    packageSizeLabel = status.packageSizeBytes?.takeIf { it > 0L }?.let(::formatBinaryBytes),
                    progressLabel = status.totalBytes
                        ?.takeIf { total -> total > 0L && status.phase == DiagnosticUploadPhase.Uploading }
                        ?.let { total ->
                            val percent = ((status.bytesUploaded.toDouble() / total.toDouble()) * 100.0)
                                .roundToInt()
                                .coerceIn(0, 100)
                            "$percent% (${formatBinaryBytes(status.bytesUploaded)} / ${formatBinaryBytes(total)})"
                        },
                    speedLabel = status.uploadSpeedBytesPerSecond
                        ?.takeIf { speed -> speed > 0L && status.phase == DiagnosticUploadPhase.Uploading }
                        ?.let { speed -> "${formatBinaryBytes(speed)}/s" },
                    lastDiagnosticId = status.lastSuccess?.diagnosticId,
                    lastDiagnosticAtLabel = recentDiagnosticAtLabel,
                    lastUploadedAtLabel = status.lastSuccess?.receivedAt?.let(DIAGNOSTIC_TIME_FORMATTER::format),
                    progressOverlay = buildDiagnosticUploadOverlay(status),
                ),
            ),
        )
    }

    private fun buildDiagnosticUploadOverlay(status: DiagnosticUploadStatus): AppUpdateDownloadOverlayUiState {
        return when (status.phase) {
            DiagnosticUploadPhase.PreparingPackage -> AppUpdateDownloadOverlayUiState(
                visible = true,
                statusLabel = "正在生成诊断包",
                fileSizeLabel = status.packageSizeBytes?.takeIf { it > 0L }?.let(::formatBinaryBytes) ?: "待确认",
                progressLabel = "",
                transferredLabel = "0 B / --",
                speedLabel = null,
            )
            DiagnosticUploadPhase.Uploading -> {
                val totalBytes = status.totalBytes?.takeIf { it > 0L }
                val progressPercent = totalBytes?.let { total ->
                    ((status.bytesUploaded.toDouble() / total.toDouble()) * 100.0)
                        .roundToInt()
                        .coerceIn(0, 100)
                }
                val transferredLabel = totalBytes?.let { total ->
                    "${formatBinaryBytes(status.bytesUploaded)} / ${formatBinaryBytes(total)}"
                } ?: "${formatBinaryBytes(status.bytesUploaded)} / --"
                AppUpdateDownloadOverlayUiState(
                    visible = true,
                    statusLabel = "上传诊断中",
                    fileSizeLabel = totalBytes?.let(::formatBinaryBytes) ?: "待确认",
                    progressLabel = progressPercent?.let { percent -> "$percent%" } ?: "",
                    transferredLabel = transferredLabel,
                    speedLabel = status.uploadSpeedBytesPerSecond
                        ?.takeIf { speed -> speed > 0L }
                        ?.let { speed -> "${formatBinaryBytes(speed)}/s" },
                )
            }
            DiagnosticUploadPhase.Failed,
            DiagnosticUploadPhase.Idle,
            -> AppUpdateDownloadOverlayUiState()
        }
    }

    private fun logUserAction(
        name: String,
        fields: Map<String, Any?> = emptyMap(),
    ) {
        viewModelScope.launch {
            diagnosticEventLogger.log(
                event = "user_action",
                fields = mapOf(
                    "action" to (mapOf("name" to name) + fields),
                ),
            )
        }
    }

    private fun executeDiagnosticPromptAction(action: DiagnosticPromptAction?) {
        dismissDiagnosticPrompt()
        when (action) {
            DiagnosticPromptAction.UploadPending,
            DiagnosticPromptAction.ConfirmUpload,
            -> uploadDiagnostics()

            DiagnosticPromptAction.ClearPending -> {
                logUserAction(name = "clear_pending_diagnostic_upload")
                diagnosticUploadManager?.clearPendingPackage()
            }

            null -> Unit
        }
    }

    private fun updateDiagnosticPairedState() {
        diagnosticRuntimeContext?.updateHasPaired(hasPaired)
    }

    private fun reconcileInstalledAppUpdatePreferences() {
        var next = appUpdatePreferences
        var changed = false
        val pendingInstalledNotes = next.pendingInstalledVersionNotes
        if (pendingInstalledNotes != null && pendingInstalledNotes.versionCode <= config.appVersionCode) {
            next = next.copy(
                currentVersionNotes = pendingInstalledNotes,
                pendingInstalledVersionNotes = null,
            )
            changed = true
        }
        val filteredIgnored = next.ignoredVersionCodes.filterTo(linkedSetOf()) { versionCode ->
            versionCode > config.appVersionCode
        }
        if (filteredIgnored != next.ignoredVersionCodes) {
            next = next.copy(ignoredVersionCodes = filteredIgnored)
            changed = true
        }
        if (changed) {
            persistAppUpdatePreferences(next)
        } else {
            appUpdatePreferences = next
        }
    }

    private fun persistAppUpdatePreferences(preferences: AppUpdatePreferences) {
        appUpdatePreferences = preferences
        appUpdatePreferenceStore.write(preferences)
    }

    private fun refreshAppUpdatePreferenceState() {
        updateAppUpdateState {
            it.copy(
                currentVersionLabel = formatVersionLabel(config.appVersion, config.appVersionCode),
                channelLabel = currentAppUpdateChannelLabel(),
                autoCheckEnabled = appUpdatePreferences.autoCheckEnabled,
                currentVersionNotes = currentVersionNotesForDisplay(persistIfMissing = false),
            )
        }
    }

    private fun syncCurrentVersionNotesState(persistIfMissing: Boolean) {
        updateAppUpdateState {
            it.copy(
                currentVersionNotes = currentVersionNotesForDisplay(persistIfMissing = persistIfMissing),
            )
        }
    }

    private fun currentVersionNotesForDisplay(persistIfMissing: Boolean): AppUpdateVersionNotesUiState {
        val cachedNotes = when {
            appUpdatePreferences.currentVersionNotes?.versionCode == config.appVersionCode -> {
                appUpdatePreferences.currentVersionNotes
            }

            appUpdatePreferences.currentVersionNotes == null && persistIfMissing -> {
                val emptyNotes = AppUpdateVersionNotes(
                    versionName = config.appVersion,
                    versionCode = config.appVersionCode,
                    emptyMessage = "当前版本暂无更新说明",
                )
                persistAppUpdatePreferences(appUpdatePreferences.copy(currentVersionNotes = emptyNotes))
                emptyNotes
            }

            else -> null
        }
        return buildVersionNotesUiState(
            versionLabel = currentVersionNotesVersionLabel(),
            notes = cachedNotes,
            emptyLabel = "当前版本暂无更新说明",
        )
    }

    private fun buildLatestVersionNotesUiState(update: WatcherApkUpdate): AppUpdateVersionNotesUiState {
        return buildVersionNotesUiState(
            versionLabel = formatVersionLabel(update.versionName, update.versionCode),
            notes = AppUpdateVersionNotes(
                versionName = update.versionName,
                versionCode = update.versionCode,
                notes = update.releaseNotes.map { note ->
                    AppUpdateNote(
                        publishedAtLabel = note.publishedAtLabel,
                        summary = note.summary,
                    )
                },
            ),
            emptyLabel = "该版本暂未提供更新说明",
        )
    }

    private fun buildVersionNotesUiState(
        versionLabel: String,
        notes: AppUpdateVersionNotes?,
        emptyLabel: String,
    ): AppUpdateVersionNotesUiState {
        return AppUpdateVersionNotesUiState(
            versionLabel = versionLabel,
            notes = notes?.notes.orEmpty().map { note ->
                AppUpdateNoteUiState(
                    publishedAtLabel = note.publishedAtLabel,
                    summary = note.summary,
                )
            },
            emptyLabel = notes?.emptyMessage ?: emptyLabel,
        )
    }

    private fun buildPendingInstalledVersionNotes(update: WatcherApkUpdate): AppUpdateVersionNotes {
        return AppUpdateVersionNotes(
            versionName = update.versionName,
            versionCode = update.versionCode,
            notes = update.releaseNotes.map { note ->
                AppUpdateNote(
                    publishedAtLabel = note.publishedAtLabel,
                    summary = note.summary,
                )
            },
            emptyMessage = "该版本暂未提供更新说明",
        )
    }

    private fun cachePendingInstalledVersionNotes(update: WatcherApkUpdate) {
        persistAppUpdatePreferences(
            appUpdatePreferences.copy(
                pendingInstalledVersionNotes = buildPendingInstalledVersionNotes(update),
            ),
        )
        refreshAppUpdatePreferenceState()
    }

    private fun shouldAutoCheckAppUpdateOnColdStart(): Boolean {
        val debugScenario = debugStore?.current() ?: DebugDemoScenario.NONE
        return hasPaired &&
            appUpdatePreferences.autoCheckEnabled &&
            debugScenario == DebugDemoScenario.NONE
    }

    private suspend fun fetchAutoAppUpdate(): WatcherApkUpdate? {
        return when (val result = performTimedAppUpdateCheck()) {
            is WatcherApkUpdateCheckResult.Failure -> null
            is WatcherApkUpdateCheckResult.Success -> {
                val update = result.update
                if (update.versionCode <= config.appVersionCode) {
                    null
                } else if (update.versionCode in appUpdatePreferences.ignoredVersionCodes) {
                    null
                } else {
                    pendingAppUpdate = update
                    update
                }
            }
        }
    }

    private suspend fun performTimedAppUpdateCheck(): WatcherApkUpdateCheckResult {
        val startedAtMs = monotonicNowMs()
        val result = withTimeoutOrNull(appUpdateCheckTimeoutMs) {
            appUpdateManager.fetchLatestUpdate()
        } ?: WatcherApkUpdateCheckResult.Failure("访问超时")
        val elapsedMs = (monotonicNowMs() - startedAtMs).coerceAtLeast(0L)
        val remainingMs = appUpdateCheckMinDurationMs - elapsedMs
        if (remainingMs > 0L) {
            delay(remainingMs)
        }
        return result
    }

    private suspend fun prepareAutoAppUpdateDestination(update: WatcherApkUpdate) {
        val hasCachedUpdate = appUpdateManager.hasCachedUpdate(update)
        val latestLabel = formatVersionLabel(update.versionName, update.versionCode)
        val comparisonLabel = "当前 ${formatVersionShort(config.appVersion)} -> 最新 ${update.versionName}"
        if (hasCachedUpdate) {
            showCachedUpdateNotesAndResumeInstall(update, latestLabel, comparisonLabel)
        } else {
            updateAppUpdateState {
                it.copy(
                    status = AppUpdateStatus.Available,
                    latestVersionLabel = latestLabel,
                    comparisonLabel = comparisonLabel,
                    progressPercent = null,
                    progressDetailLabel = null,
                    downloadSpeedLabel = null,
                    hasPendingUpdate = true,
                    detailLabel = "发现新版本",
                    latestVersionNotes = buildLatestVersionNotesUiState(update),
                    downloadOverlay = AppUpdateDownloadOverlayUiState(),
                )
            }
            _uiState.value = _uiState.value.copy(
                dashboard = _uiState.value.dashboard.copy(pagerPage = DashboardPage.Settings),
                settings = _uiState.value.settings.copy(destination = SettingsDestination.UpdateNotes),
            )
        }
    }

    private suspend fun showCachedUpdateNotesAndResumeInstall(
        update: WatcherApkUpdate,
        latestLabel: String,
        comparisonLabel: String,
    ) {
        updateAppUpdateState {
            it.copy(
                status = AppUpdateStatus.ReadyToInstall,
                latestVersionLabel = latestLabel,
                comparisonLabel = comparisonLabel,
                progressPercent = 100,
                progressDetailLabel = null,
                downloadSpeedLabel = null,
                hasPendingUpdate = true,
                detailLabel = "",
                latestVersionNotes = buildLatestVersionNotesUiState(update),
                downloadOverlay = AppUpdateDownloadOverlayUiState(),
            )
        }
        _uiState.value = _uiState.value.copy(
            dashboard = _uiState.value.dashboard.copy(pagerPage = DashboardPage.Settings),
            settings = _uiState.value.settings.copy(destination = SettingsDestination.UpdateNotes),
        )
        cachePendingInstalledVersionNotes(update)
        applyAppUpdateInstallResult(
            appUpdateManager.startInstallFromCache(update),
            successDetail = "已打开系统安装器",
        )
    }

    private fun updateAppUpdateState(transform: (AppUpdateUiState) -> AppUpdateUiState) {
        _uiState.value = _uiState.value.copy(
            settings = _uiState.value.settings.copy(
                update = transform(_uiState.value.settings.update),
            ),
        )
    }

    private fun syncInstallPermissionState(detailLabel: String? = null) {
        val enabled = appUpdateManager.canRequestPackageInstalls()
        updateAppUpdateState {
            val nextStatus = if (enabled && it.status == AppUpdateStatus.PermissionRequired && it.hasPendingUpdate) {
                AppUpdateStatus.ReadyToInstall
            } else {
                it.status
            }
            val nextDetail = when {
                detailLabel != null -> detailLabel
                enabled && it.status == AppUpdateStatus.PermissionRequired && it.hasPendingUpdate -> "已允许安装未知来源，可继续安装"
                else -> it.detailLabel
            }
            it.copy(
                status = nextStatus,
                installPermissionEnabled = enabled,
                installPermissionLabel = formatInstallPermissionLabel(enabled),
                detailLabel = nextDetail,
            )
        }
    }

    private fun resetSettingsDestination() {
        if (_uiState.value.settings.destination == SettingsDestination.Root) {
            return
        }
        _uiState.value = _uiState.value.copy(
            settings = _uiState.value.settings.copy(destination = SettingsDestination.Root),
        )
    }

    private fun applyAppUpdateInstallResult(
        result: WatcherApkInstallResult,
        successDetail: String,
    ) {
        when (result) {
            WatcherApkInstallResult.Started -> {
                updateAppUpdateState {
                    it.copy(
                        status = AppUpdateStatus.ReadyToInstall,
                        progressPercent = 100,
                        progressDetailLabel = null,
                        downloadSpeedLabel = null,
                        hasPendingUpdate = pendingAppUpdate != null,
                        detailLabel = successDetail,
                        downloadOverlay = AppUpdateDownloadOverlayUiState(),
                    )
                }
            }

            is WatcherApkInstallResult.PermissionRequired -> {
                updateAppUpdateState {
                    it.copy(
                        status = AppUpdateStatus.PermissionRequired,
                        progressPercent = it.progressPercent ?: 100,
                        progressDetailLabel = null,
                        downloadSpeedLabel = null,
                        hasPendingUpdate = pendingAppUpdate != null,
                        detailLabel = result.message,
                        downloadOverlay = AppUpdateDownloadOverlayUiState(),
                    )
                }
            }

            is WatcherApkInstallResult.Failure -> {
                updateAppUpdateState {
                    val fallbackStatus = when (it.status) {
                        AppUpdateStatus.ReadyToInstall,
                        AppUpdateStatus.PermissionRequired,
                        -> AppUpdateStatus.ReadyToInstall

                        else -> AppUpdateStatus.Available
                    }
                    it.copy(
                        status = fallbackStatus,
                        progressPercent = if (fallbackStatus == AppUpdateStatus.ReadyToInstall) 100 else null,
                        progressDetailLabel = null,
                        downloadSpeedLabel = null,
                        hasPendingUpdate = pendingAppUpdate != null,
                        detailLabel = if (fallbackStatus == AppUpdateStatus.ReadyToInstall) {
                            "已下载更新，等待安装"
                        } else {
                            "发现新版本"
                        },
                        downloadOverlay = AppUpdateDownloadOverlayUiState(),
                    )
                }
                showTransientNotice(result.message)
            }
        }
    }

    private fun buildDebugPreviewUpdateState(
        updatePreview: String?,
        installPermissionEnabled: Boolean,
    ): AppUpdateUiState {
        val nextVersionName = incrementPatchVersionName(config.appVersion)
        val nextVersionCode = config.appVersionCode + 1
        val latestVersionLabel = formatVersionLabel(nextVersionName, nextVersionCode)
        val baseState = _uiState.value.settings.update.copy(
            status = AppUpdateStatus.Idle,
            isExpanded = false,
            latestVersionLabel = null,
            comparisonLabel = null,
            progressPercent = null,
            progressDetailLabel = null,
            downloadSpeedLabel = null,
            hasPendingUpdate = false,
            detailLabel = "未检查更新",
            installPermissionEnabled = installPermissionEnabled,
            installPermissionLabel = formatInstallPermissionLabel(installPermissionEnabled),
            autoCheckEnabled = appUpdatePreferences.autoCheckEnabled,
            latestVersionNotes = AppUpdateVersionNotesUiState(),
            downloadOverlay = AppUpdateDownloadOverlayUiState(),
        )
        return when (updatePreview?.lowercase(Locale.US)) {
            "available" -> baseState.copy(
                status = AppUpdateStatus.Available,
                latestVersionLabel = latestVersionLabel,
                comparisonLabel = "当前 ${formatVersionShort(config.appVersion)} -> 最新 $nextVersionName",
                hasPendingUpdate = true,
                detailLabel = "发现新版本",
                latestVersionNotes = AppUpdateVersionNotesUiState(
                    versionLabel = latestVersionLabel,
                    notes = listOf(
                        AppUpdateNoteUiState(
                            publishedAtLabel = BEIJING_TIME_FORMATTER.format(wallClockNow()),
                            summary = "调试预览更新说明",
                        ),
                    ),
                ),
            )

            else -> baseState
        }
    }

    private fun buildDebugPreviewUpdate(latestVersionLabel: String?): WatcherApkUpdate {
        val versionName = latestVersionLabel?.substringBefore(" (")?.ifBlank { incrementPatchVersionName(config.appVersion) }
            ?: incrementPatchVersionName(config.appVersion)
        return WatcherApkUpdate(
            channel = activeEnvironment(),
            versionName = versionName,
            versionCode = config.appVersionCode + 1,
            artifact = "openwatcher-watchapp-v$versionName-debug-preview.apk",
            sha256 = "",
            commit = "debug-preview",
            builtAt = wallClockNow().toString(),
            downloadUrl = "${currentBaseUrl.trimEnd('/')}/file/dev/apk",
        )
    }

    private fun currentAppUpdateChannelLabel(): String {
        return when (activeEnvironment()) {
            AppUpdateChannel.Beta -> "beta"
            AppUpdateChannel.Dev -> "dev"
        }
    }

    private fun currentVersionNotesVersionLabel(): String {
        return "${formatVersionLabel(config.appVersion, config.appVersionCode)} · ${currentAppUpdateChannelLabel()}"
    }

    private fun buildOfflineState(message: String): OfflineUiState {
        return OfflineUiState(
            title = "服务不可达",
            message = message,
            detailLabel = "请检查手机与服务地址的网络连接",
            serviceHostLabel = serviceHostLabel,
            tokenFingerprint = tokenRepository.tokenFingerprint(currentToken),
            qrPayload = if (currentToken.isBlank()) {
                ""
            } else {
                tokenRepository.buildPairingPayload(
                    baseUrl = currentBaseUrl,
                    token = currentToken,
                    deviceName = config.deviceName,
                )
            },
        )
    }

    private fun bootstrapDashboardState(): DashboardUiState {
        val quotaStatus = latestSnapshot?.quota?.status ?: QuotaStatus.Unavailable
        val quotaDimmed = hasPaired && (isServiceDegraded || quotaStatus != QuotaStatus.Ok)
        val base = latestSnapshot?.let {
            buildDashboardState(it, _uiState.value.dashboard.pagerPage)
        } ?: DashboardUiState(serviceHostLabel = serviceHostLabel)
        return base.copy(
            serviceStatus = if (hasPaired) ServiceStatus.Offline else ServiceStatus.WaitingPairing,
            serviceLabel = if (hasPaired) "离线" else "等待配对",
            serviceColor = if (hasPaired) Color(0xFF8E98A7) else Color(0xFFFFC857),
            updatedAtLabel = latestSnapshot?.let { buildUpdatedLabel(it.observedAt, fresh = false) } ?: "尚未刷新",
            serviceHostLabel = serviceHostLabel,
            syncStatusLabel = if (hasPaired) "等待连接" else "等待数据",
            isServiceDegraded = hasPaired,
            home = base.home.copy(
                fiveHour = base.home.fiveHour.copy(isDimmed = quotaDimmed),
                weekly = base.home.weekly.copy(isDimmed = quotaDimmed),
                isServiceDegraded = hasPaired,
            ),
            heatmap24h = base.heatmap24h.copy(isServiceDegraded = hasPaired),
            sessionDetails = base.sessionDetails.copy(isServiceDegraded = hasPaired),
            errors = emptyList(),
        )
    }

    private fun applyPairedServiceFailure(
        serviceTitle: String,
        serviceLabel: String,
        serviceColor: Color,
        syncStatusLabel: String,
        detailMessage: String,
        status: ServiceStatus,
    ) {
        isServiceDegraded = true
        val dashboardState = (latestSnapshot?.let {
            buildDashboardState(it, _uiState.value.dashboard.pagerPage)
        } ?: bootstrapDashboardState()).copy(
            serviceStatus = status,
            serviceLabel = serviceLabel,
            serviceColor = serviceColor,
            syncStatusLabel = syncStatusLabel,
            errors = emptyList(),
        )
        val nextScreen = when (_uiState.value.screen) {
            AppScreen.Splash,
            AppScreen.Pairing,
            AppScreen.Offline,
            -> AppScreen.Dashboard
            else -> _uiState.value.screen
        }
        _uiState.value = _uiState.value.copy(
            screen = nextScreen,
            dashboard = dashboardState,
            settings = buildSettingsState(dashboardState).copy(
                serviceTitle = serviceTitle,
                serviceSubtitle = detailMessage,
                serviceColor = serviceColor,
                syncStatusLabel = syncStatusLabel,
            ),
            pairing = buildPairingState(
                status = "已配对",
                hint = "服务恢复后会自动刷新",
                service = serviceLabel,
                color = serviceColor,
                scanStepCompleted = true,
                confirmStepActive = false,
                authStepCompleted = true,
            ),
        )
    }

    private fun resetSelectionAndTransientState() {
        selectedHeatmapHourStart = null
        heatmapCursorPosition = null
        selectedTrendDayIndex = null
        heatmapTrendCursorPosition = null
        heatmapTrendPendingDegrees = 0f
        heatmapTrendTipVisible = false
        heatmapTrendTipHideJob?.cancel()
        heatmapTrendTipHideJob = null
        heatmapRotaryMode = HeatmapRotaryMode.HourRing
        selectedSessionThreadId = null
        selectedSessionSlotIndex = null
        sessionSelectionCursorPosition = null
        detailWindowState = DetailWindowState()
        screenshotFeedbackHideJob?.cancel()
        screenshotFeedbackHideJob = null
        _uiState.value = _uiState.value.copy(screenshotUpload = ScreenshotUploadUiState())
        updateAppUpdateState {
            it.copy(downloadOverlay = AppUpdateDownloadOverlayUiState())
        }
    }

    private fun switchEnvironmentRuntime() {
        pollJob?.cancel()
        pollJob = null
        stopStatusStream()
        stopLocalRefreshTicker()
        stopSessionStream(clearMessage = true)
        stopDetailWindowStream(clearState = true)
        homeQuotaTipHideJob?.cancel()
        homeQuotaTipHideJob = null
        pendingBootstrapRequest = null
        pendingBootstrapContinuation = false
        dailyTrendStore.clear()
        statusSnapshotStore.clear()
        sessionDetailsWindowStore.clear()
        latestSnapshot = null
        cachedDailyTrend30d = null
        refreshAppUpdatePreferenceState()
        syncCurrentVersionNotesState(persistIfMissing = true)
        initializeWithoutBootstrap()
        resumeCurrentEnvironmentAfterConfigChange()
    }

    private fun resumeCurrentEnvironmentAfterConfigChange() {
        if (hasPaired && autoPolling && isForegroundVisible) {
            startStatusStream()
            startLocalRefreshTicker()
        } else if (!hasPaired && autoPolling) {
            startPairingLoop()
        }
        if (hasPaired) {
            viewModelScope.launch {
                fetchStatus(showRefreshing = false, fromPairingLoop = false)
            }
        }
        updateSessionStreamForCurrentScreen(clearMessage = true)
    }

    private fun updateHomeQuotaEasterEggState(state: HomeQuotaEasterEggUiState) {
        _uiState.value = _uiState.value.copy(
            dashboard = _uiState.value.dashboard.copy(
                home = _uiState.value.dashboard.home.copy(
                    quotaEasterEgg = state,
                ),
            ),
        )
    }

    private fun refreshDerivedState() {
        val snapshot = latestSnapshot ?: return
        val dashboardState = buildDashboardState(snapshot, _uiState.value.dashboard.pagerPage)
        _uiState.value = _uiState.value.copy(
            dashboard = dashboardState,
            settings = buildSettingsState(dashboardState),
        )
        updateSessionRuntimeDurationTicker()
    }

    private fun updateSessionStreamForCurrentScreen(clearMessage: Boolean) {
        if (isBootstrapping || isServiceDegraded || !isForegroundVisible || currentToken.isBlank()) {
            stopSessionStream(clearMessage = clearMessage)
            stopDetailWindowStream(clearState = false)
            return
        }
        when {
            _uiState.value.screen == AppScreen.SessionDetails -> {
                stopSessionStream(clearMessage = false)
                startDetailWindowStream()
            }
            _uiState.value.screen == AppScreen.Dashboard &&
                _uiState.value.dashboard.pagerPage == DashboardPage.Home -> {
                stopDetailWindowStream(clearState = false)
                startSessionStream(clearMessage = clearMessage, includeMessages = false)
            }
            else -> {
                stopSessionStream(clearMessage = clearMessage)
                stopDetailWindowStream(clearState = false)
            }
        }
    }

    private fun startDetailWindowStream() {
        if (currentToken.isBlank()) {
            return
        }
        if (detailWindowStreamJob?.isActive == true) {
            return
        }
        markDetailWindowConnecting()
        detailWindowStreamJob = viewModelScope.launch {
            runDetailWindowStreamLoop()
        }
    }

    private suspend fun runDetailWindowStreamLoop() {
        var reconnectAttempt = 0
        var startedAtLeastOnce = false
        while (shouldKeepDetailWindowStream()) {
            logDetailWindowStreamState(
                action = if (startedAtLeastOnce) "reconnect_requested" else "connect_requested",
                fields = mapOf(
                    "reconnectAttempt" to reconnectAttempt,
                    "preferredOrderCount" to detailWindowState.orderedThreadIds.size,
                    "selectedThreadId" to selectedSessionThreadId,
                ),
            )
            if (startedAtLeastOnce) {
                markDetailWindowConnecting()
            }
            startedAtLeastOnce = true

            var firstEventType: String? = null
            var terminalFailure: SessionWindowStreamEvent.Failure? = null
            api.streamSessionWindow(
                token = currentToken,
                limit = DETAIL_WINDOW_LIMIT,
                preferredOrder = detailWindowState.orderedThreadIds,
            )
                .flowOn(ioDispatcher)
                .catch { error ->
                    if (error is CancellationException) {
                        throw error
                    }
                    emit(unexpectedDetailWindowFailure(error))
                }
                .collect { event ->
                    if (event is SessionWindowStreamEvent.Failure && event.terminal) {
                        terminalFailure = event
                    } else if (firstEventType == null) {
                        firstEventType = when (event) {
                            is SessionWindowStreamEvent.Window -> "sessions_window"
                            is SessionWindowStreamEvent.RuntimeState -> "session_runtime_state"
                            is SessionWindowStreamEvent.AgentMessage -> "session_agent_message"
                            SessionWindowStreamEvent.Heartbeat -> "heartbeat"
                            is SessionWindowStreamEvent.Failure -> null
                        }
                        if (firstEventType != null) {
                            logDetailWindowStreamState(
                                action = if (reconnectAttempt > 0) "reconnect_success" else "connected",
                                fields = mapOf(
                                    "reconnectAttempt" to reconnectAttempt,
                                    "firstEventType" to firstEventType,
                                ),
                            )
                            reconnectAttempt = 0
                        }
                    }
                    handleDetailWindowStreamEvent(event)
                }

            if (!shouldKeepDetailWindowStream()) {
                if (terminalFailure != null) {
                    logDetailWindowDisconnect(
                        failure = terminalFailure,
                        reconnectAttempt = reconnectAttempt,
                        retryable = terminalFailure.retryable,
                        nextRetryDelayMs = null,
                    )
                }
                return
            }

            val failure = terminalFailure ?: SessionWindowStreamEvent.Failure(
                message = "会话窗口流连接失败",
                reason = SessionStreamFailureReason.StreamClosed,
                retryable = true,
                terminal = true,
                detail = "collector_completed",
            )
            if (!failure.retryable) {
                logDetailWindowDisconnect(
                    failure = failure,
                    reconnectAttempt = reconnectAttempt,
                    retryable = false,
                    nextRetryDelayMs = null,
                )
                return
            }
            reconnectAttempt += 1
            val retryDelayMs = reconnectDelayForAttempt(reconnectAttempt)
            logDetailWindowDisconnect(
                failure = failure,
                reconnectAttempt = reconnectAttempt,
                retryable = true,
                nextRetryDelayMs = retryDelayMs,
            )
            delay(retryDelayMs)
        }
    }

    private fun startSessionStream(clearMessage: Boolean, includeMessages: Boolean) {
        val threadId = selectedSessionThreadId ?: return
        if (currentToken.isBlank()) {
            return
        }
        if (
            sessionStreamJob?.isActive == true &&
            sessionStreamThreadId == threadId &&
            sessionStreamIncludeMessages == includeMessages
        ) {
            return
        }
        sessionStreamJob?.cancel()
        sessionStreamThreadId = threadId
        sessionStreamIncludeMessages = includeMessages
        if (clearMessage) {
            prepareSessionStreamState(threadId, AgentMessageStreamStatus.Connecting)
            refreshSessionDetailsAgentState()
        } else {
            markSessionStreamConnecting(threadId)
        }
        sessionStreamJob = viewModelScope.launch {
            runSessionStreamLoop(threadId, includeMessages)
        }
    }

    private suspend fun runSessionStreamLoop(threadId: String, includeMessages: Boolean) {
        var reconnectAttempt = 0
        var startedAtLeastOnce = false
        while (shouldKeepSessionStream(threadId, includeMessages)) {
            if (startedAtLeastOnce) {
                markSessionStreamConnecting(threadId)
            }
            startedAtLeastOnce = true

            val connectedAtMs = monotonicNowMs()
            var receivedAgentMessage = false
            var firstEventType: String? = null
            var terminalFailure: SessionStreamEvent.Failure? = null

            api.streamSessionAgentMessages(currentToken, threadId, includeMessages)
                .flowOn(ioDispatcher)
                .catch { error ->
                    if (error is CancellationException) {
                        throw error
                    }
                    emit(unexpectedStreamFailure(error))
                }
                .collect { event ->
                    when (event) {
                        is SessionStreamEvent.AgentMessage -> {
                            if (firstEventType == null) {
                                firstEventType = "agent_message"
                                if (reconnectAttempt > 0) {
                                    reportSessionStreamClientEvent(
                                        SessionStreamClientEventReport(
                                            eventType = SessionStreamClientEventType.ReconnectSuccess,
                                            threadId = threadId,
                                            deviceName = config.deviceName,
                                            appVersion = config.appVersion,
                                            reconnectAttempt = reconnectAttempt,
                                            firstEventType = firstEventType,
                                        ),
                                    )
                                }
                            }
                            reconnectAttempt = 0
                            receivedAgentMessage = true
                        }
                        is SessionStreamEvent.RuntimeState -> {
                            if (firstEventType == null) {
                                firstEventType = "runtime_state"
                                if (reconnectAttempt > 0) {
                                    reportSessionStreamClientEvent(
                                        SessionStreamClientEventReport(
                                            eventType = SessionStreamClientEventType.ReconnectSuccess,
                                            threadId = threadId,
                                            deviceName = config.deviceName,
                                            appVersion = config.appVersion,
                                            reconnectAttempt = reconnectAttempt,
                                            firstEventType = firstEventType,
                                        ),
                                    )
                                }
                            }
                            reconnectAttempt = 0
                        }
                        SessionStreamEvent.Heartbeat -> {
                            if (firstEventType == null) {
                                firstEventType = "heartbeat"
                                if (reconnectAttempt > 0) {
                                    reportSessionStreamClientEvent(
                                        SessionStreamClientEventReport(
                                            eventType = SessionStreamClientEventType.ReconnectSuccess,
                                            threadId = threadId,
                                            deviceName = config.deviceName,
                                            appVersion = config.appVersion,
                                            reconnectAttempt = reconnectAttempt,
                                            firstEventType = firstEventType,
                                        ),
                                    )
                                }
                            }
                            reconnectAttempt = 0
                        }
                        is SessionStreamEvent.Failure -> {
                            if (event.terminal) {
                                terminalFailure = event
                            }
                        }
                    }
                    handleSessionStreamEvent(threadId, event)
                }

            if (!shouldKeepSessionStream(threadId, includeMessages)) {
                return
            }

            val failure = terminalFailure ?: SessionStreamEvent.Failure(
                message = "会话流连接失败",
                reason = SessionStreamFailureReason.StreamClosed,
                retryable = true,
                terminal = true,
                detail = "collector_completed",
            )
            if (!failure.retryable) {
                reportSessionStreamClientEvent(
                    SessionStreamClientEventReport(
                        eventType = SessionStreamClientEventType.Disconnect,
                        threadId = threadId,
                        deviceName = config.deviceName,
                        appVersion = config.appVersion,
                        reconnectAttempt = reconnectAttempt,
                        reason = failure.reason,
                        detail = failure.detail,
                        statusCode = failure.statusCode,
                        retryable = false,
                        connectedMs = elapsedSince(connectedAtMs),
                        receivedAgentMessage = receivedAgentMessage,
                    ),
                )
                return
            }

            reconnectAttempt += 1
            val retryDelayMs = reconnectDelayForAttempt(reconnectAttempt)
            reportSessionStreamClientEvent(
                SessionStreamClientEventReport(
                    eventType = SessionStreamClientEventType.Disconnect,
                    threadId = threadId,
                    deviceName = config.deviceName,
                    appVersion = config.appVersion,
                    reconnectAttempt = reconnectAttempt,
                    reason = failure.reason,
                    detail = failure.detail,
                    statusCode = failure.statusCode,
                    retryable = true,
                    connectedMs = elapsedSince(connectedAtMs),
                    nextRetryDelayMs = retryDelayMs,
                    receivedAgentMessage = receivedAgentMessage,
                ),
            )
            delay(retryDelayMs)
        }
    }

    private fun stopSessionStream(clearMessage: Boolean) {
        sessionStreamJob?.cancel()
        sessionStreamJob = null
        sessionStreamThreadId = null
        sessionStreamIncludeMessages = false
        sessionAgentState = if (clearMessage) {
            SessionAgentState()
        } else {
            sessionAgentState.copy(status = AgentMessageStreamStatus.Disconnected)
        }
        refreshDerivedState()
        updateSessionRuntimeDurationTicker()
    }

    private fun stopDetailWindowStream(clearState: Boolean) {
        detailWindowStreamJob?.cancel()
        detailWindowStreamJob = null
        viewModelScope.launch {
            logDetailWindowStreamState(
                action = "stopped",
                fields = mapOf(
                    "clearState" to clearState,
                    "selectedThreadId" to selectedSessionThreadId,
                    "windowSize" to detailWindowState.orderedThreadIds.size,
                ),
            )
        }
        detailWindowState = if (clearState) {
            DetailWindowState()
        } else {
            detailWindowState.copy(
                status = AgentMessageStreamStatus.Disconnected,
                error = null,
            )
        }
        refreshDerivedState()
    }

    private fun maybeVibrateForCompletedRuntime(
        previous: SessionRuntimeState?,
        next: SessionRuntimeState,
    ) {
        val turnId = next.turnId ?: return
        val sameTurnCompleted = previous?.turnId == turnId &&
            previous.running &&
            next.lifecycle == SessionRuntimeLifecycle.Completed &&
            !next.running
        if (
            sameTurnCompleted &&
            !sessionAgentState.suppressNextRuntimeHaptic &&
            isForegroundVisible &&
            selectedSessionThreadId == next.threadId &&
            turnId !in sessionAgentState.completedHapticTurnIds
        ) {
            hapticController.vibrateSessionCompleted()
        }
    }

    private fun rememberCompletedHapticTurn(
        previous: SessionRuntimeState?,
        next: SessionRuntimeState,
        completedTurnIds: Set<String>,
    ): Set<String> {
        val turnId = next.turnId ?: return completedTurnIds
        val sameTurnCompleted = previous?.turnId == turnId &&
            previous.running &&
            next.lifecycle == SessionRuntimeLifecycle.Completed &&
            !next.running
        return if (
            sameTurnCompleted &&
            !sessionAgentState.suppressNextRuntimeHaptic &&
            isForegroundVisible &&
            selectedSessionThreadId == next.threadId
        ) {
            completedTurnIds + turnId
        } else {
            completedTurnIds
        }
    }

    private fun runtimeStateForThread(threadId: String): SessionRuntimeState? {
        return sessionAgentState.runtimeState?.takeIf { it.threadId == threadId }
    }

    private fun sessionTurnDurationForThread(threadId: String): SessionTurnDuration? {
        return sessionTurnDurationByThreadId[threadId]
    }

    private fun shouldKeepDetailWindowStream(): Boolean {
        return isForegroundVisible &&
            _uiState.value.screen == AppScreen.SessionDetails &&
            currentToken.isNotBlank() &&
            !isBootstrapping &&
            !isServiceDegraded
    }

    private fun formatRuntimeStatusLine(
        state: SessionRuntimeState?,
        contextCompaction: ContextCompactionSnapshot? = null,
        lastActiveLabel: String,
        runtimeTime: Instant?,
        turnDuration: SessionTurnDuration?,
    ): String {
        contextCompaction?.let {
            return formatContextCompactionStatusLine(it)
        }
        if (state?.running == true) {
            val timeLabel = runtimeTime?.let(TIME_FORMATTER::format) ?: "last: $lastActiveLabel"
            val durationLabel = turnDuration?.durationLabel()
            return listOfNotNull(
                "运行中",
                timeLabel,
                formatRuntimePhaseLabel(state.phase),
                durationLabel,
            ).joinToString(" · ")
        }
        return formatRecentRuntimeStatusLine(
            recentLabel = lastActiveLabel,
            durationLabel = turnDuration?.durationLabel(),
        )
    }

    private fun runtimeTimeForSelectedSession(state: SessionRuntimeState): Instant? {
        val latestMessageAt = sessionAgentState.latestMessage
            ?.takeIf { it.threadId == state.threadId }
            ?.createdAt
        return latestMessageAt ?: state.updatedAt
    }

    private fun runtimeTimeForAgentStatus(state: SessionRuntimeState): Instant? {
        if (state.running) {
            return sessionAgentState.latestMessage
                ?.takeIf { it.threadId == state.threadId }
                ?.createdAt
                ?: state.updatedAt
        }
        return state.updatedAt ?: sessionAgentState.latestMessage
            ?.takeIf { it.threadId == state.threadId }
            ?.createdAt
    }

    private fun formatAgentStatusLine(
        state: SessionRuntimeState?,
        contextCompaction: ContextCompactionSnapshot? = null,
        runtimeTime: Instant?,
        activeMinutes: Int,
        turnDuration: SessionTurnDuration?,
    ): String {
        contextCompaction?.let {
            return formatContextCompactionStatusLine(it)
        }
        if (state?.running == true) {
            return listOfNotNull(
                runtimeTime?.let(TIME_FORMATTER::format),
                formatRuntimePhaseLabel(state.phase),
                turnDuration?.durationLabel(),
            ).joinToString(" · ")
        }
        return formatRecentRuntimeStatusLine(
            recentLabel = formatAgoShort(activeMinutes),
            durationLabel = turnDuration?.durationLabel(),
        )
    }

    private fun runtimeTimeForDetailWindowThread(entry: DetailWindowEntryState): Instant? {
        return entry.latestMessage?.createdAt ?: entry.runtimeState.updatedAt
    }

    private fun formatRuntimePhaseLabel(phase: SessionRuntimePhase): String {
        return when (phase) {
            SessionRuntimePhase.Reasoning,
            SessionRuntimePhase.Unknown,
            -> "思考中"
            SessionRuntimePhase.ToolRunning -> "工具调用中"
            SessionRuntimePhase.AgentCommentary -> "正在输出·说明"
            SessionRuntimePhase.AgentFinal -> "正在输出·最终答复"
        }
    }

    private fun formatRecentStatusLabel(activeMinutes: Int): String {
        return formatRecentRuntimeStatusLine(
            recentLabel = formatAgoShort(activeMinutes),
            durationLabel = null,
        )
    }

    private fun formatHomeActivityLabel(
        isRunning: Boolean,
        isCompacting: Boolean,
        recentLabel: String,
        durationLabel: String?,
    ): String {
        return when {
            isCompacting -> durationLabel?.let { "压缩中 $it" } ?: "压缩中"
            isRunning -> durationLabel ?: "运行中"
            else -> "最近：$recentLabel"
        }
    }

    private fun formatRecentRuntimeStatusLine(
        recentLabel: String,
        durationLabel: String?,
    ): String {
        return if (durationLabel.isNullOrBlank()) {
            "最近：${recentLabel}前"
        } else {
            "最近：${recentLabel}前 用时：$durationLabel"
        }
    }

    private fun rememberSessionTurnDuration(state: SessionRuntimeState) {
        val turnId = state.turnId ?: return
        val previous = sessionTurnDurationByThreadId[state.threadId]
        val next = when {
            state.running -> {
                val resolvedStartedAt = state.startedAt ?: previous?.takeIf { it.turnId == turnId }?.startedAt
                if (previous?.turnId == turnId && resolvedStartedAt != null) {
                    previous.copy(
                        startedAt = resolvedStartedAt,
                        completedDurationMs = null,
                        lifecycle = state.lifecycle,
                        updatedAt = state.updatedAt,
                    )
                } else {
                    SessionTurnDuration(
                        turnId = turnId,
                        startedAt = resolvedStartedAt,
                        lifecycle = state.lifecycle,
                        updatedAt = state.updatedAt,
                    )
                }
            }
            state.lifecycle == SessionRuntimeLifecycle.Completed || state.lifecycle == SessionRuntimeLifecycle.Aborted -> {
                val startedAt = state.startedAt ?: previous
                    ?.takeIf { it.turnId == turnId }
                    ?.startedAt
                val completedDurationMs = computeRuntimeDurationMs(
                    startedAt = startedAt,
                    endedAt = state.updatedAt,
                )
                SessionTurnDuration(
                    turnId = turnId,
                    startedAt = startedAt,
                    completedDurationMs = completedDurationMs,
                    lifecycle = state.lifecycle,
                    updatedAt = state.updatedAt,
                )
            }
            else -> previous ?: SessionTurnDuration(
                turnId = turnId,
                startedAt = state.startedAt,
                lifecycle = state.lifecycle,
                updatedAt = state.updatedAt,
            )
        }
        sessionTurnDurationByThreadId = sessionTurnDurationByThreadId + (state.threadId to next)
    }

    private fun SessionTurnDuration.durationLabel(): String? {
        val durationMs = completedDurationMs ?: computeRuntimeDurationMs(
            startedAt = startedAt,
            endedAt = wallClockNow(),
        )
        val durationSeconds = ((durationMs ?: return null) / 1_000L).coerceAtLeast(1L)
            .coerceAtMost(MAX_SESSION_RUNTIME_DURATION_SECONDS)
        return formatSessionRuntimeDuration(durationSeconds)
    }

    private fun ContextCompactionSnapshot.durationLabel(): String? {
        val durationMs = computeRuntimeDurationMs(startedAt, wallClockNow()) ?: return null
        val durationSeconds = (durationMs / 1_000L).coerceAtLeast(1L)
            .coerceAtMost(MAX_SESSION_RUNTIME_DURATION_SECONDS)
        return formatSessionRuntimeDuration(durationSeconds)
    }

    private fun formatContextCompactionStatusLine(compaction: ContextCompactionSnapshot): String {
        return listOfNotNull(
            contextCompactionTitle(compaction),
            compaction.startedAt?.let(TIME_FORMATTER::format),
            compaction.durationLabel(),
        ).joinToString(" · ")
    }

    private fun contextCompactionTitle(compaction: ContextCompactionSnapshot): String {
        return when (compaction.trigger.lowercase(Locale.US)) {
            "auto" -> "自动压缩中"
            "manual" -> "手动压缩中"
            else -> "压缩中"
        }
    }

    private fun computeRuntimeDurationMs(startedAt: Instant?, endedAt: Instant?): Long? {
        if (startedAt == null || endedAt == null) {
            return null
        }
        return (endedAt.toEpochMilli() - startedAt.toEpochMilli()).coerceAtLeast(0L)
    }

    internal fun formatSessionRuntimeDuration(durationSeconds: Long): String {
        val safeSeconds = durationSeconds.coerceAtLeast(1L).coerceAtMost(MAX_SESSION_RUNTIME_DURATION_SECONDS)
        val hours = safeSeconds / 3_600L
        val minutes = (safeSeconds % 3_600L) / 60L
        val seconds = safeSeconds % 60L
        return when {
            hours > 0L -> "%dh%02dm%02ds".format(Locale.US, hours, minutes, seconds)
            minutes > 0L -> "%dm%02ds".format(Locale.US, minutes, seconds)
            else -> "${seconds}s"
        }
    }

    private fun prepareSessionStreamState(
        threadId: String,
        status: AgentMessageStreamStatus,
    ) {
        sessionAgentState = SessionAgentState(
            threadId = threadId,
            status = status,
            suppressNextRuntimeHaptic = true,
        )
    }

    private fun markSessionStreamConnecting(threadId: String) {
        sessionAgentState = if (sessionAgentState.threadId == threadId) {
            sessionAgentState.copy(
                status = AgentMessageStreamStatus.Connecting,
                error = null,
                suppressNextRuntimeHaptic = true,
            )
        } else {
            SessionAgentState(
                threadId = threadId,
                status = AgentMessageStreamStatus.Connecting,
                suppressNextRuntimeHaptic = true,
            )
        }
        refreshSessionDetailsAgentState()
    }

    private fun markDetailWindowConnecting() {
        detailWindowState = detailWindowState.copy(
            status = AgentMessageStreamStatus.Connecting,
            error = null,
        )
        refreshDerivedState()
    }

    private fun shouldKeepSessionStream(threadId: String, includeMessages: Boolean): Boolean {
        val screenAllowsStream = if (includeMessages) {
            _uiState.value.screen == AppScreen.SessionDetails
        } else {
            _uiState.value.screen == AppScreen.Dashboard &&
                _uiState.value.dashboard.pagerPage == DashboardPage.Home
        }
        return isForegroundVisible &&
            screenAllowsStream &&
            currentToken.isNotBlank() &&
            selectedSessionThreadId == threadId
    }

    private fun handleSessionStreamEvent(threadId: String, event: SessionStreamEvent) {
        if (sessionAgentState.threadId != threadId) {
            if (event is SessionStreamEvent.RuntimeState && selectedSessionThreadId == threadId) {
                sessionAgentState = SessionAgentState(threadId = threadId)
            } else {
                return
            }
        }
        sessionAgentState = when (event) {
            is SessionStreamEvent.AgentMessage -> sessionAgentState.copy(
                latestMessage = event.message,
                status = AgentMessageStreamStatus.Live,
                error = null,
            )
            is SessionStreamEvent.RuntimeState -> {
                rememberSessionTurnDuration(event.state)
                maybeVibrateForCompletedRuntime(sessionAgentState.runtimeState, event.state)
                sessionAgentState.copy(
                    runtimeState = event.state,
                    status = if (sessionAgentState.latestMessage == null) {
                        AgentMessageStreamStatus.Waiting
                    } else {
                        AgentMessageStreamStatus.Live
                    },
                    error = null,
                    completedHapticTurnIds = rememberCompletedHapticTurn(
                        previous = sessionAgentState.runtimeState,
                        next = event.state,
                        completedTurnIds = sessionAgentState.completedHapticTurnIds,
                    ),
                    suppressNextRuntimeHaptic = false,
                )
            }
            SessionStreamEvent.Heartbeat -> {
                if (sessionAgentState.latestMessage == null) {
                    sessionAgentState.copy(status = AgentMessageStreamStatus.Waiting, error = null)
                } else {
                    sessionAgentState.copy(status = AgentMessageStreamStatus.Live, error = null)
                }
            }
            is SessionStreamEvent.Failure -> sessionAgentState.copy(
                status = AgentMessageStreamStatus.Disconnected,
                error = event.message,
            )
        }
        refreshDerivedState()
    }

    private fun handleDetailWindowStreamEvent(event: SessionWindowStreamEvent) {
        detailWindowState = when (event) {
            is SessionWindowStreamEvent.Window -> {
                val previousSelectedThreadId = selectedSessionThreadId
                val previousSelectedSlotIndex = selectedSessionSlotIndex
                val entriesByThreadId = event.window.sessions.associate { entry ->
                    rememberSessionTurnDuration(entry.runtimeState)
                    entry.session.threadId to DetailWindowEntryState(
                        session = entry.session,
                        runtimeState = entry.runtimeState,
                        latestMessage = entry.latestAgentMessage,
                    )
                }
                val nextOrder = event.window.threadOrder.filter { it in entriesByThreadId.keys }
                if (
                    previousSelectedThreadId != null &&
                    previousSelectedThreadId !in nextOrder &&
                    previousSelectedSlotIndex != null
                ) {
                    val replacementThreadId = nextOrder.getOrNull(previousSelectedSlotIndex)
                    viewModelScope.launch {
                        logDetailWindowStreamState(
                            action = "selection_ejected",
                            level = DiagnosticLevel.Warn,
                            fields = mapOf(
                                "previousSelectedThreadId" to previousSelectedThreadId,
                                "previousSelectedSlotIndex" to previousSelectedSlotIndex,
                                "replacementThreadId" to replacementThreadId,
                                "threadOrder" to nextOrder,
                            ),
                        )
                    }
                }
                val nextState = detailWindowState.copy(
                    orderedThreadIds = nextOrder,
                    entriesByThreadId = entriesByThreadId,
                )
                detailWindowState.copy(
                    orderedThreadIds = nextOrder,
                    entriesByThreadId = entriesByThreadId,
                    status = detailWindowContentStatus(
                        selectedThreadId = selectedSessionThreadId,
                        state = nextState,
                    ),
                    error = null,
                )
            }
            is SessionWindowStreamEvent.AgentMessage -> {
                val updatedEntries = detailWindowState.entriesByThreadId.toMutableMap()
                val current = updatedEntries[event.message.threadId]
                if (current != null) {
                    updatedEntries[event.message.threadId] = current.copy(latestMessage = event.message)
                }
                val nextState = detailWindowState.copy(entriesByThreadId = updatedEntries)
                detailWindowState.copy(
                    entriesByThreadId = updatedEntries,
                    status = detailWindowContentStatus(
                        selectedThreadId = selectedSessionThreadId,
                        state = nextState,
                    ),
                    error = null,
                )
            }
            is SessionWindowStreamEvent.RuntimeState -> {
                rememberSessionTurnDuration(event.state)
                val updatedEntries = detailWindowState.entriesByThreadId.toMutableMap()
                val current = updatedEntries[event.state.threadId]
                if (current != null) {
                    updatedEntries[event.state.threadId] = current.copy(runtimeState = event.state)
                }
                val nextState = detailWindowState.copy(entriesByThreadId = updatedEntries)
                detailWindowState.copy(
                    entriesByThreadId = updatedEntries,
                    status = detailWindowContentStatus(
                        selectedThreadId = selectedSessionThreadId,
                        state = nextState,
                    ),
                    error = null,
                )
            }
            SessionWindowStreamEvent.Heartbeat -> detailWindowState.copy(
                status = detailWindowContentStatus(
                    selectedThreadId = selectedSessionThreadId,
                    state = detailWindowState,
                ),
                error = null,
            )
            is SessionWindowStreamEvent.Failure -> detailWindowState.copy(
                status = AgentMessageStreamStatus.Disconnected,
                error = event.message,
            )
        }
        refreshDerivedState()
        persistDetailWindowSnapshot()
    }

    private fun reconnectDelayForAttempt(attempt: Int): Long {
        if (sessionStreamReconnectDelaysMs.isEmpty()) {
            return 1_000L
        }
        val safeIndex = min(attempt - 1, sessionStreamReconnectDelaysMs.lastIndex)
        return sessionStreamReconnectDelaysMs[safeIndex]
    }

    private fun detailWindowDisplayStatus(
        selectedThreadId: String?,
        state: DetailWindowState,
    ): AgentMessageStreamStatus {
        if (
            state.status == AgentMessageStreamStatus.Connecting ||
            state.status == AgentMessageStreamStatus.Disconnected
        ) {
            return state.status
        }
        return detailWindowContentStatus(selectedThreadId, state)
    }

    private fun detailWindowContentStatus(
        selectedThreadId: String?,
        state: DetailWindowState,
    ): AgentMessageStreamStatus {
        val selectedEntry = selectedThreadId?.let(state.entriesByThreadId::get)
            ?: state.orderedThreadIds.firstNotNullOfOrNull(state.entriesByThreadId::get)
        return if (selectedEntry?.latestMessage == null) {
            AgentMessageStreamStatus.Waiting
        } else {
            AgentMessageStreamStatus.Live
        }
    }

    private fun restorePersistedDetailWindowState() {
        if (
            detailWindowState.orderedThreadIds.isNotEmpty() &&
            detailWindowState.entriesByThreadId.isNotEmpty()
        ) {
            return
        }
        val cached = sessionDetailsWindowStore.read() ?: return
        val entriesByThreadId = cached.window.sessions.associate { entry ->
            rememberSessionTurnDuration(entry.runtimeState)
            entry.session.threadId to DetailWindowEntryState(
                session = entry.session,
                runtimeState = entry.runtimeState,
                latestMessage = entry.latestAgentMessage,
            )
        }
        val orderedThreadIds = buildList {
            addAll(cached.window.threadOrder.filter { it in entriesByThreadId })
            addAll(entriesByThreadId.keys.filterNot { it in this })
        }
        if (orderedThreadIds.isEmpty()) {
            sessionDetailsWindowStore.clear()
            return
        }
        detailWindowState = DetailWindowState(
            orderedThreadIds = orderedThreadIds,
            entriesByThreadId = entriesByThreadId,
            status = detailWindowContentStatus(
                selectedThreadId = cached.selectedThreadId,
                state = DetailWindowState(
                    orderedThreadIds = orderedThreadIds,
                    entriesByThreadId = entriesByThreadId,
                    status = AgentMessageStreamStatus.Waiting,
                ),
            ),
        )
        selectedSessionThreadId = cached.selectedThreadId?.takeIf { it in orderedThreadIds }
        selectedSessionSlotIndex = cached.selectedSlotIndex?.coerceIn(0, orderedThreadIds.lastIndex)
        sessionSelectionCursorPosition = selectedSessionSlotIndex?.toFloat()
        refreshDerivedState()
    }

    private fun persistDetailWindowSnapshot() {
        if (detailWindowState.entriesByThreadId.isEmpty()) {
            sessionDetailsWindowStore.clear()
            return
        }
        val windowEntries = detailWindowState.orderedThreadIds.mapNotNull { threadId ->
            detailWindowState.entriesByThreadId[threadId]?.let { entry ->
                SessionWindowEntry(
                    session = entry.session,
                    runtimeState = entry.runtimeState,
                    latestAgentMessage = entry.latestMessage,
                )
            }
        }
        if (windowEntries.isEmpty()) {
            sessionDetailsWindowStore.clear()
            return
        }
        sessionDetailsWindowStore.write(
            SessionDetailsWindowCacheSnapshot(
                selectedThreadId = selectedSessionThreadId,
                selectedSlotIndex = selectedSessionSlotIndex,
                cachedAt = wallClockNow(),
                window = SessionWindowSnapshot(
                    observedAt = wallClockNow(),
                    limit = DETAIL_WINDOW_LIMIT,
                    threadOrder = detailWindowState.orderedThreadIds,
                    sessions = windowEntries,
                ),
            ),
        )
    }

    private suspend fun logDetailWindowStreamState(
        action: String,
        level: DiagnosticLevel = DiagnosticLevel.Info,
        fields: Map<String, Any?> = emptyMap(),
    ) {
        diagnosticEventLogger.log(
            event = "stream_state",
            level = level,
            fields = mapOf(
                "target" to mapOf(
                    "name" to "session_window_stream_client",
                    "path" to "/api/sessions/stream",
                    "method" to "GET",
                ),
                "state" to (mapOf("action" to action) + fields),
            ),
        )
    }

    private suspend fun logDetailWindowDisconnect(
        failure: SessionWindowStreamEvent.Failure?,
        reconnectAttempt: Int,
        retryable: Boolean,
        nextRetryDelayMs: Long?,
    ) {
        if (failure == null) {
            return
        }
        logDetailWindowStreamState(
            action = "disconnect",
            level = DiagnosticLevel.Warn,
            fields = mapOf(
                "reconnectAttempt" to reconnectAttempt,
                "reason" to failure.reason.name.lowercase(Locale.US),
                "detail" to failure.detail,
                "statusCode" to failure.statusCode,
                "retryable" to retryable,
                "nextRetryDelayMs" to nextRetryDelayMs,
            ),
        )
    }

    private fun elapsedSince(startedAtMs: Long): Long {
        return (monotonicNowMs() - startedAtMs).coerceAtLeast(0L)
    }

    private fun reportSessionStreamClientEvent(report: SessionStreamClientEventReport) {
        val token = currentToken.takeIf { it.isNotBlank() } ?: return
        viewModelScope.launch(ioDispatcher) {
            api.reportSessionStreamClientEvent(token, report)
        }
    }

    private fun unexpectedStreamFailure(error: Throwable): SessionStreamEvent.Failure {
        val detail = buildString {
            append(error::class.java.simpleName)
            val suffix = error.message?.trim().orEmpty()
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

    private fun unexpectedDetailWindowFailure(error: Throwable): SessionWindowStreamEvent.Failure {
        val detail = buildString {
            append(error::class.java.simpleName)
            val suffix = error.message?.trim().orEmpty()
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

    private fun unexpectedStatusStreamFailure(error: Throwable): StatusStreamEvent.Failure {
        val detail = buildString {
            append(error::class.java.simpleName)
            val suffix = error.message?.trim().orEmpty()
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

    private fun refreshSessionDetailsAgentState() {
        val currentDetails = _uiState.value.dashboard.sessionDetails
        val selectedThreadId = selectedSessionThreadId ?: currentDetails.rows
            .firstOrNull { it.isSelected }
            ?.sessionId
        _uiState.value = _uiState.value.copy(
            dashboard = _uiState.value.dashboard.copy(
                sessionDetails = currentDetails.withSessionAgentState(selectedThreadId),
            ),
        )
    }

    private fun SessionDetailsUiState.withSessionAgentState(threadId: String?): SessionDetailsUiState {
        if (threadId == null || sessionAgentState.threadId != threadId) {
            return copy(
                latestAgentMessage = null,
                latestAgentMessageAtLabel = null,
                agentMessageStreamStatus = AgentMessageStreamStatus.Disconnected,
                agentMessageError = null,
            )
        }
        val message = sessionAgentState.latestMessage
        return copy(
            latestAgentMessage = message?.text?.let(::cleanAgentMessageText)?.takeIf { it.isNotBlank() },
            latestAgentMessageAtLabel = message?.createdAt?.let(TIME_FORMATTER::format),
            agentMessageStreamStatus = sessionAgentState.status,
            agentMessageError = sessionAgentState.error,
        )
    }

    internal fun classifyHomeQuotaTipPool(snapshot: WatcherStatusSnapshot): HomeQuotaTipPool? {
        val weekly = snapshot.quota?.weekly ?: return null
        val observedAt = snapshot.observedAt ?: return null
        val resetAt = weekly.resetAt ?: return null
        val timeRemainingPercent = computeTimeRemainingPercent(resetAt, observedAt, WEEKLY_WINDOW_SECONDS) ?: return null
        val delta = weekly.remainingPercent - timeRemainingPercent
        return classifyHomeQuotaTipPool(delta)
    }

    internal fun classifyHomeQuotaTipPool(delta: Float): HomeQuotaTipPool {
        return when {
            delta <= -18f -> HomeQuotaTipPool.TooFast
            delta <= -6f -> HomeQuotaTipPool.Fast
            delta < 6f -> HomeQuotaTipPool.Balanced
            delta < 18f -> HomeQuotaTipPool.Slow
            else -> HomeQuotaTipPool.TooSlow
        }
    }

    internal fun pickHomeQuotaTip(pool: HomeQuotaTipPool): String? {
        val entries = HomeQuotaEasterEggCopyConfig.entriesFor(pool)
        if (entries.isEmpty()) {
            return null
        }
        val nextIndex = pickHomeQuotaTipIndex(pool, entries.size)
        homeQuotaTipLastIndexByPool[pool] = nextIndex
        return entries[nextIndex]
    }

    private fun pickHomeQuotaTipIndex(
        pool: HomeQuotaTipPool,
        size: Int,
    ): Int {
        if (size <= 1) {
            return 0
        }
        val lastIndex = homeQuotaTipLastIndexByPool[pool]
        if (lastIndex == null || lastIndex !in 0 until size) {
            return nextHomeQuotaTipRandomIndex(size)
        }
        val rawIndex = nextHomeQuotaTipRandomIndex(size - 1)
        return if (rawIndex >= lastIndex) rawIndex + 1 else rawIndex
    }

    private fun nextHomeQuotaTipRandomIndex(bound: Int): Int {
        if (bound <= 1) {
            return 0
        }
        return homeQuotaTipRandomIndex(bound).coerceIn(0, bound - 1)
    }

    private fun buildUpdatedLabel(observedAt: Instant?, fresh: Boolean): String {
        val timeText = observedAt?.let { TIME_FORMATTER.format(it) } ?: "--:--"
        val suffix = if (fresh) "新" else "缓"
        return "$timeText · $suffix"
    }

    private fun QuotaWindow?.toRingState(
        title: String,
        observedAt: Instant?,
        totalWindowSeconds: Long,
        isDimmed: Boolean,
    ): QuotaRingUiState {
        if (this == null) {
            return QuotaRingUiState(title = title, remainingLabel = "--", isDimmed = isDimmed)
        }
        val reference = observedAt ?: Instant.now()
        return QuotaRingUiState(
            title = title,
            usedPercent = usedPercent,
            remainingPercent = remainingPercent,
            timeRemainingPercent = computeTimeRemainingPercent(resetAt, observedAt, totalWindowSeconds),
            remainingLabel = formatRemainingDuration(resetAt, reference),
            isDimmed = isDimmed,
        )
    }

    private fun computeTimeRemainingPercent(
        resetAt: Instant?,
        observedAt: Instant?,
        totalWindowSeconds: Long,
    ): Float? {
        if (resetAt == null || observedAt == null || totalWindowSeconds <= 0L || resetAt <= observedAt) {
            return null
        }
        val remainingSeconds = (resetAt.epochSecond - observedAt.epochSecond).coerceAtLeast(0L)
        val fraction = (remainingSeconds.toDouble() / totalWindowSeconds.toDouble()).coerceIn(0.0, 1.0)
        return (fraction * 100.0).toFloat()
    }

    private fun buildEmptyHeatmapSegments(anchor: Instant): List<HeatmapSegmentUiState> {
        val localAnchor = anchor.atZone(ZoneId.systemDefault())
        val start = localAnchor.withHour(0).withMinute(0).withSecond(0).withNano(0)
        return List(24) { index ->
            val hourStart = start.plusHours(index.toLong())
            HeatmapSegmentUiState(
                hourLabel = hourStart.format(HOUR_FORMATTER),
                timeRangeLabel = "${hourStart.format(HOUR_MINUTE_FORMATTER)}-${hourStart.plusHours(1).format(HOUR_MINUTE_FORMATTER)}",
                intensity = 0f,
            )
        }
    }

    private fun formatCompactTokens(value: Long): String {
        val absoluteValue = abs(value)
        val units = listOf(
            1_000_000_000L to "B",
            1_000_000L to "M",
            1_000L to "K",
        )
        for ((unitValue, unitLabel) in units) {
            if (absoluteValue >= unitValue) {
                val number = value / unitValue.toDouble()
                return trimZero(String.format(Locale.US, "%.1f", number)) + unitLabel
            }
        }
        return value.toString()
    }

    private fun formatContextLabel(used: Long, window: Long): String {
        return "${formatContextMetricTokens(used)}/${formatContextMetricTokens(window)}"
    }

    private fun formatContextMetricTokens(value: Long): String {
        return formatCompactTokens(value)
    }

    private fun formatTrendDateLabel(value: String): String {
        return runCatching {
            LocalDate.parse(value).format(TREND_DATE_FORMATTER)
        }.getOrDefault(value)
    }

    private fun lastActiveLabel(session: SessionSnapshot): String {
        return formatAgoShort(lastActiveAgoMinutes(session))
    }

    private fun lastActiveAgoMinutes(session: SessionSnapshot): Int {
        val updatedAt = session.updatedAt ?: return session.lastActiveAgoMinutes
        val minutes = ((wallClockNow().epochSecond - updatedAt.epochSecond).coerceAtLeast(60L) / 60L)
        return minutes.coerceAtMost(Int.MAX_VALUE.toLong()).toInt()
    }

    private fun sessionRelativeIntensity(index: Int): Float {
        return when (index.coerceAtLeast(0)) {
            0 -> 1f
            1 -> 0.78f
            2 -> 0.56f
            3 -> 0.36f
            else -> 0.20f
        }
    }

    private fun SessionSnapshot.compactThresholdPercent(): Float? {
        return contextCompactThresholdPercent
            ?.takeIf { it in 1..100 }
            ?.toFloat()
    }

    private fun isNearCompactThreshold(
        pressurePercent: Float,
        compactThresholdPercent: Float?,
    ): Boolean {
        return compactThresholdPercent != null &&
            abs(pressurePercent.coerceIn(0f, 100f) - compactThresholdPercent.coerceIn(0f, 100f)) < 5f
    }

    private fun formatAgoShort(minutes: Int): String {
        val safeMinutes = minutes.coerceAtLeast(1)
        val hourMinutes = 60
        val dayMinutes = 24 * hourMinutes
        val weekMinutes = 7 * dayMinutes
        return when {
            safeMinutes < hourMinutes -> "${safeMinutes}m"
            safeMinutes < dayMinutes -> "${safeMinutes / hourMinutes}h"
            safeMinutes < weekMinutes -> "${safeMinutes / dayMinutes}d"
            else -> "${(safeMinutes / weekMinutes).coerceIn(1, 4)}w"
        }
    }

    private fun formatHourRange(start: Instant): String {
        val zoned = start.atZone(ZoneId.systemDefault())
        val end = zoned.plusHours(1)
        return "${HOUR_MINUTE_FORMATTER.format(zoned)}-${HOUR_MINUTE_FORMATTER.format(end)}"
    }

    private fun formatRemainingDuration(
        resetAt: Instant?,
        reference: Instant,
    ): String {
        if (resetAt == null) {
            return "--"
        }
        val totalMinutes = ((resetAt.epochSecond - reference.epochSecond).coerceAtLeast(0) + 59) / 60
        val days = totalMinutes / (24 * 60)
        val hours = (totalMinutes % (24 * 60)) / 60
        val minutes = totalMinutes % 60
        return when {
            days > 0 -> "${days}d ${hours}h ${minutes}m"
            hours > 0 -> "${hours}h ${minutes}m"
            else -> "${minutes}m"
        }
    }

    private fun formatBinaryBytes(value: Long): String {
        val absoluteValue = abs(value)
        val units = listOf(
            1024L * 1024L to "MB",
            1024L to "KB",
        )
        for ((unitValue, unitLabel) in units) {
            if (absoluteValue >= unitValue) {
                val number = value / unitValue.toDouble()
                return trimZero(String.format(Locale.US, "%.1f", number)) + " " + unitLabel
            }
        }
        return "$value B"
    }

    private fun formatVersionLabel(versionName: String, versionCode: Int): String {
        val safeName = versionName.ifBlank { "--" }
        return "$safeName ($versionCode)"
    }

    private fun formatVersionShort(versionName: String): String {
        return versionName.ifBlank { "--" }
    }

    private fun formatInstallPermissionLabel(enabled: Boolean): String {
        return if (enabled) "已允许" else "未允许"
    }

    private fun incrementPatchVersionName(versionName: String): String {
        val parts = versionName.split('.')
        if (parts.size != 3) {
            return versionName.ifBlank { "0.0.1" }
        }
        val major = parts[0].toIntOrNull() ?: return versionName
        val minor = parts[1].toIntOrNull() ?: return versionName
        val patch = parts[2].toIntOrNull() ?: return versionName
        return "$major.$minor.${patch + 1}"
    }

    private fun formatModelTitle(value: String): String {
        return value.split('-')
            .joinToString("-") { part ->
                when (part.lowercase(Locale.US)) {
                    "gpt" -> "GPT"
                    "mini" -> "Mini"
                    else -> part
                }
            }
    }

    private fun formatReasoningTitle(value: String): String {
        return when (value.lowercase(Locale.US)) {
            "xhigh", "xhight" -> "xhigh"
            "high" -> "high"
            "medium" -> "medium"
            "low" -> "low"
            else -> value
        }
    }

    private fun formatPercent(ratio: Double): String {
        return "${(ratio.coerceIn(0.0, 1.0) * 100.0).roundToInt()}%"
    }

    private fun trimZero(text: String): String {
        if (!text.contains('.')) {
            return text
        }
        return text.trimEnd('0').trimEnd('.')
    }

    override fun onCleared() {
        homeQuotaTipHideJob?.cancel()
        heatmapCursorSettleJob?.cancel()
        heatmapTrendTipHideJob?.cancel()
        sessionRuntimeDurationTickerJob?.cancel()
        stopSessionStream(clearMessage = true)
        stopDetailWindowStream(clearState = true)
        pollJob?.cancel()
        watchBootstrapJob?.cancel()
        super.onCleared()
    }

    private fun wrapIndex(index: Int, size: Int): Int {
        if (size <= 0) {
            return 0
        }
        val raw = index % size
        return if (raw < 0) raw + size else raw
    }

    private fun watchBootstrapPollDelayMs(attempt: Int): Long {
        return when {
            attempt < 10 -> 1_000L
            attempt < 22 -> 5_000L
            attempt < 28 -> 10_000L
            else -> 60_000L
        }
    }

    private fun cleanAgentMessageText(input: String): String {
        return input
            .trim()
            .lines()
            .map { it.trim() }
            .filter { it.isNotBlank() }
            .joinToString("\n")
    }

    private fun extractHostLabel(baseUrl: String): String {
        return baseUrl
            .substringAfter("://", baseUrl)
            .substringBefore("/")
            .ifBlank { baseUrl }
    }

    private fun applyRuntimeServerConfig(config: ServerConfig) {
        currentServerConfig = config
        val activeEndpoint = config.activeEndpoint()
        currentBaseUrl = activeEndpoint.url
        serviceHostLabel = extractHostLabel(activeEndpoint.url)
        activeEndpointLabel = activeEndpoint.label
        savedEndpointSummary = config.endpointSummary()
        diagnosticRuntimeContext?.updateBaseUrl(currentBaseUrl)
    }

    private fun applySelectedEnvironmentRuntimeContext() {
        applyRuntimeServerConfig(serverConfigRepository.current(activeEnvironment()))
        currentToken = currentRuntimeToken()
        hasPaired = hasCurrentRuntimeConfig()
        updateDiagnosticPairedState()
        _uiState.value = _uiState.value.copy(
            pairing = _uiState.value.pairing.copy(
                serviceHostLabel = serviceHostLabel,
                serviceBaseUrl = currentBaseUrl,
                environmentLabel = currentPairingEnvironmentLabel(),
            ),
            dashboard = _uiState.value.dashboard.copy(serviceHostLabel = serviceHostLabel),
            settings = _uiState.value.settings.copy(
                baseUrl = currentBaseUrl,
                activeEndpointLabel = activeEndpointLabel,
                savedEndpointCountLabel = endpointCountLabel(),
                savedEndpointSummary = savedEndpointSummary,
                serviceHostLabel = serviceHostLabel,
            ),
            offline = buildOfflineState("无法连接到 OpenWatcher"),
        )
    }

    private fun endpointCountLabel(): String = "${currentServerConfig.endpoints.size} 项"

    private fun endpointFailureSummary(selection: EndpointSelection): String {
        return selection.probeResults.joinToString("；") { "${it.endpoint.label}：${it.message}" }
    }

    private suspend fun selectAndApplyEndpoint(config: ServerConfig): EndpointSelection {
        val selection = endpointSelector.selectActiveEndpoint(
            endpoints = config.endpoints,
            preferredEndpointId = config.activeEndpointId,
        )
        applyRuntimeServerConfig(serverConfigRepository.updateActiveEndpoint(activeEnvironment(), selection.activeEndpoint.id))
        return selection
    }

    private suspend fun trySwitchEndpointAfterTransportFailure(fromPairingLoop: Boolean): Boolean {
        if (fromPairingLoop || !hasPaired || currentServerConfig.endpoints.size < 2) {
            return false
        }
        val previousConfig = currentServerConfig
        val selection = endpointSelector.selectActiveEndpoint(
            endpoints = previousConfig.endpoints,
            preferredEndpointId = previousConfig.activeEndpointId,
        )
        if (!selection.hasReachableEndpoint || selection.activeEndpoint.id == previousConfig.activeEndpointId) {
            return false
        }

        applyRuntimeServerConfig(serverConfigRepository.updateActiveEndpoint(activeEnvironment(), selection.activeEndpoint.id))
        return when (val retry = api.fetchStatus(currentToken)) {
            is StatusFetchResult.Success -> {
                handleSuccess(retry.snapshot)
                true
            }

            StatusFetchResult.Unauthorized -> {
                handleUnauthorized(fromPairingLoop = false)
                true
            }

            is StatusFetchResult.NetworkFailure,
            is StatusFetchResult.ParseFailure,
            is StatusFetchResult.HttpFailure -> {
                applyRuntimeServerConfig(serverConfigRepository.updateActiveEndpoint(activeEnvironment(), previousConfig.activeEndpointId))
                false
            }
        }
    }

    private fun activeEnvironment(): AppUpdateChannel {
        return if (
            appUpdatePreferences.selectedChannel == AppUpdateChannel.Dev &&
            !serverConfigRepository.hasStoredConfig(AppUpdateChannel.Dev)
        ) {
            AppUpdateChannel.Beta
        } else {
            appUpdatePreferences.selectedChannel
        }
    }

    private fun currentRuntimeToken(): String {
        return when (activeEnvironment()) {
            AppUpdateChannel.Beta -> serverConfigRepository.currentDeviceToken(AppUpdateChannel.Beta)
                ?: tokenRepository.ensureToken()
            AppUpdateChannel.Dev -> serverConfigRepository.currentDeviceToken(AppUpdateChannel.Dev).orEmpty()
        }
    }

    private fun hasCurrentRuntimeConfig(): Boolean {
        return when (activeEnvironment()) {
            AppUpdateChannel.Beta -> pairingStateStore.isPaired()
            AppUpdateChannel.Dev -> serverConfigRepository.profile(AppUpdateChannel.Dev)?.source != ServerConfigSource.RemoteBootstrap &&
                serverConfigRepository.hasStoredConfig(AppUpdateChannel.Dev)
        }
    }

    private fun pairingBaseUrl(): String {
        return serverConfigRepository.current(activeEnvironment()).activeEndpoint().url
    }

    private fun currentPairingEnvironmentLabel(): String {
        return activeEnvironment().toPairingEnvironmentLabel()
    }

    private fun AppUpdateChannel.toPairingEnvironmentLabel(): String {
        return when (this) {
            AppUpdateChannel.Beta -> ""
            AppUpdateChannel.Dev -> "环境：开发"
        }
    }

    private fun pairingToken(): String {
        return currentToken.ifBlank {
            serverConfigRepository.currentDeviceToken(activeEnvironment())
                ?: tokenRepository.ensureToken()
        }
    }

    companion object {
        private const val DETAIL_WINDOW_LIMIT = 5
        private const val SPLASH_MIN_DURATION_MS = 2_000L
        private const val HOME_QUOTA_TIP_HIDE_DURATION_MS = 2_000L
        private const val WATCH_BOOTSTRAP_CONFIG_CHECK_INTERVAL_MS = 1_000L
        private const val WATCH_BOOTSTRAP_SETUP_HINT = "请通过桌面应用的安装向导，或「手表设备->远程初始化」进行配置。"
        private val RENEWABLE_WATCH_BOOTSTRAP_ERROR_CODES = setOf(
            "invalid_bootstrap_code",
            "bootstrap_code_expired",
            "bootstrap_code_consumed",
        )
        private const val FIVE_HOUR_WINDOW_SECONDS = 5L * 60L * 60L
        private const val WEEKLY_WINDOW_SECONDS = 7L * 24L * 60L * 60L
        private val DEFAULT_SESSION_STREAM_RECONNECT_DELAYS_MS = listOf(1_000L, 2_000L, 5_000L, 10_000L)
        private val TIME_FORMATTER: DateTimeFormatter =
            DateTimeFormatter.ofPattern("HH:mm").withZone(ZoneId.systemDefault())
        private val BEIJING_TIME_FORMATTER: DateTimeFormatter =
            DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm").withZone(ZoneId.of("Asia/Shanghai"))
        private val DIAGNOSTIC_TIME_FORMATTER: DateTimeFormatter =
            DateTimeFormatter.ofPattern("MM-dd HH:mm").withZone(ZoneId.systemDefault())
        private val HOUR_FORMATTER: DateTimeFormatter =
            DateTimeFormatter.ofPattern("HH").withZone(ZoneId.systemDefault())
        private val HOUR_MINUTE_FORMATTER: DateTimeFormatter =
            DateTimeFormatter.ofPattern("HH:mm").withZone(ZoneId.systemDefault())
        private val HOME_HEATMAP_DATE_FORMATTER: DateTimeFormatter =
            DateTimeFormatter.ofPattern("MM.dd")
        private val TREND_DATE_FORMATTER: DateTimeFormatter =
            DateTimeFormatter.ofPattern("M月d日")
    }

    private data class SessionAgentState(
        val threadId: String? = null,
        val latestMessage: SessionAgentMessage? = null,
        val runtimeState: SessionRuntimeState? = null,
        val status: AgentMessageStreamStatus = AgentMessageStreamStatus.Disconnected,
        val error: String? = null,
        val completedHapticTurnIds: Set<String> = emptySet(),
        val suppressNextRuntimeHaptic: Boolean = true,
    )

    private data class SessionTurnDuration(
        val turnId: String,
        val startedAt: Instant? = null,
        val completedDurationMs: Long? = null,
        val lifecycle: SessionRuntimeLifecycle = SessionRuntimeLifecycle.Idle,
        val updatedAt: Instant? = null,
    )

    private data class DetailWindowState(
        val orderedThreadIds: List<String> = emptyList(),
        val entriesByThreadId: Map<String, DetailWindowEntryState> = emptyMap(),
        val status: AgentMessageStreamStatus = AgentMessageStreamStatus.Disconnected,
        val error: String? = null,
    )

    private data class DetailWindowEntryState(
        val session: SessionSnapshot,
        val runtimeState: SessionRuntimeState,
        val latestMessage: SessionAgentMessage?,
    )
}

class WatcherViewModelFactory(
    private val api: WatcherApi,
    private val appUpdateManager: WatcherApkUpdateManager = NoOpWatcherApkUpdateManager,
    private val appUpdatePreferenceStore: AppUpdatePreferenceStore,
    private val tokenRepository: DeviceTokenRepository,
    private val serverConfigRepository: ServerConfigRepository,
    private val endpointSelector: EndpointSelector,
    private val config: WatcherViewModel.Config,
    private val pairingStateStore: PairingStateStore,
    private val dailyTrendStore: DailyTrendStore,
    private val statusSnapshotStore: StatusSnapshotStore,
    private val sessionDetailsWindowStore: SessionDetailsWindowStore,
    private val watchBootstrapClient: WatchBootstrapGateway? = null,
    private val watchBootstrapCodeStore: WatchBootstrapCodeStore? = null,
    private val screenshotUploadQueue: ScreenshotUploadQueue = NoOpScreenshotUploadQueue,
    private val diagnosticEventLogger: DiagnosticEventLogger = NoOpDiagnosticEventLogger,
    private val diagnosticUploadManager: DiagnosticUploadManager? = null,
    private val diagnosticUiStateSink: (AppUiState) -> Unit = {},
    private val diagnosticRuntimeContext: DiagnosticRuntimeContext? = null,
    private val debugStore: DebugDemoPreferenceStore?,
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
    private val hapticController: WatcherHapticController = WatcherHapticController {},
) : ViewModelProvider.Factory {
    @Suppress("UNCHECKED_CAST")
    override fun <T : ViewModel> create(modelClass: Class<T>): T {
        return WatcherViewModel(
            api = api,
            appUpdateManager = appUpdateManager,
            appUpdatePreferenceStore = appUpdatePreferenceStore,
            tokenRepository = tokenRepository,
            serverConfigRepository = serverConfigRepository,
            endpointSelector = endpointSelector,
            config = config,
            pairingStateStore = pairingStateStore,
            dailyTrendStore = dailyTrendStore,
            statusSnapshotStore = statusSnapshotStore,
            sessionDetailsWindowStore = sessionDetailsWindowStore,
            watchBootstrapClient = watchBootstrapClient,
            watchBootstrapCodeStore = watchBootstrapCodeStore,
            screenshotUploadQueue = screenshotUploadQueue,
            diagnosticEventLogger = diagnosticEventLogger,
            diagnosticUploadManager = diagnosticUploadManager,
            diagnosticUiStateSink = diagnosticUiStateSink,
            diagnosticRuntimeContext = diagnosticRuntimeContext,
            debugStore = debugStore,
            ioDispatcher = ioDispatcher,
            autoPolling = true,
            hapticController = hapticController,
        ) as T
    }
}
