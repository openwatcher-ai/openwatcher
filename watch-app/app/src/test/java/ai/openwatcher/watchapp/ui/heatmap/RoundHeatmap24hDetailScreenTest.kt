package ai.openwatcher.watchapp.ui.heatmap

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import java.time.LocalDate
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test

class RoundHeatmap24hDetailScreenTest {
    @Test
    fun heatmapColorLevel_usesDynamicRangeBetweenDailyMinimumAndMaximum() {
        val minTotal = 0L
        val maxTotal = 600L
        val levelSize = (maxTotal - minTotal).toDouble() / 6.0

        assertEquals(0, heatmapColorLevel(0L, minTotal, maxTotal, levelSize))
        assertEquals(1, heatmapColorLevel(1L, minTotal, maxTotal, levelSize))
        assertEquals(2, heatmapColorLevel(120L, minTotal, maxTotal, levelSize))
        assertEquals(4, heatmapColorLevel(360L, minTotal, maxTotal, levelSize))
        assertEquals(6, heatmapColorLevel(600L, minTotal, maxTotal, levelSize))
    }

    @Test
    fun heatmapRingTapIndex_mapsRingTapsToSegments() {
        val size = Size(454f, 454f)
        val strokeWidth = 16f
        val inset = 3f
        val radius = (size.minDimension / 2f) - inset - (strokeWidth / 2f)
        val center = Offset(size.width / 2f, size.height / 2f)

        assertEquals(
            0,
            heatmapRingTapIndex(
                tapOffset = Offset(center.x, center.y - radius),
                canvasSize = size,
                strokeWidth = strokeWidth,
                inset = inset,
            ),
        )
        assertEquals(
            6,
            heatmapRingTapIndex(
                tapOffset = Offset(center.x + radius, center.y),
                canvasSize = size,
                strokeWidth = strokeWidth,
                inset = inset,
            ),
        )
        assertNull(
            heatmapRingTapIndex(
                tapOffset = center,
                canvasSize = size,
                strokeWidth = strokeWidth,
                inset = inset,
            ),
        )
        assertNull(
            heatmapRingTapIndex(
                tapOffset = Offset(0f, 0f),
                canvasSize = size,
                strokeWidth = strokeWidth,
                inset = inset,
            ),
        )
    }

    @Test
    fun dailyTrendChartTapIndex_mapsTapToNearestBarSlot() {
        val size = Size(210f, 120f)
        val start = LocalDate.parse("2026-05-07")
        val dayDates = List(30) { index -> start.plusDays(index.toLong()).toString() }
        val geometry = requireNotNull(dailyTrendCalendarGeometry(canvasSize = size, dayDates = dayDates))

        assertEquals(3, geometry.leadingEmptyCount)
        assertEquals(
            0,
            dailyTrendChartTapIndex(
                tapOffset = Offset(
                    geometry.gridLeft + 3 * (geometry.cellWidth + geometry.gap) + geometry.cellWidth / 2f,
                    geometry.gridTop + geometry.cellHeight / 2f,
                ),
                canvasSize = size,
                dayDates = dayDates,
            ),
        )
        assertEquals(
            29,
            dailyTrendChartTapIndex(
                tapOffset = Offset(
                    geometry.gridLeft + 4 * (geometry.cellWidth + geometry.gap) + geometry.cellWidth / 2f,
                    geometry.gridTop + 4 * (geometry.cellHeight + geometry.gap) + geometry.cellHeight / 2f,
                ),
                canvasSize = size,
                dayDates = dayDates,
            ),
        )
        assertNull(
            dailyTrendChartTapIndex(
                tapOffset = Offset(
                    geometry.gridLeft + 6 * (geometry.cellWidth + geometry.gap) + geometry.cellWidth / 2f,
                    geometry.gridTop + 4 * (geometry.cellHeight + geometry.gap) + geometry.cellHeight / 2f,
                ),
                canvasSize = size,
                dayDates = dayDates,
            ),
        )
        assertNull(
            dailyTrendChartTapIndex(
                tapOffset = Offset(
                    geometry.gridLeft + geometry.cellWidth / 2f,
                    geometry.gridTop + geometry.cellHeight / 2f,
                ),
                canvasSize = size,
                dayDates = dayDates,
            ),
        )
        assertNull(
            dailyTrendChartTapIndex(
                tapOffset = Offset(-1f, 10f),
                canvasSize = size,
                dayDates = dayDates,
            ),
        )
    }

    @Test
    fun dailyTrendCalendarGeometry_alignsColumnsToMondayThroughSunday() {
        val geometry = requireNotNull(
            dailyTrendCalendarGeometry(
                canvasSize = Size(210f, 120f),
                dayDates = listOf("2026-05-11", "2026-05-12", "2026-05-13"),
            ),
        )
        assertEquals(0, geometry.leadingEmptyCount)

        val thursdayGeometry = requireNotNull(
            dailyTrendCalendarGeometry(
                canvasSize = Size(210f, 120f),
                dayDates = listOf("2026-05-07", "2026-05-08", "2026-05-09"),
            ),
        )
        assertEquals(3, thursdayGeometry.leadingEmptyCount)
    }

    @Test
    fun dailyUsagePanelTopYOrNull_returnsNullForEmptyGeometryRange() {
        assertNull(dailyUsagePanelTopYOrNull(centerY = 0f, radius = 0f, topY = 0f))
        assertNull(dailyUsagePanelTopYOrNull(centerY = 0f, radius = 1f, topY = 0f))
    }

    @Test
    fun dailyUsageChordOrNull_handlesValidRoundGeometry() {
        val center = Offset(240f, 240f)
        val topY = dailyUsagePanelTopYOrNull(centerY = center.y, radius = 200f, topY = 220f)
        val chord = dailyUsageChordOrNull(center = center, radius = 200f, topY = requireNotNull(topY))

        assertNotNull(chord)
        val safeChord = requireNotNull(chord)
        assertTrue(safeChord.leftX < safeChord.rightX)
        assertTrue(safeChord.sweepAngle > 0f)
    }
}
