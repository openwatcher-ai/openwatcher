package ai.openwatcher.watchapp.ui.status

import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlin.math.sqrt
import ai.openwatcher.watchapp.R
import ai.openwatcher.watchapp.ui.PairingUiState
import ai.openwatcher.watchapp.ui.components.PairingHalo
import ai.openwatcher.watchapp.ui.components.StatusCapsuleDefaults
import ai.openwatcher.watchapp.ui.components.StatusIcon
import ai.openwatcher.watchapp.ui.components.StatusQrCard
import ai.openwatcher.watchapp.ui.components.StatusUiPalette
import ai.openwatcher.watchapp.util.QrCodeBitmapGenerator

private enum class PairingVisualMode {
    Pairing,
    TokenError,
    Offline,
}

private data class PairingStatusModel(
    val mode: PairingVisualMode,
    val headerLabel: String,
    val hostLabel: String,
    val title: String,
    val accentColor: Color,
    val bottomLabel: String,
    val actionTitle: String,
)

@Composable
internal fun WatcherPairingStatusScreen(
    state: PairingUiState,
    onSettings: () -> Unit,
    onRegenerate: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val clipboard = LocalClipboardManager.current
    val model = remember(state) { derivePairingStatusModel(state) }
    BoxWithConstraints(
        modifier = modifier.fillMaxSize(),
    ) {
        val minSide = minOf(maxWidth, maxHeight)
        val isInitialPairing = model.mode == PairingVisualMode.Pairing && !state.authStepCompleted
        val density = LocalDensity.current
        val horizontalPadding = minSide * 0.08f
        val verticalPadding = minSide * 0.055f
        val contentGap = minSide * 0.018f
        val haloSize = minSide * if (model.mode == PairingVisualMode.Offline) 0.326f else 0.344f
        val qrSize = haloSize * if (model.mode == PairingVisualMode.Offline) 0.72f else 0.74f
        val qrPadding = minSide * 0.022f
        val fingerprintHeight = minSide * 0.106f
        val actionHeight = minSide * 0.115f
        val actionHorizontalInset = minSide * 0.11f
        val actionGap = minSide * 0.022f

        if (state.bootstrapCode.isNotBlank()) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = horizontalPadding, vertical = verticalPadding),
            ) {
                Text(
                    text = state.statusLabel,
                    modifier = Modifier.align(Alignment.TopCenter),
                    color = StatusUiPalette.TextPrimary,
                    fontSize = 13.sp,
                    fontWeight = FontWeight.SemiBold,
                    textAlign = TextAlign.Center,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Column(
                    modifier = Modifier.align(Alignment.Center),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Text(
                        text = state.bootstrapCode,
                        color = StatusUiPalette.TextPrimary,
                        fontSize = 24.sp,
                        fontWeight = FontWeight.Bold,
                        textAlign = TextAlign.Center,
                        letterSpacing = 1.2.sp,
                        maxLines = 1,
                    )
                    Text(
                        text = state.bootstrapDetailLabel.ifBlank { state.hintLabel },
                        color = StatusUiPalette.TextSecondary,
                        fontSize = 8.sp,
                        lineHeight = 10.sp,
                        textAlign = TextAlign.Center,
                        maxLines = 3,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                BootstrapSettingsButton(
                    modifier = Modifier
                        .align(Alignment.BottomCenter)
                        .padding(bottom = 1.dp),
                    size = actionHeight,
                    onClick = onSettings,
                )
            }
        } else if (state.qrPayload.isBlank()) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = horizontalPadding, vertical = verticalPadding),
            ) {
                Text(
                    text = state.statusLabel,
                    modifier = Modifier.align(Alignment.TopCenter),
                    color = StatusUiPalette.TextPrimary,
                    fontSize = 13.sp,
                    fontWeight = FontWeight.SemiBold,
                    textAlign = TextAlign.Center,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    text = state.bootstrapDetailLabel.ifBlank { state.hintLabel },
                    modifier = Modifier
                        .align(Alignment.Center)
                        .padding(horizontal = horizontalPadding),
                    color = StatusUiPalette.TextSecondary,
                    fontSize = 9.sp,
                    lineHeight = 11.sp,
                    textAlign = TextAlign.Center,
                    maxLines = 4,
                    overflow = TextOverflow.Ellipsis,
                )
                BootstrapSettingsButton(
                    modifier = Modifier
                        .align(Alignment.BottomCenter)
                        .padding(bottom = 1.dp),
                    size = actionHeight,
                    onClick = onSettings,
                )
            }
        } else if (model.mode == PairingVisualMode.Offline) {
            OfflineServiceUnavailableContent(
                state = state,
                title = model.title,
                serviceAddress = state.serviceBaseUrl.ifBlank { model.hostLabel },
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = horizontalPadding, vertical = verticalPadding),
            )
        } else if (isInitialPairing) {
            val circleDiameter = minSide
            val qrCardSize = circleDiameter / sqrt(2f)
            val arcBandHeight = (circleDiameter - qrCardSize) / 2f
            val qrSizePx = with(density) { qrCardSize.roundToPx().coerceAtLeast(1) }
            val qrBitmap = remember(state.qrPayload, qrSizePx) {
                QrCodeBitmapGenerator.generate(state.qrPayload, qrSizePx)
            }

            Box(
                modifier = Modifier.fillMaxSize(),
            ) {
                Text(
                    text = model.title,
                    modifier = Modifier
                        .align(Alignment.TopCenter)
                        .padding(top = arcBandHeight * 0.22f),
                    textAlign = TextAlign.Center,
                    color = StatusUiPalette.TextPrimary,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )

                StatusQrCard(
                    bitmap = qrBitmap,
                    borderColor = model.accentColor,
                    modifier = Modifier
                        .size(qrCardSize)
                        .align(Alignment.Center),
                    contentPadding = 0.dp,
                )

                Text(
                    text = "${model.headerLabel} · ${model.hostLabel}",
                    modifier = Modifier
                        .align(Alignment.BottomCenter)
                        .padding(bottom = arcBandHeight * 0.22f),
                    textAlign = TextAlign.Center,
                    color = StatusUiPalette.TextSecondary,
                    fontSize = 8.sp,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        } else {
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = horizontalPadding, vertical = verticalPadding),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(contentGap),
            ) {
                PairingHeader(model = model)

                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(haloSize),
                    contentAlignment = Alignment.Center,
                ) {
                    val qrSizePx = with(density) { qrSize.roundToPx().coerceAtLeast(1) }
                    val qrBitmap = remember(state.qrPayload, qrSizePx) {
                        QrCodeBitmapGenerator.generate(state.qrPayload, qrSizePx)
                    }
                    Box(
                        modifier = Modifier
                            .size(haloSize)
                            .aspectRatio(1f),
                        contentAlignment = Alignment.Center,
                    ) {
                        PairingHalo(
                            modifier = Modifier.fillMaxSize(),
                            accentColor = if (model.mode == PairingVisualMode.TokenError) StatusUiPalette.Red else StatusUiPalette.Green,
                        )

                        StatusQrCard(
                            bitmap = qrBitmap,
                            borderColor = model.accentColor,
                            modifier = Modifier.size(qrSize),
                            contentPadding = qrPadding,
                        )
                    }
                }

                FingerprintRow(
                    fingerprint = state.tokenFingerprint,
                    label = model.bottomLabel,
                    tint = model.accentColor,
                    height = fingerprintHeight,
                    onCopy = {
                        clipboard.setText(AnnotatedString(state.tokenFingerprint))
                    },
                )

                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = actionHorizontalInset),
                    horizontalArrangement = Arrangement.spacedBy(actionGap),
                ) {
                    PairingActionButton(
                        title = model.actionTitle,
                        iconRes = R.drawable.ic_repair_link,
                        tint = model.accentColor,
                        height = actionHeight,
                        onClick = onRegenerate,
                        modifier = Modifier.weight(1f),
                    )
                    PairingActionButton(
                        title = "设置",
                        iconRes = R.drawable.ic_settings_gear,
                        tint = StatusUiPalette.Blue,
                        height = actionHeight,
                        onClick = onSettings,
                        modifier = Modifier.weight(1f),
                    )
                }

            }
        }
    }
}

