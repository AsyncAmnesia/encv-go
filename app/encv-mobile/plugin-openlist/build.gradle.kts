// plugin-openlist/build.gradle.kts
//
// 架构决策依据（combolite-core 2.0.2 真实源码分析）：
// - IPluginEntryClass.Content() 是 @Composable 接口方法 → 强依赖 compose 编译期插件
// - PluginLifecycleManager.kt:224 pluginClassLoader.parent = host's classloader
//   → host 提供的 deps (combolite-core / core-ktx / compose-ui) 用 compileOnly 即可
// - 插件使用 Service + ContentProvider + LocalBroadcastManager，不使用 Material UI
//   → 不同于 plugin-mpv-player 的「Material3 + icons + appcompat」全套；不锁镜
//
// 依赖分类（与 plugin-mpv-player 不同的部分会高亮）：
//   compileOnly:  host 已提供，插件不打包（combolite-core / core-ktx / koin-core 类型）
//   implementation: host 未提供，插件必须打包（localbroadcastmanager / openlist-classes.jar / compose-ui）
plugins {
    id("com.android.library")
    id("org.jetbrains.kotlin.android")
    // ⚠️ 强契约: IPluginEntryClass.Content() 是 @Composable → 必须开启 Compose 编译期插件
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
            // ComboLite 强约束: kotlin-reflect @Metadata 不可被 R8 破坏
            // (ProxyManager / PluginLifecycleManager 用反射读 @Metadata)
            isMinifyEnabled = false
            isShrinkResources = false  // AGP 强约束: shrinkResources 必须配 minify
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_21
        targetCompatibility = JavaVersion.VERSION_21
    }

    // ⚠️ 强契约: @Composable Content() 编译期需要
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
    // ComboLite API 接口: host 的 com.combo.core.api.IPluginEntryClass / IPluginActivity /
    // IPluginService / IPluginReceiver / PluginContext / LoadedPluginInfo 等
    // 都在 combolite-core 里；host 已 implementation(libs.combolite-core)
    // 插件通过 parent classloader 拿到，compileOnly 即可
    compileOnly(libs.combolite.core)

    // gomobile 产物: 由 CI step "Extract OpenList AAR for plugin packaging" 把
    // openlist.aar/classes.jar 拷到这里。AGP 规则: 真正的 .jar 依赖会被
    // 打进 AAR 的 libs/（不像 .aar 依赖不会进），aar2apk 进一步打进最终 APK。
    // 不能 compileOnly — OpenListBridge 在运行时调 Openlistlib.* 静态方法。
    implementation(files("libs/openlist-classes.jar"))

    // Compose 编译期 + 运行时: IPluginEntryClass.Content() 是 @Composable，
    // OpenListEmbedWebView 用了 androidx.compose.ui.viewinterop.AndroidView +
    // androidx.compose.ui.platform.LocalContext + androidx.compose.foundation.layout.fillMaxSize。
    // host 已 implementation(platform(libs.compose.bom) + libs.compose.ui)，
    // 但我们用 implementation 而不是 compileOnly —— 因为 host 用了 BOM 2024.06.00，
    // 而 plugins 可能被独立加载调试；implementation 让插件自包含。
    // ⚠️ 不引 material3 / icons / activity-compose / appcompat —— OpenListEmbedWebView
    // 只是一段 Composable + AndroidView,不需要 Material 主题（锁镜 MPV 的陷阱）
    // ⚠️ Phase 14 修复：必须加 compose.foundation + compose.foundation.layout
    // (OpenListEmbedWebView.kt:38 用 Modifier.fillMaxSize() 来自 foundation.layout)
    // (OpenListPluginEntry.kt 之前误用了 Box/fillMaxSize 已清理掉,这里仅 OpenListEmbedWebView 需要)
    implementation(platform(libs.compose.bom))
    implementation(libs.compose.ui)
    implementation(libs.compose.foundation)         // Box / Column / Row 等基础 widget
    implementation(libs.compose.foundation.layout) // fillMaxSize / padding / ColumnScope 等

    // Koin: IPluginEntryClass.pluginModule: List<Module> 的 Module 类型
    // 来自 org.koin.core.module.Module。OpenListPluginEntry.pluginModule = emptyList()
    // 不会实际用 Koin runtime,compileOnly 就够。
    // host 启动 Koin (PluginManager.startKoin 见 combolite-core/PluginManager.kt:43)
    compileOnly("io.insert-koin:koin-core:4.1.0")

    // LocalBroadcastManager: 桥接 Bridge ↔ Service 状态变化
    // (OpenListBridge.kt:380, OpenListService.kt:136/212)
    // host 没有这个包,必须 implementation
    implementation("androidx.localbroadcastmanager:localbroadcastmanager:1.1.0")

    // NotificationCompat: OpenListService.createNotificationChannel 用
    // host 已 implementation("androidx.core:core-ktx:1.17.0") (app/build.gradle.kts:125),
    // 插件 ClassLoader 走 parent → host → 拿到,compileOnly 就够
    compileOnly("androidx.core:core-ktx")
}
