package ai.openwatcher.watchapp.ui.heatmap

import androidx.annotation.DrawableRes
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.Layout
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.PlatformTextStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.time.LocalDate
import kotlin.math.PI
import kotlin.math.atan2
import kotlin.math.max
import kotlin.math.roundToInt
import kotlin.math.sqrt
import ai.openwatcher.watchapp.R
import ai.openwatcher.watchapp.ui.DailyUsageUiState
import ai.openwatcher.watchapp.ui.Heatmap24hUiState
import ai.openwatcher.watchapp.ui.HeatmapRotaryMode
import ai.openwatcher.watchapp.ui.HeatmapSegmentUiState
import ai.openwatcher.watchapp.ui.components.StatusCapsuleDefaults
import ai.openwatcher.watchapp.ui.components.degradedVisuals
import ai.openwatcher.watchapp.ui.home.weeklyHeatmapCellColor

private val HeatmapScreenBackground = Color(0xFF05070B)
private val HeatmapRingTrack = Color(0x181D2937)
private val HeatmapSoftText = Color(0xFFADB9CC)
private val HeatmapTokenFire = Color(0xFFFF5A36)
private val HeatmapCursorOuter = Color(0xFF78D7FF)
private val HeatmapCursorInner = Color(0xFFB8ECFF)
private val HeatmapDailyPanel = Color(0xF20D1016)
private val HeatmapDailyPanelStroke = Color(0xFF303844)
private val HeatmapDailyInput = Color(0xD91E84B8)
private val HeatmapDailyCached = Color(0xD96F6FD8)
private val HeatmapDailyOutput = Color(0xD91EBAAA)
private val HeatmapDailyBarText = Color(0xE8F4F7FB)
private val HeatmapDailyReasoning = Color(0xFFB987FF)
private val CodexHeatmapLow = Color(0xFF183142)
private val CodexHeatmapHigh = Color(0xFF35B8FF)
private const val HeatmapSegmentCount = 24
private const val HeatmapColorLevels = 6
private const val HeatmapSegmentTailGapDegrees = 1f
private const val HeatmapTopAngleDegrees = -90f
private const val DailyTrendCalendarRowCount = 5
private const val DailyTrendCalendarColumnCount = 7
private const val DailyTrendCalendarWidthRatio = 0.84f
private const val DailyTrendWeekdayBandHeightRatio = 0.18f
private val DailyTrendWeekdayLabels = listOf("Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun")

data class HeatmapHourSelection(
    val index: Int,
)

internal data class DailyTrendCalendarGeometry(
    val gridLeft: Float,
    val gridTop: Float,
    val cellWidth: Float,
    val cellHeight: Float,
    val gap: Float,
    val firstVisibleDayIndex: Int,
    val visibleDayCount: Int,
    val leadingEmptyCount: Int,
)

@Composable
fun RoundHeatmap24hDetailScreen(
    state: Heatmap24hUiState,
    modifier: Modifier = Modifier,
    onSelectionChanged: (HeatmapHourSelection) -> Unit = {},
    onTrendSelectionChanged: (Int) -> Unit = {},
    onTrendSelectionDismissed: () -> Unit = {},
) {
    val segments = remember(state.segments) { state.segments.toHeatmapRingSegments() }
    val selectedStats = remember(state.selectedIndex, segments) {
        segments.selectedHourStats(state.selectedIndex)
    }

    BoxWithConstraints(
        modifier = modifier
            .fillMaxSize()
            .clip(CircleShape)
            .background(HeatmapScreenBackground)
            .degradedVisuals(state.isServiceDegraded)
            .pointerInput(Unit) {
                detectTapGestures {
                    onTrendSelectionDismissed()
                }
            },
    ) {
        val density = LocalDensity.current
        val diameter = minOf(maxWidth, maxHeight)
        val diameterPx = with(density) { diameter.toPx() }
        val ringStrokeWidth = diameterPx * 0.036f
        val ringInset = diameterPx * 0.006f

        Canvas(
            modifier = Modifier
                .fillMaxSize()
                .pointerInput(ringStrokeWidth, ringInset) {
                    detectTapGestures { tapOffset ->
                        heatmapRingTapIndex(
                            tapOffset = tapOffset,
                            canvasSize = Size(size.width.toFloat(), size.height.toFloat()),
                            strokeWidth = ringStrokeWidth,
                            inset = ringInset,
                        )?.let { index ->
                            onTrendSelectionDismissed()
                            onSelectionChanged(HeatmapHourSelection(index))
                        }
                    }
                },
        ) {
            drawHeatmapRing(
                segments = segments,
                selectionCursorIndex = state.selectionCursorIndex,
                strokeWidth = ringStrokeWidth,
                inset = ringInset,
            )
        }
        SelectedHourStatsPanel(
            stats = selectedStats,
            titleFont = (diameterPx / density.density * 0.046f).sp,
            totalFont = (diameterPx / density.density * 0.104f).sp,
            metricFont = (diameterPx / density.density * 0.046f).sp,
            iconSize = diameter * 0.052f,
            modifier = Modifier
                .align(Alignment.TopCenter)
                .padding(top = diameter * 0.105f)
                .width(diameter * 0.82f)
                .height(diameter * 0.34f),
        )
        Canvas(modifier = Modifier.fillMaxSize()) {
            drawDailyUsageFloatingLayer(
                strokeWidth = ringStrokeWidth,
                inset = ringInset,
                topY = size.minDimension * 0.455f,
            )
        }
        DailyUsagePanel(
            state = state.dailyUsage,
            rotaryMode = state.rotaryMode,
            titleFont = (diameterPx / density.density * 0.034f).sp,
            valueFont = (diameterPx / density.density * 0.058f).sp,
            metaFont = (diameterPx / density.density * 0.03f).sp,
            onTrendSelectionChanged = onTrendSelectionChanged,
            modifier = Modifier
                .align(Alignment.TopCenter)
                .padding(top = diameter * 0.455f)
                .width(diameter * 0.80f)
                .height(diameter * 0.39f),
        )
        if (state.dailyUsage.dailyTrend30d.tipVisible) {
            TrendSelectionTip(
                dateLabel = state.dailyUsage.dailyTrend30d.selectedDateLabel,
                tokensLabel = state.dailyUsage.dailyTrend30d.selectedTokenLabel,
                modifier = Modifier.align(Alignment.Center),
            )
        }
    }
}

