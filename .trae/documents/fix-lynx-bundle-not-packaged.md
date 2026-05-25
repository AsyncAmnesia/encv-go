# 修复：player.lynx.bundle 未打包进 APK

## 问题分析

### 错误日志解读

```
ERROR [LynxPlayerClient] onReceivedError: code=102
  rootCause=player.lynx.bundle
  "Error occurred while fetching app bundle resource"
  at PlayerTemplateProvider.loadTemplate$lambda$4(PlayerTemplateProvider.kt:47)

ERROR [PlayerActivityLynx] createLynxView: cannot attachToLayout,
  rootLayout=FrameLayout{...}, mpvModule=null
```

两个错误，但**根因相同**：打包管道问题，不是 Lynx SDK 或 mpv lib 本身的 bug。

### 根因链

```
sync-native.mjs 被 cap copy 触发（上一轮已修复 hook）
  ↓
sync-native.mjs 检查 player.lynx.bundle 存在 ✓
  ↓
但 sync-native.mjs 从未将 bundle 复制到 android/app/src/main/assets/ ❌
  ↓
PlayerTemplateProvider.loadTemplate() 用 assets.open("player.lynx.bundle") 加载
  ↓
assets 中无此文件 → callback.onFailed() → Lynx 报错 code=102
```

同时，mpv .so 文件未打包（上一轮修复的 hook 问题）→ `System.loadLibrary("mpv")` 失败 → `MPVLib` 对象初始化失败 → `MpvPlayerModule` 创建失败 → `mpvModule=null`。

### 证据

旧脚本 `post-cap-sync.mjs` 第 416-419 行**有**复制逻辑：
```js
const assetsDir = join(ANDROID_DIR, 'app', 'src', 'main', 'assets')
mkdirSync(assetsDir, { recursive: true })
copyFileSync(LYNX_BUNDLE_PATH, join(assetsDir, 'player.lynx.bundle'))
console.log('  bundle: copied player.lynx.bundle to assets')
```

新脚本 `sync-native.mjs` 第 51-56 行**只有检查，没有复制**：
```js
if (!existsSync(LYNX_BUNDLE_PATH)) {
  console.error('ERROR: Lynx bundle not found.')
  process.exit(1)
}
console.log('  Lynx bundle: exists ✓')
```

上一轮精简 `sync-native.mjs` 时，遗漏了 Lynx bundle 的复制逻辑。

### 结论：是 Lynx 还是 lib 的问题？

**都不是。** 这是打包管道问题：
- Lynx SDK 本身工作正常（正确报告模板缺失）
- mpv lib 本身没问题（只是 .so 文件没被复制到 jniLibs）
- 两者都是 `sync-native.mjs` 精简时遗漏了复制步骤

修复后 Lynx 方案应该可以正常工作。项目已有 `PlayerActivityCapacitor` 作为备用方案（`BuildConfig.USE_LYNX_PLAYER = false` 时使用），但无需切换。

---

## 修复方案

### Step 1：在 sync-native.mjs 中添加 Lynx bundle 复制逻辑

在文件末尾（Lynx bundle 检查之后），添加复制到 Android assets 目录的代码：

```js
const assetsDir = join(ANDROID_DIR, 'app', 'src', 'main', 'assets')
mkdirSync(assetsDir, { recursive: true })
copyFileSync(LYNX_BUNDLE_PATH, join(assetsDir, 'player.lynx.bundle'))
console.log('  bundle: copied player.lynx.bundle to assets')
```

替换现有的仅检查逻辑（第 51-56 行）。

### Step 2：在 CI 验证步骤中增加 Lynx bundle 检查

在 "Verify native libraries" 步骤中增加：

```bash
echo "=== Lynx bundle ==="
if [ -f "app/encv-mobile/android/app/src/main/assets/player.lynx.bundle" ]; then
  echo "✅ player.lynx.bundle found ($(ls -lh app/encv-mobile/android/app/src/main/assets/player.lynx.bundle | awk '{print $5}'))"
else
  echo "❌ player.lynx.bundle MISSING!"
  exit 1
fi
```

### Step 3：在 APK 验证步骤中增加 Lynx bundle 检查

在 "Verify APK contents" 步骤中增加：

```bash
echo "=== Lynx bundle in APK ==="
unzip -l "$APK_PATH" | grep "player.lynx.bundle" || echo "❌ player.lynx.bundle NOT in APK!"
```

---

## 修改文件

1. `/workspace/app/encv-mobile/scripts/sync-native.mjs` — 添加 Lynx bundle 复制逻辑
2. `/workspace/.github/workflows/android.yml` — 增加验证步骤
