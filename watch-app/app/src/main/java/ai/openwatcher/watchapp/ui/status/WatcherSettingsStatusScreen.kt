package ai.openwatcher.watchapp.ui.status

import androidx.activity.compose.BackHandler
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import kotlin.math.cos
import kotlin.math.min
import kotlin.math.roundToInt
import kotlin.math.sin
import kotlin.math.sqrt
import ai.openwatcher.watchapp.BuildConfig
import ai.openwatcher.watchapp.R
import ai.openwatcher.watchapp.ui.AppUpdateStatus
import ai.openwatcher.watchapp.ui.AppUpdateDownloadOverlayUiState
import ai.openwatcher.watchapp.ui.AppUpdateUiState
import ai.openwatcher.watchapp.ui.AppUpdateVersionNotesUiState
import ai.openwatcher.watchapp.ui.DiagnosticPromptUiState
import ai.openwatcher.watchapp.ui.DiagnosticUploadUiState
import ai.openwatcher.watchapp.ui.SettingsDestination
import ai.openwatcher.watchapp.ui.SettingsUiState
import ai.openwatcher.watchapp.ui.components.StatusCapsuleDefaults
import ai.openwatcher.watchapp.ui.components.ServiceHealthIndicator
import ai.openwatcher.watchapp.ui.components.StatusUiPalette
import ai.openwatcher.watchapp.util.QrCodeBitmapGenerator

private val SettingsPageHorizontalPadding = 10.dp
private val SettingsRowCornerRadius = 10.dp
private val SettingsActionRowCornerRadius = 9.dp
private const val InscribedSquareRatio = 1.41421356f
private const val OpenWatcherQrUrl = "https://openwatcher.ai"

private data class SettingsTypography(
    val pageTitle: TextUnit,
    val rowTitle: TextUnit,
    val rowTitleLineHeight: TextUnit,
    val rowSubtitle: TextUnit,
    val rowSubtitleLineHeight: TextUnit,
    val trailing: TextUnit,
    val arrow: TextUnit,
    val panelTitle: TextUnit,
    val panelBody: TextUnit,
    val panelBodyLineHeight: TextUnit,
    val actionLabel: TextUnit,
    val actionArrow: TextUnit,
    val capsuleWidth: Dp,
    val capsuleMinHeight: Dp,
)

@Composable
internal fun WatcherSettingsStatusScreen(
    state: SettingsUiState,
    onOpenDestination: (SettingsDestination) -> Unit,
    onOpenAppUpdate: () -> Unit,
    onSecretVersionTap: () -> Unit,
    onDiagnosticEntryClick: () -> Unit,
    onConfirmDiagnosticPromptPrimary: () -> Unit,
    onConfirmDiagnosticPromptSecondary: () -> Unit,
    onDismissDiagnosticPrompt: () -> Unit,
    onToggleAutoCheckUpdate: (Boolean) -> Unit,
    onInstallUpdate: () -> Unit,
    onIgnoreUpdate: () -> Unit,
    onOpenInstallPermissionSettings: () -> Unit,
    onRepair: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val typography = rememberSettingsTypography()
    var showOpenWatcherQr by rememberSaveable { mutableStateOf(false) }
    val useEdgeToEdgeUpdateNotes =
        state.destination == SettingsDestination.UpdateNotes ||
            state.destination == SettingsDestination.CurrentVersionNotes
    LaunchedEffect(state.destination) {
        if (state.destination != SettingsDestination.About) {
            showOpenWatcherQr = false
        }
    }
    if (showOpenWatcherQr) {
        BackHandler(onBack = { showOpenWatcherQr = false })
    }
    Box(modifier = modifier.fillMaxSize()) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .padding(
                    horizontal = if (useEdgeToEdgeUpdateNotes) 0.dp else SettingsPageHorizontalPadding,
                    vertical = if (useEdgeToEdgeUpdateNotes) 0.dp else 10.dp,
                ),
        ) {
            when (state.destination) {
                SettingsDestination.CurrentVersionNotes -> Box(
                    modifier = Modifier.fillMaxSize(),
                ) {
                    VersionNotesPage(
                        title = titleForDestination(state.destination),
                        notes = state.update.currentVersionNotes,
                        onVersionLabelTap = onSecretVersionTap,
                        typography = typography,
                        modifier = Modifier.fillMaxSize(),
                    )
                }

                SettingsDestination.UpdateNotes -> Box(
                    modifier = Modifier.fillMaxSize(),
                ) {
                    UpdateNotesContent(
                        state = state.update,
                        onInstallUpdate = onInstallUpdate,
                        onIgnoreUpdate = onIgnoreUpdate,
                        onOpenInstallPermissionSettings = onOpenInstallPermissionSettings,
                        typography = typography,
                        modifier = Modifier
                            .fillMaxSize(),
                    )
                }

                else -> {
                    val scrollState = rememberScrollState()
                    LaunchedEffect(state.destination) {
                        scrollState.scrollTo(0)
                    }

                    Column(
                        modifier = Modifier
                            .fillMaxSize()
                            .verticalScroll(scrollState),
                        verticalArrangement = Arrangement.spacedBy(4.dp),
                    ) {
                        SettingsTitle(
                            title = titleForDestination(state.destination),
                            typography = typography,
                        )

                        when (state.destination) {
                            SettingsDestination.Root -> RootSettingsContent(
                                state = state,
                                diagnosticUpload = state.diagnosticUpload,
                                onDiagnosticEntryClick = onDiagnosticEntryClick,
                                onOpenDestination = onOpenDestination,
                                onRepair = onRepair,
                                typography = typography,
                            )

                            SettingsDestination.About -> AboutContent(
                                state = state,
                                onOpenOpenWatcher = { showOpenWatcherQr = true },
                                onOpenAppUpdate = onOpenAppUpdate,
                                onOpenInstallPermissionSettings = onOpenInstallPermissionSettings,
                                typography = typography,
                            )

                            SettingsDestination.UpdateCheck -> UpdateCheckContent(
                                state = state,
                                typography = typography,
                            )

                            SettingsDestination.UpdateLatest -> UpdateLatestContent(
                                state = state,
                                onOpenDestination = onOpenDestination,
                                onToggleAutoCheckUpdate = onToggleAutoCheckUpdate,
                                typography = typography,
                            )

                            SettingsDestination.CurrentVersionNotes,
                            SettingsDestination.UpdateNotes,
                            -> Unit
                        }

                        if (state.destination != SettingsDestination.UpdateCheck || state.update.status != AppUpdateStatus.Checking) {
                            Spacer(modifier = Modifier.height(24.dp))
                        }
                    }
                }
            }
        }
        if (showOpenWatcherQr) {
            OpenWatcherQrOverlay(
                versionLabel = openWatcherDisplayVersion(),
                onDismiss = { showOpenWatcherQr = false },
                modifier = Modifier.fillMaxSize(),
            )
        }
        if (state.diagnosticPrompt.visible) {
            DiagnosticPromptOverlay(
                state = state.diagnosticPrompt,
                typography = typography,
                onPrimaryAction = onConfirmDiagnosticPromptPrimary,
                onSecondaryAction = onConfirmDiagnosticPromptSecondary,
                onDismiss = onDismissDiagnosticPrompt,
                modifier = Modifier.fillMaxSize(),
            )
        }
        if (state.diagnosticUpload.progressOverlay.visible) {
            AppUpdateDownloadOverlay(
                state = state.diagnosticUpload.progressOverlay,
                typography = typography,
                modifier = Modifier.fillMaxSize(),
            )
        }
    }
}

