package ai.openwatcher.watchapp.data

import android.content.Context
import android.content.ContextWrapper
import java.io.File
import java.io.IOException
import java.net.ConnectException
import java.net.SocketTimeoutException
import java.net.UnknownHostException
import java.nio.file.Files
import java.util.ArrayDeque
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.test.runTest
import okhttp3.Call
import okhttp3.Callback
import okhttp3.Protocol
import okhttp3.Request
import okhttp3.Response
import okhttp3.ResponseBody.Companion.toResponseBody
import okio.Timeout
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class WatcherApkUpdateManagerTest {
    @Test
    fun classifyAppUpdateCheckFailure_distinguishesTimeoutNetworkAndFallback() {
        assertEquals("访问超时", classifyAppUpdateCheckFailure(SocketTimeoutException("timeout")))
        assertEquals("网络不可用", classifyAppUpdateCheckFailure(UnknownHostException("offline")))
        assertEquals("网络不可用", classifyAppUpdateCheckFailure(ConnectException("refused")))
        assertEquals("检查失败", classifyAppUpdateCheckFailure(IOException("boom")))
    }

    @Test
    fun apkDownloadProgressSampler_reportsAtFixedOneSecondIntervals() {
        val times = ArrayDeque(listOf(0L, 500L, 1_000L, 1_900L, 2_000L))
        val sampler = ApkDownloadProgressSampler(
            monotonicNowMs = { times.removeFirst() },
        )

        assertNull(sampler.next(bytesDownloaded = 50L, totalBytes = 400L))

        val first = sampler.next(bytesDownloaded = 100L, totalBytes = 400L) as WatcherApkUpdateProgress.Downloading
        assertEquals(100L, first.bytesDownloaded)
        assertEquals(400L, first.totalBytes)
        assertEquals(100L, first.speedBytesPerSecond)

        assertNull(sampler.next(bytesDownloaded = 180L, totalBytes = 400L))

        val second = sampler.next(bytesDownloaded = 220L, totalBytes = 400L) as WatcherApkUpdateProgress.Downloading
        assertEquals(220L, second.bytesDownloaded)
        assertEquals(400L, second.totalBytes)
        assertEquals(120L, second.speedBytesPerSecond)
    }

    @Test
    fun fetchLatestUpdate_betaChannelManifestSuccess_usesChannelManifestUrlAndWatchFields() = runTest {
        val primaryUrl = "https://openwatcher.ai/channels/beta.json"
        val callFactory = RoutingCallFactory(
            mapOf(
                primaryUrl to StubResponse(
                    code = 200,
                    body = """
                    {
                      "schemaVersion": 1,
                      "channel": "beta",
                      "updatedAt": "2026-06-11T08:30:00Z",
                      "source": {
                        "commit": "abc1234",
                        "publishedAt": "2026-06-11T08:00:00Z"
                      },
                      "release": {
                        "summary": "beta 发布说明"
                      },
                      "watch": {
                        "versionName": "0.1.1",
                        "versionCode": 10001,
                        "artifact": "openwatcher-watch-release.apk",
                        "downloadUrl": "https://openwatcher.ai/downloads/openwatcher-watch-release.apk",
                        "fallbackDownloadUrl": "https://openwatcher.ai/downloads/fallback-openwatcher-watch-release.apk",
                        "sha256": "sha-primary"
                      }
                    }
                    """.trimIndent(),
                ),
            ),
        )
        val manager = createManager(
            channel = AppUpdateChannel.Beta,
            currentVersionCode = 10001,
            callFactory = callFactory,
            betaPrimaryMetadataUrl = "https://openwatcher.ai/file/beta/latest.json",
        )

        val result = manager.fetchLatestUpdate()

        val success = result as WatcherApkUpdateCheckResult.Success
        assertEquals(
            listOf(primaryUrl),
            callFactory.requests.map { it.url.toString() },
        )
        assertEquals("0.1.1", success.update.versionName)
        assertEquals(10001, success.update.versionCode)
        assertEquals("openwatcher-watch-release.apk", success.update.artifact)
        assertEquals("https://openwatcher.ai/downloads/openwatcher-watch-release.apk", success.update.downloadUrl)
        assertNull(success.update.fallbackDownloadUrl)
        assertEquals("sha-primary", success.update.sha256)
        assertEquals("abc1234", success.update.commit)
        assertEquals("beta 发布说明", success.update.summary)
        assertEquals("2026-06-11T08:00:00Z", success.update.builtAt)
    }

    @Test
    fun fetchLatestUpdate_betaPrimaryFailure_returnsFailureWithoutBackupRequest() = runTest {
        val primaryUrl = "https://openwatcher.ai/channels/beta.json"
        val callFactory = RoutingCallFactory(
            mapOf(
                primaryUrl to StubResponse(code = 503, body = """{"error":"busy"}"""),
            ),
        )
        val manager = createManager(
            channel = AppUpdateChannel.Beta,
            currentVersionCode = 10002,
            callFactory = callFactory,
            betaPrimaryMetadataUrl = "https://openwatcher.ai/file/beta/latest.json",
            betaBackupMetadataUrl = "https://backup.example/releases",
        )

        val result = manager.fetchLatestUpdate()

        val failure = result as WatcherApkUpdateCheckResult.Failure
        assertEquals(
            listOf(primaryUrl),
            callFactory.requests.map { it.url.toString() },
        )
        assertEquals("更新检查失败 HTTP 503", failure.message)
    }

    @Test
    fun fetchLatestUpdate_devChannelStillUsesLegacyPathAndTokenHeader() = runTest {
        val callFactory = RoutingCallFactory(
            mapOf(
                "https://watcher.example/file/dev/latest.json" to StubResponse(
                    code = 200,
                    body = """
                    {
                      "versionName": "0.1.3-dev",
                      "versionCode": 10003,
                      "commit": "dev123",
                      "artifact": "openwatcher-watch-dev.apk",
                      "sha256": "sha-dev",
                      "builtAt": "2026-06-11T10:00:00Z"
                    }
                    """.trimIndent(),
                ),
            ),
        )
        val manager = createManager(
            channel = AppUpdateChannel.Dev,
            currentVersionCode = 10003,
            callFactory = callFactory,
            baseUrl = "https://watcher.example",
            deviceToken = "dev-token",
        )

        val result = manager.fetchLatestUpdate()

        val success = result as WatcherApkUpdateCheckResult.Success
        assertEquals(
            listOf("https://watcher.example/file/dev/latest.json"),
            callFactory.requests.map { it.url.toString() },
        )
        assertEquals("dev-token", callFactory.requests.single().header("X-OpenWatcher-Token"))
        assertEquals("https://watcher.example/file/dev/apk", success.update.downloadUrl)
        assertEquals(AppUpdateChannel.Dev, success.update.channel)
    }

    @Test
    fun fetchLatestUpdate_betaChangelogUsesWatchNotesFromAggregateChangelog() = runTest {
        val primaryUrl = "https://openwatcher.ai/channels/beta.json"
        val changelogUrl = "https://openwatcher.ai/changelog.json"
        val callFactory = RoutingCallFactory(
            mapOf(
                primaryUrl to StubResponse(
                    code = 200,
                    body = """
                    {
                      "schemaVersion": 1,
                      "channel": "beta",
                      "updatedAt": "2026-06-13T04:00:52Z",
                      "source": {
                        "commit": "cc6720bd2a791e135a8063c217718018f4c6d78e"
                      },
                      "release": {
                        "summary": "优化手表端初始化配置体验",
                        "publishedAt": "2026-06-13T04:00:52Z"
                      },
                      "watch": {
                        "versionName": "0.3.0",
                        "versionCode": 10003,
                        "artifact": "openwatcher-watchapp-v0.3.0.apk",
                        "downloadUrl": "https://openwatcher.ai/downloads/openwatcher-watchapp-v0.3.0.apk",
                        "sha256": "sha-watch"
                      }
                    }
                    """.trimIndent(),
                ),
                changelogUrl to StubResponse(
                    code = 200,
                    body = """
                    {
                      "entries": [
                        {
                          "id": "beta-2026.06.13.1",
                          "publishedAt": "2026-06-13T04:00:52Z",
                          "components": {
                            "watch": {
                              "status": "updated",
                              "versionName": "0.3.0",
                              "versionCode": 10003
                            }
                          },
                          "notes": {
                            "features": [],
                            "improvements": [
                              {
                                "component": "手表应用",
                                "text": "优化手表端初始化配置体验：服务不可达时显示服务地址和处理建议。"
                              }
                            ],
                            "fixes": [],
                            "compatibility": [
                              {
                                "component": "兼容性",
                                "text": "本次继续复用当前 Runtime Release。"
                              }
                            ]
                          }
                        }
                      ]
                    }
                    """.trimIndent(),
                ),
            ),
        )
        val manager = createManager(
            channel = AppUpdateChannel.Beta,
            currentVersionCode = 10002,
            callFactory = callFactory,
        )

        val result = manager.fetchLatestUpdate()

        val success = result as WatcherApkUpdateCheckResult.Success
        assertEquals(
            listOf(primaryUrl, changelogUrl),
            callFactory.requests.map { it.url.toString() },
        )
        assertEquals("优化手表端初始化配置体验", success.update.summary)
        assertEquals(1, success.update.releaseNotes.size)
        assertEquals(
            "优化手表端初始化配置体验：服务不可达时显示服务地址和处理建议。",
            success.update.releaseNotes.single().summary,
        )
        assertEquals("2026-06-13 12:00", success.update.releaseNotes.single().publishedAtLabel)
    }

    private fun createManager(
        channel: AppUpdateChannel,
        currentVersionCode: Int,
        callFactory: Call.Factory,
        baseUrl: String = "https://watcher.example",
        deviceToken: String? = null,
        betaPrimaryMetadataUrl: String = "https://openwatcher.ai/channels/beta.json",
        betaBackupMetadataUrl: String = "",
    ): AndroidWatcherApkUpdateManager {
        val cacheDir = Files.createTempDirectory("watcher-apk-update-test").toFile()
        return AndroidWatcherApkUpdateManager(
            context = TestContext(cacheDir),
            baseUrl = baseUrl,
            channel = channel,
            deviceToken = deviceToken,
            currentVersionCode = currentVersionCode,
            callFactory = callFactory,
            betaPrimaryMetadataUrl = betaPrimaryMetadataUrl,
            betaBackupMetadataUrl = betaBackupMetadataUrl,
            ioDispatcher = Dispatchers.Unconfined,
        )
    }

    private data class StubResponse(
        val code: Int,
        val body: String,
    )

    private class RoutingCallFactory(
        private val responses: Map<String, StubResponse>,
    ) : Call.Factory {
        val requests = mutableListOf<Request>()

        override fun newCall(request: Request): Call {
            requests += request
            val response = responses[request.url.toString()]
                ?: StubResponse(code = 404, body = """{"error":"not found"}""")
            return object : Call {
                override fun request(): Request = request

                override fun execute(): Response {
                    return Response.Builder()
                        .request(request)
                        .protocol(Protocol.HTTP_1_1)
                        .code(response.code)
                        .message(if (response.code in 200..299) "OK" else "ERR")
                        .body(response.body.toResponseBody())
                        .build()
                }

                override fun enqueue(responseCallback: Callback) {
                    throw UnsupportedOperationException("not used")
                }

                override fun cancel() = Unit

                override fun isExecuted(): Boolean = false

                override fun isCanceled(): Boolean = false

                override fun timeout(): Timeout = Timeout.NONE

                override fun clone(): Call = this
            }
        }
    }

    private class TestContext(
        private val cacheDirRoot: File,
    ) : ContextWrapper(null) {
        override fun getApplicationContext(): Context = this

        override fun getCacheDir(): File = cacheDirRoot

        override fun getPackageName(): String = "ai.openwatcher.watchapp.test"
    }
}
