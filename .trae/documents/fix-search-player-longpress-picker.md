# 文件搜索 + 播放器优化 + 长按菜单修复 + 文件选择器修复

## 问题概述

1. **文件搜索**：需要搜索框，支持子文件夹递归搜索（可选）和模糊匹配
2. **竖屏视频错位**：播放器 `aspect-ratio: 16/9` 强制横屏比例，竖屏视频显示错位，需要完善横竖屏和全屏
3. **长按菜单无效**：`@longpress.prevent` 在 Ionic web component 上不工作
4. **文件选择器无效**：`startPicking()` 只设置了状态，没有导航到 Files 页面

---

## 问题 3 & 4：先修 Bug（长按菜单 + 文件选择器）

### 问题 3 根因：`@longpress` 在 `ion-item` 上不触发

**原因**：`ion-item` 是 Ionic 的 Shadow DOM web component，原生 `longpress` 事件在 Shadow DOM 边界被阻断。移动端 Capacitor WebView 也不一定支持 `longpress` 事件。

**方案**：创建自定义 `v-longpress` 指令，基于 `touchstart`/`touchend` + `setTimeout` 实现 500ms 长按检测。

#### 新建 `src/directives/longpress.ts`

```ts
import type { Directive } from 'vue'

export const vLongpress: Directive<HTMLElement, () => void> = {
  mounted(el, binding) {
    if (typeof binding.value !== 'function') return
    let pressTimer: ReturnType<typeof setTimeout> | null = null

    const start = (e: Event) => {
      e.preventDefault()
      if (pressTimer !== null) return
      pressTimer = setTimeout(() => {
        binding.value()
        pressTimer = null
      }, 500)
    }

    const cancel = () => {
      if (pressTimer !== null) {
        clearTimeout(pressTimer)
        pressTimer = null
      }
    }

    el.addEventListener('touchstart', start, { passive: false })
    el.addEventListener('touchend', cancel)
    el.addEventListener('touchmove', cancel)
    el.addEventListener('touchcancel', cancel)
    el.addEventListener('mousedown', start)
    el.addEventListener('mouseup', cancel)
    el.addEventListener('mouseleave', cancel)
    el.addEventListener('contextmenu', (e) => e.preventDefault())

    ;(el as any)._longpress_cleanup = () => {
      el.removeEventListener('touchstart', start)
      el.removeEventListener('touchend', cancel)
      el.removeEventListener('touchmove', cancel)
      el.removeEventListener('touchcancel', cancel)
      el.removeEventListener('mousedown', start)
      el.removeEventListener('mouseup', cancel)
      el.removeEventListener('mouseleave', cancel)
    }
  },
  unmounted(el) {
    ;(el as any)._longpress_cleanup?.()
  },
}
```

#### Files.vue 变更

1. 引入 `vLongpress` 指令
2. 将 `@longpress.prevent="handleLongPress(file)"` 替换为 `v-longpress="() => handleLongPress(file)"`
3. 移除 `@longpress.prevent`

### 问题 4 根因：`startPicking()` 没有导航到 Files 页面

**原因**：`handleBrowse()` 调用 `startPicking()` 后只设置了 `isPickerMode = true`，但用户仍在 Tasks 标签页，根本看不到 Files 页面。

**方案**：`startPicking()` 之后立即 `router.push('/tabs/files')` 导航到文件页。

#### useFilePicker.ts 变更

`startPicking()` 不再负责导航（composable 不应耦合路由），改为在 Tasks.vue 的 `handleBrowse()` 中手动导航：

```ts
async function handleBrowse() {
  showNewTaskModal.value = false
  startPicking()
  router.push('/tabs/files')  // 导航到文件页
  // 等待 Promise resolve（用户选择或取消后自动 resolve）
  const result = await pickerPromise
  if (result) {
    newTaskPath.value = result.path
  }
  showNewTaskModal.value = true
}
```

但 `startPicking()` 返回 Promise，`router.push` 后需要等用户操作。当前设计已经是这样，只需加 `router.push`。

#### 具体变更

**Tasks.vue** `handleBrowse()`：
```ts
async function handleBrowse() {
  showNewTaskModal.value = false
  startPicking()
  await router.push('/tabs/files')
  // startPicking 的 Promise 会在 confirmSelection 或 cancelPicking 时 resolve
  // 但这里不能 await startPicking() 因为页面已切换
  // 需要改用 watch 或回调方式
}
```

**问题**：`startPicking()` 返回 Promise，但 `router.push` 后页面切换了，Tasks 组件可能被卸载，`await` 会丢失。

**更好的方案**：改用事件驱动而非 Promise。`useFilePicker` 增加 `onPickingComplete` 回调注册：

