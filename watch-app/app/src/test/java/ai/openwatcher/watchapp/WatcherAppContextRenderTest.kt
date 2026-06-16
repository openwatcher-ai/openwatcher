package ai.openwatcher.watchapp

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import ai.openwatcher.watchapp.ui.home.homeQuotaBubbleFontSize

class WatcherAppContextRenderTest {
    @Test
    fun screenshotLongPressTimeout_is1500Milliseconds() {
        assertEquals(1_500L, ScreenshotLongPressTimeoutMs)
    }

    @Test
    fun buildContextMetricText_renders920kWindowExplicitly() {
        val text = buildContextMetricText("35k/920k")

        assertEquals("35k", text.usedText)
        assertEquals("920K", text.windowText)
    }

    @Test
    fun homeQuotaBubbleFontSize_shrinksForLongerCopy() {
        val shortValue = homeQuotaBubbleFontSize("刚开局，先看看。").value
        val mediumValue = homeQuotaBubbleFontSize("配额掉得有点快，后面几天记得留点余地。").value
        val longValue = homeQuotaBubbleFontSize("剩下的日子，又一位天才程序员即将陨落，后半周要吃土了。").value

        assertTrue(shortValue > mediumValue)
        assertTrue(mediumValue > longValue)
    }

    @Test
    fun sessionMessagePixelRotaryScrollDelta_scalesAndCapsRawPixels() {
        assertEquals(0f, sessionMessagePixelRotaryScrollDelta(0f, 36f), 0.0001f)
        assertEquals(4.5f, sessionMessagePixelRotaryScrollDelta(10f, 36f), 0.0001f)
        assertEquals(-4.5f, sessionMessagePixelRotaryScrollDelta(-10f, 36f), 0.0001f)
        assertEquals(36f, sessionMessagePixelRotaryScrollDelta(100f, 36f), 0.0001f)
        assertEquals(-36f, sessionMessagePixelRotaryScrollDelta(-100f, 36f), 0.0001f)
    }

    @Test
    fun sessionMessageAxisRotaryScrollDelta_convertsDetentsToVisibleSteps() {
        assertEquals(0f, sessionMessageAxisRotaryScrollDelta(0f, 18f, 36f), 0.0001f)
        assertEquals(18f, sessionMessageAxisRotaryScrollDelta(1f, 18f, 36f), 0.0001f)
        assertEquals(-18f, sessionMessageAxisRotaryScrollDelta(-1f, 18f, 36f), 0.0001f)
        assertEquals(36f, sessionMessageAxisRotaryScrollDelta(3f, 18f, 36f), 0.0001f)
        assertEquals(-36f, sessionMessageAxisRotaryScrollDelta(-3f, 18f, 36f), 0.0001f)
    }
}
