package ai.openwatcher.watchapp.data

import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope

data class EndpointProbeResult(
    val endpoint: ServerEndpoint,
    val reachable: Boolean,
    val latencyMs: Long?,
    val message: String,
)

data class EndpointSelection(
    val activeEndpoint: ServerEndpoint,
    val probeResults: List<EndpointProbeResult>,
) {
    val hasReachableEndpoint: Boolean
        get() = probeResults.any { it.reachable }

    fun summaryLabel(): String {
        return if (hasReachableEndpoint) {
            val latencyLabel = probeResults.firstOrNull { it.endpoint.id == activeEndpoint.id }?.latencyMs?.let { "${it}ms" } ?: "--"
            "已选择 ${activeEndpoint.label}（$latencyLabel）"
        } else {
            "当前入口都不可达，保留 ${activeEndpoint.label}"
        }
    }
}

fun interface EndpointHealthProbe {
    suspend fun check(endpoint: ServerEndpoint): HealthCheckResult
}

class WatcherApiEndpointHealthProbe(
    private val apiFactory: (String) -> WatcherApi,
) : EndpointHealthProbe {
    override suspend fun check(endpoint: ServerEndpoint): HealthCheckResult {
        return apiFactory(endpoint.url).checkHealth()
    }
}

class EndpointSelector(
    private val probe: EndpointHealthProbe,
    private val monotonicNowMs: () -> Long = { System.nanoTime() / 1_000_000L },
) {
    suspend fun selectActiveEndpoint(
        endpoints: List<ServerEndpoint>,
        preferredEndpointId: String? = null,
    ): EndpointSelection = coroutineScope {
        require(endpoints.isNotEmpty()) { "endpoints cannot be empty" }

        val probeResults = endpoints.map { endpoint ->
            async {
                val startedAt = monotonicNowMs()
                when (val result = probe.check(endpoint)) {
                    HealthCheckResult.Online -> {
                        EndpointProbeResult(
                            endpoint = endpoint,
                            reachable = true,
                            latencyMs = (monotonicNowMs() - startedAt).coerceAtLeast(0L),
                            message = "服务在线",
                        )
                    }

                    is HealthCheckResult.Offline -> {
                        EndpointProbeResult(
                            endpoint = endpoint,
                            reachable = false,
                            latencyMs = null,
                            message = result.message,
                        )
                    }
                }
            }
        }.awaitAll()

        val activeEndpoint = probeResults
            .filter { it.reachable }
            .sortedWith(compareBy<EndpointProbeResult> { it.latencyMs ?: Long.MAX_VALUE }.thenBy { it.endpoint.priority })
            .firstOrNull()
            ?.endpoint
            ?: endpoints.firstOrNull { it.id == preferredEndpointId }
            ?: endpoints.minByOrNull { it.priority }
            ?: endpoints.first()

        EndpointSelection(
            activeEndpoint = activeEndpoint,
            probeResults = probeResults.sortedBy { it.endpoint.priority },
        )
    }
}
