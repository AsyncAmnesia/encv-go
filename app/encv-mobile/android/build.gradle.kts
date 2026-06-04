import java.util.Properties

plugins {
    id("org.jetbrains.kotlin.android") version "2.3.21" apply false
    id("org.jetbrains.kotlin.plugin.compose") version "2.3.21" apply false
    id("com.android.application") version "8.13.0" apply false
    id("com.android.library") version "8.13.0" apply false
    alias(libs.plugins.combolite.aar2apk)
}

apply(from = "variables.gradle")

val localProps = Properties().apply {
    val f = rootProject.file("local.properties")
    if (f.exists()) load(f.inputStream())
}

val ksPath = localProps.getProperty("aar2apk.keystorePath")
    ?: System.getenv("AAR2APK_KEYSTORE_PATH")
    ?: rootProject.file("../keystore/release.jks").absolutePath
val ksPassword = localProps.getProperty("aar2apk.keystorePassword")
    ?: System.getenv("AAR2APK_KEYSTORE_PASSWORD")
    ?: "encv2025"
val ksAlias = localProps.getProperty("aar2apk.keyAlias")
    ?: System.getenv("AAR2APK_KEY_ALIAS")
    ?: "encvrelease"
val ksKeyPassword = localProps.getProperty("aar2apk.keyPassword")
    ?: System.getenv("AAR2APK_KEY_PASSWORD")
    ?: "encv2025"

aar2apk {
    modules {
        // Only register modules whose projects actually exist in settings.gradle.kts.
        // When -PincludePlugins=true is NOT passed (main app builds like assembleRelease),
        // settings.gradle.kts skips include(":plugin-*"), so findProject() returns null.
        // Without this guard, aar2apk's afterEvaluate hook calls
        // evaluationDependsOn(':plugin-mpv-player') on a non-existent project → crash.
        //
        // 关键: includeDependenciesJni = true 必须打开
        // — aar2apk 默认 false (Aar2ApkExtension.kt:55-58)
        // — ConvertAarToApkTask.kt:121-129 主 aar 的 jni/ 永远会被打包,
        //   但保险起见也开 true,避免 main AAR 哪天没 jni/ 时 silently 缺 .so
        // — 不开的话 plugin APK 缺 libgojni.so,运行时 UnsatisfiedLinkError,
        //   artifact 大小会从 ~70MB (含 libgojni.so) 缩到 ~2MB (仅 dex)
        if (findProject(":plugin-mpv-player") != null) {
            module(":plugin-mpv-player") {
                includeDependenciesJni.set(true)
            }
        }
        if (findProject(":plugin-openlist") != null) {
            module(":plugin-openlist") {
                includeDependenciesJni.set(true)
            }
        }
    }
    signing {
        keystorePath.set(ksPath)
        keystorePassword.set(ksPassword)
        keyAlias.set(ksAlias)
        keyPassword.set(ksKeyPassword)
    }
}

tasks.register<Delete>("clean") {
    delete(rootProject.layout.buildDirectory)
}
