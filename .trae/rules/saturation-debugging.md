# 饱和调试规则（Saturation Debugging）

> 核心思想：**不猜问题，让运行时暴露问题。** 一次构建，饱和覆盖，收集足够信息。
> CI 构建是宝贵资源，没时间一个个猜。

---

## 一、铁律

### 1.1 不猜问题

遇到 bug 时，**禁止**凭直觉修改代码然后"试试看"。必须先加诊断入口，让运行时告诉你问题在哪。

**❌ 错误**：看到安装卡住 → 猜是广播问题 → 改广播标志 → 试了不行 → 再猜 → 再改
**✅ 正确**：看到安装卡住 → 加诊断按钮覆盖安装全链路 → 运行一次 → 从日志定位根因

### 1.2 一次构建饱和覆盖

所有诊断代码必须一次性加入，一次 CI 构建验证所有路径。不要分多轮提交。

### 1.3 每个诊断弹窗必须有复制按钮

诊断信息必须可复制，否则用户只能截图，无法有效反馈。

- Ionic alertController 的 handler 必须 `return false` 防止点击复制后弹窗关闭
- 复制内容使用 `navigator.clipboard.writeText()`
- 复制成功/失败都要有 toast 提示

### 1.4 应用内日志缓冲

Android 14+ 应用没有 `READ_LOGS` 权限，`logcat` 命令可能返回空。必须维护应用内日志缓冲：

- `GoProcessPlugin.appLogBuffer`：ConcurrentLinkedQueue，最大 3000 条
- `GoProcessPlugin.appLog(level, tag, msg)`：同时写 Log.x() 和缓冲
- `exportLogs()` 优先使用 appLogBuffer，logcat 作为补充

---

## 二、诊断流程

### 2.1 标准流程

```
发现 Bug
  → 在相关页面加诊断按钮（@PluginMethod / 前端按钮）
  → 诊断按钮覆盖全链路，每步记录状态
  → 一次构建 → 用户运行 → 从诊断日志定位根因
  → 修复根因 → 移除或保留诊断按钮
```

### 2.2 诊断入口模板

**Android @PluginMethod**：
```kotlin
@PluginMethod
fun debugXxxFlow(call: PluginCall) {
    val steps = mutableListOf<String>()
    val results = JSObject()

    steps.add("=== 关键组件状态 ===")
    // 检查所有相关组件是否初始化
    steps.add("1. XxxManager.isInitialized = ...")

    steps.add("=== CapacitorPlugin requestCodes ===")
    // 检查 requestCodes 是否正确声明
    val annotation = this.javaClass.getAnnotation(CapacitorPlugin::class.java)
    steps.add("2. requestCodes = ${annotation?.requestCodes?.toList()}")

    steps.add("=== 关键路径可达性 ===")
    // 检查 Activity/Service 是否可解析
    // 检查文件是否存在
    // 检查权限是否授予

    results.put("debugLog", steps.joinToString("\n"))
    call.resolve(results)
}
```

**前端诊断按钮**：
```vue
<ion-button @click="handleDebugXxx" color="warning" size="small">
  🔧 饱和调试：测试XXX流程
</ion-button>
```

**前端诊断弹窗**：
```typescript
const alert = await alertController.create({
  header: '🔧 XXX流程诊断',
  message: `<pre style="...">${debugText}</pre>`,
  buttons: [
    {
      text: '复制',
      handler: async () => {
        await navigator.clipboard.writeText(debugText)
        showToast({ message: '已复制', duration: 1500, color: 'success' })
        return false  // ← 关键！防止弹窗关闭
      },
    },
    'OK',
  ],
})
```

---

## 三、常见陷阱（本项目已踩过的坑）

### 3.1 Capacitor @CapacitorPlugin requestCodes

**问题**：Capacitor Bridge 的 `onActivityResult` 通过 `getPluginWithRequestCode(requestCode)` 查找处理插件。如果 `@CapacitorPlugin` 注解没有声明 `requestCodes`，`handleOnActivityResult` **永远不会被调用**。

