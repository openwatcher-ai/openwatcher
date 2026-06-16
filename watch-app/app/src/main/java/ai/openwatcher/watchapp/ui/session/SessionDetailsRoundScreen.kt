package ai.openwatcher.watchapp.ui.session

import android.graphics.Paint
import android.graphics.RectF
import android.graphics.Typeface
import android.graphics.drawable.Drawable
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.ScrollState
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.graphics.nativeCanvas
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.input.pointer.PointerEventPass
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.PlatformTextStyle
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.annotation.DrawableRes
import androidx.core.content.res.ResourcesCompat
import kotlin.math.atan2
import kotlin.math.cos
import kotlin.math.floor
import kotlin.math.min
import kotlin.math.roundToInt
import kotlin.math.sin
import kotlin.math.sqrt
import ai.openwatcher.watchapp.R
import ai.openwatcher.watchapp.ScreenshotLongPressTimeoutMs
import ai.openwatcher.watchapp.ui.AgentMessageStreamStatus
import ai.openwatcher.watchapp.ui.SessionDetailsUiState
import ai.openwatcher.watchapp.ui.components.curvedTitlePathLength
import ai.openwatcher.watchapp.ui.components.degradedVisuals
import ai.openwatcher.watchapp.ui.components.drawCurvedTitle
import ai.openwatcher.watchapp.ui.components.rememberCurvedTitleMarqueeState
import ai.openwatcher.watchapp.ui.home.HomeRoundVisualState
import ai.openwatcher.watchapp.ui.home.homeSessionTitleBaselineInsetScale
import ai.openwatcher.watchapp.ui.home.homeSessionTitleTextScale

private val SessionScreenBackground = Color(0xFF05070B)
private val SessionRingTrack = Color(0x181D2937)
private val SessionTitleText = Color(0xFF2DBBFF)
private val SessionContextTrack = Color(0x20FFFFFF)
private val SessionContextRed = Color(0xFFFF4221)
private val SessionContextAmber = Color(0xFFFFBC27)
private val SessionContextLime = Color(0xFFD7F60F)
private val SessionContextGreen = Color(0xFF74F34A)
private val SessionEmptyText = Color(0xFF73879A)
private val SessionContextWindowText = Color(0xFFADB9CC)
private val SessionAgentStatusText = Color(0xFF78BFFF)
private val SessionAgentBodyText = Color(0xFFE8F2FF)
private val SessionAgentMutedText = Color(0xFF8796A8)
private val SessionAgentErrorText = Color(0xFFFF9A8A)
private val SessionRunningRankColors = listOf(
    Color(0xFFFF4D2E),
    Color(0xFFFF7A24),
    Color(0xFFFFA62B),
    Color(0xFFFFC44D),
    Color(0xFFFFDD7A),
)
private val SessionIdleRankColors = listOf(
    Color(0xFF38BFE4),
    Color(0xFF3E9AC5),
    Color(0xFF587E9B),
    Color(0xFF758EA3),
    Color(0xFFA3AFBA),
)

private const val TopAngleDegrees = -90f
private const val SessionCursorConnectingCycleDurationMs = 2_000
private const val SessionOuterRingStrokeRatio = 0.036f
private const val SessionOuterRingInsetRatio = 0.008f
private const val SessionDetailArcCenterStrokeOffset = 1.14f
private const val SessionContextRingStrokeRatio = 0.022f
private const val SessionMetadataBaseTextRatio = 0.058f
private const val SessionMetadataDefaultRadialInsetFraction = 0.42f
private const val SessionMetadataSidePaddingScale = 0.32f
private const val SessionMetadataMinTextScale = 0.78f
private const val SessionMetadataIconGapScale = 0.20f
private const val SessionMetadataIconSizeScaleModel = 1.10f
private const val SessionMetadataIconSizeScaleReasoning = 1.12f
private const val SessionMetadataIconSizeScaleToken = 1.10f
private const val SessionMetadataTopTokenRadialInsetFraction = 0.70f
private const val SessionMetadataTopTokenTextScale = 1.0f
private const val SessionMetadataBottomContextTextScale = 1.12f
private const val SessionAgentStatusTextRatio = 0.038f
private const val SessionAgentBodyTextRatio = 0.052f

enum class SessionSelectionSource {
    Tap,
}

data class SessionSelectionChange(
    val index: Int,
    val sessionId: String,
    val source: SessionSelectionSource,
)

data class SessionDetailSegmentUiModel(
    val sessionId: String,
    val title: String,
    val activeLabel: String,
    val agentStatusLine: String,
    val contextLabel: String,
    val contextProgress: Float,
    val contextCompactThresholdPercent: Float?,
    val compactWarningVisible: Boolean,
    val totalTokensLabel: String,
    val model: String,
    val effort: String,
    val isActiveNow: Boolean,
    val runtimePhaseLabel: String,
    val activeMinutes: Int,
    val activityRank: Int,
    val sourceIndex: Int,
)

data class SessionDetailsRoundUiModel(
    val selectedIndex: Int,
    val selectionCursorIndex: Float?,
    val segments: List<SessionDetailSegmentUiModel>,
    val emptyMessage: String,
    val latestAgentMessage: String?,
    val latestAgentMessageAtLabel: String?,
    val agentMessageStreamStatus: AgentMessageStreamStatus,
    val agentMessageError: String?,
    val isServiceDegraded: Boolean,
)

interface SessionSelectionController {
    fun select(index: Int, source: SessionSelectionSource = SessionSelectionSource.Tap)
}

