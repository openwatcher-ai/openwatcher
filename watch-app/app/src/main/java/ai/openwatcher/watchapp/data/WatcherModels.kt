package ai.openwatcher.watchapp.data

import java.time.Instant

data class WatcherStatusSnapshot(
    val observedAt: Instant?,
    val quota: QuotaSnapshot?,
    val heatmap24h: Heatmap24hSnapshot?,
    val heatmap7d: Heatmap7dSnapshot?,
    val dailyUsage: DailyUsageSnapshot?,
    val dailyTrend30d: DailyTrend30dSnapshot?,
    val sessions: List<SessionSnapshot>,
    val errors: List<String>,
)

data class DailyUsageSnapshot(
    val generatedAt: Instant?,
    val totalTokens: Long,
    val inputTokens: Long,
    val cachedInputTokens: Long,
    val outputTokens: Long,
    val reasoningOutputTokens: Long,
    val activeSessions: Int,
    val estimatedValueUsd: Double?,
    val estimatedValueLabel: String?,
    val pricingDate: String?,
    val pricingSourceUrl: String?,
    val pricingUnavailableReason: String?,
    val modelShares: List<DailyUsageModelShare>,
)

data class DailyUsageModelShare(
    val model: String,
    val tokens: Long,
    val sharePercent: Double,
)

data class DailyTrend30dSnapshot(
    val timezone: String,
    val generatedAt: Instant?,
    val startDate: String,
    val endDate: String,
    val totalTokens: Long,
    val averageTokens: Long,
    val peakTokens: Long,
    val estimatedValueUsd: Double?,
    val estimatedValueLabel: String?,
    val days: List<DailyTrendDay>,
)

data class DailyTrendDay(
    val date: String,
    val totalTokens: Long,
)

data class QuotaSnapshot(
    val source: String,
    val fresh: Boolean,
    val status: QuotaStatus,
    val planType: String?,
    val fiveHour: QuotaWindow?,
    val weekly: QuotaWindow?,
)

enum class QuotaStatus {
    Ok,
    Stale,
    Unavailable,
}

data class QuotaWindow(
    val usedPercent: Float,
    val remainingPercent: Float,
    val resetAt: Instant?,
)

data class Heatmap24hSnapshot(
    val timezone: String,
    val generatedAt: Instant?,
    val peakHourStart: Instant?,
    val buckets: List<HeatmapBucket>,
)

data class HeatmapBucket(
    val hourStart: Instant?,
    val inputTokens: Long,
    val cachedInputTokens: Long,
    val outputTokens: Long,
    val reasoningOutputTokens: Long,
    val totalTokens: Long,
    val activeThreads: Int,
)

data class Heatmap7dSnapshot(
    val timezone: String,
    val generatedAt: Instant?,
    val startDate: String,
    val endDate: String,
    val peakTokens: Long,
    val days: List<Heatmap7dDay>,
)

data class Heatmap7dDay(
    val date: String,
    val totalTokens: Long,
    val hours: List<Long>,
)

data class SessionSnapshot(
    val threadId: String,
    val title: String,
    val updatedAt: Instant?,
    val model: String,
    val reasoningEffort: String,
    val tokensUsedTotal: Long,
    val contextUsedTokens: Long,
    val contextWindow: Long,
    val contextPressurePercent: Int,
    val contextCompactThresholdTokens: Long?,
    val contextCompactThresholdPercent: Int?,
    val contextCompaction: ContextCompactionSnapshot? = null,
    val lastActiveAgoMinutes: Int,
)

data class ContextCompactionSnapshot(
    val trigger: String,
    val startedAt: Instant?,
    val updatedAt: Instant?,
    val turnId: String?,
)

data class SessionAgentMessage(
    val threadId: String,
    val eventId: String,
    val createdAt: Instant?,
    val text: String,
    val truncated: Boolean,
)

data class SessionRuntimeState(
    val threadId: String,
    val turnId: String?,
    val startedAt: Instant?,
    val running: Boolean,
    val lifecycle: SessionRuntimeLifecycle,
    val phase: SessionRuntimePhase,
    val updatedAt: Instant?,
    val sequence: Long,
)

enum class SessionRuntimeLifecycle {
    Idle,
    Running,
    Completed,
    Aborted,
}

enum class SessionRuntimePhase {
    Unknown,
    Reasoning,
    ToolRunning,
    AgentCommentary,
    AgentFinal,
}

sealed interface StatusFetchResult {
    data class Success(val snapshot: WatcherStatusSnapshot) : StatusFetchResult
    data object Unauthorized : StatusFetchResult
    data class HttpFailure(val code: Int, val message: String) : StatusFetchResult
    data class NetworkFailure(val message: String) : StatusFetchResult
    data class ParseFailure(val message: String) : StatusFetchResult
}