**症状**：`startActivityForResult` 启动的 Activity 返回结果后，插件收不到回调，Promise 永远 pending。

**修复**：
```kotlin
@CapacitorPlugin(
    name = "GoProcess",
    requestCodes = [REQUEST_CODE_PLUGIN_PICK, REQUEST_CODE_INSTALL_CONFIRM]  // ← 必须！
)
```

**诊断**：在 `debugInstallFlow()` 中检查：
```kotlin
val annotation = this.javaClass.getAnnotation(CapacitorPlugin::class.java)
val codes = annotation?.requestCodes?.toList()
steps.add("requestCodes = $codes ← CRITICAL: must include your codes")
```

### 3.2 BroadcastReceiver RECEIVER_NOT_EXPORTED vs RECEIVER_EXPORTED

**问题**：Android 13+ (API 33) 上，`RECEIVER_NOT_EXPORTED` 的接收器**不能接收隐式 Intent 广播**，即使来自同一应用。

**修复方案 A**（推荐）：用 `startActivityForResult` 替代 BroadcastReceiver，彻底消除广播丢失风险。

**修复方案 B**：使用 `RECEIVER_EXPORTED` + `setPackage(packageName)` 使广播变为定向。

### 3.3 FLAG_ACTIVITY_NEW_TASK

**问题**：从非 Activity Context（Application/Service/BroadcastReceiver）启动 Activity 必须添加 `FLAG_ACTIVITY_NEW_TASK`。

**Capacitor 特殊情况**：Plugin 的 `context` 属性可能是 Application Context，不一定是 Activity。使用 `activity` 属性获取 Activity 引用。

### 3.4 logcat 权限

**问题**：Android 14+ 普通应用没有 `READ_LOGS` 权限，`Runtime.exec("logcat")` 可能返回空。

**修复**：
1. 使用 `--pid=<PID>` 过滤本进程日志
2. 维护应用内日志缓冲（appLogBuffer）作为主要日志源
3. logcat 作为补充，空时标注原因

### 3.5 Ionic alertController 复制按钮关闭弹窗

**问题**：Ionic alertController 的 button handler 如果不返回 `false`，点击后弹窗会关闭。

**修复**：handler 末尾加 `return false`。

---

## 四、日志导出规范

### 4.1 导出内容

| 文件 | 来源 | 说明 |
|------|------|------|
| `app_log_<ts>.txt` | GoProcessPlugin.appLogBuffer | 应用内日志缓冲（主要） |
| `logcat_<ts>.txt` | `logcat -d --pid=<PID>` | 系统日志（补充，可能为空） |
| `go_backend/go_backend_<ts>.txt` | EncvGoService.getOutputSnapshot() | Go 后端输出 |
| `frontend/devlogs.json` | getFrontendLogsJson() | 前端 DevLogs |
| `device_info_<ts>.txt` | 运行时收集 | 设备/应用状态 |

### 4.2 关键方法

- `GoProcessPlugin.appLog(level, tag, msg)` — 写应用内日志
- `EncvGoService.getOutputSnapshot()` — 获取 Go 后端输出
- `getFrontendLogsJson()` — 获取前端 DevLogs JSON
- `saveDevLogs(logs)` — 保存前端日志到文件（exportLogs 前调用）

---

## 五、本项目已实现的诊断入口

| 入口 | 位置 | 覆盖范围 |
|------|------|----------|
| `debugInstallFlow()` | GoProcessPlugin | PluginManager 状态、requestCodes、Activity 解析、APK 文件、权限 |
| `🔧 饱和调试：测试安装流程` | ExtensionsPage.vue | 调用 debugInstallFlow() 并显示结果 |
| `exportLogs()` | GoProcessPlugin | app_log + logcat + go_backend + devlogs + device_info |
| `saveDevLogs()` | GoProcessPlugin | 保存前端 DevLogs 到文件 |
