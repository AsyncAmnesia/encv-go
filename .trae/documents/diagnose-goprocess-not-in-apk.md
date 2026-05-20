# 修复：Go 二进制启动失败 + Logcat 完全无日志

## 当前状态

### ✅ 已解决
- GoProcess 插件编译+注册成功（CI 铁证：✅ encvgo .class in kotlin-classes）
- 权限申请正常

### 🔴 问题 1：Logcat 完全空白（含系统日志）

**根因**：CI 的 Patch AndroidManifest 步骤权限列表中 **缺少 `android.permission.READ_LOGS`**。
Logcat 库（com.github.getActivity:Logcat）需要此权限才能读取 logcat buffer。没有它 → 悬浮窗打开后完全空白。

### 🔴 问题 2：Go 后端启动失败

前端日志 `{"port":0,"error":"start_failed"}` — ProcessBuilder.start() 抛异常。
最可能原因：Android 10+ 禁止直接执行 app 私有目录中的二进制文件。

## 修复（3 个文件）

### 改动 1：android.yml — 补回 READ_LOGS 权限

在 perms 数组中追加：

```python
('android.permission.READ_LOGS', None),
```

### 改动 2：MainActivity.kt — 两处修改

**A) findExecutableBinary 失败时输出完整诊断到 notifyFrontend**

```kotlin
// 在 return null 前加:
for ((dir, name) in candidateDirs) {
    if (dir == null) continue
    val binary = File(dir, BINARY_NAME)
    Log.e(TAG, "Binary $name: exists=${binary.exists()}, len=${binary.length()}, exec=${binary.canExecute()}")
}
notifyFrontend(0, "no_binary")
```

**B) startGoDaemon 中用 `/system/bin/sh -c` 替代直接启动**

```kotlin
// 原来:
goProcess = pb.start()

// 改为:
val cmd = "${binary.absolutePath} start"
goProcess = ProcessBuilder("/system/bin/sh", "-c", cmd).apply {
    environment()["ENCV_CONFIG_PATH"] = configPath
    environment()["ENCV_MOBILE"] = "1"
    environment()["HOME"] = filesDir.absolutePath
    redirectErrorStream(true)
    directory(filesDir)
}.start()
```

**C) catch 块将具体异常信息传给前端**

```kotlin
catch (e: Exception) {
    Log.e(TAG, "Start daemon FAILED", e)
    e.stackTraceToString().lines().forEach { Log.e(TAG, it) }
    notifyFrontend(0, "start_failed:${e.message?.take(200) ?: 'unknown'}")
}
```

### 改动 3：android.yml Verify 步骤 — 增加 Go 二进制验证

Verify APK 步骤末尾追加：

```bash
echo "=== Go binary in APK ==="
unzip -p "$APK_PATH" assets/encv-go > /tmp/test-binary 2>/dev/null
if [ -f /tmp/test-binary ]; then
  echo "Size: $(wc -c < /tmp/test-binary) bytes"
  file /tmp/test-binary || true
else
  echo "❌ encv-go not in APK!"
fi
```
