package ai.openwatcher.watchapp.data

import kotlinx.coroutines.test.runTest
import kotlinx.serialization.SerializationException
import okhttp3.Call
import okhttp3.Callback
import okhttp3.Protocol
import okhttp3.Request
import okhttp3.Response
import okhttp3.ResponseBody.Companion.toResponseBody
import okio.Buffer
import okio.Timeout
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class WatchBootstrapClientTest {
    @Test
    fun register_postsDeviceInfoAndNormalizesReturnedCode() = runTest {
        val callFactory = RecordingCallFactory { request ->
            assertEquals("POST", request.method)
            assertEquals("https://bootstrap.example.com/v1/watch-bootstrap/register", request.url.toString())
            assertTrue(requestBody(request).contains("Xiaomi Watch 5"))
            assertTrue(requestBody(request).contains("0.2.10 (16)"))
            StubResponse(
                body = """{"ok":true,"bootstrapCode":"ab12cd34","registeredAt":"2026-06-12T00:00:00Z"}""",
            )
        }
        val client = WatchBootstrapClient("https://bootstrap.example.com", callFactory)

        val registration = client.register("Xiaomi Watch 5", "0.2.10 (16)")

        assertEquals("AB12CD34", registration.bootstrapCode)
        assertEquals("2026-06-12T00:00:00Z", registration.registeredAt)
    }

    @Test
    fun poll_returnsPendingAndReadyConfig() = runTest {
        val responses = ArrayDeque(
            listOf(
                StubResponse(body = """{"ok":true,"status":"pending"}"""),
                StubResponse(body = """{"ok":true,"status":"ready","config":{"environment":"dev","apiBase":"https://dev.example.com/","source":"desktop-remote-bootstrap","configuredAt":"2026-06-12T00:00:00Z"}}"""),
            ),
        )
        val expectedCodes = ArrayDeque(listOf("AB12-CD34", "AB12CD34"))
        val callFactory = RecordingCallFactory { request ->
            assertEquals("GET", request.method)
            assertTrue(request.url.toString().startsWith("https://bootstrap.example.com/v1/watch-bootstrap/config?"))
            assertEquals(expectedCodes.removeFirst(), request.url.queryParameter("code"))
            responses.removeFirst()
        }
        val client = WatchBootstrapClient("https://bootstrap.example.com", callFactory)

        assertEquals(WatchBootstrapPollResult.Pending, client.poll(" ab12-cd34 "))
        val ready = client.poll("AB12CD34") as WatchBootstrapPollResult.Ready

        assertEquals(AppUpdateChannel.Dev, ready.config.environment)
        assertEquals("https://dev.example.com", ready.config.apiBase)
        assertEquals("desktop-remote-bootstrap", ready.config.source)
    }

    @Test
    fun poll_usesStructuredErrorMessage() = runTest {
        val callFactory = RecordingCallFactory {
            StubResponse(
                code = 410,
                body = """{"ok":false,"code":"bootstrap_code_expired","message":"临时配置码已失效"}""",
            )
        }
        val client = WatchBootstrapClient("https://bootstrap.example.com", callFactory)

        val error = kotlin.runCatching { client.poll("AB12CD34") }.exceptionOrNull()

        assertTrue(error is WatchBootstrapException)
        assertEquals("bootstrap_code_expired", (error as WatchBootstrapException).code)
        assertEquals("临时配置码已失效", error?.message)
    }

    @Test
    fun register_rejectsIncompleteSuccessResponse() = runTest {
        val callFactory = RecordingCallFactory {
            StubResponse(body = """{"ok":true,"registeredAt":"2026-06-12T00:00:00Z"}""")
        }
        val client = WatchBootstrapClient("https://bootstrap.example.com", callFactory)

        val error = kotlin.runCatching { client.register("watch", "0.2.10") }.exceptionOrNull()

        assertTrue(error is SerializationException)
    }

    private data class StubResponse(
        val code: Int = 200,
        val body: String,
    )

    private class RecordingCallFactory(
        private val handler: (Request) -> StubResponse,
    ) : Call.Factory {
        override fun newCall(request: Request): Call {
            return object : Call {
                override fun request(): Request = request

                override fun execute(): Response {
                    val response = handler(request)
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

    private fun requestBody(request: Request): String {
        val buffer = Buffer()
        request.body?.writeTo(buffer)
        return buffer.readUtf8()
    }
}
