package ai.openwatcher.watchapp.util

import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import android.graphics.Bitmap
import com.google.zxing.BarcodeFormat
import com.google.zxing.EncodeHintType
import com.google.zxing.MultiFormatWriter
import com.google.zxing.qrcode.decoder.ErrorCorrectionLevel

object QrCodeBitmapGenerator {
    fun generate(content: String, sizePx: Int): ImageBitmap {
        val hints = mapOf(
            EncodeHintType.MARGIN to 1,
            EncodeHintType.ERROR_CORRECTION to ErrorCorrectionLevel.M,
        )
        val matrix = MultiFormatWriter().encode(
            content,
            BarcodeFormat.QR_CODE,
            sizePx,
            sizePx,
            hints,
        )
        val bitmap = Bitmap.createBitmap(sizePx, sizePx, Bitmap.Config.ARGB_8888)
        for (x in 0 until sizePx) {
            for (y in 0 until sizePx) {
                bitmap.setPixel(x, y, if (matrix[x, y]) android.graphics.Color.BLACK else android.graphics.Color.WHITE)
            }
        }
        return bitmap.asImageBitmap()
    }
}
