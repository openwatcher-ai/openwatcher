package ai.openwatcher.watchapp.data

import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.provider.Settings
import androidx.core.content.FileProvider
import java.io.File
import java.io.IOException
import java.net.ConnectException
import java.net.NoRouteToHostException
import java.net.SocketException
import java.net.SocketTimeoutException
import java.net.UnknownHostException
import java.security.MessageDigest
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.Call
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.Request
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticEventLogger
import ai.openwatcher.watchapp.data.diagnostics.DiagnosticLevel
import ai.openwatcher.watchapp.data.diagnostics.NoOpDiagnosticEventLogger

private const val DEFAULT_BETA_CHANNEL_MANIFEST_URL = "https://openwatcher.ai/channels/beta.json"
private const val DEFAULT_BETA_CHANGELOG_URL = "https://openwatcher.ai/changelog.json"

data class WatcherApkUpdate(
    val channel: AppUpdateChannel = AppUpdateChannel.Beta,
    val versionName: String,
    val versionCode: Int,
    val artifact: String,
    val sha256: String,
    val commit: String,
    val builtAt: String,
    val downloadUrl: String,
    val fallbackDownloadUrl: String? = null,
    val summary: String? = null,
    val changelogUrl: String? = null,
    val apkSizeBytes: Long? = null,
    val releaseNotes: List<WatcherApkReleaseNote> = emptyList(),
)

data class WatcherApkReleaseNote(
    val versionName: String,
    val versionCode: Int,
    val publishedAt: Instant? = null,
    val publishedAtLabel: String,
    val summary: String,
)

sealed interface WatcherApkUpdateCheckResult {
    data class Success(val update: WatcherApkUpdate) : WatcherApkUpdateCheckResult
    data class Failure(val message: String) : WatcherApkUpdateCheckResult
}

sealed interface WatcherApkInstallResult {
    data object Started : WatcherApkInstallResult
    data class PermissionRequired(val message: String, val settingsOpened: Boolean) : WatcherApkInstallResult
    data class Failure(val message: String) : WatcherApkInstallResult
}

sealed interface WatcherApkUpdateProgress {
    data class Downloading(
        val bytesDownloaded: Long,
        val totalBytes: Long?,
        val speedBytesPerSecond: Long? = null,
    ) : WatcherApkUpdateProgress

    data object PreparingInstall : WatcherApkUpdateProgress
}

interface WatcherApkUpdateManager {
    fun canRequestPackageInstalls(): Boolean
    fun openInstallPermissionSettings(): Boolean
    suspend fun fetchLatestUpdate(): WatcherApkUpdateCheckResult
    suspend fun hasCachedUpdate(update: WatcherApkUpdate): Boolean
    suspend fun startInstallFromCache(update: WatcherApkUpdate): WatcherApkInstallResult
    suspend fun downloadAndStartInstall(
        update: WatcherApkUpdate,
        onProgress: (WatcherApkUpdateProgress) -> Unit = {},
    ): WatcherApkInstallResult
}

private const val APK_DOWNLOAD_PROGRESS_INTERVAL_MS = 1_000L

internal class ApkDownloadProgressSampler(
    private val intervalMs: Long = APK_DOWNLOAD_PROGRESS_INTERVAL_MS,
    private val monotonicNowMs: () -> Long = { System.nanoTime() / 1_000_000L },
) {
    private var lastReportedAtMs: Long = monotonicNowMs()
    private var lastReportedBytes: Long = 0L

    fun next(bytesDownloaded: Long, totalBytes: Long?): WatcherApkUpdateProgress.Downloading? {
        val nowMs = monotonicNowMs()
        if (nowMs - lastReportedAtMs < intervalMs) {
            return null
        }
        val elapsedMs = (nowMs - lastReportedAtMs).coerceAtLeast(1L)
        val speedBytesPerSecond = (((bytesDownloaded - lastReportedBytes).coerceAtLeast(0L)) * 1000L) / elapsedMs
        lastReportedAtMs = nowMs
        lastReportedBytes = bytesDownloaded
        return WatcherApkUpdateProgress.Downloading(
            bytesDownloaded = bytesDownloaded,
            totalBytes = totalBytes,
            speedBytesPerSecond = speedBytesPerSecond,
        )
    }
}

object NoOpWatcherApkUpdateManager : WatcherApkUpdateManager {
    override fun canRequestPackageInstalls(): Boolean = false

    override fun openInstallPermissionSettings(): Boolean = false

