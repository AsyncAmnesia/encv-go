# Android 构建系统规则

> 来自 CI 实战踩坑 + Gradle 插件解析机制分析。

---

## 一、依赖仓库顺序铁律（违反 = 构建失败）

> **关键认知**：Gradle 仓库解析采用「短路求值」策略——按配置顺序逐个搜索，命中即停止。因此**源的可信度必须与排列优先级成正比**。

### 1.1 `pluginManagement` 仓库顺序

**核心原则：官方源优先，镜像源兜底。**

Gradle 解析 plugin 时按 `pluginManagement.repositories` 列表**顺序搜索**，找到第一个匹配即停止。阿里云镜像无法代理以下源的元数据格式：

| 源 | 阿里云能否代理 | 说明 |
|----|--------------|------|
| `google()` | ✅ 能 | Android 库（AndroidX 等） |
| `mavenCentral()` | ⚠️ 部分 | 标准库（kotlin-reflect 等），但版本可能滞后 |
| **`gradlePluginPortal()`** | **❌ 不能** | **Plugin Marker POM 格式不同** |
| **`plugins.gradle.org/m2/`** | N/A | Plugin Portal 的直接 Maven 仓库 |

**✅ 正确配置**：
```kotlin
// settings.gradle.kts
pluginManagement {
    repositories {
        mavenCentral()           // ① 标准 Maven Central
        google()                 // ② Android 官方
        gradlePluginPortal()     // ③ Gradle 插件门户（必须靠前！）
        maven { url = uri("https://plugins.gradle.org/m2/") }  // ④ 直接 URL fallback
        maven { url = uri("https://maven.aliyun.com/repository/google") }
        maven { url = uri("https://maven.aliyun.com/repository/central") }
        maven { url = uri("https://maven.aliyun.com/repository/gradle-plugin") }
        maven { url = uri("https://maven.aliyun.com/repository/public") }
        maven { url = uri("https://mirrors.tencent.com/nexus/repository/maven-tencent/") }
    }
}
```

**❌ 错误配置**：
```kotlin
// gradlePluginPortal() 放在末尾 → 阿里云先返回空 → 可能超时
pluginManagement {
    repositories {
        mavenCentral()
        maven { url = uri("https://maven.aliyun.com/repository/google") }  // 先搜镜像
        maven { url = uri("https://maven.aliyun.com/repository/gradle-plugin") }  // 无法代理 plugin portal
        // ... 更多镜像 ...
        google()              // ← 太晚！
        gradlePluginPortal()  // ← 最晚！镜像已耗尽超时预算
    }
}
```

### 1.2 `dependencyResolutionManagement` 仓库顺序

与 `pluginManagement` 类似，但额外注意：

```kotlin
dependencyResolutionManagement {
    repositories {
        google()                    // AndroidX / Google 库
        mavenCentral()              // Kotlin stdlib / kotlin-reflect / 第三方库
        maven { url = uri("https://jitpack.io") }  // ComboLite 等发布在 JitPack 的库
        maven { url = uri("https://maven.aliyun.com/repository/google") }
        if (System.getenv("CI") == null) {
            maven { url = uri("https://mirrors.tencent.com/nexus/repository/maven-public/") }
        }
        maven { url = uri("https://mirrors.tencent.com/repository/maven-tencent/") }
        maven { url = uri("https://maven.aliyun.com/repository/public") }
        mavenCentral()
    }
}
```

### 1.3 为什么阿里云不能代理 Gradle Plugin Portal

Gradle Plugin Portal 使用特殊的 **Plugin Marker POM** 格式：

```
请求: io.github.lnzz123:combolite-aar2apk:1.1.1
  → Portal 返回 Marker POM:
    <groupId>io.github.lnzz123</groupId>
    <artifactId>combolite-aar2apk</artifactId>
    <version>1.1.1</version>
    → 其中 <dependencies> 指向实际插件:
      <groupId>io.github.lnzz123</groupId>
      <artifactId>combolite-aar2apk.gradle.plugin</artifactId>
      <version>1.1.1</version>

阿里云 gradle-plugin 镜像:
  → 只缓存标准 Maven 坐标 (group:artifact:version)
  → 不理解 Plugin Marker POM 的间接引用机制
  → 返回 404 或空结果
```

