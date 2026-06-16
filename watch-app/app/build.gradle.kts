import org.gradle.api.GradleException
import java.net.URI
import java.util.Properties

val releaseSigningPropsFile = rootProject.layout.projectDirectory.file("signing/release.properties").asFile
val releaseSigningProps = Properties().apply {
    if (releaseSigningPropsFile.exists()) {
        releaseSigningPropsFile.inputStream().use(::load)
    }
}
val releaseTasksRequested = gradle.startParameter.taskNames.any { it.contains("release", ignoreCase = true) }
val hasReleaseSigning = releaseSigningPropsFile.exists()
val openWatcherPublicWebsiteHosts = setOf("openwatcher.ai", "www.openwatcher.ai")
val releaseWatcherBaseUrl = providers.gradleProperty("openWatcherReleaseBaseUrl")
    .orElse(providers.gradleProperty("openWatcherBaseUrl"))
    .orElse("https://127.0.0.1.invalid")
val debugWatcherBaseUrl = providers.gradleProperty("openWatcherDebugBaseUrl")
    .orElse("http://10.0.2.2:18787")
val watchBootstrapBaseUrl = providers.gradleProperty("openWatcherBootstrapBaseUrl")
    .orElse("https://api.worker.openwatcher.ai")
val usesCleartextTraffic = providers.gradleProperty("usesCleartextTraffic").orElse("false")

fun gitOutput(vararg args: String): String {
    return try {
        val command = listOf("git") + args
        val process = ProcessBuilder(command)
            .directory(rootProject.projectDir)
            .redirectErrorStream(true)
            .start()
        val output = process.inputStream.bufferedReader().readText().trim()
        if (process.waitFor() == 0) output else ""
    } catch (_: Exception) {
        ""
    }
}

val explicitWatchVersionName = providers.gradleProperty("openWatcherVersionName")
    .orElse(providers.environmentVariable("OPENWATCHER_WATCH_VERSION_NAME"))
    .orNull
    ?.trim()
    .orEmpty()
val explicitWatchVersionCode = providers.gradleProperty("openWatcherVersionCode")
    .orElse(providers.environmentVariable("OPENWATCHER_WATCH_VERSION_CODE"))
    .orNull
    ?.trim()
    .orEmpty()

if (releaseTasksRequested && (explicitWatchVersionName.isBlank() || explicitWatchVersionCode.isBlank())) {
    throw GradleException("release 构建必须通过 -PopenWatcherVersionName/-PopenWatcherVersionCode 或 OPENWATCHER_WATCH_VERSION_NAME/OPENWATCHER_WATCH_VERSION_CODE 传入版本。")
}

if (explicitWatchVersionName.isNotBlank() && !Regex("""^[0-9]+\.[0-9]+\.[0-9]+([-.+][0-9A-Za-z][0-9A-Za-z.+-]*)?$""").matches(explicitWatchVersionName)) {
    throw GradleException("openWatcherVersionName 不符合版本规则：$explicitWatchVersionName")
}

val parsedExplicitWatchVersionCode = explicitWatchVersionCode.toIntOrNull()
if (explicitWatchVersionCode.isNotBlank() && (parsedExplicitWatchVersionCode == null || parsedExplicitWatchVersionCode <= 0)) {
    throw GradleException("openWatcherVersionCode 必须是正整数：$explicitWatchVersionCode")
}

val resolvedWatchVersionName = explicitWatchVersionName.ifBlank {
    "dev-${gitOutput("rev-parse", "--short", "HEAD").ifBlank { "local" }}"
}
val resolvedWatchVersionCode = parsedExplicitWatchVersionCode ?: (
    gitOutput("rev-list", "--count", "HEAD").toIntOrNull()?.takeIf { it > 0 } ?: 1
)

fun String.asBuildConfigString(): String {
    return "\"" + replace("\\", "\\\\").replace("\"", "\\\"") + "\""
}

fun normalizedWatcherUrl(rawUrl: String): URI {
    val trimmed = rawUrl.trim().trimEnd('/')
    val uri = try {
        URI(trimmed)
    } catch (error: IllegalArgumentException) {
        throw GradleException("watcher URL 无法解析：$rawUrl", error)
    }
    val scheme = uri.scheme?.lowercase()
    if (scheme != "http" && scheme != "https") {
        throw GradleException("watcher URL 只支持 http 或 https：$rawUrl")
    }
    if (uri.host.isNullOrBlank()) {
        throw GradleException("watcher URL 缺少 host：$rawUrl")
    }
    return uri
}

fun requireDebugWatcherUrl(rawUrl: String): String {
    val uri = normalizedWatcherUrl(rawUrl)
    val host = uri.host.lowercase()
    if (host in openWatcherPublicWebsiteHosts) {
        throw GradleException("debug 构建禁止使用 OpenWatcher 公网站点：$host")
    }
    if (uri.port == 8787) {
        throw GradleException("debug 构建禁止使用生产端口 8787：$rawUrl")
    }
    return uri.toString()
}

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
    id("org.jetbrains.kotlin.plugin.serialization")
}

