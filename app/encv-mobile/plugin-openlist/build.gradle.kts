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

    // Phase 25 A3.3 修复: aar2apk 任务的 localDependencyClasses 只接受
    // ProjectComponentIdentifier (Aar2ApkPlugin.kt:141) 和 remoteDependencyAars
    // 只接受 ModuleComponentIdentifier (Aar2ApkPlugin.kt:117)；
    // implementation(files("libs/openlist-classes.jar")) 是直接 file 依赖，
    // 不属于这两类，会被 Gradle dependency graph 过滤掉，aar2apk 永远拿不到
    // openlistlib 类。最终 plugin APK dex 不含 openlistlib.Event/LogCallback，
    // 运行时 NoClassDefFoundError。
    //
    // 修复: preBuild 任务把 libs/openlist-classes.jar 解压到 build/generated/
    // openlistlib/ 目录，加入 main sourceSet。这样 Kotlin/Java 编译时能找到
    // openlistlib 类，AGP 打 aar 时 classes.jar 自然含 openlistlib，aar2apk
    // 解压 aar 拿 classes.jar → d8 合并到 plugin APK dex → 运行时类能加载。
    //
    // sourceSet 注入 vs aar2apk 改造 vs 包成 .aar 走 maven coordinate:
    // - sourceSet 注入最稳，不依赖 aar2apk 内部行为，不依赖 AGP 版本
    // - 包成 .aar 走 maven 需要新 flatDir/mavenLocal 仓库配置
    // - 改 aar2apk 是第三方库，要 fork
    sourceSets {
        getByName("main") {
            java.srcDir(layout.buildDirectory.dir("generated/openlistlib"))
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

// Phase 25 A3.3 修复: 解压 libs/openlist-classes.jar → build/generated/openlistlib/
// 作为 sourceSet 的一部分参与编译（见 android.sourceSets 块）。
// 这样 aar2apk 转换 plugin-openlist.aar 时，openlistlib 类已经在 aar 的
// classes.jar 里，d8 会合并到 plugin APK dex。
//
// 为什么用 task 而不是 fileTree: 必须先解压 jar 才能让 sourceSet 看到 .class 文件。
// AGP 编译期需要 .class 文件按包结构目录布局（openlistlib/Event.class 等）。
val unpackOpenlistClasses by tasks.registering {
    val input = file("libs/openlist-classes.jar")
    val outputDir = layout.buildDirectory.dir("generated/openlistlib").get().asFile
    inputs.file(input)
    outputs.dir(outputDir)
    doLast {
        if (!input.exists()) {
            // 必须 WARN 不要 fail —— 沙箱没 jar 时正常 fail-fast 不可取
            // （CI 上 jar 一定存在，sandbox 测试编译时不存在才合理）
            logger.warn("[plugin-openlist] libs/openlist-classes.jar not found, " +
                    "skipping unpack. plugin APK will lack openlistlib classes.")
            outputDir.deleteRecursively()
            outputDir.mkdirs()
            return@doLast
        }
        outputDir.deleteRecursively()
        outputDir.mkdirs()
        copy {
            from(zipTree(input))
            into(outputDir)
        }
        logger.lifecycle("[plugin-openlist] unpacked ${input.length() / 1024}KB → " +
                "${outputDir.walkTopDown().count { it.isFile }} files into $outputDir")
    }
}

// 让所有 build type 的 preBuild 都先跑 unpackOpenlistClasses
// AGP 8.13.0: preBuild + preDebugBuild + preReleaseBuild 都存在
// configureEach 比 configure 范围更广（debug/release 都能 hook）
afterEvaluate {
    tasks.matching { it.name.startsWith("pre") && it.name.endsWith("Build") }
        .configureEach {
            dependsOn(unpackOpenlistClasses)
        }
}
