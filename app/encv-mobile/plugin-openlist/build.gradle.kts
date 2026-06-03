plugins {
    id("com.android.library")
    id("org.jetbrains.kotlin.android")
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

    // Phase 0 修复: 删除 buildFeatures { compose = true }
    // Content() 改为嵌入式 WebView，不再需要 Compose Material3
}

kotlin {
    compilerOptions {
        jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.fromTarget("21"))
    }
}

dependencies {
    compileOnly(libs.combolite.core)

    // openlist-classes.jar is extracted from openlist.aar by the CI step
    // "Extract OpenList AAR for plugin packaging". Unlike .aar dependencies,
    // regular .jar deps ARE bundled into the library AAR by AGP, so aar2apk
    // can package the gomobile bindings (openlistlib.*) and libgojni.so
    // (copied to src/main/jniLibs/) into the final plugin APK.
    implementation(files("libs/openlist-classes.jar"))
    implementation("androidx.core:core-ktx")
    implementation("androidx.localbroadcastmanager:localbroadcastmanager:1.1.0")
    compileOnly("io.insert-koin:koin-core:4.1.0")

    // Phase 0 修复: 移除所有 compose 依赖
    // - implementation(platform(libs.compose.bom))
    // - implementation(libs.compose.ui)
    // - implementation(libs.compose.runtime)
    // - implementation(libs.compose.material3)
    // - implementation("androidx.compose.material:material-icons-extended")
    // - implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.4")
    // Content() 现在用 AndroidView(WebView) 仅需 compose runtime 最小子集，
    // 但 runtime 子集仍由 androidx.compose.runtime 提供，已被 core-ktx 等间接引入。
}
