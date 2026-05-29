# 修复 MPV 播放器两致命 Bug

## Bug 诊断

### Bug 1: 三种 MPV 模式静默回滚到 Artplayer（严重违反铁律）

**根因链**：
```
用户在 Settings 选择 "MPV (Activity)"
  → Settings.vue 存 localStorage('encv_player_video', 'mpv-activity')  ✅
  → Files.vue getPlayMode() 读取 'mpv-activity'
    → Line 474: if (stored === ARTPLAYER || MPV_PLUGIN || EXTERNAL) return stored
    → 'mpv-activity' 不匹配任何一项！→ 跳到 else → 返回 VIDEO_DEFAULT = 'artplayer'  ❌
  → switch(mode) 进入 case ARTPLAYER → router.push('/player')  ← 静默回滚！
```

**3 个断点全部断裂**：

| 断点 | 位置 | 问题 |
|------|------|------|
| `getPlayMode()` 白名单 | [Files.vue L474](app/encv-mobile/src/views/Files.vue#L474) | 只认 `ARTPLAYER/MPV_PLUGIN/EXTERNAL`，不认 `MPV_ACTIVITY/MPV_FRAGMENT/MPV_COMPOSE` |
| `switch(mode)` 分发 | [Files.vue L497](app/encv-mobile/src/views/Files.vue#L497) | 只有 `case MPV_PLUGIN`，缺少 3 个新 sub-mode 的 case |
| `openPlayer()` 参数 | [Files.vue L499](app/encv-mobile/src/views/Files.vue#L499) | 硬编码传 `PLAY_MODE.MPV_PLUGIN`，应传实际 `mode` 变量 |

### Bug 2: 插件状态变更后 Settings 不自动刷新

**根因分析**：

Settings.vue 的 MPV 状态徽章仅在以下时机刷新：
- `onMounted`（页面首次加载）
- `ionChange`（用户手动切换播放器选项时）

当用户在 ExtensionsPage 执行以下操作后，Settings 的 MPV 徽章不会自动更新：
- **安装插件** (`pickAndInstallPlugin` 成功)
- **启用/禁用插件** (`togglePluginEnabled`)
- **卸载插件** (`uninstallPlugin`)

这些操作都会改变 PluginManager 内部状态（XML 持久化 / ClassLoader 加载），但 Settings 页面完全不知情。

**修复方向**：所有插件状态变更操作完成后，广播通用事件 `'plugin-state-changed'`，Settings 监听后自动刷新。

---

## 修复方案

### Task 1: 修复 Files.vue — mode 识别 + 分发 + 传参（Bug 1 核心修复）

#### SubTask 1.1: `getPlayMode()` 白名单扩展

```typescript
// Before (L474):
if (stored === PLAY_MODE.ARTPLAYER || stored === PLAY_MODE.MPV_PLUGIN || stored === PLAY_MODE.EXTERNAL)

// After:
if (isValidPlayMode(stored))
```

新增工具函数：
```typescript
import { PLAY_MODE, isMpvSubMode } from '@/constants/player'

function isValidPlayMode(value: string): value is PlayMode {
  const allModes = [
    PLAY_MODE.ARTPLAYER,
    PLAY_MODE.MPV_PLUGIN,      // 兼容旧值
    PLAY_MODE.MPV_ACTIVITY,
    PLAY_MODE.MPV_FRAGMENT,
    PLAY_MODE.MPV_COMPOSE,
    PLAY_MODE.EXTERNAL,
  ]
  return allModes.includes(value as PlayMode)
}
```

#### SubTask 1.2: `switch(mode)` 分发覆盖所有 MPV 子模式

```typescript
switch (mode) {
  case PLAY_MODE.ARTPLAYER:
    router.push({ path: '/player', query: { path: file.path, name: file.name } })
    break
  // 所有 MPV 子模式统一走 native openPlayer
  case PLAY_MODE.MPV_PLUGIN:
  case PLAY_MODE.MPV_ACTIVITY:
  case PLAY_MODE.MPV_FRAGMENT:
  case PLAY_MODE.MPV_COMPOSE:
    if (isNative()) {
      const result = await openPlayer(file.path, file.name, mimeType, mode)  // ← 传实际 mode!
      if (!result.success) { /* 显示错误 banner */ }
    } else { /* fallback */ }
    break
  case PLAY_MODE.EXTERNAL:
    /* ... */
    break
  default:
    // 未知 mode → 不应发生，显示警告
    console.warn('[Files] Unknown play mode:', mode)
    break
}
```

#### SubTask 1.3: `openPlayer()` 传参修正

```typescript
// Before: 硬编码 MPV_PLUGIN
const result = await openPlayer(file.path, file.name, mimeType, PLAY_MODE.MPV_PLUGIN)

// After: 传递用户选择的实际 mode
const result = await openPlayer(file.path, file.name, mimeType, mode)
```

---

### Task 2: 修复 GoProcessPlugin.openPlayer — mode 分发完整性检查

当前 `openPlayer` 已有 `effectiveMode` 归一化逻辑（L128），确认 `"mpv-plugin"` / `"mpv"` 会映射到 `"mpv-activity"`。但需确保非 mpv-* 模式不走 mpv 分支。

**验证点**：当 mode 为 `"mpv-activity"` 时，进入 startActivityForResult 分支 ✅（已正确）

---

### Task 3: 插件状态变更后自动刷新 MPV 状态（Bug 2 修复）

#### SubTask 3.1: ExtensionsPage 所有状态变更操作后广播事件

以下 3 个操作成功后均 dispatch 事件：

```typescript
// 安装成功后 (L208-209)
if (result.success) {
  showToast({ message: t('extensions.installSuccess'), ... })
  window.dispatchEvent(new CustomEvent('plugin-state-changed'))
  await loadExtensions()
}

// 启用/禁用成功后 (L244)
await loadExtensions()
window.dispatchEvent(new CustomEvent('plugin-state-changed'))

// 卸载成功后 (L273)
await loadExtensions()
window.dispatchEvent(new CustomEvent('plugin-state-changed'))
```

#### SubTask 3.2: Settings.vue 监听插件状态变更事件

```typescript
onMounted(() => {
  window.addEventListener('plugin-state-changed', refreshMpvPluginStatus)
})
onUnmounted(() => {
  window.removeEventListener('plugin-state-changed', refreshMpvPluginStatus)
})
```

---

## 改动文件清单

| 文件 | 改动内容 |
|------|---------|
| [Files.vue](app/encv-mobile/src/views/Files.vue) | `getPlayMode()` 白名单扩展 + switch 覆盖全部子模式 + openPlayer 传实际 mode |
| [Settings.vue](app/encv-mobile/src/views/Settings.vue) | 监听 `plugin-state-changed` 事件自动刷新 MPV 状态 |
| [ExtensionsPage.vue](app/encv-mobile/src/views/ExtensionsPage.vue) | 安装/启用/卸载成功后 dispatch `plugin-state-changed` 事件 |

## 铁律合规检查

| 铁律 | 合规方式 |
|------|---------|
| **严禁自动 fallback** | 未知 mode 进 default 分支打 warning 日志，不再静默切换到 Artplayer |
| **严禁 Toast** | 错误通过现有 error banner 展示 |
| **饱和调试** | `console.info('[Files] playMedia: ... mode=...')` 已有，增加 unknown mode warning |

## 验证清单

- [ ] Settings 选 "MPV (Activity)" → localStorage 存 `'mpv-activity'`
- [ ] Files.vue 打开视频 → `getPlayMode('video')` 返回 `'mpv-activity'`（不是 artplayer）
- [ ] `openPlayer(..., 'mpv-activity')` 被正确调用
- [ ] Kotlin 端 `[ModeC-Activity]` 日志出现
- [ ] Settings 选 "MPV (Fragment)" → 同上流程，Kotlin 端 `[ModeB-Fragment]` 日志出现
- [ ] Settings 选 "MPV (Compose)" → 同上流程，Kotlin 端 `[ModeA-Compose]` 日志出现
- [ ] 安装 MPV 后 Settings 自动刷新状态（无需手动切选项）
- [ ] 启用/禁用 MPV 后 Settings 徽章自动更新
- [ ] 卸载 MPV 后 Settings 徽章自动变为"未安装"