private data class HeatmapRingSegment(
    val index: Int,
    val timeRangeLabel: String,
    val tokenLabel: String,
    val inputLabel: String,
    val cachedInputLabel: String,
    val outputLabel: String,
    val cacheHitRateLabel: String,
    val totalTokens: Long,
    val colorLevel: Int,
    val isPeak: Boolean,
)

private data class SelectedHourStats(
    val timeRangeLabel: String,
    val tokenLabel: String,
    val inputLabel: String,
    val cachedInputLabel: String,
    val outputLabel: String,
    val cacheHitRateLabel: String,
    val colorLevel: Int,
)

private fun List<HeatmapSegmentUiState>.toHeatmapRingSegments(): List<HeatmapRingSegment> {
    val paddedSegments = buildList {
        addAll(this@toHeatmapRingSegments.take(HeatmapSegmentCount))
        val firstMissingHour = size
        repeat((HeatmapSegmentCount - size).coerceAtLeast(0)) { index ->
            add(
                HeatmapSegmentUiState(
                    hourLabel = (firstMissingHour + index).toString().padStart(2, '0'),
                    timeRangeLabel = "--",
                    intensity = 0f,
                ),
            )
        }
    }
    val totals = paddedSegments.map { it.totalTokens.coerceAtLeast(0L) }
    val minTotal = totals.minOrNull() ?: 0L
    val maxTotal = totals.maxOrNull() ?: 0L
    val levelSize = max(1.0, (maxTotal - minTotal).toDouble() / HeatmapColorLevels.toDouble())

    return paddedSegments.mapIndexed { index, segment ->
        val total = segment.totalTokens.coerceAtLeast(0L)
        HeatmapRingSegment(
            index = index,
            timeRangeLabel = segment.timeRangeLabel,
            tokenLabel = segment.totalTokensLabel,
            inputLabel = segment.inputTokensLabel,
            cachedInputLabel = segment.cachedInputTokensLabel,
            outputLabel = segment.outputTokensLabel,
            cacheHitRateLabel = segment.cacheHitRateLabel,
            totalTokens = total,
            colorLevel = heatmapColorLevel(
                totalTokens = total,
                minTotal = minTotal,
                maxTotal = maxTotal,
                levelSize = levelSize,
            ),
            isPeak = segment.isPeak,
        )
    }
}

private fun List<HeatmapRingSegment>.selectedHourStats(selectedIndex: Int): SelectedHourStats {
    val selected = getOrNull(selectedIndex.coerceIn(0, lastIndex.coerceAtLeast(0)))
        ?: firstOrNull()
    return SelectedHourStats(
        timeRangeLabel = selected?.timeRangeLabel?.takeIf { it != "--" } ?: "--",
        tokenLabel = selected?.tokenLabel ?: "0",
        inputLabel = selected?.inputLabel ?: "0",
        cachedInputLabel = selected?.cachedInputLabel ?: "0",
        outputLabel = selected?.outputLabel ?: "0",
        cacheHitRateLabel = selected?.cacheHitRateLabel ?: "0%",
        colorLevel = selected?.colorLevel ?: 0,
    )
}

