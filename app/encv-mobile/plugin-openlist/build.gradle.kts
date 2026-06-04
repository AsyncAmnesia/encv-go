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
import org.gradle.api.GradleException

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

    // Phase 25 A3.3 修复（重写版）: aar2apk 任务的 localDependencyClasses 只接受
    // ProjectComponentIdentifier (Aar2ApkPlugin.kt:141) 和 remoteDependencyAars
    // 只接受 ModuleComponentIdentifier (Aar2ApkPlugin.kt:117)；
    // implementation(files("libs/openlist-classes.jar")) 是直接 file 依赖，
    // 不属于这两类，会被 Gradle dependency graph 过滤掉，aar2apk 永远拿不到
    // openlistlib 类。最终 plugin APK dex 不含 openlistlib.Event/LogCallback，
    // 运行时 NoClassDefFoundError。
    //
    // ❌ 旧版 A3.3 方案：把 .class 文件解到 build/generated/openlistlib/ 加到
    // sourceSet.main.java.srcDir —— **完全错**：AGP 的 java 编译只处理
    // .java/.kt 源文件，.class 文件被静默忽略，kotlinc 输出 classes 不含
    // openlistlib 类，bundleReleaseAar 打包的 aar/classes.jar 也不含，
    // aar2apk 拿到这个空 aar，d8 用 "DEX打包: 仅包含主模块代码" 模式只 d8
    // 2 个 jar（main_aar/classes.jar + r_classes.jar），plugin APK dex
    // 不含 openlistlib 类。runtime NoClassDefFoundError (CI 2026-06 验证)。
    //
    // ✅ 新版 A3.3 方案：unpackOpenlistClasses 解 jar → build/generated/openlistlib/，
    // **不在 sourceSet**，而是新增 injectOpenlistClassesToAar 任务：
    //   1. 解 aar → tmp 目录
    //   2. 解 aar/classes.jar + 合并 openlistlib classes
    //   3. 重打 aar/classes.jar
    //   4. 重打 aar
    //   5. 替换 outputs/aar/...aar
    // 让 aar2apk 拿到含 openlistlib 类的 aar → d8 合并到 plugin APK dex。
    //
    // 为什么不用 stub 写 openlistlib 类的 .java 源：gomobile 产物带 native
    // 方法调 libgojni.so，stub 类缺 native 实现，运行时 OpenList 永远不起。
    // 必须是 gomobile 产物的真 .class 文件（带 native 绑定）。
    //
    // 为什么不用 fileTree 合并到 kotlinc 输出目录：AGP 8.13 内部路径是
    // build/tmp/kotlin-classes/release/，但 AGP 用增量构建 transform 任务
    // 写这个目录，直接复制会被 AGP 自己的增量检测器误判。改 aar 字节是
    // 离 AGP 内部最远、最稳的方案。

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

// Phase 25 A3.3 修复（重写版 v2）: 解压 libs/openlist-classes.jar →
// build/generated/openlistlib/，**不**进 sourceSet（AGP 不编译 .class）。
// openlistlib 类的注入由 injectOpenlistClassesToAar 任务在 bundleReleaseAar
// 之后手动 merge 到 aar/classes.jar 完成（见下文）。
//
// unpack 任务仍然必需：injectOpenlistClassesToAar 任务读取这个目录。
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