android {
    namespace = "ai.openwatcher.watchapp"
    compileSdk = 35
    buildToolsVersion = "35.0.0"

    defaultConfig {
        applicationId = "ai.openwatcher.watchapp"
        minSdk = 34
        targetSdk = 34
        versionCode = resolvedWatchVersionCode
        versionName = resolvedWatchVersionName

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        vectorDrawables {
            useSupportLibrary = true
        }

        manifestPlaceholders["usesCleartextTraffic"] = usesCleartextTraffic.get()
    }

    signingConfigs {
        if (hasReleaseSigning) {
            create("release") {
                val storePath = releaseSigningProps.getProperty("storeFile")
                    ?: throw GradleException("release.properties 缺少 storeFile")
                val storePasswordValue = releaseSigningProps.getProperty("storePassword")
                    ?: throw GradleException("release.properties 缺少 storePassword")
                val keyAliasValue = releaseSigningProps.getProperty("keyAlias")
                    ?: throw GradleException("release.properties 缺少 keyAlias")
                val keyPasswordValue = releaseSigningProps.getProperty("keyPassword")
                    ?: throw GradleException("release.properties 缺少 keyPassword")

                storeFile = rootProject.file(storePath)
                storePassword = storePasswordValue
                keyAlias = keyAliasValue
                keyPassword = keyPasswordValue
                enableV1Signing = true
                enableV2Signing = true
                enableV3Signing = true
            }
        }
    }

    buildTypes {
        debug {
            applicationIdSuffix = ".debug"
            versionNameSuffix = "-debug"
            buildConfigField("String", "OPENWATCHER_BASE_URL", requireDebugWatcherUrl(debugWatcherBaseUrl.get()).asBuildConfigString())
            buildConfigField("String", "OPENWATCHER_BOOTSTRAP_BASE_URL", normalizedWatcherUrl(watchBootstrapBaseUrl.get()).toString().asBuildConfigString())
            buildConfigField("String", "OPENWATCHER_BETA_UPDATE_PRIMARY_URL", "https://openwatcher.ai/channels/beta.json".asBuildConfigString())
            buildConfigField(
                "String",
                "OPENWATCHER_BETA_UPDATE_BACKUP_URL",
                "".asBuildConfigString(),
            )
            buildConfigField("boolean", "ENABLE_DEBUG_DEMO", "true")
            manifestPlaceholders["usesCleartextTraffic"] = "true"
        }
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            ndk {
                abiFilters += listOf("armeabi-v7a", "arm64-v8a")
            }
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
            buildConfigField("String", "OPENWATCHER_BASE_URL", normalizedWatcherUrl(releaseWatcherBaseUrl.get()).toString().asBuildConfigString())
            buildConfigField("String", "OPENWATCHER_BOOTSTRAP_BASE_URL", normalizedWatcherUrl(watchBootstrapBaseUrl.get()).toString().asBuildConfigString())
            buildConfigField("String", "OPENWATCHER_BETA_UPDATE_PRIMARY_URL", "https://openwatcher.ai/channels/beta.json".asBuildConfigString())
            buildConfigField(
                "String",
                "OPENWATCHER_BETA_UPDATE_BACKUP_URL",
                "".asBuildConfigString(),
            )
            buildConfigField("boolean", "ENABLE_DEBUG_DEMO", "false")
            manifestPlaceholders["usesCleartextTraffic"] = "true"
            if (hasReleaseSigning) {
                signingConfig = signingConfigs.getByName("release")
            }
        }
        create("localBeta") {
            initWith(getByName("release"))
            isDebuggable = true
            isMinifyEnabled = false
            isShrinkResources = false
            manifestPlaceholders["usesCleartextTraffic"] = "true"
            signingConfig = signingConfigs.getByName("debug")
            matchingFallbacks += listOf("release")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    packaging {
        resources {
            excludes += "/META-INF/{AL2.0,LGPL2.1}"
        }
    }

    testOptions {
        unitTests {
            isIncludeAndroidResources = false
        }
    }
}

if (releaseTasksRequested && !hasReleaseSigning) {
    throw GradleException("缺少 watch-app/signing/release.properties，禁止构建 release。")
}

dependencies {
    implementation("androidx.core:core-ktx:1.15.0")
    implementation("androidx.activity:activity-compose:1.10.1")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.7")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.7")
    implementation("androidx.compose.ui:ui:1.7.6")
    implementation("androidx.compose.foundation:foundation:1.7.6")
    implementation("androidx.compose.material3:material3:1.3.1")
    implementation("androidx.wear.compose:compose-foundation:1.4.1")
    implementation("androidx.wear:wear-ongoing:1.1.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.9.0")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("com.google.zxing:core:3.5.3")

    debugImplementation("androidx.compose.ui:ui-tooling:1.7.6")

    testImplementation("junit:junit:4.13.2")
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.9.0")
    testImplementation("androidx.arch.core:core-testing:2.2.0")
}
