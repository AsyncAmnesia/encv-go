# 实施计划：文件信息显示修复 + 加密解密逻辑修正 + Artplayer 竖屏黑边

## 问题 1：加密容器文件信息页不显示容器信息和清单

### 根因分析

经过代码审查，发现以下可能的问题链：

**问题 A：`GetFileInfo` 只通过扩展名 `.encv` 判断容器，但 `ListFiles` 通过内容检测**
- `ListFiles` 用 `detector.DetectContainer()` 检测文件内容 → `isEncrypted: true`
- `GetFileInfo` 用 `ext == ".encv"` 判断 → 如果文件扩展名不是 `.encv`（如 `.mp4.encv` 被截断等），则不读取容器信息
- **修复**：`GetFileInfo` 应同时使用 `detector.DetectContainer()` 检测，与 `ListFiles` 保持一致

**问题 B：`OpenV4Container` 密码为空时可能失败**
- `GetFileInfo` 调用 `reader.OpenV4Container(absPath, s.cfg.Password)`
- 如果全局密码 `s.cfg.Password` 为空，`GenerateKey_v2("", salt, ...)` 生成的 key 不正确
- 但 `OpenV4Container` 只读 header + manifest（deobfuscate 不需要密码），所以密码为空时仍能成功读取元数据
- **结论**：这不是根因，但应增加防御性处理

**问题 C：前端 `FileInfo.vue` 缺少 i18n key**
- 模板中用了 `t('fileInfo.name')`、`t('fileInfo.path')`、`t('fileInfo.size')`、`t('fileInfo.modified')`、`t('fileInfo.category')`
- 但 `useI18n.ts` 中没有定义这些 key
- 虽然用了 `|| 'Name'` 兜底，但不够优雅
- **修复**：补充缺失的 i18n key

**问题 D：后端 `GetFileInfo` 中 `ext` 取自 `queryPath` 而非 `absPath`**
- `filepath.Ext(queryPath)` 和 `filepath.Ext(absPath)` 应该返回相同结果
- 但如果 `queryPath` 有 URL 编码残留（如 `%20`），可能导致扩展名解析错误
- **风险低**，Gin 会自动解码 query 参数

### 实施步骤

#### 1.1 后端 `GetFileInfo` 增加内容检测
在 `mobile_service.go` 的 `GetFileInfo` 方法中，除了检查扩展名，还使用 `detector.DetectContainer()` 检测：

```go
// 现有逻辑
if ext == ".encv" {
    result.IsEncvContainer = true
    ...
}

// 新增：即使扩展名不是 .encv，也通过内容检测判断
if !result.IsEncvContainer {
    if _, detectErr := detector.DetectContainer(absPath); detectErr == nil {
        result.IsEncvContainer = true
        result.IsEncrypted = true
    }
}

// 统一读取容器信息
if result.IsEncvContainer {
    containerInfo, openErr := reader.OpenV4Container(absPath, s.cfg.Password)
    ...
}
```

#### 1.2 前端 `FileInfo.vue` 增加错误容器数据处理
当 `containerData` 存在但包含 `error` 字段时，显示错误信息而非空白：

```html
<div v-if="info.is_encv_container">
  <div v-if="containerData?.error" class="section-card container-card">
    <p class="error-text">{{ containerData.error }}</p>
  </div>
  <div v-else-if="containerData" class="section-card container-card">
    <!-- 正常容器信息 -->
  </div>
</div>
```

#### 1.3 补充缺失的 i18n key
在 `useI18n.ts` 中添加：
```
'fileInfo.name': '文件名' / 'Name'
'fileInfo.path': '路径' / 'Path'
'fileInfo.size': '大小' / 'Size'
'fileInfo.modified': '修改时间' / 'Modified'
'fileInfo.category': '分类' / 'Category'
```