fun SessionDetailsUiState.toRoundUiModel(): SessionDetailsRoundUiModel {
    if (rows.isEmpty()) {
        return SessionDetailsRoundUiModel(
            selectedIndex = 0,
            selectionCursorIndex = null,
            segments = emptyList(),
            emptyMessage = emptyMessage,
            latestAgentMessage = null,
            latestAgentMessageAtLabel = null,
            agentMessageStreamStatus = AgentMessageStreamStatus.Disconnected,
            agentMessageError = null,
            isServiceDegraded = isServiceDegraded,
        )
    }

    val visibleRows = rows
    val selectedSourceIndex = visibleRows.indexOfFirst { it.isSelected }.takeIf { it >= 0 }
        ?: selectedIndex.coerceIn(0, visibleRows.lastIndex)

    val orderedSegments = visibleRows.mapIndexed { index, row ->
        val activeLabel = normalizeRecentLabel(row.lastActiveLabel)
        SessionDetailSegmentUiModel(
            sessionId = row.sessionId.ifBlank { "$index:${row.title}" },
            title = row.title,
            activeLabel = activeLabel,
            agentStatusLine = row.agentStatusLine,
            contextLabel = row.contextLabel,
            contextProgress = row.contextPressurePercent,
            contextCompactThresholdPercent = row.contextCompactThresholdPercent,
            compactWarningVisible = row.contextCompactWarning,
            totalTokensLabel = row.tokensLabel,
            model = row.model,
            effort = row.reasoningLabel,
            isActiveNow = row.isActiveNow,
            runtimePhaseLabel = row.runtimePhaseLabel,
            activeMinutes = row.lastActiveAgoMinutes.coerceAtLeast(0),
            activityRank = 0,
            sourceIndex = index,
        )
    }
    val runningRankBySessionId = orderedSegments
        .filter { it.isActiveNow }
        .mapIndexed { rank, segment -> segment.sessionId to sessionActivityRankForSortedIndex(rank) }
        .toMap()
    val idleRankBySessionId = orderedSegments
        .filterNot { it.isActiveNow }
        .mapIndexed { rank, segment -> segment.sessionId to sessionActivityRankForSortedIndex(rank) }
        .toMap()
    val rankedSegments = orderedSegments.map { segment ->
        val rank = if (segment.isActiveNow) {
            runningRankBySessionId[segment.sessionId]
        } else {
            idleRankBySessionId[segment.sessionId]
        } ?: 0
        segment.copy(activityRank = rank)
    }

    return SessionDetailsRoundUiModel(
        selectedIndex = selectedSourceIndex,
        selectionCursorIndex = selectionCursorIndex,
        emptyMessage = emptyMessage,
        segments = rankedSegments,
        latestAgentMessage = latestAgentMessage,
        latestAgentMessageAtLabel = latestAgentMessageAtLabel,
        agentMessageStreamStatus = agentMessageStreamStatus,
        agentMessageError = agentMessageError,
        isServiceDegraded = isServiceDegraded,
    )
}

@Composable
fun SessionDetailsRoundScreen(
    state: SessionDetailsRoundUiModel,
    modifier: Modifier = Modifier,
    messageScrollState: ScrollState = rememberScrollState(),
    onSelectionChanged: ((SessionSelectionChange) -> Unit)? = null,
) {
    if (state.segments.isEmpty()) {
        EmptySessionDetails(modifier = modifier, message = state.emptyMessage)
        return
    }

    val controller = remember(state.segments, onSelectionChanged) {
        object : SessionSelectionController {
            override fun select(index: Int, source: SessionSelectionSource) {
                val clamped = index.coerceIn(0, state.segments.lastIndex)
                onSelectionChanged?.invoke(
                    SessionSelectionChange(
                        index = state.segments[clamped].sourceIndex,
                        sessionId = state.segments[clamped].sessionId,
                        source = source,
                    ),
                )
            }
        }
    }

    BoxWithConstraints(
        modifier = modifier
            .fillMaxSize()
            .clip(CircleShape)
            .background(SessionScreenBackground)
            .pointerInput(state.segments) {
                awaitEachGesture {
                    val downEvent = awaitPointerEvent(PointerEventPass.Final)
                    val downChange = downEvent.changes.firstOrNull { it.pressed } ?: return@awaitEachGesture
                    val downPosition = downChange.position
                    val downUptime = downChange.uptimeMillis
                    val touchSlopSquared = viewConfiguration.touchSlop * viewConfiguration.touchSlop
                    var movedBeyondTouchSlop = false
                    var consumedByAnotherHandler = downChange.isConsumed

                    while (true) {
                        val event = awaitPointerEvent(PointerEventPass.Final)
                        consumedByAnotherHandler = consumedByAnotherHandler || event.changes.any { it.isConsumed }
                        val activeChange = event.changes.firstOrNull { it.id == downChange.id }
                            ?: event.changes.firstOrNull()
                            ?: continue
                        val dx = activeChange.position.x - downPosition.x
                        val dy = activeChange.position.y - downPosition.y
                        if (dx * dx + dy * dy > touchSlopSquared) {
                            movedBeyondTouchSlop = true
                        }
                        if (!activeChange.pressed) {
                            val pressDurationMs = activeChange.uptimeMillis - downUptime
                            val sectorIndex = if (
                                shouldSelectSessionFromTap(
                                    movedBeyondTouchSlop = movedBeyondTouchSlop,
                                    consumedByAnotherHandler = consumedByAnotherHandler,
                                    pressDurationMs = pressDurationMs,
                                )
                            ) {
                                locateSessionSector(
                                    tapOffset = activeChange.position,
                                    canvasSize = size,
                                    segmentCount = state.segments.size,
                                )
                            } else {
                                null
                            }
                            sectorIndex?.let { controller.select(it, SessionSelectionSource.Tap) }
                            break
                        }
                    }
                }
            }
            .degradedVisuals(state.isServiceDegraded),
        contentAlignment = Alignment.Center,
    ) {
        val density = LocalDensity.current
        val diameter = minOf(maxWidth, maxHeight)
        val diameterPx = with(density) { diameter.toPx() }
        val safeSelectedIndex = sessionCursorDisplayIndex(
            cursorIndex = state.selectionCursorIndex ?: state.selectedIndex.toFloat(),
            segmentCount = state.segments.size,
        )
        val connectingCursorOffset = rememberSessionConnectingCursorOffset(
            status = state.agentMessageStreamStatus,
            segmentCount = state.segments.size,
        )
        val selectedSegment = state.segments[safeSelectedIndex]
        val agentStatusFont = (diameterPx / density.density * SessionAgentStatusTextRatio).sp
        val agentBodyFont = (diameterPx / density.density * SessionAgentBodyTextRatio).sp
        val agentSquareSidePx = sessionContextInnerSquareSidePx(diameterPx)
        val agentSquareSide = with(density) { agentSquareSidePx.toDp() }

        SessionOuterRing(
            segments = state.segments,
            selectedIndex = safeSelectedIndex,
            selectionCursorIndex = state.selectionCursorIndex,
            connectingCursorOffset = connectingCursorOffset,
            modifier = Modifier.fillMaxSize(),
        )
        SessionAgentMessagePane(
            message = state.latestAgentMessage,
            statusLine = sessionAgentStatusLine(segment = selectedSegment),
            status = state.agentMessageStreamStatus,
            error = state.agentMessageError,
            statusFont = agentStatusFont,
            bodyFont = agentBodyFont,
            scrollState = messageScrollState,
            modifier = Modifier
                .align(Alignment.Center)
                .size(agentSquareSide),
        )
    }
}

