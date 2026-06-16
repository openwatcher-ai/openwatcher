package ai.openwatcher.watchapp.data

import java.time.Instant
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

data class SessionDetailsWindowCacheSnapshot(
    val selectedThreadId: String? = null,
    val selectedSlotIndex: Int? = null,
    val cachedAt: Instant? = null,
    val window: SessionWindowSnapshot,
)

interface SessionDetailsWindowStore {
    fun read(): SessionDetailsWindowCacheSnapshot?
    fun write(snapshot: SessionDetailsWindowCacheSnapshot)
    fun clear()
}

class SessionDetailsWindowPreferenceStore(
    private val store: KeyValueStore,
    private val json: Json = Json { ignoreUnknownKeys = true; explicitNulls = false },
) : SessionDetailsWindowStore {
    override fun read(): SessionDetailsWindowCacheSnapshot? {
        val raw = store.getString(KEY_SESSION_DETAILS_WINDOW).orEmpty().trim()
        if (raw.isBlank()) {
            return null
        }
        return runCatching {
            json.decodeFromString<SessionDetailsWindowCacheDto>(raw).toDomain()
        }.getOrNull()
    }

    override fun write(snapshot: SessionDetailsWindowCacheSnapshot) {
        store.putString(
            KEY_SESSION_DETAILS_WINDOW,
            json.encodeToString(SessionDetailsWindowCacheDto.fromDomain(snapshot)),
        )
    }

    override fun clear() {
        store.remove(KEY_SESSION_DETAILS_WINDOW)
    }

    companion object {
        private const val KEY_SESSION_DETAILS_WINDOW = "session_details_window_snapshot"
    }
}

@Serializable
private data class SessionDetailsWindowCacheDto(
    val selectedThreadId: String? = null,
    val selectedSlotIndex: Int? = null,
    val cachedAt: String? = null,
    val window: SessionWindowSnapshotCacheDto,
) {
    fun toDomain(): SessionDetailsWindowCacheSnapshot {
        return SessionDetailsWindowCacheSnapshot(
            selectedThreadId = selectedThreadId,
            selectedSlotIndex = selectedSlotIndex,
            cachedAt = cachedAt?.let(Instant::parse),
            window = window.toDomain(),
        )
    }

    companion object {
        fun fromDomain(snapshot: SessionDetailsWindowCacheSnapshot): SessionDetailsWindowCacheDto {
            return SessionDetailsWindowCacheDto(
                selectedThreadId = snapshot.selectedThreadId,
                selectedSlotIndex = snapshot.selectedSlotIndex,
                cachedAt = snapshot.cachedAt?.toString(),
                window = SessionWindowSnapshotCacheDto.fromDomain(snapshot.window),
            )
        }
    }
}

@Serializable
private data class SessionWindowSnapshotCacheDto(
    val observedAt: String? = null,
    val limit: Int,
    val threadOrder: List<String> = emptyList(),
    val sessions: List<SessionWindowEntryCacheDto> = emptyList(),
) {
    fun toDomain(): SessionWindowSnapshot {
        return SessionWindowSnapshot(
            observedAt = observedAt?.let(Instant::parse),
            limit = limit,
            threadOrder = threadOrder,
            sessions = sessions.map { it.toDomain() },
        )
    }

    companion object {
        fun fromDomain(snapshot: SessionWindowSnapshot): SessionWindowSnapshotCacheDto {
            return SessionWindowSnapshotCacheDto(
                observedAt = snapshot.observedAt?.toString(),
                limit = snapshot.limit,
                threadOrder = snapshot.threadOrder,
                sessions = snapshot.sessions.map(SessionWindowEntryCacheDto::fromDomain),
            )
        }
    }
}

@Serializable
private data class SessionWindowEntryCacheDto(
    val session: SessionWindowSessionCacheDto,
    val runtimeState: SessionWindowRuntimeStateCacheDto,
    val latestAgentMessage: SessionWindowAgentMessageCacheDto? = null,
) {
    fun toDomain(): SessionWindowEntry {
        return SessionWindowEntry(
            session = session.toDomain(),
            runtimeState = runtimeState.toDomain(),
            latestAgentMessage = latestAgentMessage?.toDomain(),
        )
    }

    companion object {
        fun fromDomain(entry: SessionWindowEntry): SessionWindowEntryCacheDto {
            return SessionWindowEntryCacheDto(
                session = SessionWindowSessionCacheDto.fromDomain(entry.session),
                runtimeState = SessionWindowRuntimeStateCacheDto.fromDomain(entry.runtimeState),
                latestAgentMessage = entry.latestAgentMessage?.let(SessionWindowAgentMessageCacheDto::fromDomain),
            )
        }
    }
}

