# 三项修复计划

## 一、文件长按菜单增加"查看信息"

### 问题
当前 Files.vue 长按菜单（[Files.vue:458-547](file:///workspace/app/encv-mobile/src/views/Files.vue#L458-L547)）只有：打开/加密（目录）、播放/解密/删除（加密文件）、预览或播放/加密/删除（普通文件）。缺少"查看信息"入口。

### 方案

#### 1. 后端新增 `/api/file/info` 接口

在 [mobile_api.go](file:///workspace/internal/server/mobile_api.go) 新增 handler，返回文件的详细信息：

**普通文件返回：**
```json
{
  "name": "test.txt",
  "path": "/files/test.txt",
  "size": 1234,
  "modified": "2025-01-01T00:00:00Z",
  "mime_type": "text/plain",
  "category": "document",
  "is_directory": false,
  "is_encrypted": false,
  "is_encv_container": false
}
```

**ENCV 加密容器额外返回（调用 `OpenV4Container`）：**
```json
{
  ...普通字段,
  "is_encv_container": true,
  "container": {
    "version": 4,
    "container_id": "uuid",
    "container_type": "video",
    "is_seekable": true,
    "original_duration": 120.5,
    "segment_count": 3,
    "segments": [...],
    "manifest_size": 1024,
    "header": { "flags": 0, "manifest_offset": 2048, "manifest_length": 1024 }
  }
}
```

实现要点：
- 复用已有的 `detector.IsEncvContainerFromBytes` 或直接检查 `.encv` 扩展名 + magic bytes
- 调用 `reader.OpenV4Container(path, password)` 获取容器元数据（不读取实际内容，只读 header + manifest）
- 密码从全局配置获取 `(cfg.Password)`
- 新增 API 函数 `fetchFileInfo(path)` 到 [encv.ts](file:///workspace/app/encv-mobile/src/api/encv.ts)

#### 2. 前端新增 FileInfo.vue 页面

路由：`/tabs/file-info?path=xxx&name=xxx`

页面结构：
```
┌─────────────────────────────┐
│ ←  文件信息                  │
├─────────────────────────────┤
│ 📄 基本信息                  │
│   名称: test.txt            │
│   路径: /files/test.txt     │
│   大小: 1.2 KB              │
│   修改时间: 2025-01-01       │
│   类型: text/plain           │
├─────────────────────────────┤
│ 🔒 ENCV 容器信息（仅容器）    │
│   版本: V4                   │
│   容器 ID: xxx              │
│   类型: video                │
│   可寻址: 是                 │
│   原始时长: 02:00            │
│   分段数: 3                  │
├─────────────────────────────┤
│ 📋 清单数据（仅容器，可折叠）   │
│   { JSON 格式化显示 }         │
└─────────────────────────────┘
```

#### 3. Files.vue 长按菜单添加选项

在所有分类的 buttons 数组最前面插入"信息"按钮：
```typescript
buttons.push({
  text: t('files.info'),
  icon: informationCircle,
  handler: () => {
    router.push({
      path: '/tabs/file-info',
      query: { path: file.path, name: file.name },
    })
  },
})
```

#### 4. 路由注册

[router/index.ts](file:///workspace/app/encv-mobile/src/router/index.ts) 添加：
```typescript
{
  path: 'file-info',
  component: () => import('@/views/FileInfo.vue'),
}
```

#### 5. i18n

新增 key：`files.info`、`fileInfo.*` 系列

---

## 二、ff_graph_css_data 符号缺失根因分析

### 现状
构建脚本 [build-ffmpeg-android.sh:273,305](file:///workspace/app/encv-mobile/scripts/build-ffmpeg-android.sh#L273) 已有 `-Wl,--undefined=ff_graph_css_data`，但运行时仍报符号缺失。

### 根因分析

**`-Wl,--undefined=SYMBOL` 的语义是："如果链接后 SYMBOL 仍然未定义，报错"。它并不强制链接器保留该符号。**

真正的问题链：

1. **FFmpeg 8.0 的 `ff_graph_css_data` 定义在哪里？**
   - 定义在 `libavfilter` 的某个编译单元中（可能是 `graph/graph.c` 或 `vf_*` 过滤器的 .c 文件）
   - 它是一个 **被间接引用的数据符号**——没有函数直接 `extern` 引用它，而是通过函数指针表或注册机制间接使用

2. **`--gc-sections` 的行为：**
   - 链接器看到没有任何对象文件**直接引用** `ff_graph_css_data`
   - 即使有 `--undefined`，如果符号根本不在输入的 `.a` 库中，`--undefined` 只会让链接失败，不会凭空创造符号
   - 但如果符号**存在**于 `.a` 中但被 gc-sections 判定为不可达，`--undefined` 应该能保住它

3. **最可能的根因：CI 缓存的旧 .so**
   - 构建脚本第 34-46 行有缓存检测逻辑：如果 `libffmpeg.so` 已存在且包含 `ffmpeg_run` 符号就跳过构建
   - **修改了链接参数但没有触发重建**——缓存的旧 .so 没有这个符号保护
   - 用户设备上的 .so 可能是在加 `--undefined` 之前构建的

### 修复方案

**方案 A（推荐）：强制重建 + 改用 `-Wl,--defsym` 保底**
```bash
# 1. 缓存检测增加符号校验
# 把缓存检测中的符号检查从只查 ffmpeg_run 改为同时查 ff_graph_css_data
if ${NM} -D "${OUTPUT_DIR}/libffmpeg.so" | grep -q "ffmpeg_run" && \
   ${NM} -D "${OUTPUT_DIR}/libffmpeg.so" | grep -q "ff_graph_css_data"; then

# 2. 双保险：--undefined（防止被 gc）+ 如果符号不存在则用 defsyn 定义弱空版本
-Wl,--undefined=ff_graph_css_data
# 并在 fftools 编译前检查符号是否存在，不存在时生成一个 stub
```

**方案 B：找到定义位置确保被编译**
- 在 FFMPEG_SRC 中 grep `ff_graph_css_data` 确认它在哪个 .c 文件
- 确保该 .c 文件被包含在 FFMPEG_FFTOOLS 或 STATIC_LIBS 的编译列表中
- 如果它在 libavfilter 的条件编译中被排除（因为 configure 时禁用了相关 filter），需要重新考虑 configure 参数

### 执行步骤
1. 在 FFmpeg 源码目录中搜索 `ff_graph_css_data` 确认定义位置
2. 确认该符号所在的 .c / .o 是否被正确编入 libavfilter.a
3. 更新缓存检测逻辑，加入 `ff_graph_css_data` 校验
4. 如果确认是缓存问题，提供清理缓存重建的方法

---

## 三、文件预览机制修复

### 问题根因

[FilePreview.vue:98-106](file:///workspace/app/encv-mobile/src/views/FilePreview.vue#L98-L106) 的 `determinePreviewType()` 分类逻辑有缺陷：

```typescript
function determinePreviewType(name: string): PreviewType {
  const category = getFileCategory(name)  // .txt → 'document', .encv → 'encrypted'
  const ext = getFileExtension(name)

  if (category === 'image') return 'image'
  if (ext === 'pdf') return 'pdf'
  if (category === 'other') return 'text'   // ❌ 只有 'other' 能预览文本
  return 'unsupported'                      // ❌ document(.txt) → unsupported!
}
```

**问题清单：**

| 文件类型 | getCategory 返回 | determinePreviewType 结果 | 应该 |
|---------|-------------------|--------------------------|------|
| `test.txt` | `'document'` | `'unsupported'` ❌ | `'text'` |
| `test.md` | `'other'` | `'text'` ✅ | `'text'` |
| `test.log` | `'other'` | `'text'` ✅ | `'text'` |
| `test.csv` | `'other'` | `'text'` ✅ | `'text'` |
| `video.encv` | `'encrypted'` | `'unsupported'` ❌ | 容器信息页 |
| `data.json` | `'other'` | `'text'` ✅ | `'text'` |
| `code.py` | `'other'` | `'text'` ✅ | `'text'` |

另外后端 [mobile_service.go:207](file:///workspace/internal/service/mobile_service.go#L207) 的 `ReadFileContent` 对 .encv 文件直接 `os.ReadFile()` 会返回二进制乱码。

### 修复方案

#### 1. 前端 determinePreviewType 重写

```typescript
type PreviewType = 'image' | 'pdf' | 'text' | 'video' | 'audio' | 'container' | 'unsupported'

const TEXT_EXTENSIONS = new Set([
  'txt', 'md', 'csv', 'log', 'ini', 'toml', 'yaml', 'yml', 'json',
  'xml', 'html', 'htm', 'css', 'js', 'ts', 'py', 'java', 'c', 'cpp',
  'h', 'hpp', 'go', 'rs', 'sh', 'bat', 'sql', 'conf', 'env', 'gitignore',
  'dockerfile', 'makefile', 'cmake', 'proto', 'graphql', 'vue', 'jsx', 'tsx',
])

function determinePreviewType(name: string, isEncrypted?: boolean): PreviewType {
  const category = getFileCategory(name, isEncrypted)
  const ext = getFileExtension(name)

  if (category === 'image') return 'image'
  if (ext === 'pdf') return 'pdf'
  if (category === 'video') return 'video'
  if (category === 'audio') return 'audio'
  if (category === 'encrypted' || ext === 'encv') return 'container'
  if (TEXT_EXTENSIONS.has(ext)) return 'text'
  if (category === 'other' || category === 'document') return 'text'
  return 'unsupported'
}
```

关键改动：
- `'document'` 类别也走文本预览（覆盖 .txt, .doc 等可读文本）
- 显式 `TEXT_EXTENSIONS` 白名单确保常见文本格式都能预览
- 加密容器返回新的 `'container'` 类型

#### 2. FilePreview.vue 增加 container 类型渲染

新增 `previewType === 'container'` 分支：
- 调用 `fetchFileInfo(path)` 获取容器元数据
- 显示容器基本信息卡片（版本、ID、类型、分段数等）
- 可折叠显示完整 manifest JSON

#### 3. 后端 ReadFileContent 增加密容器检测

[mobile_service.go:178+](file:///workspace/internal/service/mobile_service.go#L178)：
- 在 `os.ReadFile` 前，先检查是否为 ENCV 容器（magic bytes 或扩展名）
- 如果是容器，返回错误提示 `"use /api/file/info endpoint for container metadata"` 而不是二进制垃圾
- 或者直接在这里返回结构化的容器摘要信息

#### 4. Files.vue 点击导航优化

[Files.vue:394-401](file:///workspace/app/encv-mobile/src/views/Files.vue#L394-L401) 当前点击加密文件直接 `playMedia()`。对于非视频类加密容器（如果有文本/文档类加密），应该也能进入预览/信息页。

---

## 执行顺序

1. **先修预览机制**（问题 3）——影响面最大，代码改动集中在前端
2. **再加文件信息功能**（问题 1）——依赖后端新接口
3. **最后排查 FFmpeg 符号**（问题 2）——需要确认是缓存还是真正的编译缺失
