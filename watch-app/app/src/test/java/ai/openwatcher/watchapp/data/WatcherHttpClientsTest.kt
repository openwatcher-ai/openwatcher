package ai.openwatcher.watchapp.data

import java.util.concurrent.TimeUnit
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotSame
import org.junit.Test

class WatcherHttpClientsTest {
    @Test
    fun createSessionStreamClient_disablesReadTimeoutOnlyForStreamClient() {
        val defaultClient = WatcherHttpClients.createDefaultClient()
            .newBuilder()
            .readTimeout(10, TimeUnit.SECONDS)
            .build()

        val streamClient = WatcherHttpClients.createSessionStreamClient(defaultClient)

        assertNotSame(defaultClient, streamClient)
        assertEquals(10_000, defaultClient.readTimeoutMillis)
        assertEquals(0, streamClient.readTimeoutMillis)
        assertEquals(defaultClient.connectTimeoutMillis, streamClient.connectTimeoutMillis)
    }
}
