package ai.openwatcher.watchapp.data

import java.time.Instant
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

interface PairingStateStore {
    fun isPaired(): Boolean
    fun markPaired()
    fun clear()
}

class PairingPreferenceStore(
    private val store: KeyValueStore,
) : PairingStateStore {
    override fun isPaired(): Boolean = store.getString(KEY_PAIRING_SUCCESS) == VALUE_TRUE

    override fun markPaired() {
        store.putString(KEY_PAIRING_SUCCESS, VALUE_TRUE)
    }

    override fun clear() {
        store.remove(KEY_PAIRING_SUCCESS)
    }

    companion object {
        private const val KEY_PAIRING_SUCCESS = "pairing_success"
        private const val VALUE_TRUE = "1"
    }
}

interface StatusSnapshotStore {
    fun read(): WatcherStatusSnapshot?
    fun write(snapshot: WatcherStatusSnapshot)
    fun clear()
}

class WatcherStatusSnapshotPreferenceStore(
    private val store: KeyValueStore,
    private val json: Json = Json { ignoreUnknownKeys = true; explicitNulls = false },
) : StatusSnapshotStore {
    override fun read(): WatcherStatusSnapshot? {
        val raw = store.getString(KEY_STATUS_SNAPSHOT).orEmpty().trim()
        if (raw.isBlank()) {
            return null
        }
        return runCatching {
            json.decodeFromString<WatcherStatusSnapshotCacheDto>(raw).toDomain()
        }.getOrNull()
    }

    override fun write(snapshot: WatcherStatusSnapshot) {
        store.putString(
            KEY_STATUS_SNAPSHOT,
            json.encodeToString(WatcherStatusSnapshotCacheDto.fromDomain(snapshot)),
        )
    }

    override fun clear() {
        store.remove(KEY_STATUS_SNAPSHOT)
    }

    companion object {
        private const val KEY_STATUS_SNAPSHOT = "watcher_status_snapshot"
    }
}

@Serializable
private data class WatcherStatusSnapshotCacheDto(
    val observedAt: String? = null,
    val quota: QuotaCacheDto? = null,
    val heatmap24h: Heatmap24hCacheDto? = null,
    val heatmap7d: Heatmap7dCacheDto? = null,
    val dailyUsage: DailyUsageCacheDto? = null,
    val dailyTrend30d: SnapshotDailyTrend30dCacheDto? = null,
    val sessions: List<SessionCacheDto> = emptyList(),
    val errors: List<String> = emptyList(),
) {
    fun toDomain(): WatcherStatusSnapshot {
        return WatcherStatusSnapshot(
            observedAt = observedAt?.let(Instant::parse),
            quota = quota?.toDomain(),
            heatmap24h = heatmap24h?.toDomain(),
            heatmap7d = heatmap7d?.toDomain(),
            dailyUsage = dailyUsage?.toDomain(),
            dailyTrend30d = dailyTrend30d?.toDomain(),
            sessions = sessions.map { it.toDomain() },
            errors = errors,
        )
    }

    companion object {
        fun fromDomain(snapshot: WatcherStatusSnapshot): WatcherStatusSnapshotCacheDto {
            return WatcherStatusSnapshotCacheDto(
                observedAt = snapshot.observedAt?.toString(),
                quota = snapshot.quota?.let(QuotaCacheDto::fromDomain),
                heatmap24h = snapshot.heatmap24h?.let(Heatmap24hCacheDto::fromDomain),
                heatmap7d = snapshot.heatmap7d?.let(Heatmap7dCacheDto::fromDomain),
                dailyUsage = snapshot.dailyUsage?.let(DailyUsageCacheDto::fromDomain),
                dailyTrend30d = snapshot.dailyTrend30d?.let(SnapshotDailyTrend30dCacheDto::fromDomain),
                sessions = snapshot.sessions.map(SessionCacheDto::fromDomain),
                errors = snapshot.errors,
            )
        }
    }
}

