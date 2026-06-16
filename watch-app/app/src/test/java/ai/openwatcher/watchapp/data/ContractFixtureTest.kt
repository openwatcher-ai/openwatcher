package ai.openwatcher.watchapp.data

import java.io.File
import java.time.Instant
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.boolean
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class ContractFixtureTest {
    private val parser = WatcherJsonParser()
    private val json = Json { ignoreUnknownKeys = true; explicitNulls = false }

    @Test
    fun desktopGoContractFixtures_parseWithWatchParsers() {
        assertTrue(parser.parseHealth(contractText("healthz.ok.json")))

        val status = parser.parseStatus(contractText("status.ok.json"))
        assertEquals(Instant.parse("2026-06-03T10:00:00Z"), status.observedAt)
        assertEquals("oauth-api", status.quota?.source)
        assertTrue(status.quota?.fresh == true)
        assertEquals("pro", status.quota?.planType)
        assertEquals(12f, status.quota?.fiveHour?.usedPercent)
        assertEquals(1, status.heatmap24h?.buckets?.size)
        assertEquals(150L, status.dailyUsage?.totalTokens)
        assertEquals("gpt-5.4", status.dailyUsage?.modelShares?.first()?.model)
        assertEquals("2026-06-04", status.dailyTrend30d?.endDate)
        assertEquals(1, status.sessions.size)
        assertEquals("session title", status.sessions.first().title)
        assertEquals(19, status.sessions.first().contextPressurePercent)
        assertTrue(status.errors.isEmpty())

        val unauthorized = json.parseToJsonElement(contractText("status.unauthorized.json")).jsonObject
        assertFalse(unauthorized.getValue("ok").jsonPrimitive.boolean)
        assertEquals("unauthorized", unauthorized.getValue("error").jsonPrimitive.content)

        val bootstrap = parseBootstrapRequest(contractText("bootstrap-uri.txt").trim())
        require(bootstrap is BootstrapParseResult.Success)
        assertEquals(AppUpdateChannel.Beta, bootstrap.request.channel)
        assertEquals("desktop-bootstrap", bootstrap.request.source)
        assertEquals("Contract Watch", bootstrap.request.deviceName)
        assertEquals(2, bootstrap.request.endpoints.size)
        assertEquals("lan", bootstrap.request.endpoints.first().id)
        assertEquals("http://192.168.1.12:8787", bootstrap.request.endpoints.first().url)
        assertTrue(bootstrap.request.deviceToken.startsWith("contract-token-"))

        val sseText = contractText("status.sse")
        assertTrue(sseText.contains("\ndata:   \"ok\": true"))
        assertTrue(sseText.contains("\n\n"))

        val events = parseSseEvents(sseText)
        assertEquals(
            listOf("status_snapshot", "status_quota", "status_heatmap24h", "status_sessions", "status_errors", "heartbeat"),
            events.map { it.eventName },
        )
        assertTrue(events.all { !it.id.isNullOrBlank() })

        val snapshot = parser.parseStatusStreamSnapshot(events[0].data)
        assertEquals("session title", snapshot.sessions.first().title)

        val quota = parser.parseStatusQuotaUpdate(events[1].data)
        assertEquals("pro", quota.quota?.planType)

        val heatmap = parser.parseStatusHeatmapUpdate(events[2].data)
        assertEquals(150L, heatmap.dailyUsage?.totalTokens)

        val sessions = parser.parseStatusSessionsUpdate(events[3].data)
        assertEquals("019e8943-36f6-73b2-8a7a-c30c3ecc0ef2", sessions.sessions.first().threadId)

        val errors = parser.parseStatusErrorsUpdate(events[4].data)
        assertTrue(errors.errors.isEmpty())

        val heartbeat = json.parseToJsonElement(events[5].data).jsonObject
        assertEquals("heartbeat", heartbeat.getValue("type").jsonPrimitive.content)
    }

    private fun parseSseEvents(text: String): List<SseEvent> {
        val sse = SessionSseParser()
        val events = mutableListOf<SseEvent>()
        text.lineSequence().forEach { line ->
            sse.accept(line)?.let(events::add)
        }
        sse.finish()?.let(events::add)
        return events
    }

    private fun contractText(name: String): String {
        val start = File(requireNotNull(System.getProperty("user.dir"))).absoluteFile
        val root = generateSequence(start) { it.parentFile }
            .firstOrNull { File(it, "testdata/contracts/$name").isFile }
            ?: error("未找到契约 fixture：$name，起点：$start")
        return File(root, "testdata/contracts/$name").readText()
    }
}
