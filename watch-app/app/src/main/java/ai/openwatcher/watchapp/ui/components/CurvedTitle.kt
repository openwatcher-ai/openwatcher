package ai.openwatcher.watchapp.ui.components

import android.graphics.Paint
import android.graphics.RectF
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.remember
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.nativeCanvas
import androidx.compose.ui.graphics.toArgb
import kotlin.math.PI
import kotlin.math.max
import kotlin.math.round

private const val DefaultTitleStartAngle = -150f
private const val DefaultTitleSweepAngle = 120f
private const val DefaultTitleScrollSpeedPxPerSecond = 11f
private const val DefaultTitleStartPauseMillis = 2300
private const val DefaultTitleEndPauseMillis = 2200

@Immutable
internal data class CurvedTitleMarqueeState(
    val shouldScroll: Boolean,
    val scrollOffsetPx: Float,
)

@Immutable
private data class VisibleCurvedTitleText(
    val text: String,
    val leadingTrimPx: Float,
)

@Composable
internal fun rememberCurvedTitleMarqueeState(
    title: String,
    pathLengthPx: Float,
    textSizePx: Float,
    edgePaddingPx: Float = max(textSizePx * 0.9f, 14f),
    scrollSpeedPxPerSecond: Float = DefaultTitleScrollSpeedPxPerSecond,
    startPauseMillis: Int = DefaultTitleStartPauseMillis,
    endPauseMillis: Int = DefaultTitleEndPauseMillis,
): CurvedTitleMarqueeState {
    val cleanTitle = remember(title) { title.trim() }
    val textWidthPx = remember(cleanTitle, textSizePx) {
        if (cleanTitle.isBlank() || textSizePx <= 0f) {
            0f
        } else {
            titlePaint(textSizePx, Color.White).measureText(cleanTitle)
        }
    }
    val usablePathLengthPx = remember(pathLengthPx, edgePaddingPx) {
        (pathLengthPx - edgePaddingPx * 2f).coerceAtLeast(1f)
    }
    val overflowPx = remember(textWidthPx, usablePathLengthPx) {
        (textWidthPx - usablePathLengthPx).coerceAtLeast(0f)
    }
    if (cleanTitle.isBlank() || overflowPx <= 0f) {
        return CurvedTitleMarqueeState(
            shouldScroll = false,
            scrollOffsetPx = 0f,
        )
    }

    val scrollDurationMillis = max(
        1,
        ((overflowPx / scrollSpeedPxPerSecond) * 1000f).toInt(),
    )
    val totalDurationMillis = startPauseMillis + scrollDurationMillis + endPauseMillis
    val transition = rememberInfiniteTransition(label = "curved-title-marquee")
    val elapsedMillis = transition.animateFloat(
        initialValue = 0f,
        targetValue = totalDurationMillis.toFloat(),
        animationSpec = infiniteRepeatable(
            animation = tween(
                durationMillis = totalDurationMillis,
                easing = LinearEasing,
            ),
        ),
        label = "curved-title-marquee-elapsed",
    ).value

    val scrollOffsetPx = when {
        elapsedMillis <= startPauseMillis -> 0f
        elapsedMillis >= startPauseMillis + scrollDurationMillis -> overflowPx
        else -> {
            val scrollElapsed = elapsedMillis - startPauseMillis
            overflowPx * (scrollElapsed / scrollDurationMillis.toFloat())
        }
    }
    return CurvedTitleMarqueeState(
        shouldScroll = true,
        scrollOffsetPx = round(scrollOffsetPx),
    )
}

internal fun curvedTitlePathLength(
    radius: Float,
    baselineInset: Float,
    sweepAngleDegrees: Float = DefaultTitleSweepAngle,
): Float {
    val textRadius = (radius - baselineInset).coerceAtLeast(0f)
    return ((PI.toFloat() * textRadius * 2f) * (sweepAngleDegrees / 360f)).coerceAtLeast(0f)
}