@Serializable
private data class QuotaCacheDto(
    val source: String,
    val fresh: Boolean,
    val status: String,
    val planType: String? = null,
    val fiveHour: QuotaWindowCacheDto? = null,
    val weekly: QuotaWindowCacheDto? = null,
) {
    fun toDomain(): QuotaSnapshot {
        return QuotaSnapshot(
            source = source,
            fresh = fresh,
            status = status.toQuotaStatus(),
            planType = planType,
            fiveHour = fiveHour?.toDomain(),
            weekly = weekly?.toDomain(),
        )
    }

    companion object {
        fun fromDomain(snapshot: QuotaSnapshot): QuotaCacheDto {
            return QuotaCacheDto(
                source = snapshot.source,
                fresh = snapshot.fresh,
                status = snapshot.status.toStorageValue(),
                planType = snapshot.planType,
                fiveHour = snapshot.fiveHour?.let(QuotaWindowCacheDto::fromDomain),
                weekly = snapshot.weekly?.let(QuotaWindowCacheDto::fromDomain),
            )
        }
    }
}

@Serializable
private data class QuotaWindowCacheDto(
    val usedPercent: Float,
    val remainingPercent: Float,
    val resetAt: String? = null,
) {
    fun toDomain(): QuotaWindow {
        return QuotaWindow(
            usedPercent = usedPercent,
            remainingPercent = remainingPercent,
            resetAt = resetAt?.let(Instant::parse),
        )
    }

    companion object {
        fun fromDomain(window: QuotaWindow): QuotaWindowCacheDto {
            return QuotaWindowCacheDto(
                usedPercent = window.usedPercent,
                remainingPercent = window.remainingPercent,
                resetAt = window.resetAt?.toString(),
            )
        }
    }
}

@Serializable
private data class Heatmap24hCacheDto(
    val timezone: String,
    val generatedAt: String? = null,
    val peakHourStart: String? = null,
    val buckets: List<HeatmapBucketCacheDto> = emptyList(),
) {
    fun toDomain(): Heatmap24hSnapshot {
        return Heatmap24hSnapshot(
            timezone = timezone,
            generatedAt = generatedAt?.let(Instant::parse),
            peakHourStart = peakHourStart?.let(Instant::parse),
            buckets = buckets.map { it.toDomain() },
        )
    }

    companion object {
        fun fromDomain(snapshot: Heatmap24hSnapshot): Heatmap24hCacheDto {
            return Heatmap24hCacheDto(
                timezone = snapshot.timezone,
                generatedAt = snapshot.generatedAt?.toString(),
                peakHourStart = snapshot.peakHourStart?.toString(),
                buckets = snapshot.buckets.map(HeatmapBucketCacheDto::fromDomain),
            )
        }
    }
}

@Serializable
private data class HeatmapBucketCacheDto(
    val hourStart: String? = null,
    val inputTokens: Long,
    val cachedInputTokens: Long,
    val outputTokens: Long,
    val reasoningOutputTokens: Long,
    val totalTokens: Long,
    val activeThreads: Int,
) {
    fun toDomain(): HeatmapBucket {
        return HeatmapBucket(
            hourStart = hourStart?.let(Instant::parse),
            inputTokens = inputTokens,
            cachedInputTokens = cachedInputTokens,
            outputTokens = outputTokens,
            reasoningOutputTokens = reasoningOutputTokens,
            totalTokens = totalTokens,
            activeThreads = activeThreads,
        )
    }

    companion object {
        fun fromDomain(bucket: HeatmapBucket): HeatmapBucketCacheDto {
            return HeatmapBucketCacheDto(
                hourStart = bucket.hourStart?.toString(),
                inputTokens = bucket.inputTokens,
                cachedInputTokens = bucket.cachedInputTokens,
                outputTokens = bucket.outputTokens,
                reasoningOutputTokens = bucket.reasoningOutputTokens,
                totalTokens = bucket.totalTokens,
                activeThreads = bucket.activeThreads,
            )
        }
    }
}

