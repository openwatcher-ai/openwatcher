package ai.openwatcher.watchapp.ui.home

import androidx.annotation.DrawableRes
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
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
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.layout.Layout
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.PlatformTextStyle
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlin.math.cos
import kotlin.math.min
import kotlin.math.roundToInt
import kotlin.math.sin
import kotlin.math.sqrt
import ai.openwatcher.watchapp.R
import ai.openwatcher.watchapp.ui.HomeDashboardUiState
import ai.openwatcher.watchapp.ui.HomeWeeklyHeatmapUiState
import ai.openwatcher.watchapp.ui.MiniBarUiState
import ai.openwatcher.watchapp.ui.QuotaRingUiState
import ai.openwatcher.watchapp.ui.components.StatusCapsuleDefaults
import ai.openwatcher.watchapp.ui.components.CurvedTitleMarqueeState
import ai.openwatcher.watchapp.ui.components.WatchHomePalette
import ai.openwatcher.watchapp.ui.components.curvedTitlePathLength
import ai.openwatcher.watchapp.ui.components.degradedVisuals
import ai.openwatcher.watchapp.ui.components.drawCurvedTitle
import ai.openwatcher.watchapp.ui.components.rememberCurvedTitleMarqueeState

internal data class HomeRoundVisualState(
    val sessionTitle: String,
    val contextUsedText: String,
    val contextWindowText: String,
    val contextRemainingPercent: Float,
    val contextPressurePercent: Float,
    val contextCompactThresholdPercent: Float?,
    val compactWarningVisible: Boolean,
    val totalTokensLabel: String,
    val modelLabel: String,
    val reasoningLabel: String,
    val activityLabel: String,
    val isActivityLive: Boolean,
)

internal data class QuotaRingRenderState(
    val remainingFraction: Float,
    val timeRemainingFraction: Float?,
    val showTimeRing: Boolean,
    val isDimmed: Boolean,
)

internal data class QuotaRingGeometry(
    val outerStrokeWidth: Float,
    val innerStrokeWidth: Float,
    val ringGap: Float,
)

internal data class MiniHeatmapLaneGeometry(
    val barWidthFraction: Float,
    val gapFraction: Float,
)

private const val HOME_SESSION_TITLE_TEXT_SCALE = 0.035f
private const val HOME_SESSION_TITLE_GROWTH_MULTIPLIER = 1.5f
private const val HOME_SESSION_TITLE_BASELINE_INSET_SCALE = 0.013f
private const val HOME_CONTEXT_READOUT_LARGE_SCALE = 0.11f
private const val HOME_CONTEXT_READOUT_WARNING_SCALE = 0.05f
private const val HOME_CONTEXT_READOUT_UNIFIED_MULTIPLIER = 0.75f

internal fun homeSessionTitleTextScale(): Float {
    return HOME_SESSION_TITLE_TEXT_SCALE * HOME_SESSION_TITLE_GROWTH_MULTIPLIER
}

internal fun homeSessionTitleBaselineInsetScale(): Float {
    return HOME_SESSION_TITLE_BASELINE_INSET_SCALE +
        HOME_SESSION_TITLE_TEXT_SCALE * (HOME_SESSION_TITLE_GROWTH_MULTIPLIER - 1f)
}

internal fun homeContextReadoutValueTextScale(): Float {
    return HOME_CONTEXT_READOUT_LARGE_SCALE * HOME_CONTEXT_READOUT_UNIFIED_MULTIPLIER
}

internal fun homeContextReadoutWarningTextScale(): Float {
    return HOME_CONTEXT_READOUT_WARNING_SCALE
}

internal fun heatmapAxisLabels(): List<String> {
    return (0..22 step 2).map { hour -> hour.toString().padStart(2, '0') }
}

internal fun weeklyHeatmapAxisAnchorFractions(columnCount: Int, gapFraction: Float): List<Float> {
    if (columnCount <= 0) {
        return heatmapAxisLabels().map { 0f }
    }
    val safeGapFraction = gapFraction.coerceAtLeast(0f)
    val cellFraction = ((1f - safeGapFraction * (columnCount - 1)) / columnCount).coerceAtLeast(0f)
    return heatmapAxisLabels().mapIndexed { index, _ ->
        val hour = index * 2
        when {
            hour <= 0 -> 0f
            hour >= columnCount -> 1f
            else -> hour * (cellFraction + safeGapFraction)
        }.coerceIn(0f, 1f)
    }
}

internal fun weeklyHeatmapPlaceholderLabels(): List<String> {
    return List(7) { "--.--" }
}

internal fun miniHeatmapLaneGeometry(barCount: Int): MiniHeatmapLaneGeometry {
    if (barCount <= 0) {
        return MiniHeatmapLaneGeometry(barWidthFraction = 0f, gapFraction = 0f)
    }
    val barWidthFraction = 1f / (barCount * 2.3f)
    val gapFraction = ((1f - barWidthFraction * barCount) / (barCount + 1)).coerceAtLeast(0f)
    return MiniHeatmapLaneGeometry(
        barWidthFraction = barWidthFraction,
        gapFraction = gapFraction,
    )
}

internal fun heatmapAxisAnchorFractions(barCount: Int): List<Float> {
    val geometry = miniHeatmapLaneGeometry(barCount)
    return heatmapAxisLabels().mapIndexed { index, _ ->
        val hour = index * 2
        when {
            barCount <= 0 -> 0f
            hour >= barCount -> 1f
            else -> {
                geometry.gapFraction +
                    hour * (geometry.barWidthFraction + geometry.gapFraction) +
                    geometry.barWidthFraction / 2f
            }
        }.coerceIn(0f, 1f)
    }
}

internal fun buildQuotaRingRenderState(state: QuotaRingUiState): QuotaRingRenderState {
    val timeRemainingFraction = state.timeRemainingPercent?.coerceIn(0f, 100f)?.div(100f)
    return QuotaRingRenderState(
        remainingFraction = state.remainingPercent.coerceIn(0f, 100f) / 100f,
        timeRemainingFraction = timeRemainingFraction,
        showTimeRing = timeRemainingFraction != null,
        isDimmed = state.isDimmed,
    )
}