    override suspend fun fetchLatestUpdate(): WatcherApkUpdateCheckResult {
        return WatcherApkUpdateCheckResult.Failure("更新功能未配置")
    }

    override suspend fun hasCachedUpdate(update: WatcherApkUpdate): Boolean = false

    override suspend fun startInstallFromCache(update: WatcherApkUpdate): WatcherApkInstallResult {
        return WatcherApkInstallResult.Failure("更新功能未配置")
    }

    override suspend fun downloadAndStartInstall(
        update: WatcherApkUpdate,
        onProgress: (WatcherApkUpdateProgress) -> Unit,
    ): WatcherApkInstallResult {
        return WatcherApkInstallResult.Failure("更新功能未配置")
    }
}

class AndroidWatcherApkUpdateManager(
    context: Context,
    private val baseUrl: String,
    private val channel: AppUpdateChannel,
    private val deviceToken: String?,
    private val currentVersionCode: Int,
    private val callFactory: Call.Factory,
    betaPrimaryMetadataUrl: String,
    betaBackupMetadataUrl: String,
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
    private val json: Json = Json { ignoreUnknownKeys = true },
    private val diagnosticEventLogger: DiagnosticEventLogger = NoOpDiagnosticEventLogger,
) : WatcherApkUpdateManager {
    private val appContext = context.applicationContext
    private val updatesDir = File(appContext.cacheDir, "updates")
    private val pendingMetadataFile = File(updatesDir, "pending-update.json")

    init {
        cleanupCompletedDownloadIfNeeded()
    }

    override fun canRequestPackageInstalls(): Boolean {
        return appContext.packageManager.canRequestPackageInstalls()
    }

    override fun openInstallPermissionSettings(): Boolean {
        return runCatching {
            val intent = Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES).apply {
                data = Uri.parse("package:${appContext.packageName}")
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            appContext.startActivity(intent)
            true
        }.getOrDefault(false)
    }

    override suspend fun fetchLatestUpdate(): WatcherApkUpdateCheckResult = withContext(ioDispatcher) {
        val traceId = diagnosticEventLogger.newTraceId("apk-update-check")
        val startedAt = Instant.now()
        diagnosticEventLogger.log(
            event = "user_action",
            traceId = traceId,
            fields = mapOf(
                "action" to mapOf(
                    "name" to "check_app_update",
                    "channel" to channel.name.lowercase(),
                ),
            ),
        )

        try {
            val lookup = when (channel) {
                AppUpdateChannel.Beta -> fetchBetaLatestUpdate(traceId, startedAt)
                AppUpdateChannel.Dev -> fetchDevLatestUpdate(traceId, startedAt)
            }
            val update = lookup.update
                ?: return@withContext WatcherApkUpdateCheckResult.Failure(
                    lookup.failureMessage ?: defaultUpdateCheckFailureMessage(),
                )
            val notes = if (update.versionCode > currentVersionCode) {
                fetchDisplayReleaseNotes(update)
            } else {
                emptyList()
            }
            WatcherApkUpdateCheckResult.Success(update.copy(releaseNotes = notes))
        } catch (error: SerializationException) {
            diagnosticEventLogger.log(
                event = "network_result",
                level = DiagnosticLevel.Warn,
                traceId = traceId,
                fields = mapOf(
                    "target" to mapOf(
                        "name" to "latest_apk_metadata",
                        "channel" to channel.name.lowercase(),
                        "method" to "GET",
                    ),
                    "result" to mapOf(
                        "durationMs" to (Instant.now().toEpochMilli() - startedAt.toEpochMilli()).coerceAtLeast(0L),
                        "errorType" to error::class.java.simpleName,
                        "message" to error.message,
                    ),
                ),
            )
            WatcherApkUpdateCheckResult.Failure("更新信息无法解析")
        } catch (error: IOException) {
            diagnosticEventLogger.log(
                event = "network_result",
                level = DiagnosticLevel.Warn,
                traceId = traceId,
                fields = mapOf(
                    "target" to mapOf(
                        "name" to "latest_apk_metadata",
                        "channel" to channel.name.lowercase(),
                        "method" to "GET",
                    ),
                    "result" to mapOf(
                        "durationMs" to (Instant.now().toEpochMilli() - startedAt.toEpochMilli()).coerceAtLeast(0L),
                        "errorType" to error::class.java.simpleName,
                        "message" to error.message,
                    ),
                ),
            )
            WatcherApkUpdateCheckResult.Failure(classifyAppUpdateCheckFailure(error))
        } catch (error: IllegalArgumentException) {
            WatcherApkUpdateCheckResult.Failure("更新地址无效")
        }
    }

    override suspend fun hasCachedUpdate(update: WatcherApkUpdate): Boolean = withContext(ioDispatcher) {
        val metadata = readPendingMetadata() ?: return@withContext false
        val targetFile = findCachedFile(metadata)
        if (!targetFile.isFile) {
            return@withContext false
        }
        if (metadata.versionCode != update.versionCode) {
            return@withContext false
        }
        if (metadata.versionName.isNotBlank() && metadata.versionName != update.versionName) {
            return@withContext false
        }
        if (metadata.artifact != targetFile.name || metadata.artifact != safeFileName(update.artifact)) {
            return@withContext false
        }
        val expectedSha = update.sha256.trim().lowercase()
        if (metadata.sha256.isNotBlank() && metadata.sha256.trim().lowercase() != expectedSha) {
            return@withContext false
        }
        if (expectedSha.isBlank()) {
            return@withContext true
        }
        sha256(targetFile) == expectedSha
    }

    override suspend fun startInstallFromCache(update: WatcherApkUpdate): WatcherApkInstallResult = withContext(ioDispatcher) {
        val traceId = diagnosticEventLogger.newTraceId("apk-install-cache")
        val metadata = readPendingMetadata() ?: return@withContext WatcherApkInstallResult.Failure("本地缓存不存在，请重新下载")
        val targetFile = findCachedFile(metadata)
        if (!hasCachedUpdate(update)) {
            return@withContext WatcherApkInstallResult.Failure("本地缓存已过期，请重新下载")
        }
        try {
            diagnosticEventLogger.log(
                event = "user_action",
                traceId = traceId,
                fields = mapOf(
                    "action" to mapOf(
                        "name" to "install_cached_update",
                        "artifact" to targetFile.name,
                    ),
                ),
            )
            writePendingMetadata(update, targetFile)
            if (!canRequestPackageInstalls()) {
                diagnosticEventLogger.log(
                    event = "queue_state",
                    level = DiagnosticLevel.Warn,
                    traceId = traceId,
                    fields = mapOf(
                        "queue" to mapOf(
                            "name" to "apk_update",
                            "state" to "permission_required",
                        ),
                    ),
                )
                val opened = openInstallPermissionSettings()
                return@withContext WatcherApkInstallResult.PermissionRequired(
                    message = "需要允许安装未知来源更新",
                    settingsOpened = opened,
                )
            }
            startInstallIntent(targetFile)
            WatcherApkInstallResult.Started
        } catch (error: ActivityNotFoundException) {
            WatcherApkInstallResult.Failure("未找到系统安装器")
        } catch (error: SecurityException) {
            val opened = openInstallPermissionSettings()
            WatcherApkInstallResult.PermissionRequired(
                message = "系统拒绝安装，请允许安装未知来源更新",
                settingsOpened = opened,
            )
        }
    }

    override suspend fun downloadAndStartInstall(
        update: WatcherApkUpdate,
        onProgress: (WatcherApkUpdateProgress) -> Unit,
    ): WatcherApkInstallResult = withContext(ioDispatcher) {
        val traceId = diagnosticEventLogger.newTraceId("apk-download")
        val targetFile = File(updatesDir, safeFileName(update.artifact))
        try {
            resetPendingDownloadDir()
            diagnosticEventLogger.log(
                event = "user_action",
                traceId = traceId,
                fields = mapOf(
                    "action" to mapOf(
                        "name" to "download_app_update",
                        "artifact" to update.artifact,
                        "downloadUrl" to update.downloadUrl,
                        "channel" to update.channel.name.lowercase(),
                    ),
                ),
            )
            val primaryResult = downloadIntoFile(
                traceId = traceId,
                targetFile = targetFile,
                url = update.downloadUrl,
                onProgress = onProgress,
            )
            if (primaryResult != null) {
                val fallbackUrl = update.fallbackDownloadUrl?.trim().orEmpty()
                if (fallbackUrl.isBlank() || fallbackUrl == update.downloadUrl) {
                    return@withContext primaryResult
                }
                diagnosticEventLogger.log(
                    event = "queue_state",
                    level = DiagnosticLevel.Warn,
                    traceId = traceId,
                    fields = mapOf(
                        "queue" to mapOf(
                            "name" to "apk_update",
                            "state" to "retrying_with_fallback_url",
                            "fallbackDownloadUrl" to fallbackUrl,
                        ),
                    ),
                )
                resetPendingDownloadDir()
                val fallbackResult = downloadIntoFile(
                    traceId = traceId,
                    targetFile = targetFile,
                    url = fallbackUrl,
                    onProgress = onProgress,
                )
                if (fallbackResult != null) {
                    return@withContext fallbackResult
                }
            }
            val expectedSha = update.sha256.trim().lowercase()
            if (expectedSha.isNotBlank() && sha256(targetFile) != expectedSha) {
                targetFile.delete()
                return@withContext WatcherApkInstallResult.Failure("下载文件校验失败")
            }
            writePendingMetadata(update, targetFile)
            onProgress(WatcherApkUpdateProgress.PreparingInstall)
            diagnosticEventLogger.log(
                event = "queue_state",
                traceId = traceId,
                fields = mapOf(
                    "queue" to mapOf(
                        "name" to "apk_update",
                        "state" to "download_completed",
                        "artifact" to targetFile.name,
                        "bytesDownloaded" to targetFile.length(),
                    ),
                ),
            )
            if (!canRequestPackageInstalls()) {
                val opened = openInstallPermissionSettings()
                return@withContext WatcherApkInstallResult.PermissionRequired(
                    message = "需要允许安装未知来源更新",
                    settingsOpened = opened,
                )
            }
            startInstallIntent(targetFile)
            WatcherApkInstallResult.Started
        } catch (error: ActivityNotFoundException) {
            WatcherApkInstallResult.Failure("未找到系统安装器")
        } catch (error: SecurityException) {
            val opened = openInstallPermissionSettings()
            WatcherApkInstallResult.PermissionRequired(
                message = "系统拒绝安装，请允许安装未知来源更新",
                settingsOpened = opened,
            )
        } catch (error: IOException) {
            WatcherApkInstallResult.Failure("下载失败，请检查网络")
        } catch (error: IllegalArgumentException) {
            WatcherApkInstallResult.Failure("下载地址无效")
        }
    }

    @Suppress("DEPRECATION")
    private fun startInstallIntent(file: File) {
        val uri = FileProvider.getUriForFile(
            appContext,
            "${appContext.packageName}.apkprovider",
            file,
        )
        val intent = Intent(Intent.ACTION_INSTALL_PACKAGE).apply {
            setDataAndType(uri, "application/vnd.android.package-archive")
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            putExtra(Intent.EXTRA_NOT_UNKNOWN_SOURCE, true)
        }
        appContext.startActivity(intent)
    }

    private fun safeFileName(value: String): String {
        return value.ifBlank { "openwatcher-latest.apk" }
            .replace(Regex("[^A-Za-z0-9._-]"), "-")
            .takeLast(120)
    }

    private fun fetchDisplayReleaseNotes(update: WatcherApkUpdate): List<WatcherApkReleaseNote> {
        val changelogUrl = update.changelogUrl
            ?.trim()
            ?.takeIf { it.isNotBlank() }
            ?: buildDefaultChangelogUrl()
            ?: return fallbackReleaseNotes(update)
        val request = authorizedRequest(changelogUrl)
            .get()
            .build()
        return runCatching {
            callFactory.newCall(request).execute().use { response ->
                if (!response.isSuccessful) {
                    return@use fallbackReleaseNotes(update)
                }
                val body = response.body?.string().orEmpty()
                parseReleaseNotes(body)
                    .filter { it.versionCode in (currentVersionCode + 1)..update.versionCode }
                    .sortedWith(
                        compareByDescending<WatcherApkReleaseNote> { it.versionCode }
                            .thenByDescending { it.publishedAt ?: Instant.MIN },
                    )
                    .ifEmpty { fallbackReleaseNotes(update) }
            }
        }.getOrElse {
            fallbackReleaseNotes(update)
        }
    }

    private fun buildDefaultChangelogUrl(): String? {
        return when (channel) {
            AppUpdateChannel.Beta -> DEFAULT_BETA_CHANGELOG_URL
            AppUpdateChannel.Dev -> baseUrl.toHttpUrl().newBuilder()
                .addPathSegment("file")
                .addPathSegment("dev")
                .addPathSegment("changelog.json")
                .build()
                .toString()
        }
    }

    private fun parseReleaseNotes(payload: String): List<WatcherApkReleaseNote> {
        val root = json.parseToJsonElement(payload)
        val entries = when (root) {
            is JsonArray -> root
            is JsonObject -> root["entries"] as? JsonArray ?: JsonArray(emptyList())
            else -> JsonArray(emptyList())
        }
        return entries.mapNotNull { element ->
            val item = element as? JsonObject ?: return@mapNotNull null
            val watch = item["components"]
                ?.jsonObjectOrNull()
                ?.get("watch")
                ?.jsonObjectOrNull()
                ?: return@mapNotNull null
            val versionCode = watch["versionCode"]?.jsonPrimitive?.intOrNull ?: return@mapNotNull null
            val versionName = watch["versionName"]?.jsonPrimitive?.contentOrNull.orEmpty()
            val summary = item["notes"]
                ?.jsonObjectOrNull()
                ?.watchNoteTexts()
                ?.joinToString(separator = "\n")
                .orEmpty()
            if (summary.isBlank()) {
                return@mapNotNull null
            }
            val publishedAt = item["publishedAt"]?.jsonPrimitive?.contentOrNull
                ?.let { raw -> runCatching { Instant.parse(raw) }.getOrNull() }
            val publishedAtLabel = formatPublishedAtLabel(
                publishedAt = publishedAt,
                publishedAtLabel = null,
            )
            WatcherApkReleaseNote(
                versionName = versionName,
                versionCode = versionCode,
                publishedAt = publishedAt,
                publishedAtLabel = publishedAtLabel,
                summary = summary,
            )
        }
    }

    private fun JsonObject.watchNoteTexts(): List<String> {
        return listOf("features", "improvements", "fixes", "compatibility")
            .flatMap { key -> this[key] as? JsonArray ?: JsonArray(emptyList()) }
            .mapNotNull { element ->
                val note = element as? JsonObject ?: return@mapNotNull null
                val component = note["component"]?.jsonPrimitive?.contentOrNull?.trim().orEmpty()
                val text = note["text"]?.jsonPrimitive?.contentOrNull?.trim().orEmpty()
                text.takeIf { component == "手表应用" && it.isNotBlank() }
            }
    }

    private fun fallbackReleaseNotes(update: WatcherApkUpdate): List<WatcherApkReleaseNote> {
        val summary = update.summary?.trim().orEmpty()
        if (summary.isBlank()) {
            return emptyList()
        }
        val publishedAt = runCatching { Instant.parse(update.builtAt) }.getOrNull()
        return listOf(
            WatcherApkReleaseNote(
                versionName = update.versionName,
                versionCode = update.versionCode,
                publishedAt = publishedAt,
                publishedAtLabel = formatPublishedAtLabel(publishedAt, null),
                summary = summary,
            ),
        )
    }

    private fun formatPublishedAtLabel(
        publishedAt: Instant?,
        publishedAtLabel: String?,
    ): String {
        val trimmedLabel = publishedAtLabel?.trim().orEmpty()
        if (trimmedLabel.isNotBlank()) {
            return trimmedLabel
        }
        return publishedAt?.let(BEIJING_TIME_FORMATTER::format) ?: "--"
    }

    private fun cleanupCompletedDownloadIfNeeded() {
        val metadata = readPendingMetadata() ?: return
        if (metadata.versionCode <= currentVersionCode) {
            updatesDir.deleteRecursively()
        }
    }

    private fun resetPendingDownloadDir() {
        if (updatesDir.exists()) {
            updatesDir.deleteRecursively()
        }
        updatesDir.mkdirs()
    }

    private fun findCachedFile(metadata: PendingInstallMetadataDto): File {
        return File(updatesDir, safeFileName(metadata.artifact))
    }

    private fun writePendingMetadata(update: WatcherApkUpdate, file: File) {
        val payload = json.encodeToString(
            PendingInstallMetadataDto(
                versionName = update.versionName,
                versionCode = update.versionCode,
                artifact = file.name,
                sha256 = update.sha256,
            ),
        )
        pendingMetadataFile.writeText(payload)
    }

    private fun readPendingMetadata(): PendingInstallMetadataDto? {
        if (!pendingMetadataFile.isFile) {
            return null
        }
        return runCatching {
            json.decodeFromString<PendingInstallMetadataDto>(pendingMetadataFile.readText())
        }.getOrNull()
    }

    private fun sha256(file: File): String {
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buffer = ByteArray(DEFAULT_BUFFER_SIZE)
            while (true) {
                val read = input.read(buffer)
                if (read < 0) {
                    break
                }
                digest.update(buffer, 0, read)
            }
        }
        return digest.digest().joinToString("") { "%02x".format(it) }
    }

    private suspend fun fetchBetaLatestUpdate(
        traceId: String,
        startedAt: Instant,
    ): UpdateLookupResult {
        val primaryCandidate = MetadataCandidate(
            url = normalizedBetaPrimaryMetadataUrl(),
            pathLabel = "/channels/beta.json",
            sourceLabel = "beta-channel",
        )
        val primaryResponse = fetchMetadataBody(primaryCandidate, traceId, startedAt)
        primaryResponse.body?.let { payload ->
            val parsed = parseBetaPrimaryMetadata(payload, primaryCandidate.url)
            return UpdateLookupResult(update = parsed.update)
        }
        return UpdateLookupResult(failureMessage = primaryResponse.failureMessage ?: defaultUpdateCheckFailureMessage())
    }

    private suspend fun fetchDevLatestUpdate(
        traceId: String,
        startedAt: Instant,
    ): UpdateLookupResult {
        var lastFailure: String? = null
        for (candidate in devMetadataCandidates()) {
            val response = fetchMetadataBody(candidate, traceId, startedAt)
            if (response.body == null) {
                lastFailure = response.failureMessage ?: lastFailure
                continue
            }
            val dto = json.decodeFromString<LatestApkMetadataDto>(response.body)
            return UpdateLookupResult(
                update = dto.toDomain(
                    channel = channel,
                    baseUrl = baseUrl,
                    metadataUrl = candidate.url,
                ),
            )
        }
        return UpdateLookupResult(failureMessage = lastFailure ?: defaultUpdateCheckFailureMessage())
    }

    private fun devMetadataCandidates(): List<MetadataCandidate> {
        return listOf(
            MetadataCandidate(
                url = baseUrl.toHttpUrl().newBuilder()
                    .addPathSegment("file")
                    .addPathSegment("dev")
                    .addPathSegment("latest.json")
                    .build()
                    .toString(),
                pathLabel = "/file/dev/latest.json",
                sourceLabel = "dev",
            ),
        )
    }

    private suspend fun fetchMetadataBody(
        candidate: MetadataCandidate,
        traceId: String,
        startedAt: Instant,
    ): MetadataFetchResponse {
        val resolvedMetadataUrl = candidate.url

        val request = authorizedRequest(resolvedMetadataUrl)
            .get()
            .build()
        callFactory.newCall(request).execute().use { response ->
            diagnosticEventLogger.log(
                event = "network_result",
                traceId = traceId,
                fields = mapOf(
                    "target" to mapOf(
                        "name" to "latest_apk_metadata",
                        "path" to candidate.pathLabel,
                        "method" to "GET",
                        "source" to candidate.sourceLabel,
                        "resolvedUrl" to resolvedMetadataUrl,
                    ),
                    "result" to mapOf(
                        "statusCode" to response.code,
                        "durationMs" to (Instant.now().toEpochMilli() - startedAt.toEpochMilli()).coerceAtLeast(0L),
                    ),
                ),
            )
            if (!response.isSuccessful) {
                return MetadataFetchResponse(
                    failureMessage = failureMessageForStatusCode(response.code),
                )
            }
            return MetadataFetchResponse(
                body = response.body?.string().orEmpty(),
                metadataUrl = resolvedMetadataUrl,
            )
        }
    }

    private fun parseBetaPrimaryMetadata(
        payload: String,
        metadataUrl: String,
    ): ParsedWatchManifest {
        val channelManifest = runCatching {
            json.decodeFromString<WatchChannelManifestDto>(payload)
        }.getOrNull()
        channelManifest?.toPrimaryManifest()?.let { return it }

        val dto = json.decodeFromString<LatestApkMetadataDto>(payload)
        return ParsedWatchManifest(
            update = dto.toDomain(
                channel = AppUpdateChannel.Beta,
                baseUrl = baseUrl,
                metadataUrl = metadataUrl,
            ),
        )
    }

    private fun normalizedBetaPrimaryMetadataUrl(): String {
        return DEFAULT_BETA_CHANNEL_MANIFEST_URL
    }

    private fun failureMessageForStatusCode(statusCode: Int): String {
        return when (channel) {
            AppUpdateChannel.Beta -> "更新检查失败 HTTP $statusCode"
            AppUpdateChannel.Dev -> "开发通道更新检查失败 HTTP $statusCode"
        }
    }

    private fun defaultUpdateCheckFailureMessage(): String {
        return when (channel) {
            AppUpdateChannel.Beta -> "更新检查失败"
            AppUpdateChannel.Dev -> "开发通道更新检查失败"
        }
    }

    private fun authorizedRequest(url: String): Request.Builder {
        return Request.Builder()
            .url(url)
            .apply {
                if (channel == AppUpdateChannel.Dev) {
                    val token = deviceToken?.trim().orEmpty()
                    if (token.isNotBlank()) {
                        header("X-OpenWatcher-Token", token)
                    }
                }
            }
    }

    private suspend fun downloadIntoFile(
        traceId: String,
        targetFile: File,
        url: String,
        onProgress: (WatcherApkUpdateProgress) -> Unit,
    ): WatcherApkInstallResult? {
        val request = authorizedRequest(url)
            .get()
            .build()
        callFactory.newCall(request).execute().use { response ->
            if (!response.isSuccessful) {
                return WatcherApkInstallResult.Failure("下载失败 HTTP ${response.code}")
            }
            val body = response.body ?: return WatcherApkInstallResult.Failure("下载内容为空")
            val totalBytes = body.contentLength().takeIf { it > 0L }
            var nextDiagnosticPercent = 5
            val progressSampler = ApkDownloadProgressSampler()
            body.byteStream().use { input ->
                targetFile.outputStream().use { output ->
                    val buffer = ByteArray(DEFAULT_BUFFER_SIZE)
                    var downloadedBytes = 0L
                    while (true) {
                        val read = input.read(buffer)
                        if (read < 0) {
                            break
                        }
                        output.write(buffer, 0, read)
                        downloadedBytes += read
                        val nextPercent = totalBytes?.let {
                            ((downloadedBytes * 100) / it).toInt().coerceIn(0, 100)
                        }
                        progressSampler.next(
                            bytesDownloaded = downloadedBytes,
                            totalBytes = totalBytes,
                        )?.let(onProgress)
                        if (nextPercent != null && nextPercent >= nextDiagnosticPercent) {
                            diagnosticEventLogger.log(
                                event = "queue_state",
                                traceId = traceId,
                                fields = mapOf(
                                    "queue" to mapOf(
                                        "name" to "apk_update",
                                        "state" to "downloading",
                                        "progressPercent" to nextPercent,
                                        "bytesDownloaded" to downloadedBytes,
                                        "totalBytes" to totalBytes,
                                    ),
                                ),
                            )
                            while (nextDiagnosticPercent <= nextPercent) {
                                nextDiagnosticPercent += 5
                            }
                        }
                    }
                }
            }
        }
        return null
    }
}