@Serializable
private data class Heatmap7dCacheDto(
    val timezone: String,
    val generatedAt: String? = null,
    val startDate: String,
    val endDate: String,
    val peakTokens: Long,
    val days: List<Heatmap7dDayCacheDto> = emptyList(),
) {
    fun toDomain(): Heatmap7dSnapshot {
        return Heatmap7dSnapshot(
            timezone = timezone,
            generatedAt = generatedAt?.let(Instant::parse),
            startDate = startDate,
            endDate = endDate,
            peakTokens = peakTokens,
            days = days.map { it.toDomain() },
        )
    }

    companion object {
        fun fromDomain(snapshot: Heatmap7dSnapshot): Heatmap7dCacheDto {
            return Heatmap7dCacheDto(
                timezone = snapshot.timezone,
                generatedAt = snapshot.generatedAt?.toString(),
                startDate = snapshot.startDate,
                endDate = snapshot.endDate,
                peakTokens = snapshot.peakTokens,
                days = snapshot.days.map(Heatmap7dDayCacheDto::fromDomain),
            )
        }
    }
}

@Serializable
private data class Heatmap7dDayCacheDto(
    val date: String,
    val totalTokens: Long,
    val hours: List<Long> = emptyList(),
) {
    fun toDomain(): Heatmap7dDay {
        return Heatmap7dDay(
            date = date,
            totalTokens = totalTokens,
            hours = hours,
        )
    }

    companion object {
        fun fromDomain(day: Heatmap7dDay): Heatmap7dDayCacheDto {
            return Heatmap7dDayCacheDto(
                date = day.date,
                totalTokens = day.totalTokens,
                hours = day.hours,
            )
        }
    }
}

@Serializable
private data class DailyUsageCacheDto(
    val generatedAt: String? = null,
    val totalTokens: Long,
    val inputTokens: Long,
    val cachedInputTokens: Long,
    val outputTokens: Long,
    val reasoningOutputTokens: Long,
    val activeSessions: Int,
    val estimatedValueUsd: Double? = null,
    val estimatedValueLabel: String? = null,
    val pricingDate: String? = null,
    val pricingSourceUrl: String? = null,
    val pricingUnavailableReason: String? = null,
    val modelShares: List<DailyUsageModelShareCacheDto> = emptyList(),
) {
    fun toDomain(): DailyUsageSnapshot {
        return DailyUsageSnapshot(
            generatedAt = generatedAt?.let(Instant::parse),
            totalTokens = totalTokens,
            inputTokens = inputTokens,
            cachedInputTokens = cachedInputTokens,
            outputTokens = outputTokens,
            reasoningOutputTokens = reasoningOutputTokens,
            activeSessions = activeSessions,
            estimatedValueUsd = estimatedValueUsd,
            estimatedValueLabel = estimatedValueLabel,
            pricingDate = pricingDate,
            pricingSourceUrl = pricingSourceUrl,
            pricingUnavailableReason = pricingUnavailableReason,
            modelShares = modelShares.map { it.toDomain() },
        )
    }

    companion object {
        fun fromDomain(snapshot: DailyUsageSnapshot): DailyUsageCacheDto {
            return DailyUsageCacheDto(
                generatedAt = snapshot.generatedAt?.toString(),
                totalTokens = snapshot.totalTokens,
                inputTokens = snapshot.inputTokens,
                cachedInputTokens = snapshot.cachedInputTokens,
                outputTokens = snapshot.outputTokens,
                reasoningOutputTokens = snapshot.reasoningOutputTokens,
                activeSessions = snapshot.activeSessions,
                estimatedValueUsd = snapshot.estimatedValueUsd,
                estimatedValueLabel = snapshot.estimatedValueLabel,
                pricingDate = snapshot.pricingDate,
                pricingSourceUrl = snapshot.pricingSourceUrl,
                pricingUnavailableReason = snapshot.pricingUnavailableReason,
                modelShares = snapshot.modelShares.map(DailyUsageModelShareCacheDto::fromDomain),
            )
        }
    }
}

