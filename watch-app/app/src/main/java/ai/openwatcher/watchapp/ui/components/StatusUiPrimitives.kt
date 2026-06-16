package ai.openwatcher.watchapp.ui.components

import androidx.annotation.DrawableRes
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import ai.openwatcher.watchapp.ui.ServiceHealthStatus

internal object StatusUiPalette {
    val ScreenBackground = Color(0xFF05070B)
    val Surface = Color(0xFF11161F)
    val SurfaceAlt = Color(0xFF121B27)
    val Outline = Color(0xFF2A3242)
    val TextPrimary = Color(0xFFF7F9FC)
    val TextSecondary = Color(0xFF9AA7BB)
    val Green = Color(0xFF65F07D)
    val GreenDim = Color(0xFF12382B)
    val Blue = Color(0xFF36B8FF)
    val BlueDim = Color(0xFF0A2137)
    val Cyan = Color(0xFF2DE8F3)
    val Amber = Color(0xFFFFC857)
    val Red = Color(0xFFFF6155)
    val RedDim = Color(0xFF35110F)
    val Orange = Color(0xFFFF8A3D)
    val ShadowGreen = Color(0xFF0D2B1A)
}

internal object StatusCapsuleDefaults {
    val panelCorner = 36.dp
    val pillCorner = 34.dp
    val rowCorner = 30.dp
    val compactCorner = 26.dp
    val bubbleCorner = 24.dp
    val panelHorizontalPadding = 6.dp
    val rowHorizontalPadding = 12.dp
    val rowVerticalPadding = 8.dp
    val rowContentGap = 8.dp
    val denseHorizontalPadding = 5.dp
    val tightHorizontalPadding = 4.dp
}

@Composable
internal fun StatusSectionHeader(
    @DrawableRes iconRes: Int,
    title: String,
    subtitle: String,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier.fillMaxWidth(),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(3.dp),
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            StatusIcon(
                iconRes = iconRes,
                tint = StatusUiPalette.TextPrimary,
                size = 16.dp,
            )
            Text(
                text = title,
                color = StatusUiPalette.TextPrimary,
                fontSize = 17.sp,
                fontWeight = FontWeight.SemiBold,
            )
        }
        Text(
            text = subtitle,
            color = StatusUiPalette.TextSecondary,
            fontSize = 8.sp,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

@Composable
internal fun StatusPanelCard(
    modifier: Modifier = Modifier,
    borderColor: Color = StatusUiPalette.Outline,
    containerColor: Color = StatusUiPalette.Surface,
    content: @Composable () -> Unit,
) {
    Card(
        modifier = modifier,
        shape = RoundedCornerShape(StatusCapsuleDefaults.panelCorner),
        colors = CardDefaults.cardColors(containerColor = containerColor),
    ) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .border(
                    1.dp,
                    borderColor.copy(alpha = 0.75f),
                    RoundedCornerShape(StatusCapsuleDefaults.panelCorner),
                )
                .padding(
                    horizontal = StatusCapsuleDefaults.panelHorizontalPadding,
                    vertical = 12.dp,
                ),
        ) {
            content()
        }
    }
}