private data class MetadataCandidate(
    val url: String,
    val pathLabel: String,
    val sourceLabel: String,
)

private data class MetadataFetchResponse(
    val body: String? = null,
    val failureMessage: String? = null,
    val metadataUrl: String? = null,
)

private data class UpdateLookupResult(
    val update: WatcherApkUpdate? = null,
    val failureMessage: String? = null,
)

private data class ParsedWatchManifest(
    val update: WatcherApkUpdate,
)

private fun LatestApkMetadataDto.toDomain(
    channel: AppUpdateChannel,
    baseUrl: String,
    metadataUrl: String,
): WatcherApkUpdate {
    val fallbackDownloadUrl = when (channel) {
        AppUpdateChannel.Beta -> metadataUrl.substringBeforeLast('/') + "/" + artifact.ifBlank { "openwatcher-watchapp-latest.apk" }
        AppUpdateChannel.Dev -> baseUrl.toHttpUrl().newBuilder()
            .addPathSegment("file")
            .addPathSegment("dev")
            .addPathSegment("apk")
            .build()
            .toString()
    }
    return WatcherApkUpdate(
        channel = channel,
        versionName = versionName,
        versionCode = versionCode,
        artifact = artifact,
        sha256 = sha256,
        commit = commit.trim(),
        builtAt = publishedAt?.trim()?.takeIf { it.isNotBlank() } ?: builtAt,
        downloadUrl = downloadUrl?.takeIf { it.isNotBlank() } ?: apkUrl?.takeIf { it.isNotBlank() } ?: fallbackDownloadUrl,
        fallbackDownloadUrl = this.fallbackDownloadUrl?.trim()?.takeIf {
            channel == AppUpdateChannel.Dev && it.isNotBlank()
        },
        summary = summary?.trim()?.takeIf { it.isNotBlank() },
        changelogUrl = changelogUrl?.trim()?.takeIf { it.isNotBlank() },
        apkSizeBytes = apkSizeBytes
            ?.takeIf { it > 0L }
            ?: sizeBytes?.takeIf { it > 0L }
            ?: artifactSizeBytes?.takeIf { it > 0L },
    )
}

