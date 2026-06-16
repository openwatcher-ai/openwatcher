package ai.openwatcher.watchapp

import android.graphics.Bitmap
import android.graphics.Canvas
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.VibrationEffect
import android.os.VibratorManager
import android.view.InputDevice
import android.view.MotionEvent
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import androidx.lifecycle.Lifecycle
import java.io.ByteArrayOutputStream
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import ai.openwatcher.watchapp.data.FileScreenshotUploadQueue
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticAppInfo
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticDeviceInfo
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticEventStore
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticRuntimeContext
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticSnapshotCollector
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticUiStateHolder
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticUploadManager
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticUploadPreferenceStore
import ai.openwatcher.watchapp.data.diagnostics.StructuredDiagnosticEventLogger
import ai.openwatcher.watchapp.data.DebugDemoScenario
import ai.openwatcher.watchapp.data.DebugDemoPreferenceStore
import ai.openwatcher.watchapp.data.DebugAwareWatcherApi
import ai.openwatcher.watchapp.data.DailyTrendPreferenceStore
import ai.openwatcher.watchapp.data.DeviceTokenRepository
import ai.openwatcher.watchapp.data.AppUpdateChannel
import ai.openwatcher.watchapp.data.DynamicWatcherApi
import ai.openwatcher.watchapp.data.DynamicWatcherApkUpdateManager
import ai.openwatcher.watchapp.data.EndpointSelector
import ai.openwatcher.watchapp.data.HttpWatcherApi
import ai.openwatcher.watchapp.data.PairingPreferenceStore
import ai.openwatcher.watchapp.data.SessionDetailsWindowPreferenceStore
import ai.openwatcher.watchapp.data.ServerConfigRepository
import ai.openwatcher.watchapp.data.BootstrapParseResult
import ai.openwatcher.watchapp.data.WatcherApiEndpointHealthProbe
import ai.openwatcher.watchapp.data.parseBootstrapRequest
import ai.openwatcher.watchapp.data.SecureRandomTokenGenerator
import ai.openwatcher.watchapp.data.SharedPreferencesKeyValueStore
import ai.openwatcher.watchapp.data.SharedPreferencesAppUpdatePreferenceStore
import ai.openwatcher.watchapp.data.WatcherStatusSnapshotPreferenceStore
import ai.openwatcher.watchapp.data.WatcherHttpClients
import ai.openwatcher.watchapp.data.WatchBootstrapClient
import ai.openwatcher.watchapp.data.WatchBootstrapCodeStore
import ai.openwatcher.watchapp.ui.AppScreen
import ai.openwatcher.watchapp.ui.SettingsDestination
import ai.openwatcher.watchapp.ui.WatcherHapticController
import ai.openwatcher.watchapp.ui.WatcherViewModel
import ai.openwatcher.watchapp.ui.WatcherViewModelFactory
import ai.openwatcher.watchapp.ui.theme.OpenWatcherTheme

class MainActivity : ComponentActivity() {
    private lateinit var visibilityController: WearVisibilityController
    private var debugPreviewJob: Job? = null
    private lateinit var diagnosticUploadManager: DiagnosticUploadManager

