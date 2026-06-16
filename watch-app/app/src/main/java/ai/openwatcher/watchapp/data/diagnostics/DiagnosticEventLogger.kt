package ai.openwatcher.watchapp.data.diagnostics

import java.time.Instant
import java.util.UUID
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import ai.openwatcher.watchapp.ui.AppUiState

interface DiagnosticEventLogger {
    fun newTraceId(prefix: String = "trace"): String

    suspend fun log(
        event: String,
        level: DiagnosticLevel = DiagnosticLevel.Info,
        traceId: String? = null,
        screen: String? = null,
        fields: Map<String, Any?> = emptyMap(),
    )
}

object NoOpDiagnosticEventLogger : DiagnosticEventLogger {
    override fun newTraceId(prefix: String): String = "$prefix-noop"

    override suspend fun log(
        event: String,
        level: DiagnosticLevel,
        traceId: String?,
        screen: String?,
        fields: Map<String, Any?>,
    ) = Unit
}

class StructuredDiagnosticEventLogger(
    private val store: DiagnosticEventStore,
    private val runtimeContext: DiagnosticRuntimeContext,
    private val uiStateProvider: () -> AppUiState,
    private val deviceInfo: DiagnosticDeviceInfo,
    private val appInfo: DiagnosticAppInfo,
    private val clock: () -> Instant = { Instant.now() },
    private val sessionId: String = "app-${UUID.randomUUID().toString().take(8)}",
    private val json: Json = Json { explicitNulls = false },
) : DiagnosticEventLogger {
    override fun newTraceId(prefix: String): String {
        return "$prefix-${UUID.randomUUID().toString().replace("-", "").take(12)}"
    }

    override suspend fun log(
        event: String,
        level: DiagnosticLevel,
        traceId: String?,
        screen: String?,
        fields: Map<String, Any?>,
    ) {
        val timestamp = clock()
        val payload = buildJsonObject {
            put("ts", JsonPrimitive(timestamp.toString()))
            put("event", JsonPrimitive(event))
            put("level", JsonPrimitive(level.wireValue()))
            put("sessionId", JsonPrimitive(sessionId))
            put("traceId", JsonPrimitive(traceId?.takeIf { it.isNotBlank() } ?: newTraceId()))
            put("screen", JsonPrimitive(screen ?: diagnosticScreenName(uiStateProvider())))
            put("device", deviceInfo.toJson())
            put("app", appInfo.toJson())
            put("network", runtimeContext.currentNetworkInfo().toJson())
            fields.forEach { (key, value) ->
                put(key, value.toJsonElement())
            }
        }
        store.append(
            timestamp = timestamp,
            jsonLine = json.encodeToString(JsonObject.serializer(), payload),
        )
    }
}

private fun DiagnosticDeviceInfo.toJson(): JsonObject {
    return buildJsonObject {
        put("manufacturer", JsonPrimitive(manufacturer))
        put("model", JsonPrimitive(model))
        put("sdkInt", JsonPrimitive(sdkInt))
        put("screenWidthPx", JsonPrimitive(screenWidthPx))
        put("screenHeightPx", JsonPrimitive(screenHeightPx))
        put("densityDpi", JsonPrimitive(densityDpi))
        put("fontScale", JsonPrimitive(fontScale))
        put("isRound", JsonPrimitive(isRound))
        put("smallestWidthDp", JsonPrimitive(smallestWidthDp))
    }
}

private fun DiagnosticAppInfo.toJson(): JsonObject {
    return buildJsonObject {
        put("versionName", JsonPrimitive(versionName))
        put("versionCode", JsonPrimitive(versionCode))
        put("buildType", JsonPrimitive(buildType))
    }
}

private fun DiagnosticNetworkInfo.toJson(): JsonObject {
    return buildJsonObject {
        put("baseUrl", JsonPrimitive(baseUrl))
        put("hasPaired", JsonPrimitive(hasPaired))
    }
}

private fun Any?.toJsonElement(): JsonElement {
    return when (this) {
        null -> JsonNull
        is JsonElement -> this
        is String -> JsonPrimitive(this)
        is Boolean -> JsonPrimitive(this)
        is Int -> JsonPrimitive(this)
        is Long -> JsonPrimitive(this)
        is Float -> JsonPrimitive(this)
        is Double -> JsonPrimitive(this)
        is Number -> JsonPrimitive(this)
        is Instant -> JsonPrimitive(this.toString())
        is Enum<*> -> JsonPrimitive(this.name)
        is Map<*, *> -> JsonObject(
            entries.associate { (key, value) ->
                key.toString() to value.toJsonElement()
            },
        )
        is Iterable<*> -> JsonArray(map { it.toJsonElement() })
        is Array<*> -> JsonArray(map { it.toJsonElement() })
        else -> JsonPrimitive(toString())
    }
}
