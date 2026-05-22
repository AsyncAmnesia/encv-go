# 修复播放闪退 — Lynx bundle 缺失 + 多重崩溃点

## 根本原因 1（致命）：CI 没有构建 Lynx 播放器 bundle

CI 工作流 `android.yml` 中：
- `npm run build` 只构建主应用（Vue + Vite），**不构建 Lynx 播放器**
- 完全缺少 `cd lynx-player && npm install && npm run build` 步骤
- `lynx-player/dist/player.lynx.bundle` 不存在
- post-cap-sync.mjs 第 365 行 `if (existsSync(LYNX_BUNDLE_PATH))` 为 false，bundle 不会被复制到 assets
- APK 中没有 `player.lynx.bundle`
- `PlayerTemplateProvider.loadTemplate("player.lynx.bundle")` → `assets.open(uri)` 抛 `IOException` → `callback.onFailed()` → Lynx 渲染崩溃

**这是最可能的闪退原因。**

## 根本原因 2（致命）：MPVLib EventObserver 回调在非主线程

MPVLib 的 `eventProperty()` / `event()` 回调在 mpv 的 native 线程上执行，不是 Android 主线程。
`dispatchStateChange()` 在这些回调中调用 `lynxContext.sendGlobalEvent()`，这可能在非主线程上操作 Lynx UI，导致崩溃。

官方 mpv-android 的 `BaseMPVView` 也有同样的问题，但它的 UI 更新通过 `View.post()` 回到主线程。我们需要在 `dispatchStateChange` / `dispatchPositionUpdate` 中使用 `Handler(Looper.getMainLooper()).post()` 确保在主线程执行。

## 根本原因 3（潜在）：libplayer.so 可能缺失

setup-mpv-libs.sh 第 21 行：
```bash
unzip -o -j "$AAR_TMP" "jni/$abi/libmpv.so" "jni/$abi/libplayer.so" -d "$JNI_DIR/$abi" 2>/dev/null || true
```
`|| true` 意味着如果 `libplayer.so` 不在 AAR 中，脚本不会报错。但第 22 行只检查 `libmpv.so`：
```bash
if [ -f "$JNI_DIR/$abi/libmpv.so" ]; then
    echo "  ✓ $abi: libmpv.so + libplayer.so"
```
如果 `libplayer.so` 缺失，`System.loadLibrary("player")` 会抛 `UnsatisfiedLinkError` 崩溃。

## 执行计划

### 1. 在 CI 工作流中添加 Lynx 播放器构建步骤

**文件**: `.github/workflows/android.yml`

在 "Install frontend dependencies & Build" 步骤之后添加：
```yaml
- name: Build Lynx player bundle
  run: cd app/encv-mobile/lynx-player && npm install && npm run build
```

### 2. 在 post-cap-sync.mjs 中添加 bundle 缺失检查

**文件**: `scripts/post-cap-sync.mjs`

当 `LYNX_BUNDLE_PATH` 不存在时，打印错误并退出（而不是静默跳过）：
```js
if (!existsSync(LYNX_BUNDLE_PATH)) {
  console.error('ERROR: Lynx bundle not found at', LYNX_BUNDLE_PATH)
  console.error('Run: cd lynx-player && npm install && npm run build')
  process.exit(1)
}
```

### 3. 修复 MPVLib EventObserver 线程问题

**文件**: `android-overlay/app/src/main/java/com/encvgo/app/MpvPlayerModule.kt`

添加 `Handler(Looper.getMainLooper())` 成员变量，在 `dispatchStateChange` / `dispatchPositionUpdate` 中使用 `handler.post {}` 确保在主线程执行 `sendGlobalEvent`。

### 4. 在 setup-mpv-libs.sh 中验证 libplayer.so

**文件**: `scripts/setup-mpv-libs.sh`

提取后检查 `libplayer.so` 是否存在：
```bash
if [ ! -f "$JNI_DIR/$abi/libplayer.so" ]; then
  echo "  ✗ ERROR: libplayer.so not found in AAR!"
  exit 1
fi
```

### 5. PlayerTemplateProvider 加载失败时优雅降级

**文件**: `android-overlay/app/src/main/java/com/encvgo/app/PlayerTemplateProvider.kt`

`loadTemplate` 失败时 `callback.onFailed()` 可能导致 Lynx 崩溃。需要确保回调被正确调用，并在 `PlayerActivityLynx` 中处理 LynxView 初始化失败的情况。
