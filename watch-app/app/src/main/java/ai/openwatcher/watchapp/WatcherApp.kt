package ai.openwatcher.watchapp

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.input.pointer.PointerEventPass
import androidx.compose.ui.input.rotary.onRotaryScrollEvent
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.filter
import kotlinx.coroutines.withTimeoutOrNull
import ai.openwatcher.watchapp.ui.AppScreen
import ai.openwatcher.watchapp.ui.DashboardPage
import ai.openwatcher.watchapp.ui.DashboardUiState
import ai.openwatcher.watchapp.ui.Heatmap24hUiState
import ai.openwatcher.watchapp.ui.HeatmapBarUiState
import ai.openwatcher.watchapp.ui.HomeDashboardUiState
import ai.openwatcher.watchapp.ui.MiniBarUiState
import ai.openwatcher.watchapp.ui.PairingUiState
import ai.openwatcher.watchapp.ui.QuotaRingUiState
import ai.openwatcher.watchapp.ui.SettingsDestination
import ai.openwatcher.watchapp.ui.SessionDetailsUiState
import ai.openwatcher.watchapp.ui.SessionRowUiState
import ai.openwatcher.watchapp.ui.SettingsUiState
import ai.openwatcher.watchapp.ui.ScreenshotUploadUiState
import ai.openwatcher.watchapp.ui.WatcherViewModel
import ai.openwatcher.watchapp.ui.home.RoundHomeDashboardPage
import ai.openwatcher.watchapp.ui.heatmap.RoundHeatmap24hDetailScreen
import ai.openwatcher.watchapp.ui.session.SessionDetailsRoundScreen
import ai.openwatcher.watchapp.ui.session.toRoundUiModel
import ai.openwatcher.watchapp.ui.status.WatcherPairingStatusScreen
import ai.openwatcher.watchapp.ui.status.WatcherBootstrapStatusScreen
import ai.openwatcher.watchapp.ui.status.WatcherSettingsStatusScreen
import ai.openwatcher.watchapp.util.QrCodeBitmapGenerator

private val ScreenBackground = Color(0xFF05070B)
private val PanelBackground = Color(0xFF10161F)
private val PanelBackgroundAlt = Color(0xFF121C29)
private val AccentBlue = Color(0xFF35B8FF)
private val AccentGreen = Color(0xFF55F36A)
private val AccentAmber = Color(0xFFFFC542)
private val AccentTeal = Color(0xFF2DF1D3)
private val SoftText = Color(0xFFADB9CC)
private val DividerColor = Color(0xFF1E2A3C)
private val ScreenGap = 6.dp
private val ScrollTailPadding = 32.dp
private val SummaryRowHeight = 74.dp
private val DetailHeaderHeight = 80.dp
private val OverviewRingSize = 26.dp
private val FocusRingSize = 42.dp
private val OverviewRingBoxHeight = 40.dp
private val QuotaRingSize = 62.dp
private val ScreenHorizontalInset = 12.dp
private val ScreenTopInset = 10.dp
private val ScreenBottomInset = 16.dp
private val DetailPageInset = 8.dp
private val SessionMessageRotaryMaxStep = 36.dp
private val SessionMessageRotaryAxisStep = 18.dp
private const val SessionMessageRotaryScale = 0.45f
internal const val ScreenshotLongPressTimeoutMs = 1_500L

internal fun sessionMessagePixelRotaryScrollDelta(rawDelta: Float, maxStepPx: Float): Float {
    if (rawDelta == 0f || maxStepPx <= 0f) {
        return 0f
    }
    return (rawDelta * SessionMessageRotaryScale).coerceIn(-maxStepPx, maxStepPx)
}

internal fun sessionMessageAxisRotaryScrollDelta(
    rawDelta: Float,
    axisStepPx: Float,
    maxStepPx: Float,
): Float {
    if (rawDelta == 0f || axisStepPx <= 0f || maxStepPx <= 0f) {
        return 0f
    }
    return (rawDelta * axisStepPx).coerceIn(-maxStepPx, maxStepPx)
}

internal data class ContextMetricText(
    val usedText: String,
    val windowText: String,
)