internal fun buildQuotaRingGeometry(outerStrokeWidth: Float): QuotaRingGeometry {
    return QuotaRingGeometry(
        outerStrokeWidth = outerStrokeWidth,
        innerStrokeWidth = outerStrokeWidth * 0.85f,
        ringGap = outerStrokeWidth * 0.01f,
    )
}

internal fun timeRingRemainingColor(isDimmed: Boolean = false): Color {
    return accentTone(Color(0xFF7C66FF), isDimmed)
}

internal fun timeRingElapsedTrackColor(): Color {
    return WatchHomePalette.Track
}

@Composable
internal fun RoundHomeDashboardPage(
    homeState: HomeDashboardUiState,
    errors: List<String>,
    onOpenHeatmap: () -> Unit,
    onOpenSessionDetails: () -> Unit,
    onShowQuotaEasterEgg: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val visualState = remember(homeState) { buildRoundHomeVisualState(homeState) }

    BoxWithConstraints(
        modifier = modifier
            .fillMaxSize()
            .clip(CircleShape),
    ) {
        val density = LocalDensity.current
        val diameter = minOf(maxWidth, maxHeight)
        val diameterPx = with(density) { diameter.toPx() }
        val radiusPx = diameterPx / 2f
        val arcStroke = diameter * 0.022f
        val arcInset = diameter * 0.006f
        val titleTextSizePx = diameterPx * homeSessionTitleTextScale()
        val titleInsetPx = diameterPx * homeSessionTitleBaselineInsetScale()
        val titlePathRadiusPx = radiusPx - with(density) { arcInset.toPx() } - with(density) { arcStroke.toPx() * 1.78f }
        val homeTitleMarqueeState = rememberCurvedTitleMarqueeState(
            title = visualState.sessionTitle,
            pathLengthPx = curvedTitlePathLength(
                radius = titlePathRadiusPx,
                baselineInset = titleInsetPx,
            ),
            textSizePx = titleTextSizePx,
            edgePaddingPx = titleTextSizePx * 1.02f,
        )
        val readoutTop = diameter * 0.14f
        val metaTop = diameter * 0.262f
        val metaWidth = diameter * 0.64f
        val metaHeight = diameter * 0.185f
        val metaFont = (diameterPx / density.density * 0.046f).sp
        val metaIcon = diameter * 0.07f
        val activityTrackWidth = diameter * 0.18f
        val activityTrackHeight = diameter * 0.04f

        val quotaSize = diameter * 0.235f
        val quotaRadiusPx = with(density) { quotaSize.toPx() } / 2f
        val quotaOrbitRadiusPx = radiusPx - quotaRadiusPx - with(density) { (diameter * 0.01f).toPx() }
        val quotaCenterOffsetXPx = diameterPx * 0.145f
        val quotaCenterYOffsetPx = sqrt((quotaOrbitRadiusPx * quotaOrbitRadiusPx - quotaCenterOffsetXPx * quotaCenterOffsetXPx).coerceAtLeast(0f))
        val quotaCenterYPx = radiusPx + quotaCenterYOffsetPx
        val quotaTop = with(density) { (quotaCenterYPx - quotaRadiusPx).toDp() }
        val leftQuotaX = with(density) { (radiusPx - quotaCenterOffsetXPx).toDp() } - (quotaSize / 2f)
        val rightQuotaX = with(density) { (radiusPx + quotaCenterOffsetXPx).toDp() } - (quotaSize / 2f)
        val quotaTapTop = (quotaTop - diameter * 0.018f).coerceAtLeast(diameter * 0.6f)
        val quotaTapHeight = (diameter - quotaTapTop).coerceAtLeast(diameter * 0.22f)

        val heatmapWidth = diameter * 0.82f
        val heatmapTop = diameter * 0.425f
        val heatmapHeight = (quotaTop - heatmapTop - diameter * 0.034f).coerceAtLeast(diameter * 0.18f)

        val contextValueFont = (diameterPx / density.density * homeContextReadoutValueTextScale()).sp
        val contextWarningFont = (diameterPx / density.density * homeContextReadoutWarningTextScale()).sp
        val quotaPercentFont = (diameterPx / density.density * 0.064f).sp

        Box(modifier = Modifier.fillMaxSize().degradedVisuals(homeState.isServiceDegraded)) {
            StrictTopContextArc(
                remainingPercent = visualState.contextRemainingPercent,
                compactThresholdPercent = visualState.contextCompactThresholdPercent,
                isServiceDegraded = homeState.isServiceDegraded,
                strokeWidth = arcStroke,
                inset = arcInset,
                modifier = Modifier.fillMaxSize(),
            )

            HomeCurvedSessionTitle(
                title = visualState.sessionTitle,
                isServiceDegraded = homeState.isServiceDegraded,
                marqueeState = homeTitleMarqueeState,
                titleTextSizePx = titleTextSizePx,
                baselineInsetPx = titleInsetPx,
                radiusPx = titlePathRadiusPx,
                edgePaddingPx = titleTextSizePx * 1.02f,
                modifier = Modifier.fillMaxSize(),
            )

            Box(
                modifier = Modifier
                    .align(Alignment.TopCenter)
                    .fillMaxWidth()
                    .height(diameter * 0.42f)
                    .clickable(onClick = onOpenSessionDetails),
            )

            if (errors.isNotEmpty()) {
                HomeErrorBanner(
                    errors = errors,
                    modifier = Modifier
                        .align(Alignment.TopCenter)
                        .padding(top = diameter * 0.06f),
                )
            }

            ContextReadout(
                state = visualState,
                valueFont = contextValueFont,
                warningFont = contextWarningFont,
                modifier = Modifier
                    .align(Alignment.TopCenter)
                    .padding(top = readoutTop),
            )

            HomeMetaGrid(
                state = visualState,
                isServiceDegraded = homeState.isServiceDegraded,
                fontSize = metaFont,
                iconSize = metaIcon,
                activityTrackWidth = activityTrackWidth,
                activityTrackHeight = activityTrackHeight,
                modifier = Modifier
                    .align(Alignment.TopCenter)
                    .padding(top = metaTop)
                    .width(metaWidth)
                    .height(metaHeight),
            )

            HeatmapRibbon(
                state = homeState.weeklyHeatmap,
                isServiceDegraded = homeState.isServiceDegraded,
                labelFont = (diameterPx / density.density * 0.024f).sp,
                modifier = Modifier
                    .align(Alignment.TopCenter)
                    .padding(top = heatmapTop)
                    .width(heatmapWidth)
                    .height(heatmapHeight)
                    .clickable(onClick = onOpenHeatmap),
            )

            Box(
                modifier = Modifier
                    .align(Alignment.TopCenter)
                    .padding(top = quotaTapTop)
                    .width(diameter * 0.74f)
                    .height(quotaTapHeight)
                    .clickable(onClick = onShowQuotaEasterEgg),
            )

            QuotaGapRing(
                state = homeState.fiveHour,
                label = "5 h",
                ringSize = quotaSize,
                percentFont = quotaPercentFont,
                modifier = Modifier
                    .align(Alignment.TopStart)
                    .offset(x = leftQuotaX, y = quotaTop),
            )

            QuotaGapRing(
                state = homeState.weekly,
                label = "7 d",
                ringSize = quotaSize,
                percentFont = quotaPercentFont,
                modifier = Modifier
                    .align(Alignment.TopStart)
                    .offset(x = rightQuotaX, y = quotaTop),
            )

            AnimatedVisibility(
                visible = homeState.quotaEasterEgg.visible && !homeState.quotaEasterEgg.text.isNullOrBlank(),
                modifier = Modifier
                    .align(Alignment.Center)
                    .offset(y = -(diameter * 0.01f)),
                enter = fadeIn(animationSpec = tween(durationMillis = 120)),
                exit = fadeOut(animationSpec = tween(durationMillis = 180)),
            ) {
                HomeQuotaTipBubble(
                    text = homeState.quotaEasterEgg.text.orEmpty(),
                    maxWidth = diameter * 0.74f,
                )
            }
        }
    }
}

