package ai.openwatcher.watchapp.ui.session

import kotlin.math.floor
import kotlin.math.max
import kotlin.math.min
import kotlin.math.sqrt

internal data class ArcLayoutInput(
    val text: String,
    val viewportHeightPx: Float,
    val lineHeightPx: Float,
    val centerX: Float,
    val centerY: Float,
    val innerArcRadius: Float,
    val horizontalInsetPx: Float,
    val scrollOffsetPx: Float,
)

internal data class ArcLaidOutRow(
    val lineIndex: Int,
    val startIndex: Int,
    val endIndex: Int,
    val text: String,
    val topPx: Float,
    val bottomPx: Float,
    val leftPx: Float,
    val rightPx: Float,
    val widthPx: Float,
    val isVisible: Boolean,
)

internal data class ArcLayoutResult(
    val rows: List<ArcLaidOutRow>,
    val visibleRows: List<ArcLaidOutRow>,
    val totalLineCount: Int,
    val visibleLineCount: Int,
    val documentHeightPx: Float,
    val maxScrollOffsetPx: Float,
)

internal fun interface ArcTextWidthMeasurer {
    fun measure(text: String): Float
}

internal object AgentMessageArcLayoutEngine {
    fun layout(
        input: ArcLayoutInput,
        widthMeasurer: ArcTextWidthMeasurer,
    ): ArcLayoutResult {
        val viewportHeight = input.viewportHeightPx.coerceAtLeast(input.lineHeightPx)
        val lineHeight = input.lineHeightPx.coerceAtLeast(1f)
        val visibleLineCount = max(1, floor(viewportHeight / lineHeight).toInt())
        val normalizedText = input.text
        val baseLayout = layoutRows(
            input = input.copy(viewportHeightPx = viewportHeight, lineHeightPx = lineHeight, text = normalizedText),
            widthMeasurer = widthMeasurer,
            visibleLineCount = visibleLineCount,
            centerShiftLines = 0,
        )
        val centeredLayout = if (
            input.scrollOffsetPx <= 0f &&
            baseLayout.totalLineCount < visibleLineCount &&
            baseLayout.maxScrollOffsetPx <= 0f
        ) {
            val shift = ((visibleLineCount - baseLayout.totalLineCount) / 2f).toInt()
            layoutRows(
                input = input.copy(viewportHeightPx = viewportHeight, lineHeightPx = lineHeight, text = normalizedText),
                widthMeasurer = widthMeasurer,
                visibleLineCount = visibleLineCount,
                centerShiftLines = shift,
            )
        } else {
            baseLayout
        }
        return centeredLayout
    }

    private fun layoutRows(
        input: ArcLayoutInput,
        widthMeasurer: ArcTextWidthMeasurer,
        visibleLineCount: Int,
        centerShiftLines: Int,
    ): ArcLayoutResult {
        val text = input.text
        val lineHeight = input.lineHeightPx.coerceAtLeast(1f)
        val viewportHeight = input.viewportHeightPx.coerceAtLeast(lineHeight)
        val firstVisibleLine = max(0, floor(input.scrollOffsetPx.coerceAtLeast(0f) / lineHeight).toInt())
        val pixelShift = input.scrollOffsetPx.coerceAtLeast(0f) - firstVisibleLine * lineHeight
        val rows = mutableListOf<ArcLaidOutRow>()
        var cursor = 0
        var lineIndex = 0

        while (cursor < text.length) {
            val displayLineIndex = lineIndex + centerShiftLines
            val rowGeometry = computeRowGeometry(
                lineIndex = lineIndex,
                displayLineIndex = displayLineIndex,
                firstVisibleLine = firstVisibleLine,
                pixelShift = pixelShift,
                visibleLineCount = visibleLineCount,
                viewportHeight = viewportHeight,
                lineHeight = lineHeight,
                centerX = input.centerX,
                centerY = input.centerY,
                innerArcRadius = input.innerArcRadius,
                horizontalInsetPx = input.horizontalInsetPx,
            )
            val breakResult = breakLine(
                text = text,
                startIndex = cursor,
                maxWidthPx = rowGeometry.widthPx,
                widthMeasurer = widthMeasurer,
            )
            rows += ArcLaidOutRow(
                lineIndex = lineIndex,
                startIndex = cursor,
                endIndex = breakResult.nextIndex,
                text = breakResult.lineText,
                topPx = rowGeometry.topPx,
                bottomPx = rowGeometry.bottomPx,
                leftPx = rowGeometry.leftPx,
                rightPx = rowGeometry.rightPx,
                widthPx = rowGeometry.widthPx,
                isVisible = rowGeometry.isVisible,
            )
            cursor = breakResult.nextIndex
            lineIndex += 1
        }

        val totalLineCount = rows.size.coerceAtLeast(1)
        val documentHeightPx = totalLineCount * lineHeight
        val maxScrollOffsetPx = (documentHeightPx - viewportHeight).coerceAtLeast(0f)
        val visibleRows = rows.filter { it.isVisible }

        return ArcLayoutResult(
            rows = rows,
            visibleRows = visibleRows,
            totalLineCount = totalLineCount,
            visibleLineCount = visibleLineCount,
            documentHeightPx = documentHeightPx,
            maxScrollOffsetPx = maxScrollOffsetPx,
        )
    }