@Composable
fun WatcherApp(
    viewModel: WatcherViewModel = viewModel(),
    onScreenshotRequested: (() -> Unit)? = null,
) {
    val uiState by viewModel.uiState.collectAsStateWithLifecycle()
    val useUpdateNotesEdgeRoundLayout =
        (uiState.screen == AppScreen.Dashboard &&
            uiState.dashboard.pagerPage == DashboardPage.Settings &&
            (uiState.settings.destination == SettingsDestination.UpdateNotes ||
                uiState.settings.destination == SettingsDestination.CurrentVersionNotes)) ||
            (uiState.screen == AppScreen.Settings &&
                (uiState.settings.destination == SettingsDestination.UpdateNotes ||
                    uiState.settings.destination == SettingsDestination.CurrentVersionNotes))
    val useEdgeRoundLayout = (uiState.screen == AppScreen.Dashboard &&
        uiState.dashboard.pagerPage == DashboardPage.Home) ||
        uiState.screen == AppScreen.Heatmap24h ||
        uiState.screen == AppScreen.SessionDetails ||
        useUpdateNotesEdgeRoundLayout
    val screenshotGestureModifier = onScreenshotRequested?.let { callback ->
        Modifier.highPriorityLongPress(callback)
    } ?: Modifier

    Surface(
        modifier = Modifier.fillMaxSize(),
        color = ScreenBackground,
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .then(screenshotGestureModifier)
                .padding(
                    start = if (useEdgeRoundLayout) 0.dp else ScreenHorizontalInset,
                    top = if (useEdgeRoundLayout) 0.dp else ScreenTopInset,
                    end = if (useEdgeRoundLayout) 0.dp else ScreenHorizontalInset,
                    bottom = if (useEdgeRoundLayout) 0.dp else ScreenBottomInset,
                ),
        ) {
            when (uiState.screen) {
                AppScreen.Splash -> SplashScreen()

                AppScreen.Pairing -> WatcherPairingStatusScreen(
                    state = uiState.pairing,
                    onSettings = viewModel::openSettings,
                    onRegenerate = viewModel::regenerateToken,
                )

                AppScreen.BootstrapConfirm -> WatcherBootstrapStatusScreen(
                    state = uiState.bootstrap,
                    onConfirm = viewModel::confirmBootstrapRequest,
                    onCancel = viewModel::cancelBootstrapRequest,
                )

                AppScreen.Dashboard -> DashboardScreen(
                    dashboard = uiState.dashboard,
                    settings = uiState.settings,
                    onOpenHeatmap = viewModel::openHeatmap,
                    onOpenSessionDetails = viewModel::openSessionDetails,
                    onShowQuotaEasterEgg = viewModel::showHomeQuotaEasterEgg,
                    onDashboardPageChanged = viewModel::setDashboardPage,
                    onSettingsBack = viewModel::navigateBackFromSettings,
                    onOpenSettingsDestination = viewModel::openSettingsDestination,
                    onOpenAppUpdate = viewModel::openAppUpdateFromAbout,
                    onSecretVersionTap = viewModel::registerSecretAppUpdateChannelTap,
                    onDiagnosticEntryClick = viewModel::onDiagnosticEntryClick,
                    onConfirmDiagnosticPromptPrimary = viewModel::confirmDiagnosticPromptPrimary,
                    onConfirmDiagnosticPromptSecondary = viewModel::confirmDiagnosticPromptSecondary,
                    onDismissDiagnosticPrompt = viewModel::dismissDiagnosticPrompt,
                    onToggleAutoCheckUpdate = viewModel::setAutoCheckAppUpdateEnabled,
                    onInstallUpdate = viewModel::downloadAndInstallAppUpdate,
                    onIgnoreUpdate = viewModel::ignoreAppUpdate,
                    onOpenInstallPermissionSettings = viewModel::openInstallPermissionSettings,
                    onRepair = viewModel::repair,
                )

                AppScreen.Heatmap24h -> HeatmapDetailScreen(
                    state = uiState.dashboard.heatmap24h,
                    errors = uiState.dashboard.errors,
                    onBack = viewModel::closeDetailScreen,
                    onSelectHour = viewModel::selectHeatmapHour,
                    onSelectTrendDay = viewModel::selectHeatmapTrendDay,
                    onClearTrendSelection = viewModel::clearHeatmapTrendSelection,
                    onRotateHour = viewModel::rotateHeatmapCursor,
                )

                AppScreen.SessionDetails -> SessionDetailsScreen(
                    state = uiState.dashboard.sessionDetails,
                    errors = uiState.dashboard.errors,
                    sessionMessageRotaryScrollDeltas = viewModel.sessionMessageRotaryScrollDeltas,
                    onBack = viewModel::closeDetailScreen,
                    onSelectSession = viewModel::selectSession,
                )

                AppScreen.Settings -> StandaloneSettingsScreen(
                    state = uiState.settings,
                    onBack = viewModel::navigateBackFromSettings,
                    onOpenSettingsDestination = viewModel::openSettingsDestination,
                    onOpenAppUpdate = viewModel::openAppUpdateFromAbout,
                    onSecretVersionTap = viewModel::registerSecretAppUpdateChannelTap,
                    onDiagnosticEntryClick = viewModel::onDiagnosticEntryClick,
                    onConfirmDiagnosticPromptPrimary = viewModel::confirmDiagnosticPromptPrimary,
                    onConfirmDiagnosticPromptSecondary = viewModel::confirmDiagnosticPromptSecondary,
                    onDismissDiagnosticPrompt = viewModel::dismissDiagnosticPrompt,
                    onToggleAutoCheckUpdate = viewModel::setAutoCheckAppUpdateEnabled,
                    onInstallUpdate = viewModel::downloadAndInstallAppUpdate,
                    onIgnoreUpdate = viewModel::ignoreAppUpdate,
                    onOpenInstallPermissionSettings = viewModel::openInstallPermissionSettings,
                    onRepair = viewModel::repair,
                )

                AppScreen.Offline -> WatcherPairingStatusScreen(
                    state = uiState.pairing,
                    onSettings = viewModel::openSettings,
                    onRegenerate = viewModel::regenerateToken,
                )
            }
            ScreenshotUploadOverlay(
                state = uiState.screenshotUpload,
                modifier = Modifier
                    .align(Alignment.BottomCenter)
                    .padding(bottom = 18.dp),
            )
        }
    }
}