    private val viewModel: WatcherViewModel by viewModels {
        val prefs = getSharedPreferences("openwatcher_watch", MODE_PRIVATE)
        val store = SharedPreferencesKeyValueStore(prefs)
        val tokenRepository = DeviceTokenRepository(store, SecureRandomTokenGenerator())
        val serverConfigRepository = ServerConfigRepository(
            store = store,
            fallbackBaseUrl = BuildConfig.OPENWATCHER_BASE_URL,
        )
        val pairingStateStore = PairingPreferenceStore(store)
        val dailyTrendStore = DailyTrendPreferenceStore(store)
        val statusSnapshotStore = WatcherStatusSnapshotPreferenceStore(store)
        val sessionDetailsWindowStore = SessionDetailsWindowPreferenceStore(store)
        val watchBootstrapCodeStore = WatchBootstrapCodeStore(store)
        val appUpdatePreferenceStore = SharedPreferencesAppUpdatePreferenceStore(store)
        val selectedEnvironmentProvider = { appUpdatePreferenceStore.read().selectedChannel }
        val diagnosticUiStateHolder = DiagnosticUiStateHolder()
        val diagnosticRuntimeContext = DiagnosticRuntimeContext(
            baseUrl = serverConfigRepository.currentBaseUrl(selectedEnvironmentProvider()),
        )
        val diagnosticEventStore = DiagnosticEventStore(filesDir.resolve("diagnostics/events"))
        val diagnosticEventLogger = StructuredDiagnosticEventLogger(
            store = diagnosticEventStore,
            runtimeContext = diagnosticRuntimeContext,
            uiStateProvider = diagnosticUiStateHolder::current,
            deviceInfo = diagnosticDeviceInfo(),
            appInfo = DiagnosticAppInfo(
                versionName = BuildConfig.VERSION_NAME,
                versionCode = BuildConfig.VERSION_CODE,
                buildType = BuildConfig.BUILD_TYPE,
            ),
        )
        val diagnosticSnapshotCollector = DiagnosticSnapshotCollector(
            uiStateProvider = diagnosticUiStateHolder::current,
            eventLogger = diagnosticEventLogger,
        )
        val diagnosticUploadStateStore = DiagnosticUploadPreferenceStore(store)
        val debugStore = if (BuildConfig.ENABLE_DEBUG_DEMO) DebugDemoPreferenceStore(store) else null
        parseDebugScenario(intent)?.let { debugStore?.set(it) }
        val defaultClient = WatcherHttpClients.createDefaultClient()
        val baseApi = DynamicWatcherApi(
            baseUrlProvider = { serverConfigRepository.currentBaseUrl(selectedEnvironmentProvider()) },
            callFactory = defaultClient,
            streamCallFactory = WatcherHttpClients.createSessionStreamClient(defaultClient),
            diagnosticEventLogger = diagnosticEventLogger,
        )
        val endpointSelector = EndpointSelector(
            probe = WatcherApiEndpointHealthProbe { baseUrl ->
                HttpWatcherApi(
                    baseUrl = baseUrl,
                    callFactory = defaultClient,
                    diagnosticEventLogger = diagnosticEventLogger,
                )
            },
        )
        val api = if (BuildConfig.ENABLE_DEBUG_DEMO) {
            DebugAwareWatcherApi(
                delegate = baseApi,
                demoStore = requireNotNull(debugStore),
            )
        } else {
            baseApi
        }
        val appUpdateManager = DynamicWatcherApkUpdateManager(
            context = this,
            baseUrlProvider = { serverConfigRepository.currentBaseUrl(selectedEnvironmentProvider()) },
            channelProvider = selectedEnvironmentProvider,
            deviceTokenProvider = {
                val selectedEnvironment = selectedEnvironmentProvider()
                serverConfigRepository.currentDeviceToken(selectedEnvironment)
                    ?: if (selectedEnvironment == AppUpdateChannel.Beta) tokenRepository.currentToken() else null
            },
            currentVersionCode = BuildConfig.VERSION_CODE,
            callFactory = defaultClient,
            betaPrimaryMetadataUrl = BuildConfig.OPENWATCHER_BETA_UPDATE_PRIMARY_URL,
            betaBackupMetadataUrl = BuildConfig.OPENWATCHER_BETA_UPDATE_BACKUP_URL,
            ioDispatcher = kotlinx.coroutines.Dispatchers.IO,
            diagnosticEventLogger = diagnosticEventLogger,
        )
        diagnosticUploadManager = DiagnosticUploadManager(
            api = api,
            eventStore = diagnosticEventStore,
            eventLogger = diagnosticEventLogger,
            snapshotCollector = diagnosticSnapshotCollector,
            stateStore = diagnosticUploadStateStore,
            pendingDirectory = filesDir.resolve("diagnostics/pending"),
            deviceName = deviceName(),
            appVersion = BuildConfig.VERSION_NAME,
        )

        WatcherViewModelFactory(
            api = api,
            appUpdateManager = appUpdateManager,
            appUpdatePreferenceStore = appUpdatePreferenceStore,
            tokenRepository = tokenRepository,
            serverConfigRepository = serverConfigRepository,
            endpointSelector = endpointSelector,
            config = WatcherViewModel.Config(
                baseUrl = BuildConfig.OPENWATCHER_BASE_URL,
                deviceName = deviceName(),
                appVersion = BuildConfig.VERSION_NAME,
                appVersionCode = BuildConfig.VERSION_CODE,
                debugToolsEnabled = BuildConfig.ENABLE_DEBUG_DEMO,
            ),
            pairingStateStore = pairingStateStore,
            dailyTrendStore = dailyTrendStore,
            statusSnapshotStore = statusSnapshotStore,
            sessionDetailsWindowStore = sessionDetailsWindowStore,
            watchBootstrapClient = WatchBootstrapClient(
                baseUrl = BuildConfig.OPENWATCHER_BOOTSTRAP_BASE_URL,
                callFactory = defaultClient,
            ),
            watchBootstrapCodeStore = watchBootstrapCodeStore,
            screenshotUploadQueue = FileScreenshotUploadQueue(filesDir.resolve("pending-screenshots")),
            diagnosticEventLogger = diagnosticEventLogger,
            diagnosticUploadManager = diagnosticUploadManager,
            diagnosticUiStateSink = diagnosticUiStateHolder::update,
            diagnosticRuntimeContext = diagnosticRuntimeContext,
            debugStore = debugStore,
            hapticController = AndroidWatcherHapticController(this),
        )
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        visibilityController = WearVisibilityController(this)
        enableEdgeToEdge()
        lifecycleScope.launch {
            repeatOnLifecycle(Lifecycle.State.STARTED) {
                viewModel.uiState.collect { state ->
                    visibilityController.update(state)
                }
            }
        }
        setContent {
            OpenWatcherTheme {
                WatcherApp(
                    viewModel = viewModel,
                    onScreenshotRequested = {
                        viewModel.captureAndUploadScreenshot(::captureCurrentAppScreenshotPng)
                    },
                )
            }
        }
        applyDebugIntent(intent)
        applyBootstrapIntent(intent)
    }

