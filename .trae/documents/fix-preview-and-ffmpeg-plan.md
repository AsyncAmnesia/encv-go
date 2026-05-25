# 三个问题修复计划（正确理解版）

## 问题概览

| # | 问题 | 根因 |
|---|------|------|
| 1 | 预览加密容器显示元数据而非内容 | `FilePreview.vue` 对所有加密容器显示元数据卡片，应根据 `container_type` 调用对应预览方式 |
| 2 | 容器信息/清单未正确解析 | 需验证 API 调用和数据传递 |
| 3 | ffmpeg/ffprobe 失败无变化 | 旧缓存 .so 仍含 `ff_graph_css_data` 符号，需强制重建 |

---

## 问题 1：加密容器预览应根据类型调用对应插件

### 容器类型系统

后端 `GetFileInfo` 返回 `container_type`：
- `"video"` → 视频容器
- `"audio"` → 音频容器
- `"image"` → 图片容器
- `"document"` → 文档容器

### 流式传输端点

`/stream?path=xxx` 端点会：
1. 检测文件是否是 ENCV 容器
2. 如果是容器，解密并流式传输内容
3. 如果不是容器，直接提供原始文件

### 正确的预览逻辑

根据 `container_type` 决定预览方式：
- **image** → `<img src="/stream?path=xxx">` 显示图片
- **video** → 跳转到 `/player` 播放器
- **audio** → 跳转到 `/player` 播放器
- **document** → PDF 用 iframe，文本用文本预览

### 当前代码问题

`FilePreview.vue` 的 `determinePreviewType()` 只根据 `category === 'encrypted'` 返回 `'container'`，然后在模板中显示元数据卡片。

### 修复方案

**方案**：在 `loadFile()` 中，先调用 `/api/file/info` 获取 `container_type`，然后根据类型设置 `previewType`：

```typescript
async function loadFile() {
  // ... 省略前面代码 ...

  const isEncrypted = route.query.isEncrypted === 'true'

  if (isEncrypted) {
    // 加密文件需要先获取 container_type 来决定预览方式
    try {
      const baseUrl = getApiBaseUrl()
      const resp = await fetch(`${baseUrl}/api/file/info?path=${encodeURIComponent(path)}`)
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
      const info = await resp.json()

      if (info.is_encv_container && info.container) {
        const containerType = info.container.container_type
        containerInfo.value = info.container
        fileSize.value = info.size || 0
        manifestJson.value = JSON.stringify(info.container.manifest || info.container, null, 2)

        // 根据 container_type 设置预览类型
        switch (containerType) {
          case 'image':
            previewType.value = 'image'
            streamUrl.value = getFileStreamUrl(path)
            break
          case 'video':
          case 'audio':
            // 跳转到播放器
            router.push({ path: '/player', query: { path, name: fileName.value } })
            return
          case 'document':
            // 判断是 PDF 还是文本
            const ext = getFileExtension(fileName.value)
            if (ext === 'pdf') {
              previewType.value = 'pdf'
              streamUrl.value = getFileStreamUrl(path)
            } else {
              previewType.value = 'text'
              // 需要读取解密后的文本内容
              const data = await readFileContent(path)
              content.value = data.content
              fileSize.value = data.size
              encoding.value = data.encoding
            }
            break
          default:
            previewType.value = 'container' // 显示元数据
        }
      } else {
        previewType.value = 'unsupported'
      }
    } catch (e: any) {
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
    return
  }

  // 非加密文件的现有逻辑
  previewType.value = await determinePreviewType(fileName.value, isEncrypted)
  // ... 省略后续代码 ...
}
```

---

## 问题 2：容器信息未正确解析

### 验证

后端 `GetFileInfo` 返回正确的 `Container` 数据结构，前端接口定义也正确。问题可能是：
- API 调用失败（之前 `getApiBaseUrl` 问题已修复）
- 网络错误

修复问题 1 后，如果仍有问题需要进一步排查。

---

## 问题 3：ffmpeg/ffprobe 失败无变化

### 根因

之前的修复需要重新构建才能生效。旧缓存 .so 文件仍含 `ff_graph_css_data` 未解析符号。

### 修复方案

在构建脚本缓存检查前添加 `ff_graph_css_data` 符号检测，强制重建：

```bash
echo "=== Checking for cached ffmpeg output ==="
if [ -f "${OUTPUT_DIR}/libffmpeg.so" ] && [ -f "${OUTPUT_DIR}/libffprobe.so" ]; then
    # 检查是否包含已弃用的 ff_graph_css_data 符号
    if ${NM} -D "${OUTPUT_DIR}/libffmpeg.so" 2>/dev/null | grep -q "ff_graph_css_data"; then
        echo "⚠️  Cached libraries contain deprecated ff_graph_css_data symbol, forcing rebuild..."
        rm -f "${OUTPUT_DIR}/libffmpeg.so" "${OUTPUT_DIR}/libffprobe.so"
    elif ${NM} -D "${OUTPUT_DIR}/libffmpeg.so" | grep -q "ffmpeg_run" && \
         ...; then
        echo "✅ All ffmpeg libraries cached and valid, skipping build"
        exit 0
    fi
fi
```

---

## 执行步骤

### Step 1：修复 FilePreview.vue 加密容器预览逻辑

修改 `loadFile()` 函数：
1. 对加密文件先调用 `/api/file/info` 获取 `container_type`
2. 根据 `container_type` 设置正确的 `previewType`
3. image → 显示图片，video/audio → 跳转播放器，document → PDF/文本预览

### Step 2：修复 FFmpeg 构建脚本缓存问题

添加 `ff_graph_css_data` 符号检测强制重建。

### Step 3：验证

在设备上测试各种类型的加密容器预览。

---

## 文件修改清单

| 文件 | 修改内容 |
|------|----------|
| `src/views/FilePreview.vue` | 根据 `container_type` 调用对应预览方式 |
| `scripts/build-ffmpeg-android.sh` | 添加 `ff_graph_css_data` 符号检测强制重建 |
