package ai.openwatcher.watchapp.data

import android.net.Uri
import java.net.URI
import java.net.URLDecoder
import java.time.Instant
import java.util.Base64
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

data class ServerEndpoint(
    val id: String,
    val label: String,
    val url: String,
    val priority: Int,
)

data class ServerConfig(
    val endpoints: List<ServerEndpoint>,
    val activeEndpointId: String?,
    val configuredAt: Instant,
    val source: ServerConfigSource,
    val deviceToken: String? = null,
    val deviceName: String? = null,
) {
    fun activeEndpoint(): ServerEndpoint {
        return endpoints.firstOrNull { it.id == activeEndpointId }
            ?: endpoints.minByOrNull { it.priority }
            ?: error("server config requires at least one endpoint")
    }

    fun endpointSummary(): String = endpoints.joinToString("、") { it.label }
}

enum class ServerConfigSource {
    Manual,
    DesktopBootstrap,
    RemoteBootstrap,
    Adb,
    Qr,
    BuildDefault,
}

data class BootstrapRequest(
    val channel: AppUpdateChannel,
    val endpoints: List<ServerEndpoint>,
    val deviceToken: String,
    val deviceName: String,
    val source: String,
)

sealed interface BootstrapParseResult {
    data class Success(val request: BootstrapRequest) : BootstrapParseResult
    data class Invalid(val message: String) : BootstrapParseResult
}

class ServerConfigRepository(
    private val store: KeyValueStore,
    private val fallbackBaseUrl: String,
    private val clock: () -> Instant = { Instant.now() },
    private val json: Json = Json { ignoreUnknownKeys = true; explicitNulls = false },
) {
    fun current(channel: AppUpdateChannel = AppUpdateChannel.Beta): ServerConfig {
        return profile(channel) ?: buildDefaultConfig()
    }

    fun currentBaseUrl(channel: AppUpdateChannel = AppUpdateChannel.Beta): String = current(channel).activeEndpoint().url

    fun currentEndpoint(channel: AppUpdateChannel = AppUpdateChannel.Beta): ServerEndpoint = current(channel).activeEndpoint()

    fun currentDeviceToken(channel: AppUpdateChannel = AppUpdateChannel.Beta): String? =
        profile(channel)?.deviceToken?.trim()?.ifBlank { null }

    fun currentDeviceName(channel: AppUpdateChannel = AppUpdateChannel.Beta): String? =
        profile(channel)?.deviceName?.trim()?.ifBlank { null }

    fun hasStoredConfig(channel: AppUpdateChannel = AppUpdateChannel.Beta): Boolean = profile(channel) != null

    fun hasAnyStoredConfig(): Boolean {
        val state = readStored() ?: return false
        return state.betaProfile != null || state.devProfile != null
    }

    fun profile(channel: AppUpdateChannel): ServerConfig? {
        val state = readStored() ?: return null
        return when (channel) {
            AppUpdateChannel.Beta -> state.betaProfile?.toDomain()
            AppUpdateChannel.Dev -> state.devProfile?.toDomain()
        }
    }

    fun save(
        channel: AppUpdateChannel = AppUpdateChannel.Beta,
        endpoints: List<ServerEndpoint>,
        source: ServerConfigSource,
        deviceToken: String? = null,
        deviceName: String? = null,
        activeEndpointId: String? = null,
        configuredAt: Instant = clock(),
    ): ServerConfig {
        val normalizedEndpoints = normalizeEndpoints(endpoints)
        val next = ServerConfig(
            endpoints = normalizedEndpoints,
            activeEndpointId = resolveActiveEndpointId(normalizedEndpoints, activeEndpointId),
            configuredAt = configuredAt,
            source = source,
            deviceToken = deviceToken?.trim()?.ifBlank { null },
            deviceName = deviceName?.trim()?.ifBlank { null },
        )
        write(channel, next)
        return next
    }

    fun updateActiveEndpoint(
        channel: AppUpdateChannel = AppUpdateChannel.Beta,
        endpointId: String?,
    ): ServerConfig {
        val existing = current(channel)
        val next = existing.copy(
            activeEndpointId = resolveActiveEndpointId(existing.endpoints, endpointId),
        )
        write(channel, next)
        return next
    }

    fun clear(channel: AppUpdateChannel? = null) {
        if (channel == null) {
            store.remove(KEY_SERVER_CONFIG)
            return
        }
        val state = readStored() ?: return
        val nextState = when (channel) {
            AppUpdateChannel.Beta -> state.copy(betaProfile = null)
            AppUpdateChannel.Dev -> state.copy(devProfile = null)
        }
        if (nextState.betaProfile == null && nextState.devProfile == null) {
            store.remove(KEY_SERVER_CONFIG)
            return
        }
        writeState(nextState)
    }

    private fun buildDefaultConfig(): ServerConfig {
        val endpoint = ServerEndpoint(
            id = "build-default",
            label = "默认地址",
            url = normalizeServerBaseUrl(fallbackBaseUrl),
            priority = 0,
        )
        return ServerConfig(
            endpoints = listOf(endpoint),
            activeEndpointId = endpoint.id,
            configuredAt = Instant.EPOCH,
            source = ServerConfigSource.BuildDefault,
        )
    }

    private fun normalizeEndpoints(endpoints: List<ServerEndpoint>): List<ServerEndpoint> {
        require(endpoints.isNotEmpty()) { "endpoints cannot be empty" }
        return endpoints.mapIndexed { index, endpoint ->
            val normalizedId = endpoint.id.trim().ifBlank { throw IllegalArgumentException("endpoint id 不能为空") }
            ServerEndpoint(
                id = normalizedId,
                label = endpoint.label.trim().ifBlank { normalizedId },
                url = normalizeServerBaseUrl(endpoint.url),
                priority = if (endpoint.priority >= 0) endpoint.priority else index,
            )
        }.sortedBy { it.priority }
    }

    private fun resolveActiveEndpointId(endpoints: List<ServerEndpoint>, endpointId: String?): String {
        val normalized = endpointId?.trim().orEmpty()
        if (normalized.isNotBlank() && endpoints.any { it.id == normalized }) {
            return normalized
        }
        return endpoints.minByOrNull { it.priority }?.id
            ?: throw IllegalArgumentException("endpoints cannot be empty")
    }

    private fun readStored(): ServerConfigStateDto? {
        val raw = store.getString(KEY_SERVER_CONFIG)?.trim().orEmpty()
        if (raw.isBlank()) {
            return null
        }
        return runCatching {
            json.decodeFromString<ServerConfigStateDto>(raw)
        }.getOrNull()
    }

    private fun write(channel: AppUpdateChannel, config: ServerConfig) {
        val currentState = readStored() ?: ServerConfigStateDto()
        val nextState = when (channel) {
            AppUpdateChannel.Beta -> currentState.copy(betaProfile = ServerConfigDto.fromDomain(config))
            AppUpdateChannel.Dev -> currentState.copy(devProfile = ServerConfigDto.fromDomain(config))
        }
        writeState(nextState)
    }

    private fun writeState(state: ServerConfigStateDto) {
        store.putString(KEY_SERVER_CONFIG, json.encodeToString(state))
    }

    companion object {
        private const val KEY_SERVER_CONFIG = "server_config"
    }
}