private fun Modifier.highPriorityLongPress(onLongPress: () -> Unit): Modifier =
    pointerInput(onLongPress) {
        awaitEachGesture {
            val downEvent = awaitPointerEvent(PointerEventPass.Initial)
            val downChange = downEvent.changes.firstOrNull { it.pressed } ?: return@awaitEachGesture
            val initialPosition = downChange.position
            val touchSlopSquared = viewConfiguration.touchSlop * viewConfiguration.touchSlop

            val cancelledBeforeLongPress = withTimeoutOrNull(ScreenshotLongPressTimeoutMs) {
                while (true) {
                    val event = awaitPointerEvent(PointerEventPass.Initial)
                    val activeChange = event.changes.firstOrNull { it.pressed } ?: return@withTimeoutOrNull true
                    val dx = activeChange.position.x - initialPosition.x
                    val dy = activeChange.position.y - initialPosition.y
                    if (dx * dx + dy * dy > touchSlopSquared) {
                        return@withTimeoutOrNull true
                    }
                }
            }
            if (cancelledBeforeLongPress == true) {
                return@awaitEachGesture
            }

            onLongPress()
            do {
                val event = awaitPointerEvent(PointerEventPass.Initial)
                event.changes.forEach { it.consume() }
            } while (event.changes.any { it.pressed })
        }
    }

@Composable
private fun ScreenshotUploadOverlay(
    state: ScreenshotUploadUiState,
    modifier: Modifier = Modifier,
) {
    if (!state.visible || state.message.isBlank()) {
        return
    }
    Row(
        modifier = modifier
            .clip(RoundedCornerShape(999.dp))
            .background(Color(0xE6111722))
            .padding(horizontal = 10.dp, vertical = 5.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.Center,
    ) {
        if (state.inProgress) {
            CircularProgressIndicator(
                modifier = Modifier.size(10.dp),
                color = AccentBlue,
                strokeWidth = 1.dp,
            )
            Spacer(modifier = Modifier.width(5.dp))
        }
        Text(
            text = state.message,
            color = Color.White,
            fontSize = 10.sp,
            fontWeight = FontWeight.SemiBold,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

@Composable
private fun SplashScreen() {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF05070B)),
        contentAlignment = Alignment.Center,
    ) {
        Image(
            painter = painterResource(id = R.drawable.openwatcher_splash),
            contentDescription = null,
            modifier = Modifier.fillMaxSize(),
            contentScale = ContentScale.Fit,
        )
    }
}

@Composable
private fun PairingScreen(
    state: PairingUiState,
    onSettings: () -> Unit,
    onRegenerate: () -> Unit,
) {
    val qrSizePx = with(LocalDensity.current) { 120.dp.roundToPx() }
    val qrBitmap = remember(state.qrPayload, qrSizePx) {
        QrCodeBitmapGenerator.generate(state.qrPayload, qrSizePx)
    }

    Column(
        modifier = Modifier.fillMaxSize(),
        verticalArrangement = Arrangement.SpaceBetween,
    ) {
        Column(
            modifier = Modifier.fillMaxWidth(),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text(
                text = state.statusLabel,
                style = MaterialTheme.typography.titleSmall,
                color = Color.White,
                textAlign = TextAlign.Center,
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = state.serviceLabel,
                fontSize = 11.sp,
                color = state.serviceColor,
                textAlign = TextAlign.Center,
            )
        }

        Column(
            modifier = Modifier.fillMaxWidth(),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Box(
                modifier = Modifier
                    .size(136.dp)
                    .clip(RoundedCornerShape(20.dp))
                    .background(Color.White)
                    .padding(8.dp),
                contentAlignment = Alignment.Center,
            ) {
                Image(
                    bitmap = qrBitmap,
                    contentDescription = "配对二维码",
                    modifier = Modifier.fillMaxSize(),
                )
            }
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = "指纹 ${state.tokenFingerprint}",
                fontSize = 9.sp,
                color = Color.White,
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = "手机扫码后到电脑或手机侧确认配对",
                fontSize = 9.sp,
                color = SoftText,
                textAlign = TextAlign.Center,
                lineHeight = 11.sp,
            )
        }

        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Button(
                onClick = onRegenerate,
                modifier = Modifier.weight(1f),
                shape = RoundedCornerShape(14.dp),
            ) {
                Text("重配", fontSize = 11.sp)
            }
            TextButton(
                onClick = onSettings,
                modifier = Modifier.weight(1f),
            ) {
                Text("设置", fontSize = 11.sp)
            }
        }
    }
}

