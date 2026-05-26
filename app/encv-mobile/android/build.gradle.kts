plugins {
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