@Composable
private fun RootSettingsContent(
    state: SettingsUiState,
    diagnosticUpload: DiagnosticUploadUiState,
    onDiagnosticEntryClick: () -> Unit,
    onOpenDestination: (SettingsDestination) -> Unit,
    onRepair: () -> Unit,
    typography: SettingsTypography,
) {
    SettingsListRow(
        title = "服务状态",
        subtitle = state.baseUrl.ifBlank { "未配置" },
        trailing = {
            ServiceHealthIndicator(
                status = state.healthCheck.status,
            )
        },
        subtitleMaxLines = 2,
        typography = typography,
    )
    SettingsListRow(
        title = "应用诊断",
        subtitle = diagnosticUpload.entrySubtitle,
        enabled = diagnosticUpload.entryEnabled,
        subtitleMaxLines = Int.MAX_VALUE,
        typography = typography,
        onClick = onDiagnosticEntryClick,
    )
    SettingsListRow(
        title = "重新配对",
        subtitle = "",
        subtitleMaxLines = 1,
        typography = typography,
        onClick = onRepair,
    )
    SettingsListRow(
        title = "关于",
        subtitle = "",
        subtitleMaxLines = 1,
        typography = typography,
        onClick = { onOpenDestination(SettingsDestination.About) },
    )
}

@Composable
private fun AboutContent(
    state: SettingsUiState,
    onOpenOpenWatcher: () -> Unit,
    onOpenAppUpdate: () -> Unit,
    onOpenInstallPermissionSettings: () -> Unit,
    typography: SettingsTypography,
) {
    SettingsListRow(
        title = "产品功能介绍",
        subtitle = "OpenWatcher(v${openWatcherDisplayVersion()})",
        subtitleMaxLines = 1,
        typography = typography,
        onClick = onOpenOpenWatcher,
    )
    SettingsListRow(
        title = "版本更新",
        subtitle = "",
        subtitleMaxLines = 1,
        typography = typography,
        onClick = onOpenAppUpdate,
    )
    SettingsListRow(
        title = "安装未知来源",
        subtitle = if (state.update.installPermissionEnabled) {
            "已允许从当前应用安装更新"
        } else {
            "点击前往系统设置开启"
        },
        trailingText = state.update.installPermissionLabel,
        subtitleMaxLines = 2,
        typography = typography,
        onClick = onOpenInstallPermissionSettings,
    )
}

@Composable
private fun OpenWatcherQrOverlay(
    versionLabel: String,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier,
) {
    BoxWithConstraints(
        modifier = modifier
            .background(Color(0xC20A0D14))
            .clickable(
                interactionSource = remember { MutableInteractionSource() },
                indication = null,
                onClick = onDismiss,
            ),
    ) {
        val circleDiameter = if (maxWidth < maxHeight) maxWidth else maxHeight
        val contentSquareSize = (circleDiameter.value / InscribedSquareRatio).dp
        val qrSizePx = with(LocalDensity.current) { contentSquareSize.roundToPx().coerceAtLeast(1) }
        val qrBitmap = remember(qrSizePx) {
            QrCodeBitmapGenerator.generate(OpenWatcherQrUrl, qrSizePx)
        }

        Box(
            modifier = Modifier.fillMaxSize(),
            contentAlignment = Alignment.Center,
        ) {
            Image(
                bitmap = qrBitmap,
                contentDescription = "OpenWatcher v$versionLabel 官网二维码",
                modifier = Modifier
                    .size(contentSquareSize)
                    .clickable(
                        interactionSource = remember { MutableInteractionSource() },
                        indication = null,
                        onClick = {},
                    ),
            )
        }
    }
}