@Composable
internal fun StatusPillButton(
    title: String,
    subtitle: String,
    @DrawableRes iconRes: Int,
    tint: Color,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    containerColor: Color = StatusUiPalette.SurfaceAlt,
) {
    Card(
        modifier = modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(StatusCapsuleDefaults.pillCorner),
        colors = CardDefaults.cardColors(containerColor = containerColor),
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .border(
                    1.dp,
                    tint.copy(alpha = 0.55f),
                    RoundedCornerShape(StatusCapsuleDefaults.pillCorner),
                )
                .padding(horizontal = StatusCapsuleDefaults.tightHorizontalPadding, vertical = 11.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            StatusIcon(iconRes = iconRes, tint = tint, size = 20.dp)
            Text(
                text = title,
                color = StatusUiPalette.TextPrimary,
                fontSize = 10.sp,
                fontWeight = FontWeight.SemiBold,
                textAlign = TextAlign.Center,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                text = subtitle,
                color = StatusUiPalette.TextSecondary,
                fontSize = 7.sp,
                textAlign = TextAlign.Center,
                lineHeight = 9.sp,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

@Composable
internal fun StatusWideButton(
    title: String,
    subtitle: String,
    @DrawableRes iconRes: Int,
    tint: Color,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    containerColor: Color = StatusUiPalette.Surface,
) {
    Card(
        modifier = modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(StatusCapsuleDefaults.rowCorner),
        colors = CardDefaults.cardColors(containerColor = containerColor),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .border(
                    1.dp,
                    tint.copy(alpha = 0.35f),
                    RoundedCornerShape(StatusCapsuleDefaults.rowCorner),
                )
                .padding(
                    horizontal = StatusCapsuleDefaults.rowHorizontalPadding,
                    vertical = StatusCapsuleDefaults.rowVerticalPadding,
                ),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(StatusCapsuleDefaults.rowContentGap),
        ) {
            StatusIcon(iconRes = iconRes, tint = tint, size = 20.dp)
            Column(
                modifier = Modifier.weight(1f),
                verticalArrangement = Arrangement.spacedBy(2.dp),
            ) {
                Text(
                    text = title,
                    color = StatusUiPalette.TextPrimary,
                    fontSize = 10.sp,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    text = subtitle,
                    color = StatusUiPalette.TextSecondary,
                    fontSize = 7.sp,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
    }
}

@Composable
internal fun StatusMiniAction(
    title: String,
    @DrawableRes iconRes: Int,
    tint: Color,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Card(
        modifier = modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(StatusCapsuleDefaults.compactCorner),
        colors = CardDefaults.cardColors(containerColor = StatusUiPalette.Surface),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .border(
                    1.dp,
                    StatusUiPalette.Outline,
                    RoundedCornerShape(StatusCapsuleDefaults.compactCorner),
                )
                .padding(horizontal = StatusCapsuleDefaults.denseHorizontalPadding, vertical = 9.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            StatusIcon(iconRes = iconRes, tint = tint, size = 18.dp)
            Text(
                text = title,
                modifier = Modifier.weight(1f),
                color = StatusUiPalette.TextPrimary,
                fontSize = 9.sp,
                fontWeight = FontWeight.Medium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

@Composable
internal fun StatusDangerPanel(
    title: String,
    subtitle: String,
    @DrawableRes iconRes: Int,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Card(
        modifier = modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(StatusCapsuleDefaults.pillCorner),
        colors = CardDefaults.cardColors(containerColor = StatusUiPalette.RedDim.copy(alpha = 0.92f)),
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .border(
                    1.dp,
                    StatusUiPalette.Red.copy(alpha = 0.5f),
                    RoundedCornerShape(StatusCapsuleDefaults.pillCorner),
                )
                .padding(horizontal = StatusCapsuleDefaults.denseHorizontalPadding, vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                StatusIcon(iconRes = iconRes, tint = StatusUiPalette.Red, size = 22.dp)
                Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(
                        text = title,
                        color = StatusUiPalette.Red,
                        fontSize = 11.sp,
                        fontWeight = FontWeight.SemiBold,
                    )
                    Text(
                        text = subtitle,
                        color = StatusUiPalette.TextSecondary,
                        fontSize = 7.sp,
                        lineHeight = 9.sp,
                    )
                }
            }
        }
    }
}

@Composable
internal fun StatusInfoRow(
    @DrawableRes iconRes: Int,
    label: String,
    value: String,
    valueColor: Color = StatusUiPalette.TextPrimary,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(7.dp),
    ) {
        StatusIcon(iconRes = iconRes, tint = StatusUiPalette.Blue, size = 14.dp)
        Text(
            text = label,
            color = StatusUiPalette.TextSecondary,
            fontSize = 7.sp,
            maxLines = 1,
        )
        Spacer(modifier = Modifier.weight(1f))
        Text(
            text = value,
            color = valueColor,
            fontSize = 8.sp,
            fontWeight = FontWeight.Medium,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

@Composable
internal fun StatusIcon(
    @DrawableRes iconRes: Int,
    tint: Color,
    size: Dp,
    modifier: Modifier = Modifier,
) {
    Icon(
        painter = painterResource(id = iconRes),
        contentDescription = null,
        tint = tint,
        modifier = modifier.size(size),
    )
}

@Composable
internal fun StatusGlowDot(
    color: Color,
    modifier: Modifier = Modifier,
    innerSize: Dp = 18.dp,
) {
    Box(
        modifier = modifier.size(innerSize + 22.dp),
        contentAlignment = Alignment.Center,
    ) {
        Box(
            modifier = Modifier
                .size(innerSize + 18.dp)
                .clip(CircleShape)
                .background(color.copy(alpha = 0.08f)),
        )
        Box(
            modifier = Modifier
                .size(innerSize + 8.dp)
                .clip(CircleShape)
                .background(color.copy(alpha = 0.16f)),
        )
        Box(
            modifier = Modifier
                .size(innerSize)
                .clip(CircleShape)
                .background(color),
        )
    }
}

@Composable
internal fun ServiceHealthIndicator(
    status: ServiceHealthStatus,
    modifier: Modifier = Modifier,
) {
    when (status) {
        ServiceHealthStatus.Checking -> {
            val transition = rememberInfiniteTransition(label = "service-health-indicator")
            val angle by transition.animateFloat(
                initialValue = 0f,
                targetValue = 360f,
                animationSpec = infiniteRepeatable(
                    animation = tween(durationMillis = 900, easing = LinearEasing),
                    repeatMode = RepeatMode.Restart,
                ),
                label = "service-health-indicator-angle",
            )
            ServiceHealthIndicatorCanvas(
                status = status,
                angle = angle,
                modifier = modifier,
            )
        }

        else -> ServiceHealthIndicatorCanvas(
            status = status,
            angle = 0f,
            modifier = modifier,
        )
    }
}

@Composable
private fun ServiceHealthIndicatorCanvas(
    status: ServiceHealthStatus,
    angle: Float,
    modifier: Modifier = Modifier,
) {
    Canvas(modifier = modifier.size(18.dp)) {
        val stroke = 1.9.dp.toPx()
        val fillRadius = size.minDimension / 2f
        val center = Offset(size.width / 2f, size.height / 2f)
        when (status) {
            ServiceHealthStatus.Checking -> {
                drawCircle(
                    color = Color.White.copy(alpha = 0.12f),
                    radius = fillRadius - stroke / 2f,
                    center = center,
                    style = Stroke(width = stroke),
                )
                drawArc(
                    color = StatusUiPalette.Blue,
                    startAngle = angle,
                    sweepAngle = 108f,
                    useCenter = false,
                    topLeft = Offset(stroke / 2f, stroke / 2f),
                    size = androidx.compose.ui.geometry.Size(
                        width = size.width - stroke,
                        height = size.height - stroke,
                    ),
                    style = Stroke(width = stroke, cap = StrokeCap.Round),
                )
            }

            ServiceHealthStatus.Online -> {
                drawCircle(
                    color = StatusUiPalette.Green,
                    radius = fillRadius,
                    center = center,
                )
                drawHealthCheckMark(color = Color.White, stroke = stroke)
            }

            ServiceHealthStatus.Offline -> {
                drawCircle(
                    color = StatusUiPalette.Red,
                    radius = fillRadius,
                    center = center,
                )
                drawHealthCrossMark(color = Color.White, stroke = stroke)
            }

            ServiceHealthStatus.Idle -> {
                drawCircle(
                    color = Color(0xFF263041),
                    radius = fillRadius,
                    center = center,
                )
                drawCircle(
                    color = StatusUiPalette.TextSecondary.copy(alpha = 0.9f),
                    radius = 1.6.dp.toPx(),
                    center = center,
                )
            }
        }
    }
}

private fun androidx.compose.ui.graphics.drawscope.DrawScope.drawHealthCheckMark(
    color: Color,
    stroke: Float,
) {
    drawLine(
        color = color,
        start = Offset(size.width * 0.31f, size.height * 0.55f),
        end = Offset(size.width * 0.43f, size.height * 0.68f),
        strokeWidth = stroke,
        cap = StrokeCap.Round,
    )
    drawLine(
        color = color,
        start = Offset(size.width * 0.43f, size.height * 0.68f),
        end = Offset(size.width * 0.71f, size.height * 0.38f),
        strokeWidth = stroke,
        cap = StrokeCap.Round,
    )
}

private fun androidx.compose.ui.graphics.drawscope.DrawScope.drawHealthCrossMark(
    color: Color,
    stroke: Float,
) {
    drawLine(
        color = color,
        start = Offset(size.width * 0.33f, size.height * 0.33f),
        end = Offset(size.width * 0.67f, size.height * 0.67f),
        strokeWidth = stroke,
        cap = StrokeCap.Round,
    )
    drawLine(
        color = color,
        start = Offset(size.width * 0.67f, size.height * 0.33f),
        end = Offset(size.width * 0.33f, size.height * 0.67f),
        strokeWidth = stroke,
        cap = StrokeCap.Round,
    )
}

@Composable
internal fun StatusQrCard(
    bitmap: ImageBitmap,
    borderColor: Color,
    modifier: Modifier = Modifier,
    contentPadding: Dp = 8.dp,
) {
    Card(
        modifier = modifier,
        shape = RoundedCornerShape(StatusCapsuleDefaults.compactCorner),
        colors = CardDefaults.cardColors(containerColor = Color.White),
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .border(
                    1.dp,
                    borderColor.copy(alpha = 0.55f),
                    RoundedCornerShape(StatusCapsuleDefaults.compactCorner),
                )
                .padding(contentPadding),
            contentAlignment = Alignment.Center,
        ) {
            androidx.compose.foundation.Image(
                bitmap = bitmap,
                contentDescription = "配对二维码",
                modifier = Modifier.fillMaxSize(),
            )
        }
    }
}

@Composable
internal fun PairingHalo(
    modifier: Modifier = Modifier,
    accentColor: Color = StatusUiPalette.Green,
) {
    Canvas(modifier = modifier) {
        val ringStroke = size.minDimension * 0.05f
        val inset = ringStroke / 2f + 5.dp.toPx()
        val arcSize = androidx.compose.ui.geometry.Size(
            width = size.width - inset * 2f,
            height = size.height - inset * 2f,
        )
        val topLeft = Offset(inset, inset)

        drawCircle(
            color = Color.White.copy(alpha = 0.06f),
            radius = size.minDimension / 2f - 2.dp.toPx(),
            style = Stroke(width = 1.2.dp.toPx()),
        )
        drawArc(
            color = accentColor.copy(alpha = 0.35f),
            startAngle = 142f,
            sweepAngle = 92f,
            useCenter = false,
            topLeft = topLeft,
            size = arcSize,
            style = Stroke(width = ringStroke, cap = StrokeCap.Round),
        )
        drawArc(
            color = accentColor.copy(alpha = 0.5f),
            startAngle = 244f,
            sweepAngle = 88f,
            useCenter = false,
            topLeft = topLeft,
            size = arcSize,
            style = Stroke(width = ringStroke, cap = StrokeCap.Round),
        )
        drawArc(
            color = StatusUiPalette.Cyan,
            startAngle = 336f,
            sweepAngle = 28f,
            useCenter = false,
            topLeft = topLeft,
            size = arcSize,
            style = Stroke(width = ringStroke, cap = StrokeCap.Round),
        )
        drawArc(
            color = accentColor.copy(alpha = 0.24f),
            startAngle = 38f,
            sweepAngle = 78f,
            useCenter = false,
            topLeft = topLeft,
            size = arcSize,
            style = Stroke(width = ringStroke * 0.78f, cap = StrokeCap.Round),
        )
        drawCircle(
            color = accentColor.copy(alpha = 0.92f),
            radius = ringStroke * 0.45f,
            center = Offset(size.width / 2f, inset + ringStroke * 0.55f),
        )
    }
}

@Composable
internal fun OfflineHalo(
    modifier: Modifier = Modifier,
) {
    Canvas(modifier = modifier) {
        val ringStroke = size.minDimension * 0.038f
        val inset = ringStroke / 2f + 5.dp.toPx()
        val arcSize = androidx.compose.ui.geometry.Size(
            width = size.width - inset * 2f,
            height = size.height - inset * 2f,
        )
        val topLeft = Offset(inset, inset)

        drawCircle(
            color = Color.White.copy(alpha = 0.06f),
            radius = size.minDimension / 2f - 2.dp.toPx(),
            style = Stroke(width = 1.2.dp.toPx()),
        )
        listOf(
            Triple(StatusUiPalette.Red, 110f, 74f),
            Triple(StatusUiPalette.Red, 208f, 46f),
            Triple(StatusUiPalette.Orange, 264f, 52f),
            Triple(StatusUiPalette.Red, 320f, 68f),
            Triple(StatusUiPalette.Red, 32f, 44f),
        ).forEach { (color, start, sweep) ->
            drawArc(
                color = color,
                startAngle = start,
                sweepAngle = sweep,
                useCenter = false,
                topLeft = topLeft,
                size = arcSize,
                style = Stroke(width = ringStroke, cap = StrokeCap.Round),
            )
        }
        drawArc(
            color = StatusUiPalette.Red.copy(alpha = 0.22f),
            startAngle = 152f,
            sweepAngle = 30f,
            useCenter = false,
            topLeft = topLeft,
            size = arcSize,
            style = Stroke(width = ringStroke * 0.72f, cap = StrokeCap.Round),
        )
    }
}
