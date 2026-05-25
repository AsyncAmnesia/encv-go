# 三个 UI/功能问题修复计划

## 问题概览

| # | 问题 | 根因 | 修复方向 |
|---|------|------|----------|
| 1 | 预览加密容器显示的是"信息卡片"而非播放器 | `FilePreview.vue` 对 `previewType === 'container'` 分支渲染的是容器元数据卡片，而非视频/音频播放器 | 添加播放器组件，或跳转到播放页面 |
| 2 | 容器信息和清单信息没有正确解析 | 可能是 API 调用失败（之前 `getApiBaseUrl` 问题），或数据未正确传递 | 验证 API 响应处理逻辑，确保 `containerInfo` 正确赋值 |
| 3 | ffmpeg/ffprobe 失败没有变化 | **之前的修复需要重新构建 FFmpeg 才能生效**。构建脚本有缓存检查，旧 .so 文件仍包含 `ff_graph_css_data` 未解析符号 | 删除旧缓存强制重建，或验证构建脚本是否正确执行 |

---

## 问题 1 详细分析：预览 vs 查看信息

### 当前行为

[FilePreview.vue](src/views/FilePreview.vue) 第 52-95 行：

```vue
<div v-else-if="previewType === 'container'" class="preview-wrapper container-info">
  <div class="container-card">
    <!-- 显示容器元数据：version, container_id, container_type, is_seekable, duration, segment_count -->
  </div>
  <div class="manifest-section">
    <!-- 显示 manifest JSON -->
  </div>
</div>
```

### 用户期望

"预览"加密容器应该是**播放视频/音频内容**，而不是显示元数据信息。元数据应该在"查看信息"（FileInfo.vue）中显示。

### 修复方案

**方案 A（推荐）**：预览加密容器时跳转到播放页面

1. 在 `determinePreviewType()` 中，对 `category === 'encrypted'` 返回 `'unsupported'` 或新类型 `'playback'`
2. 在模板中添加播放器组件或跳转按钮

**方案 B**：在 FilePreview.vue 中嵌入播放器组件

需要引入现有的播放器组件（如果存在），或创建一个简单的视频播放器。

### 推荐方案 A 实现

修改 `determinePreviewType()` 函数：

```typescript
async function determinePreviewType(name: string, isEncrypted?: boolean): Promise<PreviewType> {
  const category = getFileCategory(name, isEncrypted)
  const ext = getFileExtension(name)

  if (category === 'image') return 'image'
  if (ext === 'pdf') return 'pdf'
  // 加密容器不应该在"预览"中显示元数据，而是播放或提示跳转
  if (category === 'encrypted') return 'encrypted'  // 新类型，用于显示播放提示

  const textExts = await fetchTextPreviewExts()
  if (textExts.has(ext)) return 'text'
  if (category === 'document' || category === 'other') return 'text'
  return 'unsupported'
}
```

在模板中添加 `encrypted` 类型的处理：

```vue
<div v-else-if="previewType === 'encrypted'" class="preview-wrapper encrypted-preview">
  <ion-icon :icon="playCircle} class="play-icon"></ion-icon>
  <h3>{{ fileName }}</h3>
  <p class="hint">{{ t('filePreview.encryptedHint') }}</p>
  <ion-button @click="goToPlayer" color="primary">
    <ion-icon :icon="play} slot="start"></ion-icon>
    {{ t('filePreview.play') }}
  </ion-button>
</div>
```

---

## 问题 2 详细分析：容器信息未正确解析

### 可能原因

1. **API 调用失败**：之前 `getApiBaseUrl` 动态导入问题导致 `fetch` 调用失败（已修复）
2. **数据结构不匹配**：后端返回的字段名与前端期望不一致
3. **容器打开失败**：后端 `reader.OpenV4Container` 返回错误，导致 `Container` 字段只有 `error` 键

### 验证步骤

1. 检查后端 `GetFileInfo` 返回的数据结构（已确认正确）：
   ```go
   result.Container = map[string]interface{}{
       "version":           4,
       "container_id":      mf.ContainerID,
       "container_type":    containerTypeStr,
       "is_seekable":       hdr.IsSeekable == 1,
       "original_duration": mf.OriginalDuration,
       "segment_count":     len(mf.Segments),
       "segments":          mf.Segments,
       "manifest_size":     hdr.ManifestLength,
       "header":            {...},
       "manifest":          mf,
   }
   ```