@Composable
private fun DashboardScreen(
    dashboard: DashboardUiState,
    settings: SettingsUiState,
    onOpenHeatmap: () -> Unit,
    onOpenSessionDetails: () -> Unit,
    onShowQuotaEasterEgg: () -> Unit,
    onDashboardPageChanged: (DashboardPage) -> Unit,
    onSettingsBack: () -> Unit,
    onOpenSettingsDestination: (SettingsDestination) -> Unit,
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
) {
    val pages = DashboardPage.entries
    val pagerState = rememberPagerState(
        initialPage = pages.indexOf(dashboard.pagerPage).coerceAtLeast(0),
        pageCount = { pages.size },
    )

    LaunchedEffect(dashboard.pagerPage) {
        val targetPage = pages.indexOf(dashboard.pagerPage).coerceAtLeast(0)
        if (pagerState.currentPage != targetPage) {
            pagerState.scrollToPage(targetPage)
        }
    }

    LaunchedEffect(pagerState) {
        snapshotFlow { pagerState.currentPage }
            .distinctUntilChanged()
            .filter { it in pages.indices }
            .collect { onDashboardPageChanged(pages[it]) }
    }

    Column(
        modifier = Modifier.fillMaxSize(),
        verticalArrangement = Arrangement.spacedBy(ScreenGap),
    ) {
        HorizontalPager(
            state = pagerState,
            modifier = Modifier.weight(1f),
        ) { page ->
            when (pages[page]) {
                DashboardPage.Home -> HomePage(
                    state = dashboard.home,
                    errors = dashboard.errors,
                    onOpenHeatmap = onOpenHeatmap,
                    onOpenSessionDetails = onOpenSessionDetails,
                    onShowQuotaEasterEgg = onShowQuotaEasterEgg,
                )

                DashboardPage.Settings -> Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .padding(
                            horizontal = if (
                                settings.destination == SettingsDestination.UpdateNotes ||
                                settings.destination == SettingsDestination.CurrentVersionNotes
                            ) {
                                0.dp
                            } else {
                                DetailPageInset
                            },
                        ),
                ) {
                    SettingsContent(
                        state = settings,
                        onBack = onSettingsBack,
                        onOpenDestination = onOpenSettingsDestination,
                        onOpenAppUpdate = onOpenAppUpdate,
                        onSecretVersionTap = onSecretVersionTap,
                        onDiagnosticEntryClick = onDiagnosticEntryClick,
                        onConfirmDiagnosticPromptPrimary = onConfirmDiagnosticPromptPrimary,
                        onConfirmDiagnosticPromptSecondary = onConfirmDiagnosticPromptSecondary,
                        onDismissDiagnosticPrompt = onDismissDiagnosticPrompt,
                        onToggleAutoCheckUpdate = onToggleAutoCheckUpdate,
                        onInstallUpdate = onInstallUpdate,
                        onIgnoreUpdate = onIgnoreUpdate,
                        onOpenInstallPermissionSettings = onOpenInstallPermissionSettings,
                        onRepair = onRepair,
                    )
                }
            }
        }
    }
}

@Composable
private fun HeatmapDetailScreen(
    state: Heatmap24hUiState,
    errors: List<String>,
    onBack: () -> Unit,
    onSelectHour: (Int) -> Unit,
    onSelectTrendDay: (Int) -> Unit,
    onClearTrendSelection: () -> Unit,
    onRotateHour: (Float) -> Unit,
) {
    BackHandler(onBack = onBack)

    RotaryContinuousFocusContainer(
        modifier = Modifier
            .fillMaxSize(),
        onScroll = onRotateHour,
    ) { rotaryModifier ->
        Column(
            modifier = rotaryModifier,
            verticalArrangement = Arrangement.spacedBy(ScreenGap),
        ) {
            if (errors.isNotEmpty()) {
                ErrorBanner(errors = errors)
            }
            RoundHeatmap24hDetailScreen(
                state = state,
                modifier = Modifier.fillMaxSize(),
                onSelectionChanged = { onSelectHour(it.index) },
                onTrendSelectionChanged = onSelectTrendDay,
                onTrendSelectionDismissed = onClearTrendSelection,
            )
        }
    }
}

