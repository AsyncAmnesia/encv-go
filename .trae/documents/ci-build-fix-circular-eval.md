# CI 构建修复：aar2apk signing 循环引用

## 问题分析

CI 日志显示唯一的错误是：

```
Circular evaluation detected: property 'keystorePath'
   -> property 'keystorePath'
```

**根因**：`build.gradle.kts` 中顶层变量名 `keystorePath` / `keystorePassword` / `keyAlias` / `keyPassword` 与 aar2apk DSL `signing { }` 块内的同名属性冲突。

在 `signing { keystorePath.set(keystorePath) }` 中：
- `keystorePath`（左侧，DSL 属性）= aar2apk signing 的 `Property<String>` 对象
- `keystorePath`（右侧，本意引用局部变量）= 在 DSL 作用域内被解析为同一个 `Property<String>` 对象
- 结果：`property.set(property)` → Gradle 检测到循环引用

**好消息**：Kotlin 编译已全部通过（步骤 26 `BUILD SUCCESSFUL`），CI 工作流任务名也已修复（步骤 27 使用了 `convert_plugin-mpv-player_debug`）。唯一剩余问题就是这个循环引用。

## 修复步骤

### 1. 重命名 build.gradle.kts 中的局部变量

文件：`app/encv-mobile/android/build.gradle.kts`

将 `val keystorePath` → `val ksPath`，`val keystorePassword` → `val ksPassword`，`val keyAlias` → `val ksAlias`，`val keyPassword` → `val ksKeyPassword`

修改前：
```kotlin
val keystorePath = localProps.getProperty("aar2apk.keystorePath")
    ?: System.getenv("AAR2APK_KEYSTORE_PATH")
    ?: rootProject.file("../keystore/release.jks").absolutePath
val keystorePassword = localProps.getProperty("aar2apk.keystorePassword")
    ?: System.getenv("AAR2APK_KEYSTORE_PASSWORD")
    ?: "encv2025"
val keyAlias = localProps.getProperty("aar2apk.keyAlias")
    ?: System.getenv("AAR2APK_KEY_ALIAS")
    ?: "encvrelease"
val keyPassword = localProps.getProperty("aar2apk.keyPassword")
    ?: System.getenv("AAR2APK_KEY_PASSWORD")
    ?: "encv2025"

aar2apk {
    modules {
        module(":plugin-mpv-player")
    }
    signing {
        keystorePath.set(keystorePath)
        keystorePassword.set(keystorePassword)
        keyAlias.set(keyAlias)
        keyPassword.set(keyPassword)
    }
}
```

修改后：
```kotlin
val ksPath = localProps.getProperty("aar2apk.keystorePath")
    ?: System.getenv("AAR2APK_KEYSTORE_PATH")
    ?: rootProject.file("../keystore/release.jks").absolutePath
val ksPassword = localProps.getProperty("aar2apk.keystorePassword")
    ?: System.getenv("AAR2APK_KEYSTORE_PASSWORD")
    ?: "encv2025"
val ksAlias = localProps.getProperty("aar2apk.keyAlias")
    ?: System.getenv("AAR2APK_KEY_ALIAS")
    ?: "encvrelease"
val ksKeyPassword = localProps.getProperty("aar2apk.keyPassword")
    ?: System.getenv("AAR2APK_KEY_PASSWORD")
    ?: "encv2025"

aar2apk {
    modules {
        module(":plugin-mpv-player")
    }
    signing {
        keystorePath.set(ksPath)
        keystorePassword.set(ksPassword)
        keyAlias.set(ksAlias)
        keyPassword.set(ksKeyPassword)
    }
}
```

### 2. 清理日志文件

删除 `job_logs/` 目录和 `job_logs.zip`。

## 影响范围

- 仅修改 1 个文件：`app/encv-mobile/android/build.gradle.kts`
- 变更内容：4 个局部变量重命名（纯重命名，逻辑不变）
- 风险：极低，只是消除命名冲突