#### 1.4 涉及文件
| 文件 | 改动 |
|------|------|
| `internal/service/mobile_service.go` | `GetFileInfo` 增加内容检测 + 统一容器信息读取 |
| `app/encv-mobile/src/views/FileInfo.vue` | 增加错误容器数据处理 |
| `app/encv-mobile/src/composables/useI18n.ts` | 补充 5 个 fileInfo i18n key |

---

## 问题 2：加密解密逻辑修正

### 需求拆解

1. **移除加密弹窗的密码输入框** — 加密统一使用全局密码，不需要用户手动输入
2. **全局密码为空时加密应当失败** — 前端校验 + 后端校验
3. **覆盖已有文件的逻辑修正** — 需要理解 `recover` 配置项与前端覆盖弹窗的关系

### 当前逻辑分析

**前端 `handleEncryptFile`（Files.vue 第558-617行）：**
1. 读取全局密码 `globalPassword`
2. 弹出 alert，包含目标路径输入框 + 密码输入框（预填全局密码）
3. 用户确认后，检查输出文件是否存在
4. 如果存在，弹出覆盖确认弹窗
5. 创建任务

**前端 `handleDecryptFile`（Files.vue 第619-678行）：**
1. 读取全局密码 `globalPassword`
2. 弹出 alert，包含目标路径输入框 + 密码输入框（预填全局密码）
3. 用户确认后，检查输出文件是否存在
4. 如果存在，弹出覆盖确认弹窗
5. 创建任务

**后端 `getConfigForTask`（task_manager.go 第347-354行）：**
- 如果 `task.Password != ""`，用任务密码覆盖全局密码
- 否则使用全局密码

**后端 `Recover` 配置项（config.go 第20行）：**
- `Recover bool` — "在解密时是否尝试覆盖已有文件"
- 但前端覆盖弹窗是独立于 `Recover` 的，前端总是检查输出文件是否存在并弹窗确认
- **问题**：`Recover` 配置项在后端解密流程中没有被使用（搜索无结果），是一个**死配置**

### 实施步骤

#### 2.1 移除加密弹窗的密码输入框
`handleEncryptFile` 中：
- 移除 `password` 输入框
- 加密时直接使用全局密码（从 config 读取）
- 如果全局密码为空，显示错误提示并阻止操作

```typescript
async function handleEncryptFile(file: FileItem) {
  const parentDir = file.path.substring(0, file.path.lastIndexOf('/')) || '/'
  let globalPassword = ''
  try {
    const cfg = await fetchConfig()
    globalPassword = (cfg as any).password || ''
  } catch {}

  if (!globalPassword) {
    showToast({ message: t('files.noPassword'), duration: 2000, color: 'danger' })
    return
  }

  const alert = await alertController.create({
    header: t('files.encrypt'),
    inputs: [
      {
        name: 'targetPath',
        type: 'text',
        placeholder: t('tasks.targetPathPlaceholder'),
        value: parentDir,
        attributes: { autocomplete: 'off' },
      },
    ],
    buttons: [
      { text: t('files.cancelSelect'), role: 'cancel' },
      {
        text: t('files.encrypt'),
        handler: async (data: Record<string, string>) => {
          const targetPath = (data.targetPath || '').trim()
          // 覆盖检查逻辑保持不变
          ...
          await doCreateTask('encrypt', file.path, targetPath, globalPassword)
        },
      },
    ],
  })
  await alert.present()
}
```

#### 2.2 解密弹窗保留密码输入框
解密可能需要不同密码（文件可能用不同密码加密），所以保留密码输入框，但预填全局密码。

#### 2.3 后端增加密码为空校验
在 `task_manager.go` 的 `processEncrypt` 方法开头增加密码检查：

```go
func (tm *TaskManager) processEncrypt(task *MobileTask, absPath string) {
    // 检查密码
    cfg := tm.getConfigForTask(task, context.Background())
    actualCfg := config.FromContext(cfg)
    if actualCfg.Password == "" {
        tm.failTask(task.ID, "encryption requires a password: global password is empty and no task password provided")
        return
    }
    ...
}
```

注意：`getConfigForTask` 返回的是 `context.Context`，需要从中提取 config。更好的方式是直接检查：

