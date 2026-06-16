package ai.openwatcher.watchapp.ui.home

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class HomeQuotaEasterEggCopyConfigTest {
    @Test
    fun everyPool_hasEnoughCopyAndNoBlankOrDuplicateEntries() {
        HomeQuotaTipPool.entries.forEach { pool ->
            val entries = HomeQuotaEasterEggCopyConfig.entriesFor(pool)
            assertTrue("$pool 文案数量不足", entries.size >= 20)
            assertTrue("$pool 存在空文案", entries.all { it.isNotBlank() })
            assertEquals("$pool 存在重复文案", entries.size, entries.toSet().size)
        }
    }

    @Test
    fun everyPool_copyLengthStaysWithinWatchBubbleBudget() {
        HomeQuotaTipPool.entries.forEach { pool ->
            val entries = HomeQuotaEasterEggCopyConfig.entriesFor(pool)
            assertTrue(
                "$pool 存在过长文案",
                entries.maxOf { it.length } <= 30,
            )
        }
    }
}