@Serializable
private data class LatestApkMetadataDto(
    val channel: String? = null,
    val builtAt: String = "",
    val publishedAt: String? = null,
    val versionName: String = "",
    val versionCode: Int = 0,
    val commit: String = "",
    val artifact: String = "",
    val sha256: String = "",
    val downloadUrl: String? = null,
    val fallbackDownloadUrl: String? = null,
    val apkUrl: String? = null,
    val summary: String? = null,
    val changelogUrl: String? = null,
    val apkSizeBytes: Long? = null,
    val sizeBytes: Long? = null,
    val artifactSizeBytes: Long? = null,
)

@Serializable
private data class WatchChannelManifestDto(
    val schemaVersion: Int? = null,
    val channel: String? = null,
    val updatedAt: String? = null,
    val source: WatchChannelSourceDto? = null,
    val release: WatchChannelReleaseDto? = null,
    val product: WatchChannelProductDto? = null,
    val watch: WatchChannelWatchDto? = null,
)

@Serializable
private data class WatchChannelSourceDto(
    val commit: String? = null,
    val summary: String? = null,
    val publishedAt: String? = null,
    val updatedAt: String? = null,
)

@Serializable
private data class WatchChannelReleaseDto(
    val summary: String? = null,
    val publishedAt: String? = null,
    val updatedAt: String? = null,
)