private fun buildRoundHomeVisualState(homeState: HomeDashboardUiState): HomeRoundVisualState {
    val contextUsedText = homeState.selectedSessionContextLabel.substringBefore("/", "--").uppercase().ifBlank { "--" }
    val contextWindowText = homeState.selectedSessionContextLabel.substringAfter("/", "--").uppercase().ifBlank { "--" }
    val totalTokensLabel = homeState.totalTokensLabel.ifBlank { "--" }
    val activityLabel = homeState.selectedSessionActiveLabel.takeIf { it.isNotBlank() && it != "--" } ?: "--"
    return HomeRoundVisualState(
        sessionTitle = homeState.selectedSessionTitle,
        contextUsedText = contextUsedText,
        contextWindowText = contextWindowText,
        contextRemainingPercent = (100f - homeState.selectedSessionPressurePercent).coerceIn(0f, 100f),
        contextPressurePercent = homeState.selectedSessionPressurePercent.coerceIn(0f, 100f),
        contextCompactThresholdPercent = homeState.selectedSessionCompactThresholdPercent?.coerceIn(0f, 100f),
        compactWarningVisible = homeState.selectedSessionCompactWarning,
        totalTokensLabel = totalTokensLabel,
        modelLabel = homeState.selectedSessionModel,
        reasoningLabel = homeState.selectedSessionReasoning,
        activityLabel = activityLabel,
        isActivityLive = homeState.selectedSessionIsActiveNow,
    )
}

@Composable
private fun HomeCurvedSessionTitle(
    title: String,
    isServiceDegraded: Boolean,
    marqueeState: CurvedTitleMarqueeState,
    titleTextSizePx: Float,
    baselineInsetPx: Float,
    radiusPx: Float,
    edgePaddingPx: Float,
    modifier: Modifier = Modifier,
) {
    Canvas(modifier = modifier) {
        drawCurvedTitle(
            title = title,
            center = Offset(size.width / 2f, size.height / 2f),
            radius = radiusPx,
            baselineInset = baselineInsetPx,
            textSizePx = titleTextSizePx,
            marqueeState = marqueeState,
            color = accentTone(Color(0xFF2DBBFF), isServiceDegraded),
            edgePaddingPx = edgePaddingPx,
        )
    }
}

@Composable
private fun StrictTopContextArc(
    remainingPercent: Float,
    compactThresholdPercent: Float?,
    isServiceDegraded: Boolean,
    strokeWidth: Dp,
    inset: Dp,
    modifier: Modifier = Modifier,
) {
    Canvas(modifier = modifier) {
        val minSide = minOf(size.width, size.height)
        val strokePx = strokeWidth.toPx()
        val insetPx = inset.toPx()
        val center = Offset(size.width / 2f, size.height / 2f)
        val radius = (minSide / 2f) - insetPx - (strokePx / 2f)
        val arcSize = Size(radius * 2f, radius * 2f)
        val topLeft = Offset(center.x - radius, center.y - radius)
        val startAngle = 180f
        val sweepAngle = 180f
        val segmentCount = 72
        val gapDegrees = 0.55f
        val remainingFraction = remainingPercent.coerceIn(0f, 100f) / 100f

        drawArc(
            color = WatchHomePalette.Track,
            startAngle = startAngle,
            sweepAngle = sweepAngle,
            useCenter = false,
            topLeft = topLeft,
            size = arcSize,
            style = Stroke(width = strokePx, cap = StrokeCap.Round),
        )

        repeat(segmentCount) { index ->
            val segmentStart = index / segmentCount.toFloat()
            val segmentEnd = (index + 1) / segmentCount.toFloat()
            val activeEnd = minOf(segmentEnd, remainingFraction)
            if (activeEnd <= segmentStart) {
                return@repeat
            }
            val activeSweep = sweepAngle * (activeEnd - segmentStart) - gapDegrees
            if (activeSweep <= 0f) {
                return@repeat
            }
            val color = semanticArcColor((segmentStart + activeEnd) / 2f, isServiceDegraded)
            drawArc(
                color = color.copy(alpha = 0.12f),
                startAngle = startAngle + sweepAngle * segmentStart,
                sweepAngle = activeSweep,
                useCenter = false,
                topLeft = topLeft,
                size = arcSize,
                style = Stroke(width = strokePx * 1.3f, cap = StrokeCap.Round),
            )
            drawArc(
                color = color,
                startAngle = startAngle + sweepAngle * segmentStart,
                sweepAngle = activeSweep,
                useCenter = false,
                topLeft = topLeft,
                size = arcSize,
                style = Stroke(width = strokePx, cap = StrokeCap.Round),
            )
        }

        if (remainingFraction > 0f) {
            val dotAngle = Math.toRadians((startAngle + sweepAngle * remainingFraction).toDouble())
            val dotCenter = Offset(
                x = center.x + (radius * cos(dotAngle)).toFloat(),
                y = center.y + (radius * sin(dotAngle)).toFloat(),
            )
            drawCircle(
                color = semanticArcColor(remainingFraction.coerceIn(0.02f, 1f), isServiceDegraded),
                radius = strokePx * 0.56f,
                center = dotCenter,
            )
        }

        compactThresholdPercent
            ?.takeIf { it in 0f..100f }
            ?.let { threshold ->
                drawContextThresholdMarker(
                    thresholdRemainingFraction = (100f - threshold).coerceIn(0f, 100f) / 100f,
                    startAngle = startAngle,
                    sweepAngle = sweepAngle,
                    center = center,
                    radius = radius,
                    strokeWidth = strokePx,
                    color = accentTone(WatchHomePalette.Amber, isServiceDegraded),
                )
            }
    }
}