enum class StatusStreamFailureReason {
    Unauthorized,
    HttpError,
    NetworkError,
    ParseError,
    ServerError,
    StreamClosed,
}

sealed interface StatusStreamEvent {
    data class Snapshot(val snapshot: WatcherStatusSnapshot) : StatusStreamEvent
    data class Quota(val observedAt: Instant?, val quota: QuotaSnapshot?) : StatusStreamEvent
    data class Heatmap24h(
        val observedAt: Instant?,
        val heatmap24h: Heatmap24hSnapshot?,
        val heatmap7d: Heatmap7dSnapshot?,
        val dailyUsage: DailyUsageSnapshot?,
    ) : StatusStreamEvent
    data class Sessions(val observedAt: Instant?, val sessions: List<SessionSnapshot>) : StatusStreamEvent
    data class Errors(val observedAt: Instant?, val errors: List<String>) : StatusStreamEvent
    data object Heartbeat : StatusStreamEvent
    data class Failure(
        val message: String,
        val reason: StatusStreamFailureReason,
        val retryable: Boolean,
        val terminal: Boolean,
        val detail: String? = null,
        val statusCode: Int? = null,
    ) : StatusStreamEvent
}

sealed interface HealthCheckResult {
    data object Online : HealthCheckResult
    data class Offline(val message: String) : HealthCheckResult
}

sealed interface ScreenshotUploadResult {
    data class Success(val filename: String) : ScreenshotUploadResult
    data object Unauthorized : ScreenshotUploadResult
    data class HttpFailure(val code: Int, val message: String) : ScreenshotUploadResult
    data class NetworkFailure(val message: String) : ScreenshotUploadResult
}

sealed interface DiagnosticUploadResult {
    data class Success(
        val diagnosticId: String,
        val receivedAt: Instant,
    ) : DiagnosticUploadResult

    data object Unauthorized : DiagnosticUploadResult
    data class HttpFailure(val code: Int, val message: String) : DiagnosticUploadResult
    data class NetworkFailure(val message: String) : DiagnosticUploadResult
}

enum class SessionStreamFailureReason {
    Unauthorized,
    HttpError,
    NetworkError,
    ParseError,
    ServerError,
    StreamClosed,
}

sealed interface SessionStreamEvent {
    data class AgentMessage(val message: SessionAgentMessage) : SessionStreamEvent
    data class RuntimeState(val state: SessionRuntimeState) : SessionStreamEvent
    data object Heartbeat : SessionStreamEvent
    data class Failure(
        val message: String,
        val reason: SessionStreamFailureReason,
        val retryable: Boolean,
        val terminal: Boolean,
        val detail: String? = null,
        val statusCode: Int? = null,
    ) : SessionStreamEvent
}

data class SessionWindowSnapshot(
    val observedAt: Instant?,
    val limit: Int,
    val threadOrder: List<String>,
    val sessions: List<SessionWindowEntry>,
)

data class SessionWindowEntry(
    val session: SessionSnapshot,
    val runtimeState: SessionRuntimeState,
    val latestAgentMessage: SessionAgentMessage?,
)

sealed interface SessionWindowStreamEvent {
    data class Window(val window: SessionWindowSnapshot) : SessionWindowStreamEvent
    data class AgentMessage(val message: SessionAgentMessage) : SessionWindowStreamEvent
    data class RuntimeState(val state: SessionRuntimeState) : SessionWindowStreamEvent
    data object Heartbeat : SessionWindowStreamEvent
    data class Failure(
        val message: String,
        val reason: SessionStreamFailureReason,
        val retryable: Boolean,
        val terminal: Boolean,
        val detail: String? = null,
        val statusCode: Int? = null,
    ) : SessionWindowStreamEvent
}

enum class SessionStreamClientEventType {
    Disconnect,
    ReconnectSuccess,
}

data class SessionStreamClientEventReport(
    val eventType: SessionStreamClientEventType,
    val threadId: String,
    val deviceName: String,
    val appVersion: String,
    val reconnectAttempt: Int = 0,
    val reason: SessionStreamFailureReason? = null,
    val detail: String? = null,
    val statusCode: Int? = null,
    val retryable: Boolean? = null,
    val connectedMs: Long? = null,
    val nextRetryDelayMs: Long? = null,
    val receivedAgentMessage: Boolean = false,
    val firstEventType: String? = null,
)

enum class DebugDemoScenario {
    NONE,
    DASHBOARD,
    QUOTA_STALE,
    UNAUTHORIZED,
    OFFLINE,
}