fun parseBootstrapRequest(rawUri: String): BootstrapParseResult {
    val uri = runCatching { URI(rawUri.trim()) }.getOrElse {
        return BootstrapParseResult.Invalid("配置链接无法解析")
    }
    return parseBootstrapRequestInternal(
        scheme = uri.scheme,
        host = uri.host,
        params = parseQueryParams(uri.rawQuery.orEmpty()),
    )
}

fun parseBootstrapRequest(uri: Uri): BootstrapParseResult {
    return parseBootstrapRequestInternal(
        scheme = uri.scheme,
        host = uri.host,
        params = uri.queryParameterNames.associateWith { key ->
            uri.getQueryParameter(key).orEmpty().trim()
        },
    )
}

private fun parseBootstrapRequestInternal(
    scheme: String?,
    host: String?,
    params: Map<String, String>,
): BootstrapParseResult {
    if (!scheme.equals("openwatcher", ignoreCase = true)) {
        return BootstrapParseResult.Invalid("仅支持 OpenWatcher 配置链接")
    }
    val channel = when {
        host.equals("bootstrap", ignoreCase = true) -> AppUpdateChannel.Beta
        host.equals("dev-bootstrap", ignoreCase = true) -> AppUpdateChannel.Dev
        else -> return BootstrapParseResult.Invalid("仅支持 OpenWatcher 配置链接")
    }

    val encodedEndpoints = params["endpoints"].orEmpty().trim()
    val token = params["deviceToken"].orEmpty().trim()
    val deviceName = params["deviceName"].orEmpty().trim().ifBlank { "Android 手表" }
    val source = params["source"].orEmpty().trim().ifBlank {
        if (channel == AppUpdateChannel.Dev) "desktop-dev" else "desktop-bootstrap"
    }

    if (encodedEndpoints.isBlank()) {
        return BootstrapParseResult.Invalid("endpoints 不能为空")
    }

    val endpoints = runCatching {
        val decoded = Base64.getUrlDecoder().decode(encodedEndpoints)
        Json { ignoreUnknownKeys = true; explicitNulls = false }
            .decodeFromString<List<BootstrapEndpointDto>>(decoded.decodeToString())
            .mapIndexed { index, dto ->
                ServerEndpoint(
                    id = dto.id.trim().ifBlank { throw IllegalArgumentException("endpoint id 不能为空") },
                    label = dto.label.trim().ifBlank { dto.id.trim() },
                    url = normalizeServerBaseUrl(dto.url),
                    priority = dto.priority ?: index,
                )
            }
            .sortedBy { it.priority }
    }.getOrElse {
        return BootstrapParseResult.Invalid(it.message ?: "endpoints 非法")
    }

    if (endpoints.isEmpty()) {
        return BootstrapParseResult.Invalid("至少需要一个入口")
    }
    if (token.length < 32) {
        return BootstrapParseResult.Invalid("deviceToken 过短")
    }

    return BootstrapParseResult.Success(
        BootstrapRequest(
            channel = channel,
            endpoints = endpoints,
            deviceToken = token,
            deviceName = deviceName,
            source = source,
        ),
    )
}