private fun DrawScope.drawContextThresholdMarker(
    thresholdRemainingFraction: Float,
    startAngle: Float,
    sweepAngle: Float,
    center: Offset,
    radius: Float,
    strokeWidth: Float,
    color: Color,
) {
    val angle = Math.toRadians((startAngle + sweepAngle * thresholdRemainingFraction).toDouble())
    val dx = cos(angle).toFloat()
    val dy = sin(angle).toFloat()
    val inner = Offset(
        x = center.x + dx * (radius - strokeWidth * 1.35f),
        y = center.y + dy * (radius - strokeWidth * 1.35f),
    )
    val outer = Offset(
        x = center.x + dx * (radius + strokeWidth * 0.42f),
        y = center.y + dy * (radius + strokeWidth * 0.42f),
    )
    val markerCenter = Offset(
        x = center.x + dx * radius,
        y = center.y + dy * radius,
    )
    drawLine(
        color = Color.Black.copy(alpha = 0.62f),
        start = inner,
        end = outer,
        strokeWidth = strokeWidth * 0.7f,
        cap = StrokeCap.Round,
    )
    drawLine(
        color = color.copy(alpha = 0.98f),
        start = inner,
        end = outer,
        strokeWidth = strokeWidth * 0.46f,
        cap = StrokeCap.Round,
    )
    drawCircle(
        color = Color.Black.copy(alpha = 0.56f),
        radius = strokeWidth * 0.42f,
        center = markerCenter,
    )
    drawCircle(
        color = Color.White.copy(alpha = 0.9f),
        radius = strokeWidth * 0.2f,
        center = markerCenter,
    )
}

private fun semanticArcColor(fraction: Float, isServiceDegraded: Boolean): Color {
    val clamped = fraction.coerceIn(0f, 1f)
    val color = when {
        clamped < 0.34f -> lerp(WatchHomePalette.Red, WatchHomePalette.Amber, clamped / 0.34f)
        clamped < 0.68f -> lerp(WatchHomePalette.Amber, WatchHomePalette.Lime, (clamped - 0.34f) / 0.34f)
        else -> lerp(WatchHomePalette.Lime, WatchHomePalette.Green, (clamped - 0.68f) / 0.32f)
    }
    return accentTone(color, isServiceDegraded)
}

@Composable
private fun ContextReadout(
    state: HomeRoundVisualState,
    valueFont: TextUnit,
    warningFont: TextUnit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier,
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        if (state.compactWarningVisible) {
            Text(
                text = "即将压缩",
                color = WatchHomePalette.Amber,
                style = TextStyle(
                    fontSize = (warningFont.value * 0.72f).sp,
                    lineHeight = (warningFont.value * 0.72f).sp,
                    fontWeight = FontWeight.SemiBold,
                    platformStyle = PlatformTextStyle(includeFontPadding = false),
                ),
                maxLines = 1,
            )
        }
        Text(
            text = buildAnnotatedString {
                append(state.contextUsedText)
                withStyle(
                    SpanStyle(
                        fontSize = valueFont,
                        fontWeight = FontWeight.Medium,
                        color = WatchHomePalette.WhiteSoft,
                    ),
                ) {
                    append("/")
                    append(state.contextWindowText)
                }
            },
            color = Color.White,
            style = TextStyle(
                fontSize = valueFont,
                lineHeight = valueFont,
                fontWeight = FontWeight.Light,
                letterSpacing = 0.sp,
                platformStyle = PlatformTextStyle(includeFontPadding = false),
            ),
            maxLines = 1,
        )
    }
}

