package ai.openwatcher.watchapp.data

import java.time.Instant
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class WatcherJsonParserTest {
    private val parser = WatcherJsonParser()

    @Test
    fun parseStatus_mapsQuotaAndSessions() {
        val snapshot = parser.parseStatus(
            """
            {
              "ok": true,
              "observedAt": "2026-06-03T03:29:00Z",
              "quota": {
                "source": "oauth-api",
                "fresh": true,
                "planType": "pro",
                "fiveHour": {
                  "usedPercent": 12,
                  "remainingPercent": 88,
                  "resetAt": 1780437338
                },
                "weekly": {
                  "usedPercent": 20,
                  "remainingPercent": 80,
                  "resetAt": 1780845860
                }
              },
              "heatmap24h": {
                "timezone": "Asia/Shanghai",
                "generatedAt": "2026-06-03T03:29:00Z",
                "peakHourStart": "2026-06-03T02:00:00Z",
                "buckets": [
                  {
                    "hourStart": "2026-06-03T02:00:00Z",
                    "inputTokens": 88000,
                    "cachedInputTokens": 26000,
                    "outputTokens": 14000,
                    "reasoningOutputTokens": 6000,
                    "totalTokens": 128000,
                    "activeThreads": 4
                  }
                ]
              },
              "heatmap7d": {
                "timezone": "Asia/Shanghai",
                "generatedAt": "2026-06-03T03:29:00Z",
                "startDate": "2026-05-28",
                "endDate": "2026-06-03",
                "peakTokens": 64000,
                "days": [
                  {
                    "date": "2026-05-28",
                    "totalTokens": 90000,
                    "hours": [0, 0, 12000, 30000, 48000, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
                  },
                  {
                    "date": "2026-05-29",
                    "totalTokens": 128000,
                    "hours": [0, 0, 0, 0, 0, 64000, 32000, 16000, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 8000, 8000]
                  }
                ]
              },
              "dailyUsage": {
                "generatedAt": "2026-06-03T03:29:00Z",
                "totalTokens": 200000,
                "inputTokens": 120000,
                "cachedInputTokens": 40000,
                "outputTokens": 50000,
                "reasoningOutputTokens": 30000,
                "activeSessions": 3,
                "estimatedValueUsd": 1.2345,
                "estimatedValueLabel": "${'$'}1.23",
                "pricingDate": "2026-06-03",
                "pricingSourceUrl": "https://developers.openai.com/api/docs/pricing",
                "modelShares": [
                  {
                    "model": "gpt-5.4",
                    "tokens": 140000,
                    "sharePercent": 70
                  }
                ]
              },
              "dailyTrend30d": {
                "timezone": "Asia/Shanghai",
                "generatedAt": "2026-06-03T03:29:00Z",
                "startDate": "2026-05-04",
                "endDate": "2026-06-02",
                "totalTokens": 3000,
                "averageTokens": 15500,
                "peakTokens": 30000,
                "estimatedValueUsd": 31.46,
                "estimatedValueLabel": "${'$'}31.46",
                "days": [
                  {
                    "date": "2026-05-04",
                    "totalTokens": 1000
                  },
                  {
                    "date": "2026-05-05",
                    "totalTokens": 2000
                  }
                ]
              },
              "sessions": [
                {
                  "threadId": "019e8943-36f6-73b2-8a7a-c30c3ecc0ef2",
                  "title": "AnyChat",
                  "updatedAt": "2026-06-03T03:28:00Z",
                  "model": "gpt-5.5",
                  "reasoningEffort": "high",
                  "tokensUsedTotal": 183000,
                  "contextUsedTokens": 183000,
                  "contextWindow": 256000,
                  "contextPressurePercent": 71,
                  "contextCompactThresholdTokens": 192000,
                  "contextCompactThresholdPercent": 75,
                  "lastActiveAgoMinutes": 1
                }
              ],
              "errors": []
            }
            """.trimIndent(),
        )

        assertEquals("oauth-api", snapshot.quota?.source)
        assertTrue(snapshot.quota?.fresh == true)
        assertEquals("pro", snapshot.quota?.planType)
        assertEquals(12f, snapshot.quota?.fiveHour?.usedPercent)
        assertNotNull(snapshot.quota?.weekly?.resetAt)
        assertEquals(1, snapshot.heatmap24h?.buckets?.size)
        assertEquals(128000L, snapshot.heatmap24h?.buckets?.first()?.totalTokens)
        assertEquals(2, snapshot.heatmap7d?.days?.size)
        assertEquals(64000L, snapshot.heatmap7d?.peakTokens)
        assertEquals(24, snapshot.heatmap7d?.days?.first()?.hours?.size)
        assertEquals(200000L, snapshot.dailyUsage?.totalTokens)
        assertEquals("\$1.23", snapshot.dailyUsage?.estimatedValueLabel)
        assertEquals(3, snapshot.dailyUsage?.activeSessions)
        assertEquals("gpt-5.4", snapshot.dailyUsage?.modelShares?.first()?.model)
        assertEquals("2026-06-02", snapshot.dailyTrend30d?.endDate)
        assertEquals(3000L, snapshot.dailyTrend30d?.totalTokens)
        assertEquals(15500L, snapshot.dailyTrend30d?.averageTokens)
        assertEquals("\$31.46", snapshot.dailyTrend30d?.estimatedValueLabel)
        assertEquals(2, snapshot.dailyTrend30d?.days?.size)
        assertEquals(1, snapshot.sessions.size)
        assertEquals("AnyChat", snapshot.sessions.first().title)
        assertEquals(183000L, snapshot.sessions.first().tokensUsedTotal)
        assertEquals(71, snapshot.sessions.first().contextPressurePercent)
        assertEquals(192000L, snapshot.sessions.first().contextCompactThresholdTokens)
        assertEquals(75, snapshot.sessions.first().contextCompactThresholdPercent)
        assertTrue(snapshot.errors.isEmpty())
    }

    @Test
    fun parseHealth_recognizesHealthyResponse() {
        assertTrue(parser.parseHealth("""{"ok":true}"""))
        assertTrue(
            parser.parseHealth(
                """
                {
                  "ok": true,
                  "build": {
                    "version": "2026.06.04.1",
                    "commit": "abc1234",
                    "builtAt": "2026-06-04T10:00:00Z"
                  }
                }
                """.trimIndent(),
            ),
        )
        assertFalse(parser.parseHealth("""{"ok":false}"""))
    }

    @Test
    fun parseScreenshotUploadFilename_readsFilename() {
        val filename = parser.parseScreenshotUploadFilename(
            """{"ok":true,"filename":" watch-20260605T131500Z-xiaomi-watch-0.7.4.png "}""",
        )

        assertEquals("watch-20260605T131500Z-xiaomi-watch-0.7.4.png", filename)
    }

    @Test
    fun parseSessionAgentMessage_mapsStreamData() {
        val message = parser.parseSessionAgentMessage(
            eventId = "offset-42",
            data = """
            {
              "type": "agent_message",
              "threadId": "session-1",
              "createdAt": "2026-06-04T02:35:00Z",
              "text": " 已完成 Android 侧实现 ",
              "truncated": true
            }
            """.trimIndent(),
        )

        assertEquals("session-1", message.threadId)
        assertEquals("offset-42", message.eventId)
        assertEquals("已完成 Android 侧实现", message.text)
        assertTrue(message.truncated)
        assertNotNull(message.createdAt)
    }

    @Test
    fun parseSessionRuntimeState_mapsLifecycleAndPhase() {
        val state = parser.parseSessionRuntimeState(
            """
            {
              "type": "runtime_state",
              "threadId": "session-1",
              "turnId": "turn-1",
              "startedAt": "2026-06-04T02:35:31Z",
              "running": true,
              "lifecycle": "running",
              "phase": "tool_running",
              "updatedAt": "2026-06-04T02:36:00Z",
              "sequence": 12
            }
            """.trimIndent(),
        )

        assertEquals("session-1", state.threadId)
        assertEquals("turn-1", state.turnId)
        assertEquals(Instant.parse("2026-06-04T02:35:31Z"), state.startedAt)
        assertTrue(state.running)
        assertEquals(SessionRuntimeLifecycle.Running, state.lifecycle)
        assertEquals(SessionRuntimePhase.ToolRunning, state.phase)
        assertEquals(12L, state.sequence)
        assertNotNull(state.updatedAt)
    }

    @Test
    fun parseSessionWindowRuntimeState_mapsStartedAt() {
        val state = parser.parseSessionWindowRuntimeState(
            """
            {
              "type": "session_runtime_state",
              "runtimeState": {
                "type": "runtime_state",
                "threadId": "session-2",
                "turnId": "turn-2",
                "startedAt": "2026-06-04T03:00:00Z",
                "running": false,
                "lifecycle": "completed",
                "phase": "agent_final",
                "updatedAt": "2026-06-04T03:00:09Z",
                "sequence": 4
              }
            }
            """.trimIndent(),
        )

        assertEquals("session-2", state.threadId)
        assertEquals("turn-2", state.turnId)
        assertEquals(Instant.parse("2026-06-04T03:00:00Z"), state.startedAt)
        assertEquals(SessionRuntimeLifecycle.Completed, state.lifecycle)
        assertEquals(SessionRuntimePhase.AgentFinal, state.phase)
    }

    @Test
    fun sessionSseParser_readsEventIdAndMultilineData() {
        val sse = SessionSseParser()

        assertNull(sse.accept("event: agent_message"))
        assertNull(sse.accept("id: 99"))
        assertNull(sse.accept("data: {\"type\":\"agent_message\","))
        assertNull(sse.accept("data: \"threadId\":\"session-1\"}"))
        val event = sse.accept("")

        assertNotNull(event)
        assertEquals("agent_message", event?.eventName)
        assertEquals("99", event?.id)
        assertEquals(
            "{\"type\":\"agent_message\",\n\"threadId\":\"session-1\"}",
            event?.data,
        )
    }
}
