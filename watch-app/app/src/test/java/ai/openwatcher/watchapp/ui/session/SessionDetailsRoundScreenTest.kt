package ai.openwatcher.watchapp.ui.session

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.unit.IntSize
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Test
import ai.openwatcher.watchapp.ScreenshotLongPressTimeoutMs
import ai.openwatcher.watchapp.R
import ai.openwatcher.watchapp.ui.SessionDetailsUiState
import ai.openwatcher.watchapp.ui.SessionRowUiState
import ai.openwatcher.watchapp.ui.home.homeSessionTitleBaselineInsetScale
import ai.openwatcher.watchapp.ui.home.homeSessionTitleTextScale

class SessionDetailsRoundScreenTest {
    @Test
    fun toRoundUiModel_preservesCompactRecentLabels() {
        val state = SessionDetailsUiState(
            rows = listOf(
                sessionRow(
                    sessionId = "minute",
                    title = "Minute",
                    lastActiveLabel = "59m",
                    lastActiveAgoMinutes = 59,
                    isSelected = true,
                ),
                sessionRow(
                    sessionId = "hour",
                    title = "Hour",
                    lastActiveLabel = "2h",
                    lastActiveAgoMinutes = 120,
                ),
                sessionRow(
                    sessionId = "day",
                    title = "Day",
                    lastActiveLabel = "3d",
                    lastActiveAgoMinutes = 4_320,
                ),
                sessionRow(
                    sessionId = "week",
                    title = "Week",
                    lastActiveLabel = "4w",
                    lastActiveAgoMinutes = 40_320,
                ),
            ),
        )

        val model = state.toRoundUiModel()

        assertEquals(listOf("59m", "2h", "3d", "4w"), model.segments.map { it.activeLabel })
    }

    @Test
    fun toRoundUiModel_preservesRowOrderForSegments() {
        val state = SessionDetailsUiState(
            selectedIndex = 1,
            rows = listOf(
                sessionRow(
                    sessionId = "day",
                    title = "Day",
                    lastActiveLabel = "2d",
                    lastActiveAgoMinutes = 2_880,
                ),
                sessionRow(
                    sessionId = "now",
                    title = "Now",
                    lastActiveLabel = "1m",
                    lastActiveAgoMinutes = 0,
                    isSelected = true,
                ),
                sessionRow(
                    sessionId = "hour",
                    title = "Hour",
                    lastActiveLabel = "2h",
                    lastActiveAgoMinutes = 120,
                ),
            ),
        )

        val model = state.toRoundUiModel()

        assertEquals(listOf("Day", "Now", "Hour"), model.segments.map { it.title })
        assertEquals(1, model.selectedIndex)
    }

    @Test
    fun toRoundUiModel_usesRelativeActivityRanksWithinPreservedOrder() {
        val state = SessionDetailsUiState(
            rows = listOf(
                sessionRow(
                    sessionId = "stale-big",
                    title = "Stale Big",
                    lastActiveLabel = "2h",
                    lastActiveAgoMinutes = 120,
                    tokensLabel = "900m",
                ),
                sessionRow(
                    sessionId = "fresh-small",
                    title = "Fresh Small",
                    lastActiveLabel = "1m",
                    lastActiveAgoMinutes = 1,
                    tokensLabel = "1k",
                    isSelected = true,
                ),
                sessionRow(
                    sessionId = "old",
                    title = "Old",
                    lastActiveLabel = "2d",
                    lastActiveAgoMinutes = 2_880,
                    tokensLabel = "500m",
                ),
            ),
        )

        val model = state.toRoundUiModel()

        assertEquals(listOf("Stale Big", "Fresh Small", "Old"), model.segments.map { it.title })
        assertEquals(listOf(0, 1, 2), model.segments.map { it.activityRank })
    }

    @Test
    fun toRoundUiModel_assignsIdleRanksRelativeToOtherIdleSessions() {
        val state = SessionDetailsUiState(
            rows = listOf(
                sessionRow(
                    sessionId = "idle-1",
                    title = "Idle 1",
                    lastActiveLabel = "1m",
                    lastActiveAgoMinutes = 1,
                    isSelected = true,
                ),
                sessionRow(
                    sessionId = "idle-2",
                    title = "Idle 2",
                    lastActiveLabel = "2m",
                    lastActiveAgoMinutes = 2,
                ),
            ),
        )

        val model = state.toRoundUiModel()

        assertEquals(listOf("Idle 1", "Idle 2"), model.segments.map { it.title })
        assertEquals(listOf(0, 1), model.segments.map { it.activityRank })
    }