@Composable
private fun SelectedHourStatsPanel(
    stats: SelectedHourStats,
    titleFont: TextUnit,
    totalFont: TextUnit,
    metricFont: TextUnit,
    iconSize: Dp,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.SpaceBetween,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = stats.timeRangeLabel,
            modifier = Modifier.fillMaxWidth(),
            color = HeatmapSoftText,
            style = heatmapMetaTextStyle(titleFont, FontWeight.Medium),
            maxLines = 1,
            textAlign = TextAlign.Center,
        )
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(14.dp, Alignment.CenterHorizontally),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            HeatmapPrimaryMetric(
                iconRes = R.drawable.ic_token_burn,
                tint = HeatmapTokenFire,
                label = stats.tokenLabel,
                fontSize = totalFont,
                fontWeight = FontWeight.Light,
                iconSize = iconSize * 1.42f,
            )
            HeatmapPrimaryMetric(
                iconRes = R.drawable.ic_tokens_cached_layers,
                tint = Color(0xFF78BFFF),
                label = "CHR ${stats.cacheHitRateLabel}",
                fontSize = (totalFont.value * 0.52f).sp,
                fontWeight = FontWeight.Medium,
                iconSize = iconSize * 1.16f,
            )
        }
        Row(
            modifier = Modifier
                .fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceEvenly,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            HeatmapMiniMetric(
                iconRes = R.drawable.ic_tokens_input_up,
                tint = CodexHeatmapHigh,
                label = stats.inputLabel,
                caption = "输入",
                fontSize = metricFont,
                iconSize = iconSize,
            )
            HeatmapMiniMetric(
                iconRes = R.drawable.ic_tokens_cached_layers,
                tint = Color(0xFF78BFFF),
                label = stats.cachedInputLabel,
                caption = "缓存输入",
                fontSize = metricFont,
                iconSize = iconSize,
            )
            HeatmapMiniMetric(
                iconRes = R.drawable.ic_tokens_output_forward,
                tint = Color(0xFF2DBBFF),
                label = stats.outputLabel,
                caption = "输出",
                fontSize = metricFont,
                iconSize = iconSize,
            )
        }
    }
}

