package ai.openwatcher.watchapp.data.diagnostics

import androidx.compose.runtime.Immutable
import ai.openwatcher.watchapp.ui.AppScreen
import ai.openwatcher.watchapp.ui.AppUiState
import ai.openwatcher.watchapp.ui.DashboardPage
import ai.openwatcher.watchapp.ui.SettingsDestination

class DiagnosticRuntimeContext(
    private val baseUrl: String,
) {
    @Volatile
    private var hasPaired: Boolean = false
    @Volatile
    private var currentBaseUrl: String = baseUrl

    fun updateHasPaired(value: Boolean) {
        hasPaired = value
    }

    fun updateBaseUrl(value: String) {
        currentBaseUrl = value
    }

    fun currentNetworkInfo(): DiagnosticNetworkInfo {
        return DiagnosticNetworkInfo(
            baseUrl = currentBaseUrl,
            hasPaired = hasPaired,
        )
    }
}

@Immutable
class DiagnosticUiStateHolder(
    initialState: AppUiState = AppUiState(),
) {
    @Volatile
    private var state: AppUiState = initialState

    fun current(): AppUiState = state

    fun update(next: AppUiState) {
        state = next
    }
}

internal fun diagnosticScreenName(state: AppUiState): String {
    return when (state.screen) {
        AppScreen.Splash -> "Splash"
        AppScreen.Pairing -> "Pairing"
        AppScreen.BootstrapConfirm -> "BootstrapConfirm"
        AppScreen.Dashboard -> {
            if (state.dashboard.pagerPage == DashboardPage.Settings) {
                "Settings/${settingsDestinationName(state.settings.destination)}"
            } else {
                "Dashboard"
            }
        }
        AppScreen.Heatmap24h -> "Heatmap24h"
        AppScreen.SessionDetails -> "SessionDetails"
        AppScreen.Settings -> "Settings/${settingsDestinationName(state.settings.destination)}"
        AppScreen.Offline -> "Offline"
    }
}

private fun settingsDestinationName(destination: SettingsDestination): String {
    return when (destination) {
        SettingsDestination.Root -> "Root"
        SettingsDestination.About -> "About"
        SettingsDestination.UpdateCheck -> "UpdateCheck"
        SettingsDestination.UpdateLatest -> "UpdateLatest"
        SettingsDestination.CurrentVersionNotes -> "CurrentVersionNotes"
        SettingsDestination.UpdateNotes -> "UpdateNotes"
    }
}
