# CI 构建修复计划 — packagePlugins 闭包内 FQCN 解析失败

## 问题分析

CI 错误（`app/build.gradle` line 65）：

```
Could not get unknown property 'io' for extension 'packagePlugins' of type com.combo.aar2apk.PackagePluginsExtension.
```

**根因**：`packagePlugins` 闭包内使用了 Java 完全限定类名 `io.github.combolite.core.build.PackageBuildType.DEBUG`，但 Groovy DSL 的闭包作用域中，`io` 被当作 `packagePlugins` 扩展的属性查找，而不是 Java 包名前缀。

**ComboLite 官方示例**（`app/build.gradle.kts`）的正确做法：
```kotlin
import com.combo.aar2apk.PackageBuildType  // 顶部 import

packagePlugins {
    buildType.set(PackageBuildType.RELEASE)  // 使用短名
}
```

注意：官方示例的 `PackageBuildType` 包名是 `com.combo.aar2apk.PackageBuildType`，不是我们之前写的 `io.github.combolite.core.build.PackageBuildType`。

## 修复步骤

### 步骤 1：修改 `app/build.gradle`

**文件**：`/workspace/app/encv-mobile/android/app/build.gradle`

1. 在文件顶部添加 import 语句
2. 修改 `packagePlugins` 块中的 `buildType` 引用，使用 import 后的短名

修改前：
```groovy
plugins {
    id 'kotlin-android'
}

apply plugin: 'com.android.application'
apply plugin: 'io.github.lnzz123.combolite-aar2apk'

// ... android { ... } ...

packagePlugins {
    enabled.set(true)
    buildType.set(io.github.combolite.core.build.PackageBuildType.DEBUG)
    pluginsDir.set("plugins")
}
```

修改后：
```groovy
import com.combo.aar2apk.PackageBuildType

plugins {
    id 'kotlin-android'
}

apply plugin: 'com.android.application'
apply plugin: 'io.github.lnzz123.combolite-aar2apk'

// ... android { ... } ...

packagePlugins {
    enabled.set(true)
    buildType.set(PackageBuildType.DEBUG)
    pluginsDir.set("plugins")
}
```

### 步骤 2：删除 job_logs/ 和 job_logs.zip

按用户要求，修复完成后删除：
- `/workspace/job_logs/` 目录
- `/workspace/job_logs.zip` 文件

## 影响范围

- 仅修改 `app/build.gradle` 文件
- 添加 import + 修改 buildType 引用方式
- 不涉及 Kotlin/Go/前端代码变更
- CI 工作流无需修改

## 验证

修复后 CI 应能：
1. 成功解析 `packagePlugins` DSL 块
2. 成功编译 `:plugin-mpv-player:compileDebugKotlin`
3. 成功执行 `:plugin-mpv-player:buildDebugPluginApk`
4. 成功构建 Debug APK
