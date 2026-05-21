# PlayerActivity 待修复问题 Plan

## 问题 1：应用内打开视频路径 404

### 根因
Files.vue 通过 `openInPlayer(file.path, ...)` 发送的路径是 `/123云盘/xxx.mp4`（相对于 serve root），但 Android 文件系统需要完整绝对路径 `/storage/emulated/0/123云盘/xxx.mp4`。后端 `StreamExternalFile` 对该路径 `os.Stat()` 返回不存在 → 404。

### 修复：StandalonePlayer.vue 路径补全

在 `startPlayback()` 或 `streamUrl` computed 中，检测并转换非标准绝对路径：

```typescript
// 在 StandalonePlayer.vue 中添加
function resolveNativePath(raw: string): string {
  if (!raw.startsWith('/')) return raw
  if (raw.startsWith('/storage/') || raw.startsWith('/sdcard/')) return raw
  // 相对 serve root 的路径 → 补全 Android 存储根目录
  return `/storage/emulated/0${raw}`
}

// streamUrl computed 中使用：
const streamUrl = computed(() => {
  if (!filePath.value) return ''
  const resolvedPath = resolveNativePath(filePath.value)
  if (isExternalFile.value) return getExternalStreamUrl(resolvedPath)
  return getFileStreamUrl(resolvedPath)
})
```

**文件**：`src/views/StandalonePlayer.vue`

---

## 问题 2：设置按钮无响应

### 根因
`goSettings()` 使用 `router.push('/player/settings')`，但 `<ion-router-outlet>` 已移除，Vue Router 不工作。

### 修复方案：事件驱动 + 条件渲染

**Step 1**: `StandalonePlayer.vue` — 将路由跳转改为 emit 事件

```typescript
// 替换 goSettings() 方法
const emit = defineEmits(['open-settings'])
function goSettings() {
  emit('open-settings')
}
```

**Step 2**: `PlayerApp.vue` — 添加设置页面条件渲染 + 状态管理

```vue
<template>
  <ion-app>
    <Suspense>
      <template #default>
        <StandalonePlayer v-if="currentView === 'player'" @open-settings="currentView = 'settings'" />
        <PlayerSettings v-else @close="currentView = 'player'" />
      </template>
      <template #fallback>
        <div style="display:flex;justify-content:center;align-items:center;height:100vh;background:#1a1a2e;color:#fff;">
          Loading...
        </div>
      </template>
    </Suspense>
  </ion-app>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { IonApp } from '@ionic/vue'
import StandalonePlayer from '@/views/StandalonePlayer.vue'
import PlayerSettings from '@/views/PlayerSettings.vue'

const currentView = ref<'player' | 'settings'>('player')
</script>
```

**Step 3**: `PlayerSettings.vue` — 添加返回按钮 emit close 事件

```typescript
// 添加返回逻辑
const emit = defineEmits(['close'])
function goBack() {
  emit('close')
}
```

**文件**：
- `src/PlayerApp.vue`
- `src/views/StandalonePlayer.vue`
- `src/views/PlayerSettings.vue`

---

## 问题 3：ArtPlayer 全屏旋转屏幕

### 需求
进入全屏时根据视频宽高比自动选择横屏/竖屏，退出全屏恢复原始方向。

### 修复：ArtPlayer fullscreen 监听 + Capacitor Screen Orientation

```typescript
// 在 StandalonePlayer.vue 的 initArtPlayer() 中添加：

import { ScreenOrientation } from '@capacitor/screen-orientation'

art.on('fullscreen', (state: boolean) => {
  isFullscreen.value = state
  if (state) {
    // 进入全屏：根据视频宽高比决定方向
    const video = art.template?.<HTMLVideoElement>('video')
    if (video?.videoWidth && video?.videoHeight) {
      const ratio = video.videoWidth / video.videoHeight
      if (ratio > 1.3) {
        // 宽视频 → 横屏
        ScreenOrientation.lock({ orientation: 'landscape' })
      } else if (ratio < 0.77) {
        // 竖视频 → 竖屏
        ScreenOrientation.lock({ orientation: 'portrait' })
      } else {
        // 接近方屏 → 保持当前或横屏
        ScreenOrientation.lock({ orientation: 'landscape' })
      }
    }
  } else {
    // 退出全屏：恢复
    ScreenOrientation.unlock()
  }
})
```

**依赖**：需要安装 `@capacitor/screen-orientation` 插件。

**备选**：如果不想引入新插件，可以通过 GoProcessPlugin 调用原生 Activity.setRequestedOrientation()。

**文件**：
- `src/views/StandalonePlayer.vue`
- `package.json`（新增依赖）

---

## 修改文件清单

| # | 文件 | 修改内容 |
|---|------|---------|
| 1 | `src/views/StandalonePlayer.vue` | 路径补全函数 + 设置按钮改为 emit + ArtPlayer 全屏旋转 |
| 2 | `src/PlayerApp.vue` | 条件渲染 player/settings |
| 3 | `src/views/PlayerSettings.vue` | 返回按钮 emit close 事件 |
| 4 | `package.json` | 新增 @capacitor/screen-orientation（可选） |

---

## 执行顺序建议

1. **先修路径问题**（问题 1）— 这是播放功能的核心阻塞
2. **再修设置按钮**（问题 2）— 用户体验
3. **最后做全屏旋转**（问题 3）— 增强功能，需要新依赖
