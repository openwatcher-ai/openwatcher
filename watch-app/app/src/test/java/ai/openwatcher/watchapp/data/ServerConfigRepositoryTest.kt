package ai.openwatcher.watchapp.data

import java.time.Instant
import java.util.Base64
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class ServerConfigRepositoryTest {
    @Test
    fun saveAndRead_normalizesEndpointsAndActiveEntry() {
        val repository = ServerConfigRepository(
            store = FakeKeyValueStore(),
            fallbackBaseUrl = "https://default.example.com/",
            clock = { Instant.parse("2026-06-07T11:00:00Z") },
        )

        repository.save(
            endpoints = listOf(
                ServerEndpoint(
                    id = "public",
                    label = "公网",
                    url = "https://watch.example.com/",
                    priority = 1,
                ),
                ServerEndpoint(
                    id = "lan",
                    label = "局域网",
                    url = "http://192.168.1.12:8787/",
                    priority = 0,
                ),
            ),
            source = ServerConfigSource.DesktopBootstrap,
            activeEndpointId = "public",
        )
        val current = repository.current()

        assertEquals("public", current.activeEndpointId)
        assertEquals("https://watch.example.com", current.activeEndpoint().url)
        assertEquals("局域网", current.endpoints.first().label)
        assertEquals(ServerConfigSource.DesktopBootstrap, current.source)
    }

    @Test
    fun parseBootstrapRequest_acceptsEndpointList() {
        val encodedEndpoints = Base64.getUrlEncoder().withoutPadding().encodeToString(
            Json.encodeToString(
                listOf(
                    BootstrapEndpointDto(
                        id = "lan",
                        label = "局域网",
                        url = "http://192.168.1.12:8787/",
                        priority = 0,
                    ),
                    BootstrapEndpointDto(
                        id = "public",
                        label = "公网",
                        url = "https://watch.example.com",
                        priority = 1,
                    ),
                ),
            ).toByteArray(),
        )

        val result = parseBootstrapRequest(
            "openwatcher://bootstrap?endpoints=$encodedEndpoints&deviceToken=test-token-0123456789abcdef0123456789&deviceName=Xiaomi%20Watch",
        )

        require(result is BootstrapParseResult.Success)
        assertEquals(AppUpdateChannel.Beta, result.request.channel)
        assertEquals(2, result.request.endpoints.size)
        assertEquals("http://192.168.1.12:8787", result.request.endpoints.first().url)
        assertEquals("Xiaomi Watch", result.request.deviceName)
    }

    @Test
    fun parseBootstrapRequest_acceptsDevBootstrapHost() {
        val encodedEndpoints = Base64.getUrlEncoder().withoutPadding().encodeToString(
            Json.encodeToString(
                listOf(
                    BootstrapEndpointDto(
                        id = "dev-primary",
                        label = "开发环境",
                        url = "http://192.168.1.12:8787",
                        priority = 0,
                    ),
                ),
            ).toByteArray(),
        )

        val result = parseBootstrapRequest(
            "openwatcher://dev-bootstrap?endpoints=$encodedEndpoints&deviceToken=test-token-0123456789abcdef0123456789&deviceName=Dev%20Watch",
        )

        require(result is BootstrapParseResult.Success)
        assertEquals(AppUpdateChannel.Dev, result.request.channel)
        assertEquals("Dev Watch", result.request.deviceName)
    }

    @Test
    fun parseBootstrapRequest_rejectsInvalidInput() {
        val invalidScheme = parseBootstrapRequest(
            "watcher://bootstrap?endpoints=xxx&deviceToken=test-token-0123456789abcdef0123456789",
        )
        require(invalidScheme is BootstrapParseResult.Invalid)
        assertTrue(invalidScheme.message.contains("配置链接"))

        val shortToken = parseBootstrapRequest(
            "openwatcher://bootstrap?endpoints=xxx&deviceToken=short-token",
        )
        require(shortToken is BootstrapParseResult.Invalid)
        assertTrue(shortToken.message.isNotBlank())
    }

    private class FakeKeyValueStore : KeyValueStore {
        private val values = mutableMapOf<String, String>()

        override fun getString(key: String): String? = values[key]

        override fun putString(key: String, value: String) {
            values[key] = value
        }

        override fun remove(key: String) {
            values.remove(key)
        }
    }
}
