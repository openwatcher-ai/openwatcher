package ai.openwatcher.watchapp.ui

import androidx.compose.ui.graphics.Color
import ai.openwatcher.watchapp.data.DebugDemoScenario
import ai.openwatcher.watchapp.ui.home.HomeQuotaTipPool

enum class AppScreen {
    Splash,
    Pairing,
    BootstrapConfirm,
    Dashboard,
    Heatmap24h,
    SessionDetails,
    Settings,
    Offline,
}

enum class DashboardPage {
    Home,
    Settings,
}

enum class SettingsDestination {
    Root,
    About,
    UpdateCheck,
    UpdateLatest,
    CurrentVersionNotes,
    UpdateNotes,
}

enum class ServiceStatus {
    Online,
    Offline,
    WaitingPairing,
    TokenError,
    Refreshing,
    ParseFailure,
}

enum class ServiceHealthStatus {
    Idle,
    Checking,
    Online,
    Offline,
}

enum class AgentMessageStreamStatus {
    Connecting,
    Live,
    Waiting,
    Disconnected,
}

data class AppUiState(
    val screen: AppScreen = AppScreen.Splash,
    val pairing: PairingUiState = PairingUiState(),
    val bootstrap: BootstrapUiState = BootstrapUiState(),
    val dashboard: DashboardUiState = DashboardUiState(),
    val settings: SettingsUiState = SettingsUiState(),
    val offline: OfflineUiState = OfflineUiState(),
    val screenshotUpload: ScreenshotUploadUiState = ScreenshotUploadUiState(),
)

data class BootstrapUiState(
    val title: String = "等待新的服务配置",
    val detailLabel: String = "新的服务地址和 token 会写入这块手表。",
    val currentHostLabel: String = "",
    val newHostLabel: String = "",
    val deviceNameLabel: String = "",
    val tokenFingerprint: String = "------",
    val warningLabel: String = "",
    val resultLabel: String = "",
    val canConfirm: Boolean = false,
    val isProcessing: Boolean = false,
)

data class ScreenshotUploadUiState(
    val visible: Boolean = false,
    val message: String = "",
    val inProgress: Boolean = false,
)

data class PairingUiState(
    val statusLabel: String = "等待手机扫码",
    val hintLabel: String = "请在手机端扫码配对",
    val serviceLabel: String = "等待服务连接",
    val serviceHostLabel: String = "",
    val serviceBaseUrl: String = "",
    val environmentLabel: String = "",
    val serviceColor: Color = Color(0xFFADB9CC),
    val tokenFingerprint: String = "------",
    val qrPayload: String = "",
    val bootstrapCode: String = "",
    val bootstrapDetailLabel: String = "",
    val scanStepLabel: String = "扫码",
    val scanStepCompleted: Boolean = false,
    val confirmStepLabel: String = "确认中",
    val confirmStepActive: Boolean = false,
    val authStepLabel: String = "已授权",
    val authStepCompleted: Boolean = false,
)

data class DashboardUiState(
    val pagerPage: DashboardPage = DashboardPage.Home,
    val serviceStatus: ServiceStatus = ServiceStatus.WaitingPairing,
    val serviceLabel: String = "等待配对",
    val serviceColor: Color = Color(0xFFFFC857),
    val updatedAtLabel: String = "尚未刷新",
    val serviceHostLabel: String = "",
    val syncStatusLabel: String = "等待数据",
    val isServiceDegraded: Boolean = false,
    val home: HomeDashboardUiState = HomeDashboardUiState(),
    val heatmap24h: Heatmap24hUiState = Heatmap24hUiState(),
    val sessionDetails: SessionDetailsUiState = SessionDetailsUiState(),
    val errors: List<String> = emptyList(),
)