@Composable
private fun OfflineServiceUnavailableContent(
    state: PairingUiState,
    title: String,
    serviceAddress: String,
    modifier: Modifier = Modifier,
) {
    val scrollState = rememberScrollState()
    val resolvedServiceAddress = serviceAddress.ifBlank { "未配置" }
    Column(
        modifier = modifier,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = title,
            color = StatusUiPalette.TextPrimary,
            fontSize = 13.sp,
            fontWeight = FontWeight.SemiBold,
            textAlign = TextAlign.Center,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        Spacer(modifier = Modifier.height(8.dp))
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .weight(1f)
                .verticalScroll(scrollState),
            verticalArrangement = Arrangement.spacedBy(7.dp),
        ) {
            if (state.environmentLabel.isNotBlank()) {
                OfflineBodyText(
                    text = state.environmentLabel,
                    color = StatusUiPalette.TextPrimary,
                    fontWeight = FontWeight.Medium,
                )
            }
            OfflineBodyText(
                text = "服务地址：$resolvedServiceAddress",
                color = StatusUiPalette.TextPrimary,
                fontWeight = FontWeight.Medium,
            )
            OfflineBodyText(
                text = "建议：",
                color = StatusUiPalette.TextSecondary,
                fontWeight = FontWeight.Medium,
            )
            OfflineBodyText(text = "1. 检查服务状态是否正常。")
            OfflineBodyText(
                text = "2. 重新通过桌面应用的安装向导，或「手表设备->远程初始化」进行配置重新设置（选择远程初始化需清空应用数据）。",
            )
        }
    }
}

