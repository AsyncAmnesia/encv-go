# 修复：Go 二进制启动失败 + Logcat 日志丢失

## 当前状态

### ✅ 已解决：GoProcess 插件注册
新 CI 日志铁证：
- `✅ encvgo .class in kotlin-classes` — GoProcessPlugin.class + MainActivity.class **编译成功**
- `pkg-ok: GoProcessPlugin.kt → com.encvgo.app` — 包名一致
- `[diag] L73: android.sourceSets {` — sourceSets 生效
- 前端不再报 "not implemented on android"，权限申请成功

### 🔴 新问题：Go 后端启动失败
前端日志：
```
[ENCV] Backend ready from native bridge: {"port":0,"error":"stopped"}
[ENCV] Backend ready from native bridge: {"port":0,"error":"start_failed"}
ERROR [ENCV] GoProcess.restart() failed: {}
```
+ 用户反馈 "logcat 日志又没了"

## 根因分析

### 问题 1：Go 二进制启动失败

MainActivity.kt 的 `startGoDaemon()` 有 3 个失败路径：

| 错误值 | 触发位置 | 含义 |
|--------|----------|------|
| `"no_binary"` | L74 | `findExecutableBinary()` 返回 null（所有位置都不可执行）|
| `"start_failed"` | L112 | `ProcessBuilder.start()` 抛异常 |
| `"stopped"` | L57 | `stopGoDaemon()` 被调用 |

用户看到 `"stopped"` + `"start_failed"` 说明 restart 流程（先 stop 再 start）中 start 阶段抛了异常。

**最可能原因**：
1. Android 10+ (API 29+) **禁止从 app 私有目录 (`filesDir`) 执行二进制文件**
2. `setExecutable(true)` 在高版本 Android 上对非 ELF 文件无效或被忽略
3. SELinux 策略阻止执行

### 问题 2：Logcat 日志丢失

debug AndroidManifest.xml 已正确生成（post-cap-sync.mjs L203-219），但 Logcat 浮动入口不显示的可能原因：
- debugImplementation 依赖的 Logcat 库可能在某些情况下未正确打包
- 设备上 Logcat 应用需要单独安装或权限

## 修复方案（3 个文件）

### 改动 1：MainActivity.kt — 增强错误诊断

目标：让每次失败都在 Logcat 中留下足够的诊断信息，即使看不到完整 Logcat 也能通过 `notifyFrontend` 传递给前端。

```kotlin
// findExecutableBinary() 中增加详细诊断
private fun findExecutableBinary(): File? {
    // ... 现有逻辑 ...

    // 最终失败时输出完整诊断到 Logcat + 通知前端
    Log.e(TAG, "=== Binary diagnosis ===")
    for ((dir, name) in candidateDirs) {
        if (dir == null) continue
        val binary = File(dir, BINARY_NAME)
        val msg = buildString {
            append("$name: exists=${binary.exists()}, ")
            append("length=${binary.length()}, ")
            append("canRead=${binary.canRead()}, ")
            append("canExecute=${binary.canExecute()}, ")
            append("path=${binary.absolutePath}")
        }
        Log.e(TAG, msg)
    }
    Log.e(TAG, "SDK_INT=$sdkInt, RELEASE=${Build.VERSION.RELEASE}")
    notifyFrontend(0, "no_binary")
    return null
}

// startGoDaemon() catch 块增强
catch (e: Exception) {
    Log.e(TAG, "=== Start daemon FAILED ===", e)
    Log.e(TAG, "Exception class: ${e.javaClass.name}")
    Log.e(TAG, "Exception message: ${e.message}")
    e.stackTraceToString().lines().forEach { Log.e(TAG, it) }
    // 将具体错误信息传给前端
    val errorMsg = e.message?.takeIf { it.length < 200 } ?: "unknown_error"
    notifyFrontend(0, "start_failed:$errorMsg")
}
```

### 改动 2：CI Verify 步骤 — 增加 Go 二进制验证

在 android.yml 的 Verify APK 步骤末尾追加：

```bash
echo "=== Go binary in APK ==="
unzip -p "$APK_PATH" assets/encv-go > /tmp/test-binary 2>/dev/null
if [ -f /tmp/test-binary ]; then
  echo "Size: $(wc -c < /tmp/test-binary) bytes"
  file /tmp/test-binary || true
  head -4 /tmp/test-binary | xxd | head -2 || true
else
  echo "❌ encv-go not found in APK assets!"
fi
```

这确认 Go 二进制确实被打包进 APK 且格式正确。

### 改动 3：MainActivity.kt — 使用 `/system/bin/sh -c` 执行绕过限制

Android 高版本禁止直接执行 filesDir 中的二进制。解决方案：通过 shell 启动。

```kotlin
// 替换原来的 pb.start()
// 原来:
// goProcess = pb.start()

// 改为:
val cmd = "${binary.absolutePath} start"
goProcess = ProcessBuilder("/system/bin/sh", "-c", cmd).apply {
    environment()["ENCV_CONFIG_PATH"] = configPath
    environment()["ENCV_MOBILE"] = "1"
    environment()["HOME"] = filesDir.absolutePath
    redirectErrorStream(true)
    directory(filesDir)  // 工作目录设为 filesDir
}.start()
```

使用 `/system/bin/sh -c` 绕过 Android 对直接执行 app 私有目录二进制的限制。