@Composable
internal fun HomeMetaGrid(
    state: HomeRoundVisualState,
    isServiceDegraded: Boolean,
    fontSize: TextUnit,
    iconSize: Dp,
    activityTrackWidth: Dp,
    activityTrackHeight: Dp,
    modifier: Modifier = Modifier,
    equalColumns: Boolean = true,
) {
    Column(
        modifier = modifier,
        verticalArrangement = Arrangement.spacedBy(1.dp),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .weight(1f),
            horizontalArrangement = Arrangement.spacedBy(2.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (equalColumns) {
                MetaGridCell(
                    modifier = Modifier.weight(1f),
                    alignment = Alignment.CenterStart,
                ) {
                    HomeMetaItem(
                        iconRes = R.drawable.ic_token_burn,
                        tint = accentTone(Color(0xFFFF5A36), isServiceDegraded),
                        label = state.totalTokensLabel,
                        fontSize = fontSize,
                        iconSize = iconSize,
                    )
                }
                MetaGridCell(
                    modifier = Modifier.weight(1f),
                    alignment = Alignment.CenterEnd,
                ) {
                    HomeMetaItem(
                        iconRes = R.drawable.ic_model_cube,
                        tint = accentTone(Color(0xFF47F2A0), isServiceDegraded),
                        label = state.modelLabel,
                        fontSize = fontSize,
                        iconSize = iconSize,
                    )
                }
            } else {
                HomeMetaItem(
                    iconRes = R.drawable.ic_token_burn,
                    tint = accentTone(Color(0xFFFF5A36), isServiceDegraded),
                    label = state.totalTokensLabel,
                    fontSize = fontSize,
                    iconSize = iconSize,
                )
                HomeMetaItem(
                    iconRes = R.drawable.ic_model_cube,
                    tint = accentTone(Color(0xFF47F2A0), isServiceDegraded),
                    label = state.modelLabel,
                    fontSize = fontSize,
                    iconSize = iconSize,
                    modifier = Modifier.weight(1f, fill = false),
                )
            }
        }
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .weight(1f),
            horizontalArrangement = Arrangement.spacedBy(2.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (equalColumns) {
                MetaGridCell(
                    modifier = Modifier.weight(1f),
                    alignment = Alignment.CenterStart,
                ) {
                    HomeMetaItem(
                        iconRes = R.drawable.ic_reasoning_brain,
                        tint = accentTone(WatchHomePalette.Purple, isServiceDegraded),
                        label = state.reasoningLabel,
                        fontSize = fontSize,
                        iconSize = iconSize,
                    )
                }
                MetaGridCell(
                    modifier = Modifier.weight(1f),
                    alignment = Alignment.CenterEnd,
                ) {
                    HomeActivityMetaItem(
                        isServiceDegraded = isServiceDegraded,
                        isLive = state.isActivityLive,
                        label = state.activityLabel,
                        fontSize = fontSize,
                        iconSize = iconSize,
                        trackWidth = activityTrackWidth,
                        trackHeight = activityTrackHeight,
                    )
                }
            } else {
                HomeMetaItem(
                    iconRes = R.drawable.ic_reasoning_brain,
                    tint = accentTone(WatchHomePalette.Purple, isServiceDegraded),
                    label = state.reasoningLabel,
                    fontSize = fontSize,
                    iconSize = iconSize,
                )
                HomeActivityMetaItem(
                    isServiceDegraded = isServiceDegraded,
                    isLive = state.isActivityLive,
                    label = state.activityLabel,
                    fontSize = fontSize,
                    iconSize = iconSize,
                    trackWidth = activityTrackWidth,
                    trackHeight = activityTrackHeight,
                    modifier = Modifier.weight(1f, fill = false),
                )
            }
        }
    }
}

@Composable
private fun MetaGridCell(
    modifier: Modifier = Modifier,
    alignment: Alignment,
    content: @Composable () -> Unit,
) {
    Box(
        modifier = modifier.fillMaxWidth(),
        contentAlignment = alignment,
    ) {
        content()
    }
}

