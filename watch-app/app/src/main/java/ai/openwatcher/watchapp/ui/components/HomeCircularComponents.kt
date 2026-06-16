package ai.openwatcher.watchapp.ui.components

import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlin.math.min

internal object WatchHomePalette {
    val Background = Color(0xFF05070B)
    val Panel = Color(0xFF0D121A)
    val PanelAlt = Color(0xFF111823)
    val Divider = Color(0x26FFFFFF)
    val Track = Color(0x20FFFFFF)
    val SoftTrack = Color(0x14FFFFFF)
    val WhiteSoft = Color(0xFFCFD6E2)
    val Cyan = Color(0xFF27E7F4)
    val Green = Color(0xFF74F34A)
    val Lime = Color(0xFFD7F60F)
    val Amber = Color(0xFFFFBC27)
    val Orange = Color(0xFFFF6A1A)
    val Red = Color(0xFFFF4221)
    val Purple = Color(0xFFD991FF)
}

@Composable
internal fun ContextAvailabilityArc(
    remainingPercent: Float,
    modifier: Modifier = Modifier,
) {
    ArcAvailabilityMeter(
        modifier = modifier,
        remainingPercent = remainingPercent,
        startAngle = 160f,
        sweepAngle = 220f,
        strokeFactor = 0.088f,
        glowFactor = 1.75f,
    )
}

@Composable
internal fun QuotaAvailabilityRing(
    remainingPercent: Float,
    modifier: Modifier = Modifier,
) {
    ArcAvailabilityMeter(
        modifier = modifier,
        remainingPercent = remainingPercent,
        startAngle = 142f,
        sweepAngle = 255f,
        strokeFactor = 0.102f,
        glowFactor = 1.5f,
    )
}

@Composable
private fun ArcAvailabilityMeter(
    remainingPercent: Float,
    startAngle: Float,
    sweepAngle: Float,
    strokeFactor: Float,
    glowFactor: Float,
    modifier: Modifier = Modifier,
) {
    Canvas(modifier = modifier.aspectRatio(1f)) {
        val remainingFraction = remainingPercent.coerceIn(0f, 100f) / 100f
        val strokeWidth = size.minDimension * strokeFactor
        val segmentCount = 72
        val gapDegrees = sweepAngle / 360f
        val topLeft = Offset(strokeWidth, strokeWidth)
        val arcSize = androidx.compose.ui.geometry.Size(
            width = size.width - strokeWidth * 2f,
            height = size.height - strokeWidth * 2f,
        )

        drawArc(
            color = WatchHomePalette.Track,
            startAngle = startAngle,
            sweepAngle = sweepAngle,
            useCenter = false,
            topLeft = topLeft,
            size = arcSize,
            style = Stroke(width = strokeWidth, cap = StrokeCap.Round),
        )

        for (index in 0 until segmentCount) {
            val segmentStart = index / segmentCount.toFloat()
            val segmentEnd = (index + 1) / segmentCount.toFloat()
            val activeEnd = minOf(segmentEnd, remainingFraction)
            if (activeEnd <= segmentStart) {
                continue
            }

            val activeSweep = sweepAngle * (activeEnd - segmentStart) - gapDegrees
            if (activeSweep <= 0f) {
                continue
            }

            val centerFraction = (segmentStart + activeEnd) / 2f
            val color = semanticArcColor(centerFraction)

            drawArc(
                color = color.copy(alpha = 0.18f),
                startAngle = startAngle + sweepAngle * segmentStart,
                sweepAngle = activeSweep,
                useCenter = false,
                topLeft = topLeft,
                size = arcSize,
                style = Stroke(width = strokeWidth * glowFactor, cap = StrokeCap.Round),
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
    }
}

private fun semanticArcColor(fraction: Float): Color {
    val clamped = fraction.coerceIn(0f, 1f)
    return when {
        clamped < 0.34f -> lerp(WatchHomePalette.Red, WatchHomePalette.Amber, clamped / 0.34f)
        clamped < 0.68f -> lerp(WatchHomePalette.Amber, WatchHomePalette.Lime, (clamped - 0.34f) / 0.34f)
        else -> lerp(WatchHomePalette.Lime, WatchHomePalette.Green, (clamped - 0.68f) / 0.32f)
    }
}

@Composable
internal fun ActivityStatusIndicator(
    isLive: Boolean,
    label: String,
    modifier: Modifier = Modifier,
) {
    if (isLive) {
        val transition = rememberInfiniteTransition(label = "home-activity")
        val rotation = transition.animateFloat(
            initialValue = 0f,
            targetValue = 360f,
            animationSpec = infiniteRepeatable(
                animation = tween(durationMillis = 2200, easing = LinearEasing),
                repeatMode = RepeatMode.Restart,
            ),
            label = "home-activity-rotation",
        )

        Box(
            modifier = modifier
                .clip(CircleShape)
                .background(WatchHomePalette.PanelAlt),
            contentAlignment = Alignment.Center,
        ) {
            Canvas(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(4.dp)
                    .rotate(rotation.value),
            ) {
                val strokeWidth = min(size.width, size.height) * 0.12f
                val segmentSweep = 18f
                val segmentCount = 12
                repeat(segmentCount) { index ->
                    drawArc(
                        color = WatchHomePalette.Cyan.copy(alpha = 0.3f + (index / segmentCount.toFloat()) * 0.7f),
                        startAngle = (index * (360f / segmentCount.toFloat())),
                        sweepAngle = segmentSweep,
                        useCenter = false,
                        style = Stroke(width = strokeWidth, cap = StrokeCap.Round),
                    )
                }
            }
        }
        return
    }

    Box(
        modifier = modifier
            .clip(CircleShape)
            .background(WatchHomePalette.PanelAlt),
        contentAlignment = Alignment.Center,
    ) {
        Canvas(
            modifier = Modifier
                .fillMaxSize()
                .padding(4.dp),
        ) {
            val strokeWidth = min(size.width, size.height) * 0.082f
            drawArc(
                color = WatchHomePalette.Cyan.copy(alpha = 0.7f),
                startAngle = -90f,
                sweepAngle = 360f,
                useCenter = false,
                style = Stroke(width = strokeWidth, cap = StrokeCap.Round),
            )
        }
        Text(
            text = label,
            color = Color.White,
            fontSize = 7.sp,
            fontWeight = FontWeight.SemiBold,
            maxLines = 1,
        )
    }
}