@Composable
private fun OfflineBodyText(
    text: String,
    color: Color = StatusUiPalette.TextSecondary,
    fontWeight: FontWeight = FontWeight.Normal,
) {
    Text(
        text = text,
        color = color,
        fontSize = 9.sp,
        lineHeight = 11.sp,
        fontWeight = fontWeight,
        textAlign = TextAlign.Start,
    )
}

@Composable
private fun PairingHeader(
    model: PairingStatusModel,
) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(3.dp),
    ) {
        Text(
            text = model.title,
            color = StatusUiPalette.TextPrimary,
            fontSize = 10.sp,
            fontWeight = FontWeight.SemiBold,
            textAlign = TextAlign.Center,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        Text(
            text = "${model.headerLabel} · ${model.hostLabel}",
            color = StatusUiPalette.TextSecondary,
            fontSize = 6.sp,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

@Composable
private fun FingerprintRow(
    fingerprint: String,
    label: String,
    tint: Color,
    height: Dp,
    onCopy: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.Center,
    ) {
        Row(
            modifier = Modifier
                .height(height)
                .clip(RoundedCornerShape(StatusCapsuleDefaults.bubbleCorner))
                .background(Color.White.copy(alpha = 0.035f))
                .border(
                    1.dp,
                    tint.copy(alpha = 0.35f),
                    RoundedCornerShape(StatusCapsuleDefaults.bubbleCorner),
                )
                .padding(horizontal = StatusCapsuleDefaults.tightHorizontalPadding, vertical = 0.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(5.dp),
        ) {
            Text(
                text = label,
                color = StatusUiPalette.TextSecondary,
                fontSize = 6.sp,
                maxLines = 1,
            )
            Text(
                text = fingerprint,
                color = StatusUiPalette.TextPrimary,
                fontSize = 7.sp,
                fontWeight = FontWeight.Medium,
                letterSpacing = 0.4.sp,
                maxLines = 1,
            )
            Box(
                modifier = Modifier
                    .size(12.dp)
                    .clip(CircleShape)
                    .clickable(onClick = onCopy)
                    .background(Color.White.copy(alpha = 0.04f)),
                contentAlignment = Alignment.Center,
            ) {
                StatusIcon(
                    iconRes = R.drawable.ic_copy_outline,
                    tint = StatusUiPalette.TextSecondary,
                    size = 8.dp,
                )
            }
        }
    }
}

@Composable
private fun PairingActionButton(
    title: String,
    iconRes: Int,
    tint: Color,
    height: Dp,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Card(
        modifier = modifier
            .fillMaxWidth()
            .height(height)
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(StatusCapsuleDefaults.compactCorner),
        colors = CardDefaults.cardColors(containerColor = StatusUiPalette.SurfaceAlt),
    ) {
        Row(
            modifier = Modifier
                .fillMaxSize()
                .border(
                    1.dp,
                    tint.copy(alpha = 0.5f),
                    RoundedCornerShape(StatusCapsuleDefaults.compactCorner),
                )
                .padding(horizontal = StatusCapsuleDefaults.tightHorizontalPadding, vertical = 2.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            StatusIcon(iconRes = iconRes, tint = tint, size = 11.dp)
            Text(
                text = title,
                color = StatusUiPalette.TextPrimary,
                fontSize = 7.sp,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

@Composable
private fun BootstrapSettingsButton(
    size: Dp,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier,
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        Box(
            modifier = Modifier
                .size(size)
                .clip(CircleShape)
                .background(StatusUiPalette.SurfaceAlt)
                .border(
                    1.dp,
                    StatusUiPalette.Blue.copy(alpha = 0.55f),
                    CircleShape,
                )
                .clickable(onClick = onClick),
            contentAlignment = Alignment.Center,
        ) {
            Box(
                modifier = Modifier
                    .size(size * 0.74f)
                    .clip(CircleShape)
                    .background(StatusUiPalette.Blue.copy(alpha = 0.14f)),
                contentAlignment = Alignment.Center,
            ) {
                StatusIcon(
                    iconRes = R.drawable.ic_settings_gear,
                    tint = StatusUiPalette.Blue,
                    size = size * 0.36f,
                )
            }
        }
        Text(
            text = "设置",
            color = StatusUiPalette.TextSecondary,
            fontSize = 7.sp,
            fontWeight = FontWeight.Medium,
            maxLines = 1,
        )
    }
}

private fun derivePairingStatusModel(state: PairingUiState): PairingStatusModel {
    val hostLabel = state.serviceHostLabel.ifBlank { extractHostLabel(state.qrPayload) }
    return when (state.statusLabel) {
        "服务不可达", "服务异常", "返回异常", "等待服务连接" -> PairingStatusModel(
            mode = PairingVisualMode.Offline,
            headerLabel = "连接异常",
            hostLabel = hostLabel,
            title = if (state.statusLabel == "等待服务连接") "等待服务连接" else state.statusLabel,
            accentColor = if (state.statusLabel == "返回异常") StatusUiPalette.Orange else StatusUiPalette.Red,
            bottomLabel = "",
            actionTitle = "重试",
        )

        "token 错误" -> PairingStatusModel(
            mode = PairingVisualMode.TokenError,
            headerLabel = "配对失效",
            hostLabel = hostLabel,
            title = "需要重新配对",
            accentColor = StatusUiPalette.Red,
            bottomLabel = "失效",
            actionTitle = "重配",
        )

        else -> PairingStatusModel(
            mode = PairingVisualMode.Pairing,
            headerLabel = state.serviceLabel,
            hostLabel = hostLabel,
            title = state.statusLabel,
            accentColor = if (state.serviceLabel.contains("已连接") || state.statusLabel == "已配对") {
                StatusUiPalette.Green
            } else {
                StatusUiPalette.Cyan
            },
            bottomLabel = if (state.authStepCompleted) "已授权" else "待授权",
            actionTitle = "重配",
        )
    }
}

private fun extractHostLabel(payload: String): String {
    if (payload.isBlank()) {
        return "配对地址待生成"
    }
    val uri = runCatching { Uri.parse(payload) }.getOrNull()
    return uri?.host?.takeIf { it.isNotBlank() }
        ?: payload.removePrefix("https://").removePrefix("http://").take(28)
}
