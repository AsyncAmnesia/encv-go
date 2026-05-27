# 集成 Bugly SDK（崩溃上报）

## 技术选型

| 项目 | 选择 | 理由 |
|------|------|------|
| SDK 版本 | **Bugly Pro 4.4.7.8** | 用户已设置 `BUGLY_APP_ID` + `BUGLY_APP_KEY`，Pro 版需要两者 |
| Maven 坐标 | `com.tencent.bugly:bugly-pro:4.4.7.8` | Maven Central 可用，无需额外仓库 |
| 初始化位置 | `EncvApplication.onCreate()` | 最早入口点，捕获所有崩溃 |
| Secret 注入方式 | `buildConfigField` 从环境变量读取 | CI 中 `${{ secrets.BUGLY_APP_ID }}` → 环境变量 → BuildConfig |

## 执行步骤

### Step 1: 添加版本到 libs.versions.toml

**文件**: `app/encv-mobile/android/gradle/libs.versions.toml`

```diff
 [versions]
+bugly = "4.4.7.8"

 [libraries]
+bugly-pro = { group = "com.tencent.bugly", name = "bugly-pro", version.ref = "bugly" }
```

### Step 2: 添加依赖到 app/build.gradle.kts

**文件**: `app/encv-mobile/android/app/build.gradle.kts`

```diff
 dependencies {
     ...
+    implementation(libs.bugly.pro)
 }
```

同时在 `defaultConfig` 中添加 BuildConfig 字段（从环境变量读取 Secret）：

```diff
 defaultConfig {
     ...
+    buildConfigField("String", "BUGLY_APP_ID", "\"${System.getenv("BUGLY_APP_ID") ?: ""}\"")
+    buildConfigField("String", "BUGLY_APP_KEY", "\"${System.getenv("BUGLY_APP_KEY") ?: ""}\"")
 }
```

> `buildConfig = true` 已在 L49 开启，无需额外配置。

### Step 3: 创建 EncvApplication.kt 并初始化 Bugly

**文件**: `app/encv-mobile/android/app/src/main/java/com/encvgo/app/EncvApplication.kt`（新建）

Manifest 已声明 `android:name=".EncvApplication"` 但该类只存在于 `android-overlay/` 模块中。创建此文件可同时：
1. **修复潜在的 ClassNotFoundException 启动崩溃**
2. **提供最早的 Bugly 初始化入口**

```kotlin
package com.encvgo.app

import android.app.Application
import com.tencent.bugly.Bugly
import com.tencent.bugly.crashreport.CrashReport
import android.util.Log

class EncvApplication : Application() {
    override fun onCreate() {
        super.onCreate()
        initBugly()
    }

    private fun initBugly() {
        try {
            val appId = BuildConfig.BUGLY_APP_ID
            val appKey = BuildConfig.BUGLY_APP_KEY
            if (appId.isNullOrEmpty() || appKey.isNullOrEmpty()) {
                Log.w("ENCV-Bugly", "Bugly APP_ID or APP_KEY not configured, skipping initialization")
                return
            }
            // Pro 版初始化：需要 AppID + AppKey
            // 第三个参数为 debug 模式开关（false=发布模式）
            Bugly.init(applicationContext, appId, appKey)
            Log.i("ENCV-Bugly", "Bugly initialized successfully")
        } catch (e: Exception) {
            Log.e("ENCV-Bugly", "Failed to initialize Bugly", e)
        }
    }
}
```

### Step 4: 创建 Proguard 混淆规则

**文件**: `app/encv-mobile/android/app/proguard-rules.pro`（新建，当前不存在但 release 构建引用了它）

```
# Bugly Pro — 保持类不被混淆
-dontwarn com.tencent.bugly.**
-keep public class com.tencent.bugly.**{*;}
```

### Step 5: 更新 CI Workflow 传递 Secrets

**文件**: `.github/workflows/android.yml`

在 Build Debug APK / Build Release APK 步骤之前注入环境变量：

```yaml
      - name: Build ${{ inputs.version && 'Release' || 'Debug' }} APK
        env:
          BUGLY_APP_ID: ${{ secrets.BUGLY_APP_ID }}
          BUGLY_APP_KEY: ${{ secrets.BUGLY_APP_KEY }}
        run: |
          cd app/encv-mobile
          ...
```

### Step 6: 清理日志文件

```bash
rm -rf /workspace/job_logs /workspace/job_logs.zip
```

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `android/gradle/libs.versions.toml` | 修改 | 添加 bugly 版本和库声明 |
| `android/app/build.gradle.kts` | 修改 | 添加依赖 + BuildConfig 字段 |
| `android/app/src/main/java/.../EncvApplication.kt` | **新建** | Bugly 初始化 |
| `android/app/proguard-rules.pro` | **新建** | 混淆保留规则 |
| `.github/workflows/android.yml` | 修改 | 传递 Secrets 到构建环境 |

## NDK Native Crash 支持

项目已有大量 native 代码（Go backend libencv-go.so、MPV libmpv.so、FFmpeg .so），Bugly Pro 4.4.7.8 **已内置 NDK 崩溃捕获**（从 4.0.0 起合并为单一 AAR），无需额外添加 `nativecrashreport` 依赖。

当前 `abiFilters` 已设为 `arm64-v8a`，与 native 库一致。

## 验证方式

1. CI 编译通过（新增依赖能从 Maven Central 解析）
2. 安装 APK 后不闪退（EncvApplication 正常加载）
3. 在 Bugly 控制台能看到崩溃数据上报（或手动调用 `CrashReport.testJavaCrash()` 测试）
