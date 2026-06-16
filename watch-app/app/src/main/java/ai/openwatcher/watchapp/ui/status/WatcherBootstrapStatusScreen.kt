package ai.openwatcher.watchapp.ui.status

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlin.math.sqrt
import ai.openwatcher.watchapp.ui.BootstrapUiState
import ai.openwatcher.watchapp.ui.components.StatusUiPalette

@Composable
internal fun WatcherBootstrapStatusScreen(
    state: BootstrapUiState,
    onConfirm: () -> Unit,
    onCancel: () -> Unit,
    modifier: Modifier = Modifier,
) {
    BoxWithConstraints(
        modifier = modifier
            .fillMaxSize()
    ) {
        val circleDiameter = minOf(maxWidth, maxHeight)
        val contentSquareSize = (circleDiameter.value / sqrt(2f)).dp

        Box(
            modifier = Modifier
                .align(Alignment.Center)
                .size(contentSquareSize)
        ) {
            val message = buildString {
                append(state.title)
                append("\n")
                append(state.detailLabel)
            }

            Text(
                text = message,
                color = StatusUiPalette.TextPrimary,
                fontSize = 13.sp,
                lineHeight = 16.sp,
                fontWeight = FontWeight.SemiBold,
                maxLines = 6,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier
                    .align(Alignment.TopStart)
                    .fillMaxWidth()
                    .padding(start = 10.dp, top = 12.dp, end = 10.dp)
            )

            Row(
                modifier = Modifier
                    .align(Alignment.BottomCenter)
                    .padding(bottom = 10.dp),
                horizontalArrangement = Arrangement.spacedBy(22.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                CircleActionButton(
                    kind = ActionKind.Confirm,
                    backgroundColor = if (state.canConfirm && !state.isProcessing) Color(0xFF25A55A) else Color(0xFF294236),
                    onClick = onConfirm,
                    enabled = state.canConfirm && !state.isProcessing,
                )
                CircleActionButton(
                    kind = ActionKind.Reject,
                    backgroundColor = if (!state.isProcessing) Color(0xFFCC4747) else Color(0xFF4B2B2B),
                    onClick = onCancel,
                    enabled = !state.isProcessing,
                )
            }
        }
    }
}

private enum class ActionKind {
    Confirm,
    Reject,
}

@Composable
private fun CircleActionButton(
    kind: ActionKind,
    backgroundColor: Color,
    onClick: () -> Unit,
    enabled: Boolean,
) {
    Box(
        modifier = Modifier
            .size(38.dp)
            .clip(CircleShape)
            .background(backgroundColor)
            .then(if (enabled) Modifier.clickable(onClick = onClick) else Modifier),
        contentAlignment = Alignment.Center,
    ) {
        Canvas(modifier = Modifier.size(20.dp)) {
            val strokeWidth = 3.4.dp.toPx()
            if (kind == ActionKind.Confirm) {
                val start = Offset(size.width * 0.14f, size.height * 0.56f)
                val middle = Offset(size.width * 0.38f, size.height * 0.82f)
                val end = Offset(size.width * 0.88f, size.height * 0.20f)
                drawLine(
                    color = Color.White,
                    start = start,
                    end = middle,
                    strokeWidth = strokeWidth,
                    cap = StrokeCap.Round,
                )
                drawLine(
                    color = Color.White,
                    start = middle,
                    end = end,
                    strokeWidth = strokeWidth,
                    cap = StrokeCap.Round,
                )
            } else {
                val inset = size.width * 0.20f
                drawLine(
                    color = Color.White,
                    start = Offset(inset, inset),
                    end = Offset(size.width - inset, size.height - inset),
                    strokeWidth = strokeWidth,
                    cap = StrokeCap.Round,
                )
                drawLine(
                    color = Color.White,
                    start = Offset(size.width - inset, inset),
                    end = Offset(inset, size.height - inset),
                    strokeWidth = strokeWidth,
                    cap = StrokeCap.Round,
                )
            }
        }
    }
}
