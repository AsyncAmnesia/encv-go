plugins {
    id("com.android.library")
    id("org.jetbrains.kotlin.android")
    // Phase 13 架构修复: Content() 是 @Composable 接口，AndroidView 宿主需要 Compose 编译期插件
    // 之前 Phase 0 误以为删除 buildFeatures.compose = true 就能"去 Compose"——但插件入口的 Content()
    // 是 @Composable，必须打开 Compose 编译。删除只会让 build 报"Unresolved reference: Composable"。
    // 正确做法：参考 plugin-mpv-player/build.gradle.kts 的同款 Compose 配置。
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

    // Phase 13: 与 plugin-mpv-player 保持一致，启用 Compose
    buildFeatures {
        compose = true
    }
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

    // Phase 13 架构修复:
    // 1) androidx.core:core-ktx 必须用 compileOnly（无版本），让 AGP/Compose-BOM 隐式解析，
    //    与 plugin-mpv-player 完全一致——之前写 implementation + 漏版本 → "Could not find androidx.core:core-ktx:"。
    // 2) androidx.localbroadcastmanager 保持 implementation（OpenListBridge 需广播状态给 web 端）。
    // 3) 以下 Compose 依赖参考 plugin-mpv-player 的最小必需集合：
    //    - BOM 提供一致版本
    //    - compose.ui + material3 给 AndroidView 宿主使用
    //    - activity-compose / appcompat 给宿主 BasePluginActivity 用
    //    - compose runtime 由 material3 间接拉入
    implementation(platform(libs.compose.bom))
    implementation(libs.compose.ui)
    implementation(libs.compose.material3)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.appcompat)
    implementation("androidx.localbroadcastmanager:localbroadcastmanager:1.1.0")
    compileOnly("androidx.core:core-ktx")
    compileOnly("androidx.compose.material:material-icons-extended")
    compileOnly("io.insert-koin:koin-core:4.1.0")
}