@Composable
private fun RotaryContinuousFocusContainer(
    modifier: Modifier = Modifier,
    onScroll: (Float) -> Unit,
    content: @Composable (Modifier) -> Unit,
) {
    val focusRequester = remember { FocusRequester() }
    LaunchedEffect(Unit) {
        focusRequester.requestFocus()
    }
    val rotaryModifier = modifier
        .focusRequester(focusRequester)
        .focusable()
        .onRotaryScrollEvent { event ->
            if (event.verticalScrollPixels == 0f) {
                return@onRotaryScrollEvent false
            }
            onScroll(event.verticalScrollPixels)
            true
        }
    content(rotaryModifier)
}

@Composable
private fun SessionDetailsScreen(
    state: SessionDetailsUiState,
    errors: List<String>,
    sessionMessageRotaryScrollDeltas: SharedFlow<Float>,
    onBack: () -> Unit,
    onSelectSession: (Int) -> Unit,
) {
    BackHandler(onBack = onBack)
    val selectedSessionId = state.rows.firstOrNull { it.isSelected }?.sessionId
        ?: state.selectedIndex.toString()
    val messageScrollState = rememberScrollState()
    val maxRotaryStepPx = with(LocalDensity.current) { SessionMessageRotaryMaxStep.toPx() }
    val axisRotaryStepPx = with(LocalDensity.current) { SessionMessageRotaryAxisStep.toPx() }
    LaunchedEffect(selectedSessionId) {
        messageScrollState.scrollTo(0)
    }
    LaunchedEffect(sessionMessageRotaryScrollDeltas, axisRotaryStepPx, maxRotaryStepPx) {
        sessionMessageRotaryScrollDeltas.collect { rawDelta ->
            messageScrollState.dispatchRawDelta(
                sessionMessageAxisRotaryScrollDelta(
                    rawDelta = rawDelta,
                    axisStepPx = axisRotaryStepPx,
                    maxStepPx = maxRotaryStepPx,
                ),
            )
        }
    }

    RotaryContinuousFocusContainer(
        modifier = Modifier.fillMaxSize(),
        onScroll = { rawDelta ->
            messageScrollState.dispatchRawDelta(
                sessionMessagePixelRotaryScrollDelta(rawDelta, maxRotaryStepPx),
            )
        },
    ) { rotaryModifier ->
        Box(modifier = rotaryModifier) {
            SessionDetailsRoundScreen(
                state = state.toRoundUiModel(),
                messageScrollState = messageScrollState,
                modifier = Modifier.fillMaxSize(),
                onSelectionChanged = { onSelectSession(it.index) },
            )
            if (errors.isNotEmpty()) {
                Box(
                    modifier = Modifier
                        .align(Alignment.TopCenter)
                        .padding(top = 4.dp),
                ) {
                    ErrorBanner(errors = errors)
                }
            }
        }
    }
}

@Composable
private fun StandaloneSettingsScreen(
    state: SettingsUiState,
    onBack: () -> Unit,
    onOpenSettingsDestination: (SettingsDestination) -> Unit,
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
) {
    Column(
        modifier = Modifier.fillMaxSize(),
        verticalArrangement = Arrangement.spacedBy(ScreenGap),
    ) {
        SettingsContent(
            state = state,
            onBack = onBack,
            onOpenDestination = onOpenSettingsDestination,
            onOpenAppUpdate = onOpenAppUpdate,
            onSecretVersionTap = onSecretVersionTap,
            onDiagnosticEntryClick = onDiagnosticEntryClick,
            onConfirmDiagnosticPromptPrimary = onConfirmDiagnosticPromptPrimary,
            onConfirmDiagnosticPromptSecondary = onConfirmDiagnosticPromptSecondary,
            onDismissDiagnosticPrompt = onDismissDiagnosticPrompt,
            onToggleAutoCheckUpdate = onToggleAutoCheckUpdate,
            onInstallUpdate = onInstallUpdate,
            onIgnoreUpdate = onIgnoreUpdate,
            onOpenInstallPermissionSettings = onOpenInstallPermissionSettings,
            onRepair = onRepair,
        )
    }
}

@Composable
private fun HomePage(
    state: HomeDashboardUiState,
    errors: List<String>,
    onOpenHeatmap: () -> Unit,
    onOpenSessionDetails: () -> Unit,
    onShowQuotaEasterEgg: () -> Unit,
) {
    RoundHomeDashboardPage(
        homeState = state,
        errors = errors,
        onOpenHeatmap = onOpenHeatmap,
        onOpenSessionDetails = onOpenSessionDetails,
        onShowQuotaEasterEgg = onShowQuotaEasterEgg,
    )
}