@Composable
private fun UpdateCheckContent(
    state: SettingsUiState,
    typography: SettingsTypography,
) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        UpdateCheckHero(
            status = state.update.status,
            detailLabel = state.update.detailLabel,
            typography = typography,
        )
    }
}

@Composable
private fun UpdateLatestContent(
    state: SettingsUiState,
    onOpenDestination: (SettingsDestination) -> Unit,
    onToggleAutoCheckUpdate: (Boolean) -> Unit,
    typography: SettingsTypography,
) {
    SettingsListRow(
        title = "当前版本",
        subtitle = "${state.update.currentVersionLabel} · ${state.update.channelLabel}",
        subtitleMaxLines = 1,
        typography = typography,
        onClick = { onOpenDestination(SettingsDestination.CurrentVersionNotes) },
    )
    SettingsListRow(
        title = "自动检查",
        subtitle = "",
        subtitleMaxLines = 1,
        trailing = {
            Switch(
                checked = state.update.autoCheckEnabled,
                onCheckedChange = onToggleAutoCheckUpdate,
                colors = SwitchDefaults.colors(
                    checkedThumbColor = Color.White,
                    checkedTrackColor = Color(0xFF1677FF),
                    checkedBorderColor = Color(0xFF1677FF),
                ),
            )
        },
        typography = typography,
        onClick = { onToggleAutoCheckUpdate(!state.update.autoCheckEnabled) },
    )
}

@Composable
private fun UpdateCheckHero(
    status: AppUpdateStatus,
    detailLabel: String,
    typography: SettingsTypography,
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 8.dp, vertical = 8.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        UpdateCheckSpinner(
            animate = status == AppUpdateStatus.Checking,
        )
        Text(
            text = when (status) {
                AppUpdateStatus.Checking -> "设备更新中"
                AppUpdateStatus.Failed -> "检查更新失败"
                AppUpdateStatus.UpToDate -> "当前已是最新版本"
                AppUpdateStatus.Idle -> "设备更新中"
                else -> "设备更新中"
            },
            color = StatusUiPalette.TextPrimary,
            fontSize = typography.panelTitle,
            fontWeight = FontWeight.SemiBold,
            textAlign = TextAlign.Center,
        )
        if (status == AppUpdateStatus.Failed) {
            Text(
                text = detailLabel,
                color = StatusUiPalette.TextSecondary,
                fontSize = typography.panelBody,
                lineHeight = typography.panelBodyLineHeight,
                textAlign = TextAlign.Center,
            )
        }
    }
}

@Composable
private fun UpdateNotesContent(
    state: AppUpdateUiState,
    onInstallUpdate: () -> Unit,
    onIgnoreUpdate: () -> Unit,
    onOpenInstallPermissionSettings: () -> Unit,
    typography: SettingsTypography,
    modifier: Modifier = Modifier,
) {
    val primaryAction = when (state.status) {
        AppUpdateStatus.Available -> "下载更新"
        AppUpdateStatus.PermissionRequired -> "去开启"
        AppUpdateStatus.Failed -> "重试"
        else -> null
    }
    val showIgnoreAction = state.hasPendingUpdate && state.status != AppUpdateStatus.Downloading
    VersionNotesPage(
        notes = state.latestVersionNotes,
        onVersionLabelTap = {},
        comparisonLabel = state.comparisonLabel,
        statusLabel = state.detailLabel.takeUnless { state.status == AppUpdateStatus.ReadyToInstall || it.isBlank() },
        primaryActionLabel = primaryAction,
        onPrimaryAction = when (state.status) {
            AppUpdateStatus.PermissionRequired -> onOpenInstallPermissionSettings
            AppUpdateStatus.Available,
            AppUpdateStatus.Failed,
            -> onInstallUpdate
            else -> null
        },
        secondaryActionLabel = if (showIgnoreAction) "忽略本次" else null,
        onSecondaryAction = if (showIgnoreAction) onIgnoreUpdate else null,
        overlay = state.downloadOverlay,
        typography = typography,
        modifier = modifier,
    )
}

