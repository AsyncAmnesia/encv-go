# 修复计划：Lynx NativeModules + Admin 404 + 后台服务优化

## 问题 1（最关键）：Lynx NativeModules 访问方式错误

### 根因分析

通过深入学习 Lynx 官方文档和 ReactLynx 源码，发现以下关键事实：

**Lynx ReactLynx 双线程架构：**
- **主线程（Main Thread）**：负责 React 组件渲染和 JSX 求值
- **后台线程（Background Thread）**：管理 effects、事件处理和原生模块调用

**NativeModules 访问规则（Lynx 官方文档明确说明）：**
1. `NativeModules` 是 Lynx 在 JavaScript 运行时中提供的**全局内建对象**
2. 通过 `declare let NativeModules: { ... }` 在 `typing.d.ts` 中声明
3. 原生模块调用**不能在主线程渲染阶段**进行（会导致 UI 阻塞）
4. 原生模块调用**只能在后台线程**的以下上下文中使用：
   - `useEffect` / `useLayoutEffect` hooks
   - 事件处理函数（bindtap 等）
   - 使用 `'background only'` 指令标记的函数
   - `useImperativeHandle` 实现、Ref 回调

**当前代码的 3 个错误：**

1. **使用 `globalThis.NativeModules` 而非 `NativeModules`**：
   - 在 Lepus 线程中，`globalThis.NativeModules` 被显式设为 `undefined`（env.js 第 31-34 行）
   - Lynx 官方文档示例使用 `NativeModules.XXX` 直接访问，通过 `declare let NativeModules` 声明
   - 在后台线程上下文中，`NativeModules` 作为全局内建对象可用

2. **在组件顶层（主线程渲染阶段）调用 NativeModules**：
   - `lynxLog` 对象在模块顶层定义，其方法调用 `globalThis.NativeModules.LogBridgeModule.log()`
   - 第 78 行 `lynxLog.info('PlayerApp: initData=...')` 在组件函数体中直接调用，属于渲染阶段（主线程）
   - 这违反了 Lynx 的线程安全规则

3. **LogBridge 模块名不匹配**：
   - Android 端注册为 `viewBuilder.registerModule("LogBridge", LogBridgeModule::class.java)`
   - JS 端使用 `NativeModules.LogBridgeModule`，但应该是 `NativeModules.LogBridge`

### 修复步骤

#### 1.1 创建 `src/typing.d.ts`

```typescript
declare let NativeModules: {
  GoBackendModule: {
    getBackendStatus(callback: (result: { running: boolean; port: number }) => void): void;
    startBackend(callback: (result: any) => void): void;
    getStreamUrl(path: string, isExternal: boolean, callback: (url: string) => void): void;
    closePlayer(callback: (result: any) => void): void;
  };
  MpvPlayerModule: {
    play(url: string, callback: (result: any) => void): void;
    pause(callback: (result: any) => void): void;
    resume(callback: (result: any) => void): void;
    seekTo(positionMs: number, callback: (result: any) => void): void;
    setFullscreen(enabled: boolean, callback: (result: any) => void): void;
    setProperty(key: string, value: string, callback: (result: any) => void): void;
  };
  LogBridge: {
    log(level: string, msg: string, callback: (result: any) => void): void;
  };
};
```

#### 1.2 重写 `lynxLog` 为 `'background only'` 函数

```typescript
function lynxLogInfo(msg: string) {
  'background only';
  console.info(msg);
  try {
    NativeModules.LogBridge.log('info', msg, () => {});
  } catch (_e) {}
}

function lynxLogError(msg: string) {
  'background only';
  console.error(msg);
  try {
    NativeModules.LogBridge.log('error', msg, () => {});
  } catch (_e) {}
}

function lynxLogWarn(msg: string) {
  'background only';
  console.warn(msg);
  try {
    NativeModules.LogBridge.log('warn', msg, () => {});
  } catch (_e) {}
}
```

#### 1.3 替换所有 `globalThis.NativeModules.XXX` 为 `NativeModules.XXX`

共 16 处需要替换：
- `globalThis.NativeModules.LogBridgeModule.log` → `NativeModules.LogBridge.log`（3 处，模块名也需修正）
- `globalThis.NativeModules.GoBackendModule.XXX` → `NativeModules.GoBackendModule.XXX`（4 处）
- `globalThis.NativeModules.MpvPlayerModule.XXX` → `NativeModules.MpvPlayerModule.XXX`（9 处）

#### 1.4 将组件顶层的 `lynxLog` 调用移到 useEffect 中

第 78 行的 `lynxLog.info('PlayerApp: initData=...')` 在组件函数体中直接调用（渲染阶段/主线程），需要移到 useEffect 中：

```typescript
useEffect(() => {
  lynxLogInfo('PlayerApp: initData=' + JSON.stringify(initData) + ...);
}, []);
```

