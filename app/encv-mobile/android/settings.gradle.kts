pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.PREFER_SETTINGS)
    repositories {
        google()
        mavenCentral()
        maven { url = uri("https://jitpack.io") }
    }
}

rootProject.name = "encv-mobile"

include(":app")
include(":capacitor-cordova-android-plugins")
include(":plugin-mpv-player")

project(":capacitor-cordova-android-plugins").projectDir = file("./capacitor-cordova-android-plugins/")

apply(from = "capacitor.settings.gradle")
