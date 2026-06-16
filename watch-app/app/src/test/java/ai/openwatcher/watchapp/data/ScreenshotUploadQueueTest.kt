package ai.openwatcher.watchapp.data

import java.nio.file.Files
import java.time.Instant
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class ScreenshotUploadQueueTest {
    @Test
    fun fileQueuePersistsPendingScreenshotsInCreatedOrder() = runTest {
        val directory = Files.createTempDirectory("pending-screenshots").toFile()
        val firstPng = byteArrayOf(0x01)
        val secondPng = byteArrayOf(0x02)
        val queue = FileScreenshotUploadQueue(directory)

        assertTrue(queue.enqueue(secondPng, Instant.parse("2026-06-06T02:00:00Z")))
        assertTrue(queue.enqueue(firstPng, Instant.parse("2026-06-06T01:00:00Z")))

        val restoredQueue = FileScreenshotUploadQueue(directory)
        val pending = restoredQueue.pending()

        assertEquals(2, pending.size)
        assertArrayEquals(firstPng, restoredQueue.read(pending[0]))
        assertArrayEquals(secondPng, restoredQueue.read(pending[1]))

        restoredQueue.delete(pending[0])
        assertEquals(1, restoredQueue.pending().size)
    }

    @Test
    fun fileQueueRejectsNewScreenshotWhenStorageLimitIsReached() = runTest {
        val directory = Files.createTempDirectory("pending-screenshots-limit").toFile()
        val queue = FileScreenshotUploadQueue(
            directory = directory,
            maxPendingFiles = 1,
            maxPendingBytes = 2,
        )

        assertTrue(queue.enqueue(byteArrayOf(0x01), Instant.parse("2026-06-06T01:00:00Z")))
        assertEquals(false, queue.enqueue(byteArrayOf(0x02), Instant.parse("2026-06-06T02:00:00Z")))
        assertEquals(1, queue.pending().size)
    }
}