```ts
// useFilePicker.ts
let onComplete: ((result: { path: string; name: string } | null) => void) | null = null

function startPicking(onDone: (result: { path: string; name: string } | null) => void) {
  isPickerMode.value = true
  selectedPath.value = ''
  selectedName.value = ''
  onComplete = onDone
}

function confirmSelection(path: string, name: string) {
  selectedPath.value = path
  selectedName.value = name
  isPickerMode.value = false
  onComplete?.({ path, name })
  onComplete = null
}

function cancelPicking() {
  isPickerMode.value = false
  selectedPath.value = ''
  selectedName.value = ''
  onComplete?.(null)
  onComplete = null
}
```

**Tasks.vue**：
```ts
async function handleBrowse() {
  showNewTaskModal.value = false
  startPicking((result) => {
    if (result) {
      newTaskPath.value = result.path
    }
    showNewTaskModal.value = true
  })
  router.push('/tabs/files')
}
```

**Files.vue** `handleCancelPicker()`：
```ts
function handleCancelPicker() {
  cancelPicking()
  router.push('/tabs/tasks')
}
```

**Files.vue** picker 选择完成：
```ts
// 在 handleFileClick 的 picker 分支
confirmSelection(file.path, file.name)
router.push('/tabs/tasks')
```

---

## 问题 2：竖屏视频错位 + 横竖屏切换 + 全屏

### 根因

1. `.video-player` 设置了 `aspect-ratio: 16/9`，强制横屏比例，竖屏视频被压缩或留黑边
2. ArtPlayer 配置了 `autoSize: true` 但容器 CSS 覆盖了自动尺寸
3. 没有全屏播放支持
4. 非全屏时空白区域没有利用

### 方案

#### Player.vue 变更

1. **移除 `aspect-ratio: 16/9`**：让 ArtPlayer 的 `autoSize` 根据视频实际比例自动调整
2. **容器改为自适应**：`.video-player` 使用 `width: 100%` + `max-height: 70vh`，不强制比例
3. **ArtPlayer 配置增强**：
   - 添加 `fullscreen: true` 启用全屏按钮
   - 添加 `miniProgressBar: true`
   - 添加 `autoOrientation: true`（ArtPlayer 插件，自动根据视频方向旋转）
   - 监听 `resize` 和 `video:loadedmetadata` 事件，根据视频实际宽高比调整容器
4. **视频信息区域**：非全屏时在播放器下方显示文件名、大小、路径等信息
5. **ArtPlayer fullscreen 事件**：监听 `fullscreen` 和 `fullscreenExit` 事件

#### 具体实现

**模板**：
```html
<div v-else class="player-container">
  <div v-if="isVideo && !playerError" ref="artContainer" class="video-player"></div>

  <div v-if="isVideo && playerError" class="player-error">
    <!-- 错误 UI 不变 -->
  </div>

  <!-- 非全屏时的视频信息区域 -->
  <div v-if="isVideo && !playerError && !isFullscreen" class="video-info">
    <h3>{{ fileName }}</h3>
    <p v-if="filePath" class="video-path">{{ filePath }}</p>
  </div>

  <div v-if="isAudio" class="audio-player-wrapper">
    <!-- 音频 UI 不变 -->
  </div>
</div>
```

**CSS**：
```css
.video-player {
  width: 100%;
  background: #000;
  /* 不设 aspect-ratio，让 ArtPlayer autoSize 控制 */
}

.video-info {
  padding: 16px;
  border-bottom: 1px solid var(--ion-color-light);
}

.video-path {
  font-size: 12px;
  color: var(--encv-text-secondary);
  word-break: break-all;
  margin-top: 4px;
}
```

**ArtPlayer 初始化**：
```ts
function initArtPlayer() {
  if (!artContainer.value || !streamUrl.value) return

  art = new Artplayer({
    container: artContainer.value,
    url: streamUrl.value,
    autoplay: true,
    autoSize: true,
    autoMini: true,
    mutex: true,
    playsInline: true,
    theme: '#ffad00',
    volume: 0.7,
    fullscreen: true,
    miniProgressBar: true,
  })

  art.on('video:loadedmetadata', () => {
    // autoSize 会自动调整容器尺寸
    console.info('[Player] Video metadata loaded, autoSize applied')
  })

  art.on('fullscreen', () => {
    isFullscreen.value = true
  })

  art.on('fullscreenExit', () => {
    isFullscreen.value = false
  })

  art.on('error', () => {
    console.error('[Player] ArtPlayer playback error')
    playerError.value = true
    handlePlayerError()
  })

  console.info('[Player] ArtPlayer initialized')
}
```

新增 `isFullscreen` ref。

---

## 问题 1：文件搜索

### 方案

分为后端和前端两部分。

### 后端：新增搜索 API

#### `internal/service/mobile_service.go` — 新增 `SearchFiles` 方法