@Composable
private fun QuotaRingCard(
    modifier: Modifier,
    state: QuotaRingUiState,
    accent: Color,
) {
    Card(
        modifier = modifier,
        shape = RoundedCornerShape(26.dp),
        colors = CardDefaults.cardColors(containerColor = PanelBackground),
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .padding(8.dp),
            contentAlignment = Alignment.Center,
        ) {
            ProgressRing(
                modifier = Modifier.size(QuotaRingSize),
                progressPercent = state.usedPercent,
                accent = accent,
                track = DividerColor,
            )
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Text(
                    text = state.title,
                    color = SoftText,
                    fontSize = 7.sp,
                )
                Text(
                    text = "${state.usedPercent.toInt()}%",
                    color = Color.White,
                    fontSize = 20.sp,
                    fontWeight = FontWeight.Bold,
                )
                Text(
                    text = state.remainingLabel,
                    color = SoftText,
                    fontSize = 7.sp,
                    textAlign = TextAlign.Center,
                    lineHeight = 9.sp,
                    maxLines = 2,
                )
            }
        }
    }
}

@Composable
private fun MiniHeatmapOverviewCard(
    modifier: Modifier,
    bars: List<MiniBarUiState>,
) {
    Card(
        modifier = modifier,
        shape = RoundedCornerShape(24.dp),
        colors = CardDefaults.cardColors(containerColor = PanelBackgroundAlt),
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = 9.dp, vertical = 7.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.End,
            ) {
                Text(
                    text = "›",
                    color = SoftText,
                    fontSize = 14.sp,
                    lineHeight = 14.sp,
                )
            }
            HeatmapBars(
                bars = bars.mapIndexed { index, item ->
                    HeatmapBarUiState(
                        hourLabel = item.hourLabel,
                        intensity = item.intensity,
                        isPeak = index == bars.indexOfFirst { it.intensity == bars.maxOfOrNull(MiniBarUiState::intensity) },
                    )
                },
                showAxis = false,
                modifier = Modifier
                    .fillMaxWidth()
                    .height(36.dp),
            )
            Spacer(modifier = Modifier.weight(1f))
            HeatmapAxisRow(
                labels = labelsAt(
                    labels = bars.map(MiniBarUiState::hourLabel),
                    0,
                    12,
                    bars.lastIndex,
                ),
            )
        }
    }
}

@Composable
private fun CurrentSessionOverviewCard(
    modifier: Modifier,
    model: String,
    reasoning: String,
    contextLabel: String,
    pressurePercent: Float,
    sessionAvailable: Boolean,
) {
    val metricText = buildContextMetricText(contextLabel)

    Card(
        modifier = modifier,
        shape = RoundedCornerShape(24.dp),
        colors = CardDefaults.cardColors(containerColor = PanelBackgroundAlt),
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = 8.dp, vertical = 7.dp),
            verticalArrangement = Arrangement.SpaceBetween,
        ) {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(OverviewRingBoxHeight + 24.dp),
                contentAlignment = Alignment.Center,
            ) {
                ProgressRing(
                    modifier = Modifier.size(OverviewRingSize + 16.dp),
                    progressPercent = pressurePercent,
                    accent = AccentTeal,
                    track = DividerColor,
                )
                Column(
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(1.dp),
                ) {
                    Column(
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(1.dp),
                    ) {
                        Text(
                            text = metricText.usedText,
                            color = Color.White,
                            fontSize = 13.sp,
                            fontWeight = FontWeight.Bold,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                        Text(
                            text = metricText.windowText,
                            color = SoftText,
                            fontSize = 7.sp,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
            }
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(start = 2.dp, top = 2.dp, end = 8.dp, bottom = 4.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = if (sessionAvailable) model else "暂无会话",
                    color = Color.White,
                    fontSize = 7.sp,
                    fontWeight = FontWeight.Medium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                if (sessionAvailable) {
                    ReasoningBadge(text = reasoning)
                }
            }
        }
    }
}

@Composable
private fun SettingsContent(
    state: SettingsUiState,
    onBack: () -> Unit,
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
) {
    BackHandler(onBack = onBack)

    WatcherSettingsStatusScreen(
        state = state,
        onOpenDestination = onOpenDestination,
        onOpenAppUpdate = onOpenAppUpdate,
        onSecretVersionTap = onSecretVersionTap,
        onDiagnosticEntryClick = onDiagnosticEntryClick,
        onConfirmDiagnosticPromptPrimary = onConfirmDiagnosticPromptPrimary,
        onConfirmDiagnosticPromptSecondary = onConfirmDiagnosticPromptSecondary,
        onDismissDiagnosticPrompt = onDismissDiagnosticPrompt,
        onToggleAutoCheckUpdate = onToggleAutoCheckUpdate,
        onInstallUpdate = onInstallUpdate,
        onIgnoreUpdate = onIgnoreUpdate,
        onOpenInstallPermissionSettings = onOpenInstallPermissionSettings,
        onRepair = onRepair,
    )
}

@Composable
private fun ActionTile(
    label: String,
    onClick: () -> Unit,
    accent: Color = Color.White,
    compact: Boolean = false,
) {
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(if (compact) 16.dp else 20.dp),
        colors = CardDefaults.cardColors(containerColor = PanelBackgroundAlt),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 11.dp, vertical = if (compact) 7.dp else 10.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                text = label,
                color = accent,
                fontSize = if (compact) 9.sp else 10.sp,
                fontWeight = FontWeight.Medium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                text = "›",
                color = SoftText,
                fontSize = 12.sp,
            )
        }
    }
}

