package ai.openwatcher.watchapp.data.diagnostics

import java.nio.file.Files
import java.time.Instant
import java.time.ZoneId
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import ai.openwatcher.watchapp.ui.AgentMessageStreamStatus
import ai.openwatcher.watchapp.ui.AppScreen
import ai.openwatcher.watchapp.ui.AppUiState
import ai.openwatcher.watchapp.ui.DailyTrend30dUiState
import ai.openwatcher.watchapp.ui.DailyUsageBarSegmentUiState
import ai.openwatcher.watchapp.ui.DailyUsageModelShareUiState
import ai.openwatcher.watchapp.ui.DailyUsageSegmentKind
import ai.openwatcher.watchapp.ui.DailyUsageUiState
import ai.openwatcher.watchapp.ui.DashboardUiState
import ai.openwatcher.watchapp.ui.Heatmap24hUiState
import ai.openwatcher.watchapp.ui.HeatmapBarUiState
import ai.openwatcher.watchapp.ui.HeatmapSegmentUiState
import ai.openwatcher.watchapp.ui.HomeDashboardUiState
import ai.openwatcher.watchapp.ui.HomeWeeklyHeatmapCellUiState
import ai.openwatcher.watchapp.ui.HomeWeeklyHeatmapRowUiState
import ai.openwatcher.watchapp.ui.HomeWeeklyHeatmapUiState
import ai.openwatcher.watchapp.ui.MiniBarUiState
import ai.openwatcher.watchapp.ui.PairingUiState
import ai.openwatcher.watchapp.ui.SessionDetailsUiState
import ai.openwatcher.watchapp.ui.SessionSegmentUiState
import ai.openwatcher.watchapp.ui.SessionRowUiState

class DiagnosticSnapshotCollectorTest {
    @Test
    fun capture_omitsLatestAgentMessageAndQrPayload() = runTest {
        val uiStateHolder = DiagnosticUiStateHolder(
            AppUiState(
                screen = AppScreen.SessionDetails,
                pairing = PairingUiState(
                    qrPayload = "token=plain-secret",
                    tokenFingerprint = "abc123",
                ),
                dashboard = DashboardUiState(
                    home = HomeDashboardUiState(
                        miniBars = listOf(MiniBarUiState(hourLabel = "00", intensity = 0.75f)),
                        weeklyHeatmap = HomeWeeklyHeatmapUiState(
                            available = true,
                            rows = listOf(
                                HomeWeeklyHeatmapRowUiState(
                                    dateLabel = "06-06",
                                    cells = listOf(HomeWeeklyHeatmapCellUiState(intensity = 0.5f, totalTokens = 128)),
                                ),
                            ),
                        ),
                    ),
                    heatmap24h = Heatmap24hUiState(
                        bars = listOf(HeatmapBarUiState(hourLabel = "00", intensity = 0.6f, isPeak = true)),
                        segments = listOf(
                            HeatmapSegmentUiState(
                                hourLabel = "00",
                                timeRangeLabel = "00:00-01:00",
                                intensity = 0.6f,
                                totalTokensLabel = "128K",
                                totalTokens = 128_000,
                                isPeak = true,
                                isSelected = true,
                                isNonEmpty = true,
                            ),
                        ),
                        dailyUsage = DailyUsageUiState(
                            segments = listOf(DailyUsageBarSegmentUiState(DailyUsageSegmentKind.Input, 0.4f)),
                            modelShares = listOf(DailyUsageModelShareUiState(model = "gpt-5.5", shareLabel = "80%", fraction = 0.8f)),
                            dailyTrend30d = DailyTrend30dUiState(
                                available = true,
                                dayDates = listOf("2026-06-06"),
                                dayFractions = listOf(0.5f),
                                dayLabels = listOf("06-06"),
                                dayTokenLabels = listOf("128K"),
                            ),
                        ),
                    ),
                    sessionDetails = SessionDetailsUiState(
                        selectedSessionModel = "gpt-5.5",
                        latestAgentMessage = "这里不该出现在诊断里",
                        latestAgentMessageAtLabel = "12:34",
                        agentMessageStreamStatus = AgentMessageStreamStatus.Live,
                        segments = listOf(
                            SessionSegmentUiState(
                                threadId = "session-1",
                                activeLabel = "1m",
                                intensity = 0.9f,
                                isSelected = true,
                            ),
                        ),
                        rows = listOf(
                            SessionRowUiState(
                                sessionId = "session-1",
                                title = "不该出现的标题",
                                tokensLabel = "18K",
                                model = "gpt-5.5",
                                reasoningLabel = "High",
                                lastActiveLabel = "1m",
                            ),
                        ),
                    ),
                ),
            ),
        )
        val eventStore = DiagnosticEventStore(
            directory = Files.createTempDirectory("snapshot-events").toFile(),
            zoneId = ZoneId.of("UTC"),
            clock = { Instant.parse("2026-06-06T01:10:00Z") },
        )
        val logger = StructuredDiagnosticEventLogger(
            store = eventStore,
            runtimeContext = DiagnosticRuntimeContext(baseUrl = "https://watcher.example"),
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
            clock = { Instant.parse("2026-06-06T01:10:00Z") },
        )

        DiagnosticSnapshotCollector(
            uiStateProvider = uiStateHolder::current,
            eventLogger = logger,
        ).capture("trace-snapshot")

        val lines = eventStore.readRecentLines(hours = 24, now = Instant.parse("2026-06-06T01:10:00Z"))
        val merged = lines.joinToString("\n")
        assertTrue(merged.contains("snapshot_session_details"))
        assertFalse(merged.contains("这里不该出现在诊断里"))
        assertFalse(merged.contains("token=plain-secret"))
        assertFalse(merged.contains("不该出现的标题"))
        assertTrue(merged.contains("session-1"))
        assertTrue(merged.contains("miniBars"))
        assertTrue(merged.contains("weeklyHeatmap"))
        assertTrue(merged.contains("totalTokens"))
        assertTrue(merged.contains("segments"))
    }
}
