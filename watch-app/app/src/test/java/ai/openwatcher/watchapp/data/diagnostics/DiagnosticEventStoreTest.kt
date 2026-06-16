package ai.openwatcher.watchapp.data.diagnostics

import java.nio.file.Files
import java.time.Instant
import java.time.ZoneId
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class DiagnosticEventStoreTest {
    @Test
    fun append_reusesSameHourFileAndRotatesAcrossHour() = runTest {
        var now = Instant.parse("2026-06-06T01:10:00Z")
        val directory = Files.createTempDirectory("diagnostic-events").toFile()
        val store = DiagnosticEventStore(
            directory = directory,
            clock = { now },
            zoneId = ZoneId.of("UTC"),
        )

        store.append(now, jsonLine(now, "first"))
        store.append(now.plusSeconds(30), jsonLine(now.plusSeconds(30), "second"))
        now = Instant.parse("2026-06-06T02:00:00Z")
        store.append(now, jsonLine(now, "third"))

        assertTrue(directory.resolve("2026-06-06-01.jsonl").isFile)
        assertTrue(directory.resolve("2026-06-06-02.jsonl").isFile)
        assertEquals(2, directory.resolve("2026-06-06-01.jsonl").readLines().size)
        assertEquals(1, directory.resolve("2026-06-06-02.jsonl").readLines().size)
    }

    @Test
    fun append_cleansUpFilesOlderThan24Hours() = runTest {
        var now = Instant.parse("2026-06-05T00:10:00Z")
        val directory = Files.createTempDirectory("diagnostic-events").toFile()
        val store = DiagnosticEventStore(
            directory = directory,
            clock = { now },
            zoneId = ZoneId.of("UTC"),
        )

        store.append(now, jsonLine(now, "old"))
        assertTrue(directory.resolve("2026-06-05-00.jsonl").isFile)

        now = Instant.parse("2026-06-06T02:10:00Z")
        store.append(now, jsonLine(now, "new"))

        assertFalse(directory.resolve("2026-06-05-00.jsonl").exists())
        assertTrue(directory.resolve("2026-06-06-02.jsonl").isFile)
    }

    private fun jsonLine(ts: Instant, event: String): String {
        return """{"ts":"$ts","event":"$event"}"""
    }
}