@Composable
private fun HeatmapPrimaryMetric(
    @DrawableRes iconRes: Int,
    tint: Color,
    label: String,
    fontSize: TextUnit,
    fontWeight: FontWeight,
    iconSize: Dp,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier,
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(5.dp),
    ) {
        Icon(
            painter = painterResource(id = iconRes),
            contentDescription = null,
            tint = tint,
            modifier = Modifier.size(iconSize),
        )
        Text(
            text = label,
            color = Color.White,
            style = heatmapMetaTextStyle(fontSize, fontWeight),
            maxLines = 1,
            textAlign = TextAlign.Center,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

@Composable
private fun HeatmapMiniMetric(
    @DrawableRes iconRes: Int,
    tint: Color,
    label: String,
    caption: String,
    fontSize: TextUnit,
    iconSize: Dp,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier.width(iconSize * 4.8f),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(3.dp),
    ) {
        Text(
            text = caption,
            color = HeatmapSoftText,
            style = heatmapMetaTextStyle(fontSize, FontWeight.Medium),
            maxLines = 1,
            textAlign = TextAlign.Center,
        )
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Icon(
                painter = painterResource(id = iconRes),
                contentDescription = null,
                tint = tint,
                modifier = Modifier.size(iconSize),
            )
            Text(
                text = label,
                color = Color.White,
                style = heatmapMetaTextStyle(fontSize, FontWeight.SemiBold),
                maxLines = 1,
                textAlign = TextAlign.Center,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

@Composable
private fun DailyUsagePanel(
    state: DailyUsageUiState,
    rotaryMode: HeatmapRotaryMode,
    titleFont: TextUnit,
    valueFont: TextUnit,
    metaFont: TextUnit,
    onTrendSelectionChanged: (Int) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier,
        verticalArrangement = Arrangement.spacedBy(6.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        DailyUsageContainedBar(
            state = state,
            fontSize = titleFont,
            modifier = Modifier
                .fillMaxWidth()
                .height(18.dp)
                .offset(y = 4.dp),
        )
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                text = "今日总Token：${state.totalTokensLabel}",
                color = Color.White,
                style = heatmapMetaTextStyle(titleFont, FontWeight.Medium),
                modifier = Modifier.weight(1f),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                text = "API折算价值：${state.estimatedValueLabel}",
                color = Color.White,
                style = heatmapMetaTextStyle(titleFont, FontWeight.Medium),
                modifier = Modifier.weight(1f),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                textAlign = TextAlign.End,
            )
        }
        DailyUsageTrendSection(
            state = state,
            rotaryMode = rotaryMode,
            titleFont = titleFont,
            metaFont = metaFont,
            onTrendSelectionChanged = onTrendSelectionChanged,
            modifier = Modifier
                .fillMaxWidth()
                .weight(1f),
        )
    }
}

@Composable
private fun DailyUsageTrendSection(
    state: DailyUsageUiState,
    rotaryMode: HeatmapRotaryMode,
    titleFont: TextUnit,
    metaFont: TextUnit,
    onTrendSelectionChanged: (Int) -> Unit,
    modifier: Modifier = Modifier,
) {
    BoxWithConstraints(modifier = modifier) {
        val chartTop = -(maxHeight * 0.032f)
        val chartHeight = maxHeight * 0.645f
        val summaryTop = maxHeight * 0.658f
        val valueTop = maxHeight * 1.025f
        val summaryWidth = maxWidth * 0.64f
        val summaryValueFont = (titleFont.value * 0.86f).sp
        val summaryLabelFont = (metaFont.value * 0.82f).sp
        val valueFont = (titleFont.value * 0.98f).sp
        val valueLabelFont = (metaFont.value * 0.80f).sp

        DailyTrend30dChart(
            state = state,
            rotaryMode = rotaryMode,
            labelFont = (metaFont.value * 0.76f).sp,
            onSelectionChanged = onTrendSelectionChanged,
            modifier = Modifier
                .align(Alignment.TopCenter)
                .offset(y = chartTop)
                .fillMaxWidth()
                .height(chartHeight)
                .padding(start = 6.dp, end = 6.dp),
        )
        if (state.dailyTrend30d.available) {
            DailyTrendSummaryRow(
                state = state.dailyTrend30d,
                labelFont = summaryLabelFont,
                valueFont = summaryValueFont,
                modifier = Modifier
                    .align(Alignment.TopCenter)
                    .offset(y = summaryTop)
                    .width(summaryWidth),
            )
            DailyTrendValueBlock(
                valueLabel = state.dailyTrend30d.valueLabel,
                valueFont = valueFont,
                labelFont = valueLabelFont,
                modifier = Modifier
                    .align(Alignment.TopCenter)
                    .offset(y = valueTop),
            )
        }
    }
}

@Composable
private fun DailyTrend30dChart(
    state: DailyUsageUiState,
    rotaryMode: HeatmapRotaryMode,
    labelFont: TextUnit,
    onSelectionChanged: (Int) -> Unit,
    modifier: Modifier = Modifier,
) {
    BoxWithConstraints(modifier = modifier) {
        val weekdayBandHeight = maxHeight * DailyTrendWeekdayBandHeightRatio
        val weekdayGap = 2.dp
        val gridTop = weekdayBandHeight + weekdayGap

        DailyTrendWeekdayHeader(
            fontSize = labelFont,
            modifier = Modifier
                .align(Alignment.TopCenter)
                .fillMaxWidth()
                .height(weekdayBandHeight),
        )

        Canvas(
            modifier = Modifier
                .align(Alignment.TopCenter)
                .fillMaxWidth()
                .height((maxHeight - gridTop).coerceAtLeast(24.dp))
                .offset(y = gridTop)
                .pointerInput(state.dailyTrend30d.dayDates) {
                    detectTapGestures { tapOffset ->
                        dailyTrendChartTapIndex(
                            tapOffset = tapOffset,
                            canvasSize = Size(size.width.toFloat(), size.height.toFloat()),
                            dayDates = state.dailyTrend30d.dayDates,
                        )?.let(onSelectionChanged)
                    }
                },
        ) {
            val fractions = state.dailyTrend30d.dayFractions
            if (fractions.isEmpty()) {
                return@Canvas
            }

            val geometry = dailyTrendCalendarGeometry(
                canvasSize = size,
                dayDates = state.dailyTrend30d.dayDates,
            ) ?: return@Canvas
            val cellRadius = minOf(geometry.cellWidth, geometry.cellHeight) * 0.24f
            val emptyCellColor = Color(0xFF0E1318)

            repeat(DailyTrendCalendarRowCount * DailyTrendCalendarColumnCount) { slotIndex ->
                val row = slotIndex / DailyTrendCalendarColumnCount
                val column = slotIndex % DailyTrendCalendarColumnCount
                val left = geometry.gridLeft + column * (geometry.cellWidth + geometry.gap)
                val top = geometry.gridTop + row * (geometry.cellHeight + geometry.gap)
                val dayIndex = slotIndex - geometry.leadingEmptyCount
                val trendIndex = if (dayIndex in 0 until geometry.visibleDayCount) {
                    geometry.firstVisibleDayIndex + dayIndex
                } else {
                    null
                }
                val isSelected = rotaryMode == HeatmapRotaryMode.Trend30d &&
                    trendIndex != null &&
                    state.dailyTrend30d.selectedIndex == trendIndex
                if (isSelected) {
                    drawRoundRect(
                        color = Color.White.copy(alpha = 0.14f),
                        topLeft = Offset(left - 2f, top - 2f),
                        size = Size(geometry.cellWidth + 4f, geometry.cellHeight + 4f),
                        cornerRadius = CornerRadius(cellRadius + 2f, cellRadius + 2f),
                    )
                }
                drawRoundRect(
                    color = trendIndex
                        ?.let { weeklyHeatmapCellColor(fractions[it]) }
                        ?: emptyCellColor,
                    topLeft = Offset(left, top),
                    size = Size(geometry.cellWidth, geometry.cellHeight),
                    cornerRadius = CornerRadius(cellRadius, cellRadius),
                )
                if (isSelected) {
                    drawRoundRect(
                        color = Color.White.copy(alpha = 0.40f),
                        topLeft = Offset(left, top),
                        size = Size(geometry.cellWidth, geometry.cellHeight),
                        cornerRadius = CornerRadius(cellRadius, cellRadius),
                        style = Stroke(width = 1.2f),
                    )
                }
            }
        }
    }
}

@Composable
private fun DailyTrendWeekdayHeader(
    fontSize: TextUnit,
    modifier: Modifier = Modifier,
) {
    Layout(
        modifier = modifier,
        content = {
            DailyTrendWeekdayLabels.forEach { label ->
                Text(
                    text = label,
                    color = HeatmapSoftText.copy(alpha = 0.88f),
                    style = heatmapMetaTextStyle(fontSize, FontWeight.Medium),
                    maxLines = 1,
                )
            }
        },
    ) { measurables, constraints ->
        val placeables = measurables.map { measurable ->
            measurable.measure(
                constraints.copy(
                    minWidth = 0,
                    minHeight = 0,
                ),
            )
        }
        val width = constraints.maxWidth
        val height = constraints.maxHeight
        val gapPx = dailyTrendCalendarGap(width.toFloat())
        val gridWidth = width * DailyTrendCalendarWidthRatio
        val cellWidth = ((gridWidth - gapPx * (DailyTrendCalendarColumnCount - 1)) / DailyTrendCalendarColumnCount)
            .coerceAtLeast(1f)
        val gridLeft = ((width - gridWidth) / 2f).coerceAtLeast(0f)

        layout(width, height) {
            placeables.forEachIndexed { index, placeable ->
                val anchorCenterX = gridLeft + index * (cellWidth + gapPx) + cellWidth / 2f
                val x = (anchorCenterX - placeable.width / 2f)
                    .coerceIn(0f, (width - placeable.width).coerceAtLeast(0).toFloat())
                    .roundToInt()
                val y = (height - placeable.height).coerceAtLeast(0)
                placeable.placeRelative(x, y)
            }
        }
    }
}

@Composable
private fun TrendSelectionTip(
    dateLabel: String,
    tokensLabel: String,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .clip(RoundedCornerShape(StatusCapsuleDefaults.bubbleCorner))
            .background(Color(0xE6101620))
            .padding(horizontal = StatusCapsuleDefaults.denseHorizontalPadding, vertical = 10.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        Text(
            text = dateLabel,
            color = HeatmapSoftText,
            style = heatmapMetaTextStyle(11.sp, FontWeight.Medium),
            maxLines = 1,
        )
        Text(
            text = tokensLabel,
            color = Color.White,
            style = heatmapMetaTextStyle(20.sp, FontWeight.SemiBold),
            maxLines = 1,
        )
    }
}

@Composable
private fun DailyTrendSummaryRow(
    state: ai.openwatcher.watchapp.ui.DailyTrend30dUiState,
    labelFont: TextUnit,
    valueFont: TextUnit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier
            .background(Color.Transparent),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        DailyTrendSummaryStat(
            value = state.totalLabel,
            label = "30天累计",
            valueFont = valueFont,
            labelFont = labelFont,
            modifier = Modifier.weight(1f),
        )
        DailyTrendSummaryDivider()
        DailyTrendSummaryStat(
            value = state.peakLabel,
            label = "峰值",
            valueFont = valueFont,
            labelFont = labelFont,
            modifier = Modifier.weight(1f),
        )
        DailyTrendSummaryDivider()
        DailyTrendSummaryStat(
            value = state.averageLabel,
            label = "日均",
            valueFont = valueFont,
            labelFont = labelFont,
            modifier = Modifier.weight(1f),
        )
    }
}

@Composable
private fun DailyTrendSummaryStat(
    value: String,
    label: String,
    valueFont: TextUnit,
    labelFont: TextUnit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier,
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(1.dp),
    ) {
        Text(
            text = value,
            color = Color.White,
            style = heatmapMetaTextStyle(valueFont, FontWeight.SemiBold),
            maxLines = 1,
        )
        Text(
            text = label,
            color = HeatmapSoftText,
            style = heatmapMetaTextStyle(labelFont, FontWeight.Medium),
            maxLines = 1,
        )
    }
}

