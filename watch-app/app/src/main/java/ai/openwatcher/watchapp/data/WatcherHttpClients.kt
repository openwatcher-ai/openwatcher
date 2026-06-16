package ai.openwatcher.watchapp.data

import java.util.concurrent.TimeUnit
import okhttp3.OkHttpClient

object WatcherHttpClients {
    fun createDefaultClient(): OkHttpClient = OkHttpClient()

    fun createSessionStreamClient(base: OkHttpClient): OkHttpClient {
        return base.newBuilder()
            .readTimeout(0, TimeUnit.MILLISECONDS)
            .build()
    }
}
