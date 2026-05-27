# 修复 CI 构建通过但应用启动闪退

## CI 构建结果分析

### ✅ 编译阶段 — 全部通过

| Step | 状态 | 说明 |
|------|------|------|
| 23 Build MPV player plugin (Kotlin) | ✅ BUILD SUCCESSFUL | Icons import 已修复 |
| 24 Package MPV plugin APK | ✅ plugin-mpv-player-debug.apk (1.7MB) | 含 libmpv.so/libplayer.so 等 |
| 26 Build Debug APK | ✅ assembleDebug 成功 | encv-go-debug.apk (46MB) |
| 27 Verify APK contents | ✅ 通过 | Go二进制/MPV分离/plugin APK 均正常 |

### ❌ 运行时 — 启动即崩溃（用户反馈）

CI 日志无 logcat 输出（CI 只编译不运行），需从代码层面排查。

---

## 根因定位：`EncvApplication` 类缺失（高概率）

### 证据链

```
AndroidManifest.xml (android/app/src/main/)
  L18: android:name=".EncvApplication"    ← 声明了 Application 子类

settings.gradle.kts
  L34-36: include(:app), include(:capacitor-cordova-android-plugins), include(:plugin-mpv-player)
          ↑ 没有包含 :android-overlay！

文件实际位置：
  android-overlay/app/src/main/java/com/encvgo/app/EncvApplication.kt  ← 在未包含的模块中！
```

**结果**：编译时 Manifest 引用的类不在 classpath 中 → 但编译器只检查 XML 中引用的类是否在 DEX 中能找到（延迟绑定），**不会报编译错误**。运行时 Android Framework 尝试实例化 `EncvApplication` → **`java.lang.ClassNotFoundException: com.encvgo.app.EncvApplication`** → 进程立即终止。

### 为什么之前可能没崩？

- 如果 `android-overlay` 曾经被手动复制到 `android/` 目录，或者有构建脚本做 copy 操作，则类存在
- 或者之前用 `cap sync` / `cap build` 时 Capacitor 可能处理了这个问题
- 当前 CI 流程用的是 `./gradlew assembleDebug`（L312），没有触发任何 overlay 合并逻辑

---

## 修复方案

### 方案 A（推荐）：将 EncvApplication 复制到主模块

将 `android-overlay/app/src/main/java/com/encvgo/app/EncvApplication.kt` 复制到 `android/app/src/main/java/com/encvgo/app/EncvApplication.kt`。

这是最小改动——该类只是一个空的 Application 子类：

```kotlin
class EncvApplication : Application() {
    override fun onCreate() {
        super.onCreate()
    }
}
```

### 方案 B（备选）：移除 Manifest 中的 application name

如果不需要自定义 Application 类，从 `AndroidManifest.xml` 删除 `android:name=".EncvApplication"`。

风险：如果未来需要在 Application.onCreate() 中做初始化（如 LeakCanary、SDK 初始化），需要重新加回来。

### 排查其他潜在问题（按优先级）

#### 1. GoProcessPlugin.kt L184 的 `!!` 非空断言

```kotlin
PlayerEntry.play(context ?: activity!!, filePath!!, name!!, mimeType!!)
```

`call.getString("filePath", "")` 返回 `String?`。虽然默认值是 `""`（非空），但 Kotlin 类型系统仍标记为可空。
- **影响范围**：仅在 JS 调用 `openPlayer()` 时触发，不影响启动
- **风险等级**：低（有 try-catch 包裹）

#### 2. Kotlin 2.3.21 + Compose BOM 兼容性

当前 Compose BOM 版本需要确认是否支持 Kotlin 2.3.21。

检查 `libs.versions.toml` 中的 compose bom 版本。

#### 3. Gradle 9 + AGP 8.13 运行时行为变化

Gradle 9 对 R8/ProGuard、资源压缩等可能有行为差异。当前 debug 构建 `isMinifyEnabled = false`，不受 R8 影响。

---

## 执行步骤

### Step 1: 复制 EncvApplication.kt 到主 android 模块

```bash
cp android-overlay/app/src/main/java/com/encvgo/app/EncvApplication.kt \
   android/app/src/main/java/com/encvgo/app/EncvApplication.kt
```

### Step 2: 验证 libs.versions.toml 中 Compose BOM 版本与 Kotlin 2.3.21 的兼容性

确认 compose BOM 版本 ≥ 2024.06.00（支持 Kotlin 2.x）。

### Step 3: 清理日志文件

```bash
rm -rf /workspace/job_logs /workspace/job_logs.zip
```

---

## 验证方式

修复后推送到 CI：
1. 编译应继续通过
2. 用户安装 debug APK 后不再闪退