internal fun DrawScope.drawCurvedTitle(
    title: String,
    center: Offset,
    radius: Float,
    baselineInset: Float,
    textSizePx: Float,
    marqueeState: CurvedTitleMarqueeState,
    color: Color,
    edgePaddingPx: Float = max(textSizePx * 0.9f, 14f),
    sweepAngleDegrees: Float = DefaultTitleSweepAngle,
    startAngleDegrees: Float = DefaultTitleStartAngle,
    shadowColor: Color = Color.Black.copy(alpha = 0.54f),
    shadowRadiusPx: Float = textSizePx * 0.11f,
) {
    val cleanTitle = title.trim()
    if (cleanTitle.isBlank() || textSizePx <= 0f) {
        return
    }

    val textRadius = (radius - baselineInset).coerceAtLeast(0f)
    val arcRect = RectF(
        center.x - textRadius,
        center.y - textRadius,
        center.x + textRadius,
        center.y + textRadius,
    )
    val path = android.graphics.Path().apply {
        addArc(arcRect, startAngleDegrees, sweepAngleDegrees)
    }
    val paint = titlePaint(textSizePx, color).apply {
        if (!marqueeState.shouldScroll && shadowRadiusPx > 0f) {
            setShadowLayer(shadowRadiusPx, 0f, 0f, shadowColor.toArgb())
        }
    }
    val pathLengthPx = curvedTitlePathLength(radius, baselineInset, sweepAngleDegrees)
    val usablePathLengthPx = (pathLengthPx - edgePaddingPx * 2f).coerceAtLeast(1f)
    if (!marqueeState.shouldScroll) {
        val centeredOffset = edgePaddingPx + ((usablePathLengthPx - paint.measureText(cleanTitle)) / 2f)
        drawContext.canvas.nativeCanvas.drawTextOnPath(cleanTitle, path, centeredOffset.coerceAtLeast(edgePaddingPx), 0f, paint)
        return
    }

    val visibleText = visibleCurvedTitleText(
        title = cleanTitle,
        paint = paint,
        maxWidthPx = usablePathLengthPx,
        scrollOffsetPx = marqueeState.scrollOffsetPx,
    )

    drawContext.canvas.nativeCanvas.drawTextOnPath(
        visibleText.text,
        path,
        edgePaddingPx - visibleText.leadingTrimPx,
        0f,
        paint,
    )
}

private fun visibleCurvedTitleText(
    title: String,
    paint: Paint,
    maxWidthPx: Float,
    scrollOffsetPx: Float,
): VisibleCurvedTitleText {
    if (title.isBlank() || maxWidthPx <= 0f) {
        return VisibleCurvedTitleText("", 0f)
    }

    val glyphs = title.map { it.toString() }
    var consumedWidthPx = 0f
    var startIndex = 0
    while (startIndex < glyphs.size) {
        val glyphWidthPx = paint.measureText(glyphs[startIndex])
        if (consumedWidthPx + glyphWidthPx > scrollOffsetPx) {
            break
        }
        consumedWidthPx += glyphWidthPx
        startIndex += 1
    }

    val leadingTrimPx = (scrollOffsetPx - consumedWidthPx).coerceAtLeast(0f)
    val builder = StringBuilder()
    var visibleWidthPx = -leadingTrimPx
    var index = startIndex
    while (index < glyphs.size) {
        val glyph = glyphs[index]
        val glyphWidthPx = paint.measureText(glyph)
        if (builder.isNotEmpty() && visibleWidthPx + glyphWidthPx > maxWidthPx) {
            break
        }
        builder.append(glyph)
        visibleWidthPx += glyphWidthPx
        index += 1
    }

    if (builder.isEmpty() && startIndex < glyphs.size) {
        builder.append(glyphs[startIndex])
    }

    return VisibleCurvedTitleText(
        text = builder.toString(),
        leadingTrimPx = round(leadingTrimPx),
    )
}

private fun titlePaint(textSizePx: Float, color: Color): Paint {
    return Paint(Paint.ANTI_ALIAS_FLAG).apply {
        this.color = color.toArgb()
        textSize = textSizePx
        typeface = android.graphics.Typeface.create(
            android.graphics.Typeface.DEFAULT,
            android.graphics.Typeface.BOLD,
        )
        textAlign = Paint.Align.LEFT
        isSubpixelText = false
        isLinearText = true
    }
}
