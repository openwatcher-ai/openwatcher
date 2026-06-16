package ai.openwatcher.watchapp.data.diagnostics

import java.io.File
import java.time.Instant
import java.time.LocalDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

class DiagnosticEventStore(
    private val directory: File,
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
    private val clock: () -> Instant = { Instant.now() },
    private val zoneId: ZoneId = ZoneId.systemDefault(),
    private val json: Json = Json { ignoreUnknownKeys = true; explicitNulls = false },
) {
    suspend fun append(timestamp: Instant, jsonLine: String) = withContext(ioDispatcher) {
        if (!directory.exists() && !directory.mkdirs()) {
            return@withContext
        }
        val file = File(directory, buildFileName(timestamp))
        file.appendText(jsonLine + "\n", Charsets.UTF_8)
        cleanupExpiredFiles(clock())
    }

    suspend fun readRecentLines(hours: Int, now: Instant = clock()): List<String> = withContext(ioDispatcher) {
        val safeHours = hours.coerceAtLeast(1)
        val cutoff = now.minusSeconds(safeHours * 60L * 60L)
        eventFiles()
            .filter { file ->
                val start = parseHourStart(file.name) ?: return@filter false
                start.plusSeconds(60L * 60L) > cutoff
            }
            .sortedBy { parseHourStart(it.name) ?: Instant.EPOCH }
            .flatMap { file ->
                file.readLines().filter { line -> lineIsAtOrAfter(line, cutoff) }
            }
    }

    suspend fun cleanupExpiredFiles(now: Instant = clock()) = withContext(ioDispatcher) {
        if (!directory.isDirectory) {
            return@withContext
        }
        val cutoff = now.minusSeconds(24L * 60L * 60L)
        eventFiles().forEach { file ->
            val start = parseHourStart(file.name) ?: return@forEach
            if (start.plusSeconds(60L * 60L) <= cutoff) {
                file.delete()
            }
        }
    }

    private fun eventFiles(): List<File> {
        return directory
            .listFiles { file -> file.isFile && file.extension == "jsonl" }
            .orEmpty()
            .toList()
    }

    private fun buildFileName(timestamp: Instant): String {
        return FILE_NAME_FORMATTER.format(timestamp.atZone(zoneId)) + ".jsonl"
    }

    private fun parseHourStart(fileName: String): Instant? {
        val raw = fileName.removeSuffix(".jsonl")
        return runCatching {
            LocalDateTime.parse(raw, FILE_NAME_FORMATTER).atZone(zoneId).toInstant()
        }.getOrNull()
    }

    private fun lineIsAtOrAfter(line: String, cutoff: Instant): Boolean {
        val eventTime = runCatching {
            json.parseToJsonElement(line)
                .jsonObject["ts"]
                ?.jsonPrimitive
                ?.content
                ?.let(Instant::parse)
        }.getOrNull()
        return eventTime == null || eventTime >= cutoff
    }

    companion object {
        private val FILE_NAME_FORMATTER = DateTimeFormatter.ofPattern("yyyy-MM-dd-HH")
    }
}
