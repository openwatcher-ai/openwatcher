package ai.openwatcher.watchapp.data

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test

class AppUpdatePreferenceStoreTest {
    @Test
    fun read_returnsDefaultsWhenStoreIsEmpty() {
        val store = SharedPreferencesAppUpdatePreferenceStore(FakeKeyValueStore())

        val preferences = store.read()

        assertEquals(AppUpdateChannel.Beta, preferences.selectedChannel)
        assertFalse(preferences.autoCheckEnabled)
        assertTrue(preferences.ignoredVersionCodes.isEmpty())
        assertEquals(null, preferences.currentVersionNotes)
        assertEquals(null, preferences.pendingInstalledVersionNotes)
    }

    @Test
    fun write_roundTripsAllUpdatePreferences() {
        val backingStore = FakeKeyValueStore()
        val store = SharedPreferencesAppUpdatePreferenceStore(backingStore)
        val preferences = AppUpdatePreferences(
            selectedChannel = AppUpdateChannel.Dev,
            autoCheckEnabled = true,
            ignoredVersionCodes = setOf(17, 18),
            currentVersionNotes = AppUpdateVersionNotes(
                versionName = "0.2.10",
                versionCode = 16,
                notes = listOf(
                    AppUpdateNote(
                        publishedAtLabel = "2026-06-05 10:00",
                        summary = "当前版本说明",
                    ),
                ),
            ),
            pendingInstalledVersionNotes = AppUpdateVersionNotes(
                versionName = "0.2.15",
                versionCode = 17,
                notes = listOf(
                    AppUpdateNote(
                        publishedAtLabel = "2026-06-06 20:00",
                        summary = "待安装版本说明",
                    ),
                ),
            ),
        )

        store.write(preferences)
        val restored = store.read()

        assertEquals(AppUpdateChannel.Dev, restored.selectedChannel)
        assertTrue(restored.autoCheckEnabled)
        assertEquals(setOf(17, 18), restored.ignoredVersionCodes)
        assertNotNull(restored.currentVersionNotes)
        assertEquals("0.2.10", restored.currentVersionNotes?.versionName)
        assertEquals("当前版本说明", restored.currentVersionNotes?.notes?.single()?.summary)
        assertEquals("0.2.15", restored.pendingInstalledVersionNotes?.versionName)
        assertEquals("待安装版本说明", restored.pendingInstalledVersionNotes?.notes?.single()?.summary)
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
}