@Serializable
private data class WatchChannelProductDto(
    val version: String? = null,
    val summary: String? = null,
    val publishedAt: String? = null,
    val updatedAt: String? = null,
)

@Serializable
private data class WatchChannelWatchDto(
    val versionName: String? = null,
    val versionCode: Int? = null,
    val artifact: String? = null,
    val downloadUrl: String? = null,
    val changelogUrl: String? = null,
    val apkUrl: String? = null,
    val sha256: String? = null,
    val apkSizeBytes: Long? = null,
    val sizeBytes: Long? = null,
    val artifactSizeBytes: Long? = null,
)

@Serializable
private data class PendingInstallMetadataDto(
    val versionName: String = "",
    val versionCode: Int = 0,
    val artifact: String = "",
    val sha256: String = "",
)

internal fun classifyAppUpdateCheckFailure(error: IOException): String {
    return when (error) {
        is SocketTimeoutException -> "访问超时"
        is UnknownHostException,
        is ConnectException,
        is NoRouteToHostException,
        is SocketException,
        -> "网络不可用"
        else -> "检查失败"
    }
}

private val BEIJING_TIME_FORMATTER: DateTimeFormatter =
    DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm").withZone(ZoneId.of("Asia/Shanghai"))

private fun WatchChannelManifestDto.toPrimaryManifest(): ParsedWatchManifest? {
    val update = toWatcherApkUpdate() ?: return null
    return ParsedWatchManifest(update = update)
}

