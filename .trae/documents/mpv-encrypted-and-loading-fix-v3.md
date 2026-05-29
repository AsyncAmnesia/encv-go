# 修复：加密视频回滚 ArtPlayer + MPV 加载状态转圈圈（v3）

## 设计原则

**加密是独立状态，不是文件类型。加密视频就是视频，加密文本就是文本。**

- `getFileCategory()` 只根据文件名/扩展名返回内容类型：`'video' | 'audio' | 'image' | 'document' | 'other'`
- `isEncrypted` 是独立的 boolean 维度，不影响分类
- 加密视频 = `category === 'video'` + `isEncrypted === true` → 走视频播放
- 加密文本 = `category === 'document'` + `isEncrypted === true` → 走文本预览
- 对于扩展名无法判断内容类型的加密容器文件（如 `.encv`），路由到预览页由后端 `container.container_type` 决定

## 问题分析

### 问题 1：加密视频回滚到 ArtPlayer

**根因**：`getFileCategory()` 把加密状态混入文件类型，返回 `'encrypted-video'`/`'encrypted-audio'` 等混合类型。路由逻辑不识别这些混合类型，导致加密视频落入预览分支 → ArtPlayer。

**修复**：
1. `getFileCategory()` 移除 `isEncrypted` 参数和所有 `'encrypted*'` 类型，只返回纯内容类型
2. 路由逻辑使用 `category`（内容类型）+ `file.isEncrypted`（加密状态）两个独立维度
3. `category === 'video' || category === 'audio'` → `playMedia()`（无论是否加密）
4. 其他类型 → 预览页（预览页已有基于 `container.container_type` 的正确路由）

### 问题 2：加载状态一直转圈圈

根因不变：`stateListener` 时序 + Compose 不监听 engine 状态。

---

## 实施步骤

### Step 1：重构 getFileCategory — 加密与类型分离

文件：`src/api/encv.ts`

```typescript
export type FileCategory = 'video' | 'audio' | 'image' | 'document' | 'other'

export function getFileCategory(name: string): FileCategory {
  const ext = getFileExtension(name)
  const videoExts = ['mp4', 'mkv', 'avi', 'mov', 'wmv', 'flv', 'webm', 'm4v']
  const audioExts = ['mp3', 'flac', 'wav', 'aac', 'ogg', 'wma', 'm4a']
  const imageExts = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp', 'svg']
  const docExts = ['pdf', 'doc', 'docx', 'txt', 'xls', 'xlsx', 'ppt', 'pptx']

  if (videoExts.includes(ext)) return 'video'
  if (audioExts.includes(ext)) return 'audio'
  if (imageExts.includes(ext)) return 'image'
  if (docExts.includes(ext)) return 'document'
  return 'other'
}
```

关键变化：
- 移除 `isEncrypted` 参数
- 移除 `'encrypted'`、`'encrypted-video'`、`'encrypted-audio'`、`'encrypted-image'` 类型
- 函数只关注内容类型，加密状态由调用方通过 `file.isEncrypted` 单独判断

### Step 2：handleFileClick — 基于内容类型路由

文件：`src/views/Files.vue`

```typescript
async function handleFileClick(file: FileItem) {
  if (file.isDirectory) {
    const newPath = currentPath.value === '/'
      ? '/' + file.name
      : currentPath.value + '/' + file.name
    navigateTo(newPath)
    return
  }

  const category = getFileCategory(file.name)
  console.info('[Files] Click:', file.name, 'category:', category, 'encrypted:', !!file.isEncrypted)
  if (category === 'video' || category === 'audio') {
    playMedia(file, category)
  } else {
    router.push({
      path: '/tabs/preview',
      query: { path: file.path, name: file.name, isEncrypted: String(!!file.isEncrypted) },
    })
  }
}
```

关键变化：
- `getFileCategory(file.name)` 不再传 `isEncrypted`
- 加密视频（`category === 'video'` + `isEncrypted === true`）直接走 `playMedia()` → MPV
- 非媒体类型（包括加密文档/图片等）走预览页

