package ai.openwatcher.watchapp.data

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class EndpointSelectorTest {
    @Test
    fun selectActiveEndpoint_prefersReachableAndFasterEntry() {
        val selector = EndpointSelector(
            probe = EndpointHealthProbe { endpoint ->
                when (endpoint.id) {
                    "lan" -> HealthCheckResult.Offline("局域网不可达")
                    "public" -> {
                        kotlinx.coroutines.delay(30)
                        HealthCheckResult.Online
                    }
                    else -> {
                        kotlinx.coroutines.delay(5)
                        HealthCheckResult.Online
                    }
                }
            },
        )

        val selection = runBlockingSelection(selector)

        assertEquals("managedTunnel", selection.activeEndpoint.id)
        assertTrue(selection.hasReachableEndpoint)
    }

    @Test
    fun selectActiveEndpoint_keepsPreferredWhenAllEndpointsOffline() {
        val selector = EndpointSelector(
            probe = EndpointHealthProbe { HealthCheckResult.Offline("不可达") },
        )

        val endpoints = listOf(
            ServerEndpoint("lan", "局域网", "http://192.168.1.12:8787", 0),
            ServerEndpoint("public", "公网", "https://watch.example.com", 1),
        )
        val selection = kotlinx.coroutines.runBlocking {
            selector.selectActiveEndpoint(endpoints, preferredEndpointId = "public")
        }

        assertEquals("public", selection.activeEndpoint.id)
        assertTrue(!selection.hasReachableEndpoint)
    }

    private fun runBlockingSelection(selector: EndpointSelector): EndpointSelection {
        val endpoints = listOf(
            ServerEndpoint("lan", "局域网", "http://192.168.1.12:8787", 0),
            ServerEndpoint("public", "公网", "https://watch.example.com", 1),
            ServerEndpoint("managedTunnel", "托管隧道", "https://demo.openwatcher.ai", 2),
        )
        return kotlinx.coroutines.runBlocking {
            selector.selectActiveEndpoint(endpoints, preferredEndpointId = "lan")
        }
    }
}