@Composable
private fun HeatmapBars(
    bars: List<HeatmapBarUiState>,
    showAxis: Boolean,
    modifier: Modifier = Modifier,
) {
    val chartHeight = if (showAxis) 44.dp else 30.dp
    Row(
        modifier = modifier,
        horizontalArrangement = Arrangement.spacedBy(3.dp),
        verticalAlignment = Alignment.Bottom,
    ) {
        bars.forEachIndexed { index, bar ->
            Column(
                modifier = Modifier.weight(1f),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Box(
                    modifier = Modifier.height(chartHeight),
                    contentAlignment = Alignment.BottomCenter,
                ) {
                    if (bar.intensity > 0f) {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth(0.72f)
                                .height((chartHeight * bar.intensity.coerceIn(0.08f, 1f)).coerceAtLeast(4.dp))
                                .clip(RoundedCornerShape(topStart = 6.dp, topEnd = 6.dp))
                                .background(barColor(index, bars.size, bar.intensity, bar.isPeak)),
                        )
                    }
                }
                if (showAxis && (index == 0 || index == 6 || index == 12 || index == 18 || index == bars.lastIndex)) {
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = bar.hourLabel,
                        color = SoftText,
                        fontSize = 7.sp,
                    )
                } else if (showAxis) {
                    Spacer(modifier = Modifier.height(14.dp))
                }
            }
        }
    }
}

@Composable
private fun HeatmapAxisRow(labels: List<String>) {
    Row(
        modifier = Modifier.fillMaxWidth(),
    ) {
        labels.forEach { label ->
            Text(
                text = label,
                modifier = Modifier.weight(1f),
                fontSize = 7.sp,
                color = SoftText,
                textAlign = TextAlign.Center,
                maxLines = 1,
            )
        }
    }
}

@Composable
private fun LegendStat(
    modifier: Modifier = Modifier,
    color: Color,
    label: String,
    value: String,
) {
    Column(
        modifier = modifier,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Box(
                modifier = Modifier
                    .size(8.dp)
                    .clip(CircleShape)
                    .background(color),
            )
            Text(
                text = label,
                color = SoftText,
                fontSize = 8.sp,
            )
        }
        Text(
            text = value,
            color = Color.White,
            fontSize = 10.sp,
            fontWeight = FontWeight.Medium,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

@Composable
private fun SessionFocusCard(
    modifier: Modifier,
    contextLabel: String,
    pressurePercent: Float,
) {
    val metricText = buildContextMetricText(contextLabel)

    Card(
        modifier = modifier,
        shape = RoundedCornerShape(24.dp),
        colors = CardDefaults.cardColors(containerColor = PanelBackground),
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = 8.dp),
            contentAlignment = Alignment.CenterStart,
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                ProgressRing(
                    modifier = Modifier.size(FocusRingSize),
                    progressPercent = pressurePercent,
                    accent = AccentTeal,
                    track = DividerColor,
                )
                Column(
                    modifier = Modifier.weight(1f),
                    verticalArrangement = Arrangement.spacedBy(2.dp),
                ) {
                    Text(
                        text = metricText.usedText,
                        color = Color.White,
                        fontSize = 16.sp,
                        fontWeight = FontWeight.Bold,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    Text(
                        text = metricText.windowText,
                        color = SoftText,
                        fontSize = 9.sp,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
        }
    }
}

@Composable
private fun InfoPill(
    title: String,
    value: String,
    accent: Color,
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(containerColor = PanelBackgroundAlt),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 8.dp, vertical = 5.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                text = title,
                color = SoftText,
                fontSize = 6.sp,
            )
            Text(
                text = value,
                color = accent,
                fontSize = 8.sp,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

@Composable
private fun SessionListCard(
    modifier: Modifier,
    rows: List<SessionRowUiState>,
    emptyMessage: String,
) {
    Card(
        modifier = modifier,
        shape = RoundedCornerShape(24.dp),
        colors = CardDefaults.cardColors(containerColor = PanelBackground),
    ) {
        if (rows.isEmpty()) {
            Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    text = emptyMessage,
                    color = SoftText,
                    fontSize = 10.sp,
                )
            }
        } else {
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = 8.dp, vertical = 7.dp)
                    .verticalScroll(rememberScrollState()),
                verticalArrangement = Arrangement.spacedBy(5.dp),
            ) {
                rows.forEach { row ->
                    SessionRowCard(row = row)
                }
                Spacer(modifier = Modifier.height(ScrollTailPadding))
            }
        }
    }
}