@Composable
private fun DailyTrendSummaryDivider() {
    Box(
        modifier = Modifier
            .width(1.dp)
            .height(18.dp)
            .background(Color.White.copy(alpha = 0.12f)),
    )
}

@Composable
private fun DailyTrendValueBlock(
    valueLabel: String,
    valueFont: TextUnit,
    labelFont: TextUnit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier,
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(1.dp),
    ) {
        Text(
            text = valueLabel,
            color = Color.White,
            style = heatmapMetaTextStyle(valueFont, FontWeight.SemiBold),
            maxLines = 1,
        )
        Text(
            text = "30天价值",
            color = HeatmapSoftText,
            style = heatmapMetaTextStyle(labelFont, FontWeight.Medium),
            maxLines = 1,
        )
    }
}

@Composable
private fun DailyUsageContainedBar(
    state: DailyUsageUiState,
    fontSize: TextUnit,
    modifier: Modifier = Modifier,
) {
    val cachedInset = with(LocalDensity.current) { 1f.toDp() }
    Row(
        modifier = modifier
            .clip(CircleShape)
            .background(HeatmapDailyInput),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Row(
            modifier = Modifier
                .weight(1f)
                .fillMaxHeight(),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            DailyUsageBarText(
                text = "输入${state.inputLabel}",
                fontSize = fontSize,
                modifier = Modifier
                    .fillMaxHeight()
                    .padding(start = 10.dp, end = 4.dp),
            )
            DailyUsageBarText(
                text = "缓存${state.cachedInputLabel}(${state.cacheHitRateLabel})",
                fontSize = fontSize,
                modifier = Modifier
                    .weight(1f)
                    .fillMaxHeight()
                    .padding(cachedInset)
                    .background(HeatmapDailyCached),
            )
        }
        DailyUsageBarText(
            text = "输出${state.outputLabel}",
            fontSize = fontSize,
            modifier = Modifier
                .fillMaxHeight()
                .background(HeatmapDailyOutput)
                .padding(horizontal = 10.dp),
        )
    }
}