@Composable
private fun VersionNotesPage(
    title: String = "更新说明",
    notes: AppUpdateVersionNotesUiState,
    onVersionLabelTap: () -> Unit,
    comparisonLabel: String? = null,
    statusLabel: String? = null,
    primaryActionLabel: String? = null,
    onPrimaryAction: (() -> Unit)? = null,
    secondaryActionLabel: String? = null,
    onSecondaryAction: (() -> Unit)? = null,
    overlay: AppUpdateDownloadOverlayUiState = AppUpdateDownloadOverlayUiState(),
    typography: SettingsTypography,
    modifier: Modifier = Modifier,
) {
    val scrollState = rememberScrollState()
    val scope = rememberCoroutineScope()
    val useBottomImageActions = primaryActionLabel == "下载更新" &&
        secondaryActionLabel == "忽略本次" &&
        onPrimaryAction != null &&
        onSecondaryAction != null
    val hasBottomActions = !useBottomImageActions && (primaryActionLabel != null || secondaryActionLabel != null)
    val contentBottomPadding = if (hasBottomActions) 20.dp else 16.dp
    val canScrollUp = scrollState.value > 0
    val canScrollDown = scrollState.value < scrollState.maxValue
    val showScrollArrows = true

    BoxWithConstraints(modifier = modifier.fillMaxSize()) {
        val circleDiameter = if (maxWidth < maxHeight) maxWidth else maxHeight
        val contentSquareSize = (circleDiameter.value / sqrt(2f)).dp
        val topBandHeight = ((maxHeight - contentSquareSize) / 2f).coerceAtLeast(0.dp)
        val bottomBandHeight = topBandHeight
        val rightBandWidth = ((maxWidth - contentSquareSize) / 2f).coerceAtLeast(0.dp)
        val bodyHorizontalPadding = (contentSquareSize.value * 0.035f).dp
        val bodyTopPadding = (contentSquareSize.value * 0.03f).dp

        if (topBandHeight > 0.dp) {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(topBandHeight),
                contentAlignment = Alignment.Center,
            ) {
                UpdateNotesMeasuredTitle(
                    title = title,
                    circleDiameter = circleDiameter,
                    bandHeight = topBandHeight,
                )
            }
        }

        if (useBottomImageActions && bottomBandHeight > 0.dp) {
            Box(
                modifier = Modifier
                    .align(Alignment.BottomCenter)
                    .fillMaxWidth()
                    .height(bottomBandHeight),
                contentAlignment = Alignment.Center,
            ) {
                UpdateNotesBottomImageButtons(
                    onDownload = onPrimaryAction ?: {},
                    onIgnore = onSecondaryAction ?: {},
                )
            }
        }

        Box(
            modifier = Modifier
                .align(Alignment.Center)
                .size(contentSquareSize),
        ) {
            VersionNotesTransparentContent(
                notes = notes,
                onVersionLabelTap = onVersionLabelTap,
                comparisonLabel = comparisonLabel,
                statusLabel = statusLabel,
                scrollState = scrollState,
                typography = typography,
                hasBottomActions = hasBottomActions,
                primaryActionLabel = primaryActionLabel,
                onPrimaryAction = onPrimaryAction,
                secondaryActionLabel = secondaryActionLabel,
                onSecondaryAction = onSecondaryAction,
                contentBottomPadding = contentBottomPadding,
                bodyHorizontalPadding = bodyHorizontalPadding,
                bodyTopPadding = bodyTopPadding,
            )
        }

        if (showScrollArrows && rightBandWidth >= 18.dp) {
            Box(
                modifier = Modifier
                    .align(Alignment.CenterEnd)
                    .width(rightBandWidth)
                    .height(contentSquareSize),
                contentAlignment = Alignment.Center,
            ) {
                ScrollArrowControls(
                    canScrollUp = canScrollUp,
                    canScrollDown = canScrollDown,
                    onScrollUp = { scope.launch { scrollState.animateScrollTo(0) } },
                    onScrollDown = { scope.launch { scrollState.animateScrollTo(scrollState.maxValue) } },
                    arrowWidth = 18.dp,
                    arrowHeight = 28.dp,
                )
            }
        }
        if (overlay.visible) {
            AppUpdateDownloadOverlay(
                state = overlay,
                typography = typography,
                modifier = Modifier.fillMaxSize(),
            )
        }
    }
}

@Composable
private fun UpdateNotesMeasuredTitle(
    title: String,
    circleDiameter: Dp,
    bandHeight: Dp,
    modifier: Modifier = Modifier,
) {
    val density = LocalDensity.current
    val textMeasurer = rememberTextMeasurer()
    val fontSize = remember(circleDiameter, bandHeight, density) {
        with(density) {
            val radiusPx = circleDiameter.toPx() / 2f
            val centerYPx = bandHeight.toPx() / 2f
            val dy = radiusPx - centerYPx
            val chordWidthPx = 2f * sqrt((radiusPx * radiusPx - dy * dy).coerceAtLeast(0f))
            val availableWidthPx = (chordWidthPx - 24.dp.toPx()).coerceAtLeast(0f).roundToInt()
            val availableHeightPx = (bandHeight.toPx() - 12.dp.toPx()).coerceAtLeast(0f).roundToInt()
            var low = 14f
            var high = 30f
            var best = 20f
            repeat(12) {
                val mid = (low + high) / 2f
                val result = textMeasurer.measure(
                    text = AnnotatedString(title),
                    style = TextStyle(
                        fontSize = mid.sp,
                        fontWeight = FontWeight.SemiBold,
                    ),
                    softWrap = false,
                )
                if (result.size.width <= availableWidthPx && result.size.height <= availableHeightPx) {
                    best = mid
                    low = mid
                } else {
                    high = mid
                }
            }
            best.sp
        }
    }
    Text(
        text = title,
        color = StatusUiPalette.TextPrimary,
        fontSize = fontSize,
        fontWeight = FontWeight.SemiBold,
        textAlign = TextAlign.Center,
        modifier = modifier.fillMaxWidth(),
    )
}

@Composable
private fun UpdateNotesBottomImageButtons(
    onDownload: () -> Unit,
    onIgnore: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Row(
            horizontalArrangement = Arrangement.spacedBy(10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            UpdateNotesImageButton(
                drawableRes = R.drawable.ic_update_download_button,
                contentDescription = "下载更新",
                onClick = onDownload,
            )
            UpdateNotesImageButton(
                drawableRes = R.drawable.ic_update_ignore_button,
                contentDescription = "忽略本次",
                onClick = onIgnore,
            )
        }
    }
}

@Composable
private fun UpdateNotesImageButton(
    drawableRes: Int,
    contentDescription: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Image(
        painter = painterResource(drawableRes),
        contentDescription = contentDescription,
        modifier = modifier
            .size(25.dp)
            .clickable(onClick = onClick),
    )
}

