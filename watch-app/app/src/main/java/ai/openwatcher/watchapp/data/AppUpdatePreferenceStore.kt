package ai.openwatcher.watchapp.data

import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

interface AppUpdatePreferenceStore {
    fun read(): AppUpdatePreferences
    fun write(preferences: AppUpdatePreferences)
    fun clear()
}

enum class AppUpdateChannel {
    Beta,
    Dev,
}

data class AppUpdatePreferences(
    val selectedChannel: AppUpdateChannel = AppUpdateChannel.Beta,
    val autoCheckEnabled: Boolean = false,
    val ignoredVersionCodes: Set<Int> = emptySet(),
    val currentVersionNotes: AppUpdateVersionNotes? = null,
    val pendingInstalledVersionNotes: AppUpdateVersionNotes? = null,
)

data class AppUpdateVersionNotes(
    val versionName: String,
    val versionCode: Int,
    val notes: List<AppUpdateNote> = emptyList(),
    val emptyMessage: String = "该版本暂未提供更新说明",
)

data class AppUpdateNote(
    val publishedAtLabel: String,
    val summary: String,
)

class SharedPreferencesAppUpdatePreferenceStore(
    private val store: KeyValueStore,
    private val json: Json = Json { ignoreUnknownKeys = true; explicitNulls = false },
) : AppUpdatePreferenceStore {
    override fun read(): AppUpdatePreferences {
        val raw = store.getString(KEY_APP_UPDATE_PREFERENCES).orEmpty().trim()
        if (raw.isBlank()) {
            return AppUpdatePreferences()
        }
        return runCatching {
            json.decodeFromString<AppUpdatePreferencesDto>(raw).toDomain()
        }.getOrDefault(AppUpdatePreferences())
    }

    override fun write(preferences: AppUpdatePreferences) {
        val payload = json.encodeToString(AppUpdatePreferencesDto.fromDomain(preferences))
        store.putString(KEY_APP_UPDATE_PREFERENCES, payload)
    }

    override fun clear() {
        store.remove(KEY_APP_UPDATE_PREFERENCES)
    }

    companion object {
        private const val KEY_APP_UPDATE_PREFERENCES = "app_update_preferences"
    }
}

@Serializable
private data class AppUpdatePreferencesDto(
    val selectedChannel: String = AppUpdateChannel.Beta.name,
    val autoCheckEnabled: Boolean = false,
    val ignoredVersionCodes: List<Int> = emptyList(),
    val currentVersionNotes: AppUpdateVersionNotesDto? = null,
    val pendingInstalledVersionNotes: AppUpdateVersionNotesDto? = null,
) {
    fun toDomain(): AppUpdatePreferences {
        return AppUpdatePreferences(
            selectedChannel = parseChannel(selectedChannel),
            autoCheckEnabled = autoCheckEnabled,
            ignoredVersionCodes = ignoredVersionCodes.toSet(),
            currentVersionNotes = currentVersionNotes?.toDomain(),
            pendingInstalledVersionNotes = pendingInstalledVersionNotes?.toDomain(),
        )
    }

    companion object {
        fun fromDomain(preferences: AppUpdatePreferences): AppUpdatePreferencesDto {
            return AppUpdatePreferencesDto(
                selectedChannel = preferences.selectedChannel.name,
                autoCheckEnabled = preferences.autoCheckEnabled,
                ignoredVersionCodes = preferences.ignoredVersionCodes.sorted(),
                currentVersionNotes = preferences.currentVersionNotes?.let(AppUpdateVersionNotesDto::fromDomain),
                pendingInstalledVersionNotes = preferences.pendingInstalledVersionNotes?.let(AppUpdateVersionNotesDto::fromDomain),
            )
        }

        private fun parseChannel(rawValue: String): AppUpdateChannel {
            return when (rawValue.trim().lowercase()) {
                "dev" -> AppUpdateChannel.Dev
                else -> AppUpdateChannel.Beta
            }
        }
    }
}

@Serializable
private data class AppUpdateVersionNotesDto(
    val versionName: String,
    val versionCode: Int,
    val notes: List<AppUpdateNoteDto> = emptyList(),
    val emptyMessage: String = "该版本暂未提供更新说明",
) {
    fun toDomain(): AppUpdateVersionNotes {
        return AppUpdateVersionNotes(
            versionName = versionName,
            versionCode = versionCode,
            notes = notes.map(AppUpdateNoteDto::toDomain),
            emptyMessage = emptyMessage,
        )
    }

    companion object {
        fun fromDomain(notes: AppUpdateVersionNotes): AppUpdateVersionNotesDto {
            return AppUpdateVersionNotesDto(
                versionName = notes.versionName,
                versionCode = notes.versionCode,
                notes = notes.notes.map(AppUpdateNoteDto::fromDomain),
                emptyMessage = notes.emptyMessage,
            )
        }
    }
}

@Serializable
private data class AppUpdateNoteDto(
    val publishedAtLabel: String,
    val summary: String,
) {
    fun toDomain(): AppUpdateNote {
        return AppUpdateNote(
            publishedAtLabel = publishedAtLabel,
            summary = summary,
        )
    }

    companion object {
        fun fromDomain(note: AppUpdateNote): AppUpdateNoteDto {
            return AppUpdateNoteDto(
                publishedAtLabel = note.publishedAtLabel,
                summary = note.summary,
            )
        }
    }
}