#### 1.5 确保 startPlayback 中的 NativeModules 调用在合法上下文中

当前 `startPlayback` 在 useEffect（第 314-321 行）中调用，useEffect 运行在后台线程，所以 NativeModules 调用是合法的。但 `startPlayback` 本身是 `useCallback`，需要确认它在后台线程上下文中被调用。

事件处理函数（handlePlayPause、handleSeek 等）运行在后台线程，所以其中的 NativeModules 调用也是合法的。

#### 1.6 处理 GlobalEventEmitter 访问

第 289-297 行使用 `(globalThis as any).lynx.getJSModule('GlobalEventEmitter')`，这在 useEffect 中调用，是后台线程上下文，应该没问题。但最好也检查一下。

---

## 问题 2：Admin 路由 404

### 根因分析

gin 重构后路由注册情况：
- `/admin` 路由组（`adminGroup := r.Group("/admin")`）只注册了 API 端点：
  - `POST /admin/file/analyze`
  - `POST /admin/file/rename`
- 没有 GET `/admin` 的处理程序
- `NoRoute` 处理器（第 224 行）回退到 `handleRequest`，尝试从服务目录找 "admin" 文件/目录，找不到 → 404

管理后台 UI 的实际架构：
- 管理功能（工具栏、分析、重命名按钮）通过 `injectAdminAssets()` 注入到 FSProxy 页面（`/p/`）
- 登录成功后重定向到 `/p/`
- 所以 `/admin` 应该重定向到 `/p/`

### 修复步骤

#### 2.1 在 server.go 中添加 GET `/admin` 重定向

在 admin 路由组之前添加：

```go
r.GET(routes.Admin, func(c *gin.Context) {
    c.Redirect(http.StatusFound, routes.FSProxy+"/")
})
```

---

## 问题 3：后台服务无法即时响应

### 根因分析

当前状态：
- EncvGoService 是前台服务（`FOREGROUND_SERVICE_DATA_SYNC`）
- Go 进程作为子进程运行
- `WAKE_LOCK` 权限已声明但未使用
- 没有 `REQUEST_IGNORE_BATTERY_OPTIMIZATIONS` 权限
- Android Doze 模式可能限制后台网络访问

### 修复步骤

#### 3.1 在 AndroidManifest.xml 添加电池优化豁免权限

```xml
<uses-permission android:name="android.permission.REQUEST_IGNORE_BATTERY_OPTIMIZATIONS" />
```

#### 3.2 在 MainActivity 中请求电池优化豁免

在 `onCreate` 中添加请求逻辑：

```kotlin
private fun requestBatteryOptimizationExemption() {
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
        val pm = getSystemService(Context.POWER_SERVICE) as android.os.PowerManager
        if (!pm.isIgnoringBatteryOptimizations(packageName)) {
            try {
                val intent = Intent(
                    android.provider.Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS
                ).apply {
                    data = Uri.parse("package:$packageName")
                }
                startActivity(intent)
            } catch (e: Exception) {
                Log.w(TAG, "Failed to request battery optimization exemption", e)
            }
        }
    }
}
```

#### 3.3 在 EncvGoService 中使用 WakeLock

```kotlin
private var wakeLock: android.os.PowerManager.WakeLock? = null

private fun acquireWakeLock() {
    if (wakeLock?.isHeld == true) return
    val pm = getSystemService(Context.POWER_SERVICE) as android.os.PowerManager
    wakeLock = pm.newWakeLock(
        android.os.PowerManager.PARTIAL_WAKE_LOCK,
        "encvgo::GoService"
    )
    wakeLock?.acquire()
}

private fun releaseWakeLock() {
    wakeLock?.let {
        if (it.isHeld) it.release()
    }
    wakeLock = null
}
```

在 `startGoProcess` 中 `acquireWakeLock()`，在 `stopGoProcess` 中 `releaseWakeLock()`。

#### 3.4 优化前台通知

当前通知使用 `android.R.drawable.ic_dialog_info` 图标和低优先级，应改为更专业的通知。

---

## 修改文件清单

| 文件 | 修改内容 |
|------|----------|
| `lynx-player/src/typing.d.ts` | 新建，声明 NativeModules 类型 |
| `lynx-player/src/player/PlayerApp.tsx` | 重写 NativeModules 访问方式，修复模块名，移动日志调用到合法上下文 |
| `internal/server/server.go` | 添加 GET `/admin` 重定向到 `/p/` |
| `android/app/src/main/AndroidManifest.xml` | 添加 REQUEST_IGNORE_BATTERY_OPTIMIZATIONS 权限 |
| `android/app/src/main/java/com/encvgo/app/MainActivity.kt` | 添加电池优化豁免请求 |
| `android/app/src/main/java/com/encvgo/app/EncvGoService.kt` | 添加 WakeLock，优化前台通知 |
