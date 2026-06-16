package ai.openwatcher.watchapp

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class WatchBuildEnvironmentTest {
    @Test
    fun debugBuildDoesNotTargetProductionServer() {
        if (!BuildConfig.ENABLE_DEBUG_DEMO) {
            return
        }

        val baseUrl = BuildConfig.OPENWATCHER_BASE_URL
        assertTrue(BuildConfig.APPLICATION_ID.endsWith(".debug"))
        assertFalse(baseUrl.contains("127.0.0.1.invalid"))
        assertFalse(baseUrl.contains(":8787"))
    }
}
