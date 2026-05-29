# 修复：加密视频回滚 ArtPlayer + MPV 加载状态转圈圈（v4）

## 设计原则

1. **加密是独立状态，不是文件类型**：`getFileCategory()` 只返回内容类型，`isEncrypted` 是独立 boolean
2. **加密视频就是视频，加密文本就是文本**：内容类型决定路由，加密状态不影响分类
3. **加密容器必须走预览页**：扩展名无法判断内容类型的加密容器文件（如 `.encv`），必须通过预览页查询后端 `container.container_type` 后再决定路由
4. **预览页是加密文件的权威路由器**：基于插件运行时声明（`container.container_type`），不是前端猜测

## 两种加密文件的路由策略

| 加密文件类型 | 示例 | `getFileCategory()` | 路由 |
|---|---|---|---|
| 扩展名可识别的加密媒体 | `video.mp4` (isEncrypted) | `'video'` | `playMedia()` → MPV |
| 扩展名可识别的加密文档 | `doc.pdf` (isEncrypted) | `'document'` | 预览页 |
| 加密容器（不可识别扩展名） | `file.encv` (isEncrypted) | `'other'` | 预览页 → 后端 `container_type` 决定 |

---

## 实施步骤

### Step 1：重构 getFileCategory — 加密与类型分离

文件：`src/api/encv.ts`

- 移除 `isEncrypted` 参数
- 移除 `'encrypted'`、`'encrypted-video'`、`'encrypted-audio'`、`'encrypted-image'` 类型
- 函数只关注内容类型

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

路由逻辑：
- `video.mp4` (加密) → `category='video'` → `playMedia()` → MPV ✓
- `song.mp3` (加密) → `category='audio'` → `playMedia()` → MPV ✓
- `file.encv` (加密容器) → `category='other'` → 预览页 → 后端 `container_type` 决定 ✓
- `doc.pdf` (加密) → `category='document'` → 预览页 ✓
- `photo.jpg` (加密) → `category='image'` → 预览页 ✓

### Step 3：handleLongPress — 加密文件分支重构

文件：`src/views/Files.vue`

将 `else if (category.startsWith('encrypted'))` 改为 `else if (file.isEncrypted)`，使用 `getFileCategory(file.name)` 获取内容类型：

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
  buttons.push({
    text: t('files.decrypt'),
    icon: lockClosed,
    handler: () => { handleDecryptFile(file) },
  })
  buttons.push({
    text: t('files.delete'),
    icon: trash,
    role: 'destructive',
    handler: () => { handleDeleteFile(file) },
  })
} else {
```

关键变化：
- 加密视频/音频（`category='video'|'audio'`）→ "播放"按钮 → `playMedia()`
- 加密容器/文档/图片（`category='other'|'document'|'image'`）→ "预览"按钮 → 预览页
- 预览页查询后端 `container.container_type` 后决定实际路由

### Step 4：加密徽章判断简化

文件：`src/views/Files.vue` 第 250 行和第 317 行

```html
<ion-badge v-if="file.isEncrypted" color="warning" slot="end">
```

`file.isEncrypted` 已经是独立的 boolean，不需要 `getFileCategory` 辅助判断。

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

### Step 6：FilePickerModal.vue — 同步适配

文件：`src/components/FilePickerModal.vue`

同 Step 5 逻辑：加密文件优先显示锁图标，`getFileCategory` 不传 `isEncrypted`。

### Step 7：FilePreview.vue — 已正确（无需修改）

`FilePreview.vue` 已经基于后端 `container.container_type` 路由：
- `video`/`audio` → `openPlayer()` → MPV
- `image` → 显示图片
- `document`/`text` → 显示文本
- 其他 → 显示容器信息

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