```go
password := tm.cfg.Password
if task.Password != "" {
    password = task.Password
}
if password == "" {
    tm.failTask(task.ID, "encryption requires a password")
    return
}
```

#### 2.4 覆盖弹窗逻辑修正
当前前端总是检查输出文件是否存在并弹窗确认。这个逻辑是合理的，但需要与 `recover` 配置项协调：

- **方案**：前端覆盖弹窗保持不变（总是提示用户确认），`recover` 配置项在后端解密时控制是否自动覆盖（不提示）。由于后端目前没有使用 `recover`，暂时保持前端逻辑不变。

#### 2.5 新增 i18n key
```
'files.noPassword': '请先在设置中配置全局密码' / 'Please set a global password in settings first'
```

#### 2.6 涉及文件
| 文件 | 改动 |
|------|------|
| `app/encv-mobile/src/views/Files.vue` | 加密弹窗移除密码框 + 空密码校验 |
| `internal/service/task_manager.go` | `processEncrypt` 增加密码为空校验 |
| `app/encv-mobile/src/composables/useI18n.ts` | 新增 `files.noPassword` i18n key |

---

## 问题 3：竖屏比例视频 Artplayer 黑边过大

### 根因分析

当前 `ArtPlayerView.vue` 的视频容器逻辑：

1. **初始化时**（第182-183行）：
   ```typescript
   artContainer.value.style.minHeight = '200px'
   artContainer.value.style.maxHeight = `${window.innerHeight - 56}px`
   ```

2. **`video:loadedmetadata` 事件**（第236-243行）：
   ```typescript
   const ratio = video.videoHeight / video.videoWidth
   const containerWidth = artContainer.value?.clientWidth || window.innerWidth
   const naturalHeight = Math.round(containerWidth * ratio)
   const maxHeight = window.innerHeight - 56
   const finalHeight = Math.min(naturalHeight, maxHeight)
   if (artContainer.value) {
     artContainer.value.style.height = `${finalHeight}px`
   }
   ```

3. **Artplayer 配置**：`autoSize: true`

**问题**：对于竖屏视频（如 9:16），`ratio` ≈ 1.78，`naturalHeight` = `containerWidth * 1.78`。如果屏幕宽度为 360px，`naturalHeight` ≈ 640px。`maxHeight` = `window.innerHeight - 56` ≈ 700px。所以 `finalHeight` = 640px。

但 Artplayer 的 `autoSize: true` 会让播放器根据视频原始宽高比调整内部 video 元素大小。Artplayer 默认行为是让 video 元素 `object-fit: contain`，在容器内居中显示。

**真正的问题**：`.video-player` 容器设置了 `width: 100%` 和 `background: #000`，但没有限制高度。Artplayer 的 `autoSize` 会将容器调整为视频的原始宽高比，但如果容器高度被 `maxHeight` 限制，Artplayer 可能在容器内添加黑边。

对于竖屏视频，更合理的做法是：
- 容器宽度不占满屏幕，而是根据视频宽高比计算合适的宽度
- 或者让 Artplayer 使用 `autoHeight` 而非手动设置高度

### 实施步骤

#### 3.1 移除手动高度设置，改用 Artplayer 的 `autoSize`
当前代码在 `video:loadedmetadata` 中手动设置容器高度，与 Artplayer 的 `autoSize` 冲突。应该让 Artplayer 自己管理容器大小。

```typescript
art.on('video:loadedmetadata', () => {
  const video = art?.video
  if (video) {
    if (video.videoWidth && video.videoHeight) {
      mediaInfo.value.resolution = `${video.videoWidth}×${video.videoHeight}`
    }
    if (video.duration && isFinite(video.duration)) {
      mediaInfo.value.duration = formatDuration(video.duration)
    }
  }
  hideNativeControls()
})
```

#### 3.2 添加 CSS 让竖屏视频正确适配
关键：`.video-player` 容器不应强制 `width: 100%`，而应让 Artplayer 根据视频比例自动调整。