data class HomeDashboardUiState(
    val fiveHour: QuotaRingUiState = QuotaRingUiState(title = "5h"),
    val weekly: QuotaRingUiState = QuotaRingUiState(title = "weekly"),
    val miniBars: List<MiniBarUiState> = List(24) { MiniBarUiState(hourLabel = it.toString(), intensity = 0f) },
    val weeklyHeatmap: HomeWeeklyHeatmapUiState = HomeWeeklyHeatmapUiState(),
    val selectedSessionTitle: String = "暂无活跃会话",
    val totalTokensLabel: String = "0",
    val selectedSessionModel: String = "--",
    val selectedSessionReasoning: String = "--",
    val selectedSessionContextLabel: String = "-- / --",
    val selectedSessionPressurePercent: Float = 0f,
    val selectedSessionCompactThresholdPercent: Float? = null,
    val selectedSessionCompactWarning: Boolean = false,
    val selectedSessionActiveLabel: String = "--",
    val selectedSessionIsActiveNow: Boolean = false,
    val selectedSessionRuntimePhaseLabel: String = "--",
    val sessionAvailable: Boolean = false,
    val isServiceDegraded: Boolean = false,
    val quotaEasterEgg: HomeQuotaEasterEggUiState = HomeQuotaEasterEggUiState(),
)

data class HomeQuotaEasterEggUiState(
    val visible: Boolean = false,
    val text: String? = null,
    val pool: HomeQuotaTipPool? = null,
)

data class Heatmap24hUiState(
    val totalTokensLabel: String = "0",
    val peakHourLabel: String = "--",
    val selectedHourRangeLabel: String = "--",
    val selectedHourTokensLabel: String = "0",
    val selectedIndex: Int = -1,
    val selectionCursorIndex: Float? = null,
    val bars: List<HeatmapBarUiState> = List(24) { HeatmapBarUiState(hourLabel = it.toString().padStart(2, '0'), intensity = 0f) },
    val segments: List<HeatmapSegmentUiState> = List(24) {
        HeatmapSegmentUiState(
            hourLabel = it.toString().padStart(2, '0'),
            timeRangeLabel = "--",
            intensity = 0f,
        )
    },
    val inputLabel: String = "0",
    val cachedInputLabel: String = "0",
    val outputLabel: String = "0",
    val cacheHitRateLabel: String = "0%",
    val dailyUsage: DailyUsageUiState = DailyUsageUiState(),
    val rotaryMode: HeatmapRotaryMode = HeatmapRotaryMode.HourRing,
    val emptyMessage: String = "暂无 24h 数据",
    val isServiceDegraded: Boolean = false,
)

enum class HeatmapRotaryMode {
    HourRing,
    Trend30d,
}

data class DailyUsageUiState(
    val totalTokensLabel: String = "0",
    val inputLabel: String = "0",
    val cachedInputLabel: String = "0",
    val outputLabel: String = "0",
    val cacheHitRateLabel: String = "0%",
    val reasoningOutputLabel: String = "0",
    val activeSessionsLabel: String = "0 会话",
    val estimatedValueLabel: String = "--",
    val valueCaption: String = "今日价值",
    val segments: List<DailyUsageBarSegmentUiState> = emptyList(),
    val modelShares: List<DailyUsageModelShareUiState> = emptyList(),
    val dailyTrend30d: DailyTrend30dUiState = DailyTrend30dUiState(),
)

data class DailyUsageBarSegmentUiState(
    val kind: DailyUsageSegmentKind,
    val fraction: Float,
)

enum class DailyUsageSegmentKind {
    Input,
    CachedInput,
    Output,
    ReasoningOutput,
}

data class DailyUsageModelShareUiState(
    val model: String,
    val shareLabel: String,
    val fraction: Float,
)

data class DailyTrend30dUiState(
    val available: Boolean = false,
    val totalLabel: String = "0",
    val averageLabel: String = "0",
    val dayFractions: List<Float> = emptyList(),
    val dayDates: List<String> = emptyList(),
    val dayLabels: List<String> = emptyList(),
    val dayTokenLabels: List<String> = emptyList(),
    val peakLabel: String = "0",
    val valueLabel: String = "--",
    val selectedIndex: Int = -1,
    val selectedDateLabel: String = "--",
    val selectedTokenLabel: String = "0",
    val tipVisible: Boolean = false,
)