    override fun onDestroy() {
        visibilityController.cancel()
        if (::diagnosticUploadManager.isInitialized) {
            diagnosticUploadManager.close()
        }
        super.onDestroy()
    }

    override fun onStart() {
        super.onStart()
        viewModel.setForegroundVisible(true)
    }

    override fun onStop() {
        viewModel.setForegroundVisible(false)
        super.onStop()
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        applyDebugIntent(intent)
        applyBootstrapIntent(intent)
    }

    override fun dispatchGenericMotionEvent(event: MotionEvent): Boolean {
        if (handleRotaryScroll(event)) {
            return true
        }
        return super.dispatchGenericMotionEvent(event)
    }

    private fun deviceName(): String {
        val manufacturer = Build.MANUFACTURER.orEmpty().trim()
        val model = Build.MODEL.orEmpty().trim()
        val raw = listOf(manufacturer, model)
            .filter { it.isNotBlank() }
            .joinToString(" ")
            .trim()
        return if (raw.isBlank()) "Android 手表" else raw
    }

    private fun diagnosticDeviceInfo(): DiagnosticDeviceInfo {
        val metrics = resources.displayMetrics
        val configuration = resources.configuration
        return DiagnosticDeviceInfo(
            manufacturer = Build.MANUFACTURER.orEmpty().trim(),
            model = Build.MODEL.orEmpty().trim(),
            sdkInt = Build.VERSION.SDK_INT,
            screenWidthPx = metrics.widthPixels,
            screenHeightPx = metrics.heightPixels,
            densityDpi = metrics.densityDpi,
            fontScale = configuration.fontScale,
            isRound = configuration.isScreenRound,
            smallestWidthDp = configuration.smallestScreenWidthDp,
        )
    }

    private fun handleRotaryScroll(event: MotionEvent): Boolean {
        if (event.action != MotionEvent.ACTION_SCROLL) {
            return false
        }
        if (!event.isFromSource(InputDevice.SOURCE_ROTARY_ENCODER)) {
            return false
        }

        val rawScroll = event.getAxisValue(MotionEvent.AXIS_SCROLL)
            .takeIf { it != 0f }
            ?: event.getAxisValue(MotionEvent.AXIS_VSCROLL)
        if (rawScroll == 0f) {
            return false
        }

        when (viewModel.uiState.value.screen) {
            AppScreen.Heatmap24h -> viewModel.rotateHeatmapCursor(rawScroll)
            AppScreen.SessionDetails -> viewModel.scrollSessionMessage(rawScroll)
            else -> return false
        }
        return true
    }

    private fun captureCurrentAppScreenshotPng(): ByteArray? {
        val view = window.decorView.rootView ?: return null
        val width = view.width
        val height = view.height
        if (width <= 0 || height <= 0) {
            return null
        }
        val bitmap = Bitmap.createBitmap(width, height, Bitmap.Config.ARGB_8888)
        return try {
            val canvas = Canvas(bitmap)
            view.draw(canvas)
            ByteArrayOutputStream().use { output ->
                if (!bitmap.compress(Bitmap.CompressFormat.PNG, 100, output)) {
                    null
                } else {
                    output.toByteArray()
                }
            }
        } finally {
            bitmap.recycle()
        }
    }

