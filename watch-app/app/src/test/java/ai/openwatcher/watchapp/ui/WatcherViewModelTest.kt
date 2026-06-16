package ai.openwatcher.watchapp.ui

import java.io.IOException
import java.nio.file.Files
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.Locale
import kotlin.collections.ArrayDeque
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.awaitCancellation
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.emptyFlow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import ai.openwatcher.watchapp.data.DeviceTokenRepository
import ai.openwatcher.watchapp.data.AppUpdateChannel
import ai.openwatcher.watchapp.data.AppUpdatePreferences
import ai.openwatcher.watchapp.data.AppUpdateVersionNotes
import ai.openwatcher.watchapp.data.AppUpdateNote
import ai.openwatcher.watchapp.data.SharedPreferencesAppUpdatePreferenceStore
import ai.openwatcher.watchapp.data.DailyTrend30dSnapshot
import ai.openwatcher.watchapp.data.DailyTrendDay
import ai.openwatcher.watchapp.data.DailyTrendPreferenceStore
import ai.openwatcher.watchapp.data.DailyUsageModelShare
import ai.openwatcher.watchapp.data.DailyUsageSnapshot
import ai.openwatcher.watchapp.data.DiagnosticUploadRequest
import ai.openwatcher.watchapp.data.DiagnosticUploadResult
import ai.openwatcher.watchapp.data.Heatmap24hSnapshot
import ai.openwatcher.watchapp.data.HeatmapBucket
import ai.openwatcher.watchapp.data.Heatmap7dDay
import ai.openwatcher.watchapp.data.Heatmap7dSnapshot
import ai.openwatcher.watchapp.data.HealthCheckResult
import ai.openwatcher.watchapp.data.EndpointHealthProbe
import ai.openwatcher.watchapp.data.EndpointSelector
import ai.openwatcher.watchapp.data.KeyValueStore
import ai.openwatcher.watchapp.data.PairingPreferenceStore
import ai.openwatcher.watchapp.data.QuotaStatus
import ai.openwatcher.watchapp.data.QuotaSnapshot
import ai.openwatcher.watchapp.data.QuotaWindow
import ai.openwatcher.watchapp.data.SessionAgentMessage
import ai.openwatcher.watchapp.data.SessionDetailsWindowCacheSnapshot
import ai.openwatcher.watchapp.data.SessionDetailsWindowPreferenceStore
import ai.openwatcher.watchapp.data.SessionRuntimeLifecycle
import ai.openwatcher.watchapp.data.SessionRuntimePhase
import ai.openwatcher.watchapp.data.SessionRuntimeState
import ai.openwatcher.watchapp.data.SessionStreamClientEventReport
import ai.openwatcher.watchapp.data.SessionStreamClientEventType
import ai.openwatcher.watchapp.data.SessionSnapshot
import ai.openwatcher.watchapp.data.SessionStreamEvent
import ai.openwatcher.watchapp.data.SessionStreamFailureReason
import ai.openwatcher.watchapp.data.SessionWindowEntry
import ai.openwatcher.watchapp.data.SessionWindowSnapshot
import ai.openwatcher.watchapp.data.SessionWindowStreamEvent
import ai.openwatcher.watchapp.data.ScreenshotUploadRequest
import ai.openwatcher.watchapp.data.PendingScreenshotUpload
import ai.openwatcher.watchapp.data.ScreenshotUploadQueue
import ai.openwatcher.watchapp.data.ScreenshotUploadResult
import ai.openwatcher.watchapp.data.ServerConfigRepository
import ai.openwatcher.watchapp.data.ServerConfigSource
import ai.openwatcher.watchapp.data.ServerEndpoint
import ai.openwatcher.watchapp.data.StatusFetchResult
import ai.openwatcher.watchapp.data.WatcherStatusSnapshotPreferenceStore
import ai.openwatcher.watchapp.data.StatusStreamEvent
import ai.openwatcher.watchapp.data.TokenGenerator
import ai.openwatcher.watchapp.data.WatchBootstrapCodeStore
import ai.openwatcher.watchapp.data.WatchBootstrapConfig
import ai.openwatcher.watchapp.data.WatchBootstrapException
import ai.openwatcher.watchapp.data.WatchBootstrapGateway
import ai.openwatcher.watchapp.data.WatchBootstrapPollResult
import ai.openwatcher.watchapp.data.WatchBootstrapRegistration
import ai.openwatcher.watchapp.data.WatcherApi
import ai.openwatcher.watchapp.data.WatcherApkInstallResult
import ai.openwatcher.watchapp.data.WatcherApkUpdate
import ai.openwatcher.watchapp.data.WatcherApkUpdateCheckResult
import ai.openwatcher.watchapp.data.WatcherApkUpdateManager
import ai.openwatcher.watchapp.data.WatcherApkUpdateProgress
import ai.openwatcher.watchapp.data.WatcherStatusSnapshot
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticAppInfo
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticDeviceInfo
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticEventLogger
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticEventStore
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticRuntimeContext
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticSnapshotCollector
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticUiStateHolder
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticUploadManager
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticUploadPreferenceStore
import ai.openwatcher.watchapp.data.diagnostics.NoOpDiagnosticEventLogger
import ai.openwatcher.watchapp.data.diagnostics.StructuredDiagnosticEventLogger
import ai.openwatcher.watchapp.ui.home.HomeQuotaEasterEggCopyConfig
import ai.openwatcher.watchapp.ui.home.HomeQuotaTipPool