@Composable
private fun DailyUsageBarText(
    text: String,
    fontSize: TextUnit,
    modifier: Modifier = Modifier,
) {
    Box(
        modifier = modifier,
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = text,
            color = HeatmapDailyBarText,
            style = heatmapMetaTextStyle((fontSize.value * 0.92f).sp, FontWeight.Medium),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            textAlign = TextAlign.Center,
        )
    }
}

private fun androidx.compose.ui.graphics.drawscope.DrawScope.drawDailyUsageFloatingLayer(
    strokeWidth: Float,
    inset: Float,
    topY: Float,
) {
    val diameter = size.minDimension
    val center = Offset(size.width / 2f, size.height / 2f)
    val radius = (diameter / 2f) - inset - strokeWidth * 1.18f
    val clampedTopY = dailyUsagePanelTopYOrNull(center.y, radius, topY) ?: return
    val shadowSoft = buildDailyUsageFloatingPath(
        center = center.copy(y = center.y + strokeWidth * 0.28f),
        radius = radius + strokeWidth * 0.1f,
        topY = clampedTopY + strokeWidth * 0.22f,
    ) ?: return
    val shadowTight = buildDailyUsageFloatingPath(
        center = center.copy(y = center.y + strokeWidth * 0.12f),
        radius = radius,
        topY = clampedTopY + strokeWidth * 0.08f,
    ) ?: return
    val panel = buildDailyUsageFloatingPath(
        center = center,
        radius = radius,
        topY = clampedTopY,
    ) ?: return
    drawPath(shadowSoft, Color.White.copy(alpha = 0.05f))
    drawPath(shadowTight, Color.Black.copy(alpha = 0.32f))
    drawPath(panel, HeatmapDailyPanel)
    val chord = dailyUsageChordOrNull(center, radius, clampedTopY) ?: return
    drawPath(
        path = panel,
        color = HeatmapDailyPanelStroke.copy(alpha = 0.50f),
        style = Stroke(width = 1.3f),
    )
    drawLine(
        color = Color.White.copy(alpha = 0.10f),
        start = Offset(chord.leftX, clampedTopY),
        end = Offset(chord.rightX, clampedTopY),
        strokeWidth = 1.2f,
    )
}

internal data class DailyUsageChord(
    val leftX: Float,
    val rightX: Float,
    val startAngle: Float,
    val sweepAngle: Float,
)

private fun buildDailyUsageFloatingPath(
    center: Offset,
    radius: Float,
    topY: Float,
): Path? {
    val chord = dailyUsageChordOrNull(center, radius, topY) ?: return null
    return Path().apply {
        moveTo(chord.leftX, topY)
        lineTo(chord.rightX, topY)
        arcTo(
            rect = androidx.compose.ui.geometry.Rect(
                left = center.x - radius,
                top = center.y - radius,
                right = center.x + radius,
                bottom = center.y + radius,
            ),
            startAngleDegrees = chord.startAngle,
            sweepAngleDegrees = chord.sweepAngle,
            forceMoveTo = false,
        )
        close()
    }
}

internal fun dailyUsagePanelTopYOrNull(
    centerY: Float,
    radius: Float,
    topY: Float,
): Float? {
    if (radius <= 1f) {
        return null
    }
    val min = centerY - radius + 1f
    val max = centerY + radius - 1f
    if (min > max) {
        return null
    }
    return topY.coerceIn(min, max)
}

internal fun dailyUsageChordOrNull(
    center: Offset,
    radius: Float,
    topY: Float,
): DailyUsageChord? {
    if (radius <= 1f) {
        return null
    }
    val minDy = -radius + 1f
    val maxDy = radius - 1f
    if (minDy > maxDy) {
        return null
    }
    val dy = (topY - center.y).coerceIn(-radius + 1f, radius - 1f)
    val halfWidth = sqrt((radius * radius - dy * dy).coerceAtLeast(1f))
    val rawStartAngle = atan2(dy, halfWidth) * 180f / PI.toFloat()
    val startAngle = if (rawStartAngle < 0f) rawStartAngle + 360f else rawStartAngle
    val endAngle = 180f - rawStartAngle
    val sweep = (endAngle - startAngle + 360f) % 360f
    return DailyUsageChord(
        leftX = center.x - halfWidth,
        rightX = center.x + halfWidth,
        startAngle = startAngle,
        sweepAngle = sweep,
    )
}

private fun heatmapMetaTextStyle(
    fontSize: TextUnit,
    fontWeight: FontWeight,
): TextStyle {
    return TextStyle(
        fontSize = fontSize,
        lineHeight = fontSize,
        fontWeight = fontWeight,
        platformStyle = PlatformTextStyle(includeFontPadding = false),
    )
}

internal fun heatmapColorLevel(
    totalTokens: Long,
    minTotal: Long,
    maxTotal: Long,
    levelSize: Double,
): Int {
    if (totalTokens <= 0L || maxTotal <= 0L) {
        return 0
    }
    if (maxTotal == minTotal) {
        return HeatmapColorLevels
    }
    return (((totalTokens - minTotal).toDouble() / levelSize).toInt() + 1)
        .coerceIn(1, HeatmapColorLevels)
}