    private fun applyBootstrapIntent(intent: Intent?) {
        val dataString = intent?.dataString?.trim().orEmpty()
        if (intent?.action != Intent.ACTION_VIEW || dataString.isBlank()) {
            return
        }
        val data = runCatching { Uri.parse(dataString) }.getOrNull() ?: return
        if (!data.scheme.equals("openwatcher", ignoreCase = true)) {
            return
        }
        val host = data.host.orEmpty()
        if (!host.equals("bootstrap", ignoreCase = true) && !host.equals("dev-bootstrap", ignoreCase = true)) {
            return
        }

        when (val result = parseBootstrapRequest(data)) {
            is BootstrapParseResult.Success -> viewModel.presentBootstrapRequest(result.request)
            is BootstrapParseResult.Invalid -> viewModel.showBootstrapError(result.message)
        }
    }

    private fun parseDebugScenario(intent: Intent?): DebugDemoScenario? {
        if (!BuildConfig.ENABLE_DEBUG_DEMO) {
            return null
        }
        val raw = intent?.getStringExtra(EXTRA_DEBUG_SCENARIO)?.trim().orEmpty()
        if (raw.isBlank()) {
            return null
        }
        return DebugDemoScenario.entries.firstOrNull { it.name.equals(raw, ignoreCase = true) }
    }

    private fun applyDebugIntent(intent: Intent?) {
        parseDebugScenario(intent)?.let(viewModel::setDebugScenario)
        if (!BuildConfig.ENABLE_DEBUG_DEMO || intent == null) {
            return
        }
        val destination = parseDebugSettingsDestination(intent)
        val updatePreview = intent.getStringExtra(EXTRA_DEBUG_UPDATE_PREVIEW)?.trim()
        val installPermissionEnabled = if (intent.hasExtra(EXTRA_DEBUG_INSTALL_PERMISSION_ENABLED)) {
            intent.getBooleanExtra(EXTRA_DEBUG_INSTALL_PERMISSION_ENABLED, false)
        } else {
            null
        }
        val shouldOpenSettings = intent.getBooleanExtra(EXTRA_DEBUG_OPEN_SETTINGS, false) ||
            destination != null ||
            !updatePreview.isNullOrBlank() ||
            installPermissionEnabled != null
        if (!shouldOpenSettings) {
            return
        }
        debugPreviewJob?.cancel()
        debugPreviewJob = lifecycleScope.launch {
            delay(800)
            viewModel.applyDebugSettingsPreview(
                destination = destination ?: SettingsDestination.Root,
                updatePreview = updatePreview,
                installPermissionEnabledOverride = installPermissionEnabled,
            )
        }
    }

    private fun parseDebugSettingsDestination(intent: Intent?): SettingsDestination? {
        if (!BuildConfig.ENABLE_DEBUG_DEMO) {
            return null
        }
        val raw = intent?.getStringExtra(EXTRA_DEBUG_SETTINGS_DESTINATION)?.trim().orEmpty()
        if (raw.isBlank()) {
            return null
        }
        return when (raw.lowercase()) {
            "root" -> SettingsDestination.Root
            "service", "service_status", "service-status" -> SettingsDestination.Root
            "about" -> SettingsDestination.About
            "update", "app_update", "app-update" -> SettingsDestination.UpdateCheck
            "update_latest", "update-latest", "update_overview", "update-overview" -> SettingsDestination.UpdateLatest
            "current_version", "current-version", "current_version_notes", "current-version-notes" -> SettingsDestination.CurrentVersionNotes
            "update_notes", "update-notes" -> SettingsDestination.UpdateNotes
            else -> null
        }
    }

    companion object {
        const val EXTRA_DEBUG_SCENARIO = "openwatcher_debug_scenario"
        const val EXTRA_DEBUG_OPEN_SETTINGS = "openwatcher_debug_open_settings"
        const val EXTRA_DEBUG_SETTINGS_DESTINATION = "openwatcher_debug_settings_destination"
        const val EXTRA_DEBUG_UPDATE_PREVIEW = "openwatcher_debug_update_preview"
        const val EXTRA_DEBUG_INSTALL_PERMISSION_ENABLED = "openwatcher_debug_install_permission_enabled"
    }
}

private class AndroidWatcherHapticController(
    context: ComponentActivity,
) : WatcherHapticController {
    private val vibrator = context.getSystemService(VibratorManager::class.java).defaultVibrator

    override fun vibrateSessionCompleted() {
        vibrate(120L)
    }

    override fun vibrateScreenshotCaptured() {
        vibrate(45L)
    }

    private fun vibrate(durationMs: Long) {
        vibrator.vibrate(
            VibrationEffect.createOneShot(
                durationMs,
                VibrationEffect.DEFAULT_AMPLITUDE,
            ),
        )
    }
}
