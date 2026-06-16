package ai.openwatcher.watchapp.data.diagnostics

import ai.openwatcher.watchapp.ui.AppUiState
import ai.openwatcher.watchapp.ui.ServiceHealthStatus

class DiagnosticSnapshotCollector(
    private val uiStateProvider: () -> AppUiState,
    private val eventLogger: DiagnosticEventLogger,
) {
    suspend fun capture(traceId: String = eventLogger.newTraceId("snapshot")) {
        val state = uiStateProvider()
        capturePairing(state, traceId)
        captureHomeDashboard(state, traceId)
        captureHeatmap24h(state, traceId)
        captureSessionDetails(state, traceId)
        captureSettingsRoot(state, traceId)
    }

    private suspend fun capturePairing(state: AppUiState, traceId: String) {
        eventLogger.log(
            event = "snapshot_pairing",
            traceId = traceId,
            fields = mapOf(
                "pairing" to mapOf(
                    "statusLabel" to state.pairing.statusLabel,
                    "hintLabel" to state.pairing.hintLabel,
                    "serviceLabel" to state.pairing.serviceLabel,
                    "serviceHostLabel" to state.pairing.serviceHostLabel,
                    "scanStepCompleted" to state.pairing.scanStepCompleted,
                    "confirmStepActive" to state.pairing.confirmStepActive,
                    "authStepCompleted" to state.pairing.authStepCompleted,
                ),
            ),
        )
    }

    private suspend fun captureHomeDashboard(state: AppUiState, traceId: String) {
        eventLogger.log(
            event = "snapshot_home_dashboard",
            traceId = traceId,
            fields = mapOf(
                "dashboard" to mapOf(
                    "serviceStatus" to state.dashboard.serviceStatus.name,
                    "serviceLabel" to state.dashboard.serviceLabel,
                    "updatedAtLabel" to state.dashboard.updatedAtLabel,
                    "syncStatusLabel" to state.dashboard.syncStatusLabel,
                    "isServiceDegraded" to state.dashboard.isServiceDegraded,
                    "errors" to state.dashboard.errors,
                    "quota" to mapOf(
                        "fiveHour" to mapOf(
                            "usedPercent" to state.dashboard.home.fiveHour.usedPercent,
                            "remainingPercent" to state.dashboard.home.fiveHour.remainingPercent,
                            "timeRemainingPercent" to state.dashboard.home.fiveHour.timeRemainingPercent,
                            "remainingLabel" to state.dashboard.home.fiveHour.remainingLabel,
                            "isDimmed" to state.dashboard.home.fiveHour.isDimmed,
                        ),
                        "weekly" to mapOf(
                            "usedPercent" to state.dashboard.home.weekly.usedPercent,
                            "remainingPercent" to state.dashboard.home.weekly.remainingPercent,
                            "timeRemainingPercent" to state.dashboard.home.weekly.timeRemainingPercent,
                            "remainingLabel" to state.dashboard.home.weekly.remainingLabel,
                            "isDimmed" to state.dashboard.home.weekly.isDimmed,
                        ),
                    ),
                    "selectedSession" to mapOf(
                        "model" to state.dashboard.home.selectedSessionModel,
                        "reasoning" to state.dashboard.home.selectedSessionReasoning,
                        "contextLabel" to state.dashboard.home.selectedSessionContextLabel,
                        "contextPressurePercent" to state.dashboard.home.selectedSessionPressurePercent,
                        "compactThresholdPercent" to state.dashboard.home.selectedSessionCompactThresholdPercent,
                        "compactWarning" to state.dashboard.home.selectedSessionCompactWarning,
                        "activeLabel" to state.dashboard.home.selectedSessionActiveLabel,
                        "isActiveNow" to state.dashboard.home.selectedSessionIsActiveNow,
                        "runtimePhaseLabel" to state.dashboard.home.selectedSessionRuntimePhaseLabel,
                        "sessionAvailable" to state.dashboard.home.sessionAvailable,
                        "totalTokensLabel" to state.dashboard.home.totalTokensLabel,
                    ),
                    "miniBars" to state.dashboard.home.miniBars.map { bar ->
                        mapOf(
                            "hourLabel" to bar.hourLabel,
                            "intensity" to bar.intensity,
                        )
                    },
                    "weeklyHeatmap" to mapOf(
                        "available" to state.dashboard.home.weeklyHeatmap.available,
                        "rows" to state.dashboard.home.weeklyHeatmap.rows.map { row ->
                            mapOf(
                                "dateLabel" to row.dateLabel,
                                "cells" to row.cells.map { cell ->
                                    mapOf(
                                        "intensity" to cell.intensity,
                                        "totalTokens" to cell.totalTokens,
                                    )
                                },
                            )
                        },
                    ),
                    "quotaEasterEgg" to mapOf(
                        "visible" to state.dashboard.home.quotaEasterEgg.visible,
                        "text" to state.dashboard.home.quotaEasterEgg.text,
                        "pool" to state.dashboard.home.quotaEasterEgg.pool?.name,
                    ),
                ),
            ),
        )
    }

    private suspend fun captureHeatmap24h(state: AppUiState, traceId: String) {
        eventLogger.log(
            event = "snapshot_heatmap24h",
            traceId = traceId,
            fields = mapOf(
                "heatmap24h" to mapOf(
                    "totalTokensLabel" to state.dashboard.heatmap24h.totalTokensLabel,
                    "peakHourLabel" to state.dashboard.heatmap24h.peakHourLabel,
                    "selectedHourRangeLabel" to state.dashboard.heatmap24h.selectedHourRangeLabel,
                    "selectedHourTokensLabel" to state.dashboard.heatmap24h.selectedHourTokensLabel,
                    "selectedIndex" to state.dashboard.heatmap24h.selectedIndex,
                    "selectionCursorIndex" to state.dashboard.heatmap24h.selectionCursorIndex,
                    "rotaryMode" to state.dashboard.heatmap24h.rotaryMode.name,
                    "emptyMessage" to state.dashboard.heatmap24h.emptyMessage,
                    "isServiceDegraded" to state.dashboard.heatmap24h.isServiceDegraded,
                    "inputLabel" to state.dashboard.heatmap24h.inputLabel,
                    "cachedInputLabel" to state.dashboard.heatmap24h.cachedInputLabel,
                    "outputLabel" to state.dashboard.heatmap24h.outputLabel,
                    "cacheHitRateLabel" to state.dashboard.heatmap24h.cacheHitRateLabel,
                    "bars" to state.dashboard.heatmap24h.bars.map { bar ->
                        mapOf(
                            "hourLabel" to bar.hourLabel,
                            "intensity" to bar.intensity,
                            "isPeak" to bar.isPeak,
                        )
                    },
                    "segments" to state.dashboard.heatmap24h.segments.map { segment ->
                        mapOf(
                            "hourLabel" to segment.hourLabel,
                            "timeRangeLabel" to segment.timeRangeLabel,
                            "intensity" to segment.intensity,
                            "totalTokensLabel" to segment.totalTokensLabel,
                            "totalTokens" to segment.totalTokens,
                            "inputTokensLabel" to segment.inputTokensLabel,
                            "cachedInputTokensLabel" to segment.cachedInputTokensLabel,
                            "outputTokensLabel" to segment.outputTokensLabel,
                            "cacheHitRateLabel" to segment.cacheHitRateLabel,
                            "isPeak" to segment.isPeak,
                            "isSelected" to segment.isSelected,
                            "isNonEmpty" to segment.isNonEmpty,
                        )
                    },
                    "dailyUsage" to mapOf(
                        "totalTokensLabel" to state.dashboard.heatmap24h.dailyUsage.totalTokensLabel,
                        "inputLabel" to state.dashboard.heatmap24h.dailyUsage.inputLabel,
                        "cachedInputLabel" to state.dashboard.heatmap24h.dailyUsage.cachedInputLabel,
                        "outputLabel" to state.dashboard.heatmap24h.dailyUsage.outputLabel,
                        "reasoningOutputLabel" to state.dashboard.heatmap24h.dailyUsage.reasoningOutputLabel,
                        "cacheHitRateLabel" to state.dashboard.heatmap24h.dailyUsage.cacheHitRateLabel,
                        "activeSessionsLabel" to state.dashboard.heatmap24h.dailyUsage.activeSessionsLabel,
                        "estimatedValueLabel" to state.dashboard.heatmap24h.dailyUsage.estimatedValueLabel,
                        "valueCaption" to state.dashboard.heatmap24h.dailyUsage.valueCaption,
                        "segments" to state.dashboard.heatmap24h.dailyUsage.segments.map { segment ->
                            mapOf(
                                "kind" to segment.kind.name,
                                "fraction" to segment.fraction,
                            )
                        },
                        "modelShares" to state.dashboard.heatmap24h.dailyUsage.modelShares.map { share ->
                            mapOf(
                                "model" to share.model,
                                "shareLabel" to share.shareLabel,
                                "fraction" to share.fraction,
                            )
                        },
                        "dailyTrend30d" to mapOf(
                            "available" to state.dashboard.heatmap24h.dailyUsage.dailyTrend30d.available,
                            "totalLabel" to state.dashboard.heatmap24h.dailyUsage.dailyTrend30d.totalLabel,
                            "averageLabel" to state.dashboard.heatmap24h.dailyUsage.dailyTrend30d.averageLabel,
                            "dayFractions" to state.dashboard.heatmap24h.dailyUsage.dailyTrend30d.dayFractions,
                            "dayDates" to state.dashboard.heatmap24h.dailyUsage.dailyTrend30d.dayDates,
                            "dayLabels" to state.dashboard.heatmap24h.dailyUsage.dailyTrend30d.dayLabels,
                            "dayTokenLabels" to state.dashboard.heatmap24h.dailyUsage.dailyTrend30d.dayTokenLabels,
                            "peakLabel" to state.dashboard.heatmap24h.dailyUsage.dailyTrend30d.peakLabel,
                            "valueLabel" to state.dashboard.heatmap24h.dailyUsage.dailyTrend30d.valueLabel,
                            "selectedIndex" to state.dashboard.heatmap24h.dailyUsage.dailyTrend30d.selectedIndex,
                            "selectedDateLabel" to state.dashboard.heatmap24h.dailyUsage.dailyTrend30d.selectedDateLabel,
                            "selectedTokenLabel" to state.dashboard.heatmap24h.dailyUsage.dailyTrend30d.selectedTokenLabel,
                            "tipVisible" to state.dashboard.heatmap24h.dailyUsage.dailyTrend30d.tipVisible,
                        ),
                    ),
                ),
            ),
        )
    }

    private suspend fun captureSessionDetails(state: AppUiState, traceId: String) {
        eventLogger.log(
            event = "snapshot_session_details",
            traceId = traceId,
            fields = mapOf(
                "sessionDetails" to mapOf(
                    "selectedSessionId" to state.dashboard.sessionDetails.rows.firstOrNull { it.isSelected }?.sessionId,
                    "selectedSessionModel" to state.dashboard.sessionDetails.selectedSessionModel,
                    "selectedSessionReasoning" to state.dashboard.sessionDetails.selectedSessionReasoning,
                    "selectedSessionActiveLabel" to state.dashboard.sessionDetails.selectedSessionActiveLabel,
                    "selectedSessionIsActiveNow" to state.dashboard.sessionDetails.selectedSessionIsActiveNow,
                    "selectedSessionRuntimePhaseLabel" to state.dashboard.sessionDetails.selectedSessionRuntimePhaseLabel,
                    "selectedSessionContextLabel" to state.dashboard.sessionDetails.selectedSessionContextLabel,
                    "selectedSessionPressurePercent" to state.dashboard.sessionDetails.selectedSessionPressurePercent,
                    "selectedSessionCompactThresholdPercent" to state.dashboard.sessionDetails.selectedSessionCompactThresholdPercent,
                    "selectedSessionCompactWarning" to state.dashboard.sessionDetails.selectedSessionCompactWarning,
                    "selectedSessionTokensLabel" to state.dashboard.sessionDetails.selectedSessionTokensLabel,
                    "selectedIndex" to state.dashboard.sessionDetails.selectedIndex,
                    "selectionCursorIndex" to state.dashboard.sessionDetails.selectionCursorIndex,
                    "agentMessageStreamStatus" to state.dashboard.sessionDetails.agentMessageStreamStatus.name,
                    "agentMessageError" to state.dashboard.sessionDetails.agentMessageError,
                    "sessionAvailable" to state.dashboard.sessionDetails.sessionAvailable,
                    "emptyMessage" to state.dashboard.sessionDetails.emptyMessage,
                    "rows" to state.dashboard.sessionDetails.rows.map { row ->
                        mapOf(
                            "threadId" to row.sessionId,
                            "model" to row.model,
                            "reasoningLabel" to row.reasoningLabel,
                            "lastActiveLabel" to row.lastActiveLabel,
                            "lastActiveAgoMinutes" to row.lastActiveAgoMinutes,
                            "runtimePhaseLabel" to row.runtimePhaseLabel,
                            "contextLabel" to row.contextLabel,
                            "contextPressurePercent" to row.contextPressurePercent,
                            "contextCompactThresholdPercent" to row.contextCompactThresholdPercent,
                            "contextCompactWarning" to row.contextCompactWarning,
                            "isActiveNow" to row.isActiveNow,
                            "isSelected" to row.isSelected,
                            "tokensLabel" to row.tokensLabel,
                        )
                    },
                    "segments" to state.dashboard.sessionDetails.segments.map { segment ->
                        mapOf(
                            "threadId" to segment.threadId,
                            "activeLabel" to segment.activeLabel,
                            "intensity" to segment.intensity,
                            "isSelected" to segment.isSelected,
                        )
                    },
                ),
            ),
        )
    }

    private suspend fun captureSettingsRoot(state: AppUiState, traceId: String) {
        eventLogger.log(
            event = "snapshot_settings_root",
            traceId = traceId,
            fields = mapOf(
                "settings" to mapOf(
                    "destination" to state.settings.destination.name,
                    "serviceTitle" to state.settings.serviceTitle,
                    "serviceSubtitle" to state.settings.serviceSubtitle,
                    "serviceHostLabel" to state.settings.serviceHostLabel,
                    "updatedAtLabel" to state.settings.updatedAtLabel,
                    "syncStatusLabel" to state.settings.syncStatusLabel,
                    "healthCheck" to mapOf(
                        "status" to state.settings.healthCheck.status.name,
                        "resultLabel" to state.settings.healthCheck.resultLabel,
                        "detailLabel" to state.settings.healthCheck.detailLabel,
                        "errorLabel" to state.settings.healthCheck.errorLabel.takeUnless { it == "暂无" },
                        "isChecking" to (state.settings.healthCheck.status == ServiceHealthStatus.Checking),
                    ),
                    "diagnosticUpload" to mapOf(
                        "actionLabel" to state.settings.diagnosticUpload.actionLabel,
                        "actionEnabled" to state.settings.diagnosticUpload.actionEnabled,
                        "statusLabel" to state.settings.diagnosticUpload.statusLabel,
                        "hasPendingPackage" to state.settings.diagnosticUpload.hasPendingPackage,
                        "packageSizeLabel" to state.settings.diagnosticUpload.packageSizeLabel,
                        "progressLabel" to state.settings.diagnosticUpload.progressLabel,
                        "speedLabel" to state.settings.diagnosticUpload.speedLabel,
                        "lastDiagnosticId" to state.settings.diagnosticUpload.lastDiagnosticId,
                        "lastUploadedAtLabel" to state.settings.diagnosticUpload.lastUploadedAtLabel,
                    ),
                ),
            ),
        )
    }
}