@OptIn(ExperimentalCoroutinesApi::class)
class WatcherViewModelTest {
    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    @Test
    fun initialState_showsWaitingScanBeforeFirstPoll() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Unauthorized)
        val viewModel = newViewModel(api)

        assertEquals(AppScreen.Pairing, viewModel.uiState.value.screen)
        assertEquals("等待手机扫码", viewModel.uiState.value.pairing.statusLabel)
        viewModel.stopPolling()
    }

    @Test
    fun bootstrap_unpairedShowsPairingImmediately() = runTest(mainDispatcherRule.dispatcher) {
        val viewModel = newViewModel(
            api = FakeWatcherApi(StatusFetchResult.Unauthorized),
            enableBootstrap = true,
        )

        assertEquals(AppScreen.Pairing, viewModel.uiState.value.screen)
        assertEquals("等待手机扫码", viewModel.uiState.value.pairing.statusLabel)
    }

    @Test
    fun preloadedPairedState_opensDashboardFromCachedSnapshot() = runTest(mainDispatcherRule.dispatcher) {
        val snapshot = sampleSnapshot()
        val viewModel = newViewModel(
            api = FakeWatcherApi(StatusFetchResult.NetworkFailure("服务连接失败")),
            preloadedSnapshot = snapshot,
            preloadedPairingSuccess = true,
        )

        assertEquals(AppScreen.Dashboard, viewModel.uiState.value.screen)
        assertEquals("离线", viewModel.uiState.value.dashboard.serviceLabel)
        assertTrue(viewModel.uiState.value.dashboard.home.isServiceDegraded)
        assertEquals("GPT-5.5", viewModel.uiState.value.dashboard.home.selectedSessionModel)
    }

    @Test
    fun bootstrap_preloadedPairedState_keepsSplashThenOpensDashboard() = runTest(mainDispatcherRule.dispatcher) {
        val snapshot = sampleSnapshot()
        val viewModel = newViewModel(
            api = FakeWatcherApi(StatusFetchResult.NetworkFailure("服务连接失败")),
            preloadedSnapshot = snapshot,
            preloadedPairingSuccess = true,
            enableBootstrap = true,
        )

        assertEquals(AppScreen.Splash, viewModel.uiState.value.screen)

        advanceTimeBy(2_000L)
        runCurrent()

        assertEquals(AppScreen.Dashboard, viewModel.uiState.value.screen)
        assertEquals("离线", viewModel.uiState.value.dashboard.serviceLabel)
    }

    @Test
    fun remoteBootstrap_noStoredConfigRegistersCodeAndUsesBackoffDelays() = runTest(mainDispatcherRule.dispatcher) {
        val gateway = FakeWatchBootstrapGateway(
            registerResults = ArrayDeque(listOf(WatchBootstrapRegistration("ABCD2345", "2026-06-12T00:00:00Z"))),
        )
        val codeStore = WatchBootstrapCodeStore(FakeKeyValueStore())
        val viewModel = newViewModel(
            api = FakeWatcherApi(StatusFetchResult.Unauthorized),
            enableBootstrap = true,
            watchBootstrapClient = gateway,
            watchBootstrapCodeStore = codeStore,
        )

        assertEquals(AppScreen.Pairing, viewModel.uiState.value.screen)
        assertEquals("正在申请临时配置码", viewModel.uiState.value.pairing.statusLabel)
        assertEquals("", viewModel.uiState.value.pairing.qrPayload)
        assertEquals("", viewModel.uiState.value.pairing.bootstrapCode)

        runCurrent()

        assertEquals(AppScreen.Pairing, viewModel.uiState.value.screen)
        assertEquals("ABCD2345", viewModel.uiState.value.pairing.bootstrapCode)
        assertTrue(viewModel.uiState.value.pairing.bootstrapDetailLabel.contains("安装向导"))
        assertTrue(viewModel.uiState.value.pairing.bootstrapDetailLabel.contains("手表设备->远程初始化"))
        assertEquals("ABCD2345", codeStore.currentCode())
        assertEquals(1, gateway.registerCalls.size)
        assertEquals(1, gateway.pollCalls.size)

        advanceTimeBy(10_000L)
        runCurrent()
        assertEquals(11, gateway.pollCalls.size)

        advanceTimeBy(4_999L)
        runCurrent()
        assertEquals(11, gateway.pollCalls.size)

        advanceTimeBy(1L)
        runCurrent()
        assertEquals(12, gateway.pollCalls.size)
        viewModel.stopPolling()
    }

    @Test
    fun remoteBootstrap_readyConfigSavesApiBaseAndShowsPairingQr() = runTest(mainDispatcherRule.dispatcher) {
        val gateway = FakeWatchBootstrapGateway(
            registerResults = ArrayDeque(listOf(WatchBootstrapRegistration("ABCD2345", "2026-06-12T00:00:00Z"))),
            pollResults = ArrayDeque(
                listOf(
                    WatchBootstrapPollResult.Pending,
                    WatchBootstrapPollResult.Ready(
                        WatchBootstrapConfig(
                            environment = AppUpdateChannel.Dev,
                            apiBase = "https://dev.example.com/",
                            source = "desktop-remote-bootstrap",
                            configuredAt = "2026-06-12T00:00:00Z",
                        ),
                    ),
                ),
            ),
        )
        val codeStore = WatchBootstrapCodeStore(FakeKeyValueStore())
        val viewModel = newViewModel(
            api = FakeWatcherApi(StatusFetchResult.Unauthorized),
            enableBootstrap = true,
            watchBootstrapClient = gateway,
            watchBootstrapCodeStore = codeStore,
        )

        runCurrent()
        advanceTimeBy(1_000L)
        runCurrent()

        assertEquals(AppScreen.Pairing, viewModel.uiState.value.screen)
        assertEquals("等待手机扫码", viewModel.uiState.value.pairing.statusLabel)
        assertTrue(viewModel.uiState.value.pairing.hintLabel.contains("API 基址已保存"))
        assertTrue(viewModel.uiState.value.pairing.qrPayload.isNotBlank())
        assertTrue(viewModel.uiState.value.pairing.qrPayload.startsWith("https://dev.example.com/pair?"))
        assertEquals("", viewModel.uiState.value.pairing.bootstrapCode)
        assertNull(codeStore.currentCode())
        assertEquals("https://dev.example.com", viewModel.uiState.value.settings.baseUrl)
        assertEquals("远程 dev", viewModel.uiState.value.settings.activeEndpointLabel)
        viewModel.stopPolling()
    }

    @Test
    fun remoteBootstrap_renewsCodeWhenPollReportsInvalidOrExpiredCode() = runTest(mainDispatcherRule.dispatcher) {
        val gateway = FakeWatchBootstrapGateway(
            registerResults = ArrayDeque(
                listOf(
                    WatchBootstrapRegistration("ABCD2345", "2026-06-12T00:00:00Z"),
                    WatchBootstrapRegistration("WXYZ6789", "2026-06-12T00:00:01Z"),
                ),
            ),
            pollResults = ArrayDeque(listOf(WatchBootstrapException("bootstrap_code_expired", "临时配置码已失效"))),
        )
        val codeStore = WatchBootstrapCodeStore(FakeKeyValueStore())
        val viewModel = newViewModel(
            api = FakeWatcherApi(StatusFetchResult.Unauthorized),
            enableBootstrap = true,
            watchBootstrapClient = gateway,
            watchBootstrapCodeStore = codeStore,
        )

        runCurrent()

        assertEquals(listOf("ABCD2345", "WXYZ6789"), gateway.registerCalls.map { it.returnedCode })
        assertEquals("WXYZ6789", codeStore.currentCode())
        assertEquals("WXYZ6789", viewModel.uiState.value.pairing.bootstrapCode)
        viewModel.stopPolling()
    }

    @Test
    fun remoteBootstrap_registerFailureWaitsForExternallyStoredConfig() = runTest(mainDispatcherRule.dispatcher) {
        val store = FakeKeyValueStore()
        val externalRepository = ServerConfigRepository(
            store = store,
            fallbackBaseUrl = "https://127.0.0.1.invalid",
        )
        val gateway = FakeWatchBootstrapGateway(
            registerResults = ArrayDeque(listOf(IOException("网络不可用"))),
        )
        val viewModel = newViewModel(
            api = FakeWatcherApi(StatusFetchResult.Unauthorized),
            enableBootstrap = true,
            sharedStore = store,
            watchBootstrapClient = gateway,
            watchBootstrapCodeStore = WatchBootstrapCodeStore(store),
        )

        runCurrent()

        assertEquals(AppScreen.Pairing, viewModel.uiState.value.screen)
        assertEquals("临时配置不可用", viewModel.uiState.value.pairing.statusLabel)
        assertTrue(viewModel.uiState.value.pairing.bootstrapDetailLabel.contains("官方初始配置通道异常"))
        assertTrue(viewModel.uiState.value.pairing.bootstrapDetailLabel.contains("手表设备->远程初始化"))

        externalRepository.save(
            endpoints = listOf(ServerEndpoint("external", "外部入口", "https://external.example.com", 0)),
            source = ServerConfigSource.DesktopBootstrap,
        )
        advanceTimeBy(1_000L)
        runCurrent()

        assertEquals(AppScreen.Pairing, viewModel.uiState.value.screen)
        assertEquals("等待手机扫码", viewModel.uiState.value.pairing.statusLabel)
        assertTrue(viewModel.uiState.value.pairing.qrPayload.startsWith("https://external.example.com/pair?"))
        viewModel.stopPolling()
    }

    @Test
    fun remoteBootstrap_registerFailureShowsErrorWithoutSavingCode() = runTest(mainDispatcherRule.dispatcher) {
        val gateway = FakeWatchBootstrapGateway(
            registerResults = ArrayDeque(listOf(IOException("临时配置服务不可达"))),
        )
        val codeStore = WatchBootstrapCodeStore(FakeKeyValueStore())
        val viewModel = newViewModel(
            api = FakeWatcherApi(StatusFetchResult.Unauthorized),
            enableBootstrap = true,
            watchBootstrapClient = gateway,
            watchBootstrapCodeStore = codeStore,
        )

        runCurrent()

        assertEquals(AppScreen.Pairing, viewModel.uiState.value.screen)
        assertEquals("临时配置不可用", viewModel.uiState.value.pairing.statusLabel)
        assertTrue(viewModel.uiState.value.pairing.bootstrapDetailLabel.contains("官方初始配置通道异常"))
        assertTrue(viewModel.uiState.value.pairing.bootstrapDetailLabel.contains("安装向导"))
        assertTrue(viewModel.uiState.value.pairing.bootstrapDetailLabel.contains("请检查手表网络"))
        assertNull(codeStore.currentCode())
        assertEquals(0, gateway.pollCalls.size)
        viewModel.stopPolling()
    }

    @Test
    fun remoteBootstrap_existingStoredConfigDoesNotRegisterCode() = runTest(mainDispatcherRule.dispatcher) {
        val gateway = FakeWatchBootstrapGateway()
        val codeStore = WatchBootstrapCodeStore(FakeKeyValueStore())
        val viewModel = newViewModel(
            api = FakeWatcherApi(StatusFetchResult.Unauthorized),
            preloadedServerBaseUrl = "https://beta.example.com",
            enableBootstrap = true,
            watchBootstrapClient = gateway,
            watchBootstrapCodeStore = codeStore,
        )

        runCurrent()

        assertEquals(0, gateway.registerCalls.size)
        assertEquals("", viewModel.uiState.value.pairing.bootstrapCode)
        viewModel.stopPolling()
    }

    @Test
    fun unauthorizedResult_showsWaitingConfirm() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Unauthorized)
        val viewModel = newViewModel(api)

        viewModel.fetchForTesting(fromPairingLoop = true)

        assertEquals(AppScreen.Pairing, viewModel.uiState.value.screen)
        assertEquals("等待服务确认", viewModel.uiState.value.pairing.statusLabel)
        assertEquals("服务已连接", viewModel.uiState.value.pairing.serviceLabel)
    }

    @Test
    fun networkFailurePairingState_exposesServiceAddressWithoutBetaLabel() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.NetworkFailure("服务不可达"))
        val viewModel = newViewModel(
            api = api,
            preloadedServerBaseUrl = "http://192.168.1.22:8787",
        )

        viewModel.fetchForTesting(fromPairingLoop = true)

        val pairing = viewModel.uiState.value.pairing
        assertEquals(AppScreen.Pairing, viewModel.uiState.value.screen)
        assertEquals("服务不可达", pairing.statusLabel)
        assertEquals("http://192.168.1.22:8787", pairing.serviceBaseUrl)
        assertEquals("", pairing.environmentLabel)
    }

    @Test
    fun devNetworkFailurePairingState_exposesEnvironmentAndServiceAddress() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.NetworkFailure("服务不可达"))
        val viewModel = newViewModel(
            api = api,
            endpointProbe = EndpointHealthProbe { HealthCheckResult.Offline("服务不可达") },
        )

        viewModel.presentBootstrapRequest(
            sampleBootstrapRequest(
                channel = AppUpdateChannel.Dev,
                endpoints = listOf(
                    ServerEndpoint("dev-primary", "开发环境", "http://192.168.1.33:8787", 0),
                ),
            ),
        )
        runCurrent()

        val pairing = viewModel.uiState.value.pairing
        assertEquals(AppScreen.Pairing, viewModel.uiState.value.screen)
        assertEquals("服务不可达", pairing.statusLabel)
        assertEquals("环境：开发", pairing.environmentLabel)
        assertEquals("http://192.168.1.33:8787", pairing.serviceBaseUrl)
    }

    @Test
    fun successfulFetch_entersDashboard() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)

        viewModel.fetchForTesting(fromPairingLoop = true)

        assertEquals(AppScreen.Dashboard, viewModel.uiState.value.screen)
        assertEquals(DashboardPage.Home, viewModel.uiState.value.dashboard.pagerPage)
        assertEquals("在线", viewModel.uiState.value.dashboard.serviceLabel)
        assertEquals(1, viewModel.uiState.value.dashboard.sessionDetails.rows.size)
        assertEquals("GPT-5.5", viewModel.uiState.value.dashboard.home.selectedSessionModel)
        assertEquals("3h 2m", viewModel.uiState.value.dashboard.home.fiveHour.remainingLabel)
    }

    @Test
    fun successfulFetch_formatsHomeModelAndReasoningLabels() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(
            StatusFetchResult.Success(
                sampleSnapshot(
                    sessions = listOf(
                        sampleSession(
                            index = 1,
                            model = "gpt-5.5-mini",
                            reasoningEffort = "xhigh",
                            lastActiveAgoMinutes = 1,
                        ),
                    ),
                ),
            ),
        )
        val viewModel = newViewModel(api)

        viewModel.fetchForTesting(fromPairingLoop = true)

        assertEquals("GPT-5.5-Mini", viewModel.uiState.value.dashboard.home.selectedSessionModel)
        assertEquals("xhigh", viewModel.uiState.value.dashboard.home.selectedSessionReasoning)
    }

    @Test
    fun checkForAppUpdate_marksAvailableWhenNewerVersionExists() = runTest(mainDispatcherRule.dispatcher) {
        val updateManager = FakeWatcherApkUpdateManager(
            checkResult = WatcherApkUpdateCheckResult.Success(
                sampleUpdate(
                    releaseNotes = listOf(
                        sampleReleaseNote("2026-06-05 10:00", "支持新的手表端更新说明页"),
                    ),
                ),
            ),
        )
        val viewModel = newViewModel(FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())), updateManager = updateManager)

        viewModel.checkForAppUpdate()
        runCurrent()

        assertEquals(AppUpdateStatus.Available, viewModel.uiState.value.settings.update.status)
        assertEquals(SettingsDestination.UpdateNotes, viewModel.uiState.value.settings.destination)
        assertEquals("0.2.15 (17)", viewModel.uiState.value.settings.update.latestVersionLabel)
        assertEquals("当前 0.2.10 -> 最新 0.2.15", viewModel.uiState.value.settings.update.comparisonLabel)
        assertEquals(1, viewModel.uiState.value.settings.update.latestVersionNotes.notes.size)
    }

    @Test
    fun checkForAppUpdate_marksUpToDateWhenCurrentVersionMatches() = runTest(mainDispatcherRule.dispatcher) {
        val updateManager = FakeWatcherApkUpdateManager(
            checkResult = WatcherApkUpdateCheckResult.Success(
                WatcherApkUpdate(
                    versionName = "0.2.10",
                    versionCode = 16,
                    artifact = "openwatcher-watchapp-v0.2.10-release.apk",
                    sha256 = "abc123",
                    commit = "samever",
                    builtAt = "2026-06-05T02:00:00Z",
                    downloadUrl = "https://127.0.0.1.invalid/file/latest-apk",
                ),
            ),
        )
        val viewModel = newViewModel(FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())), updateManager = updateManager)

        viewModel.checkForAppUpdate()
        runCurrent()

        assertEquals(AppUpdateStatus.UpToDate, viewModel.uiState.value.settings.update.status)
        assertEquals("当前已是所选通道最新版本", viewModel.uiState.value.settings.update.detailLabel)
        assertEquals(SettingsDestination.UpdateLatest, viewModel.uiState.value.settings.destination)
    }

    @Test
    fun openAbout_doesNotStartUpdateCheck() = runTest(mainDispatcherRule.dispatcher) {
        val updateManager = FakeWatcherApkUpdateManager(
            checkResult = WatcherApkUpdateCheckResult.Success(sampleUpdate()),
        )
        val viewModel = newViewModel(FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())), updateManager = updateManager)

        viewModel.openSettingsDestination(SettingsDestination.About)
        runCurrent()

        assertEquals(SettingsDestination.About, viewModel.uiState.value.settings.destination)
        assertEquals(AppUpdateStatus.Idle, viewModel.uiState.value.settings.update.status)
        assertEquals("未检查更新", viewModel.uiState.value.settings.update.detailLabel)
        assertNull(viewModel.uiState.value.settings.update.latestVersionLabel)
    }

    @Test
    fun openSettings_autoRunsHealthCheckWhenEnteringStandaloneSettings() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        api.nextHealthCheck = HealthCheckResult.Online
        val viewModel = newViewModel(api)

        viewModel.openSettings()
        runCurrent()

        assertEquals(1, api.healthCheckCount)
        assertEquals(AppScreen.Settings, viewModel.uiState.value.screen)
        assertEquals(ServiceHealthStatus.Online, viewModel.uiState.value.settings.healthCheck.status)
        assertEquals("服务在线", viewModel.uiState.value.settings.healthCheck.resultLabel)
    }

    @Test
    fun setDashboardPage_toSettingsRunsHealthCheckOnlyOncePerEntry() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        api.nextHealthCheck = HealthCheckResult.Online
        val viewModel = newViewModel(
            api,
            preloadedPairingSuccess = true,
            preloadedSnapshot = sampleSnapshot(),
        )

        assertEquals(AppScreen.Dashboard, viewModel.uiState.value.screen)
        assertEquals(DashboardPage.Home, viewModel.uiState.value.dashboard.pagerPage)

        viewModel.setDashboardPage(DashboardPage.Settings)
        runCurrent()

        assertEquals(1, api.healthCheckCount)
        assertEquals(DashboardPage.Settings, viewModel.uiState.value.dashboard.pagerPage)

        viewModel.openSettingsDestination(SettingsDestination.About)
        runCurrent()
        viewModel.navigateBackFromSettings()
        runCurrent()

        assertEquals(SettingsDestination.Root, viewModel.uiState.value.settings.destination)
        assertEquals(1, api.healthCheckCount)
    }

    @Test
    fun openAppUpdateFromAbout_withoutInstallPermission_staysOnAboutAndShowsNotice() = runTest(mainDispatcherRule.dispatcher) {
        val updateManager = FakeWatcherApkUpdateManager(
            checkResult = WatcherApkUpdateCheckResult.Success(sampleUpdate()),
            installPermissionEnabled = false,
        )
        val viewModel = newViewModel(FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())), updateManager = updateManager)

        viewModel.openSettingsDestination(SettingsDestination.About)
        runCurrent()
        viewModel.openAppUpdateFromAbout()
        runCurrent()

        assertEquals(SettingsDestination.About, viewModel.uiState.value.settings.destination)
        assertEquals(AppUpdateStatus.Idle, viewModel.uiState.value.settings.update.status)
        assertTrue(viewModel.uiState.value.screenshotUpload.visible)
        assertEquals("请允许安装未知来源才可更新应用", viewModel.uiState.value.screenshotUpload.message)
    }

    @Test
    fun navigateBackFromCurrentVersionNotes_returnsThroughUpdateLatestAndAbout() = runTest(mainDispatcherRule.dispatcher) {
        val updateManager = FakeWatcherApkUpdateManager(
            checkResult = WatcherApkUpdateCheckResult.Success(
                WatcherApkUpdate(
                    versionName = "0.2.10",
                    versionCode = 16,
                    artifact = "openwatcher-watchapp-v0.2.10-release.apk",
                    sha256 = "abc123",
                    commit = "samever",
                    builtAt = "2026-06-05T02:00:00Z",
                    downloadUrl = "https://127.0.0.1.invalid/file/latest-apk",
                ),
            ),
        )
        val viewModel = newViewModel(FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())), updateManager = updateManager)

        viewModel.openSettingsDestination(SettingsDestination.About)
        runCurrent()
        viewModel.checkForAppUpdate()
        runCurrent()
        viewModel.openSettingsDestination(SettingsDestination.CurrentVersionNotes)
        runCurrent()

        assertEquals(SettingsDestination.CurrentVersionNotes, viewModel.uiState.value.settings.destination)

        viewModel.navigateBackFromSettings()
        assertEquals(SettingsDestination.UpdateLatest, viewModel.uiState.value.settings.destination)

        viewModel.navigateBackFromSettings()
        assertEquals(SettingsDestination.About, viewModel.uiState.value.settings.destination)

        viewModel.navigateBackFromSettings()
        assertEquals(SettingsDestination.Root, viewModel.uiState.value.settings.destination)
    }

    @Test
    fun navigateBackFromUpdateNotes_returnsToAbout() = runTest(mainDispatcherRule.dispatcher) {
        val updateManager = FakeWatcherApkUpdateManager(
            checkResult = WatcherApkUpdateCheckResult.Success(sampleUpdate()),
        )
        val viewModel = newViewModel(FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())), updateManager = updateManager)

        viewModel.checkForAppUpdate()
        runCurrent()

        assertEquals(SettingsDestination.UpdateNotes, viewModel.uiState.value.settings.destination)

        viewModel.navigateBackFromSettings()
        assertEquals(SettingsDestination.About, viewModel.uiState.value.settings.destination)
    }

    @Test
    fun checkForAppUpdate_cachedUpdateAutomaticallyStartsInstaller() = runTest(mainDispatcherRule.dispatcher) {
        val update = sampleUpdate()
        val updateManager = FakeWatcherApkUpdateManager(
            checkResult = WatcherApkUpdateCheckResult.Success(update),
            cachedUpdateAvailable = true,
            cachedInstallResult = WatcherApkInstallResult.Started,
        )
        val viewModel = newViewModel(FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())), updateManager = updateManager)

        viewModel.checkForAppUpdate()
        runCurrent()

        assertEquals(1, updateManager.cachedInstallRequests)
        assertEquals(AppUpdateStatus.ReadyToInstall, viewModel.uiState.value.settings.update.status)
        assertEquals("已打开系统安装器", viewModel.uiState.value.settings.update.detailLabel)
        assertEquals(SettingsDestination.UpdateNotes, viewModel.uiState.value.settings.destination)
    }

    @Test
    fun downloadAndInstallAppUpdate_keepsReadyToInstallAfterInstallerOpens() = runTest(mainDispatcherRule.dispatcher) {
        val update = WatcherApkUpdate(
            versionName = "0.2.15",
            versionCode = 17,
            artifact = "openwatcher-watchapp-v0.2.15-release.apk",
            sha256 = "abc123",
            commit = "deadbee",
            builtAt = "2026-06-05T02:00:00Z",
            downloadUrl = "https://127.0.0.1.invalid/file/latest-apk",
        )
        val updateManager = FakeWatcherApkUpdateManager(
            checkResult = WatcherApkUpdateCheckResult.Success(update),
            installResult = WatcherApkInstallResult.Started,
        )
        val viewModel = newViewModel(FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())), updateManager = updateManager)

        viewModel.checkForAppUpdate()
        runCurrent()
        viewModel.downloadAndInstallAppUpdate()
        runCurrent()

        assertEquals(1, updateManager.downloadRequests)
        assertEquals(AppUpdateStatus.ReadyToInstall, viewModel.uiState.value.settings.update.status)
        assertEquals("已打开系统安装器", viewModel.uiState.value.settings.update.detailLabel)
    }

    @Test
    fun downloadAndInstallAppUpdate_marksPermissionRequiredAfterDownload() = runTest(mainDispatcherRule.dispatcher) {
        val update = WatcherApkUpdate(
            versionName = "0.2.15",
            versionCode = 17,
            artifact = "openwatcher-watchapp-v0.2.15-release.apk",
            sha256 = "abc123",
            commit = "deadbee",
            builtAt = "2026-06-05T02:00:00Z",
            downloadUrl = "https://127.0.0.1.invalid/file/latest-apk",
        )
        val updateManager = FakeWatcherApkUpdateManager(
            checkResult = WatcherApkUpdateCheckResult.Success(update),
            installResult = WatcherApkInstallResult.PermissionRequired(
                message = "需要允许安装未知来源更新",
                settingsOpened = true,
            ),
            downloadProgress = listOf(
                WatcherApkUpdateProgress.Downloading(bytesDownloaded = 50, totalBytes = 100),
                WatcherApkUpdateProgress.PreparingInstall,
            ),
        )
        val viewModel = newViewModel(FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())), updateManager = updateManager)

        viewModel.checkForAppUpdate()
        runCurrent()
        viewModel.downloadAndInstallAppUpdate()
        runCurrent()

        assertEquals(AppUpdateStatus.PermissionRequired, viewModel.uiState.value.settings.update.status)
        assertEquals("需要允许安装未知来源更新", viewModel.uiState.value.settings.update.detailLabel)
        assertEquals(100, viewModel.uiState.value.settings.update.progressPercent)
    }

    @Test
    fun downloadAndInstallAppUpdate_updatesDownloadSpeedAndSizeLabels() = runTest(mainDispatcherRule.dispatcher) {
        val update = sampleUpdate()
        val releaseResult = CompletableDeferred<WatcherApkInstallResult>()
        val updateManager = object : WatcherApkUpdateManager {
            override fun canRequestPackageInstalls(): Boolean = false
            override fun openInstallPermissionSettings(): Boolean = false
            override suspend fun fetchLatestUpdate(): WatcherApkUpdateCheckResult =
                WatcherApkUpdateCheckResult.Success(update)
            override suspend fun hasCachedUpdate(update: WatcherApkUpdate): Boolean = false
            override suspend fun startInstallFromCache(update: WatcherApkUpdate): WatcherApkInstallResult =
                WatcherApkInstallResult.Failure("unused")

            override suspend fun downloadAndStartInstall(
                update: WatcherApkUpdate,
                onProgress: (WatcherApkUpdateProgress) -> Unit,
            ): WatcherApkInstallResult {
                onProgress(
                    WatcherApkUpdateProgress.Downloading(
                        bytesDownloaded = 1024,
                        totalBytes = 4096,
                        speedBytesPerSecond = 2048,
                    ),
                )
                return releaseResult.await()
            }
        }
        val viewModel = newViewModel(
            FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())),
            updateManager = updateManager,
        )

        viewModel.checkForAppUpdate()
        runCurrent()
        viewModel.downloadAndInstallAppUpdate()
        runCurrent()

        val updateUi = viewModel.uiState.value.settings.update
        assertEquals(AppUpdateStatus.Downloading, updateUi.status)
        assertEquals(25, updateUi.progressPercent)
        assertEquals("1 KB / 4 KB", updateUi.progressDetailLabel)
        assertEquals("2 KB/s", updateUi.downloadSpeedLabel)
        assertTrue(updateUi.downloadOverlay.visible)
        assertEquals("4 KB", updateUi.downloadOverlay.fileSizeLabel)
        assertEquals("25%", updateUi.downloadOverlay.progressLabel)
        assertEquals("1 KB / 4 KB", updateUi.downloadOverlay.transferredLabel)
        assertEquals("2 KB/s", updateUi.downloadOverlay.speedLabel)

        releaseResult.complete(WatcherApkInstallResult.Failure("下载失败"))
        runCurrent()

        val failedUi = viewModel.uiState.value.settings.update
        assertEquals(AppUpdateStatus.Available, failedUi.status)
        assertFalse(failedUi.downloadOverlay.visible)
        assertEquals("发现新版本", failedUi.detailLabel)
        assertTrue(viewModel.uiState.value.screenshotUpload.visible)
        assertEquals("下载失败", viewModel.uiState.value.screenshotUpload.message)
    }

    @Test
    fun checkForAppUpdate_distinguishesTimeoutFailure() = runTest(mainDispatcherRule.dispatcher) {
        val viewModel = newViewModel(
            FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())),
            updateManager = FakeWatcherApkUpdateManager(
                checkResult = WatcherApkUpdateCheckResult.Failure("访问超时"),
            ),
        )

        viewModel.checkForAppUpdate()
        runCurrent()

        assertEquals(SettingsDestination.UpdateCheck, viewModel.uiState.value.settings.destination)
        assertEquals(AppUpdateStatus.Failed, viewModel.uiState.value.settings.update.status)
        assertEquals("访问超时", viewModel.uiState.value.settings.update.detailLabel)
    }

    @Test
    fun checkForAppUpdate_distinguishesNetworkFailure() = runTest(mainDispatcherRule.dispatcher) {
        val viewModel = newViewModel(
            FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())),
            updateManager = FakeWatcherApkUpdateManager(
                checkResult = WatcherApkUpdateCheckResult.Failure("网络不可用"),
            ),
        )

        viewModel.checkForAppUpdate()
        runCurrent()

        assertEquals(SettingsDestination.UpdateCheck, viewModel.uiState.value.settings.destination)
        assertEquals(AppUpdateStatus.Failed, viewModel.uiState.value.settings.update.status)
        assertEquals("网络不可用", viewModel.uiState.value.settings.update.detailLabel)
    }

    @Test
    fun bootstrap_autoCheckIgnoredUpdateSkipsUpdateNotes() = runTest(mainDispatcherRule.dispatcher) {
        val updateManager = FakeWatcherApkUpdateManager(
            checkResult = WatcherApkUpdateCheckResult.Success(sampleUpdate()),
        )
        val viewModel = newViewModel(
            api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())),
            updateManager = updateManager,
            preloadedPairingSuccess = true,
            preloadedSnapshot = sampleSnapshot(),
            preloadedAppUpdatePreferences = AppUpdatePreferences(
                autoCheckEnabled = true,
                ignoredVersionCodes = setOf(17),
            ),
            enableBootstrap = true,
        )

        advanceTimeBy(2_000L)
        runCurrent()

        assertEquals(AppScreen.Dashboard, viewModel.uiState.value.screen)
        assertEquals(DashboardPage.Home, viewModel.uiState.value.dashboard.pagerPage)
        assertEquals(SettingsDestination.Root, viewModel.uiState.value.settings.destination)
    }

    @Test
    fun init_promotesPendingInstalledVersionNotesAfterEnteringNewVersion() = runTest(mainDispatcherRule.dispatcher) {
        val viewModel = newViewModel(
            api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())),
            preloadedAppUpdatePreferences = AppUpdatePreferences(
                pendingInstalledVersionNotes = AppUpdateVersionNotes(
                    versionName = "0.2.10",
                    versionCode = 16,
                    notes = listOf(
                        AppUpdateNote(
                            publishedAtLabel = "2026-06-05 10:00",
                            summary = "新的应用更新页和下载浮层",
                        ),
                    ),
                ),
            ),
        )

        viewModel.openSettingsDestination(SettingsDestination.CurrentVersionNotes)
        runCurrent()

        val notes = viewModel.uiState.value.settings.update.currentVersionNotes
        assertEquals("0.2.10 (16) · beta", notes.versionLabel)
        assertEquals(1, notes.notes.size)
        assertEquals("新的应用更新页和下载浮层", notes.notes.single().summary)
    }

    @Test
    fun secretVersionTap_togglesUpdateChannelAfterFiveTaps() = runTest(mainDispatcherRule.dispatcher) {
        val viewModel = newViewModel(
            api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())),
            preloadedEndpoints = listOf(
                ServerEndpoint("lan", "局域网", "http://192.168.1.12:8787", 0),
            ),
            preloadedAppUpdatePreferences = AppUpdatePreferences(
                selectedChannel = AppUpdateChannel.Beta,
            ),
        )
        viewModel.presentBootstrapRequest(
            sampleBootstrapRequest(
                channel = AppUpdateChannel.Dev,
                endpoints = listOf(
                    ServerEndpoint("dev-primary", "开发环境", "http://192.168.1.33:8787", 0),
                ),
            ),
        )
        runCurrent()
        viewModel.registerSecretAppUpdateChannelTap()
        viewModel.registerSecretAppUpdateChannelTap()
        viewModel.registerSecretAppUpdateChannelTap()
        viewModel.registerSecretAppUpdateChannelTap()
        assertEquals("dev", viewModel.uiState.value.settings.update.channelLabel)
        assertEquals("0.2.10 (16) · dev", viewModel.uiState.value.settings.update.currentVersionNotes.versionLabel)

        repeat(5) {
            viewModel.registerSecretAppUpdateChannelTap()
        }
        runCurrent()

        assertEquals("beta", viewModel.uiState.value.settings.update.channelLabel)
    }

    @Test
    fun openInstallPermissionSettings_updatesDetailAndOpensSettings() = runTest(mainDispatcherRule.dispatcher) {
        val updateManager = FakeWatcherApkUpdateManager(installPermissionEnabled = false)
        val viewModel = newViewModel(FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())), updateManager = updateManager)

        viewModel.openInstallPermissionSettings()
        runCurrent()

        assertTrue(updateManager.permissionSettingsOpened)
        assertEquals("已打开系统设置", viewModel.uiState.value.settings.update.detailLabel)
        assertFalse(viewModel.uiState.value.settings.update.installPermissionEnabled)
    }

    @Test
    fun setForegroundVisible_refreshesInstallPermissionState() = runTest(mainDispatcherRule.dispatcher) {
        val updateManager = FakeWatcherApkUpdateManager(installPermissionEnabled = false)
        val viewModel = newViewModel(FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())), updateManager = updateManager)

        viewModel.setForegroundVisible(false)
        updateManager.installPermissionEnabled = true
        viewModel.setForegroundVisible(true)
        runCurrent()

        assertTrue(viewModel.uiState.value.settings.update.installPermissionEnabled)
        assertEquals("已允许", viewModel.uiState.value.settings.update.installPermissionLabel)
    }

    @Test
    fun setForegroundVisible_whenPermissionReturns_marksReadyToInstall() = runTest(mainDispatcherRule.dispatcher) {
        val update = WatcherApkUpdate(
            versionName = "0.2.15",
            versionCode = 17,
            artifact = "openwatcher-watchapp-v0.2.15-release.apk",
            sha256 = "abc123",
            commit = "deadbee",
            builtAt = "2026-06-05T02:00:00Z",
            downloadUrl = "https://127.0.0.1.invalid/file/latest-apk",
        )
        val updateManager = FakeWatcherApkUpdateManager(
            checkResult = WatcherApkUpdateCheckResult.Success(update),
            installPermissionEnabled = false,
            installResult = WatcherApkInstallResult.PermissionRequired(
                message = "需要允许安装未知来源更新",
                settingsOpened = true,
            ),
            downloadProgress = listOf(WatcherApkUpdateProgress.PreparingInstall),
        )
        val viewModel = newViewModel(FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())), updateManager = updateManager)

        viewModel.checkForAppUpdate()
        runCurrent()
        viewModel.downloadAndInstallAppUpdate()
        runCurrent()

        viewModel.setForegroundVisible(false)
        updateManager.installPermissionEnabled = true
        viewModel.setForegroundVisible(true)
        runCurrent()

        assertEquals(AppUpdateStatus.ReadyToInstall, viewModel.uiState.value.settings.update.status)
        assertEquals("已允许安装未知来源，可继续安装", viewModel.uiState.value.settings.update.detailLabel)
    }

    @Test
    fun homeQuotaEasterEgg_classifiesAllPoolsAcrossScenarios() = runTest(mainDispatcherRule.dispatcher) {
        val viewModel = newViewModel(FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())))
        val cases = listOf(
            QuotaPoolCase("too_fast", HomeQuotaTipPool.TooFast, snapshotForWeeklyDelta(delta = -24f)),
            QuotaPoolCase("fast", HomeQuotaTipPool.Fast, snapshotForWeeklyDelta(delta = -10f)),
            QuotaPoolCase("balanced", HomeQuotaTipPool.Balanced, snapshotForWeeklyDelta(delta = 0f)),
            QuotaPoolCase("slow", HomeQuotaTipPool.Slow, snapshotForWeeklyDelta(delta = 12f)),
            QuotaPoolCase("too_slow", HomeQuotaTipPool.TooSlow, snapshotForWeeklyDelta(delta = 26f)),
        )

        cases.forEach { case ->
            assertEquals(case.name, case.expectedPool, viewModel.classifyHomeQuotaTipPool(case.snapshot))
        }
    }

    @Test
    fun homeQuotaEasterEgg_classifiesBoundaryDeltas() = runTest(mainDispatcherRule.dispatcher) {
        val viewModel = newViewModel(FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot())))
        val cases = listOf(
            QuotaPoolCase("minus_18", HomeQuotaTipPool.TooFast, snapshotForWeeklyDelta(delta = -18f)),
            QuotaPoolCase("minus_17_9", HomeQuotaTipPool.Fast, snapshotForWeeklyDelta(delta = -17.9f)),
            QuotaPoolCase("minus_6", HomeQuotaTipPool.Fast, snapshotForWeeklyDelta(delta = -6f)),
            QuotaPoolCase("minus_5_9", HomeQuotaTipPool.Balanced, snapshotForWeeklyDelta(delta = -5.9f)),
            QuotaPoolCase("plus_5_9", HomeQuotaTipPool.Balanced, snapshotForWeeklyDelta(delta = 5.9f)),
            QuotaPoolCase("plus_6", HomeQuotaTipPool.Slow, snapshotForWeeklyDelta(delta = 6f)),
            QuotaPoolCase("plus_18", HomeQuotaTipPool.TooSlow, snapshotForWeeklyDelta(delta = 18f)),
        )

        cases.forEach { case ->
            assertEquals(case.name, case.expectedPool, viewModel.classifyHomeQuotaTipPool(case.snapshot))
        }
    }

    @Test
    fun showHomeQuotaEasterEgg_showsTipFromMatchedPool() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(snapshotForWeeklyDelta(delta = 0f)))
        val viewModel = newViewModel(
            api = api,
            randomIndexPicker = { 0 },
        )
        viewModel.fetchForTesting(fromPairingLoop = true)

        viewModel.showHomeQuotaEasterEgg()
        runCurrent()

        val tipState = viewModel.uiState.value.dashboard.home.quotaEasterEgg
        assertTrue(tipState.visible)
        assertEquals(HomeQuotaTipPool.Balanced, tipState.pool)
        assertEquals(
            HomeQuotaEasterEggCopyConfig.entriesFor(HomeQuotaTipPool.Balanced).first(),
            tipState.text,
        )
    }

    @Test
    fun showHomeQuotaEasterEgg_replacesTipWithoutRepeatingAndResetsHideTimer() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(snapshotForWeeklyDelta(delta = 0f)))
        val viewModel = newViewModel(
            api = api,
            randomIndexPicker = { 0 },
            homeQuotaTipHideMs = 2_000L,
        )
        viewModel.fetchForTesting(fromPairingLoop = true)

        viewModel.showHomeQuotaEasterEgg()
        runCurrent()
        val firstTip = viewModel.uiState.value.dashboard.home.quotaEasterEgg.text

        advanceTimeBy(1_500L)
        runCurrent()

        viewModel.showHomeQuotaEasterEgg()
        runCurrent()
        val secondState = viewModel.uiState.value.dashboard.home.quotaEasterEgg
        assertTrue(secondState.visible)
        assertNotEquals(firstTip, secondState.text)

        advanceTimeBy(1_999L)
        runCurrent()
        assertTrue(viewModel.uiState.value.dashboard.home.quotaEasterEgg.visible)

        advanceTimeBy(1L)
        runCurrent()
        assertFalse(viewModel.uiState.value.dashboard.home.quotaEasterEgg.visible)
        assertEquals(null, viewModel.uiState.value.dashboard.home.quotaEasterEgg.text)
    }

    @Test
    fun showHomeQuotaEasterEgg_ignoresMissingOrInvalidWeeklyData() = runTest(mainDispatcherRule.dispatcher) {
        val missingWeeklyViewModel = newViewModel(
            FakeWatcherApi(
                StatusFetchResult.Success(
                    sampleSnapshot(weekly = null),
                ),
            ),
        )
        missingWeeklyViewModel.fetchForTesting(fromPairingLoop = true)
        missingWeeklyViewModel.showHomeQuotaEasterEgg()
        runCurrent()
        assertFalse(missingWeeklyViewModel.uiState.value.dashboard.home.quotaEasterEgg.visible)

        val observedAt = Instant.parse("2026-06-03T03:30:00Z")
        val invalidResetViewModel = newViewModel(
            FakeWatcherApi(
                StatusFetchResult.Success(
                    sampleSnapshot(
                        observedAt = observedAt,
                        weekly = QuotaWindow(
                            usedPercent = 20f,
                            remainingPercent = 80f,
                            resetAt = observedAt.minusSeconds(60),
                        ),
                    ),
                ),
            ),
        )
        invalidResetViewModel.fetchForTesting(fromPairingLoop = true)
        invalidResetViewModel.showHomeQuotaEasterEgg()
        runCurrent()
        assertFalse(invalidResetViewModel.uiState.value.dashboard.home.quotaEasterEgg.visible)

        val missingObservedAtViewModel = newViewModel(
            FakeWatcherApi(
                StatusFetchResult.Success(
                    sampleSnapshot(
                        observedAt = null,
                    ),
                ),
            ),
        )
        missingObservedAtViewModel.fetchForTesting(fromPairingLoop = true)
        missingObservedAtViewModel.showHomeQuotaEasterEgg()
        runCurrent()
        assertFalse(missingObservedAtViewModel.uiState.value.dashboard.home.quotaEasterEgg.visible)
    }

    @Test
    fun successfulFetch_computesTimeRingPercentagesForQuotaWindows() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)

        viewModel.fetchForTesting(fromPairingLoop = true)

        assertEquals(60.67f, viewModel.uiState.value.dashboard.home.fiveHour.timeRemainingPercent ?: -1f, 0.01f)
        assertEquals(64.58f, viewModel.uiState.value.dashboard.home.weekly.timeRemainingPercent ?: -1f, 0.01f)
    }

    @Test
    fun refreshNow_marksRefreshingBeforeResult() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)

        val deferred = CompletableDeferred<StatusFetchResult>()
        api.next = SuspendedStatus(deferred)

        viewModel.refreshNow()
        runCurrent()

        assertEquals("刷新中", viewModel.uiState.value.dashboard.serviceLabel)

        deferred.complete(StatusFetchResult.Success(sampleSnapshot()))
        runCurrent()

        assertEquals("在线", viewModel.uiState.value.dashboard.serviceLabel)
    }

    @Test
    fun captureAndUploadScreenshot_uploadsPngWithDeviceMetadata() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val haptic = FakeHapticController()
        val viewModel = newViewModel(
            api = api,
            hapticController = haptic,
            preloadedPairingSuccess = true,
            preloadedSnapshot = sampleSnapshot(),
        )
        val pngBytes = byteArrayOf(0x01, 0x02, 0x03)

        viewModel.captureAndUploadScreenshot { pngBytes }
        runCurrent()

        assertEquals(1, api.screenshotUploads.size)
        val upload = api.screenshotUploads.single()
        assertEquals("test-token-0123456789abcdef0123456789", upload.token)
        assertTrue(upload.request.pngBytes.contentEquals(pngBytes))
        assertEquals("Xiaomi Watch 5", upload.request.deviceName)
        assertEquals("0.2.10", upload.request.appVersion)
        assertEquals(1, haptic.screenshotCount)
        assertEquals("截图已上传", viewModel.uiState.value.screenshotUpload.message)
        assertFalse(viewModel.uiState.value.screenshotUpload.inProgress)

        advanceTimeBy(1_800L)
        runCurrent()
        assertFalse(viewModel.uiState.value.screenshotUpload.visible)
    }

    @Test
    fun captureAndUploadScreenshot_beforePairingDoesNotCaptureOrUpload() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Unauthorized)
        val viewModel = newViewModel(api)
        var captured = false

        viewModel.captureAndUploadScreenshot {
            captured = true
            byteArrayOf(0x01)
        }
        runCurrent()

        assertFalse(captured)
        assertTrue(api.screenshotUploads.isEmpty())
        assertEquals("未配对，无法上传截图", viewModel.uiState.value.screenshotUpload.message)
    }

    @Test
    fun captureAndUploadScreenshot_networkFailurePersistsScreenshotAndRetriesAfterSuccess() =
        runTest(mainDispatcherRule.dispatcher) {
            val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
            val queue = InMemoryScreenshotUploadQueue()
            val viewModel = newViewModel(
                api = api,
                screenshotUploadQueue = queue,
                preloadedPairingSuccess = true,
                preloadedSnapshot = sampleSnapshot(),
            )
            val pngBytes = byteArrayOf(0x0A, 0x0B)
            api.nextScreenshotUpload = ScreenshotUploadResult.NetworkFailure("服务连接失败")

            viewModel.captureAndUploadScreenshot { pngBytes }
            runCurrent()

            assertEquals(1, api.screenshotUploads.size)
            assertEquals(1, queue.pending().size)
            assertEquals("网络离线，截图已暂存", viewModel.uiState.value.screenshotUpload.message)

            api.nextScreenshotUpload = ScreenshotUploadResult.Success("retried.png")
            viewModel.fetchForTesting(fromPairingLoop = false)
            runCurrent()

            assertEquals(2, api.screenshotUploads.size)
            assertTrue(api.screenshotUploads[1].request.pngBytes.contentEquals(pngBytes))
            assertTrue(queue.pending().isEmpty())
            assertEquals("暂存截图已补传", viewModel.uiState.value.screenshotUpload.message)
        }

    @Test
    fun captureAndUploadScreenshot_withPendingQueueUploadsOldestFirst() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val queue = InMemoryScreenshotUploadQueue()
        val viewModel = newViewModel(
            api = api,
            screenshotUploadQueue = queue,
            preloadedPairingSuccess = true,
            preloadedSnapshot = sampleSnapshot(),
        )
        val oldPng = byteArrayOf(0x01)
        val newPng = byteArrayOf(0x02)
        queue.enqueue(oldPng, Instant.parse("2000-01-01T00:00:00Z"))

        viewModel.captureAndUploadScreenshot { newPng }
        runCurrent()

        assertEquals(2, api.screenshotUploads.size)
        assertTrue(api.screenshotUploads[0].request.pngBytes.contentEquals(oldPng))
        assertTrue(api.screenshotUploads[1].request.pngBytes.contentEquals(newPng))
        assertTrue(queue.pending().isEmpty())
    }

    @Test
    fun uploadDiagnostics_successUpdatesServiceStatusUi() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val uiStateHolder = DiagnosticUiStateHolder()
        val runtimeContext = DiagnosticRuntimeContext(baseUrl = "https://127.0.0.1.invalid")
        val eventStore = newDiagnosticEventStore()
        val logger = newDiagnosticEventLogger(uiStateHolder, runtimeContext, eventStore)
        val manager = newDiagnosticUploadManager(api, uiStateHolder, eventStore, logger)
        val viewModel = newViewModel(
            api = api,
            preloadedPairingSuccess = true,
            preloadedSnapshot = sampleSnapshot(),
            diagnosticEventLogger = logger,
            diagnosticUploadManager = manager,
            diagnosticUiStateSink = uiStateHolder::update,
            diagnosticRuntimeContext = runtimeContext,
        )
        try {
            viewModel.uploadDiagnostics()
            runCurrent()
            manager.awaitIdleForTesting()
            runCurrent()

            val diagnostic = viewModel.uiState.value.settings.diagnosticUpload
            assertEquals("上传完成", diagnostic.statusLabel)
            assertEquals("diag-test", diagnostic.lastDiagnosticId)
            assertEquals("上传诊断信息", diagnostic.actionLabel)
            assertFalse(diagnostic.hasPendingPackage)
            assertTrue(diagnostic.entrySubtitle.startsWith("最近诊断："))
            assertTrue(diagnostic.entrySubtitle.endsWith("已上传"))
            assertEquals(1, api.diagnosticUploads.size)
        } finally {
            manager.close()
        }
    }

    @Test
    fun uploadDiagnostics_failureShowsRetryAndKeepsPendingPackage() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        api.nextDiagnosticUpload = DiagnosticUploadResult.NetworkFailure("服务连接失败")
        val uiStateHolder = DiagnosticUiStateHolder()
        val runtimeContext = DiagnosticRuntimeContext(baseUrl = "https://127.0.0.1.invalid")
        val eventStore = newDiagnosticEventStore()
        val logger = newDiagnosticEventLogger(uiStateHolder, runtimeContext, eventStore)
        val manager = newDiagnosticUploadManager(api, uiStateHolder, eventStore, logger)
        val viewModel = newViewModel(
            api = api,
            preloadedPairingSuccess = true,
            preloadedSnapshot = sampleSnapshot(),
            diagnosticEventLogger = logger,
            diagnosticUploadManager = manager,
            diagnosticUiStateSink = uiStateHolder::update,
            diagnosticRuntimeContext = runtimeContext,
        )
        try {
            viewModel.uploadDiagnostics()
            runCurrent()
            manager.awaitIdleForTesting()
            runCurrent()

            val diagnostic = viewModel.uiState.value.settings.diagnosticUpload
            assertEquals("上传失败，可重试", diagnostic.statusLabel)
            assertEquals("重新尝试上传诊断", diagnostic.actionLabel)
            assertTrue(diagnostic.hasPendingPackage)
            assertTrue(diagnostic.entrySubtitle.startsWith("最近诊断："))
            assertTrue(diagnostic.entrySubtitle.endsWith("未上传"))
            assertTrue((manager.state.value.packageSizeBytes ?: 0L) > 0L)
        } finally {
            manager.close()
        }
    }

    @Test
    fun diagnosticEntryClick_withPendingPackageShowsUploadAndClearPrompt() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        api.nextDiagnosticUpload = DiagnosticUploadResult.NetworkFailure("服务连接失败")
        val uiStateHolder = DiagnosticUiStateHolder()
        val runtimeContext = DiagnosticRuntimeContext(baseUrl = "https://127.0.0.1.invalid")
        val eventStore = newDiagnosticEventStore()
        val logger = newDiagnosticEventLogger(uiStateHolder, runtimeContext, eventStore)
        val manager = newDiagnosticUploadManager(api, uiStateHolder, eventStore, logger)
        val viewModel = newViewModel(
            api = api,
            preloadedPairingSuccess = true,
            preloadedSnapshot = sampleSnapshot(),
            diagnosticEventLogger = logger,
            diagnosticUploadManager = manager,
            diagnosticUiStateSink = uiStateHolder::update,
            diagnosticRuntimeContext = runtimeContext,
        )
        try {
            viewModel.uploadDiagnostics()
            runCurrent()
            manager.awaitIdleForTesting()
            runCurrent()

            viewModel.onDiagnosticEntryClick()
            runCurrent()

            val prompt = viewModel.uiState.value.settings.diagnosticPrompt
            assertTrue(prompt.visible)
            assertEquals("上传", prompt.primaryLabel)
            assertEquals("清除", prompt.secondaryLabel)
            assertEquals(1, api.diagnosticUploads.size)
        } finally {
            manager.close()
        }
    }

    @Test
    fun diagnosticPromptSecondaryAction_clearsPendingPackage() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        api.nextDiagnosticUpload = DiagnosticUploadResult.NetworkFailure("服务连接失败")
        val uiStateHolder = DiagnosticUiStateHolder()
        val runtimeContext = DiagnosticRuntimeContext(baseUrl = "https://127.0.0.1.invalid")
        val eventStore = newDiagnosticEventStore()
        val logger = newDiagnosticEventLogger(uiStateHolder, runtimeContext, eventStore)
        val manager = newDiagnosticUploadManager(api, uiStateHolder, eventStore, logger)
        val viewModel = newViewModel(
            api = api,
            preloadedPairingSuccess = true,
            preloadedSnapshot = sampleSnapshot(),
            diagnosticEventLogger = logger,
            diagnosticUploadManager = manager,
            diagnosticUiStateSink = uiStateHolder::update,
            diagnosticRuntimeContext = runtimeContext,
        )
        try {
            viewModel.uploadDiagnostics()
            runCurrent()
            manager.awaitIdleForTesting()
            runCurrent()

            viewModel.onDiagnosticEntryClick()
            runCurrent()
            viewModel.confirmDiagnosticPromptSecondary()
            runCurrent()

            val diagnostic = viewModel.uiState.value.settings.diagnosticUpload
            assertFalse(diagnostic.hasPendingPackage)
            assertEquals("最近诊断：暂无", diagnostic.entrySubtitle)
            assertFalse(viewModel.uiState.value.settings.diagnosticPrompt.visible)
        } finally {
            manager.close()
        }
    }

    @Test
    fun diagnosticEntryClick_afterSuccessfulUploadWaitsForConfirmation() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val uiStateHolder = DiagnosticUiStateHolder()
        val runtimeContext = DiagnosticRuntimeContext(baseUrl = "https://127.0.0.1.invalid")
        val eventStore = newDiagnosticEventStore()
        val logger = newDiagnosticEventLogger(uiStateHolder, runtimeContext, eventStore)
        val manager = newDiagnosticUploadManager(api, uiStateHolder, eventStore, logger)
        val viewModel = newViewModel(
            api = api,
            preloadedPairingSuccess = true,
            preloadedSnapshot = sampleSnapshot(),
            diagnosticEventLogger = logger,
            diagnosticUploadManager = manager,
            diagnosticUiStateSink = uiStateHolder::update,
            diagnosticRuntimeContext = runtimeContext,
        )
        try {
            viewModel.uploadDiagnostics()
            runCurrent()
            manager.awaitIdleForTesting()
            runCurrent()
            assertEquals(1, api.diagnosticUploads.size)

            viewModel.onDiagnosticEntryClick()
            runCurrent()
            assertEquals(1, api.diagnosticUploads.size)
            assertEquals("确认上传", viewModel.uiState.value.settings.diagnosticPrompt.primaryLabel)
            assertNull(viewModel.uiState.value.settings.diagnosticPrompt.secondaryLabel)

            viewModel.confirmDiagnosticPromptPrimary()
            runCurrent()
            manager.awaitIdleForTesting()
            runCurrent()

            assertEquals(2, api.diagnosticUploads.size)
            assertFalse(viewModel.uiState.value.settings.diagnosticPrompt.visible)
        } finally {
            manager.close()
        }
    }

    @Test
    fun dashboardAfterPairing_usesStatusStreamInsteadOfStatusPolling() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api, autoPolling = true)

        viewModel.fetchForTesting(fromPairingLoop = true)
        runCurrent()
        val fetchesAfterPairing = api.fetchStatusCount

        advanceTimeBy(120_000)
        runCurrent()

        assertEquals(fetchesAfterPairing, api.fetchStatusCount)
        assertEquals(1, api.statusStreamRequests)
        assertEquals(listOf(true), api.statusStreamTrendRequests)
        viewModel.setForegroundVisible(false)
        runCurrent()
        viewModel.stopPolling()
        runCurrent()
    }

    @Test
    fun dashboardAfterPairing_skipsDailyTrendRequestWhenYesterdayAlreadyCached() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(
            api,
            autoPolling = true,
            preloadedDailyTrend30d = sampleDailyTrend30d(endDate = "2026-06-02"),
        )

        viewModel.fetchForTesting(fromPairingLoop = true)
        runCurrent()

        assertEquals(1, api.statusStreamRequests)
        assertEquals(listOf(false), api.statusStreamTrendRequests)
        viewModel.setForegroundVisible(false)
        runCurrent()
        viewModel.stopPolling()
        runCurrent()
    }

    @Test
    fun statusStreamSectionUpdate_replacesCachedSessions() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api, autoPolling = true)
        viewModel.fetchForTesting(fromPairingLoop = true)
        runCurrent()

        val nextSessions = sampleSnapshot().sessions.mapIndexed { index, session ->
            if (index == 0) session.copy(title = "SSE 更新会话") else session
        }
        api.emitStatusStream(
            StatusStreamEvent.Sessions(
                observedAt = Instant.parse("2026-06-03T03:31:00Z"),
                sessions = nextSessions,
            ),
        )
        runCurrent()

        assertEquals("SSE 更新会话", viewModel.uiState.value.dashboard.home.selectedSessionTitle)
        viewModel.setForegroundVisible(false)
        runCurrent()
        viewModel.stopPolling()
        runCurrent()
    }

    @Test
    fun heatmapWithZeroInputAndCachedTokens_keepsZeroCacheHitRate() = runTest(mainDispatcherRule.dispatcher) {
        val zeroBuckets = List(24) { index ->
            HeatmapBucket(
                hourStart = Instant.parse("2026-06-03T00:00:00Z").plusSeconds(index * 3600L),
                inputTokens = 0,
                cachedInputTokens = 0,
                outputTokens = 0,
                reasoningOutputTokens = 0,
                totalTokens = 0,
                activeThreads = 0,
            )
        }
        val api = FakeWatcherApi(
            StatusFetchResult.Success(
                sampleSnapshot(
                    heatmap24h = Heatmap24hSnapshot(
                        timezone = "Asia/Shanghai",
                        generatedAt = Instant.parse("2026-06-03T00:00:00Z"),
                        peakHourStart = null,
                        buckets = zeroBuckets,
                    ),
                ),
            ),
        )
        val viewModel = newViewModel(api)

        viewModel.fetchForTesting(fromPairingLoop = true)

        assertEquals("0%", viewModel.uiState.value.dashboard.heatmap24h.cacheHitRateLabel)
        assertTrue(viewModel.uiState.value.dashboard.heatmap24h.segments.all { !it.isNonEmpty })
    }

    @Test
    fun heatmapCacheHitRate_usesCachedInputAsSubsetOfInputTokens() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)

        viewModel.fetchForTesting(fromPairingLoop = true)

        val heatmap = viewModel.uiState.value.dashboard.heatmap24h
        assertEquals("30%", heatmap.cacheHitRateLabel)
        assertEquals("30%", heatmap.segments.first().cacheHitRateLabel)
    }

    @Test
    fun homeWeeklyHeatmap_showsTodayFirstAndOldestLast() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot(heatmap7d = sampleHeatmap7d())))
        val viewModel = newViewModel(api)

        viewModel.fetchForTesting(fromPairingLoop = true)

        val heatmap = viewModel.uiState.value.dashboard.home.weeklyHeatmap
        assertTrue(heatmap.available)
        assertEquals("06.03", heatmap.rows[0].dateLabel)
        assertEquals("06.02", heatmap.rows[1].dateLabel)
        assertEquals("05.28", heatmap.rows[6].dateLabel)
        assertEquals(7_000L, heatmap.rows[0].cells[6].totalTokens)
        assertEquals(1_000L, heatmap.rows[6].cells[0].totalTokens)
    }

    @Test
    fun dailyUsageFromStatus_buildsTodaySummaryState() = runTest(mainDispatcherRule.dispatcher) {
        val dailyUsage = DailyUsageSnapshot(
            generatedAt = Instant.parse("2026-06-03T03:30:00Z"),
            totalTokens = 200_000,
            inputTokens = 120_000,
            cachedInputTokens = 40_000,
            outputTokens = 50_000,
            reasoningOutputTokens = 30_000,
            activeSessions = 3,
            estimatedValueUsd = 1.2345,
            estimatedValueLabel = "\$1.23",
            pricingDate = "2026-06-03",
            pricingSourceUrl = "https://developers.openai.com/api/docs/pricing",
            pricingUnavailableReason = null,
            modelShares = listOf(
                DailyUsageModelShare(model = "gpt-5.4", tokens = 140_000, sharePercent = 70.0),
                DailyUsageModelShare(model = "gpt-5-mini", tokens = 60_000, sharePercent = 30.0),
            ),
        )
        val viewModel = newViewModel(
            FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot(dailyUsage = dailyUsage))),
        )

        viewModel.fetchForTesting(fromPairingLoop = true)

        val state = viewModel.uiState.value.dashboard.heatmap24h.dailyUsage
        assertEquals("200K", state.totalTokensLabel)
        assertEquals("120K", state.inputLabel)
        assertEquals("40K", state.cachedInputLabel)
        assertEquals("33%", state.cacheHitRateLabel)
        assertEquals("80K", state.outputLabel)
        assertEquals("3 会话", state.activeSessionsLabel)
        assertEquals("\$1.23", state.estimatedValueLabel)
        assertEquals("今日价值", state.valueCaption)
        assertEquals(4, state.segments.size)
        assertEquals("gpt-5.4", state.modelShares.first().model)
        assertEquals("70%", state.modelShares.first().shareLabel)
    }

    @Test
    fun dailyUsageTokenLabels_useLargestCompactUnitWithOneDecimal() = runTest(mainDispatcherRule.dispatcher) {
        val dailyUsage = DailyUsageSnapshot(
            generatedAt = Instant.parse("2026-06-03T03:30:00Z"),
            totalTokens = 50_000_000_000,
            inputTokens = 50_000_000_000,
            cachedInputTokens = 47_500_000_000,
            outputTokens = 1_000_000_000,
            reasoningOutputTokens = 0,
            activeSessions = 1,
            estimatedValueUsd = null,
            estimatedValueLabel = null,
            pricingDate = "",
            pricingSourceUrl = "",
            pricingUnavailableReason = null,
            modelShares = emptyList(),
        )
        val viewModel = newViewModel(
            FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot(dailyUsage = dailyUsage))),
        )

        viewModel.fetchForTesting(fromPairingLoop = true)

        val state = viewModel.uiState.value.dashboard.heatmap24h.dailyUsage
        assertEquals("50B", state.totalTokensLabel)
        assertEquals("50B", state.inputLabel)
        assertEquals("47.5B", state.cachedInputLabel)
        assertEquals("95%", state.cacheHitRateLabel)
        assertEquals("1B", state.outputLabel)
    }

    @Test
    fun unauthorizedAfterPaired_movesBackToTokenError() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)

        api.next = ImmediateStatus(StatusFetchResult.Unauthorized)
        viewModel.fetchForTesting(fromPairingLoop = false)

        assertEquals(AppScreen.Pairing, viewModel.uiState.value.screen)
        assertEquals("token 错误", viewModel.uiState.value.pairing.statusLabel)
    }

    @Test
    fun networkAndParseFailures_mapToVisibleStates() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)

        api.next = ImmediateStatus(StatusFetchResult.NetworkFailure("服务连接失败"))
        viewModel.fetchForTesting(fromPairingLoop = false)
        assertEquals(AppScreen.Dashboard, viewModel.uiState.value.screen)
        assertEquals("离线", viewModel.uiState.value.dashboard.serviceLabel)

        api.next = ImmediateStatus(StatusFetchResult.ParseFailure("返回数据无法解析"))
        viewModel.fetchForTesting(fromPairingLoop = false)
        assertEquals(AppScreen.Dashboard, viewModel.uiState.value.screen)
        assertEquals("解析失败", viewModel.uiState.value.dashboard.serviceLabel)
        assertTrue(viewModel.uiState.value.dashboard.home.isServiceDegraded)
    }

    @Test
    fun networkFailureWhileViewingHeatmap_keepsCurrentScreen() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot(heatmap24h = sampleHeatmap24h())))
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)
        viewModel.openHeatmap()

        api.next = ImmediateStatus(StatusFetchResult.NetworkFailure("服务连接失败"))
        viewModel.fetchForTesting(fromPairingLoop = false)

        assertEquals(AppScreen.Heatmap24h, viewModel.uiState.value.screen)
        assertTrue(viewModel.uiState.value.dashboard.heatmap24h.isServiceDegraded)
    }

    @Test
    fun quotaStale_onlyDimsQuotaRings() = runTest(mainDispatcherRule.dispatcher) {
        val snapshot = sampleSnapshot().copy(
            quota = sampleSnapshot().quota?.copy(
                fresh = false,
                status = QuotaStatus.Stale,
            ),
        )
        val viewModel = newViewModel(FakeWatcherApi(StatusFetchResult.Success(snapshot)))

        viewModel.fetchForTesting(fromPairingLoop = true)

        assertTrue(viewModel.uiState.value.dashboard.home.fiveHour.isDimmed)
        assertTrue(viewModel.uiState.value.dashboard.home.weekly.isDimmed)
        assertFalse(viewModel.uiState.value.dashboard.home.isServiceDegraded)
        viewModel.openHeatmap()
        assertEquals(AppScreen.Heatmap24h, viewModel.uiState.value.screen)
        assertFalse(viewModel.uiState.value.dashboard.heatmap24h.isServiceDegraded)
    }

    @Test
    fun openSettings_beforePaired_usesStandaloneScreen() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Unauthorized)
        val viewModel = newViewModel(api)

        viewModel.openSettings()

        assertEquals(AppScreen.Settings, viewModel.uiState.value.screen)
    }

    @Test
    fun openSettings_afterPaired_switchesDashboardPager() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)

        viewModel.openSettings()

        assertEquals(AppScreen.Dashboard, viewModel.uiState.value.screen)
        assertEquals(DashboardPage.Settings, viewModel.uiState.value.dashboard.pagerPage)
    }

    @Test
    fun openDetails_andClose_returnsToDashboardHome() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)
        viewModel.setDashboardPage(DashboardPage.Settings)

        viewModel.openHeatmap()
        assertEquals(AppScreen.Heatmap24h, viewModel.uiState.value.screen)

        viewModel.closeDetailScreen()
        assertEquals(AppScreen.Dashboard, viewModel.uiState.value.screen)
        assertEquals(DashboardPage.Home, viewModel.uiState.value.dashboard.pagerPage)

        viewModel.openSessionDetails()
        assertEquals(AppScreen.SessionDetails, viewModel.uiState.value.screen)
    }

    @Test
    fun heatmapRotaryDeltaToCursorDelta_usesThreeToOneGearRatio() {
        assertEquals(8.0f, heatmapRotaryDeltaToCursorDelta(360f), 0.0001f)
        assertEquals(-8.0f, heatmapRotaryDeltaToCursorDelta(-360f), 0.0001f)
        assertEquals(1.0f, heatmapRotaryDeltaToCursorDelta(45f), 0.0001f)
        assertEquals(4.0f / 3.0f, heatmapRotaryDeltaToCursorDelta(1f), 0.0001f)
        assertEquals(8.0f, List(6) { heatmapRotaryDeltaToCursorDelta(1f) }.sum(), 0.0001f)
    }

    @Test
    fun heatmapTrendRotaryDeltaToDegrees_matchesHourRingGearRatio() {
        assertEquals(50.0f, heatmapTrendRotaryDeltaToDegrees(1f), 0.0001f)
        assertEquals(24.0f, heatmapTrendRotaryDeltaToDegrees(0.48f), 0.0001f)
        assertEquals(-24.0f, heatmapTrendRotaryDeltaToDegrees(-0.48f), 0.0001f)
    }

    @Test
    fun heatmapCursorRounding_wrapsAroundTwentyFourHourRing() {
        assertEquals(0, roundHeatmapCursorPosition(0.49f, 24))
        assertEquals(1, roundHeatmapCursorPosition(0.5f, 24))
        assertEquals(23, roundHeatmapCursorPosition(22.6f, 24))
        assertEquals(0, roundHeatmapCursorPosition(23.6f, 24))
    }

    @Test
    fun rotateHeatmapCursor_movesContinuouslyThenSettlesToNearestHour() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(
            StatusFetchResult.Success(
                sampleSnapshot(heatmap24h = sampleHeatmap24h()),
            ),
        )
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)
        viewModel.selectHeatmapHour(0)

        viewModel.rotateHeatmapCursor(1f)
        runCurrent()
        assertEquals(4.0f / 3.0f, viewModel.uiState.value.dashboard.heatmap24h.selectionCursorIndex ?: -1f, 0.0001f)
        assertEquals(0, viewModel.uiState.value.dashboard.heatmap24h.selectedIndex)

        advanceTimeBy(500L)
        runCurrent()
        assertEquals(1, viewModel.uiState.value.dashboard.heatmap24h.selectedIndex)

        viewModel.rotateHeatmapCursor(1f)
        runCurrent()
        assertEquals(1, viewModel.uiState.value.dashboard.heatmap24h.selectedIndex)

        advanceTimeBy(500L)
        runCurrent()
        assertEquals(2, viewModel.uiState.value.dashboard.heatmap24h.selectedIndex)
    }

    @Test
    fun selectHeatmapTrendDay_capturesRotaryAndShowsTip() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(
            StatusFetchResult.Success(
                sampleSnapshot(
                    heatmap24h = sampleHeatmap24h(),
                    dailyTrend30d = sampleDailyTrend30d(),
                ),
            ),
        )
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)
        viewModel.openHeatmap()

        viewModel.selectHeatmapTrendDay(2)
        runCurrent()

        val state = viewModel.uiState.value.dashboard.heatmap24h
        assertEquals(HeatmapRotaryMode.Trend30d, state.rotaryMode)
        assertEquals(2, state.dailyUsage.dailyTrend30d.selectedIndex)
        assertEquals("5月6日", state.dailyUsage.dailyTrend30d.selectedDateLabel)
        assertEquals("3K", state.dailyUsage.dailyTrend30d.selectedTokenLabel)
        assertTrue(state.dailyUsage.dailyTrend30d.tipVisible)

        advanceTimeBy(3_000L)
        runCurrent()
        assertFalse(viewModel.uiState.value.dashboard.heatmap24h.dailyUsage.dailyTrend30d.tipVisible)
        assertEquals(HeatmapRotaryMode.Trend30d, viewModel.uiState.value.dashboard.heatmap24h.rotaryMode)
    }

    @Test
    fun rotateHeatmapCursor_whenTrendFocused_movesTrendSelectionAndResetsTip() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(
            StatusFetchResult.Success(
                sampleSnapshot(
                    heatmap24h = sampleHeatmap24h(),
                    dailyTrend30d = sampleDailyTrend30d(),
                ),
            ),
        )
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)
        viewModel.openHeatmap()
        viewModel.selectHeatmapTrendDay(1)
        runCurrent()

        advanceTimeBy(2_500L)
        runCurrent()
        viewModel.rotateHeatmapCursor(0.47f)
        runCurrent()

        assertEquals(1, viewModel.uiState.value.dashboard.heatmap24h.dailyUsage.dailyTrend30d.selectedIndex)
        assertTrue(viewModel.uiState.value.dashboard.heatmap24h.dailyUsage.dailyTrend30d.tipVisible)

        viewModel.rotateHeatmapCursor(0.01f)
        runCurrent()

        val state = viewModel.uiState.value.dashboard.heatmap24h
        assertEquals(HeatmapRotaryMode.Trend30d, state.rotaryMode)
        assertEquals(2, state.dailyUsage.dailyTrend30d.selectedIndex)
        assertEquals("5月6日", state.dailyUsage.dailyTrend30d.selectedDateLabel)
        assertTrue(state.dailyUsage.dailyTrend30d.tipVisible)

        advanceTimeBy(2_999L)
        runCurrent()
        assertTrue(viewModel.uiState.value.dashboard.heatmap24h.dailyUsage.dailyTrend30d.tipVisible)

        advanceTimeBy(1L)
        runCurrent()
        assertFalse(viewModel.uiState.value.dashboard.heatmap24h.dailyUsage.dailyTrend30d.tipVisible)
    }

    @Test
    fun selectHeatmapHour_clearsTrendRotaryMode() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(
            StatusFetchResult.Success(
                sampleSnapshot(
                    heatmap24h = sampleHeatmap24h(),
                    dailyTrend30d = sampleDailyTrend30d(),
                ),
            ),
        )
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)
        viewModel.openHeatmap()
        viewModel.selectHeatmapTrendDay(4)
        runCurrent()

        viewModel.selectHeatmapHour(0)
        runCurrent()

        val state = viewModel.uiState.value.dashboard.heatmap24h
        assertEquals(HeatmapRotaryMode.HourRing, state.rotaryMode)
        assertEquals(-1, state.dailyUsage.dailyTrend30d.selectedIndex)
        assertFalse(state.dailyUsage.dailyTrend30d.tipVisible)
    }

    @Test
    fun successfulFetch_formatsFullContextWindowLabel() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot(contextUsedTokens = 35_000, contextWindow = 920_000)))
        val viewModel = newViewModel(api)

        viewModel.fetchForTesting(fromPairingLoop = true)

        assertEquals("35K/920K", viewModel.uiState.value.dashboard.home.selectedSessionContextLabel)
        assertEquals("35K/920K", viewModel.uiState.value.dashboard.sessionDetails.selectedSessionContextLabel)
    }

    @Test
    fun successfulFetch_exposesCompactThresholdWarningOnHomeAndDetails() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(
            StatusFetchResult.Success(
                sampleSnapshot(
                    contextUsedTokens = 183_000,
                    contextWindow = 256_000,
                    contextPressurePercent = 72,
                    contextCompactThresholdTokens = 192_000,
                    contextCompactThresholdPercent = 75,
                ),
            ),
        )
        val viewModel = newViewModel(api)

        viewModel.fetchForTesting(fromPairingLoop = true)

        val home = viewModel.uiState.value.dashboard.home
        assertEquals(75f, home.selectedSessionCompactThresholdPercent)
        assertTrue(home.selectedSessionCompactWarning)

        viewModel.openSessionDetails()
        val details = viewModel.uiState.value.dashboard.sessionDetails
        assertEquals(75f, details.selectedSessionCompactThresholdPercent)
        assertTrue(details.selectedSessionCompactWarning)
        assertEquals(75f, details.rows.first().contextCompactThresholdPercent)
        assertTrue(details.rows.first().contextCompactWarning)
    }

    @Test
    fun successfulFetch_formatsRecentActivityWithSingleCompactUnit() = runTest(mainDispatcherRule.dispatcher) {
        val sessions = listOf(
            sampleSession(index = 1, title = "One Minute", lastActiveAgoMinutes = 1),
            sampleSession(index = 2, title = "Max Minute", lastActiveAgoMinutes = 59),
            sampleSession(index = 3, title = "One Hour", lastActiveAgoMinutes = 60),
            sampleSession(index = 4, title = "Max Hour", lastActiveAgoMinutes = 1_439),
            sampleSession(index = 5, title = "One Day", lastActiveAgoMinutes = 1_440),
            sampleSession(index = 6, title = "Max Day", lastActiveAgoMinutes = 10_079),
            sampleSession(index = 7, title = "One Week", lastActiveAgoMinutes = 10_080),
            sampleSession(index = 8, title = "Four Weeks", lastActiveAgoMinutes = 40_320),
            sampleSession(index = 9, title = "Beyond Month", lastActiveAgoMinutes = 60_000),
        )
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot(sessions = sessions)))
        val viewModel = newViewModel(api)

        viewModel.fetchForTesting(fromPairingLoop = true)

        val labelsByTitle = viewModel.uiState.value.dashboard.sessionDetails.rows
            .associate { it.title to it.lastActiveLabel }
        assertEquals("1m", labelsByTitle["One Minute"])
        assertEquals("59m", labelsByTitle["Max Minute"])
        assertEquals("1h", labelsByTitle["One Hour"])
        assertEquals("23h", labelsByTitle["Max Hour"])
        assertEquals("1d", labelsByTitle["One Day"])
        assertEquals("6d", labelsByTitle["Max Day"])
        assertEquals("1w", labelsByTitle["One Week"])
        assertEquals("4w", labelsByTitle["Four Weeks"])
        assertEquals("4w", labelsByTitle["Beyond Month"])
    }

    @Test
    fun sessionDetails_usesBackendSessionsSortedByRecentActivity() = runTest(mainDispatcherRule.dispatcher) {
        val sessions = listOf(
            sampleSession(index = 1, title = "AnyChat", lastActiveAgoMinutes = 7),
            sampleSession(index = 2, title = "Android Build", lastActiveAgoMinutes = 3),
            sampleSession(index = 3, title = "UI Refactor", lastActiveAgoMinutes = 5),
            sampleSession(index = 4, title = "Docs Sync", lastActiveAgoMinutes = 12),
            sampleSession(
                index = 5,
                title = "Long Context Strategy Review For Round Watch",
                model = "gpt-5.5-mini",
                reasoningEffort = "xhigh",
                tokensUsedTotal = 12_345_000,
                lastActiveAgoMinutes = 28,
            ),
            sampleSession(index = 6, title = "Sixth Recent", lastActiveAgoMinutes = 29),
            sampleSession(index = 7, title = "Outside Window", lastActiveAgoMinutes = 31),
        )
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot(sessions = sessions)))
        val viewModel = newViewModel(api)

        viewModel.fetchForTesting(fromPairingLoop = true)

        val details = viewModel.uiState.value.dashboard.sessionDetails
        assertEquals(7, details.rows.size)
        assertEquals("Android Build", details.rows.first().title)
        assertEquals("Outside Window", details.rows.last().title)

        viewModel.selectSession(4)

        assertEquals(
            "Long Context Strategy Review For Round Watch",
            viewModel.uiState.value.dashboard.sessionDetails.selectedSessionTitle,
        )
        assertEquals("GPT-5.5-Mini", viewModel.uiState.value.dashboard.sessionDetails.selectedSessionModel)
        assertEquals("xhigh", viewModel.uiState.value.dashboard.sessionDetails.selectedSessionReasoning)
    }

    @Test
    fun sessionDetails_keepsBackendSessionsOutsideHalfHourWindow() = runTest(mainDispatcherRule.dispatcher) {
        val sessions = listOf(
            sampleSession(index = 1, title = "Stale Long Run", lastActiveAgoMinutes = 143_999),
            sampleSession(index = 2, title = "Nearest Old Session", lastActiveAgoMinutes = 31),
        )
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot(sessions = sessions)))
        val viewModel = newViewModel(api)

        viewModel.fetchForTesting(fromPairingLoop = true)

        val details = viewModel.uiState.value.dashboard.sessionDetails
        assertEquals(2, details.rows.size)
        assertEquals("Nearest Old Session", details.selectedSessionTitle)
        assertEquals("31m", details.selectedSessionActiveLabel)
        assertEquals("Stale Long Run", details.rows.last().title)
    }

    @Test
    fun openSessionDetails_startsSessionWindowStream() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)
        runCurrent()

        viewModel.openSessionDetails()
        runCurrent()

        assertEquals(listOf("session-1"), api.streamRequests)
        assertEquals(listOf(false), api.streamIncludeMessagesRequests)
        assertEquals(listOf(5), api.windowStreamLimits)
        assertEquals(listOf(emptyList<String>()), api.windowStreamPreferredOrders)
        assertEquals(1, api.activeStreams)
        assertEquals(
            AgentMessageStreamStatus.Connecting,
            viewModel.uiState.value.dashboard.sessionDetails.agentMessageStreamStatus,
        )
    }

    @Test
    fun openSessionDetails_restoresPersistedWindowWhileShowingConnecting() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val persistedWindow = SessionDetailsWindowCacheSnapshot(
            selectedThreadId = "session-2",
            selectedSlotIndex = 1,
            window = sampleWindowSnapshot(
                entries = listOf(
                    sampleWindowEntry(
                        session = sampleSession(index = 1, title = "First", lastActiveAgoMinutes = 1),
                        runtimeState = sampleRuntimeState(threadId = "session-1", running = true, sequence = 1).state,
                        latestMessage = sampleAgentMessage("session-1", "第一页缓存"),
                    ),
                    sampleWindowEntry(
                        session = sampleSession(index = 2, title = "Second", lastActiveAgoMinutes = 2),
                        runtimeState = sampleRuntimeState(threadId = "session-2", running = false, sequence = 1).state,
                        latestMessage = sampleAgentMessage("session-2", "第二页缓存"),
                    ),
                ),
            ),
        )
        val viewModel = newViewModel(
            api = api,
            preloadedSessionDetailsWindow = persistedWindow,
        )
        viewModel.fetchForTesting(fromPairingLoop = true)
        runCurrent()

        viewModel.openSessionDetails()
        runCurrent()

        val details = viewModel.uiState.value.dashboard.sessionDetails
        assertEquals("Second", details.selectedSessionTitle)
        assertEquals("第二页缓存", details.latestAgentMessage)
        assertEquals(2, details.rows.size)
        assertEquals(AgentMessageStreamStatus.Connecting, details.agentMessageStreamStatus)

        viewModel.setForegroundVisible(false)
        runCurrent()
        viewModel.stopPolling()
        runCurrent()
    }

    @Test
    fun dashboardHome_startsRuntimeOnlyStreamForSelectedSession() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)

        viewModel.fetchForTesting(fromPairingLoop = true)
        runCurrent()

        assertEquals(AppScreen.Dashboard, viewModel.uiState.value.screen)
        assertEquals(listOf("session-1"), api.streamRequests)
        assertEquals(listOf(false), api.streamIncludeMessagesRequests)
        assertEquals(1, api.activeStreams)
    }

    @Test
    fun runtimeState_updatesHomeAndDetailsRunningLabels() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(
            api,
            wallClockNow = { Instant.parse("2026-06-03T02:35:01Z") },
        )
        try {
            viewModel.fetchForTesting(fromPairingLoop = true)
            runCurrent()
            runCurrent()
            runCurrent()
            runCurrent()

            assertEquals(listOf("session-1"), api.streamRequests)
            api.emitStream("session-1", sampleRuntimeState(running = true, phase = SessionRuntimePhase.ToolRunning))
            runCurrent()
            runCurrent()
            runCurrent()

            assertTrue(viewModel.uiState.value.dashboard.home.selectedSessionIsActiveNow)
            assertEquals("1s", viewModel.uiState.value.dashboard.home.selectedSessionActiveLabel)

            viewModel.openSessionDetails()
            runCurrent()
            api.emitWindowStream(
                SessionWindowStreamEvent.Window(
                    sampleWindowSnapshot(
                        entries = listOf(
                            sampleWindowEntry(
                                session = sampleSession(index = 1, lastActiveAgoMinutes = 1),
                                runtimeState = sampleRuntimeState(
                                    threadId = "session-1",
                                    running = true,
                                    phase = SessionRuntimePhase.ToolRunning,
                                    sequence = 2,
                                ).state,
                                latestMessage = sampleAgentMessage("session-1", "详情页最新消息"),
                            ),
                        ),
                    ),
                ),
            )
            runCurrent()

            val details = viewModel.uiState.value.dashboard.sessionDetails
            assertTrue(details.selectedSessionIsActiveNow)
            val runtimeTimeLabel = formatTestTime(Instant.parse("2026-06-03T02:35:01Z"))
            assertEquals("运行中 · $runtimeTimeLabel · 工具调用中 · 1s", details.selectedSessionRuntimePhaseLabel)
            assertEquals("运行中 · $runtimeTimeLabel · 工具调用中 · 1s", details.rows.first().runtimePhaseLabel)
            assertEquals("$runtimeTimeLabel · 工具调用中 · 1s", details.rows.first().agentStatusLine)
        } finally {
            viewModel.setForegroundVisible(false)
            runCurrent()
            viewModel.stopPolling()
            runCurrent()
        }
    }

    @Test
    fun sessionDetailsStatusLine_countsBySecondAndKeepsCompletedDuration() = runTest(mainDispatcherRule.dispatcher) {
        val baseWallClock = Instant.parse("2026-06-03T02:35:00Z")
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(
            api,
            monotonicNowMs = { testScheduler.currentTime },
            wallClockNow = { baseWallClock.plusMillis(testScheduler.currentTime) },
        )
        try {
            viewModel.fetchForTesting(fromPairingLoop = true)
            runCurrent()
            viewModel.openSessionDetails()
            runCurrent()

            api.emitWindowStream(
                SessionWindowStreamEvent.Window(
                    sampleWindowSnapshot(
                        entries = listOf(
                            sampleWindowEntry(
                                session = sampleSession(index = 1, lastActiveAgoMinutes = 1),
                                runtimeState = sampleRuntimeState(
                                    threadId = "session-1",
                                    running = true,
                                    phase = SessionRuntimePhase.ToolRunning,
                                    startedAt = Instant.parse("2026-06-03T02:33:56Z"),
                                    updatedAt = Instant.parse("2026-06-03T02:36:00Z"),
                                ).state,
                                latestMessage = null,
                            ),
                        ),
                        observedAt = Instant.parse("2026-06-03T02:35:01Z"),
                    ),
                ),
            )
            runCurrent()
            val runtimeTimeLabel = formatTestTime(Instant.parse("2026-06-03T02:36:00Z"))
            assertEquals("$runtimeTimeLabel · 工具调用中 · 1m04s", viewModel.uiState.value.dashboard.sessionDetails.rows.first().agentStatusLine)

            advanceTimeBy(1_000L)
            runCurrent()
            assertEquals("$runtimeTimeLabel · 工具调用中 · 1m05s", viewModel.uiState.value.dashboard.sessionDetails.rows.first().agentStatusLine)

            api.emitWindowStream(
                SessionWindowStreamEvent.RuntimeState(
                    sampleRuntimeState(
                        threadId = "session-1",
                        running = false,
                        lifecycle = SessionRuntimeLifecycle.Completed,
                        phase = SessionRuntimePhase.AgentFinal,
                        sequence = 2,
                        startedAt = Instant.parse("2026-06-03T02:33:56Z"),
                        updatedAt = Instant.parse("2026-06-03T02:35:01Z"),
                    ).state,
                ),
            )
            runCurrent()
            assertEquals("最近：1m前 用时：1m05s", viewModel.uiState.value.dashboard.sessionDetails.selectedSessionRuntimePhaseLabel)
            assertEquals("最近：1m前 用时：1m05s", viewModel.uiState.value.dashboard.sessionDetails.rows.first().runtimePhaseLabel)
            assertEquals("最近：1m前 用时：1m05s", viewModel.uiState.value.dashboard.sessionDetails.rows.first().agentStatusLine)

            advanceTimeBy(5_000L)
            runCurrent()
            assertEquals("最近：1m前 用时：1m05s", viewModel.uiState.value.dashboard.sessionDetails.rows.first().agentStatusLine)
        } finally {
            viewModel.setForegroundVisible(false)
            runCurrent()
            viewModel.stopPolling()
            runCurrent()
        }
    }

    @Test
    fun selectSessionInDetails_switchesSelectionWithoutRestartingWindowStream() = runTest(mainDispatcherRule.dispatcher) {
        val sessions = listOf(
            sampleSession(index = 1, title = "First", lastActiveAgoMinutes = 1),
            sampleSession(index = 2, title = "Second", lastActiveAgoMinutes = 2),
        )
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot(sessions = sessions)))
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)
        runCurrent()
        viewModel.openSessionDetails()
        runCurrent()
        api.emitWindowStream(
            SessionWindowStreamEvent.Window(
                sampleWindowSnapshot(
                    entries = listOf(
                        sampleWindowEntry(
                            session = sessions[0],
                            runtimeState = sampleRuntimeState(threadId = "session-1", running = true, sequence = 1).state,
                            latestMessage = sampleAgentMessage("session-1", "旧消息"),
                        ),
                        sampleWindowEntry(
                            session = sessions[1],
                            runtimeState = sampleRuntimeState(threadId = "session-2", running = false, sequence = 1).state,
                            latestMessage = null,
                        ),
                    ),
                ),
            ),
        )
        runCurrent()
        assertEquals("旧消息", viewModel.uiState.value.dashboard.sessionDetails.latestAgentMessage)

        viewModel.selectSession(1)
        runCurrent()

        val details = viewModel.uiState.value.dashboard.sessionDetails
        assertEquals("Second", details.selectedSessionTitle)
        assertEquals(null, details.latestAgentMessage)
        assertEquals(AgentMessageStreamStatus.Waiting, details.agentMessageStreamStatus)
        assertEquals(listOf("session-1"), api.streamRequests)
        assertEquals(listOf(5), api.windowStreamLimits)
        assertEquals(1, api.activeStreams)
    }

    @Test
    fun closeDetailScreen_cancelsSessionStream() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)
        viewModel.openSessionDetails()
        runCurrent()
        assertEquals(1, api.activeStreams)

        viewModel.closeDetailScreen()
        runCurrent()

        assertEquals(AppScreen.Dashboard, viewModel.uiState.value.screen)
        assertEquals(1, api.activeStreams)
        assertEquals(false, api.streamIncludeMessagesRequests.last())
    }

    @Test
    fun foregroundRunningToCompleted_vibratesOnceForTurn() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val haptic = FakeHapticController()
        val viewModel = newViewModel(api, hapticController = haptic)
        viewModel.fetchForTesting(fromPairingLoop = true)
        runCurrent()

        api.emitStream("session-1", sampleRuntimeState(running = true, lifecycle = SessionRuntimeLifecycle.Running, turnId = "turn-1"))
        api.emitStream("session-1", sampleRuntimeState(running = false, lifecycle = SessionRuntimeLifecycle.Completed, turnId = "turn-1", sequence = 2))
        api.emitStream("session-1", sampleRuntimeState(running = false, lifecycle = SessionRuntimeLifecycle.Completed, turnId = "turn-1", sequence = 3))
        runCurrent()

        assertEquals(1, haptic.completedCount)
    }

    @Test
    fun initialCompletedAbortedAndBackgroundEvents_doNotVibrate() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val haptic = FakeHapticController()
        val viewModel = newViewModel(api, hapticController = haptic)
        viewModel.fetchForTesting(fromPairingLoop = true)
        runCurrent()

        api.emitStream("session-1", sampleRuntimeState(running = false, lifecycle = SessionRuntimeLifecycle.Completed, turnId = "turn-1"))
        runCurrent()
        assertEquals(0, haptic.completedCount)

        api.emitStream("session-1", sampleRuntimeState(running = true, lifecycle = SessionRuntimeLifecycle.Running, turnId = "turn-2", sequence = 2))
        api.emitStream("session-1", sampleRuntimeState(running = false, lifecycle = SessionRuntimeLifecycle.Aborted, turnId = "turn-2", sequence = 3))
        runCurrent()
        assertEquals(0, haptic.completedCount)

        api.emitStream("session-1", sampleRuntimeState(running = true, lifecycle = SessionRuntimeLifecycle.Running, turnId = "turn-3", sequence = 4))
        runCurrent()
        viewModel.setForegroundVisible(false)
        runCurrent()
        api.emitStream("session-1", sampleRuntimeState(running = false, lifecycle = SessionRuntimeLifecycle.Completed, turnId = "turn-3", sequence = 5))
        runCurrent()

        assertEquals(0, haptic.completedCount)
        assertEquals(0, api.activeStreams)
    }

    @Test
    fun agentMessage_updatesLatestMessageFromWindowStream() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)
        viewModel.openSessionDetails()
        runCurrent()

        api.emitWindowStream(
            SessionWindowStreamEvent.Window(
                sampleWindowSnapshot(
                    entries = listOf(
                        sampleWindowEntry(
                            session = sampleSession(index = 1, lastActiveAgoMinutes = 1),
                            runtimeState = sampleRuntimeState(threadId = "session-1", running = true, sequence = 1).state,
                            latestMessage = null,
                        ),
                    ),
                ),
            ),
        )
        runCurrent()

        api.emitWindowStream(
            SessionWindowStreamEvent.AgentMessage(sampleAgentMessage("session-1", "新回复")),
        )
        runCurrent()

        val details = viewModel.uiState.value.dashboard.sessionDetails
        assertEquals("新回复", details.latestAgentMessage)
        assertEquals(AgentMessageStreamStatus.Live, details.agentMessageStreamStatus)
        assertEquals(null, details.agentMessageError)

        viewModel.setForegroundVisible(false)
        runCurrent()
    }

    @Test
    fun agentMessage_removesBlankLinesBeforeDisplay() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)
        viewModel.openSessionDetails()
        runCurrent()

        api.emitWindowStream(
            SessionWindowStreamEvent.Window(
                sampleWindowSnapshot(
                    entries = listOf(
                        sampleWindowEntry(
                            session = sampleSession(index = 1, lastActiveAgoMinutes = 1),
                            runtimeState = sampleRuntimeState(threadId = "session-1", running = true).state,
                            latestMessage = null,
                        ),
                    ),
                ),
            ),
        )
        runCurrent()

        api.emitWindowStream(
            SessionWindowStreamEvent.AgentMessage(
                sampleAgentMessage(
                    threadId = "session-1",
                    "  第一行  \n\n\n 第二行 \n   \n第三行  ",
                ),
            ),
        )
        runCurrent()

        assertEquals(
            "第一行\n第二行\n第三行",
            viewModel.uiState.value.dashboard.sessionDetails.latestAgentMessage,
        )

        viewModel.setForegroundVisible(false)
        runCurrent()
    }

    @Test
    fun heartbeat_doesNotOverwriteLatestMessage() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)
        viewModel.openSessionDetails()
        runCurrent()

        api.emitWindowStream(
            SessionWindowStreamEvent.Window(
                sampleWindowSnapshot(
                    entries = listOf(
                        sampleWindowEntry(
                            session = sampleSession(index = 1, lastActiveAgoMinutes = 1),
                            runtimeState = sampleRuntimeState(threadId = "session-1", running = true).state,
                            latestMessage = sampleAgentMessage("session-1", "保留这条"),
                        ),
                    ),
                ),
            ),
        )
        runCurrent()

        api.emitWindowStream(SessionWindowStreamEvent.Heartbeat)
        runCurrent()

        val details = viewModel.uiState.value.dashboard.sessionDetails
        assertEquals("保留这条", details.latestAgentMessage)
        assertEquals(AgentMessageStreamStatus.Live, details.agentMessageStreamStatus)

        viewModel.setForegroundVisible(false)
        runCurrent()
    }

    @Test
    fun foregroundRestore_keepsCachedWindowAndReturnsToConnecting() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)
        api.enqueueWindowStreamPlan(
            flow {
                emit(
                    SessionWindowStreamEvent.Window(
                        sampleWindowSnapshot(
                            entries = listOf(
                                sampleWindowEntry(
                                    session = sampleSession(index = 1, lastActiveAgoMinutes = 1),
                                    runtimeState = sampleRuntimeState(threadId = "session-1", running = false, sequence = 1).state,
                                    latestMessage = sampleAgentMessage("session-1", "保留这条缓存"),
                                ),
                            ),
                        ),
                    ),
                )
                awaitCancellation()
            },
        )
        viewModel.openSessionDetails()
        runCurrent()
        assertEquals(AgentMessageStreamStatus.Live, viewModel.uiState.value.dashboard.sessionDetails.agentMessageStreamStatus)

        viewModel.setForegroundVisible(false)
        runCurrent()
        assertEquals(AgentMessageStreamStatus.Disconnected, viewModel.uiState.value.dashboard.sessionDetails.agentMessageStreamStatus)

        viewModel.setForegroundVisible(true)
        runCurrent()

        val details = viewModel.uiState.value.dashboard.sessionDetails
        assertEquals("保留这条缓存", details.latestAgentMessage)
        assertEquals(AgentMessageStreamStatus.Connecting, details.agentMessageStreamStatus)
        assertEquals(1, api.activeStreams)

        viewModel.setForegroundVisible(false)
        runCurrent()
    }

    @Test
    fun sessionStreamFailures_showShortErrors() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)
        viewModel.openSessionDetails()
        runCurrent()

        api.emitWindowStream(
            SessionWindowStreamEvent.Failure(
                message = "需要重新配对",
                reason = SessionStreamFailureReason.Unauthorized,
                retryable = false,
                terminal = true,
            ),
        )
        runCurrent()
        assertEquals("需要重新配对", viewModel.uiState.value.dashboard.sessionDetails.agentMessageError)
        assertEquals(
            AgentMessageStreamStatus.Disconnected,
            viewModel.uiState.value.dashboard.sessionDetails.agentMessageStreamStatus,
        )

        api.emitWindowStream(
            SessionWindowStreamEvent.Failure(
                message = "会话窗口流连接失败",
                reason = SessionStreamFailureReason.NetworkError,
                retryable = true,
                terminal = true,
            ),
        )
        runCurrent()
        assertEquals("会话窗口流连接失败", viewModel.uiState.value.dashboard.sessionDetails.agentMessageError)

        api.emitWindowStream(
            SessionWindowStreamEvent.Failure(
                message = "消息解析失败",
                reason = SessionStreamFailureReason.ParseError,
                retryable = false,
                terminal = false,
            ),
        )
        runCurrent()
        assertEquals("消息解析失败", viewModel.uiState.value.dashboard.sessionDetails.agentMessageError)
    }

    @Test
    fun sessionStreamException_showsShortErrorWithoutCrashing() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api)
        viewModel.fetchForTesting(fromPairingLoop = true)
        runCurrent()
        viewModel.setDashboardPage(DashboardPage.Settings)
        runCurrent()
        api.failNextWindowStream(IOException("stream closed"))

        viewModel.openSessionDetails()
        runCurrent()

        val details = viewModel.uiState.value.dashboard.sessionDetails
        assertEquals("会话窗口流连接失败", details.agentMessageError)
        assertEquals(AgentMessageStreamStatus.Disconnected, details.agentMessageStreamStatus)
    }

    @Test
    fun sessionStreamRetryableFailure_reconnectsAutomatically() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(
            api = api,
            reconnectDelaysMs = listOf(1_000L, 2_000L, 5_000L),
        )
        viewModel.fetchForTesting(fromPairingLoop = true)
        runCurrent()
        viewModel.setDashboardPage(DashboardPage.Settings)
        runCurrent()
        api.failNextWindowStream(IOException("timeout"))

        viewModel.openSessionDetails()
        runCurrent()
        assertEquals(1, api.windowStreamLimits.size)
        assertEquals("会话窗口流连接失败", viewModel.uiState.value.dashboard.sessionDetails.agentMessageError)

        advanceTimeBy(1_000L)
        runCurrent()
        assertEquals(2, api.windowStreamLimits.size)

        api.emitWindowStream(
            SessionWindowStreamEvent.Window(
                sampleWindowSnapshot(
                    entries = listOf(
                        sampleWindowEntry(
                            session = sampleSession(index = 1, lastActiveAgoMinutes = 1),
                            runtimeState = sampleRuntimeState(threadId = "session-1", running = true).state,
                            latestMessage = null,
                        ),
                    ),
                ),
            ),
        )
        runCurrent()

        val details = viewModel.uiState.value.dashboard.sessionDetails
        assertEquals(AgentMessageStreamStatus.Waiting, details.agentMessageStreamStatus)
        assertEquals(null, details.agentMessageError)

        viewModel.setForegroundVisible(false)
        runCurrent()
    }

    @Test
    fun sessionStreamUnauthorizedFailure_doesNotReconnect() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(
            api = api,
            reconnectDelaysMs = listOf(1_000L, 2_000L),
        )
        viewModel.fetchForTesting(fromPairingLoop = true)
        runCurrent()
        viewModel.setDashboardPage(DashboardPage.Settings)
        runCurrent()
        api.enqueueWindowStreamPlan(
            flow {
                emit(
                    SessionWindowStreamEvent.Failure(
                        message = "需要重新配对",
                        reason = SessionStreamFailureReason.Unauthorized,
                        retryable = false,
                        terminal = true,
                    ),
                )
            },
        )

        viewModel.openSessionDetails()
        runCurrent()
        advanceTimeBy(5_000L)
        runCurrent()

        assertEquals(1, api.windowStreamLimits.size)
        assertEquals("需要重新配对", viewModel.uiState.value.dashboard.sessionDetails.agentMessageError)
    }

    @Test
    fun bootstrapRequest_savesFirstConfigWithoutConfirmation() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(api = api, enableBootstrap = false)

        viewModel.presentBootstrapRequest(
            sampleBootstrapRequest(
                endpoints = listOf(
                    ServerEndpoint("lan", "局域网", "http://192.168.1.12:8787", 0),
                    ServerEndpoint("public", "公网", "https://watch.example.com", 1),
                ),
            ),
        )
        runCurrent()

        val state = viewModel.uiState.value
        assertEquals(AppScreen.Dashboard, state.screen)
        assertEquals("局域网", state.settings.activeEndpointLabel)
        assertEquals("http://192.168.1.12:8787", state.settings.baseUrl)
        assertEquals(1, api.fetchStatusCount)
    }

    @Test
    fun bootstrapRequest_overwritesExistingConfigWithoutConfirmation() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        val viewModel = newViewModel(
            api = api,
            enableBootstrap = false,
            preloadedPairingSuccess = true,
            preloadedServerBaseUrl = "https://old.example.com",
        )

        viewModel.presentBootstrapRequest(
            sampleBootstrapRequest(
                endpoints = listOf(
                    ServerEndpoint("public", "公网", "https://new.example.com", 0),
                    ServerEndpoint("lan", "局域网", "http://192.168.1.12:8787", 1),
                ),
            ),
        )
        runCurrent()

        val state = viewModel.uiState.value
        assertEquals(AppScreen.Dashboard, state.screen)
        assertEquals("公网", state.settings.activeEndpointLabel)
        assertEquals("https://new.example.com", state.settings.baseUrl)
        assertEquals(1, api.fetchStatusCount)
    }

    @Test
    fun bootstrapRequest_networkFailureShowsServiceUnavailable() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.NetworkFailure("服务不可达"))
        val viewModel = newViewModel(
            api = api,
            enableBootstrap = false,
            endpointProbe = EndpointHealthProbe { HealthCheckResult.Offline("服务不可达") },
        )

        viewModel.presentBootstrapRequest(
            sampleBootstrapRequest(
                endpoints = listOf(
                    ServerEndpoint("public", "公网", "https://new.example.com", 0),
                    ServerEndpoint("lan", "局域网", "http://192.168.1.12:8787", 1),
                ),
            ),
        )

        runCurrent()

        val state = viewModel.uiState.value
        assertEquals(AppScreen.Pairing, state.screen)
        assertEquals("服务不可达", state.pairing.statusLabel)
        assertEquals("https://new.example.com", state.settings.baseUrl)
    }

    @Test
    fun presentBootstrapRequest_savesConfigAndFetchesStatusWhenHealthIsOnline() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        api.healthCheckResult = HealthCheckResult.Online
        val viewModel = newViewModel(api = api, enableBootstrap = false)

        viewModel.presentBootstrapRequest(
            sampleBootstrapRequest(
                endpoints = listOf(
                    ServerEndpoint("public", "公网", "https://bootstrap.example.com", 1),
                    ServerEndpoint("lan", "局域网", "http://192.168.1.12:8787", 0),
                ),
            ),
        )
        runCurrent()

        assertEquals(AppScreen.Dashboard, viewModel.uiState.value.screen)
        assertEquals("局域网", viewModel.uiState.value.settings.activeEndpointLabel)
        assertEquals("192.168.1.12:8787", viewModel.uiState.value.settings.serviceHostLabel)
        assertEquals("http://192.168.1.12:8787", viewModel.uiState.value.settings.baseUrl)
        assertEquals(1, api.fetchStatusCount)
    }

    @Test
    fun presentBootstrapRequest_keepsSavedConfigWhenHealthFails() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.Success(sampleSnapshot()))
        api.healthCheckResult = HealthCheckResult.Offline("服务不可达")
        val viewModel = newViewModel(
            api = api,
            enableBootstrap = false,
            endpointProbe = EndpointHealthProbe { HealthCheckResult.Offline("服务不可达") },
        )

        viewModel.presentBootstrapRequest(
            sampleBootstrapRequest(
                endpoints = listOf(
                    ServerEndpoint("lan", "局域网", "http://192.168.1.22:8787", 0),
                    ServerEndpoint("public", "公网", "https://watch.example.com", 1),
                ),
            ),
        )
        runCurrent()

        val state = viewModel.uiState.value
        assertEquals(AppScreen.Pairing, state.screen)
        assertEquals("服务不可达", state.pairing.statusLabel)
        assertTrue(state.pairing.serviceLabel.contains("服务不可达"))
        assertEquals("http://192.168.1.22:8787", state.settings.baseUrl)
        assertEquals(0, api.fetchStatusCount)
    }

    @Test
    fun fetchStatus_networkFailureSwitchesToReachableEndpoint() = runTest(mainDispatcherRule.dispatcher) {
        val api = FakeWatcherApi(StatusFetchResult.NetworkFailure("局域网不可达"))
        api.next = SequenceStatus(
            ArrayDeque(
                listOf(
                    StatusFetchResult.NetworkFailure("局域网不可达"),
                    StatusFetchResult.Success(sampleSnapshot()),
                ),
            ),
        )
        val viewModel = newViewModel(
            api = api,
            enableBootstrap = false,
            preloadedPairingSuccess = true,
            preloadedEndpoints = listOf(
                ServerEndpoint("lan", "局域网", "http://192.168.1.12:8787", 0),
                ServerEndpoint("public", "公网", "https://watch.example.com", 1),
            ),
            endpointProbe = EndpointHealthProbe { endpoint ->
                if (endpoint.id == "lan") {
                    HealthCheckResult.Offline("局域网不可达")
                } else {
                    HealthCheckResult.Online
                }
            },
        )

        viewModel.fetchForTesting(fromPairingLoop = false)

        assertEquals(AppScreen.Dashboard, viewModel.uiState.value.screen)
        assertEquals("公网", viewModel.uiState.value.settings.activeEndpointLabel)
        assertEquals("https://watch.example.com", viewModel.uiState.value.settings.baseUrl)
        assertEquals(2, api.fetchStatusCount)
    }

    private fun newViewModel(
        api: FakeWatcherApi,
        updateManager: WatcherApkUpdateManager = FakeWatcherApkUpdateManager(),
        reconnectDelaysMs: List<Long> = listOf(1_000L, 2_000L, 5_000L, 10_000L),
        randomIndexPicker: (Int) -> Int = { 0 },
        homeQuotaTipHideMs: Long = 2_000L,
        hapticController: WatcherHapticController = WatcherHapticController {},
        autoPolling: Boolean = false,
        preloadedAppUpdatePreferences: AppUpdatePreferences = AppUpdatePreferences(),
        preloadedDailyTrend30d: DailyTrend30dSnapshot? = null,
        preloadedSnapshot: WatcherStatusSnapshot? = null,
        preloadedSessionDetailsWindow: SessionDetailsWindowCacheSnapshot? = null,
        preloadedPairingSuccess: Boolean = false,
        preloadedServerBaseUrl: String? = null,
        preloadedEndpoints: List<ServerEndpoint>? = null,
        enableBootstrap: Boolean = false,
        screenshotUploadQueue: ScreenshotUploadQueue = InMemoryScreenshotUploadQueue(),
        diagnosticEventLogger: DiagnosticEventLogger = NoOpDiagnosticEventLogger,
        diagnosticUploadManager: DiagnosticUploadManager? = null,
        diagnosticUiStateSink: (AppUiState) -> Unit = {},
        diagnosticRuntimeContext: DiagnosticRuntimeContext? = null,
        endpointProbe: EndpointHealthProbe = EndpointHealthProbe { HealthCheckResult.Online },
        watchBootstrapClient: WatchBootstrapGateway? = null,
        watchBootstrapCodeStore: WatchBootstrapCodeStore? = null,
        sharedStore: FakeKeyValueStore = FakeKeyValueStore(),
        monotonicNowMs: () -> Long = { 0L },
        wallClockNow: () -> Instant = { Instant.parse("2026-06-03T03:31:00Z") },
    ): WatcherViewModel {
        val store = sharedStore
        val pairingStateStore = PairingPreferenceStore(store)
        val serverConfigRepository = ServerConfigRepository(
            store = store,
            fallbackBaseUrl = "https://127.0.0.1.invalid",
        )
        val dailyTrendStore = DailyTrendPreferenceStore(store)
        val statusSnapshotStore = WatcherStatusSnapshotPreferenceStore(store)
        val sessionDetailsWindowStore = SessionDetailsWindowPreferenceStore(store)
        val appUpdatePreferenceStore = SharedPreferencesAppUpdatePreferenceStore(store)
        preloadedDailyTrend30d?.let(dailyTrendStore::write)
        appUpdatePreferenceStore.write(preloadedAppUpdatePreferences)
        if (preloadedPairingSuccess) {
            pairingStateStore.markPaired()
        }
        preloadedEndpoints?.let {
            serverConfigRepository.save(
                endpoints = it,
                source = ServerConfigSource.DesktopBootstrap,
            )
        } ?: preloadedServerBaseUrl?.let {
            serverConfigRepository.save(
                endpoints = listOf(ServerEndpoint("preloaded", "预置入口", it, 0)),
                source = ServerConfigSource.DesktopBootstrap,
            )
        }
        preloadedSnapshot?.let(statusSnapshotStore::write)
        preloadedSessionDetailsWindow?.let(sessionDetailsWindowStore::write)
        return WatcherViewModel(
            api = api,
            appUpdateManager = updateManager,
            appUpdatePreferenceStore = appUpdatePreferenceStore,
            tokenRepository = DeviceTokenRepository(
                store = store,
                tokenGenerator = FixedTokenGenerator("test-token-0123456789abcdef0123456789"),
            ),
            serverConfigRepository = serverConfigRepository,
            endpointSelector = EndpointSelector(
                probe = endpointProbe,
                monotonicNowMs = monotonicNowMs,
            ),
            config = WatcherViewModel.Config(
                baseUrl = "https://127.0.0.1.invalid",
                deviceName = "Xiaomi Watch 5",
                appVersion = "0.2.10",
                appVersionCode = 16,
                debugToolsEnabled = true,
            ),
            pairingStateStore = pairingStateStore,
            dailyTrendStore = dailyTrendStore,
            statusSnapshotStore = statusSnapshotStore,
            sessionDetailsWindowStore = sessionDetailsWindowStore,
            screenshotUploadQueue = screenshotUploadQueue,
            diagnosticEventLogger = diagnosticEventLogger,
            diagnosticUploadManager = diagnosticUploadManager,
            diagnosticUiStateSink = diagnosticUiStateSink,
            diagnosticRuntimeContext = diagnosticRuntimeContext,
            debugStore = null,
            watchBootstrapClient = watchBootstrapClient,
            watchBootstrapCodeStore = watchBootstrapCodeStore,
            ioDispatcher = mainDispatcherRule.dispatcher,
            autoPolling = autoPolling,
            enableBootstrap = enableBootstrap,
            appUpdateCheckMinDurationMs = 0L,
            sessionStreamReconnectDelaysMs = reconnectDelaysMs,
            monotonicNowMs = monotonicNowMs,
            wallClockNow = wallClockNow,
            homeQuotaTipRandomIndex = randomIndexPicker,
            homeQuotaTipHideDurationMs = homeQuotaTipHideMs,
            hapticController = hapticController,
        )
    }

    private fun formatTestTime(instant: Instant): String {
        return DateTimeFormatter.ofPattern("HH:mm")
            .withZone(ZoneId.systemDefault())
            .format(instant)
    }

    private fun sampleBootstrapRequest(
        channel: AppUpdateChannel = AppUpdateChannel.Beta,
        endpoints: List<ServerEndpoint>,
    ): ai.openwatcher.watchapp.data.BootstrapRequest {
        return ai.openwatcher.watchapp.data.BootstrapRequest(
            channel = channel,
            endpoints = endpoints,
            deviceToken = "test-token-0123456789abcdef0123456789",
            deviceName = "Xiaomi Watch 5",
            source = "desktop-bootstrap",
        )
    }

    private fun sampleSnapshot(
        observedAt: Instant? = Instant.parse("2026-06-03T03:30:00Z"),
        contextUsedTokens: Long = 183_000,
        contextWindow: Long = 256_000,
        contextPressurePercent: Int = 71,
        contextCompactThresholdTokens: Long? = 192_000,
        contextCompactThresholdPercent: Int? = 75,
        fiveHour: QuotaWindow? = QuotaWindow(12f, 88f, Instant.parse("2026-06-03T06:32:00Z")),
        weekly: QuotaWindow? = QuotaWindow(20f, 80f, Instant.parse("2026-06-07T16:00:00Z")),
        heatmap24h: Heatmap24hSnapshot? = null,
        heatmap7d: ai.openwatcher.watchapp.data.Heatmap7dSnapshot? = null,
        dailyUsage: DailyUsageSnapshot? = null,
        dailyTrend30d: DailyTrend30dSnapshot? = null,
        sessions: List<SessionSnapshot>? = null,
    ): WatcherStatusSnapshot {
        return WatcherStatusSnapshot(
            observedAt = observedAt,
            quota = QuotaSnapshot(
                source = "oauth-api",
                fresh = true,
                status = QuotaStatus.Ok,
                planType = "pro",
                fiveHour = fiveHour,
                weekly = weekly,
            ),
            heatmap24h = heatmap24h ?: Heatmap24hSnapshot(
                timezone = "Asia/Shanghai",
                generatedAt = observedAt,
                peakHourStart = Instant.parse("2026-06-03T02:00:00Z"),
                buckets = listOf(
                    HeatmapBucket(
                        hourStart = Instant.parse("2026-06-03T02:00:00Z"),
                        inputTokens = 88_000,
                        cachedInputTokens = 26_000,
                        outputTokens = 14_000,
                        reasoningOutputTokens = 6_000,
                        totalTokens = 128_000,
                        activeThreads = 4,
                    ),
                ),
            ),
            heatmap7d = heatmap7d,
            dailyUsage = dailyUsage,
            dailyTrend30d = dailyTrend30d,
            sessions = listOf(
                sampleSession(
                    index = 1,
                    title = "AnyChat",
                    contextUsedTokens = contextUsedTokens,
                    contextWindow = contextWindow,
                    contextPressurePercent = contextPressurePercent,
                    contextCompactThresholdTokens = contextCompactThresholdTokens,
                    contextCompactThresholdPercent = contextCompactThresholdPercent,
                    lastActiveAgoMinutes = 1,
                ),
            ).let { sessions ?: it },
            errors = emptyList(),
        )
    }

    private fun newDiagnosticEventStore(): DiagnosticEventStore {
        return DiagnosticEventStore(
            directory = Files.createTempDirectory("watcher-diagnostic-events").toFile(),
            ioDispatcher = mainDispatcherRule.dispatcher,
            clock = { Instant.parse("2026-06-03T03:31:00Z") },
            zoneId = ZoneId.of("UTC"),
        )
    }

    private fun newDiagnosticEventLogger(
        uiStateHolder: DiagnosticUiStateHolder,
        runtimeContext: DiagnosticRuntimeContext,
        eventStore: DiagnosticEventStore,
    ): StructuredDiagnosticEventLogger {
        return StructuredDiagnosticEventLogger(
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
                versionName = "0.2.10",
                versionCode = 16,
                buildType = "debug",
            ),
            clock = { Instant.parse("2026-06-03T03:31:00Z") },
        )
    }

    private fun newDiagnosticUploadManager(
        api: FakeWatcherApi,
        uiStateHolder: DiagnosticUiStateHolder,
        eventStore: DiagnosticEventStore,
        logger: StructuredDiagnosticEventLogger,
    ): DiagnosticUploadManager {
        var monotonic = 0L
        return DiagnosticUploadManager(
            api = api,
            eventStore = eventStore,
            eventLogger = logger,
            snapshotCollector = DiagnosticSnapshotCollector(
                uiStateProvider = uiStateHolder::current,
                eventLogger = logger,
            ),
            stateStore = DiagnosticUploadPreferenceStore(FakeKeyValueStore()),
            pendingDirectory = Files.createTempDirectory("watcher-diagnostic-pending").toFile(),
            deviceName = "Xiaomi Watch 5",
            appVersion = "0.2.10",
            ioDispatcher = mainDispatcherRule.dispatcher,
            clock = { Instant.parse("2026-06-03T03:31:00Z") },
            monotonicNowMs = { monotonic.also { monotonic += 300L } },
        )
    }

    private fun sampleDailyTrend30d(endDate: String = "2026-06-02"): DailyTrend30dSnapshot {
        val startDate = LocalDate.parse("2026-05-04")
        val days = List(30) { index ->
            DailyTrendDay(
                date = startDate.plusDays(index.toLong()).toString(),
                totalTokens = ((index + 1) * 1_000L),
            )
        }
        return DailyTrend30dSnapshot(
            timezone = "Asia/Shanghai",
            generatedAt = Instant.parse("2026-06-03T03:30:00Z"),
            startDate = "2026-05-04",
            endDate = endDate,
            totalTokens = days.sumOf { it.totalTokens },
            averageTokens = 15_500L,
            peakTokens = 30_000L,
            estimatedValueUsd = 31.46,
            estimatedValueLabel = "\$31.46",
            days = days,
        )
    }

    private fun sampleHeatmap24h(): Heatmap24hSnapshot {
        val start = Instant.parse("2026-06-03T00:00:00Z")
        return Heatmap24hSnapshot(
            timezone = "Asia/Shanghai",
            generatedAt = start.plusSeconds(23 * 3_600L),
            peakHourStart = start.plusSeconds(23 * 3_600L),
            buckets = List(24) { index ->
                HeatmapBucket(
                    hourStart = start.plusSeconds(index * 3_600L),
                    inputTokens = 1_000L + index,
                    cachedInputTokens = 500L + index,
                    outputTokens = 100L + index,
                    reasoningOutputTokens = 50L + index,
                    totalTokens = 2_000L + index,
                    activeThreads = 1,
                )
            },
        )
    }

    private fun sampleHeatmap7d(): Heatmap7dSnapshot {
        val start = LocalDate.parse("2026-05-28")
        return Heatmap7dSnapshot(
            timezone = "Asia/Shanghai",
            generatedAt = Instant.parse("2026-06-03T03:30:00Z"),
            startDate = start.toString(),
            endDate = start.plusDays(6).toString(),
            peakTokens = 7_000L,
            days = List(7) { index ->
                val hours = MutableList(24) { 0L }
                hours[index] = (index + 1) * 1_000L
                Heatmap7dDay(
                    date = start.plusDays(index.toLong()).toString(),
                    totalTokens = hours.sum(),
                    hours = hours,
                )
            },
        )
    }

    private fun snapshotForWeeklyDelta(
        delta: Float,
        timeRemainingPercent: Float = 60f,
    ): WatcherStatusSnapshot {
        val observedAt = Instant.parse("2026-06-03T03:30:00Z")
        val remainingPercent = (timeRemainingPercent + delta).coerceIn(0f, 100f)
        val usedPercent = 100f - remainingPercent
        val resetAt = observedAt.plusSeconds((7L * 24L * 60L * 60L * (timeRemainingPercent / 100f)).toLong())
        return sampleSnapshot(
            observedAt = observedAt,
            weekly = QuotaWindow(
                usedPercent = usedPercent,
                remainingPercent = remainingPercent,
                resetAt = resetAt,
            ),
        )
    }

    private fun sampleSession(
        index: Int,
        title: String = "Session $index",
        model: String = "gpt-5.5",
        reasoningEffort: String = "high",
        tokensUsedTotal: Long = 183_000,
        contextUsedTokens: Long = 183_000,
        contextWindow: Long = 256_000,
        contextPressurePercent: Int = 71,
        contextCompactThresholdTokens: Long? = 192_000,
        contextCompactThresholdPercent: Int? = 75,
        lastActiveAgoMinutes: Int,
    ): SessionSnapshot {
        return SessionSnapshot(
            threadId = "session-$index",
            title = title,
            updatedAt = Instant.parse("2026-06-03T03:31:00Z").minusSeconds(lastActiveAgoMinutes * 60L),
            model = model,
            reasoningEffort = reasoningEffort,
            tokensUsedTotal = tokensUsedTotal,
            contextUsedTokens = contextUsedTokens,
            contextWindow = contextWindow,
            contextPressurePercent = contextPressurePercent,
            contextCompactThresholdTokens = contextCompactThresholdTokens,
            contextCompactThresholdPercent = contextCompactThresholdPercent,
            lastActiveAgoMinutes = lastActiveAgoMinutes,
        )
    }

    private fun sampleAgentMessage(threadId: String, text: String): SessionAgentMessage {
        return SessionAgentMessage(
            threadId = threadId,
            eventId = "event-$threadId",
            createdAt = Instant.parse("2026-06-04T02:35:00Z"),
            text = text,
            truncated = false,
        )
    }

    private fun sampleUpdate(
        releaseNotes: List<ai.openwatcher.watchapp.data.WatcherApkReleaseNote> = emptyList(),
    ): WatcherApkUpdate {
        return WatcherApkUpdate(
            versionName = "0.2.15",
            versionCode = 17,
            artifact = "openwatcher-watchapp-v0.2.15-release.apk",
            sha256 = "abc123",
            commit = "deadbee",
            builtAt = "2026-06-05T02:00:00Z",
            downloadUrl = "https://127.0.0.1.invalid/file/latest-apk",
            releaseNotes = releaseNotes,
        )
    }

    private fun sampleWindowSnapshot(
        entries: List<SessionWindowEntry>,
        observedAt: Instant? = Instant.parse("2026-06-04T02:35:00Z"),
    ): SessionWindowSnapshot {
        return SessionWindowSnapshot(
            observedAt = observedAt,
            limit = 5,
            threadOrder = entries.map { it.session.threadId },
            sessions = entries,
        )
    }

    private fun sampleWindowEntry(
        session: SessionSnapshot,
        runtimeState: SessionRuntimeState,
        latestMessage: SessionAgentMessage?,
    ): SessionWindowEntry {
        return SessionWindowEntry(
            session = session,
            runtimeState = runtimeState,
            latestAgentMessage = latestMessage,
        )
    }

    private fun sampleReleaseNote(
        publishedAtLabel: String,
        summary: String,
    ): ai.openwatcher.watchapp.data.WatcherApkReleaseNote {
        return ai.openwatcher.watchapp.data.WatcherApkReleaseNote(
            versionName = "0.2.15",
            versionCode = 17,
            publishedAtLabel = publishedAtLabel,
            summary = summary,
        )
    }

    private fun sampleRuntimeState(
        threadId: String = "session-1",
        running: Boolean,
        lifecycle: SessionRuntimeLifecycle = if (running) {
            SessionRuntimeLifecycle.Running
        } else {
            SessionRuntimeLifecycle.Completed
        },
        phase: SessionRuntimePhase = SessionRuntimePhase.Reasoning,
        turnId: String = "turn-1",
        sequence: Long = 1L,
        startedAt: Instant = Instant.parse("2026-06-04T02:35:30Z"),
        updatedAt: Instant = Instant.parse("2026-06-04T02:36:00Z"),
    ): SessionStreamEvent.RuntimeState {
        return SessionStreamEvent.RuntimeState(
            SessionRuntimeState(
                threadId = threadId,
                turnId = turnId,
                startedAt = startedAt,
                running = running,
                lifecycle = lifecycle,
                phase = phase,
                updatedAt = updatedAt,
                sequence = sequence,
            ),
        )
    }

    private class FakeHapticController : WatcherHapticController {
        var completedCount = 0
            private set
        var screenshotCount = 0
            private set

        override fun vibrateSessionCompleted() {
            completedCount += 1
        }

        override fun vibrateScreenshotCaptured() {
            screenshotCount += 1
        }
    }

    private class FakeWatchBootstrapGateway(
        private val registerResults: ArrayDeque<Any> = ArrayDeque(
            listOf(WatchBootstrapRegistration("ABCD2345", "2026-06-12T00:00:00Z")),
        ),
        private val pollResults: ArrayDeque<Any> = ArrayDeque(),
    ) : WatchBootstrapGateway {
        val registerCalls = mutableListOf<RegisterCall>()
        val pollCalls = mutableListOf<String>()

        override suspend fun register(deviceName: String, appVersion: String): WatchBootstrapRegistration {
            val next = registerResults.removeFirstOrNull()
                ?: WatchBootstrapRegistration("ABCD2345", "2026-06-12T00:00:00Z")
            if (next is Throwable) {
                throw next
            }
            val registration = next as WatchBootstrapRegistration
            registerCalls += RegisterCall(deviceName, appVersion, registration.bootstrapCode)
            return registration
        }

        override suspend fun poll(bootstrapCode: String): WatchBootstrapPollResult {
            pollCalls += bootstrapCode
            val next = pollResults.removeFirstOrNull() ?: WatchBootstrapPollResult.Pending
            if (next is Throwable) {
                throw next
            }
            return next as WatchBootstrapPollResult
        }
    }

    private data class RegisterCall(
        val deviceName: String,
        val appVersion: String,
        val returnedCode: String,
    )

    private class FakeWatcherApi(initial: StatusFetchResult) : WatcherApi {
        var next: StatusBehavior = ImmediateStatus(initial)
        var fetchStatusCount = 0
        var healthCheckCount = 0
        val streamRequests = mutableListOf<String>()
        val streamIncludeMessagesRequests = mutableListOf<Boolean>()
        val windowStreamLimits = mutableListOf<Int>()
        val windowStreamPreferredOrders = mutableListOf<List<String>>()
        var statusStreamRequests = 0
        val statusStreamTrendRequests = mutableListOf<Boolean>()
        val reportedEvents = mutableListOf<SessionStreamClientEventReport>()
        val screenshotUploads = mutableListOf<ScreenshotUploadCall>()
        var nextScreenshotUpload: ScreenshotUploadResult = ScreenshotUploadResult.Success("watch.png")
        val diagnosticUploads = mutableListOf<DiagnosticUploadCall>()
        var nextDiagnosticUpload: DiagnosticUploadResult = DiagnosticUploadResult.Success(
            diagnosticId = "diag-test",
            receivedAt = Instant.parse("2026-06-03T03:35:00Z"),
        )
        var nextHealthCheck: HealthCheckResult = HealthCheckResult.Online
        var healthCheckResult: HealthCheckResult
            get() = nextHealthCheck
            set(value) {
                nextHealthCheck = value
            }
        var activeStreams = 0
            private set
        private val streams = mutableMapOf<String, MutableSharedFlow<SessionStreamEvent>>()
        private val windowStream = MutableSharedFlow<SessionWindowStreamEvent>(replay = 1, extraBufferCapacity = 16)
        private val statusStream = MutableSharedFlow<StatusStreamEvent>(replay = 1, extraBufferCapacity = 16)
        private val streamPlans = mutableMapOf<String, ArrayDeque<StreamPlan>>()
        private val windowStreamPlans = ArrayDeque<WindowStreamPlan>()

        override suspend fun fetchStatus(token: String): StatusFetchResult {
            fetchStatusCount += 1
            return next.await()
        }

        override suspend fun checkHealth(): HealthCheckResult {
            healthCheckCount += 1
            return nextHealthCheck
        }

        override suspend fun uploadScreenshot(
            token: String,
            request: ScreenshotUploadRequest,
        ): ScreenshotUploadResult {
            screenshotUploads += ScreenshotUploadCall(token, request)
            return nextScreenshotUpload
        }

        override suspend fun uploadDiagnostics(
            token: String,
            request: DiagnosticUploadRequest,
            onProgress: (bytesUploaded: Long, totalBytes: Long) -> Unit,
        ): DiagnosticUploadResult {
            diagnosticUploads += DiagnosticUploadCall(token, request)
            onProgress(request.gzipBytes.size.toLong(), request.gzipBytes.size.toLong())
            return nextDiagnosticUpload
        }

        override fun streamStatus(token: String, includeDailyTrend30d: Boolean): Flow<StatusStreamEvent> = flow {
            statusStreamRequests += 1
            statusStreamTrendRequests += includeDailyTrend30d
            statusStream.collect { emit(it) }
        }

        override fun streamSessionAgentMessages(
            token: String,
            threadId: String,
            includeMessages: Boolean,
        ): Flow<SessionStreamEvent> = flow {
            streamRequests += threadId
            streamIncludeMessagesRequests += includeMessages
            activeStreams += 1
            try {
                when (val plan = nextStreamPlan(threadId)) {
                    is StreamPlan.Throw -> throw plan.error
                    is StreamPlan.FlowPlan -> plan.flow.collect { emit(it) }
                    null -> streamFor(threadId).collect { emit(it) }
                }
            } finally {
                activeStreams -= 1
            }
        }

        override fun streamSessionWindow(
            token: String,
            limit: Int,
            preferredOrder: List<String>,
        ): Flow<SessionWindowStreamEvent> = flow {
            windowStreamLimits += limit
            windowStreamPreferredOrders += preferredOrder
            activeStreams += 1
            try {
                when (val plan = nextWindowStreamPlan()) {
                    is WindowStreamPlan.Throw -> throw plan.error
                    is WindowStreamPlan.FlowPlan -> plan.flow.collect { emit(it) }
                    null -> windowStream.collect { emit(it) }
                }
            } finally {
                activeStreams -= 1
            }
        }

        override suspend fun reportSessionStreamClientEvent(token: String, report: SessionStreamClientEventReport) {
            reportedEvents += report
        }

        fun failNextStream(threadId: String, error: Throwable) {
            streamPlans.getOrPut(threadId) { ArrayDeque() }.addLast(StreamPlan.Throw(error))
        }

        fun enqueueStreamPlan(threadId: String, flow: Flow<SessionStreamEvent>) {
            streamPlans.getOrPut(threadId) { ArrayDeque() }.addLast(StreamPlan.FlowPlan(flow))
        }

        fun emitStream(threadId: String, event: SessionStreamEvent) {
            streamFor(threadId).tryEmit(event)
        }

        fun emitStatusStream(event: StatusStreamEvent) {
            statusStream.tryEmit(event)
        }

        fun emitWindowStream(event: SessionWindowStreamEvent) {
            windowStream.tryEmit(event)
        }

        fun enqueueWindowStreamPlan(flow: Flow<SessionWindowStreamEvent>) {
            windowStreamPlans.addLast(WindowStreamPlan.FlowPlan(flow))
        }

        fun failNextWindowStream(error: Throwable) {
            windowStreamPlans.addLast(WindowStreamPlan.Throw(error))
        }

        private fun streamFor(threadId: String): MutableSharedFlow<SessionStreamEvent> {
            return streams.getOrPut(threadId) {
                MutableSharedFlow(replay = 1, extraBufferCapacity = 16)
            }
        }

        private fun nextStreamPlan(threadId: String): StreamPlan? {
            val plans = streamPlans[threadId] ?: return null
            val next = plans.removeFirstOrNull()
            if (plans.isEmpty()) {
                streamPlans.remove(threadId)
            }
            return next
        }

        private fun nextWindowStreamPlan(): WindowStreamPlan? {
            return windowStreamPlans.removeFirstOrNull()
        }
    }

    private data class ScreenshotUploadCall(
        val token: String,
        val request: ScreenshotUploadRequest,
    )

    private data class DiagnosticUploadCall(
        val token: String,
        val request: DiagnosticUploadRequest,
    )

    private class InMemoryScreenshotUploadQueue : ScreenshotUploadQueue {
        private val items = mutableMapOf<String, Pair<Instant, ByteArray>>()
        private var nextId = 0

        override suspend fun enqueue(pngBytes: ByteArray, createdAt: Instant): Boolean {
            if (pngBytes.isEmpty()) {
                return false
            }
            val id = "screenshot-%013d-%04d.png".format(Locale.US, createdAt.toEpochMilli(), nextId++)
            items[id] = createdAt to pngBytes.copyOf()
            return true
        }

        override suspend fun pending(): List<PendingScreenshotUpload> {
            return items.entries
                .map { (id, value) -> PendingScreenshotUpload(id = id, createdAt = value.first) }
                .sortedWith(compareBy<PendingScreenshotUpload> { it.createdAt }.thenBy { it.id })
        }

        override suspend fun read(pending: PendingScreenshotUpload): ByteArray? {
            return items[pending.id]?.second?.copyOf()
        }

        override suspend fun delete(pending: PendingScreenshotUpload) {
            items.remove(pending.id)
        }
    }

    private sealed interface StreamPlan {
        data class Throw(val error: Throwable) : StreamPlan
        data class FlowPlan(val flow: Flow<SessionStreamEvent>) : StreamPlan
    }

    private sealed interface WindowStreamPlan {
        data class Throw(val error: Throwable) : WindowStreamPlan
        data class FlowPlan(val flow: Flow<SessionWindowStreamEvent>) : WindowStreamPlan
    }

    private sealed interface StatusBehavior {
        suspend fun await(): StatusFetchResult
    }

    private class ImmediateStatus(
        private val value: StatusFetchResult,
    ) : StatusBehavior {
        override suspend fun await(): StatusFetchResult = value
    }

    private class SequenceStatus(
        private val values: ArrayDeque<StatusFetchResult>,
    ) : StatusBehavior {
        override suspend fun await(): StatusFetchResult {
            return values.removeFirstOrNull() ?: error("no more status results")
        }
    }

    private class SuspendedStatus(
        private val deferred: CompletableDeferred<StatusFetchResult>,
    ) : StatusBehavior {
        override suspend fun await(): StatusFetchResult = deferred.await()
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

    private class FixedTokenGenerator(
        private val token: String,
    ) : TokenGenerator {
        override fun generate(): String = token
    }

    private class FakeWatcherApkUpdateManager(
        var checkResult: WatcherApkUpdateCheckResult = WatcherApkUpdateCheckResult.Failure("未配置"),
        var installResult: WatcherApkInstallResult = WatcherApkInstallResult.Failure("未配置"),
        var cachedInstallResult: WatcherApkInstallResult = WatcherApkInstallResult.Failure("未配置"),
        var cachedUpdateAvailable: Boolean = false,
        var downloadProgress: List<WatcherApkUpdateProgress> = emptyList(),
        var installPermissionEnabled: Boolean = false,
        var permissionSettingsOpened: Boolean = false,
    ) : WatcherApkUpdateManager {
        var downloadRequests: Int = 0
        var cachedInstallRequests: Int = 0

        override fun canRequestPackageInstalls(): Boolean = installPermissionEnabled

        override fun openInstallPermissionSettings(): Boolean {
            permissionSettingsOpened = true
            return true
        }

        override suspend fun fetchLatestUpdate(): WatcherApkUpdateCheckResult = checkResult

        override suspend fun hasCachedUpdate(update: WatcherApkUpdate): Boolean = cachedUpdateAvailable

        override suspend fun startInstallFromCache(update: WatcherApkUpdate): WatcherApkInstallResult {
            cachedInstallRequests += 1
            return cachedInstallResult
        }

        override suspend fun downloadAndStartInstall(
            update: WatcherApkUpdate,
            onProgress: (WatcherApkUpdateProgress) -> Unit,
        ): WatcherApkInstallResult {
            downloadRequests += 1
            downloadProgress.forEach(onProgress)
            return installResult
        }
    }

    private data class QuotaPoolCase(
        val name: String,
        val expectedPool: HomeQuotaTipPool,
        val snapshot: WatcherStatusSnapshot,
    )
}
