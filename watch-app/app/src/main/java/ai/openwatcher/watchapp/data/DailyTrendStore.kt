package ai.openwatcher.watchapp.data

import java.time.Instant
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

interface DailyTrendStore {
    fun read(): DailyTrend30dSnapshot?
    fun write(snapshot: DailyTrend30dSnapshot)
    fun clear()
}

class DailyTrendPreferenceStore(
    private val store: KeyValueStore,
    private val json: Json = Json { ignoreUnknownKeys = true; explicitNulls = false },
) : DailyTrendStore {
    override fun read(): DailyTrend30dSnapshot? {
        val raw = store.getString(KEY_DAILY_TREND_30D).orEmpty().trim()
        if (raw.isBlank()) {
            return null
        }
        return runCatching {
            json.decodeFromString<DailyTrend30dCacheDto>(raw).toDomain()
        }.getOrNull()
    }

    override fun write(snapshot: DailyTrend30dSnapshot) {
        val encoded = json.encodeToString(DailyTrend30dCacheDto.fromDomain(snapshot))
        store.putString(KEY_DAILY_TREND_30D, encoded)
    }

    override fun clear() {
        store.remove(KEY_DAILY_TREND_30D)
    }

    companion object {
        private const val KEY_DAILY_TREND_30D = "daily_trend_30d"
    }
}

@Serializable
private data class DailyTrend30dCacheDto(
    val timezone: String,
    val generatedAt: String? = null,
    val startDate: String,
    val endDate: String,
    val totalTokens: Long,
    val averageTokens: Long,
    val peakTokens: Long,
    val estimatedValueUsd: Double? = null,
    val estimatedValueLabel: String? = null,
    val days: List<DailyTrendDayCacheDto>,
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
            days = days.map { DailyTrendDay(date = it.date, totalTokens = it.totalTokens) },
        )
    }

    companion object {
        fun fromDomain(snapshot: DailyTrend30dSnapshot): DailyTrend30dCacheDto {
            return DailyTrend30dCacheDto(
                timezone = snapshot.timezone,
                generatedAt = snapshot.generatedAt?.toString(),
                startDate = snapshot.startDate,
                endDate = snapshot.endDate,
                totalTokens = snapshot.totalTokens,
                averageTokens = snapshot.averageTokens,
                peakTokens = snapshot.peakTokens,
                estimatedValueUsd = snapshot.estimatedValueUsd,
                estimatedValueLabel = snapshot.estimatedValueLabel,
                days = snapshot.days.map { DailyTrendDayCacheDto(date = it.date, totalTokens = it.totalTokens) },
            )
        }
    }
}

@Serializable
private data class DailyTrendDayCacheDto(
    val date: String,
    val totalTokens: Long,
)