internal fun heatmapRingTapIndex(
    tapOffset: Offset,
    canvasSize: Size,
    strokeWidth: Float,
    inset: Float,
): Int? {
    val diameter = canvasSize.minDimension
    if (diameter <= 0f) {
        return null
    }
    val radius = (diameter / 2f) - inset - (strokeWidth / 2f)
    val center = Offset(canvasSize.width / 2f, canvasSize.height / 2f)
    val dx = tapOffset.x - center.x
    val dy = tapOffset.y - center.y
    val distance = sqrt(dx * dx + dy * dy)
    val touchSlop = strokeWidth * 1.4f
    if (distance < radius - touchSlop || distance > radius + touchSlop) {
        return null
    }

    val angle = atan2(dy, dx) * (180f / PI.toFloat())
    val normalized = positiveModulo(angle - HeatmapTopAngleDegrees, 360f)
    val segmentStep = 360f / HeatmapSegmentCount
    return (normalized / segmentStep).toInt().coerceIn(0, HeatmapSegmentCount - 1)
}

internal fun dailyTrendChartTapIndex(
    tapOffset: Offset,
    canvasSize: Size,
    dayDates: List<String>,
): Int? {
    val geometry = dailyTrendCalendarGeometry(canvasSize = canvasSize, dayDates = dayDates) ?: return null
    if (tapOffset.x !in 0f..canvasSize.width || tapOffset.y !in 0f..canvasSize.height) {
        return null
    }
    if (
        tapOffset.x < geometry.gridLeft ||
        tapOffset.x > geometry.gridLeft + geometry.cellWidth * DailyTrendCalendarColumnCount + geometry.gap * (DailyTrendCalendarColumnCount - 1) ||
        tapOffset.y < geometry.gridTop ||
        tapOffset.y > geometry.gridTop + geometry.cellHeight * DailyTrendCalendarRowCount + geometry.gap * (DailyTrendCalendarRowCount - 1)
    ) {
        return null
    }
    val column = ((tapOffset.x - geometry.gridLeft) / (geometry.cellWidth + geometry.gap))
        .toInt()
        .coerceIn(0, DailyTrendCalendarColumnCount - 1)
    val row = ((tapOffset.y - geometry.gridTop) / (geometry.cellHeight + geometry.gap))
        .toInt()
        .coerceIn(0, DailyTrendCalendarRowCount - 1)
    val slotIndex = row * DailyTrendCalendarColumnCount + column
    val dayIndex = slotIndex - geometry.leadingEmptyCount
    if (dayIndex !in 0 until geometry.visibleDayCount) {
        return null
    }
    return geometry.firstVisibleDayIndex + dayIndex
}

internal fun dailyTrendCalendarGeometry(
    canvasSize: Size,
    dayDates: List<String>,
): DailyTrendCalendarGeometry? {
    if (dayDates.isEmpty() || canvasSize.width <= 0f || canvasSize.height <= 0f) {
        return null
    }
    val count = dayDates.size
    val totalSlots = DailyTrendCalendarRowCount * DailyTrendCalendarColumnCount
    val leadingEmptyCount = dailyTrendWeekdayOffset(dayDates.first())
    val maxVisibleDays = (totalSlots - leadingEmptyCount).coerceAtLeast(0)
    val firstVisibleDayIndex = (count - maxVisibleDays).coerceAtLeast(0)
    val visibleDayCount = (count - firstVisibleDayIndex).coerceAtMost(maxVisibleDays)
    val gap = dailyTrendCalendarGap(canvasSize.width)
    val targetGridWidth = (canvasSize.width * DailyTrendCalendarWidthRatio).coerceAtLeast(1f)
    val cellWidth = ((targetGridWidth - gap * (DailyTrendCalendarColumnCount - 1)) / DailyTrendCalendarColumnCount)
        .coerceAtLeast(1f)
    val cellHeight = ((canvasSize.height - gap * (DailyTrendCalendarRowCount - 1)) / DailyTrendCalendarRowCount)
        .coerceAtLeast(1f)
    val gridWidth = cellWidth * DailyTrendCalendarColumnCount + gap * (DailyTrendCalendarColumnCount - 1)
    val gridHeight = cellHeight * DailyTrendCalendarRowCount + gap * (DailyTrendCalendarRowCount - 1)
    return DailyTrendCalendarGeometry(
        gridLeft = ((canvasSize.width - gridWidth) / 2f).coerceAtLeast(0f),
        gridTop = ((canvasSize.height - gridHeight) / 2f).coerceAtLeast(0f),
        cellWidth = cellWidth,
        cellHeight = cellHeight,
        gap = gap,
        firstVisibleDayIndex = firstVisibleDayIndex,
        visibleDayCount = visibleDayCount,
        leadingEmptyCount = leadingEmptyCount,
    )
}

internal fun dailyTrendWeekdayOffset(date: String): Int {
    val parsed = runCatching { LocalDate.parse(date) }.getOrNull() ?: return 0
    return (parsed.dayOfWeek.value - 1).coerceIn(0, DailyTrendCalendarColumnCount - 1)
}

private fun dailyTrendCalendarGap(width: Float): Float {
    return (width * 0.0085f).coerceIn(2f, 3.5f)
}

private fun positiveModulo(value: Float, modulo: Float): Float {
    val raw = value % modulo
    return if (raw < 0f) raw + modulo else raw
}