data class SessionDetailsUiState(
    val selectedSessionTitle: String = "暂无活跃会话",
    val selectedSessionTitleMarquee: String = "暂无活跃会话",
    val selectedSessionModel: String = "--",
    val selectedSessionReasoning: String = "--",
    val selectedSessionActiveLabel: String = "--",
    val selectedSessionIsActiveNow: Boolean = false,
    val selectedSessionRuntimePhaseLabel: String = "--",
    val selectedSessionContextLabel: String = "-- / --",
    val selectedSessionPressurePercent: Float = 0f,
    val selectedSessionCompactThresholdPercent: Float? = null,
    val selectedSessionCompactWarning: Boolean = false,
    val selectedSessionTokensLabel: String = "0",
    val selectedIndex: Int = -1,
    val selectionCursorIndex: Float? = null,
    val rows: List<SessionRowUiState> = emptyList(),
    val segments: List<SessionSegmentUiState> = emptyList(),
    val sessionAvailable: Boolean = false,
    val emptyMessage: String = "暂无活跃会话",
    val latestAgentMessage: String? = null,
    val latestAgentMessageAtLabel: String? = null,
    val agentMessageStreamStatus: AgentMessageStreamStatus = AgentMessageStreamStatus.Disconnected,
    val agentMessageError: String? = null,
    val isServiceDegraded: Boolean = false,
)

data class QuotaRingUiState(
    val title: String,
    val usedPercent: Float = 0f,
    val remainingPercent: Float = 0f,
    val timeRemainingPercent: Float? = null,
    val remainingLabel: String = "--",
    val isDimmed: Boolean = false,
)

data class MiniBarUiState(
    val hourLabel: String,
    val intensity: Float,
)

data class HomeWeeklyHeatmapUiState(
    val available: Boolean = false,
    val rows: List<HomeWeeklyHeatmapRowUiState> = List(7) { HomeWeeklyHeatmapRowUiState() },
)

data class HomeWeeklyHeatmapRowUiState(
    val dateLabel: String = "",
    val cells: List<HomeWeeklyHeatmapCellUiState> = List(24) { HomeWeeklyHeatmapCellUiState() },
)

data class HomeWeeklyHeatmapCellUiState(
    val intensity: Float = 0f,
    val totalTokens: Long = 0L,
)

data class HeatmapSegmentUiState(
    val hourLabel: String,
    val timeRangeLabel: String,
    val intensity: Float,
    val totalTokensLabel: String = "0",
    val totalTokens: Long = 0L,
    val inputTokensLabel: String = "0",
    val cachedInputTokensLabel: String = "0",
    val outputTokensLabel: String = "0",
    val cacheHitRateLabel: String = "0%",
    val isPeak: Boolean = false,
    val isSelected: Boolean = false,
    val isNonEmpty: Boolean = false,
)

data class HeatmapBarUiState(
    val hourLabel: String,
    val intensity: Float,
    val isPeak: Boolean = false,
)

data class SessionSegmentUiState(
    val threadId: String,
    val activeLabel: String,
    val intensity: Float,
    val isSelected: Boolean = false,
)

data class SessionRowUiState(
    val sessionId: String,
    val title: String,
    val tokensLabel: String,
    val model: String,
    val reasoningLabel: String,
    val lastActiveLabel: String,
    val lastActiveAgoMinutes: Int = Int.MAX_VALUE,
    val runtimePhaseLabel: String = "--",
    val agentStatusLine: String = "--",
    val contextLabel: String = "-- / --",
    val contextPressurePercent: Float = 0f,
    val contextCompactThresholdPercent: Float? = null,
    val contextCompactWarning: Boolean = false,
    val isActiveNow: Boolean = false,
    val isSelected: Boolean = false,
)

data class SettingsUiState(
    val destination: SettingsDestination = SettingsDestination.Root,
    val baseUrl: String = "",
    val activeEndpointLabel: String = "",
    val savedEndpointCountLabel: String = "0 项",
    val savedEndpointSummary: String = "",
    val serviceTitle: String = "等待配对",
    val serviceSubtitle: String = "等待服务连接",
    val serviceColor: Color = Color(0xFFFFC857),
    val serviceHostLabel: String = "",
    val updatedAtLabel: String = "尚未刷新",
    val syncStatusLabel: String = "等待数据",
    val healthCheck: ServiceHealthUiState = ServiceHealthUiState(),
    val diagnosticUpload: DiagnosticUploadUiState = DiagnosticUploadUiState(),
    val diagnosticPrompt: DiagnosticPromptUiState = DiagnosticPromptUiState(),
    val debugToolsVisible: Boolean = false,
    val selectedScenario: DebugDemoScenario = DebugDemoScenario.NONE,
    val update: AppUpdateUiState = AppUpdateUiState(),
)