fun normalizeServerBaseUrl(rawValue: String): String {
    val trimmed = rawValue.trim().trimEnd('/')
    val uri = URI(trimmed)
    val scheme = uri.scheme?.lowercase()
    if (scheme != "http" && scheme != "https") {
        throw IllegalArgumentException("baseUrl 只支持 http 或 https")
    }
    if (uri.host.isNullOrBlank()) {
        throw IllegalArgumentException("baseUrl 缺少 host")
    }
    val normalized = uri.normalize().toString().trimEnd('/')
    if (normalized.isBlank()) {
        throw IllegalArgumentException("baseUrl 不能为空")
    }
    return normalized
}

private fun parseQueryParams(rawQuery: String): Map<String, String> {
    if (rawQuery.isBlank()) {
        return emptyMap()
    }
    return rawQuery.split("&")
        .mapNotNull { part ->
            if (part.isBlank()) {
                return@mapNotNull null
            }
            val pieces = part.split("=", limit = 2)
            val key = URLDecoder.decode(pieces[0], Charsets.UTF_8.name()).trim()
            if (key.isBlank()) {
                return@mapNotNull null
            }
            val value = if (pieces.size == 2) {
                URLDecoder.decode(pieces[1], Charsets.UTF_8.name()).trim()
            } else {
                ""
            }
            key to value
        }
        .toMap()
}

@Serializable
data class BootstrapEndpointDto(
    val id: String,
    val label: String,
    val url: String,
    val priority: Int? = null,
)

@Serializable
private data class ServerEndpointDto(
    val id: String,
    val label: String,
    val url: String,
    val priority: Int,
) {
    fun toDomain(): ServerEndpoint {
        return ServerEndpoint(
            id = id,
            label = label,
            url = url,
            priority = priority,
        )
    }

    companion object {
        fun fromDomain(endpoint: ServerEndpoint): ServerEndpointDto {
            return ServerEndpointDto(
                id = endpoint.id,
                label = endpoint.label,
                url = endpoint.url,
                priority = endpoint.priority,
            )
        }
    }
}

@Serializable
private data class ServerConfigStateDto(
    val betaProfile: ServerConfigDto? = null,
    val devProfile: ServerConfigDto? = null,
)

@Serializable
private data class ServerConfigDto(
    val endpoints: List<ServerEndpointDto>,
    val activeEndpointId: String? = null,
    val configuredAt: String,
    val source: String,
    val deviceToken: String? = null,
    val deviceName: String? = null,
) {
    fun toDomain(): ServerConfig {
        val domainEndpoints = endpoints.map { dto ->
            ServerEndpoint(
                id = dto.id.trim(),
                label = dto.label.trim().ifBlank { dto.id.trim() },
                url = normalizeServerBaseUrl(dto.url),
                priority = dto.priority,
            )
        }.sortedBy { it.priority }
        return ServerConfig(
            endpoints = domainEndpoints,
            activeEndpointId = activeEndpointId?.trim()?.takeIf { candidate -> domainEndpoints.any { it.id == candidate } }
                ?: domainEndpoints.minByOrNull { it.priority }?.id,
            configuredAt = Instant.parse(configuredAt),
            source = ServerConfigSource.valueOf(source),
            deviceToken = deviceToken?.trim()?.ifBlank { null },
            deviceName = deviceName?.trim()?.ifBlank { null },
        )
    }

    companion object {
        fun fromDomain(config: ServerConfig): ServerConfigDto {
            return ServerConfigDto(
                endpoints = config.endpoints.map(ServerEndpointDto::fromDomain),
                activeEndpointId = config.activeEndpointId,
                configuredAt = config.configuredAt.toString(),
                source = config.source.name,
                deviceToken = config.deviceToken?.trim()?.ifBlank { null },
                deviceName = config.deviceName?.trim()?.ifBlank { null },
            )
        }
    }
}
