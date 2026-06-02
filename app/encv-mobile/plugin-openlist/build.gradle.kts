plugins {
    id("com.android.library")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
    alias(libs.plugins.combolite.aar2apk)
}

android {
    namespace = "com.encvgo.plugin.openlist"
    compileSdk = libs.versions.compileSdk.get().toInt()

    defaultConfig {
        minSdk = libs.versions.minSdk.get().toInt()
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            isShrinkResources = false
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_21
        targetCompatibility = JavaVersion.VERSION_21
    }

    buildFeatures {
        compose = true
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.fromTarget("21"))
    }
}

repositories {
    flatDir {
        dirs("libs")
    }
}

dependencies {
    compileOnly(libs.combolite.core)

    // Local AAR must be consumed via a repository (flatDir), not as
    // `files(...)` — AGP rejects direct local .aar dependencies in
    // library modules because the resulting AAR would not bundle the
    // inner AAR's classes/resources. With flatDir, the group is empty
    // (no Maven group) and ext="aar" selects the local .aar artifact.
    implementation(group = "", name = "openlist", ext = "aar")
    implementation("androidx.core:core-ktx")
    implementation("androidx.localbroadcastmanager:localbroadcastmanager:1.1.0")
    implementation(platform(libs.compose.bom))
    implementation(libs.compose.ui)
    implementation(libs.compose.runtime)
    implementation(libs.compose.material3)
    compileOnly("io.insert-koin:koin-core:4.1.0")
}