internal fun sessionFooterTokenLabel(contextLabel: String): String {
    return contextLabel.ifBlank { "--" }
}

internal fun sessionHeaderPrimaryLabel(totalTokensLabel: String): String {
    return totalTokensLabel.ifBlank { "--" }
}

internal fun sessionAgentStatusLine(
    segment: SessionDetailSegmentUiModel,
): String {
    return segment.agentStatusLine.ifBlank { sessionRecentStatusLabel(segment.activeMinutes) }
}

internal fun sessionRecentStatusLabel(activeMinutes: Int): String {
    val minutes = activeMinutes.coerceAtLeast(1)
    val compactLabel = when {
        minutes < 60 -> "${minutes}m"
        minutes < 1_440 -> "${(minutes / 60).coerceAtLeast(1)}h"
        else -> "${(minutes / 1_440).coerceAtLeast(1)}d"
    }
    return "最近：${compactLabel}前"
}

@Composable
private fun SessionSideMetaBlock(
    iconRes: Int,
    tint: Color,
    label: String,
    fontSize: TextUnit,
    iconSize: androidx.compose.ui.unit.Dp,
    trackLength: androidx.compose.ui.unit.Dp,
    rotationDegrees: Float,
    modifier: Modifier = Modifier,
) {
    Box(
        modifier = modifier,
        contentAlignment = Alignment.Center,
    ) {
        Row(
            modifier = Modifier
                .width(trackLength)
                .rotate(rotationDegrees),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(3.dp, Alignment.CenterHorizontally),
        ) {
            Icon(
                painter = painterResource(id = iconRes),
                contentDescription = null,
                tint = tint,
                modifier = Modifier.size(iconSize),
            )
            Text(
                text = label.ifBlank { "--" },
                modifier = Modifier.weight(1f),
                color = Color.White,
                style = sessionTextStyle(fontSize, fontSize, FontWeight.Medium),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

@Composable
private fun SessionAgentMessagePane(
    message: String?,
    statusLine: String,
    status: AgentMessageStreamStatus,
    error: String?,
    statusFont: TextUnit,
    bodyFont: TextUnit,
    scrollState: ScrollState,
    modifier: Modifier = Modifier,
) {
    val hasMessage = !message.isNullOrBlank()
    val displayText = when {
        error != null -> error
        hasMessage -> message.orEmpty()
        status == AgentMessageStreamStatus.Connecting -> "连接中"
        status == AgentMessageStreamStatus.Disconnected -> "连接断开"
        else -> "等待 agent 回复"
    }
    val bodyColor = when {
        error != null -> SessionAgentErrorText
        message.isNullOrBlank() -> SessionAgentMutedText
        else -> SessionAgentBodyText
    }

    Column(
        modifier = modifier,
        verticalArrangement = Arrangement.spacedBy(3.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        val statusLineHeight = (statusFont.value * 1.08f).sp
        val bodyLineHeight = (bodyFont.value * 1.22f).sp

        Text(
            text = statusLine,
            modifier = Modifier.fillMaxWidth(),
            color = SessionAgentStatusText,
            style = sessionTextStyle(statusFont, statusLineHeight, FontWeight.SemiBold),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            textAlign = TextAlign.Center,
        )
        BoxWithConstraints(
            modifier = Modifier
                .fillMaxWidth()
                .weight(1f)
                .clipToBounds(),
            contentAlignment = if (hasMessage) Alignment.TopCenter else Alignment.Center,
        ) {
            if (hasMessage) {
                val viewportHeightPx = with(LocalDensity.current) { maxHeight.toPx() }
                var centerShortMessage by remember(displayText, viewportHeightPx) {
                    mutableStateOf(true)
                }
                Column(
                    modifier = Modifier
                        .fillMaxSize()
                        .verticalScroll(scrollState),
                    verticalArrangement = if (centerShortMessage) {
                        Arrangement.Center
                    } else {
                        Arrangement.Top
                    },
                    horizontalAlignment = Alignment.CenterHorizontally,
                ) {
                    Text(
                        text = displayText,
                        modifier = Modifier.fillMaxWidth(),
                        color = bodyColor,
                        style = sessionTextStyle(bodyFont, bodyLineHeight, FontWeight.Medium),
                        textAlign = TextAlign.Center,
                        onTextLayout = { result ->
                            centerShortMessage = result.size.height <= viewportHeightPx
                        },
                    )
                }
            } else {
                Text(
                    text = displayText,
                    modifier = Modifier.fillMaxWidth(),
                    color = bodyColor,
                    style = sessionTextStyle(statusFont, statusFont, FontWeight.SemiBold),
                    textAlign = TextAlign.Center,
                )
            }
        }
    }
}

@Composable
private fun SessionContextReadout(
    state: HomeRoundVisualState,
    primaryFont: TextUnit,
    secondaryFont: TextUnit,
    bottomPadding: androidx.compose.ui.unit.Dp,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .padding(bottom = bottomPadding),
        verticalArrangement = Arrangement.spacedBy(1.dp, Alignment.Bottom),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        if (state.compactWarningVisible) {
            Text(
                text = "即将压缩",
                color = SessionContextAmber,
                style = TextStyle(
                    fontSize = (secondaryFont.value * 0.72f).sp,
                    lineHeight = (secondaryFont.value * 0.72f).sp,
                    fontWeight = FontWeight.SemiBold,
                    textAlign = TextAlign.Center,
                    platformStyle = PlatformTextStyle(includeFontPadding = false),
                ),
                maxLines = 1,
                textAlign = TextAlign.Center,
            )
        }
        Text(
            text = buildAnnotatedString {
                append(state.contextUsedText)
                withStyle(
                    SpanStyle(
                        fontSize = secondaryFont,
                        fontWeight = FontWeight.SemiBold,
                        color = SessionContextWindowText,
                    ),
                ) {
                    append("/")
                    append(state.contextWindowText)
                }
            },
            color = Color.White,
            style = TextStyle(
                fontSize = primaryFont,
                lineHeight = primaryFont,
                fontWeight = FontWeight.Medium,
                letterSpacing = 0.sp,
                textAlign = TextAlign.Center,
                platformStyle = PlatformTextStyle(includeFontPadding = false),
            ),
            maxLines = 1,
            textAlign = TextAlign.Center,
        )
    }
}

private fun SessionDetailSegmentUiModel.toHomeRoundVisualState(): HomeRoundVisualState {
    val contextUsedText = contextLabel.substringBefore("/", "--").trim().uppercase().ifBlank { "--" }
    val contextWindowText = contextLabel.substringAfter("/", "--").trim().uppercase().ifBlank { "--" }
    val activityLabel = activeLabel.takeIf { it.isNotBlank() && it != "--" } ?: "--"
    return HomeRoundVisualState(
        sessionTitle = title,
        contextUsedText = contextUsedText,
        contextWindowText = contextWindowText,
        contextRemainingPercent = (100f - contextProgress).coerceIn(0f, 100f),
        contextPressurePercent = contextProgress.coerceIn(0f, 100f),
        contextCompactThresholdPercent = contextCompactThresholdPercent?.coerceIn(0f, 100f),
        compactWarningVisible = compactWarningVisible,
        totalTokensLabel = totalTokensLabel.ifBlank { "0" },
        modelLabel = model.ifBlank { "--" },
        reasoningLabel = effort.ifBlank { "--" },
        activityLabel = activityLabel,
        isActivityLive = isActiveNow,
    )
}

internal fun sessionContextInnerSquareSidePx(diameter: Float): Float {
    val contextInnerRadius = sessionContextRingCenterRadius(diameter) -
        (diameter * SessionContextRingStrokeRatio / 2f)
    return contextInnerRadius.coerceAtLeast(0f) * sqrt(2f)
}

private fun sessionOuterRingStrokeWidth(diameter: Float): Float {
    return diameter * SessionOuterRingStrokeRatio
}

private fun sessionOuterRingInset(diameter: Float): Float {
    return diameter * SessionOuterRingInsetRatio
}

private fun sessionOuterRingCenterRadius(diameter: Float): Float {
    val strokeWidth = sessionOuterRingStrokeWidth(diameter)
    return (diameter / 2f) - sessionOuterRingInset(diameter) - (strokeWidth / 2f)
}

private fun sessionContextRingCenterRadius(diameter: Float): Float {
    return sessionOuterRingCenterRadius(diameter) -
        sessionOuterRingStrokeWidth(diameter) * SessionDetailArcCenterStrokeOffset
}

@Composable
private fun EmptySessionDetails(
    modifier: Modifier,
    message: String,
) {
    Box(
        modifier = modifier
            .fillMaxSize()
            .clip(CircleShape)
            .background(SessionScreenBackground)
            .degradedVisuals(false),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = message,
            color = SessionEmptyText,
            style = sessionTextStyle(11.sp, 13.sp),
            textAlign = TextAlign.Center,
        )
    }
}

@Composable
private fun SessionOuterRing(
    segments: List<SessionDetailSegmentUiModel>,
    selectedIndex: Int,
    selectionCursorIndex: Float?,
    connectingCursorOffset: Float,
    modifier: Modifier = Modifier,
) {
    val selectedTitle = segments[selectedIndex].title
    val cursorIndex = sessionCursorAccentPosition(
        baseCursorIndex = selectionCursorIndex ?: selectedIndex.toFloat(),
        connectingCursorOffset = connectingCursorOffset,
        segmentCount = segments.size,
    )
    val density = LocalDensity.current
    val context = LocalContext.current
    val metadataIconDrawables = remember(context) {
        SessionMetadataArcIcon.entries.associateWith { icon ->
            ResourcesCompat.getDrawable(context.resources, icon.drawableRes, context.theme)?.mutate()
        }
    }

    BoxWithConstraints(modifier = modifier) {
        val diameter = with(density) { min(maxWidth.toPx(), maxHeight.toPx()) }
        val detailArcCenterRadius = sessionContextRingCenterRadius(diameter)
        val titleArcRadius = detailArcCenterRadius
        val titleBaselineInset = diameter * homeSessionTitleBaselineInsetScale()
        val titleTextSizePx = diameter * homeSessionTitleTextScale()
        val titleMarqueeState = rememberCurvedTitleMarqueeState(
            title = selectedTitle,
            pathLengthPx = curvedTitlePathLength(
                radius = titleArcRadius,
                baselineInset = titleBaselineInset,
            ),
            textSizePx = titleTextSizePx,
            edgePaddingPx = titleTextSizePx * 1.08f,
        )

        Canvas(
            modifier = Modifier.fillMaxSize(),
        ) {
            val diameter = min(size.width, size.height)
            val strokeWidth = sessionOuterRingStrokeWidth(diameter)
            val radius = sessionOuterRingCenterRadius(diameter)
            val center = Offset(size.width / 2f, size.height / 2f)
            val topLeft = Offset(center.x - radius, center.y - radius)
            val arcSize = Size(radius * 2f, radius * 2f)
            val segmentSweep = 360f / segments.size
            val gapSweep = min(3.5f, segmentSweep * 0.05f)
            val drawableSweep = (segmentSweep - gapSweep).coerceAtLeast(12f)
            val startOffset = TopAngleDegrees
            val detailArcCenterRadius = sessionContextRingCenterRadius(diameter)
            val contextStrokeWidth = diameter * SessionContextRingStrokeRatio
            val titleBaselineInset = diameter * homeSessionTitleBaselineInsetScale()
            val titleTextSizePx = diameter * homeSessionTitleTextScale()

            segments.forEachIndexed { index, _ ->
                val start = startOffset + (index * segmentSweep) + (gapSweep / 2f)
                drawArc(
                    color = SessionRingTrack,
                    startAngle = start,
                    sweepAngle = drawableSweep,
                    useCenter = false,
                    topLeft = topLeft,
                    size = arcSize,
                    style = Stroke(width = strokeWidth, cap = StrokeCap.Butt),
                )
            }

            segments.forEachIndexed { index, segment ->
                val start = startOffset + (index * segmentSweep) + (gapSweep / 2f)
                val color = sessionActivityColor(segment.isActiveNow, segment.activityRank)
                val selected = index == selectedIndex
                val activeStrokeWidth = if (selected) strokeWidth * 1.42f else strokeWidth

                drawArc(
                    color = color.copy(alpha = if (selected) 1f else 0.86f),
                    startAngle = start,
                    sweepAngle = drawableSweep,
                    useCenter = false,
                    topLeft = topLeft,
                    size = arcSize,
                    style = Stroke(width = activeStrokeWidth, cap = StrokeCap.Butt),
                )

            }

            drawSessionMetadataTracks(
                segment = segments[selectedIndex],
                iconDrawables = metadataIconDrawables,
                center = center,
                squareHalfSide = sessionContextInnerSquareSidePx(diameter) / 2f,
                outerRadius = detailArcCenterRadius - contextStrokeWidth * 0.58f,
                textSizePx = diameter * SessionMetadataBaseTextRatio,
            )

            drawSessionCursorAccent(
                cursorIndex = cursorIndex,
                segmentCount = segments.size,
                startOffset = startOffset,
                segmentSweep = segmentSweep,
                drawableSweep = drawableSweep,
                topLeft = topLeft,
                arcSize = arcSize,
                strokeWidth = strokeWidth,
            )

            drawCurvedTitle(
                title = selectedTitle,
                center = center,
                radius = titleArcRadius,
                baselineInset = titleBaselineInset,
                textSizePx = titleTextSizePx,
                marqueeState = titleMarqueeState,
                color = SessionTitleText,
                edgePaddingPx = titleTextSizePx * 1.08f,
            )
            drawContextPressureBar(
                contextPressurePercent = segments[selectedIndex].contextProgress,
                compactThresholdPercent = segments[selectedIndex].contextCompactThresholdPercent,
                center = center,
                radius = detailArcCenterRadius,
                strokeWidth = diameter * SessionContextRingStrokeRatio,
            )
        }
    }
}

@Composable
private fun rememberSessionConnectingCursorOffset(
    status: AgentMessageStreamStatus,
    segmentCount: Int,
): Float {
    if (status != AgentMessageStreamStatus.Connecting || segmentCount <= 0) {
        return 0f
    }
    val transition = rememberInfiniteTransition(label = "session-connecting-cursor")
    val offset by transition.animateFloat(
        initialValue = 0f,
        targetValue = segmentCount.toFloat(),
        animationSpec = infiniteRepeatable(
            animation = tween(
                durationMillis = SessionCursorConnectingCycleDurationMs,
                easing = LinearEasing,
            ),
        ),
        label = "session-connecting-cursor-offset",
    )
    return offset
}

internal enum class SessionMetadataArcIcon(
    @DrawableRes val drawableRes: Int,
) {
    Model(R.drawable.ic_model_cube),
    Reasoning(R.drawable.ic_reasoning_brain),
    Token(R.drawable.ic_token_burn),
}

private data class SessionMetadataArcTrack(
    val label: String,
    val startAngle: Float,
    val sweepAngle: Float,
    val icon: SessionMetadataArcIcon?,
    val iconColor: Color,
    val textColor: Color = Color.White,
    val textSizeScale: Float = 1f,
    val radialInsetFraction: Float = SessionMetadataDefaultRadialInsetFraction,
)

private data class SessionArcTextMetrics(
    val visualHeight: Float,
    val visualCenterOffsetFromBaseline: Float,
)

private data class SessionFittedArcText(
    val label: String,
    val paint: Paint,
    val metrics: SessionArcTextMetrics,
    val iconSize: Float,
    val iconGap: Float,
)

private fun DrawScope.drawSessionMetadataTracks(
    segment: SessionDetailSegmentUiModel,
    iconDrawables: Map<SessionMetadataArcIcon, Drawable?>,
    center: Offset,
    squareHalfSide: Float,
    outerRadius: Float,
    textSizePx: Float,
) {
    if (squareHalfSide <= 0f || outerRadius <= squareHalfSide) {
        return
    }
    val tracks = listOf(
        SessionMetadataArcTrack(
            label = sessionHeaderPrimaryLabel(segment.totalTokensLabel),
            startAngle = -132f,
            sweepAngle = 84f,
            icon = SessionMetadataArcIcon.Token,
            iconColor = Color(0xFFFF5A36),
            textColor = Color.White,
            textSizeScale = SessionMetadataTopTokenTextScale,
            radialInsetFraction = SessionMetadataTopTokenRadialInsetFraction,
        ),
        SessionMetadataArcTrack(
            label = segment.effort.ifBlank { "--" },
            startAngle = -38f,
            sweepAngle = 76f,
            icon = SessionMetadataArcIcon.Reasoning,
            iconColor = Color(0xFFB58CFF),
        ),
        SessionMetadataArcTrack(
            label = sessionFooterTokenLabel(segment.contextLabel),
            startAngle = 136f,
            sweepAngle = -92f,
            icon = null,
            iconColor = Color(0xFFFF5A36),
            textSizeScale = SessionMetadataBottomContextTextScale,
            radialInsetFraction = 0.18f,
        ),
        SessionMetadataArcTrack(
            label = segment.model.ifBlank { "--" },
            startAngle = 140f,
            sweepAngle = 80f,
            icon = SessionMetadataArcIcon.Model,
            iconColor = Color(0xFF47F2A0),
        ),
    )

    tracks.forEach { track ->
        drawMetadataArcText(
            track = track,
            iconDrawables = iconDrawables,
            center = center,
            radius = outerRadius - ((outerRadius - squareHalfSide) * track.radialInsetFraction),
            textSizePx = textSizePx * track.textSizeScale,
        )
    }
}

private fun DrawScope.drawMetadataArcText(
    track: SessionMetadataArcTrack,
    iconDrawables: Map<SessionMetadataArcIcon, Drawable?>,
    center: Offset,
    radius: Float,
    textSizePx: Float,
) {
    val nativeCanvas = drawContext.canvas.nativeCanvas
    val pathLength = (Math.PI.toFloat() * radius * 2f) * (kotlin.math.abs(track.sweepAngle) / 360f)
    val rawLabel = track.label.trim().ifBlank { "--" }
    val fitted = fitArcText(
        track = track,
        rawLabel = rawLabel,
        requestedTextSizePx = textSizePx,
        pathLength = pathLength,
    )
    val textWidth = fitted.paint.measureText(fitted.label)
    val totalWidth = textWidth + fitted.iconSize + fitted.iconGap
    val textOffset = ((pathLength - totalWidth) / 2f).coerceAtLeast(0f) + fitted.iconSize + fitted.iconGap

    if (track.icon != null) {
        val iconDistance = (textOffset - fitted.iconGap - fitted.iconSize / 2f).coerceAtLeast(0f)
        drawMetadataArcIcon(
            icon = track.icon,
            iconDrawables = iconDrawables,
            center = center,
            radius = radius,
            startAngle = track.startAngle,
            sweepAngle = track.sweepAngle,
            pathLength = pathLength,
            distance = iconDistance,
            size = fitted.iconSize,
            baselineCenterOffset = fitted.metrics.visualCenterOffsetFromBaseline,
            color = track.iconColor,
        )
    }

    val arcRect = RectF(
        center.x - radius,
        center.y - radius,
        center.x + radius,
        center.y + radius,
    )
    val textPath = android.graphics.Path().apply {
        addArc(arcRect, track.startAngle, track.sweepAngle)
    }
    nativeCanvas.drawTextOnPath(fitted.label, textPath, textOffset, 0f, fitted.paint)
}

private fun fitArcText(
    track: SessionMetadataArcTrack,
    rawLabel: String,
    requestedTextSizePx: Float,
    pathLength: Float,
): SessionFittedArcText {
    val sidePaddingPx = requestedTextSizePx * SessionMetadataSidePaddingScale
    val minimumTextSizePx = requestedTextSizePx * SessionMetadataMinTextScale
    var candidateSizePx = requestedTextSizePx
    while (candidateSizePx >= minimumTextSizePx) {
        val paint = metadataArcPaint(track.textColor, candidateSizePx)
        val metrics = arcTextMetrics(paint, rawLabel, candidateSizePx)
        val iconSize = track.icon?.let { iconDrawingBoxSize(it, metrics.visualHeight) } ?: 0f
        val iconGap = if (track.icon == null) 0f else metrics.visualHeight * SessionMetadataIconGapScale
        val availableTextWidth = (pathLength - iconSize - iconGap - sidePaddingPx).coerceAtLeast(0f)
        if (paint.measureText(rawLabel) <= availableTextWidth) {
            return SessionFittedArcText(
                label = rawLabel,
                paint = paint,
                metrics = metrics,
                iconSize = iconSize,
                iconGap = iconGap,
            )
        }
        candidateSizePx -= 0.5f
    }

    val fallbackPaint = metadataArcPaint(track.textColor, minimumTextSizePx)
    val fallbackMetrics = arcTextMetrics(fallbackPaint, rawLabel, minimumTextSizePx)
    val fallbackIconSize = track.icon?.let { iconDrawingBoxSize(it, fallbackMetrics.visualHeight) } ?: 0f
    val fallbackIconGap = if (track.icon == null) 0f else fallbackMetrics.visualHeight * SessionMetadataIconGapScale
    val fallbackWidth = (pathLength - fallbackIconSize - fallbackIconGap - sidePaddingPx).coerceAtLeast(minimumTextSizePx)
    return SessionFittedArcText(
        label = fittedArcLabel(rawLabel, fallbackPaint, fallbackWidth),
        paint = fallbackPaint,
        metrics = fallbackMetrics,
        iconSize = fallbackIconSize,
        iconGap = fallbackIconGap,
    )
}

private fun metadataArcPaint(color: Color, textSizePx: Float): Paint {
    return Paint(Paint.ANTI_ALIAS_FLAG).apply {
        this.color = color.toArgb()
        textSize = textSizePx
        typeface = Typeface.create(Typeface.DEFAULT, Typeface.BOLD)
        textAlign = Paint.Align.LEFT
        isSubpixelText = true
        isLinearText = true
    }
}

private fun DrawScope.drawMetadataArcIcon(
    icon: SessionMetadataArcIcon,
    iconDrawables: Map<SessionMetadataArcIcon, Drawable?>,
    center: Offset,
    radius: Float,
    startAngle: Float,
    sweepAngle: Float,
    pathLength: Float,
    distance: Float,
    size: Float,
    baselineCenterOffset: Float,
    color: Color,
) {
    val drawable = iconDrawables[icon] ?: return
    val fraction = if (pathLength <= 0f) 0f else (distance / pathLength).coerceIn(0f, 1f)
    val angleDegrees = startAngle + sweepAngle * fraction
    val angle = Math.toRadians(angleDegrees.toDouble())
    val iconCenterX = center.x + (radius * cos(angle)).toFloat()
    val iconCenterY = center.y + (radius * sin(angle)).toFloat()
    val tangentDegrees = angleDegrees + if (sweepAngle >= 0f) 90f else -90f
    val nativeCanvas = drawContext.canvas.nativeCanvas
    val halfSize = size / 2f
    nativeCanvas.save()
    nativeCanvas.translate(iconCenterX, iconCenterY)
    nativeCanvas.rotate(tangentDegrees)
    nativeCanvas.translate(0f, baselineCenterOffset)
    drawable.setTint(color.toArgb())
    drawable.setBounds(
        (-halfSize).roundToInt(),
        (-halfSize).roundToInt(),
        halfSize.roundToInt(),
        halfSize.roundToInt(),
    )
    drawable.draw(nativeCanvas)
    nativeCanvas.restore()
}

private fun arcTextMetrics(paint: Paint, label: String, fallbackTextSizePx: Float): SessionArcTextMetrics {
    val bounds = android.graphics.Rect()
    paint.getTextBounds(label, 0, label.length, bounds)
    val visualHeight = bounds.height().toFloat().takeIf { it > 0f }
        ?: (fallbackTextSizePx * 0.72f)
    val visualCenterOffsetFromBaseline = (bounds.top + bounds.bottom) / 2f
    return SessionArcTextMetrics(
        visualHeight = visualHeight,
        visualCenterOffsetFromBaseline = visualCenterOffsetFromBaseline,
    )
}

private fun iconDrawingBoxSize(icon: SessionMetadataArcIcon, textVisualHeight: Float): Float {
    return when (icon) {
        SessionMetadataArcIcon.Model -> textVisualHeight * SessionMetadataIconSizeScaleModel
        SessionMetadataArcIcon.Reasoning -> textVisualHeight * SessionMetadataIconSizeScaleReasoning
        SessionMetadataArcIcon.Token -> textVisualHeight * SessionMetadataIconSizeScaleToken
    }
}

private fun fittedArcLabel(label: String, paint: Paint, maxWidth: Float): String {
    val clean = label.trim().ifBlank { "--" }
    if (paint.measureText(clean) <= maxWidth) {
        return clean
    }
    val ellipsis = "..."
    var end = clean.length
    while (end > 0 && paint.measureText(clean.take(end) + ellipsis) > maxWidth) {
        end -= 1
    }
    return if (end > 0) clean.take(end) + ellipsis else ellipsis
}

private fun DrawScope.drawSessionCursorAccent(
    cursorIndex: Float,
    segmentCount: Int,
    startOffset: Float,
    segmentSweep: Float,
    drawableSweep: Float,
    topLeft: Offset,
    arcSize: Size,
    strokeWidth: Float,
) {
    if (segmentCount <= 0) {
        return
    }
    val normalizedCursor = wrapSessionCursorPosition(cursorIndex, segmentCount)
    val accentSweep = sessionCursorAccentSweep(drawableSweep)
    val start = startOffset + normalizedCursor * segmentSweep + ((segmentSweep - accentSweep) / 2f)
    drawArc(
        color = Color.White.copy(alpha = 0.78f),
        startAngle = start,
        sweepAngle = accentSweep,
        useCenter = false,
        topLeft = topLeft,
        size = arcSize,
        style = Stroke(width = strokeWidth * 0.36f, cap = StrokeCap.Round),
    )
}

private fun DrawScope.drawContextPressureBar(
    contextPressurePercent: Float,
    compactThresholdPercent: Float?,
    center: Offset,
    radius: Float,
    strokeWidth: Float,
) {
    val startAngle = -29f
    val sweepAngle = 238f
    val remainingFraction = ((100f - contextPressurePercent).coerceIn(0f, 100f)) / 100f
    val topLeft = Offset(center.x - radius, center.y - radius)
    val arcSize = Size(radius * 2f, radius * 2f)
    val segmentCount = 96
    val gapDegrees = 0.55f

    drawArc(
        color = SessionContextTrack,
        startAngle = startAngle,
        sweepAngle = sweepAngle,
        useCenter = false,
        topLeft = topLeft,
        size = arcSize,
        style = Stroke(width = strokeWidth, cap = StrokeCap.Round),
    )

    repeat(segmentCount) { index ->
        val segmentStart = index / segmentCount.toFloat()
        val segmentEnd = (index + 1) / segmentCount.toFloat()
        val activeEnd = min(segmentEnd, remainingFraction)
        if (activeEnd <= segmentStart) {
            return@repeat
        }

        val activeSweep = sweepAngle * (activeEnd - segmentStart) - gapDegrees
        if (activeSweep <= 0f) {
            return@repeat
        }

        val color = contextSemanticColor((segmentStart + activeEnd) / 2f)
        drawArc(
            color = color.copy(alpha = 0.12f),
            startAngle = startAngle + sweepAngle * segmentStart,
            sweepAngle = activeSweep,
            useCenter = false,
            topLeft = topLeft,
            size = arcSize,
            style = Stroke(width = strokeWidth * 1.3f, cap = StrokeCap.Round),
        )
        drawArc(
            color = color,
            startAngle = startAngle + sweepAngle * segmentStart,
            sweepAngle = activeSweep,
            useCenter = false,
            topLeft = topLeft,
            size = arcSize,
            style = Stroke(width = strokeWidth, cap = StrokeCap.Round),
        )
    }

    if (remainingFraction > 0f) {
        val dotAngle = Math.toRadians((startAngle + sweepAngle * remainingFraction).toDouble())
        val dotCenter = Offset(
            x = center.x + (radius * cos(dotAngle)).toFloat(),
            y = center.y + (radius * sin(dotAngle)).toFloat(),
        )
        drawCircle(
            color = contextSemanticColor(remainingFraction.coerceIn(0.02f, 1f)),
            radius = strokeWidth * 0.56f,
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
                strokeWidth = strokeWidth,
                color = SessionContextAmber,
            )
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

internal fun shouldSelectSessionFromTap(
    movedBeyondTouchSlop: Boolean,
    consumedByAnotherHandler: Boolean,
    pressDurationMs: Long,
): Boolean {
    return !movedBeyondTouchSlop &&
        !consumedByAnotherHandler &&
        pressDurationMs < ScreenshotLongPressTimeoutMs
}

internal fun locateSessionSector(
    tapOffset: Offset,
    canvasSize: IntSize,
    segmentCount: Int,
): Int? {
    if (segmentCount <= 0) {
        return null
    }

    val width = canvasSize.width.toFloat()
    val height = canvasSize.height.toFloat()
    val diameter = min(width, height)
    val radius = diameter / 2f
    val dx = tapOffset.x - (width / 2f)
    val dy = tapOffset.y - (height / 2f)
    val distance = sqrt(dx * dx + dy * dy)
    if (distance > radius) {
        return null
    }

    val angle = normalizeAngle(Math.toDegrees(atan2(dy.toDouble(), dx.toDouble())).toFloat())
    val segmentSweep = 360f / segmentCount
    val startOffset = TopAngleDegrees
    val relative = normalizeAngle(angle - startOffset)
    return floor(relative / segmentSweep).toInt().coerceIn(0, segmentCount - 1)
}

internal fun sessionActivityRankForSortedIndex(index: Int): Int {
    return index.coerceIn(0, maxOf(SessionRunningRankColors.lastIndex, SessionIdleRankColors.lastIndex))
}

internal fun sessionActivityColor(isActiveNow: Boolean, rank: Int): Color {
    val colors = if (isActiveNow) SessionRunningRankColors else SessionIdleRankColors
    return colors[rank.coerceIn(0, colors.lastIndex)]
}

internal fun sessionCursorAccentSweep(drawableSweep: Float): Float {
    if (drawableSweep <= 0f) {
        return 0f
    }
    return min(18f, drawableSweep * 0.28f)
        .coerceAtLeast(min(8f, drawableSweep))
        .coerceAtMost(drawableSweep)
}

internal fun sessionCursorAccentPosition(
    baseCursorIndex: Float,
    connectingCursorOffset: Float,
    segmentCount: Int,
): Float {
    if (segmentCount <= 0) {
        return 0f
    }
    return wrapSessionCursorPosition(baseCursorIndex + connectingCursorOffset, segmentCount)
}

private fun contextSemanticColor(fraction: Float): Color {
    val clamped = fraction.coerceIn(0f, 1f)
    return when {
        clamped < 0.34f -> lerp(SessionContextRed, SessionContextAmber, clamped / 0.34f)
        clamped < 0.68f -> lerp(SessionContextAmber, SessionContextLime, (clamped - 0.34f) / 0.34f)
        else -> lerp(SessionContextLime, SessionContextGreen, (clamped - 0.68f) / 0.32f)
    }
}

private fun sessionCursorDisplayIndex(cursorIndex: Float, segmentCount: Int): Int {
    if (segmentCount <= 0) {
        return 0
    }
    return floorSessionCursorPosition(cursorIndex, segmentCount)
}

private fun wrapSessionCursorPosition(position: Float, size: Int): Float {
    if (size <= 0) {
        return 0f
    }
    val raw = position % size.toFloat()
    return if (raw < 0f) raw + size.toFloat() else raw
}

private fun floorSessionCursorPosition(position: Float, size: Int): Int {
    if (size <= 0) {
        return 0
    }
    val raw = floor(wrapSessionCursorPosition(position, size).toDouble()).toInt() % size
    return if (raw < 0) raw + size else raw
}

private fun normalizeRecentLabel(value: String): String {
    val trimmed = value.trim().lowercase()
    return if (trimmed.isBlank()) "--" else trimmed
}

private fun normalizeAngle(angle: Float): Float {
    return ((angle % 360f) + 360f) % 360f
}

private fun sessionTextStyle(
    fontSize: TextUnit,
    lineHeight: TextUnit = fontSize,
    fontWeight: FontWeight = FontWeight.Medium,
): TextStyle {
    return TextStyle(
        fontSize = fontSize,
        lineHeight = lineHeight,
        fontWeight = fontWeight,
        platformStyle = PlatformTextStyle(includeFontPadding = false),
    )
}