```go
func (s *MobileService) SearchFiles(queryPath string, keyword string, recursive bool) ([]FileInfo, error) {
    absPath, err := utils.SafeURLToAbsPath(s.servingDir, queryPath)
    if err != nil {
        return nil, &ForbiddenError{Err: err}
    }

    var results []FileInfo
    keyword = strings.ToLower(keyword)

    if recursive {
        err = filepath.WalkDir(absPath, func(path string, d fs.DirEntry, err error) error {
            if err != nil { return nil } // 跳过无权限目录
            if strings.HasPrefix(d.Name(), ".") { 
                if d.IsDir() { return fs.SkipDir }
                return nil 
            }
            if strings.Contains(strings.ToLower(d.Name()), keyword) {
                relPath, _ := filepath.Rel(absPath, path)
                urlPath := queryPath
                if queryPath == "/" { urlPath = "" }
                urlPath += "/" + relPath
                info, _ := d.Info()
                results = append(results, FileInfo{
                    Name: d.Name(),
                    Path: urlPath,
                    IsDirectory: d.IsDir(),
                    Size: func() int64 { if info != nil { return info.Size() }; return 0 }(),
                    Modified: func() string { if info != nil { return info.ModTime().Format(time.RFC3339) }; return "" }(),
                })
            }
            return nil
        })
    } else {
        // 仅当前目录
        entries, err := os.ReadDir(absPath)
        if err != nil { return nil, err }
        for _, entry := range entries {
            if strings.HasPrefix(entry.Name(), ".") { continue }
            if strings.Contains(strings.ToLower(entry.Name()), keyword) {
                // 同 ListFiles 逻辑
            }
        }
    }
    return results, nil
}
```

#### `internal/server/mobile_api.go` — 新增搜索 handler

```go
func (s *Server) handleSearchFilesAPI(w http.ResponseWriter, r *http.Request) {
    queryPath := r.URL.Query().Get("path")
    keyword := r.URL.Query().Get("keyword")
    recursive := r.URL.Query().Get("recursive") == "true"

    files, err := s.mobileSvc.SearchFiles(queryPath, keyword, recursive)
    // ...
}
```

#### `internal/server/server.go` — 注册路由

添加 `/api/files/search` 路由。

### 前端

#### `src/api/encv.ts` — 新增搜索 API

```ts
export async function searchFiles(path: string, keyword: string, recursive = false): Promise<FileItem[]> {
  const baseUrl = getApiBaseUrl()
  const params = new URLSearchParams({
    path,
    keyword,
    recursive: String(recursive),
  })
  const response = await fetch(`${baseUrl}/api/files/search?${params}`)
  if (!response.ok) { throw new Error(`HTTP error! status: ${response.status}`) }
  const data = await response.json()
  return data.files || []
}
```

#### Files.vue — 搜索 UI

1. 在 header 区域添加 `ion-searchbar` 组件
2. 搜索状态管理：
   - `searchQuery` ref：搜索关键词
   - `searchRecursive` ref：是否递归搜索
   - `searchResults` ref：搜索结果
   - `isSearching` ref：搜索中状态
3. 搜索逻辑：
   - 输入防抖 300ms
   - 空关键词时恢复原始文件列表
   - 有关键词时调用 `searchFiles` API
4. 搜索结果列表复用现有 `ion-list`，显示匹配文件（包含完整路径）
5. 递归搜索开关：搜索框右侧小图标或 toggle

#### 搜索缓存策略

- 前端缓存：使用 `Map<string, { timestamp: number, results: FileItem[] }>` 缓存搜索结果
- 缓存 key：`${path}:${keyword}:${recursive}`
- 缓存有效期：30 秒
- 目录变更时（`file:change` 事件）清除相关缓存

---

## 文件变更清单

| 文件 | 操作 | 变更内容 |
|------|------|---------|
| `src/directives/longpress.ts` | 新建 | 自定义 v-longpress 指令 |
| `src/composables/useFilePicker.ts` | 修改 | 改用回调模式替代 Promise |
| `src/views/Files.vue` | 修改 | 搜索框 + v-longpress 替换 @longpress + picker 导航修复 |
| `src/views/Tasks.vue` | 修改 | handleBrowse 导航到 Files + 回调模式 |
| `src/views/Player.vue` | 修改 | 移除 aspect-ratio + 全屏支持 + 视频信息区域 |
| `src/api/encv.ts` | 修改 | 新增 searchFiles API |
| `src/composables/useI18n.ts` | 修改 | 添加搜索和播放器相关 i18n |
| `internal/service/mobile_service.go` | 修改 | 新增 SearchFiles 方法 |
| `internal/server/mobile_api.go` | 修改 | 新增搜索 handler |
| `internal/server/server.go` | 修改 | 注册搜索路由 |

## 实施顺序

1. 修复 Bug：长按菜单（v-longpress 指令）+ 文件选择器（回调模式 + 导航）
2. 修复播放器：竖屏视频 + 全屏 + 视频信息
3. 后端搜索 API
4. 前端搜索 UI
5. i18n 文案
6. 构建验证
