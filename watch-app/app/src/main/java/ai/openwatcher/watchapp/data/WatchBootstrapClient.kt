package ai.openwatcher.watchapp.data

import java.io.IOException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okhttp3.Call
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.HttpUrl.Companion.toHttpUrl

data class WatchBootstrapRegistration(
    val bootstrapCode: String,
    val registeredAt: String,
)

data class WatchBootstrapConfig(
    val environment: AppUpdateChannel,
    val apiBase: String,
    val source: String,
    val configuredAt: String,
)

class WatchBootstrapException(
    val code: String,
    message: String,
) : IOException(message)

sealed interface WatchBootstrapPollResult {
    data object Pending : WatchBootstrapPollResult
    data class Ready(val config: WatchBootstrapConfig) : WatchBootstrapPollResult
}

interface WatchBootstrapGateway {
    suspend fun register(deviceName: String, appVersion: String): WatchBootstrapRegistration

    suspend fun poll(bootstrapCode: String): WatchBootstrapPollResult
}

class WatchBootstrapCodeStore(
    private val store: KeyValueStore,
) {
    fun currentCode(): String? = store.getString(KEY_BOOTSTRAP_CODE)?.trim()?.ifBlank { null }

    fun save(code: String) {
        store.putString(KEY_BOOTSTRAP_CODE, code.trim().uppercase())
    }

    fun clear() {
        store.remove(KEY_BOOTSTRAP_CODE)
    }

    companion object {
        private const val KEY_BOOTSTRAP_CODE = "watch_bootstrap_code"
    }
}

class WatchBootstrapClient(
    private val baseUrl: String,
    private val callFactory: Call.Factory,
    private val json: Json = Json { ignoreUnknownKeys = true; explicitNulls = false },
) : WatchBootstrapGateway {
    override suspend fun register(deviceName: String, appVersion: String): WatchBootstrapRegistration = withContext(Dispatchers.IO) {
        val body = json.encodeToString(
            WatchBootstrapRegisterRequestDto(
                deviceName = deviceName,
                appVersion = appVersion,
                platform = "wear-os",
            ),
        ).toRequestBody("application/json; charset=utf-8".toMediaType())
        val request = Request.Builder()
            .url(baseUrl.toHttpUrl().newBuilder().addPathSegments("v1/watch-bootstrap/register").build())
            .post(body)
            .build()
        callFactory.newCall(request).execute().use { response ->
            val raw = response.body?.string().orEmpty()
            if (!response.isSuccessful) {
                throw parseError(raw, "临时配置服务暂时不可用")
            }
            val payload = json.decodeFromString<WatchBootstrapRegisterResponseDto>(raw)
            if (!payload.ok || payload.bootstrapCode.isBlank()) {
                throw SerializationException("临时配置服务返回的数据不完整")
            }
            WatchBootstrapRegistration(
                bootstrapCode = payload.bootstrapCode.trim().uppercase(),
                registeredAt = payload.registeredAt,
            )
        }
    }

    override suspend fun poll(bootstrapCode: String): WatchBootstrapPollResult = withContext(Dispatchers.IO) {
        val request = Request.Builder()
            .url(
                baseUrl.toHttpUrl().newBuilder()
                    .addPathSegments("v1/watch-bootstrap/config")
                    .addQueryParameter("code", bootstrapCode.trim().uppercase())
                    .build(),
            )
            .get()
            .build()
        callFactory.newCall(request).execute().use { response ->
            val raw = response.body?.string().orEmpty()
            if (!response.isSuccessful) {
                throw parseError(raw, "临时配置码暂时不可用")
            }
            val payload = json.decodeFromString<WatchBootstrapPollResponseDto>(raw)
            if (!payload.ok || payload.status == WatchBootstrapStatus.Unknown) {
                throw SerializationException("临时配置服务返回的数据不完整")
            }
            if (payload.status == WatchBootstrapStatus.Pending) {
                return@withContext WatchBootstrapPollResult.Pending
            }
            val config = payload.config ?: throw SerializationException("临时配置缺少 API 基址")
            WatchBootstrapPollResult.Ready(
                WatchBootstrapConfig(
                    environment = when (config.environment) {
                        "dev" -> AppUpdateChannel.Dev
                        else -> AppUpdateChannel.Beta
                    },
                    apiBase = normalizeServerBaseUrl(config.apiBase),
                    source = config.source.ifBlank { "remote-bootstrap" },
                    configuredAt = config.configuredAt,
                ),
            )
        }
    }

    private fun parseError(raw: String, fallback: String): IOException {
        val error = runCatching {
            json.decodeFromString<WatchBootstrapErrorDto>(raw)
        }.getOrNull()
        val message = error?.message?.ifBlank { null } ?: fallback
        val code = error?.code?.trim().orEmpty()
        return if (code.isBlank()) {
            IOException(message)
        } else {
            WatchBootstrapException(code = code, message = message)
        }
    }
}

@Serializable
private data class WatchBootstrapRegisterRequestDto(
    val deviceName: String,
    val appVersion: String,
    val platform: String,
)

@Serializable
private data class WatchBootstrapRegisterResponseDto(
    val ok: Boolean = false,
    val bootstrapCode: String = "",
    val registeredAt: String = "",
)

@Serializable
private data class WatchBootstrapPollResponseDto(
    val ok: Boolean = false,
    val status: WatchBootstrapStatus = WatchBootstrapStatus.Unknown,
    val config: WatchBootstrapConfigDto? = null,
)

@Serializable
private enum class WatchBootstrapStatus {
    @SerialName("pending")
    Pending,

    @SerialName("ready")
    Ready,

    Unknown,
}

@Serializable
private data class WatchBootstrapConfigDto(
    val environment: String,
    val apiBase: String,
    val source: String = "",
    val configuredAt: String = "",
)

@Serializable
private data class WatchBootstrapErrorDto(
    val code: String = "",
    val message: String = "",
)
