package ai.openwatcher.watchapp

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.wear.ongoing.OngoingActivity
import androidx.wear.ongoing.Status
import ai.openwatcher.watchapp.ui.AppScreen
import ai.openwatcher.watchapp.ui.AppUiState

class WearVisibilityController(
    private val context: Context,
) {
    private val notificationManager = NotificationManagerCompat.from(context)

    fun update(state: AppUiState) {
        if (state.screen != AppScreen.Dashboard) {
            cancel()
            return
        }
        ensureChannel()
        if (!notificationManager.areNotificationsEnabled()) {
            return
        }

        val activityIntent = Intent(context, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_NEW_TASK
        }
        val pendingIntent = PendingIntent.getActivity(
            context,
            0,
            activityIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )

        val dashboard = state.dashboard
        val text = buildString {
            append(dashboard.serviceLabel)
            append(" ")
            append(dashboard.home.fiveHour.remainingPercent.toInt())
            append("%/")
            append(dashboard.home.weekly.remainingPercent.toInt())
            append("%")
        }

        val notificationBuilder = NotificationCompat.Builder(context, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_ongoing)
            .setContentTitle("OpenWatcher")
            .setContentText(text)
            .setCategory(NotificationCompat.CATEGORY_STATUS)
            .setContentIntent(pendingIntent)
            .setVisibility(NotificationCompat.VISIBILITY_PUBLIC)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setShowWhen(false)
            .setSilent(true)

        val status = Status.Builder()
            .addTemplate("#main#")
            .addPart("main", Status.TextPart(text))
            .build()

        val ongoingActivity = OngoingActivity.Builder(context, NOTIFICATION_ID, notificationBuilder)
            .setStaticIcon(R.drawable.ic_ongoing)
            .setAnimatedIcon(R.drawable.ic_ongoing)
            .setTouchIntent(pendingIntent)
            .setStatus(status)
            .build()

        ongoingActivity.apply(context)
        notificationManager.notify(NOTIFICATION_ID, notificationBuilder.build())
    }

    fun cancel() {
        notificationManager.cancel(NOTIFICATION_ID)
    }

    private fun ensureChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) {
            return
        }
        val manager = context.getSystemService(NotificationManager::class.java)
        val channel = NotificationChannel(
            CHANNEL_ID,
            "OpenWatcher 可见性",
            NotificationManager.IMPORTANCE_LOW,
        ).apply {
            description = "保持 OpenWatcher 监控页在 Wear OS 上可快速返回"
            setShowBadge(false)
        }
        manager.createNotificationChannel(channel)
    }

    companion object {
        private const val CHANNEL_ID = "openwatcher_visibility"
        private const val NOTIFICATION_ID = 1001
    }
}
