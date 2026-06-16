package ai.openwatcher.watchapp.ui.session

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class AgentMessageArcLayoutEngineTest {
    @Test
    fun layout_bottomRowsBecomeNarrower() {
        val result = layout(
            text = "这是一段用于验证圆弧排版宽度变化的长消息。".repeat(12),
            scrollOffsetPx = 0f,
        )

        assertTrue(result.visibleRows.first().widthPx > result.visibleRows.last().widthPx)
    }

    @Test
    fun layout_shortMessageStartsFromMiddleRows() {
        val result = layout(
            text = "短消息",
            scrollOffsetPx = 0f,
        )

        assertEquals(1, result.totalLineCount)
        assertEquals(40f, result.visibleRows.single().topPx, 0.01f)
    }

    @Test
    fun layout_longMessageProducesScrollableDocument() {
        val result = layout(
            text = "滚动验证文本".repeat(40),
            scrollOffsetPx = 0f,
        )

        assertTrue(result.totalLineCount > result.visibleLineCount)
        assertTrue(result.maxScrollOffsetPx > 0f)
    }

    @Test
    fun layout_reflowAcrossScrollOffsetsDoesNotLoseCharacters() {
        val source = "滚动中的文字需要在不同偏移下持续重排，但不能丢字也不能重字。".repeat(18)

        listOf(0f, 9f, 17f, 28f).forEach { offset ->
            val result = layout(
                text = source,
                scrollOffsetPx = offset,
            )
            assertEquals(source, result.rows.joinToString(separator = "") { it.text })
        }
    }

    @Test
    fun layout_widerTopRowPullsTextForwardAfterScroll() {
        val source = "顶部变宽后，这一行应该能容纳更多字符，下一行的内容要自动往前补。".repeat(10)
        val initial = layout(
            text = source,
            scrollOffsetPx = 0f,
        )
        val scrolled = layout(
            text = source,
            scrollOffsetPx = 4f,
        )

        assertTrue(scrolled.visibleRows.first().widthPx > initial.visibleRows.first().widthPx)
        assertTrue(scrolled.visibleRows.first().endIndex >= initial.visibleRows.first().endIndex)
        assertTrue(scrolled.visibleRows[1].startIndex >= initial.visibleRows[1].startIndex)
    }

    @Test
    fun layout_preservesExplicitNewlineBreaks() {
        val result = layout(
            text = "第一行\n第二行",
            scrollOffsetPx = 0f,
        )

        assertEquals("第一行", result.rows[0].text)
        assertEquals("第二行", result.rows[1].text)
    }

    @Test
    fun layout_mixedTextCanBreakLongLatinWordWithoutCrashing() {
        val result = layout(
            text = "前缀 Supercalifragilisticexpialidocious 后缀",
            scrollOffsetPx = 0f,
        )

        assertFalse(result.rows.any { it.text.isEmpty() && it.startIndex == it.endIndex })
    }

    private fun layout(
        text: String,
        scrollOffsetPx: Float,
    ): ArcLayoutResult {
        return AgentMessageArcLayoutEngine.layout(
            input = ArcLayoutInput(
                text = text,
                viewportHeightPx = 100f,
                lineHeightPx = 20f,
                centerX = 100f,
                centerY = 16f,
                innerArcRadius = 108f,
                horizontalInsetPx = 8f,
                scrollOffsetPx = scrollOffsetPx,
            ),
            widthMeasurer = ArcTextWidthMeasurer(::measureWidth),
        )
    }

    private fun measureWidth(text: String): Float {
        return text.sumOf { char ->
            when {
                char == ' ' -> 6.0
                char == '\t' -> 8.0
                char.code < 128 -> 9.0
                else -> 14.0
            }
        }.toFloat()
    }
}
