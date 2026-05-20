# 修复：Go 后端二进制启动失败（多角度）

## 当前现象

```
[13:47:05] Backend ready: {"port":0,"error":"stopped"}     ← restart 先 stop
[13:47:11] ERROR GoProcess.restart() failed: {}              ← start 失败
[13:47:11] Backend ready: {"port":0,"error":"start_failed"}  ← notifyFrontend 发出
[13:47:11] Backend ready: {"port":0,"error":"timeout"}       ← 上次残留的 waitForBackend 线程
[13:47:39] Backend ready: {"port":0,"error":"timeout"}       ← 同上
```

## 多角度分析

### 角度 1：错误信息被吞掉 🔴 最关键

**问题链路**：
```
MainActivity.catch → notifyFrontend(0, "start_failed:IOException:xxx")  ← 有详情!
                    ↓ (CustomEvent 异步)
GoProcessPlugin.restart → readyCallback(-1) → call.reject("Backend failed to start")  ← 硬编码! 丢掉详情!
                    ↓
GoProcess.ts → catch(e) → console.log("failed:", e)  → 输出 {}
```

`call.reject("Backend failed to start")` 是硬编码字符串，MainActivity 的真实异常信息完全丢失。

**修复**：GoProcessPlugin.kt 的回调需要携带错误信息。

### 角度 2：ProcessBuilder.start() 到底抛了什么异常？

最可能的异常类型：

| 异常 | 含义 | 概率 |
|------|------|------|
| `IOException: No such file or directory` | `/system/bin/sh` 不存在或路径错 | 低 |
| `IOException: Permission denied` | SELinux 阻止执行 | 中 |
| `IOException: Exec format error` | 二进制文件损坏或架构不匹配 | 中 |
| `SecurityException` | 安全管理器阻止 | 极低 |

**但当前看不到具体异常！** 必须先修角度 1 才能确认。

### 角度 3：Go 进程输出未捕获到前端

MainActivity 已有进程输出读取线程（L95-105），Logcat 应该有 `[go]` 前缀的日志。如果 Logcat 能看到这些日志，说明进程启动了但立即崩溃。

### 角度 4：二进制提取完整性

23MB 文件通过 `assets.open().copyTo()` 提取。可能的问题：
- AssetManager 对大文件的读取可靠性
- 提取后的文件权限在 Android 10+ 的实际效果
- `canExecute()` 返回 true 但实际无法执行（SELinux 绕过 Java API）

### 角度 5：配置文件问题

`config.mobile.json` 从 assets 复制到 `filesDir/config.user.json`。如果格式不匹配 Go 后端期望，进程可能启动后立即退出非零。

---

## 修复方案（3 个文件，按优先级排序）

### 改动 1：GoProcessPlugin.kt — 错误信息透传（最高优先级）

**目的**：让前端的 `{}` 变成真实的异常信息。

```kotlin
// 原来 (L34-43):
mainActivity.restartGoDaemon { port ->
    if (port > 0) {
        val result = JSObject()
        result.put("success", true)
        result.put("port", port)
        call.resolve(result)
    } else {
        call.reject("Backend failed to start")   // ← 硬编码！
    }
}

// 修复：
var lastError: String? = null
mainActivity.restartGoDaemon { port ->
    if (port > 0) {
        val result = JSObject()
        result.put("success", true)
        result.put("port", port)
        call.resolve(result)
    } else {
        val msg = mainActivity.lastStartError ?: "Backend failed to start"
        call.reject(msg)   // ← 携带真实错误！
    }
}
```

MainActivity 新增属性：
```kotlin
var lastStartError: String? = null  // 在 companion object 或类级别
```

startGoDaemon catch 块中设置：
```kotlin
lastStartError = "start_failed:$errMsg"
notifyFrontend(0, lastStartError!!)
```

### 改动 2：MainActivity.kt — 增强诊断 + 进程退出检测

**A) 进程退出码捕获**

在输出读取线程中增加退出码等待和日志：

```kotlin
Thread {
    try {
        val reader = BufferedReader(InputStreamReader(goProcess?.inputStream))
        var line: String?
        while (reader.readLine().also { line = it } != null) {
            Log.i(TAG, "[go] $line")
        }
        // 进程结束，获取退出码
        val exitCode = goProcess?.waitFor() ?: -1
        Log.w(TAG, "[go] Process exited with code: $exitCode")
        if (exitCode != 0) {
            Log.e(TAG, "[go] Non-zero exit means crash/bad config")
        }
    } catch (e: Exception) {
        Log.w(TAG, "Error reading process output", e)
    }
}.start()
```

**B) 二进制提取后校验**

copyBinaryFromAssets 末尾加校验：

```kotlin
Log.i(TAG, "Copied Go binary to ${dest.absolutePath} (${dest.length()} bytes)")
if (dest.length() < 1000000) {  // < 1MB 肯定不对
    Log.e(TAG, "Binary too small! Expected ~23MB, got ${dest.length()}")
}
```

**C) 启动前打印完整环境信息**

startGoDaemon 中 ProcessBuilder.start() 前：

```kotlin
Log.i(TAG, "=== Launch environment ===")
Log.i(TAG, "binary=${binary.absolutePath}")
Log.i(TAG, "size=${binary.length()}")
Log.i(TAG, "canExec=${binary.canExecute()}")
Log.i(TAG, "configPath=$configPath")
Log.i(TAG, "workDir=${filesDir.absolutePath}")
Log.i(TAG, "SDK_INT=${Build.VERSION.SDK_INT}, ABI=${Build.SUPPORTED_ABIS.joinToString(",")}")
```

### 改动 3：GoProcess.ts — 前端错误日志增强

```kotlin
export async function restartBackend(): Promise<GoProcessResult> {
  try {
    return await GoProcess.restart()
  } catch (e: any) {
    // e.message 现在会包含真实的异常信息（改动 1 修复后）
    console.error('[ENCV] GoProcess.restart() failed:', e?.message || e)
    return { success: false }
  }
}
```

---

## 执行后预期效果

跑一次 CI + 安装 APK 后，前端日志应从：
```
ERROR GoProcess.restart() failed: {}
```
变为类似：
```
ERROR GoProcess.restart() failed: start_failed:java.io.IOException: Cannot run program "/system/bin/sh": ...
```

或：
```
ERROR GoProcess.restart() failed: start_failed:java.io.IOException: Error=13, Permission denied
```

有了具体的异常信息，就能精准定位是 SELinux、路径、架构还是其他问题。
