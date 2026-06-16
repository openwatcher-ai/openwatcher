package ai.openwatcher.watchapp.data

import kotlinx.coroutines.test.runTest
import okhttp3.Call
import okhttp3.Callback
import okhttp3.Protocol
import okhttp3.Request
import okhttp3.Response
import okhttp3.ResponseBody.Companion.toResponseBody
import okio.Timeout
import org.junit.Assert.assertEquals
import org.junit.Test

class DynamicWatcherApiTest {
    @Test
    fun checkHealth_usesLatestBaseUrlProviderValue() = runTest {
        var currentBaseUrl = "https://first.example.com"
        val callFactory = CapturingCallFactory("""{"ok":true}""")
        val api = DynamicWatcherApi(
            baseUrlProvider = { currentBaseUrl },
            callFactory = callFactory,
        )

        api.checkHealth()
        currentBaseUrl = "https://second.example.com"
        api.checkHealth()

        assertEquals(
            listOf(
                "https://first.example.com/healthz",
                "https://second.example.com/healthz",
            ),
            callFactory.urls,
        )
    }

    private class CapturingCallFactory(
        private val body: String,
    ) : Call.Factory {
        val urls = mutableListOf<String>()

        override fun newCall(request: Request): Call {
            urls += request.url.toString()
            return object : Call {
                override fun request(): Request = request

                override fun execute(): Response {
                    return Response.Builder()
                        .request(request)
                        .protocol(Protocol.HTTP_1_1)
                        .code(200)
                        .message("OK")
                        .body(body.toResponseBody())
                        .build()
                }

                override fun enqueue(responseCallback: Callback) {
                    throw UnsupportedOperationException("not used")
                }

                override fun cancel() = Unit

                override fun isExecuted(): Boolean = false

                override fun isCanceled(): Boolean = false

                override fun timeout() = Timeout.NONE

                override fun clone(): Call = this
            }
        }
    }
}