private fun wrapHeatmapCursorPosition(position: Float, size: Int): Float {
    if (size <= 0) {
        return 0f
    }
    val raw = position % size.toFloat()
    return if (raw < 0f) raw + size.toFloat() else raw
}

private fun androidx.compose.ui.graphics.drawscope.DrawScope.drawHeatmapRing(
    segments: List<HeatmapRingSegment>,
    selectionCursorIndex: Float?,
    strokeWidth: Float,
    inset: Float,
) {
    val diameter = size.minDimension
    val radius = (diameter / 2f) - inset - (strokeWidth / 2f)
    val arcSize = Size(radius * 2f, radius * 2f)
    val center = Offset(size.width / 2f, size.height / 2f)
    val topLeft = Offset(center.x - radius, center.y - radius)
    val segmentStep = 360f / HeatmapSegmentCount
    val segmentSweep = segmentStep - HeatmapSegmentTailGapDegrees

    selectionCursorIndex?.let { cursorIndex ->
        drawHeatmapCursorAccent(
            cursorIndex = cursorIndex,
            strokeWidth = strokeWidth,
            inset = inset,
            layer = HeatmapCursorLayer.Underlay,
        )
    }

    repeat(HeatmapSegmentCount) { index ->
        val startAngle = HeatmapTopAngleDegrees + index * segmentStep
        drawArc(
            color = HeatmapRingTrack,
            startAngle = startAngle,
            sweepAngle = segmentSweep,
            useCenter = false,
            topLeft = topLeft,
            size = arcSize,
            style = Stroke(width = strokeWidth, cap = StrokeCap.Butt),
        )
    }

    segments.forEach { segment ->
        if (segment.totalTokens <= 0L || segment.colorLevel <= 0) {
            return@forEach
        }
        val startAngle = HeatmapTopAngleDegrees + segment.index * segmentStep
        drawArc(
            color = heatmapColor(segment.colorLevel),
            startAngle = startAngle,
            sweepAngle = segmentSweep,
            useCenter = false,
            topLeft = topLeft,
            size = arcSize,
            style = Stroke(width = strokeWidth, cap = StrokeCap.Butt),
        )
    }

    selectionCursorIndex?.let { cursorIndex ->
        drawHeatmapCursorAccent(
            cursorIndex = cursorIndex,
            strokeWidth = strokeWidth,
            inset = inset,
            layer = HeatmapCursorLayer.Overlay,
        )
    }
}

private fun heatmapColor(level: Int): Color {
    val fraction = (level.coerceIn(1, HeatmapColorLevels) - 1).toFloat() / (HeatmapColorLevels - 1).toFloat()
    return lerp(CodexHeatmapLow, CodexHeatmapHigh, fraction)
}

private enum class HeatmapCursorLayer {
    Underlay,
    Overlay,
}

private fun androidx.compose.ui.graphics.drawscope.DrawScope.drawHeatmapCursorAccent(
    cursorIndex: Float,
    strokeWidth: Float,
    inset: Float,
    layer: HeatmapCursorLayer,
) {
    val diameter = size.minDimension
    val radius = (diameter / 2f) - inset - (strokeWidth / 2f)
    val center = Offset(size.width / 2f, size.height / 2f)
    val segmentStep = 360f / HeatmapSegmentCount
    val segmentSweep = segmentStep - HeatmapSegmentTailGapDegrees
    val normalizedCursor = wrapHeatmapCursorPosition(cursorIndex, HeatmapSegmentCount)
    val startAngle = HeatmapTopAngleDegrees + normalizedCursor * segmentStep

    when (layer) {
        HeatmapCursorLayer.Underlay -> {
            drawArcOnRadius(
                color = HeatmapCursorOuter.copy(alpha = 0.42f),
                center = center,
                radius = radius + strokeWidth * 0.62f,
                startAngle = startAngle,
                sweepAngle = segmentSweep,
                strokeWidth = strokeWidth * 0.52f,
            )
        }

        HeatmapCursorLayer.Overlay -> {
            drawArcOnRadius(
                color = HeatmapCursorInner.copy(alpha = 0.72f),
                center = center,
                radius = radius,
                startAngle = startAngle,
                sweepAngle = segmentSweep,
                strokeWidth = max(2.2f, strokeWidth * 0.24f),
            )
            drawArcOnRadius(
                color = CodexHeatmapHigh.copy(alpha = 0.46f),
                center = center,
                radius = radius - strokeWidth * 0.48f,
                startAngle = startAngle,
                sweepAngle = segmentSweep,
                strokeWidth = max(1.8f, strokeWidth * 0.18f),
            )
        }
    }
}

private fun androidx.compose.ui.graphics.drawscope.DrawScope.drawArcOnRadius(
    color: Color,
    center: Offset,
    radius: Float,
    startAngle: Float,
    sweepAngle: Float,
    strokeWidth: Float,
) {
    val arcSize = Size(radius * 2f, radius * 2f)
    drawArc(
        color = color,
        startAngle = startAngle,
        sweepAngle = sweepAngle,
        useCenter = false,
        topLeft = Offset(center.x - radius, center.y - radius),
        size = arcSize,
        style = Stroke(
            width = strokeWidth,
            cap = StrokeCap.Butt,
        )
    )
}