@Composable
private fun SessionRowCard(row: SessionRowUiState) {
    val background = if (row.isSelected) AccentTeal.copy(alpha = 0.14f) else PanelBackgroundAlt
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(containerColor = background),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 9.dp, vertical = 5.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(
                modifier = Modifier.weight(1f),
                verticalArrangement = Arrangement.spacedBy(2.dp),
            ) {
                Text(
                    text = row.title,
                    color = Color.White,
                    fontSize = 9.sp,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    text = row.tokensLabel,
                    color = SoftText,
                    fontSize = 7.sp,
                )
            }
            Column(horizontalAlignment = Alignment.End) {
                Row(
                    horizontalArrangement = Arrangement.spacedBy(4.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        text = row.model,
                        color = SoftText,
                        fontSize = 7.sp,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    ReasoningBadge(text = row.reasoningLabel)
                }
                Spacer(modifier = Modifier.height(3.dp))
                Text(
                    text = row.lastActiveLabel,
                    color = SoftText,
                    fontSize = 6.sp,
                )
            }
        }
    }
}

@Composable
private fun ReasoningBadge(text: String) {
    Box(
        modifier = Modifier
            .clip(RoundedCornerShape(9.dp))
            .background(Color(0xFF2D1E0B))
            .padding(horizontal = 4.dp, vertical = 2.dp),
    ) {
        Text(
            text = text,
            color = AccentAmber,
            fontSize = 6.sp,
            fontWeight = FontWeight.SemiBold,
        )
    }
}

@Composable
private fun ErrorBanner(errors: List<String>) {
    StatusPanel(
        modifier = Modifier.fillMaxWidth(),
        title = "",
        containerColor = Color(0x332C1B00),
    ) {
        errors.take(2).forEach { error ->
            Text(
                text = "• $error",
                color = AccentAmber,
                fontSize = 8.sp,
                lineHeight = 10.sp,
            )
        }
    }
}

@Composable
private fun ProgressRing(
    modifier: Modifier = Modifier,
    progressPercent: Float,
    accent: Color,
    track: Color,
) {
    Canvas(
        modifier = modifier.aspectRatio(1f),
    ) {
        val strokeWidth = size.minDimension * 0.085f
        drawArc(
            color = track,
            startAngle = -210f,
            sweepAngle = 240f,
            useCenter = false,
            style = Stroke(width = strokeWidth, cap = StrokeCap.Round),
        )
        drawArc(
            brush = Brush.linearGradient(listOf(accent.copy(alpha = 0.85f), accent)),
            startAngle = -210f,
            sweepAngle = 240f * (progressPercent.coerceIn(0f, 100f) / 100f),
            useCenter = false,
            style = Stroke(width = strokeWidth, cap = StrokeCap.Round),
        )
    }
}

@Composable
private fun StatusPanel(
    modifier: Modifier = Modifier,
    title: String,
    containerColor: Color = PanelBackground,
    content: @Composable () -> Unit,
) {
    Card(
        modifier = modifier,
        shape = RoundedCornerShape(22.dp),
        colors = CardDefaults.cardColors(containerColor = containerColor),
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = 9.dp, vertical = 7.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            if (title.isNotBlank()) {
                Text(
                    text = title,
                    color = SoftText,
                    fontSize = 8.sp,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            content()
        }
    }
}

private fun barColor(
    index: Int,
    size: Int,
    intensity: Float,
    isPeak: Boolean,
): Color {
    if (isPeak) {
        return AccentAmber.copy(alpha = 0.75f + (intensity * 0.25f))
    }
    val progress = if (size <= 1) 0f else index.toFloat() / (size - 1).toFloat()
    val base = when {
        progress < 0.45f -> AccentGreen
        progress < 0.8f -> AccentBlue
        else -> AccentTeal
    }
    return base.copy(alpha = 0.35f + (intensity.coerceIn(0f, 1f) * 0.65f))
}

private fun labelsAt(labels: List<String>, vararg indices: Int): List<String> {
    return indices.map { index -> labels.getOrNull(index) ?: "--" }
}

internal fun buildContextMetricText(contextLabel: String): ContextMetricText {
    val usedText = contextLabel.substringBefore("/")
    val windowText = contextLabel.substringAfter("/", "--").uppercase()
    return ContextMetricText(
        usedText = usedText,
        windowText = windowText,
    )
}