@Composable
private fun HomeMetaItem(
    @DrawableRes iconRes: Int,
    tint: Color,
    label: String,
    fontSize: TextUnit,
    iconSize: Dp,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier.heightIn(min = iconSize + 2.dp),
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
            modifier = Modifier.weight(1f, fill = false),
            color = Color.White,
            style = metaTextStyle(fontSize),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

@Composable
private fun HomeActivityMetaItem(
    isServiceDegraded: Boolean,
    isLive: Boolean,
    label: String,
    fontSize: TextUnit,
    iconSize: Dp,
    trackWidth: Dp,
    trackHeight: Dp,
    modifier: Modifier = Modifier,
) {
    val tint = accentTone(Color(0xFF2DBBFF), isServiceDegraded)
    Row(
        modifier = modifier.heightIn(min = iconSize + 2.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        if (isLive) {
            Text(
                text = label,
                color = tint,
                style = metaTextStyle(fontSize),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        } else {
            val activityLabel = label
            val activityFontSize = when {
                activityLabel.length >= 9 -> (fontSize.value * 0.56f).sp
                activityLabel.length >= 7 -> (fontSize.value * 0.72f).sp
                else -> fontSize
            }
            Text(
                text = activityLabel,
                color = tint,
                style = metaTextStyle(activityFontSize),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

@Composable
private fun metaTextStyle(fontSize: TextUnit): TextStyle {
    return TextStyle(
        fontSize = fontSize,
        lineHeight = fontSize,
        fontWeight = FontWeight.Medium,
        platformStyle = PlatformTextStyle(includeFontPadding = false),
    )
}

@Composable
private fun HeatmapRibbon(
    state: HomeWeeklyHeatmapUiState,
    isServiceDegraded: Boolean,
    labelFont: TextUnit,
    modifier: Modifier = Modifier,
) {
    BoxWithConstraints(modifier = modifier) {
        val ribbonHeight = maxHeight
        val labelBandHeight = ribbonHeight * 0.17f
        val fallbackRowLabels = remember { weeklyHeatmapPlaceholderLabels() }
        val rowLabels = state.rows.map { it.dateLabel }.takeIf { labels ->
            labels.size == 7 && labels.all(String::isNotBlank)
        } ?: fallbackRowLabels
        val rowLabelWidth = maxWidth * 0.112f
        val rowLabelGap = maxWidth * 0.03f
        val rowLabelFont = labelFont
        val gridGap = 1.dp
        Column(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(1.dp),
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(labelBandHeight)
                    .padding(top = 1.dp, end = 1.dp),
            ) {
                Box(modifier = Modifier.width(rowLabelWidth + rowLabelGap))
                Box(modifier = Modifier.weight(1f)) {
                    HeatmapAxisLabels(
                        labels = heatmapAxisLabels(),
                        columnCount = 24,
                        gap = gridGap,
                        fontSize = labelFont,
                        modifier = Modifier.fillMaxSize(),
                    )
                }
            }
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .height((ribbonHeight - labelBandHeight - 1.dp).coerceAtLeast(24.dp)),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column(
                    modifier = Modifier
                        .width(rowLabelWidth)
                        .fillMaxHeight(),
                    verticalArrangement = Arrangement.spacedBy(gridGap),
                ) {
                    rowLabels.forEach { label ->
                        Box(
                            modifier = Modifier.weight(1f),
                            contentAlignment = Alignment.CenterEnd,
                        ) {
                            Text(
                                text = label,
                                modifier = Modifier.fillMaxWidth(),
                                color = WatchHomePalette.WhiteSoft,
                                style = heatmapAxisTextStyle(rowLabelFont),
                                textAlign = TextAlign.End,
                                maxLines = 1,
                            )
                        }
                    }
                }
                Box(modifier = Modifier.width(rowLabelGap))
                WeeklyHeatmapGrid(
                    state = state,
                    isServiceDegraded = isServiceDegraded,
                    gap = gridGap,
                    modifier = Modifier
                        .weight(1f)
                        .fillMaxHeight(),
                )
            }
        }
    }
}

@Composable
private fun HeatmapAxisLabels(
    labels: List<String>,
    columnCount: Int,
    gap: Dp,
    fontSize: TextUnit,
    modifier: Modifier = Modifier,
) {
    Layout(
        modifier = modifier,
        content = {
            labels.forEach { label ->
                Text(
                    text = label,
                    color = WatchHomePalette.WhiteSoft,
                    style = heatmapAxisTextStyle(fontSize),
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
        val height = if (constraints.hasBoundedHeight) {
            constraints.maxHeight
        } else {
            placeables.maxOfOrNull { it.height } ?: 0
        }
        val gapPx = gap.toPx()
        val safeColumnCount = columnCount.coerceAtLeast(1)
        val cellWidth = ((width.toFloat() - gapPx * (safeColumnCount - 1)) / safeColumnCount).coerceAtLeast(1f)
        layout(width, height) {
            placeables.forEachIndexed { index, placeable ->
                val hour = index * 2
                val anchorX = hour * (cellWidth + gapPx)
                val maxX = (width - placeable.width).coerceAtLeast(0).toFloat()
                val x = anchorX.coerceIn(0f, maxX).roundToInt()
                val y = (height - placeable.height - 1).coerceAtLeast(0)
                placeable.placeRelative(x, y)
            }
        }
    }
}

@Composable
private fun heatmapAxisTextStyle(fontSize: TextUnit): TextStyle {
    return TextStyle(
        fontSize = fontSize,
        lineHeight = fontSize,
        fontWeight = FontWeight.Medium,
        platformStyle = PlatformTextStyle(includeFontPadding = false),
    )
}

@Composable
private fun WeeklyHeatmapGrid(
    state: HomeWeeklyHeatmapUiState,
    isServiceDegraded: Boolean,
    gap: Dp,
    modifier: Modifier = Modifier,
) {
    Canvas(modifier = modifier.fillMaxWidth()) {
        val rows = state.rows.ifEmpty { List(7) { ai.openwatcher.watchapp.ui.HomeWeeklyHeatmapRowUiState() } }
        val rowCount = rows.size.coerceAtLeast(1)
        val columnCount = rows.maxOfOrNull { it.cells.size }?.coerceAtLeast(1) ?: 24
        val gapPx = gap.toPx()
        val cellWidth = ((size.width - gapPx * (columnCount - 1)) / columnCount).coerceAtLeast(1f)
        val cellHeight = ((size.height - gapPx * (rowCount - 1)) / rowCount).coerceAtLeast(1f)
        val radius = min(cellWidth, cellHeight) * 0.26f

        rows.forEachIndexed { rowIndex, row ->
            repeat(columnCount) { columnIndex ->
                val cell = row.cells.getOrElse(columnIndex) { ai.openwatcher.watchapp.ui.HomeWeeklyHeatmapCellUiState() }
                val x = columnIndex * (cellWidth + gapPx)
                val y = rowIndex * (cellHeight + gapPx)
                drawRoundRect(
                    color = weeklyHeatmapCellColor(cell.intensity, isServiceDegraded),
                    topLeft = Offset(x, y),
                    size = Size(cellWidth, cellHeight),
                    cornerRadius = CornerRadius(radius, radius),
                )
            }
        }
    }
}

internal fun weeklyHeatmapCellColor(normalizedIntensity: Float, isServiceDegraded: Boolean = false): Color {
    val clamped = normalizedIntensity.coerceIn(0f, 1f)
    val base = when {
        clamped <= 0f -> Color(0xFF161A19)
        clamped < 0.18f -> Color(0xFF213326)
        clamped < 0.36f -> Color(0xFF4E6F25)
        clamped < 0.56f -> Color(0xFFA4B61E)
        clamped < 0.78f -> Color(0xFFFFB320)
        else -> Color(0xFFFF5B2D)
    }
    return accentTone(base, isServiceDegraded)
}

@Composable
private fun MiniHeatmapBars(
    bars: List<MiniBarUiState>,
    isServiceDegraded: Boolean,
    modifier: Modifier = Modifier,
) {
    Canvas(modifier = modifier.fillMaxWidth()) {
        val maxIntensity = bars.maxOfOrNull { it.intensity }?.coerceAtLeast(0.0001f) ?: 1f
        val laneGeometry = miniHeatmapLaneGeometry(bars.size)
        val topPadding = size.height * 0.035f
        val baseline = size.height - 3f
        val maxBarHeight = (baseline - topPadding).coerceAtLeast(1f)
        val barWidth = size.width * laneGeometry.barWidthFraction
        val gap = size.width * laneGeometry.gapFraction

        drawRoundRect(
            color = WatchHomePalette.Divider,
            topLeft = Offset(0f, baseline),
            size = Size(size.width, 1f),
            cornerRadius = CornerRadius(1f, 1f),
        )

        bars.forEachIndexed { index, item ->
            val normalized = (item.intensity / maxIntensity).coerceIn(0f, 1f)
            val x = gap + index * (barWidth + gap)
            val height = (maxBarHeight * normalized).let {
                if (item.intensity > 0f) it.coerceAtLeast(3f) else 0f
            }
            val color = miniHeatmapBarColor(normalized, isServiceDegraded)

            if (height > 0f) {
                drawRoundRect(
                    color = color.copy(alpha = color.alpha * 0.18f),
                    topLeft = Offset(x, baseline - height - 1.5f),
                    size = Size(barWidth, height + 1.5f),
                    cornerRadius = CornerRadius(barWidth, barWidth),
                )
                drawRoundRect(
                    color = color,
                    topLeft = Offset(x, baseline - height),
                    size = Size(barWidth, height),
                    cornerRadius = CornerRadius(barWidth, barWidth),
                )
            }
        }
    }
}

internal fun miniHeatmapBarColor(normalizedIntensity: Float, isServiceDegraded: Boolean = false): Color {
    val clamped = normalizedIntensity.coerceIn(0f, 1f)
    val hue = when {
        clamped < 0.5f -> lerp(WatchHomePalette.Green, WatchHomePalette.Amber, clamped / 0.5f)
        else -> lerp(WatchHomePalette.Amber, WatchHomePalette.Red, (clamped - 0.5f) / 0.5f)
    }
    val dimBase = Color(0xFF102618)
    val color = lerp(dimBase, hue, 0.42f + clamped * 0.58f)
        .copy(alpha = 0.48f + clamped * 0.52f)
    return accentTone(color, isServiceDegraded)
}

@Composable
private fun QuotaGapRing(
    state: QuotaRingUiState,
    label: String,
    ringSize: Dp,
    percentFont: TextUnit,
    modifier: Modifier = Modifier,
) {
    val gapSweep = if (label.length <= 3) 44f else 50f
    val ringStart = 90f + (gapSweep / 2f)
    val ringSweep = 360f - gapSweep
    val renderState = buildQuotaRingRenderState(state)
    Box(
        modifier = modifier.size(ringSize),
        contentAlignment = Alignment.Center,
    ) {
        Canvas(modifier = Modifier.fillMaxSize()) {
            val geometry = buildQuotaRingGeometry(size.minDimension * 0.05f)
            val outerStrokeWidth = geometry.outerStrokeWidth
            val innerStrokeWidth = geometry.innerStrokeWidth
            val ringGap = geometry.ringGap
            val outerInset = outerStrokeWidth / 2f
            val outerArcSize = Size(size.width - outerStrokeWidth, size.height - outerStrokeWidth)
            val outerTopLeft = Offset(outerInset, outerInset)
            val segmentCount = 72
            val gapDegrees = 0.5f
            val innerInset = outerInset + outerStrokeWidth + ringGap
            val innerArcSizeValue = size.minDimension - (innerInset * 2f)
            val innerArcSize = Size(innerArcSizeValue, innerArcSizeValue)
            val innerTopLeft = Offset(innerInset, innerInset)

            drawArc(
                color = WatchHomePalette.Track,
                startAngle = ringStart,
                sweepAngle = ringSweep,
                useCenter = false,
                topLeft = outerTopLeft,
                size = outerArcSize,
                style = Stroke(width = outerStrokeWidth, cap = StrokeCap.Round),
            )

            if (renderState.showTimeRing && innerArcSizeValue > 0f) {
                drawArc(
                    color = timeRingElapsedTrackColor(),
                    startAngle = ringStart,
                    sweepAngle = ringSweep,
                    useCenter = false,
                    topLeft = innerTopLeft,
                    size = innerArcSize,
                    style = Stroke(width = innerStrokeWidth, cap = StrokeCap.Round),
                )
            }

            drawSegmentedQuotaRing(
                fraction = renderState.remainingFraction,
                segmentCount = segmentCount,
                gapDegrees = gapDegrees,
                ringStart = ringStart,
                ringSweep = ringSweep,
                topLeft = outerTopLeft,
                size = outerArcSize,
                strokeWidth = outerStrokeWidth,
                isDimmed = renderState.isDimmed,
            )

            if (renderState.showTimeRing && innerArcSizeValue > 0f) {
                drawSegmentedTimeRing(
                    fraction = renderState.timeRemainingFraction ?: 0f,
                    segmentCount = segmentCount,
                    gapDegrees = 0.35f,
                    ringStart = ringStart,
                    ringSweep = ringSweep,
                    topLeft = innerTopLeft,
                    size = innerArcSize,
                    strokeWidth = innerStrokeWidth,
                    isDimmed = renderState.isDimmed,
                )
            }
        }

        Text(
            text = percentageText(state),
            modifier = Modifier.offset(y = -(ringSize * 0.11f)),
            color = Color.White,
            fontSize = percentFont,
            fontWeight = FontWeight.Bold,
            letterSpacing = 0.sp,
        )

        QuotaResetCountdown(
            label = state.remainingLabel,
            isDimmed = state.isDimmed,
            modifier = Modifier
                .align(Alignment.Center)
                .offset(y = ringSize * 0.13f),
        )

        QuotaGapLabel(
            label = label,
            isDimmed = state.isDimmed,
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .offset(y = 3.dp),
        )
    }
}

private fun DrawScope.drawSegmentedQuotaRing(
    fraction: Float,
    segmentCount: Int,
    gapDegrees: Float,
    ringStart: Float,
    ringSweep: Float,
    topLeft: Offset,
    size: Size,
    strokeWidth: Float,
    isDimmed: Boolean,
) {
    repeat(segmentCount) { index ->
        val segmentStart = index / segmentCount.toFloat()
        val segmentEnd = (index + 1) / segmentCount.toFloat()
        val activeEnd = minOf(segmentEnd, fraction)
        if (activeEnd <= segmentStart) {
            return@repeat
        }
        val activeSweep = ringSweep * (activeEnd - segmentStart) - gapDegrees
        if (activeSweep <= 0f) {
            return@repeat
        }
        val color = if (isDimmed) {
            accentTone(WatchHomePalette.WhiteSoft, true)
        } else {
            semanticArcColor((segmentStart + activeEnd) / 2f, false)
        }
        drawArc(
            color = color.copy(alpha = 0.12f),
            startAngle = ringStart + ringSweep * segmentStart,
            sweepAngle = activeSweep,
            useCenter = false,
            topLeft = topLeft,
            size = size,
            style = Stroke(width = strokeWidth * 1.3f, cap = StrokeCap.Round),
        )
        drawArc(
            color = color,
            startAngle = ringStart + ringSweep * segmentStart,
            sweepAngle = activeSweep,
            useCenter = false,
            topLeft = topLeft,
            size = size,
            style = Stroke(width = strokeWidth, cap = StrokeCap.Round),
        )
    }
}

private fun DrawScope.drawSegmentedTimeRing(
    fraction: Float,
    segmentCount: Int,
    gapDegrees: Float,
    ringStart: Float,
    ringSweep: Float,
    topLeft: Offset,
    size: Size,
    strokeWidth: Float,
    isDimmed: Boolean,
) {
    val color = timeRingRemainingColor(isDimmed)
    repeat(segmentCount) { index ->
        val segmentStart = index / segmentCount.toFloat()
        val segmentEnd = (index + 1) / segmentCount.toFloat()
        val activeEnd = minOf(segmentEnd, fraction)
        if (activeEnd <= segmentStart) {
            return@repeat
        }
        val activeSweep = ringSweep * (activeEnd - segmentStart) - gapDegrees
        if (activeSweep <= 0f) {
            return@repeat
        }
        drawArc(
            color = color.copy(alpha = 0.16f),
            startAngle = ringStart + ringSweep * segmentStart,
            sweepAngle = activeSweep,
            useCenter = false,
            topLeft = topLeft,
            size = size,
            style = Stroke(width = strokeWidth * 1.4f, cap = StrokeCap.Round),
        )
        drawArc(
            color = color.copy(alpha = 0.92f),
            startAngle = ringStart + ringSweep * segmentStart,
            sweepAngle = activeSweep,
            useCenter = false,
            topLeft = topLeft,
            size = size,
            style = Stroke(width = strokeWidth, cap = StrokeCap.Round),
        )
    }
}

@Composable
private fun QuotaResetCountdown(
    label: String,
    isDimmed: Boolean,
    modifier: Modifier = Modifier,
) {
    if (label.isBlank() || label == "--") {
        return
    }
    val fontSize = when {
        label.length >= 10 -> 5.4.sp
        label.length >= 8 -> 5.9.sp
        else -> 6.4.sp
    }
    Text(
        text = label,
        modifier = modifier.width(50.dp),
        color = if (isDimmed) accentTone(WatchHomePalette.WhiteSoft, true) else WatchHomePalette.WhiteSoft,
        style = TextStyle(
            fontSize = fontSize,
            lineHeight = fontSize,
            fontWeight = FontWeight.Medium,
            platformStyle = PlatformTextStyle(includeFontPadding = false),
        ),
        maxLines = 1,
        overflow = TextOverflow.Ellipsis,
        textAlign = TextAlign.Center,
    )
}

@Composable
private fun QuotaGapLabel(
    label: String,
    isDimmed: Boolean,
    modifier: Modifier = Modifier,
) {
    val textStyle = TextStyle(
        fontSize = 9.sp,
        lineHeight = 9.sp,
        fontWeight = FontWeight.Medium,
        platformStyle = PlatformTextStyle(includeFontPadding = false),
    )
    Text(
        text = label.replace(" ", ""),
        modifier = modifier
            .width(28.dp)
            .height(12.dp),
        color = if (isDimmed) accentTone(WatchHomePalette.WhiteSoft, true) else WatchHomePalette.WhiteSoft,
        style = textStyle,
        maxLines = 1,
        textAlign = TextAlign.Center,
    )
}

private fun accentTone(color: Color, isServiceDegraded: Boolean): Color {
    if (!isServiceDegraded) {
        return color
    }
    val luminance = (color.red * 0.299f) + (color.green * 0.587f) + (color.blue * 0.114f)
    return lerp(
        Color(luminance, luminance, luminance, color.alpha),
        Color(0xFF8C96A5).copy(alpha = color.alpha),
        0.35f,
    )
}

private fun percentageText(state: QuotaRingUiState): String {
    return if (state.remainingLabel == "--" && state.remainingPercent <= 0f && state.usedPercent <= 0f) {
        "--"
    } else {
        "${state.remainingPercent.toInt()}%"
    }
}

@Composable
private fun HomeQuotaTipBubble(
    text: String,
    maxWidth: Dp,
    modifier: Modifier = Modifier,
) {
    val fontSize = homeQuotaBubbleFontSize(text)
    Box(
        modifier = modifier
            .widthIn(max = maxWidth)
            .clip(RoundedCornerShape(StatusCapsuleDefaults.bubbleCorner))
            .background(Color(0xF211151C))
            .padding(horizontal = StatusCapsuleDefaults.tightHorizontalPadding, vertical = 8.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = text,
            color = Color.White,
            style = TextStyle(
                fontSize = fontSize,
                lineHeight = fontSize * 1.16f,
                fontWeight = FontWeight.Medium,
                platformStyle = PlatformTextStyle(includeFontPadding = false),
            ),
            textAlign = TextAlign.Center,
        )
    }
}

internal fun homeQuotaBubbleFontSize(text: String): TextUnit {
    val length = text.trim().length
    return when {
        length >= 28 -> 12.4.sp
        length >= 24 -> 13.2.sp
        length >= 18 -> 14.sp
        else -> 15.2.sp
    }
}

@Composable
private fun HomeErrorBanner(
    errors: List<String>,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier
            .clip(CircleShape)
            .background(Color(0x26FF9C2A))
            .padding(horizontal = 10.dp, vertical = 4.dp),
        horizontalArrangement = Arrangement.spacedBy(6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Canvas(modifier = Modifier.size(5.dp)) {
            drawCircle(Color(0xFFFFB13B))
        }
        Text(
            text = errors.first(),
            color = Color(0xFFFFCF78),
            fontSize = 7.sp,
            lineHeight = 9.sp,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
        )
    }
}
