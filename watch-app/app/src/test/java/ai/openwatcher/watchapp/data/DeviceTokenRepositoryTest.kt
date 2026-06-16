package ai.openwatcher.watchapp.data

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class DeviceTokenRepositoryTest {
    @Test
    fun secureRandomToken_isUrlSafeAndNonRepeating() {
        val generator = SecureRandomTokenGenerator()

        val first = generator.generate()
        val second = generator.generate()

        assertTrue(first.length >= 43)
        assertTrue(second.length >= 43)
        assertTrue(first.matches(Regex("^[A-Za-z0-9_-]+$")))
        assertTrue(second.matches(Regex("^[A-Za-z0-9_-]+$")))
        assertNotEquals(first, second)
    }

    @Test
    fun repository_canStoreReadRegenerateAndClear() {
        val store = FakeKeyValueStore()
        val generator = QueueTokenGenerator(mutableListOf("token-one", "token-two"))
        val repository = DeviceTokenRepository(store, generator)

        val first = repository.ensureToken()
        val second = repository.ensureToken()

        assertEquals("token-one", first)
        assertEquals("token-one", second)
        assertEquals("token-one", store.getString("device_token"))
        assertTrue(repository.tokenFingerprint("token-one").matches(Regex("^[0-9a-f]{6}$")))

        val regenerated = repository.regenerate()
        assertEquals("token-two", regenerated)
        assertEquals("token-two", store.getString("device_token"))

        repository.clear()
        assertNull(store.getString("device_token"))
    }

    private class FakeKeyValueStore : KeyValueStore {
        private val values = mutableMapOf<String, String>()

        override fun getString(key: String): String? = values[key]

        override fun putString(key: String, value: String) {
            values[key] = value
        }

        override fun remove(key: String) {
            values.remove(key)
        }
    }

    private class QueueTokenGenerator(
        private val values: MutableList<String>,
    ) : TokenGenerator {
        override fun generate(): String = values.removeAt(0)
    }
}