    @Test
    fun toRoundUiModel_ranksRunningAndIdleSegmentsSeparately() {
        val state = SessionDetailsUiState(
            rows = listOf(
                sessionRow(
                    sessionId = "idle-recent",
                    title = "Idle Recent",
                    lastActiveLabel = "1m",
                    lastActiveAgoMinutes = 1,
                    isSelected = true,
                ),
                sessionRow(
                    sessionId = "running-older",
                    title = "Running Older",
                    lastActiveLabel = "12m",
                    lastActiveAgoMinutes = 12,
                    isActiveNow = true,
                ),
                sessionRow(
                    sessionId = "idle-older",
                    title = "Idle Older",
                    lastActiveLabel = "13m",
                    lastActiveAgoMinutes = 13,
                ),
            ),
        )

        val model = state.toRoundUiModel()

        assertEquals(listOf("Idle Recent", "Running Older", "Idle Older"), model.segments.map { it.title })
        assertEquals(listOf(false, true, false), model.segments.map { it.isActiveNow })
        assertEquals(listOf(0, 0, 1), model.segments.map { it.activityRank })
    }

    @Test
    fun sessionActivityRankForSortedIndex_usesFiveRelativeLevels() {
        assertEquals(0, sessionActivityRankForSortedIndex(0))
        assertEquals(1, sessionActivityRankForSortedIndex(1))
        assertEquals(2, sessionActivityRankForSortedIndex(2))
        assertEquals(3, sessionActivityRankForSortedIndex(3))
        assertEquals(4, sessionActivityRankForSortedIndex(4))
        assertEquals(4, sessionActivityRankForSortedIndex(9))
    }

    @Test
    fun sessionActivityColor_usesWarmOnlyForRunningSessions() {
        assertEquals(Color(0xFFFF4D2E), sessionActivityColor(isActiveNow = true, rank = 0))
        assertEquals(Color(0xFF38BFE4), sessionActivityColor(isActiveNow = false, rank = 0))
        assertEquals(Color(0xFFA3AFBA), sessionActivityColor(isActiveNow = false, rank = 4))
    }

    @Test
    fun sessionCursorAccentSweep_isShorterThanFullSegment() {
        assertEquals(18f, sessionCursorAccentSweep(68.5f), 0.0001f)
        assertTrue(sessionCursorAccentSweep(68.5f) < 68.5f)
    }

    @Test
    fun sessionCursorAccentPosition_returnsBasePositionOutsideConnecting() {
        assertEquals(2f, sessionCursorAccentPosition(2f, 0f, 5), 0.0001f)
    }

    @Test
    fun sessionCursorAccentPosition_wrapsConnectingOffsetAroundRing() {
        assertEquals(4.5f, sessionCursorAccentPosition(2f, 2.5f, 5), 0.0001f)
        assertEquals(2.5f, sessionCursorAccentPosition(2f, 5.5f, 5), 0.0001f)
    }

    @Test
    fun sessionCursorAccentPosition_handlesEmptyRing() {
        assertEquals(0f, sessionCursorAccentPosition(2f, 1f, 0), 0.0001f)
    }

    @Test
    fun sessionHeaderPrimaryLabel_usesTokenOnly() {
        assertEquals("12k", sessionHeaderPrimaryLabel("12k"))
        assertEquals("--", sessionHeaderPrimaryLabel(""))
    }

    @Test
    fun sessionFooterTokenLabel_usesContextOnly() {
        assertEquals("249.8K/920K", sessionFooterTokenLabel("249.8K/920K"))
        assertEquals("--", sessionFooterTokenLabel(""))
    }

    @Test
    fun sessionMetadataIcons_matchHomeDrawables() {
        assertEquals(R.drawable.ic_token_burn, SessionMetadataArcIcon.Token.drawableRes)
        assertEquals(R.drawable.ic_reasoning_brain, SessionMetadataArcIcon.Reasoning.drawableRes)
        assertEquals(R.drawable.ic_model_cube, SessionMetadataArcIcon.Model.drawableRes)
    }

    @Test
    fun sessionTitleScale_matchesHomeTitleScale() {
        assertEquals(0.0525f, homeSessionTitleTextScale(), 0.0001f)
        assertEquals(0.0305f, homeSessionTitleBaselineInsetScale(), 0.0001f)
    }

    @Test
    fun sessionRecentStatusLabel_formatsChineseRelativeTime() {
        assertEquals("最近：1m前", sessionRecentStatusLabel(0))
        assertEquals("最近：59m前", sessionRecentStatusLabel(59))
        assertEquals("最近：1h前", sessionRecentStatusLabel(60))
        assertEquals("最近：23h前", sessionRecentStatusLabel(1_439))
        assertEquals("最近：1d前", sessionRecentStatusLabel(1_440))
        assertEquals("最近：28d前", sessionRecentStatusLabel(40_320))
    }