```css
.video-player {
  width: 100%;
  background: #000;
  position: relative;
  overflow: hidden;
}

/* 竖屏视频时，让 Artplayer 容器高度自适应 */
:deep(.art-video-player .art-video) {
  object-fit: contain !important;
}
```

实际上，Artplayer 的 `autoSize` 选项会自动调整容器大小以匹配视频比例。问题是我们在 `loadedmetadata` 中又手动设置了高度，覆盖了 Artplayer 的自动调整。

#### 3.3 正确的竖屏视频处理方案
- 移除 `initArtPlayer` 中的 `minHeight`/`maxHeight` 设置
- 移除 `video:loadedmetadata` 中的手动高度计算
- 依赖 Artplayer 的 `autoSize` 自动调整
- 添加 `autoHeight: true` 配置（Artplayer 5.x 的选项）

但 Artplayer 的 `autoSize` 行为是：将容器调整为视频的原始宽高比。对于竖屏视频，这意味着容器会变得很高（宽度不变，高度按比例增加），可能超出屏幕。

更好的方案是：**不使用 `autoSize`，而是让视频在固定容器内用 `object-fit: contain` 显示**。这样竖屏视频会在容器内上下有黑边，但黑边最小化。

等等，用户说"视频上边和左右很大的黑色无效区域"。这意味着竖屏视频在横屏容器中显示，上下左右都有黑边。这是因为：
1. 容器是横屏比例（宽 > 高）
2. 视频是竖屏比例（高 > 宽）
3. `object-fit: contain` 让视频在容器内等比缩放，导致左右有大黑边

**正确方案**：对于竖屏视频，容器应该变成竖屏比例，而不是保持横屏。

#### 3.4 最终方案
保留 `video:loadedmetadata` 中的高度计算，但**同时调整容器宽度**：

对于竖屏视频（`videoHeight > videoWidth`）：
- 容器高度 = `window.innerHeight - 56`（占满可用高度）
- 容器宽度 = 按视频比例计算（`height * videoWidth / videoHeight`）

对于横屏视频：
- 容器宽度 = 屏幕宽度
- 容器高度 = 按视频比例计算（不超过 `maxHeight`）

```typescript
art.on('video:loadedmetadata', () => {
  const video = art?.video
  if (video?.videoWidth && video?.videoHeight) {
    mediaInfo.value.resolution = `${video.videoWidth}×${video.videoHeight}`
    const isPortrait = video.videoHeight > video.videoWidth
    const maxH = window.innerHeight - 56
    const maxW = artContainer.value?.parentElement?.clientWidth || window.innerWidth

    if (isPortrait) {
      const h = maxH
      const w = Math.round(h * video.videoWidth / video.videoHeight)
      if (artContainer.value) {
        artContainer.value.style.width = `${Math.min(w, maxW)}px`
        artContainer.value.style.height = `${h}px`
        artContainer.value.style.margin = '0 auto'
      }
    } else {
      const w = maxW
      const h = Math.round(w * video.videoHeight / video.videoWidth)
      if (artContainer.value) {
        artContainer.value.style.width = `${w}px`
        artContainer.value.style.height = `${Math.min(h, maxH)}px`
      }
    }
  }
  ...
})
```

同时移除 `initArtPlayer` 中的 `minHeight`/`maxHeight` 设置，因为 `loadedmetadata` 会处理。

#### 3.5 涉及文件
| 文件 | 改动 |
|------|------|
| `app/encv-mobile/src/views/ArtPlayerView.vue` | 竖屏视频容器尺寸适配 + 移除冲突的手动高度设置 |

---

## 执行顺序

1. **问题 1**（文件信息显示修复）— 后端 + 前端 + i18n
2. **问题 2**（加密解密逻辑修正）— 前端弹窗 + 后端校验 + i18n
3. **问题 3**（Artplayer 竖屏黑边）— 前端播放器
4. **构建验证** — go vet + vue-tsc + vite build
