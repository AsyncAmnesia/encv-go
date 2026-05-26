plugins {
    id("org.jetbrains.kotlin.android") version "2.1.0" apply false
    id("com.android.application") version "8.13.0" apply false
    id("com.android.library") version "8.13.0" apply false
    alias(libs.plugins.combolite.aar2apk)
}

apply(from = "variables.gradle")

aar2apk {
    modules {
        module(":plugin-mpv-player")
    }
}

tasks.register<Delete>("clean") {
    delete(rootProject.layout.buildDirectory)
}
