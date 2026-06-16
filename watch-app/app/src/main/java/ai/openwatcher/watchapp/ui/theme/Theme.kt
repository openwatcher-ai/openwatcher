package ai.openwatcher.watchapp.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

private val DarkScheme = darkColorScheme(
    primary = Color(0xFF58A6FF),
    secondary = Color(0xFF65D46E),
    tertiary = Color(0xFFFFC857),
    background = Color(0xFF05070B),
    surface = Color(0xFF121722),
    onPrimary = Color.Black,
    onSecondary = Color.Black,
    onTertiary = Color.Black,
    onBackground = Color.White,
    onSurface = Color.White,
)

@Composable
fun OpenWatcherTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = DarkScheme,
        content = content,
    )
}