### Step 3：handleLongPress — 基于内容类型路由

文件：`src/views/Files.vue`

```typescript
} else if (file.isEncrypted) {
  const category = getFileCategory(file.name)
  const isMedia = category === 'video' || category === 'audio'
  buttons.push({
    text: isMedia ? t('files.play') : t('files.preview'),
    icon: isMedia ? videocam : image,
    handler: () => {
      if (isMedia) {
        playMedia(file, category)
      } else {
        router.push({
          path: '/tabs/preview',
          query: { path: file.path, name: file.name, isEncrypted: 'true' },
        })
      }
    },
  })
  buttons.push({ text: t('files.decrypt'), ... })
  buttons.push({ text: t('files.delete'), ... })
} else {
```

关键变化：
- 加密文件分支使用 `getFileCategory(file.name)` 获取内容类型（不传 `isEncrypted`）
- 加密视频/音频 → "播放"按钮 → `playMedia()`
- 加密图片/文档/其他 → "预览"按钮 → 预览页
- 保留"解密"和"删除"按钮

### Step 4：加密徽章判断简化

文件：`src/views/Files.vue` 第 250 行和第 317 行

```html
<ion-badge v-if="file.isEncrypted" color="warning" slot="end">
```

简化：`file.isEncrypted` 已经是独立的 boolean，不需要 `getFileCategory` 辅助判断。

### Step 5：useFileList.ts — 图标和颜色适配

文件：`src/composables/useFileList.ts`

```typescript
export function getFileIcon(file: FileItem) {
  if (file.isDirectory) return folder
  if (file.isEncrypted) return lockClosed
  const category = getFileCategory(file.name)
  switch (category) {
    case 'video': return videocam
    case 'audio': return musicalNotes
    case 'image': return image
    case 'document': return documentIcon
    default: return documentText
  }
}

export function getFileIconColor(file: FileItem): string {
  if (file.isDirectory) return 'primary'
  if (file.isEncrypted) return 'warning'
  const category = getFileCategory(file.name)
  switch (category) {
    case 'video': return 'danger'
    case 'audio': return 'tertiary'
    case 'image': return 'success'
    default: return 'medium'
  }
}
```

关键变化：
- 加密文件优先显示锁图标 + warning 颜色（不管内容类型）
- `getFileCategory(file.name)` 不再传 `isEncrypted`
- 移除 `case 'encrypted'` 分支

### Step 6：FilePickerModal.vue — 同步适配

文件：`src/components/FilePickerModal.vue`

同 Step 5 的逻辑：加密文件显示锁图标，`getFileCategory` 不传 `isEncrypted`。

### Step 7：FilePreview.vue — 已正确（无需修改）

`FilePreview.vue` 已经基于后端 `container.container_type` 路由，无需修改。

### Step 8：MpvPlayerActivity Surface 直接挂载

文件：`plugin-mpv-player/.../MpvPlayerActivity.kt`

删除 `stateListener` 回调方式挂载 Surface，改为 `setContent` 后直接调用 `attachSurfaceView(contentRoot)`。

### Step 9：MpvPlayerScreen 订阅 engine.stateListener

文件：`plugin-mpv-player/.../MpvPlayerScreen.kt`

添加 `DisposableEffect(engine)` 订阅 `engine.stateListener`，映射到 Compose `PlayerState`。删除 `startPlayback` 中重复的 `onStateChange(PlayerState.Loading)` 调用。

### Step 10：MpvAudioPlayerScreen 订阅 engine.stateListener

文件：`plugin-mpv-player/.../MpvAudioPlayerScreen.kt`

同步添加 `DisposableEffect(engine)` 订阅 `engine.stateListener`。

### Step 11：验证构建

```bash
cd /workspace/app/encv-mobile/android && ./gradlew :plugin-mpv-player:compileDebugKotlin
```