@Serializable
private data class SessionWindowSessionCacheDto(
    val threadId: String,
    val title: String,
    val updatedAt: String? = null,
    val model: String,
    val reasoningEffort: String,
    val tokensUsedTotal: Long,
    val contextUsedTokens: Long,
    val contextWindow: Long,
    val contextPressurePercent: Int,
    val contextCompactThresholdTokens: Long? = null,
    val contextCompactThresholdPercent: Int? = null,
    val contextCompaction: SessionWindowContextCompactionCacheDto? = null,
    val lastActiveAgoMinutes: Int,
) {
    fun toDomain(): SessionSnapshot {
        return SessionSnapshot(
            threadId = threadId,
            title = title,
            updatedAt = updatedAt?.let(Instant::parse),
            model = model,
            reasoningEffort = reasoningEffort,
            tokensUsedTotal = tokensUsedTotal,
            contextUsedTokens = contextUsedTokens,
            contextWindow = contextWindow,
            contextPressurePercent = contextPressurePercent,
            contextCompactThresholdTokens = contextCompactThresholdTokens,
            contextCompactThresholdPercent = contextCompactThresholdPercent,
            contextCompaction = contextCompaction?.toDomain(),
            lastActiveAgoMinutes = lastActiveAgoMinutes,
        )
    }

    companion object {
        fun fromDomain(snapshot: SessionSnapshot): SessionWindowSessionCacheDto {
            return SessionWindowSessionCacheDto(
                threadId = snapshot.threadId,
                title = snapshot.title,
                updatedAt = snapshot.updatedAt?.toString(),
                model = snapshot.model,
                reasoningEffort = snapshot.reasoningEffort,
                tokensUsedTotal = snapshot.tokensUsedTotal,
                contextUsedTokens = snapshot.contextUsedTokens,
                contextWindow = snapshot.contextWindow,
                contextPressurePercent = snapshot.contextPressurePercent,
                contextCompactThresholdTokens = snapshot.contextCompactThresholdTokens,
                contextCompactThresholdPercent = snapshot.contextCompactThresholdPercent,
                contextCompaction = snapshot.contextCompaction?.let(SessionWindowContextCompactionCacheDto::fromDomain),
                lastActiveAgoMinutes = snapshot.lastActiveAgoMinutes,
            )
        }
    }
}

@Serializable
private data class SessionWindowContextCompactionCacheDto(
    val trigger: String = "",
    val startedAt: String? = null,
    val updatedAt: String? = null,
    val turnId: String? = null,
) {
    fun toDomain(): ContextCompactionSnapshot {
        return ContextCompactionSnapshot(
            trigger = trigger,
            startedAt = startedAt?.let(Instant::parse),
            updatedAt = updatedAt?.let(Instant::parse),
            turnId = turnId,
        )
    }

    companion object {
        fun fromDomain(compaction: ContextCompactionSnapshot): SessionWindowContextCompactionCacheDto {
            return SessionWindowContextCompactionCacheDto(
                trigger = compaction.trigger,
                startedAt = compaction.startedAt?.toString(),
                updatedAt = compaction.updatedAt?.toString(),
                turnId = compaction.turnId,
            )
        }
    }
}

@Serializable
private data class SessionWindowRuntimeStateCacheDto(
    val threadId: String,
    val turnId: String? = null,
    val startedAt: String? = null,
    val running: Boolean,
    val lifecycle: String,
    val phase: String,
    val updatedAt: String? = null,
    val sequence: Long,
) {
    fun toDomain(): SessionRuntimeState {
        return SessionRuntimeState(
            threadId = threadId,
            turnId = turnId,
            startedAt = startedAt?.let(Instant::parse),
            running = running,
            lifecycle = enumValueOf(lifecycle),
            phase = enumValueOf(phase),
            updatedAt = updatedAt?.let(Instant::parse),
            sequence = sequence,
        )
    }

    companion object {
        fun fromDomain(state: SessionRuntimeState): SessionWindowRuntimeStateCacheDto {
            return SessionWindowRuntimeStateCacheDto(
                threadId = state.threadId,
                turnId = state.turnId,
                startedAt = state.startedAt?.toString(),
                running = state.running,
                lifecycle = state.lifecycle.name,
                phase = state.phase.name,
                updatedAt = state.updatedAt?.toString(),
                sequence = state.sequence,
            )
        }
    }
}

@Serializable
private data class SessionWindowAgentMessageCacheDto(
    val threadId: String,
    val eventId: String,
    val createdAt: String? = null,
    val text: String,
    val truncated: Boolean,
) {
    fun toDomain(): SessionAgentMessage {
        return SessionAgentMessage(
            threadId = threadId,
            eventId = eventId,
            createdAt = createdAt?.let(Instant::parse),
            text = text,
            truncated = truncated,
        )
    }

    companion object {
        fun fromDomain(message: SessionAgentMessage): SessionWindowAgentMessageCacheDto {
            return SessionWindowAgentMessageCacheDto(
                threadId = message.threadId,
                eventId = message.eventId,
                createdAt = message.createdAt?.toString(),
                text = message.text,
                truncated = message.truncated,
            )
        }
    }
}