    private data class RowGeometry(
        val topPx: Float,
        val bottomPx: Float,
        val leftPx: Float,
        val rightPx: Float,
        val widthPx: Float,
        val isVisible: Boolean,
    )

    private fun computeRowGeometry(
        lineIndex: Int,
        displayLineIndex: Int,
        firstVisibleLine: Int,
        pixelShift: Float,
        visibleLineCount: Int,
        viewportHeight: Float,
        lineHeight: Float,
        centerX: Float,
        centerY: Float,
        innerArcRadius: Float,
        horizontalInsetPx: Float,
    ): RowGeometry {
        val relativeLineIndex = displayLineIndex - firstVisibleLine
        val rowTop = relativeLineIndex * lineHeight - pixelShift
        val rowBottom = rowTop + lineHeight
        val effectiveBottom = when {
            displayLineIndex < firstVisibleLine -> lineHeight
            displayLineIndex > firstVisibleLine + visibleLineCount -> viewportHeight
            else -> rowBottom
        }
        val clampedBottom = effectiveBottom.coerceIn(0f, viewportHeight)
        val halfChord = sqrt(
            (innerArcRadius * innerArcRadius - (clampedBottom - centerY) * (clampedBottom - centerY))
                .coerceAtLeast(0f),
        )
        val left = centerX - halfChord + horizontalInsetPx
        val right = centerX + halfChord - horizontalInsetPx
        val width = (right - left).coerceAtLeast(horizontalInsetPx)
        return RowGeometry(
            topPx = rowTop,
            bottomPx = rowBottom,
            leftPx = left,
            rightPx = right,
            widthPx = width,
            isVisible = rowBottom > 0f && rowTop < viewportHeight,
        )
    }

    private data class BreakResult(
        val lineText: String,
        val nextIndex: Int,
    )

    private fun breakLine(
        text: String,
        startIndex: Int,
        maxWidthPx: Float,
        widthMeasurer: ArcTextWidthMeasurer,
    ): BreakResult {
        if (startIndex >= text.length) {
            return BreakResult("", text.length)
        }
        if (text[startIndex] == '\n') {
            return BreakResult("", startIndex + 1)
        }

        val hardBreakIndex = text.indexOf('\n', startIndex).let { if (it >= 0) it else text.length }
        val fullSegment = text.substring(startIndex, hardBreakIndex)
        if (fullSegment.isNotEmpty() && widthMeasurer.measure(fullSegment) <= maxWidthPx) {
            return BreakResult(
                lineText = fullSegment.trimEnd(),
                nextIndex = if (hardBreakIndex < text.length) hardBreakIndex + 1 else hardBreakIndex,
            )
        }

        var low = startIndex + 1
        var high = hardBreakIndex
        var bestFit = low
        while (low <= high) {
            val mid = (low + high) ushr 1
            val candidate = text.substring(startIndex, mid)
            if (candidate.isNotEmpty() && widthMeasurer.measure(candidate) <= maxWidthPx) {
                bestFit = mid
                low = mid + 1
            } else {
                high = mid - 1
            }
        }

        val whitespaceBreak = lastWhitespaceBefore(text, startIndex, bestFit)
        val breakEnd = when {
            whitespaceBreak > startIndex -> whitespaceBreak
            bestFit > startIndex -> bestFit
            else -> min(startIndex + 1, text.length)
        }
        val nextIndex = when {
            whitespaceBreak > startIndex -> skipLeadingWhitespace(text, whitespaceBreak + 1)
            else -> breakEnd
        }
        return BreakResult(
            lineText = text.substring(startIndex, breakEnd).trimEnd(),
            nextIndex = nextIndex.coerceAtLeast(startIndex + 1),
        )
    }

    private fun lastWhitespaceBefore(
        text: String,
        startIndex: Int,
        endIndex: Int,
    ): Int {
        for (index in endIndex - 1 downTo startIndex) {
            if (text[index].isWhitespace()) {
                return index
            }
        }
        return -1
    }

    private fun skipLeadingWhitespace(
        text: String,
        startIndex: Int,
    ): Int {
        var index = startIndex
        while (index < text.length && text[index].isWhitespace() && text[index] != '\n') {
            index += 1
        }
        return index
    }
}
