package ai.openwatcher.watchapp.data

import java.io.File
import java.time.Instant
import java.util.Locale
import java.util.UUID
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

data class PendingScreenshotUpload(
    val id: String,
    val createdAt: Instant,
)

interface ScreenshotUploadQueue {
    suspend fun enqueue(pngBytes: ByteArray, createdAt: Instant): Boolean
    suspend fun pending(): List<PendingScreenshotUpload>
    suspend fun read(pending: PendingScreenshotUpload): ByteArray?
    suspend fun delete(pending: PendingScreenshotUpload)
}

object NoOpScreenshotUploadQueue : ScreenshotUploadQueue {
    override suspend fun enqueue(pngBytes: ByteArray, createdAt: Instant): Boolean = false
    override suspend fun pending(): List<PendingScreenshotUpload> = emptyList()
    override suspend fun read(pending: PendingScreenshotUpload): ByteArray? = null
    override suspend fun delete(pending: PendingScreenshotUpload) = Unit
}

class FileScreenshotUploadQueue(
    private val directory: File,
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
    private val maxPendingFiles: Int = 12,
    private val maxPendingBytes: Long = 8L * 1024L * 1024L,
) : ScreenshotUploadQueue {
    override suspend fun enqueue(pngBytes: ByteArray, createdAt: Instant): Boolean = withContext(ioDispatcher) {
        if (pngBytes.isEmpty()) {
            return@withContext false
        }
        if (!directory.exists() && !directory.mkdirs()) {
            return@withContext false
        }
        val existing = pendingFiles()
        val pendingBytes = existing.sumOf { it.length() }
        if (existing.size >= maxPendingFiles || pendingBytes + pngBytes.size > maxPendingBytes) {
            return@withContext false
        }
        val file = File(directory, buildFileName(createdAt))
        runCatching {
            file.writeBytes(pngBytes)
            true
        }.getOrElse {
            file.delete()
            false
        }
    }

    override suspend fun pending(): List<PendingScreenshotUpload> = withContext(ioDispatcher) {
        pendingFiles()
            .mapNotNull { file ->
                val createdAt = parseCreatedAt(file.name) ?: return@mapNotNull null
                PendingScreenshotUpload(id = file.name, createdAt = createdAt)
            }
            .sortedWith(compareBy<PendingScreenshotUpload> { it.createdAt }.thenBy { it.id })
    }

    override suspend fun read(pending: PendingScreenshotUpload): ByteArray? = withContext(ioDispatcher) {
        val file = fileFor(pending)
        if (!file.isFile) {
            null
        } else {
            runCatching { file.readBytes() }.getOrNull()
        }
    }

    override suspend fun delete(pending: PendingScreenshotUpload) = withContext(ioDispatcher) {
        fileFor(pending).delete()
        Unit
    }

    private fun fileFor(pending: PendingScreenshotUpload): File = File(directory, pending.id)

    private fun pendingFiles(): List<File> {
        return directory
            .listFiles { file -> file.isFile && file.extension == "png" }
            .orEmpty()
            .toList()
    }

    private fun buildFileName(createdAt: Instant): String {
        val millis = createdAt.toEpochMilli().coerceAtLeast(0L)
        return "screenshot-%013d-%s.png".format(Locale.US, millis, UUID.randomUUID().toString())
    }

    private fun parseCreatedAt(fileName: String): Instant? {
        val millis = fileName
            .removePrefix("screenshot-")
            .substringBefore("-", missingDelimiterValue = "")
            .toLongOrNull()
            ?: return null
        return Instant.ofEpochMilli(millis)
    }
}