data class ServiceHealthUiState(
    val status: ServiceHealthStatus = ServiceHealthStatus.Idle,
    val resultLabel: String = "尚未检查",
    val detailLabel: String = "进入页面后自动检查",
    val errorLabel: String = "暂无",
    val resultColor: Color = Color(0xFFADB9CC),
)

data class DiagnosticUploadUiState(
    val entrySubtitle: String = "最近诊断：暂无",
    val entryEnabled: Boolean = true,
    val actionLabel: String = "上传诊断信息",
    val actionEnabled: Boolean = true,
    val statusLabel: String = "尚未上传",
    val hasPendingPackage: Boolean = false,
    val packageSizeLabel: String? = null,
    val progressLabel: String? = null,
    val speedLabel: String? = null,
    val lastDiagnosticId: String? = null,
    val lastDiagnosticAtLabel: String? = null,
    val lastUploadedAtLabel: String? = null,
    val progressOverlay: AppUpdateDownloadOverlayUiState = AppUpdateDownloadOverlayUiState(),
)

enum class DiagnosticPromptAction {
    UploadPending,
    ClearPending,
    ConfirmUpload,
}

data class DiagnosticPromptUiState(
    val visible: Boolean = false,
    val title: String = "",
    val message: String = "",
    val primaryLabel: String = "",
    val primaryAction: DiagnosticPromptAction? = null,
    val secondaryLabel: String? = null,
    val secondaryAction: DiagnosticPromptAction? = null,
)

enum class AppUpdateStatus {
    Idle,
    Checking,
    UpToDate,
    Available,
    Downloading,
    ReadyToInstall,
    PermissionRequired,
    Failed,
}

data class AppUpdateUiState(
    val status: AppUpdateStatus = AppUpdateStatus.Idle,
    val isExpanded: Boolean = false,
    val currentVersionLabel: String = "--",
    val channelLabel: String = "beta",
    val latestVersionLabel: String? = null,
    val detailLabel: String = "未检查更新",
    val comparisonLabel: String? = null,
    val progressPercent: Int? = null,
    val progressDetailLabel: String? = null,
    val downloadSpeedLabel: String? = null,
    val hasPendingUpdate: Boolean = false,
    val installPermissionEnabled: Boolean = false,
    val installPermissionLabel: String = "未允许",
    val autoCheckEnabled: Boolean = false,
    val currentVersionNotes: AppUpdateVersionNotesUiState = AppUpdateVersionNotesUiState(),
    val latestVersionNotes: AppUpdateVersionNotesUiState = AppUpdateVersionNotesUiState(),
    val downloadOverlay: AppUpdateDownloadOverlayUiState = AppUpdateDownloadOverlayUiState(),
)

data class AppUpdateVersionNotesUiState(
    val versionLabel: String = "--",
    val notes: List<AppUpdateNoteUiState> = emptyList(),
    val emptyLabel: String = "该版本暂未提供更新说明",
)

data class AppUpdateNoteUiState(
    val publishedAtLabel: String,
    val summary: String,
)

data class AppUpdateDownloadOverlayUiState(
    val visible: Boolean = false,
    val statusLabel: String = "",
    val fileSizeLabel: String = "待确认",
    val progressLabel: String = "",
    val transferredLabel: String = "",
    val speedLabel: String? = null,
)

data class OfflineUiState(
    val title: String = "服务不可达",
    val message: String = "无法连接到 OpenWatcher",
    val detailLabel: String = "请检查手机与服务地址的网络连接",
    val serviceHostLabel: String = "",
    val tokenFingerprint: String = "------",
    val qrPayload: String = "",
)

fun DebugDemoScenario.toDebugScenarioLabel(): String {
    return when (this) {
        DebugDemoScenario.NONE -> "关闭演示"
        DebugDemoScenario.DASHBOARD -> "演示仪表盘"
        DebugDemoScenario.QUOTA_STALE -> "演示额度缓存"
        DebugDemoScenario.UNAUTHORIZED -> "演示 401"
        DebugDemoScenario.OFFLINE -> "演示离线"
    }
}
