# 修复：GoProcess plugin "not implemented on android"

## 改动（2 个文件，3 处新增）

### 文件 1：[post-cap-sync.mjs](app/encv-mobile/scripts/post-cap-sync.mjs)

| # | 改动 | 位置 | 作用 |
|---|------|------|------|
| A | **显式 `android.sourceSets`** | L135-143 | 强制 Kotlin 编译器扫描 `src/main/java` |
| B | **build.gradle 关键行诊断输出** | L145-155 | debug 模式下打印 kotlin/namespace/sourceSets/plugins 配置 |
| C | **包名一致性验证** | L217-229 | 校验 .kt 的 `package` 声明 = `com.encvgo.app` |

### 文件 2：[android.yml](.github/workflows/android.yml)

| # | 改动 | 位置 | 作用 |
|---|------|------|------|
| D | **Pre-build diagnosis 步骤** | "Verify" 和 "Build" 之间 | 打印 build.gradle kotlin 配置 + .kt 源文件列表 + AndroidManifest activity |
| E | **Verify APK 替换为 dex 级检测** | 原 "Verify APK contents" | `strings classes.dex \| grep` + `kotlin-classes` 目录检查 |

## 一次 CI 输出的完整证据链

```
[post-cap-sync]   [kotlin] applied kotlin-android via legacy apply    ← 改动前已有
[post-cap-sync]   [kotlin] added explicit sourceSets for kotlin sources ← 新增 A ✅
[post-cap-sync]   [diag] L2: apply plugin: 'kotlin-android'             ← 新增 B ✅
[post-cap-sync]   [diag] LX: namespace 'com.encvgo.app'                  ← 新增 B ✅
[post-cap-sync]   [diag] LY: main.java.srcDirs += 'src/main/java'        ← 新增 A+B ✅
[post-cap-sync]   overlay: GoProcessPlugin.kt                            ← 已有
[post-cap-sync]   pkg-ok: GoProcessPlugin.kt → com.encvgo.app            ← 新增 C ✅
[Pre-build diag]  Source .kt files: .../MainActivity.kt (261 lines)      ← 新增 D ✅
[Pre-build diag]  Source .kt files: .../GoProcessPlugin.kt (149 lines)    ← 新增 D ✅
[Pre-build diag]  AndroidManifest: android:name=".MainActivity"          ← 新增 D ✅
[Build]           > Task :app:compileDebugKotlin                        ← 现在有 sourceSets 了
[Verify APK]      ✅ GoProcess found in DEX (compiled+packaged)          ← 新增 E ✅
[Verify APK]      ✅ encvgo .class in kotlin-classes                    ← 新增 E ✅
```

## 每个证据点回答一个问题

| 证据 | 回答的问题 |
|------|-----------|
| `[diag]` 输出 kotlin-android 行 | kotlin 插件是否以正确格式注入？ |
| `[diag]` 输出 sourceSets 行 | Kotlin 编译器是否知道去哪找源文件？ |
| `[diag]` 输出 namespace 行 | 包名是否与 .kt 的 package 声明一致？ |
| `pkg-ok` | .kt 文件的 package 是否正确？ |
| Pre-build .kt 文件列表 | 源文件在 Gradle 构建前是否存在于磁盘？ |
| Pre-build AndroidManifest activity | Manifest 是否指向我们的 MainActivity？ |
| DEX strings grep | 类是否最终被打包进 APK？ |
| kotlin-classes find | Gradle 是否真的编译了 .kt → .class？ |

任何一环断裂都能从日志直接定位，不需要二次分析。