@Serializable
private data class DailyUsageModelShareCacheDto(
    val model: String,
    val tokens: Long,
    val sharePercent: Double,
) {
    fun toDomain(): DailyUsageModelShare {
        return DailyUsageModelShare(
            model = model,
            tokens = tokens,
            sharePercent = sharePercent,
        )
    }

    companion object {
        fun fromDomain(share: DailyUsageModelShare): DailyUsageModelShareCacheDto {
            return DailyUsageModelShareCacheDto(
                model = share.model,
                tokens = share.tokens,
                sharePercent = share.sharePercent,
            )
        }
    }
}

@Serializable
private data class SnapshotDailyTrend30dCacheDto(
    val timezone: String,
    val generatedAt: String? = null,
    val startDate: String,
    val endDate: String,
    val totalTokens: Long,
    val averageTokens: Long,
    val peakTokens: Long,
    val estimatedValueUsd: Double? = null,
    val estimatedValueLabel: String? = null,
    val days: List<SnapshotDailyTrendDayCacheDto> = emptyList(),
) {
    fun toDomain(): DailyTrend30dSnapshot {
        return DailyTrend30dSnapshot(
            timezone = timezone,
            generatedAt = generatedAt?.let(Instant::parse),
            startDate = startDate,
            endDate = endDate,
            totalTokens = totalTokens,
            averageTokens = averageTokens,
            peakTokens = peakTokens,
            estimatedValueUsd = estimatedValueUsd,
            estimatedValueLabel = estimatedValueLabel,
            days = days.map { it.toDomain() },
        )
    }

    companion object {
        fun fromDomain(snapshot: DailyTrend30dSnapshot): SnapshotDailyTrend30dCacheDto {
            return SnapshotDailyTrend30dCacheDto(
                timezone = snapshot.timezone,
                generatedAt = snapshot.generatedAt?.toString(),
                startDate = snapshot.startDate,
                endDate = snapshot.endDate,
                totalTokens = snapshot.totalTokens,
                averageTokens = snapshot.averageTokens,
                peakTokens = snapshot.peakTokens,
                estimatedValueUsd = snapshot.estimatedValueUsd,
                estimatedValueLabel = snapshot.estimatedValueLabel,
                days = snapshot.days.map(SnapshotDailyTrendDayCacheDto::fromDomain),
            )
        }
    }
}

@Serializable
private data class SnapshotDailyTrendDayCacheDto(
    val date: String,
    val totalTokens: Long,
) {
    fun toDomain(): DailyTrendDay {
        return DailyTrendDay(date = date, totalTokens = totalTokens)
    }

    companion object {
        fun fromDomain(day: DailyTrendDay): SnapshotDailyTrendDayCacheDto {
            return SnapshotDailyTrendDayCacheDto(date = day.date, totalTokens = day.totalTokens)
        }
    }
}

@Serializable
private data class SessionCacheDto(
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
    val contextCompaction: ContextCompactionCacheDto? = null,
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
        fun fromDomain(snapshot: SessionSnapshot): SessionCacheDto {
            return SessionCacheDto(
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
                contextCompaction = snapshot.contextCompaction?.let(ContextCompactionCacheDto::fromDomain),
                lastActiveAgoMinutes = snapshot.lastActiveAgoMinutes,
            )
        }
    }
}

@Serializable
private data class ContextCompactionCacheDto(
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
        fun fromDomain(compaction: ContextCompactionSnapshot): ContextCompactionCacheDto {
            return ContextCompactionCacheDto(
                trigger = compaction.trigger,
                startedAt = compaction.startedAt?.toString(),
                updatedAt = compaction.updatedAt?.toString(),
                turnId = compaction.turnId,
            )
        }
    }
}

private fun String.toQuotaStatus(): QuotaStatus {
    return when (trim().lowercase()) {
        "ok" -> QuotaStatus.Ok
        "stale" -> QuotaStatus.Stale
        else -> QuotaStatus.Unavailable
    }
}

private fun QuotaStatus.toStorageValue(): String {
    return when (this) {
        QuotaStatus.Ok -> "ok"
        QuotaStatus.Stale -> "stale"
        QuotaStatus.Unavailable -> "unavailable"
    }
}