private fun WatchChannelManifestDto.toWatcherApkUpdate(): WatcherApkUpdate? {
    val watchPayload = watch ?: return null
    val versionName = watchPayload.versionName.trimToNull() ?: return null
    val versionCode = watchPayload.versionCode?.takeIf { it > 0 } ?: return null
    val artifact = watchPayload.artifact.trimToNull() ?: return null
    val sha256 = watchPayload.sha256.trimToNull() ?: return null
    val downloadUrl = watchPayload.downloadUrl.trimToNull()
        ?: watchPayload.apkUrl.trimToNull()
        ?: return null
    val builtAt = source?.publishedAt.trimToNull()
        ?: source?.updatedAt.trimToNull()
        ?: release?.publishedAt.trimToNull()
        ?: release?.updatedAt.trimToNull()
        ?: product?.publishedAt.trimToNull()
        ?: product?.updatedAt.trimToNull()
        ?: updatedAt.trimToNull()
        ?: ""
    return WatcherApkUpdate(
        channel = AppUpdateChannel.Beta,
        versionName = versionName,
        versionCode = versionCode,
        artifact = artifact,
        sha256 = sha256,
        commit = source?.commit?.trim().orEmpty(),
        builtAt = builtAt,
        downloadUrl = downloadUrl,
        fallbackDownloadUrl = null,
        summary = release?.summary.trimToNull()
            ?: product?.summary.trimToNull()
            ?: source?.summary.trimToNull(),
        changelogUrl = watchPayload.changelogUrl.trimToNull(),
        apkSizeBytes = watchPayload.apkSizeBytes
            ?.takeIf { it > 0L }
            ?: watchPayload.sizeBytes?.takeIf { it > 0L }
            ?: watchPayload.artifactSizeBytes?.takeIf { it > 0L },
    )
}

private fun String?.trimToNull(): String? {
    val trimmed = this?.trim().orEmpty()
    return trimmed.takeIf { it.isNotBlank() }
}

private fun JsonElement.jsonObjectOrNull(): JsonObject? {
    return this as? JsonObject
}