### 1.4 ComboLite 依赖坐标参考

| 依赖 | Group ID | Artifact ID | 版本 | 仓库 |
|------|----------|-------------|------|------|
| combolite-core | `io.github.lnzz123` | `combolite-core` | 2.0.2+ | Maven Central |
| aar2apk (Gradle Plugin) | `io.github.lnzz123.combolite-aar2apk` | (plugin id) | 1.1.1 | Gradle Plugin Portal (`plugins.gradle.org`) |

---

## 二、版本管理

### 2.1 当前依赖版本（libs.versions.toml）

| 依赖 | 版本 | 用途 |
|------|------|------|
| combolite (core) | 2.0.2 | ComboLite 核心 runtime |
| combolite-aar2apk | 1.1.1 | AAR→APK 转换 Gradle 插件 |

### 2.2 升级注意事项

- **combolite-core 升级时必须同步检查**：新版本是否引入了新的 `::function.javaMethod` 使用点（需要 R8 保持禁用）
- **aar2apk 插件升级时必须检查**：是否引入了新的 buildType 或 DSL 变更
- **两者版本独立**：core 和 aar2apk 有独立的版本号体系，不需要保持一致

---

## 三、AGP 构建选项约束

### 3.1 isMinifyEnabled 与 isShrinkResources 硬耦合

**AGP（Android Gradle Plugin）在配置阶段强制检查：`isShrinkResources=true` 必须配合 `isMinifyEnabled=true`。**

```kotlin
// AGP 源码: AndroidResourcesCreationConfigImpl.kt:91
if (!buildType.isMinifyEnabled && androidResources.shrink) {
    issueReporter.reportError(
        "Removing unused resources requires unused code shrinking to be turned on."
    )
}
```

**原因**：ResourceShrinker 的依赖图分析需要 R8/ProGuard 先生成完整的类→资源映射文件。

**对本项目的影响**：ComboLite 要求 `isMinifyEnabled=false`（R8 破坏 kotlin-reflect @Metadata），因此 `isShrinkResources` **也必须为 false**。两者无法独立配置。

| 配置 | isMinifyEnabled | isShrinkResources | 结果 |
|------|----------------|-------------------|------|
| A | `false` | `false` | ✅ 正常（本项目必须用此） |
| B | `true` | `true` | ❌ ComboLite 崩溃（@Metadata 被破坏） |
| C | `false` | `true` | ❌ AGP EvalException（CI 实测确认） |
| D | `true` | `false` | ⚠️ 技术可行但无意义（代码 shrink 不 shrink resource） |

### 错误 D：「单独开启 isShrinkResources」

> **症状**：`EvalIssueException: Removing unused resources requires unused code shrinking to be turned on.`
> **根因**：AGP 源码级硬约束，无法绕过
> **修复**：两者同时为 `false`（ComboLite 项目），或同时为 `true`（非 ComboLite 项目）

---

## 四、常见错误模式

### 错误 A：「pluginManagement 中 gradlePluginPortal 放在末尾」

> **症状**：CI 构建 `Plugin [id: 'xxx', version: 'x.y.z'] was not found in any of the following sources`
> **根因**：阿里云镜像先被搜索且返回空/超时，gradlePluginPortal 来不及 fallback
> **修复**：将 `google()` 和 `gradlePluginPortal()` 移到列表前 3 位

### 错误 B：「遗漏 jitpack.io」

> **症状**：某些第三方库（如 ComboLite）在 dependencyResolutionManagement 中找不到
> **根因**：ComboLite 发布在 JitPack 上，不在 Maven Central
> **修复**：`dependencyResolutionManagement.repositories` 中添加 `maven { url = uri("https://jitpack.io") }`

### 错误 C：「find path 排除 build.gradle.kts 文件」

> **症状**：CI guard 从未真正检查过构建配置文件
> **根因**：`find ... -not -path "*build*"` 把 `build.gradle.kts` 也排除了（文件名含 "build"）
> **修复**：使用 `-not -path "*/build/*"` （只排除目录）