    @Test
    fun sessionAgentStatusLine_prefersPrecomputedStatusText() {
        assertEquals(
            "18:29 · 思考中 · 1m05s",
            sessionAgentStatusLine(
                SessionDetailSegmentUiModel(
                    sessionId = "running",
                    title = "Running",
                    activeLabel = "1m",
                    agentStatusLine = "18:29 · 思考中 · 1m05s",
                    contextLabel = "10k/100k",
                    contextProgress = 0.1f,
                    contextCompactThresholdPercent = null,
                    compactWarningVisible = false,
                    totalTokensLabel = "10k",
                    model = "gpt-5.4",
                    effort = "XHigh",
                    isActiveNow = true,
                    runtimePhaseLabel = "运行中 · 18:29 · 思考中 · 1m05s",
                    activeMinutes = 1,
                    activityRank = 0,
                    sourceIndex = 0,
                ),
            ),
        )
        assertEquals(
            "最近：2m前",
            sessionAgentStatusLine(
                SessionDetailSegmentUiModel(
                    sessionId = "idle",
                    title = "Idle",
                    activeLabel = "2m",
                    agentStatusLine = "",
                    contextLabel = "10k/100k",
                    contextProgress = 0.1f,
                    contextCompactThresholdPercent = null,
                    compactWarningVisible = false,
                    totalTokensLabel = "10k",
                    model = "gpt-5.4",
                    effort = "XHigh",
                    isActiveNow = false,
                    runtimePhaseLabel = "--",
                    activeMinutes = 2,
                    activityRank = 0,
                    sourceIndex = 0,
                ),
            )
        )
    }

    @Test
    fun sessionContextInnerSquareSidePx_usesContextRingInnerEdge() {
        assertEquals(596.74f, sessionContextInnerSquareSidePx(1_000f), 0.01f)
        assertEquals(0f, sessionContextInnerSquareSidePx(0f), 0.01f)
    }

    @Test
    fun shouldSelectSessionFromTap_requiresShortUnconsumedStationaryPress() {
        assertTrue(
            shouldSelectSessionFromTap(
                movedBeyondTouchSlop = false,
                consumedByAnotherHandler = false,
                pressDurationMs = ScreenshotLongPressTimeoutMs - 1L,
            ),
        )
        assertFalse(
            shouldSelectSessionFromTap(
                movedBeyondTouchSlop = true,
                consumedByAnotherHandler = false,
                pressDurationMs = 80L,
            ),
        )
        assertFalse(
            shouldSelectSessionFromTap(
                movedBeyondTouchSlop = false,
                consumedByAnotherHandler = true,
                pressDurationMs = 80L,
            ),
        )
        assertFalse(
            shouldSelectSessionFromTap(
                movedBeyondTouchSlop = false,
                consumedByAnotherHandler = false,
                pressDurationMs = ScreenshotLongPressTimeoutMs,
            ),
        )
    }

    @Test
    fun locateSessionSector_acceptsCenterAndInnerArea() {
        val size = IntSize(200, 200)

        assertEquals(1, locateSessionSector(Offset(100f, 100f), size, 4))
        assertEquals(3, locateSessionSector(Offset(40f, 100f), size, 4))
        assertEquals(0, locateSessionSector(Offset(100f, 40f), size, 4))
        assertEquals(2, locateSessionSector(Offset(100f, 160f), size, 4))
    }

    @Test
    fun locateSessionSector_rejectsTouchesOutsideRoundScreen() {
        val size = IntSize(200, 200)

        assertNull(locateSessionSector(Offset(-1f, 100f), size, 4))
        assertNull(locateSessionSector(Offset(100f, 201f), size, 4))
        assertNull(locateSessionSector(Offset(0f, 0f), size, 4))
    }

    private fun sessionRow(
        sessionId: String,
        title: String,
        lastActiveLabel: String,
        lastActiveAgoMinutes: Int,
        isSelected: Boolean = false,
        tokensLabel: String = "1k",
        isActiveNow: Boolean = false,
    ): SessionRowUiState {
        return SessionRowUiState(
            sessionId = sessionId,
            title = title,
            tokensLabel = tokensLabel,
            model = "gpt-5.4",
            reasoningLabel = "XHigh",
            lastActiveLabel = lastActiveLabel,
            lastActiveAgoMinutes = lastActiveAgoMinutes,
            agentStatusLine = sessionRecentStatusLabel(lastActiveAgoMinutes),
            isActiveNow = isActiveNow,
            isSelected = isSelected,
        )
    }
}