// Phase 25 A3.3 v2: 在 bundleReleaseAar 之后手动 inject openlistlib classes
// 到 aar/classes.jar。这样 aar2apk 拿到的 aar 里 classes.jar 含 openlistlib
// 真 .class（带 native 方法调 libgojni.so），d8 合并到 plugin APK dex，
// 运行时 plugin classloader 能 resolve openlistlib.Event / LogCallback。
//
// 实现步骤（每个 variant 一次）:
//   1. 解 aar → build/tmp/aar-inject-<variant>/
//   2. 解 aar/classes.jar → build/tmp/classes-merged-<variant>/
//   3. 复制 build/generated/openlistlib/ → 上述目录（覆盖同名/追加新类）
//   4. 重打 classes-merged-<variant>.jar 替换 aar/classes.jar
//   5. 重打 aar-new-<variant>.aar 替换 outputs/aar/...aar
val injectOpenlistClassesToAar by tasks.registering {
    val generatedClasses = layout.buildDirectory.dir("generated/openlistlib")
    inputs.dir(generatedClasses)
    // 不声明 outputs.file(aar) 因为我们 in-place 修改 aar，Gradle 8.13 允许
    // 通过 upToDateWhen 自定义 up-to-date 逻辑避免 input==output 警告。
    doLast {
        val src = generatedClasses.get().asFile
        if (!src.exists() || src.walkTopDown().filter { it.isFile }.toList().isEmpty()) {
            logger.warn("[plugin-openlist] injectOpenlistClassesToAar skipped: " +
                    "no openlistlib classes in $src (libs/openlist-classes.jar missing?)")
            return@doLast
        }

        listOf("release", "debug").forEach { variant ->
            val aarFile = layout.buildDirectory.file("outputs/aar/plugin-openlist-${variant}.aar")
                .get().asFile
            if (!aarFile.exists()) {
                logger.lifecycle("[plugin-openlist] injectOpenlistClassesToAar: " +
                        "skip $variant (aar not built yet)")
                return@forEach
            }

            val tmpAarDir = file("$buildDir/tmp/aar-inject-${variant}")
            val tmpClassesDir = file("$buildDir/tmp/classes-merged-${variant}")
            tmpAarDir.deleteRecursively()
            tmpAarDir.mkdirs()
            tmpClassesDir.deleteRecursively()
            tmpClassesDir.mkdirs()

            // 1. 解 aar
            copy { from(zipTree(aarFile)); into(tmpAarDir) }

            // 2+3. 解原 classes.jar + 复制 openlistlib classes
            // ⚠️ 不能用 `copy { from(src); into(tmpClassesDir) }` —— Gradle
            // CopySpec.from(File) 会把 src 当作子目录,导致 dst/src/... 错位
            // (Python 测试时同样的 bug 在 verify 看到 0 com/encvgo/ 才暴露)。
            // 必须遍历 src 下的 entry 复制到 dst 根。
            val originalClassesJar = File(tmpAarDir, "classes.jar")
            if (originalClassesJar.exists()) {
                copy { from(zipTree(originalClassesJar)); into(tmpClassesDir) }
            }
            val openlistClassCount = src.walkTopDown().count { it.isFile && it.name.endsWith(".class") }
            src.walkTopDown().filter { it.isFile }.forEach { srcFile ->
                val target = File(tmpClassesDir, srcFile.relativeTo(src).path)
                target.parentFile?.mkdirs()
                srcFile.copyTo(target, overwrite = true)
            }

            // 4. 重打 classes.jar —— 用系统 `jar` 命令 (JDK 自带,无需 java.util.zip import)
            // 为什么不用 java.util.zip.ZipOutputStream:
            //   - Gradle build script 默认不 import java.*,FQN 写法在 build script 上下文解析失败
            //   - 即使加 import,也是 hack —— 正常 APK 构建根本不会事后重打包任何东西
            // 为什么不用 Gradle Jar 任务:
            //   - Jar 任务要 register 在 config time,不能 inside doLast 循环
            //   - forEach variant 不支持
            // 妥协方案: ProcessBuilder 直接调用系统 jar (JDK 工具链必有)
            val mergedClassesJar = file("$buildDir/tmp/classes-merged-${variant}.jar")
            runJar(tmpClassesDir, mergedClassesJar)
            if (!mergedClassesJar.exists() || mergedClassesJar.length() == 0L) {
                throw GradleException("[plugin-openlist] failed to repack classes.jar for $variant")
            }
            originalClassesJar.delete()
            mergedClassesJar.copyTo(originalClassesJar)

            // 5. 重打 aar —— 同样用系统 jar
            val newAar = file("$buildDir/tmp/aar-new-${variant}.aar")
            runJar(tmpAarDir, newAar)
            if (!newAar.exists() || newAar.length() == 0L) {
                throw GradleException("[plugin-openlist] failed to repack aar for $variant")
            }
            aarFile.delete()
            newAar.copyTo(aarFile)

            logger.lifecycle("[plugin-openlist] injected $openlistClassCount openlistlib " +
                    "classes into $variant aar (new size: ${aarFile.length() / 1024}KB)")

            // 清理 tmp
            tmpAarDir.deleteRecursively()
            tmpClassesDir.deleteRecursively()
            newAar.delete()
            mergedClassesJar.delete()
        }
    }
}

/**
 * 用系统 `jar` 命令 (JDK 自带) 重新打包一个目录成 jar/aar。
 *
 * 为什么不直接用 java.util.zip.ZipOutputStream:
 *   - Gradle build script 默认不 import java.*,即使 FQN 写 java.util.zip.X
 *     也会被 kotlinc 报 "Unresolved reference 'util'"
 *   - 加 import 是 hack,正常 APK 构建根本不写 java.util.zip
 *
 * 为什么不直接用 Gradle Jar 任务:
 *   - Jar 任务必须在 config time register,不能 inside forEach variant doLast 循环
 *
 * 妥协方案: ProcessBuilder 调用系统 jar (JDK 工具链必有,$JAVA_HOME/bin/jar)
 *   - 不需要任何 import (ProcessBuilder 在 java.lang,自动 import)
 *   - jar cf output -C dir . 语义清晰,所有 .class 文件进 output
 */
fun runJar(sourceDir: File, outputJar: File) {
    outputJar.delete()
    val proc = ProcessBuilder(
        "jar", "cf", outputJar.absolutePath, "-C", sourceDir.absolutePath, "."
    ).redirectErrorStream(true).start()
    val stderr = proc.inputStream.bufferedReader().readText()
    val exit = proc.waitFor()
    if (exit != 0) {
        throw GradleException(
            "[plugin-openlist] jar repack failed (exit=$exit): $stderr"
        )
    }
    if (!outputJar.exists() || outputJar.length() == 0L) {
        throw GradleException(
            "[plugin-openlist] jar repack produced no output: $outputJar"
        )
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

    // injectOpenlistClassesToAar 必须在 convert_plugin-openlist_* 之前跑
    // (aar2apk 任务 dependsOn(:plugin-openlist:assembleRelease/Debug) →
    // :plugin-openlist:bundleReleaseAar → aar2apk 读 aar)
    //
    // ⚠️ 任务名: aar2apk Aar2ApkPlugin.kt:82-86
    //   baseTaskName = modulePath.replace(":", "_").removePrefix("_")
    //   modulePath = ":plugin-openlist" → replace 后 "_plugin-openlist"
    //   removePrefix("_") 移除一个下划线 → "plugin-openlist" (保留连字符!)
    //   taskName = "convert_${baseTaskName}_${buildType}" = "convert_plugin-openlist_release"
    // (注意:是连字符不是下划线,modulePath 里的 '-' 原样保留)
    //
    // 我们 hook 在 convert_* 之前最稳:让 convert_* dependsOn inject 任务
    // 这样 aar 被修改后 aar2apk 才能读到含 openlistlib 的版本
    tasks.matching { it.name.startsWith("convert_plugin-openlist_") }
        .configureEach {
            dependsOn(injectOpenlistClassesToAar)
        }
}