@Composable
private fun VersionNotesTransparentContent(
    notes: AppUpdateVersionNotesUiState,
    onVersionLabelTap: () -> Unit,
    comparisonLabel: String?,
    statusLabel: String?,
    scrollState: androidx.compose.foundation.ScrollState,
    typography: SettingsTypography,
    hasBottomActions: Boolean,
    primaryActionLabel: String?,
    onPrimaryAction: (() -> Unit)?,
    secondaryActionLabel: String?,
    onSecondaryAction: (() -> Unit)?,
    contentBottomPadding: Dp,
    bodyHorizontalPadding: Dp,
    bodyTopPadding: Dp,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .verticalScroll(scrollState)
            .padding(bottom = contentBottomPadding),
        verticalArrangement = Arrangement.spacedBy(0.dp),
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = bodyHorizontalPadding, vertical = bodyTopPadding),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Text(
                text = notes.versionLabel,
                color = StatusUiPalette.TextPrimary,
                fontSize = 17.sp,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.clickable(
                    interactionSource = remember { MutableInteractionSource() },
                    indication = null,
                    onClick = onVersionLabelTap,
                ),
            )
            comparisonLabel?.let { label ->
                Text(
                    text = label,
                    color = StatusUiPalette.TextSecondary,
                    fontSize = typography.panelBody,
                    lineHeight = typography.panelBodyLineHeight,
                )
            }
            statusLabel?.let { label ->
                Text(
                    text = label,
                    color = StatusUiPalette.TextSecondary,
                    fontSize = typography.panelBody,
                    lineHeight = typography.panelBodyLineHeight,
                )
            }
        }

        Spacer(modifier = Modifier.height(bodyTopPadding))
        VersionNotesDivider(horizontalPadding = bodyHorizontalPadding)

        if (notes.notes.isEmpty()) {
            Spacer(modifier = Modifier.height(bodyTopPadding))
            Text(
                text = notes.emptyLabel,
                color = StatusUiPalette.TextPrimary,
                fontSize = typography.panelTitle,
                fontWeight = FontWeight.Medium,
                modifier = Modifier.padding(horizontal = bodyHorizontalPadding),
            )
        } else {
            val orderedNotes = notes.notes.asReversed()
            orderedNotes.forEachIndexed { index, item ->
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = bodyHorizontalPadding, vertical = 10.dp),
                    verticalArrangement = Arrangement.spacedBy(4.dp),
                ) {
                    Text(
                        text = item.publishedAtLabel,
                        color = StatusUiPalette.TextSecondary,
                        fontSize = typography.panelBody,
                    )
                    Text(
                        text = item.summary,
                        color = StatusUiPalette.TextPrimary,
                        fontSize = typography.panelTitle,
                        lineHeight = (typography.panelBodyLineHeight.value + 2f).sp,
                        fontWeight = FontWeight.Medium,
                    )
                }
                if (index < orderedNotes.lastIndex) {
                    VersionNotesDivider(horizontalPadding = bodyHorizontalPadding)
                }
            }
        }

        if (hasBottomActions) {
            primaryActionLabel?.let { label ->
                onPrimaryAction?.let { action ->
                    Spacer(modifier = Modifier.height(10.dp))
                    VersionNotesInlineActionRow(label = label, onClick = action)
                }
            }
            secondaryActionLabel?.let { label ->
                onSecondaryAction?.let { action ->
                    Spacer(modifier = Modifier.height(8.dp))
                    VersionNotesInlineActionRow(label = label, onClick = action)
                }
            }
        }
    }
}

@Composable
private fun VersionNotesDivider(
    horizontalPadding: Dp = 0.dp,
    modifier: Modifier = Modifier,
) {
    Box(
        modifier = modifier
            .fillMaxWidth()
            .padding(horizontal = horizontalPadding)
            .height(1.dp)
            .background(Color.White.copy(alpha = 0.12f)),
    )
}

@Composable
private fun VersionNotesInlineActionRow(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(999.dp))
            .background(Color.White.copy(alpha = 0.10f))
            .clickable(onClick = onClick)
            .padding(horizontal = 10.dp, vertical = 8.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = label,
            color = StatusUiPalette.TextPrimary,
            fontSize = 12.sp,
            fontWeight = FontWeight.Medium,
        )
        Text(
            text = "›",
            color = StatusUiPalette.TextSecondary,
            fontSize = 14.sp,
        )
    }
}

@Composable
private fun ScrollArrowControls(
    canScrollUp: Boolean,
    canScrollDown: Boolean,
    onScrollUp: () -> Unit,
    onScrollDown: () -> Unit,
    arrowWidth: Dp,
    arrowHeight: Dp,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier,
        verticalArrangement = Arrangement.spacedBy(12.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        ScrollArrowButton(
            upward = true,
            enabled = canScrollUp,
            onClick = onScrollUp,
            arrowWidth = arrowWidth,
            arrowHeight = arrowHeight,
        )
        ScrollArrowButton(
            upward = false,
            enabled = canScrollDown,
            onClick = onScrollDown,
            arrowWidth = arrowWidth,
            arrowHeight = arrowHeight,
        )
    }
}