2. 检查前端 `ContainerInfo` 接口定义（已确认正确）：
   ```typescript
   interface ContainerInfo {
     version: number
     container_id: string
     container_type: string
     is_seekable: boolean
     original_duration?: number
     segment_count?: number
     segments: unknown[]
   }
   ```

3. 检查前端数据赋值逻辑（已确认正确）：
   ```typescript
   containerInfo.value = info.container || null
   manifestJson.value = JSON.stringify(info.container.manifest || info.container, null, 2)
   ```

### 结论

问题 2 应该已随问题 1（`getApiBaseUrl` 修复）自动解决。如果仍有问题，需要：
- 在浏览器开发者工具中检查 `/api/file/info?path=...` 的实际响应
- 检查是否有 CORS 错误或网络错误

---

## 问题 3 详细分析：ffmpeg/ffprobe 失败没有变化

### 根因

之前的修复（移除 `-Wl,--undefined=ff_graph_css_data`）**需要重新执行构建脚本才能生效**。

构建脚本的缓存检查逻辑（第 36-43 行）：

```bash
if ${NM} -D "${OUTPUT_DIR}/libffmpeg.so" | grep -q "ffmpeg_run" && \
   ${NM} -D "${OUTPUT_DIR}/libffmpeg.so" | grep -q "ffmpeg_reset" && \
   ${NM} -D "${OUTPUT_DIR}/libffprobe.so" | grep -q "ffprobe_run" && \
   ${NM} -D "${OUTPUT_DIR}/libffprobe.so" | grep -q "ffprobe_reset"; then
    echo "✅ All ffmpeg libraries cached and valid, skipping build"
    exit 0
```

**关键点**：如果旧的 `libffmpeg.so` / `libffprobe.so` 文件存在且包含 `ffmpeg_run`/`ffprobe_run`/`ffmpeg_reset`/`ffprobe_reset` 符号，脚本会跳过构建。

但旧文件仍然包含 `ff_graph_css_data` 作为未解析符号，导致 dlopen 失败。

### 修复方案

**方案 A**：删除旧缓存强制重建

在构建脚本开头添加强制重建检查：

```bash
# 检查是否需要强制重建（移除 ff_graph_css_data 符号）
if [ -f "${OUTPUT_DIR}/libffmpeg.so" ]; then
    if ${NM} -D "${OUTPUT_DIR}/libffmpeg.so" | grep -q "ff_graph_css_data"; then
        echo "⚠️  Cached libraries contain deprecated ff_graph_css_data symbol, forcing rebuild..."
        rm -f "${OUTPUT_DIR}/libffmpeg.so" "${OUTPUT_DIR}/libffprobe.so"
    fi
fi
```

**方案 B**：用户手动删除旧文件

```bash
rm -f android/app/src/main/jniLibs/arm64-v8a/libffmpeg.so
rm -f android/app/src/main/jniLibs/arm64-v8a/libffprobe.so
```

然后重新执行构建脚本。

### 推荐方案 A 实现

在构建脚本第 33 行后添加检查逻辑。

---

## 执行步骤

### Step 1：修复 FilePreview.vue 预览逻辑

1. 修改 `PreviewType` 类型定义，添加 `'encrypted'` 类型
2. 修改 `determinePreviewType()` 函数，对加密容器返回 `'encrypted'`
3. 在模板中添加 `encrypted` 类型的处理分支，显示播放提示和跳转按钮
4. 添加 `goToPlayer()` 函数，跳转到播放页面

### Step 2：修复 FFmpeg 构建脚本缓存问题

1. 在缓存检查前添加 `ff_graph_css_data` 符号检测
2. 如果检测到该符号，删除旧缓存强制重建

### Step 3：验证

1. 删除旧的 FFmpeg 缓存文件
2. 重新执行构建脚本
3. 在设备上测试：
   - 预览加密容器应显示播放提示
   - 查看信息应正确显示容器元数据
   - 加密视频应能正常处理

---

## 文件修改清单

| 文件 | 修改内容 |
|------|----------|
| `src/views/FilePreview.vue` | 添加 `encrypted` 预览类型，显示播放提示而非元数据卡片 |
| `scripts/build-ffmpeg-android.sh` | 添加 `ff_graph_css_data` 符号检测，强制重建旧缓存 |
