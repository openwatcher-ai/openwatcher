package ai.openwatcher.watchapp.data

import android.content.SharedPreferences
import java.security.MessageDigest
import java.security.SecureRandom
import java.util.Base64

interface KeyValueStore {
    fun getString(key: String): String?
    fun putString(key: String, value: String)
    fun remove(key: String)
}

class SharedPreferencesKeyValueStore(
    private val preferences: SharedPreferences,
) : KeyValueStore {
    override fun getString(key: String): String? = preferences.getString(key, null)

    override fun putString(key: String, value: String) {
        preferences.edit().putString(key, value).apply()
    }

    override fun remove(key: String) {
        preferences.edit().remove(key).apply()
    }
}

interface TokenGenerator {
    fun generate(): String
}

class SecureRandomTokenGenerator(
    private val secureRandom: SecureRandom = SecureRandom(),
) : TokenGenerator {
    override fun generate(): String {
        val bytes = ByteArray(32)
        secureRandom.nextBytes(bytes)
        return Base64.getUrlEncoder()
            .withoutPadding()
            .encodeToString(bytes)
    }
}

class DeviceTokenRepository(
    private val store: KeyValueStore,
    private val tokenGenerator: TokenGenerator,
) {
    fun currentToken(): String? = store.getString(KEY_DEVICE_TOKEN)?.trim()?.ifBlank { null }

    fun ensureToken(): String {
        val existing = currentToken().orEmpty()
        if (existing.isNotBlank()) {
            return existing
        }
        return generateAndStore()
    }

    fun regenerate(): String {
        clear()
        return generateAndStore()
    }

    fun clear() {
        store.remove(KEY_DEVICE_TOKEN)
    }

    fun setToken(token: String): String {
        val trimmed = token.trim()
        store.putString(KEY_DEVICE_TOKEN, trimmed)
        return trimmed
    }

    fun tokenFingerprint(token: String): String {
        val digest = MessageDigest.getInstance("SHA-256")
        val bytes = digest.digest(token.toByteArray(Charsets.UTF_8))
        return bytes.take(3).joinToString("") { "%02x".format(it) }
    }

    fun buildPairingPayload(baseUrl: String, token: String, deviceName: String): String {
        val encodedToken = java.net.URLEncoder.encode(token, Charsets.UTF_8.name())
        val encodedName = java.net.URLEncoder.encode(deviceName, Charsets.UTF_8.name())
        return "$baseUrl/pair?deviceToken=$encodedToken&deviceName=$encodedName"
    }

    private fun generateAndStore(): String {
        val token = tokenGenerator.generate()
        store.putString(KEY_DEVICE_TOKEN, token)
        return token
    }

    companion object {
        private const val KEY_DEVICE_TOKEN = "device_token"
    }
}
