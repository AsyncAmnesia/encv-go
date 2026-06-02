pluginManagement {
    repositories {
        mavenCentral()
        google()
        gradlePluginPortal()
        maven { url = uri("https://plugins.gradle.org/m2/") }
        maven { url = uri("https://maven.aliyun.com/repository/google") }
        maven { url = uri("https://maven.aliyun.com/repository/central") }
        maven { url = uri("https://maven.aliyun.com/repository/gradle-plugin") }
        maven { url = uri("https://maven.aliyun.com/repository/public") }
        maven { url = uri("https://mirrors.tencent.com/nexus/repository/maven-tencent/") }
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.PREFER_PROJECT)
    repositories {
        mavenCentral()
        maven { url = uri("https://maven.aliyun.com/repository/google") }
        if (System.getenv("CI") == null) {
            maven { url = uri("https://mirrors.tencent.com/nexus/repository/maven-public/") }
        }
        maven { url = uri("https://mirrors.tencent.com/repository/maven-tencent/") }
        maven { url = uri("https://maven.aliyun.com/repository/public") }
        google()
        maven { url = uri("https://jitpack.io") }
        flatDir {
            dirs("${rootProject.projectDir}/capacitor-cordova-android-plugins/src/main/libs", "${rootProject.projectDir}/app/libs")
        }
    }
}

rootProject.name = "encv-mobile"

include(":app")
include(":capacitor-cordova-android-plugins")
include(":plugin-mpv-player")
include(":plugin-openlist")
include(":combolite-host")

project(":capacitor-cordova-android-plugins").projectDir = file("./capacitor-cordova-android-plugins/")
project(":plugin-mpv-player").projectDir = file("../plugin-mpv-player")
project(":plugin-openlist").projectDir = file("../plugin-openlist")

apply(from = "capacitor.settings.gradle")
