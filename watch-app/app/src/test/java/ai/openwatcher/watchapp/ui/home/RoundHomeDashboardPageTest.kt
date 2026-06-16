package ai.openwatcher.watchapp.ui.home

import androidx.compose.ui.graphics.luminance
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import ai.openwatcher.watchapp.ui.QuotaRingUiState
import ai.openwatcher.watchapp.ui.components.WatchHomePalette

class RoundHomeDashboardPageTest {
    @Test
    fun homeSessionTitleTextScale_increasesByHalf() {
        assertEquals(0.0525f, homeSessionTitleTextScale(), 0.0001f)
    }

    @Test
    fun homeSessionTitleBaselineInsetScale_addsClearanceForLargerTitle() {
        assertEquals(0.0305f, homeSessionTitleBaselineInsetScale(), 0.0001f)
        assertTrue(homeSessionTitleBaselineInsetScale() > 0.013f)
    }

    @Test
    fun homeContextReadoutValueTextScale_matchesOldLargeFontSeventyFivePercent() {
        assertEquals(0.0825f, homeContextReadoutValueTextScale(), 0.0001f)
        assertEquals(0.05f, homeContextReadoutWarningTextScale(), 0.0001f)
    }

    @Test
    fun buildQuotaRingGeometry_usesNarrowGapAndThickerTimeRing() {
        val geometry = buildQuotaRingGeometry(20f)

        assertEquals(20f, geometry.outerStrokeWidth, 0.0001f)
        assertEquals(17f, geometry.innerStrokeWidth, 0.0001f)
        assertEquals(0.2f, geometry.ringGap, 0.0001f)
    }

    @Test
    fun miniHeatmapBarColor_mapsLowUsageToDarkerGreenAndHighUsageToBrighterRed() {
        val low = miniHeatmapBarColor(0f)
        val mid = miniHeatmapBarColor(0.5f)
        val high = miniHeatmapBarColor(1f)

        assertTrue("低用量应偏绿", low.green > low.red)
        assertTrue("高用量应偏红", high.red > high.green)
        assertTrue("中等用量应比低用量更亮", mid.luminance() > low.luminance())
        assertTrue("中等用量应比低用量更不透明", mid.alpha > low.alpha)
        assertTrue("高用量应比中等用量更不透明", high.alpha > mid.alpha)
    }

    @Test
    fun heatmapAxisLabels_showsEveryTwoHoursFromMidnightToTwentyTwo() {
        assertEquals(
            listOf("00", "02", "04", "06", "08", "10", "12", "14", "16", "18", "20", "22"),
            heatmapAxisLabels(),
        )
    }

    @Test
    fun heatmapAxisAnchorFractions_followMiniHeatmapLaneGeometry() {
        val geometry = miniHeatmapLaneGeometry(24)
        val anchors = heatmapAxisAnchorFractions(24)

        assertEquals(12, anchors.size)
        assertEquals(geometry.gapFraction + geometry.barWidthFraction / 2f, anchors.first(), 0.0001f)
        assertEquals(
            geometry.gapFraction + 12f * (geometry.barWidthFraction + geometry.gapFraction) + geometry.barWidthFraction / 2f,
            anchors[6],
            0.0001f,
        )
        assertEquals(
            geometry.gapFraction + 22f * (geometry.barWidthFraction + geometry.gapFraction) + geometry.barWidthFraction / 2f,
            anchors.last(),
            0.0001f,
        )
    }

    @Test
    fun weeklyHeatmapAxisAnchorFractions_spanWholeDayBoundaries() {
        val anchors = weeklyHeatmapAxisAnchorFractions(columnCount = 24, gapFraction = 0.01f)

        assertEquals(12, anchors.size)
        assertEquals(0f, anchors.first(), 0.0001f)
        assertTrue("固定 gap 后中间锚点应略晚于理想等分", anchors[6] > 0.5f)
        assertTrue("02 点锚点应晚于无 gap 的 2/24", anchors[1] > (2f / 24f))
        assertTrue("最后一个刻度应对齐 22 点列左边界", anchors.last() > (22f / 24f))
    }

    @Test
    fun weeklyHeatmapPlaceholderLabels_fillMissingRowsWithDashes() {
        assertEquals(
            List(7) { "--.--" },
            weeklyHeatmapPlaceholderLabels(),
        )
    }

    @Test
    fun weeklyHeatmapCellColor_usesDarkerIdleCellsAndWarmerPeakCells() {
        val idle = weeklyHeatmapCellColor(0f)
        val mid = weeklyHeatmapCellColor(0.5f)
        val peak = weeklyHeatmapCellColor(1f)

        assertTrue("空闲格子应更暗", idle.luminance() < mid.luminance())
        assertTrue("峰值格子应偏暖色", peak.red > peak.green)
        assertTrue("峰值格子应比中强度更偏红", peak.red > mid.red)
    }

    @Test
    fun timeRingPalette_usesContrastingActiveToneAndQuotaMatchedElapsedTrack() {
        val remaining = timeRingRemainingColor()
        val elapsedTrack = timeRingElapsedTrackColor()

        assertTrue("时间剩余环应偏冷色，和额度红黄绿渐变区分", remaining.blue > remaining.green)
        assertTrue("时间剩余环不能接近白灰", remaining.red - remaining.green > 0.08f)
        assertEquals("已走过时间轨道应和额度环底轨完全一致", WatchHomePalette.Track, elapsedTrack)
        assertTrue("已走过时间轨道应弱于剩余时间高亮", elapsedTrack.alpha < remaining.alpha)
    }

    @Test
    fun buildQuotaRingRenderState_showsTimeRingWhenPercentPresent() {
        val state = buildQuotaRingRenderState(
            QuotaRingUiState(
                title = "weekly",
                remainingPercent = 72f,
                timeRemainingPercent = 64f,
            ),
        )

        assertTrue(state.showTimeRing)
        assertEquals(0.72f, state.remainingFraction, 0.0001f)
        assertEquals(0.64f, state.timeRemainingFraction ?: -1f, 0.0001f)
    }

    @Test
    fun buildQuotaRingRenderState_hidesTimeRingWhenPercentMissing() {
        val state = buildQuotaRingRenderState(
            QuotaRingUiState(
                title = "5h",
                remainingPercent = 88f,
                timeRemainingPercent = null,
            ),
        )

        assertFalse(state.showTimeRing)
        assertEquals(null, state.timeRemainingFraction)
    }

    @Test
    fun buildQuotaRingRenderState_preservesDimmedPaletteFlag() {
        val state = buildQuotaRingRenderState(
            QuotaRingUiState(
                title = "weekly",
                remainingPercent = 45f,
                timeRemainingPercent = 40f,
                isDimmed = true,
            ),
        )

        assertTrue(state.isDimmed)
    }
}