@Composable
private fun ScrollArrowButton(
    upward: Boolean,
    enabled: Boolean,
    onClick: () -> Unit,
    arrowWidth: Dp,
    arrowHeight: Dp,
    modifier: Modifier = Modifier,
) {
    Box(
        modifier = modifier
            .size(width = arrowWidth, height = arrowHeight)
            .clickable(enabled = enabled, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Canvas(modifier = Modifier.size(width = arrowWidth, height = arrowHeight)) {
            val stroke = 2.dp.toPx()
            val centerX = size.width / 2f
            val top = 2.dp.toPx()
            val bottom = size.height - 2.dp.toPx()
            val head = 4.dp.toPx()
            val color = Color.White.copy(alpha = if (enabled) 0.88f else 0.28f)
            if (upward) {
                drawLine(
                    color = color,
                    start = Offset(centerX, bottom),
                    end = Offset(centerX, top + head),
                    strokeWidth = stroke,
                    cap = StrokeCap.Round,
                )
                drawLine(
                    color = color,
                    start = Offset(centerX, top),
                    end = Offset(centerX - head, top + head),
                    strokeWidth = stroke,
                    cap = StrokeCap.Round,
                )
                drawLine(
                    color = color,
                    start = Offset(centerX, top),
                    end = Offset(centerX + head, top + head),
                    strokeWidth = stroke,
                    cap = StrokeCap.Round,
                )
            } else {
                drawLine(
                    color = color,
                    start = Offset(centerX, top),
                    end = Offset(centerX, bottom - head),
                    strokeWidth = stroke,
                    cap = StrokeCap.Round,
                )
                drawLine(
                    color = color,
                    start = Offset(centerX, bottom),
                    end = Offset(centerX - head, bottom - head),
                    strokeWidth = stroke,
                    cap = StrokeCap.Round,
                )
                drawLine(
                    color = color,
                    start = Offset(centerX, bottom),
                    end = Offset(centerX + head, bottom - head),
                    strokeWidth = stroke,
                    cap = StrokeCap.Round,
                )
            }
        }
    }
}

@Composable
private fun AppUpdateDownloadOverlay(
    state: AppUpdateDownloadOverlayUiState,
    typography: SettingsTypography,
    modifier: Modifier = Modifier,
) {
    val interactionSource = remember { MutableInteractionSource() }
    val progressFraction = state.progressLabel
        .removeSuffix("%")
        .toFloatOrNull()
        ?.div(100f)
        ?.coerceIn(0f, 1f)
        ?: 0f
    val transferredText = compactTransferLabel(
        when {
        state.transferredLabel.isNotBlank() -> state.transferredLabel
        state.fileSizeLabel.isNotBlank() -> "0 B / ${state.fileSizeLabel}"
        else -> "0 B / --"
        },
    )
    val progressText = when {
        state.progressLabel.isNotBlank() -> state.progressLabel
        state.statusLabel.isNotBlank() -> state.statusLabel
        else -> if (state.statusLabel.contains("完成")) "100%" else "0%"
    }
    val speedText = compactBinaryLabel(state.speedLabel ?: "--")
    val textMeasurer = rememberTextMeasurer()
    val density = LocalDensity.current
    val sideTextStyle = TextStyle(
        fontSize = 8.sp,
        fontWeight = FontWeight.Medium,
        color = Color.White,
    )
    val centerTextStyle = TextStyle(
        fontSize = 12.sp,
        fontWeight = FontWeight.SemiBold,
        color = Color.White,
        textAlign = TextAlign.Center,
    )

    BoxWithConstraints(
        modifier = modifier
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                onClick = {},
            ),
        contentAlignment = Alignment.Center,
    ) {
        val capsuleWidth = maxWidth * 0.95f
        val capsuleHeight = (maxHeight / 10f).coerceAtLeast(42.dp)
        val capsuleRadius = capsuleHeight / 2f
        val contentHorizontalPadding = 10.dp
        val contentGap = 8.dp
        val contentWidthPx = with(density) { (capsuleWidth - contentHorizontalPadding * 2f).toPx() }
        val gapPx = with(density) { contentGap.toPx() }
        val leftWidthPx = textMeasurer.measure(
            text = AnnotatedString(transferredText),
            style = sideTextStyle,
            softWrap = false,
        ).size.width.toFloat()
        val rightWidthPx = textMeasurer.measure(
            text = AnnotatedString(speedText),
            style = sideTextStyle,
            softWrap = false,
        ).size.width.toFloat()
        val centerWidthPx = textMeasurer.measure(
            text = AnnotatedString(progressText),
            style = centerTextStyle,
            softWrap = false,
        ).size.width.toFloat()
        val idealCenterStartPx = ((contentWidthPx - centerWidthPx) / 2f).coerceAtLeast(0f)
        val minCenterStartPx = (leftWidthPx + gapPx).coerceAtLeast(0f)
        val maxCenterStartPx = (contentWidthPx - rightWidthPx - gapPx - centerWidthPx).coerceAtLeast(0f)
        val centerStartPx = if (maxCenterStartPx >= minCenterStartPx) {
            idealCenterStartPx.coerceIn(minCenterStartPx, maxCenterStartPx)
        } else {
            maxCenterStartPx
        }
        val leftSlotWidth = with(density) { (centerStartPx - gapPx).coerceAtLeast(0f).toDp() }
        val centerSlotWidth = with(density) { centerWidthPx.toDp() }
        val rightSlotWidth = with(density) { rightWidthPx.toDp() }
        Box(
            modifier = Modifier
                .fillMaxWidth(0.95f)
                .height(capsuleHeight)
                .clip(RoundedCornerShape(capsuleRadius))
                .background(Color(0xEE171C25)),
        ) {
            if (progressFraction > 0f) {
                Box(
                    modifier = Modifier
                        .fillMaxHeight()
                        .fillMaxWidth(progressFraction)
                        .clip(RoundedCornerShape(capsuleRadius))
                        .background(Color(0xFF0A84FF)),
                )
            }

            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = contentHorizontalPadding),
            ) {
                Box(
                    modifier = Modifier
                        .align(Alignment.CenterStart)
                        .width(leftSlotWidth),
                    contentAlignment = Alignment.CenterStart,
                ) {
                    Text(
                        text = transferredText,
                        style = sideTextStyle,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                Box(
                    modifier = Modifier
                        .align(Alignment.CenterStart)
                        .offset { IntOffset(centerStartPx.roundToInt(), 0) }
                        .width(centerSlotWidth),
                    contentAlignment = Alignment.CenterStart,
                ) {
                    Text(
                        text = progressText,
                        style = centerTextStyle,
                        maxLines = 1,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                Box(
                    modifier = Modifier
                        .align(Alignment.CenterEnd)
                        .width(rightSlotWidth),
                    contentAlignment = Alignment.CenterEnd,
                ) {
                    Text(
                        text = speedText,
                        style = sideTextStyle,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
        }
    }
}

private fun compactTransferLabel(text: String): String {
    return compactBinaryLabel(text).replace("/", "/")
}

private fun compactBinaryLabel(text: String): String {
    return text
        .replace(" KB", "KB")
        .replace(" MB", "MB")
        .replace(" GB", "GB")
        .replace(" B", "B")
        .replace(" / ", "/")
}

@Composable
private fun SettingsTitle(
    title: String,
    typography: SettingsTypography,
) {
    Text(
        text = title,
        color = StatusUiPalette.TextPrimary,
        fontSize = typography.pageTitle,
        fontWeight = FontWeight.SemiBold,
        textAlign = TextAlign.Center,
        modifier = Modifier
            .fillMaxWidth()
            .padding(top = 0.dp, bottom = 0.dp),
    )
}

@Composable
private fun SettingsListRow(
    title: String,
    subtitle: String,
    trailingText: String? = null,
    trailingTextColor: Color = StatusUiPalette.TextSecondary,
    trailing: @Composable (() -> Unit)? = null,
    enabled: Boolean = true,
    subtitleMaxLines: Int = 2,
    typography: SettingsTypography,
    onClick: (() -> Unit)? = null,
) {
    val clickableModifier = if (enabled && onClick != null) {
        Modifier.clickable(onClick = onClick)
    } else {
        Modifier
    }
    Box(
        modifier = Modifier.fillMaxWidth(),
        contentAlignment = Alignment.Center,
    ) {
        Row(
            modifier = Modifier
                .width(typography.capsuleWidth)
                .heightIn(min = typography.capsuleMinHeight)
                .clip(RoundedCornerShape(SettingsRowCornerRadius))
                .background(Color(0xFF11131A))
                .then(clickableModifier)
                .padding(
                    horizontal = StatusCapsuleDefaults.rowHorizontalPadding,
                    vertical = StatusCapsuleDefaults.rowVerticalPadding,
                ),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(StatusCapsuleDefaults.rowContentGap),
        ) {
            Column(
                modifier = Modifier
                    .weight(1f)
                    .padding(end = StatusCapsuleDefaults.rowContentGap / 2f),
                verticalArrangement = Arrangement.Center,
            ) {
                Text(
                    text = title,
                    color = if (enabled) StatusUiPalette.TextPrimary else StatusUiPalette.TextSecondary,
                    fontSize = typography.rowTitle,
                    lineHeight = typography.rowTitleLineHeight,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    textAlign = TextAlign.Start,
                )
                if (subtitle.isNotBlank()) {
                    Text(
                        text = subtitle,
                        color = StatusUiPalette.TextSecondary,
                        fontSize = typography.rowSubtitle,
                        lineHeight = typography.rowSubtitleLineHeight,
                        maxLines = subtitleMaxLines,
                        overflow = TextOverflow.Ellipsis,
                        textAlign = TextAlign.Start,
                    )
                }
            }
            when {
                trailing != null -> trailing()
                trailingText != null -> Text(
                    text = trailingText,
                    color = trailingTextColor,
                    fontSize = typography.trailing,
                    fontWeight = FontWeight.Medium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )

                enabled && onClick != null -> Text(
                    text = "›",
                    color = StatusUiPalette.TextSecondary,
                    fontSize = typography.arrow,
                )
            }
        }
    }
}

@Composable
private fun UpdateActionRow(
    label: String,
    typography: SettingsTypography,
    onClick: () -> Unit,
) {
    Box(
        modifier = Modifier.fillMaxWidth(),
        contentAlignment = Alignment.Center,
    ) {
        Row(
            modifier = Modifier
                .width(typography.capsuleWidth)
                .heightIn(min = typography.capsuleMinHeight)
                .clip(RoundedCornerShape(SettingsActionRowCornerRadius))
                .background(Color(0xFF202532))
                .clickable(onClick = onClick)
                .padding(
                    horizontal = StatusCapsuleDefaults.rowHorizontalPadding,
                    vertical = StatusCapsuleDefaults.rowVerticalPadding,
                ),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                text = label,
                color = StatusUiPalette.TextPrimary,
                fontSize = typography.actionLabel,
                fontWeight = FontWeight.Medium,
            )
            Text(
                text = "›",
                color = StatusUiPalette.TextSecondary,
                fontSize = typography.actionArrow,
            )
        }
    }
}

@Composable
private fun DiagnosticPromptOverlay(
    state: DiagnosticPromptUiState,
    typography: SettingsTypography,
    onPrimaryAction: () -> Unit,
    onSecondaryAction: () -> Unit,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val backgroundInteractionSource = remember { MutableInteractionSource() }
    val cardInteractionSource = remember { MutableInteractionSource() }
    Box(
        modifier = modifier
            .background(Color(0xC20A0D14))
            .clickable(
                interactionSource = backgroundInteractionSource,
                indication = null,
                onClick = onDismiss,
            ),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            modifier = Modifier
                .width(typography.capsuleWidth)
                .clip(RoundedCornerShape(26.dp))
                .background(Color(0xFF171A22))
                .clickable(
                    interactionSource = cardInteractionSource,
                    indication = null,
                    onClick = {},
                )
                .padding(horizontal = 12.dp, vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text(
                text = state.title,
                color = StatusUiPalette.TextPrimary,
                fontSize = typography.panelTitle,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (state.message.isNotBlank()) {
                Text(
                    text = state.message,
                    color = StatusUiPalette.TextSecondary,
                    fontSize = typography.panelBody,
                    lineHeight = typography.panelBodyLineHeight,
                )
            }
            DiagnosticPromptButton(
                label = state.primaryLabel,
                onClick = onPrimaryAction,
            )
            state.secondaryLabel?.let { label ->
                DiagnosticPromptButton(
                    label = label,
                    onClick = onSecondaryAction,
                )
            }
        }
    }
}

@Composable
private fun DiagnosticPromptButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Box(
        modifier = modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(999.dp))
            .background(Color.White.copy(alpha = 0.10f))
            .clickable(onClick = onClick)
            .padding(horizontal = 10.dp, vertical = 9.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = label,
            color = StatusUiPalette.TextPrimary,
            fontSize = 12.sp,
            fontWeight = FontWeight.Medium,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

@Composable
private fun rememberSettingsTypography(): SettingsTypography {
    val configuration = LocalConfiguration.current
    val minScreenDp = min(configuration.screenWidthDp, configuration.screenHeightDp)
    return remember(minScreenDp) {
        if (minScreenDp >= 220) {
            val capsuleMinHeight = configuration.screenHeightDp.dp / 4f
            SettingsTypography(
                pageTitle = 16.sp,
                rowTitle = 16.sp,
                rowTitleLineHeight = (capsuleMinHeight.value / 3f).sp,
                rowSubtitle = 10.sp,
                rowSubtitleLineHeight = (capsuleMinHeight.value / 5f).sp,
                trailing = 11.sp,
                arrow = 18.sp,
                panelTitle = 13.5.sp,
                panelBody = 9.5.sp,
                panelBodyLineHeight = 11.5.sp,
                actionLabel = 16.sp,
                actionArrow = 12.5.sp,
                capsuleWidth = configuration.screenWidthDp.dp - (SettingsPageHorizontalPadding * 2),
                capsuleMinHeight = capsuleMinHeight,
            )
        } else {
            val capsuleMinHeight = configuration.screenHeightDp.dp / 4f
            SettingsTypography(
                pageTitle = 15.sp,
                rowTitle = 15.sp,
                rowTitleLineHeight = (capsuleMinHeight.value / 3f).sp,
                rowSubtitle = 9.5.sp,
                rowSubtitleLineHeight = (capsuleMinHeight.value / 5f).sp,
                trailing = 10.5.sp,
                arrow = 17.sp,
                panelTitle = 12.5.sp,
                panelBody = 9.sp,
                panelBodyLineHeight = 11.sp,
                actionLabel = 15.sp,
                actionArrow = 12.sp,
                capsuleWidth = configuration.screenWidthDp.dp - (SettingsPageHorizontalPadding * 2),
                capsuleMinHeight = capsuleMinHeight,
            )
        }
    }
}

@Composable
private fun UpdateCheckSpinner(
    animate: Boolean,
    modifier: Modifier = Modifier,
) {
    val transition = rememberInfiniteTransition(label = "update-check-spinner")
    val angle by transition.animateFloat(
        initialValue = 0f,
        targetValue = 360f,
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = 1400, easing = LinearEasing),
            repeatMode = RepeatMode.Restart,
        ),
        label = "update-check-spinner-angle",
    )
    Canvas(
        modifier = modifier.size(64.dp),
    ) {
        val ringStroke = 8.dp.toPx()
        val ringRadius = size.minDimension / 2f - ringStroke / 2f
        drawCircle(
            color = Color(0x88A7AFBA),
            radius = ringRadius,
            style = Stroke(width = ringStroke, cap = StrokeCap.Round),
        )
        val radians = Math.toRadians(if (animate) angle.toDouble() else -90.0)
        val orbitRadius = ringRadius
        val center = Offset(size.width / 2f, size.height / 2f)
        val ballCenter = Offset(
            x = center.x + (cos(radians) * orbitRadius).toFloat(),
            y = center.y + (sin(radians) * orbitRadius).toFloat(),
        )
        drawCircle(
            color = Color(0xFFD8DDE5),
            radius = 4.5.dp.toPx(),
            center = ballCenter,
        )
    }
}

private fun titleForDestination(destination: SettingsDestination): String {
    return when (destination) {
        SettingsDestination.Root -> "设置"
        SettingsDestination.About -> "关于"
        SettingsDestination.UpdateCheck -> "版本更新"
        SettingsDestination.UpdateLatest -> "版本更新"
        SettingsDestination.CurrentVersionNotes -> "当前版本说明"
        SettingsDestination.UpdateNotes -> "更新说明"
    }
}

private fun openWatcherDisplayVersion(): String {
    return BuildConfig.VERSION_NAME.substringBefore('-').ifBlank { BuildConfig.VERSION_NAME }
}
