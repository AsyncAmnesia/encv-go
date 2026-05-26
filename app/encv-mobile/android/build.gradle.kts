import java.util.Properties

plugins {
    id("org.jetbrains.kotlin.android") version "2.1.0" apply false
    id("org.jetbrains.kotlin.plugin.compose") version "2.1.0" apply false
    id("com.android.application") version "8.13.0" apply false
    id("com.android.library") version "8.13.0" apply false
    alias(libs.plugins.combolite.aar2apk)
}

apply(from = "variables.gradle")

val localProps = Properties().apply {
    val f = rootProject.file("local.properties")
    if (f.exists()) load(f.inputStream())
}

val keystorePath = localProps.getProperty("aar2apk.keystorePath")
    ?: System.getenv("AAR2APK_KEYSTORE_PATH")
    ?: rootProject.file("../keystore/release.jks").absolutePath
val keystorePassword = localProps.getProperty("aar2apk.keystorePassword")
    ?: System.getenv("AAR2APK_KEYSTORE_PASSWORD")
    ?: "encv2025"
val keyAlias = localProps.getProperty("aar2apk.keyAlias")
    ?: System.getenv("AAR2APK_KEY_ALIAS")
    ?: "encvrelease"
val keyPassword = localProps.getProperty("aar2apk.keyPassword")
    ?: System.getenv("AAR2APK_KEY_PASSWORD")
    ?: "encv2025"

aar2apk {
    modules {
        module(":plugin-mpv-player")
    }
    signing {
        keystorePath.set(keystorePath)
        keystorePassword.set(keystorePassword)
        keyAlias.set(keyAlias)
        keyPassword.set(keyPassword)
    }
}

tasks.register<Delete>("clean") {
    delete(rootProject.layout.buildDirectory)
}
