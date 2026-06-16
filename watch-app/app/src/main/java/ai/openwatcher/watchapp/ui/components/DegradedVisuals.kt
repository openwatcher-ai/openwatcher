package ai.openwatcher.watchapp.ui.components

import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.ColorMatrix
import androidx.compose.ui.graphics.Paint
import androidx.compose.ui.graphics.drawscope.drawIntoCanvas

internal fun Modifier.degradedVisuals(enabled: Boolean): Modifier {
    if (!enabled) {
        return this
    }
    return drawWithContent {
        val matrix = ColorMatrix().apply { setToSaturation(0f) }
        val paint = Paint().apply {
            colorFilter = ColorFilter.colorMatrix(matrix)
        }
        drawIntoCanvas { canvas ->
            canvas.saveLayer(Rect(0f, 0f, size.width, size.height), paint)
            drawContent()
            canvas.restore()
        }
    }
}
