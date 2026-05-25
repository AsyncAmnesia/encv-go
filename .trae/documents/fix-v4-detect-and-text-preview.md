# 修复计划：容器版本显示 + V4 检测 + 文本预览

## 问题 1：加密解密任务卡片增加容器版本显示

**现状**：`MobileTask` 结构没有容器版本字段，任务卡片只显示加密/解密类型和状态。

**修改**：

1. **`internal/service/task_manager.go`**：`MobileTask` 添加 `ContainerVersion int` 字段（`json:"containerVersion,omitempty"`）
2. **`internal/service/task_manager.go`**：`processEncrypt` 完成后，检测输出文件的 Header 版本，写入 `task.ContainerVersion`
3. **`app/encv-mobile/src/api/encv.ts`**：`EncvTask` 接口添加 `containerVersion?: number`
4. **`app/encv-mobile/src/views/Tasks.vue`**：任务卡片在 `completed-info` 区域显示容器版本（如 `V4`）

## 问题 2：新加密的 V4 容器无法识别

**根因**：`SingleFileContainerWriter.Close()` V4 分支中，`ManifestOffset` 指向 BlockHeader 的起始位置，但 `ManifestLength` 只是加密数据的长度（不含 BlockHeader 的 16 字节）。`OpenV4Container` 从 `ManifestOffset` 读 `ManifestLength` 字节，实际读到的是 BlockHeader + 部分加密数据，导致 `DeobfuscateManifest` 失败。

**修复**：

1. **`internal/v2/writer/single_file_container_writer.go`**：V4 Close() 中 `ManifestOffset` 应指向加密数据（跳过 BlockHeader），或者 `ManifestLength` 应包含 BlockHeader 大小。选择方案 B（`ManifestLength` 包含 BlockHeader），因为 `OpenV4Container` 直接读 `ManifestLength` 字节然后 `DeobfuscateManifest`，需要跳过 BlockHeader。

   实际上最正确的做法是：`ManifestOffset` 指向 BlockHeader 开始，`ManifestLength` = BlockHeader + 加密数据总长度。然后 `OpenV4Container` 读取后跳过 BlockHeader 再 `DeobfuscateManifest`。

   但更简单的修复是：让 `ManifestOffset` 指向加密数据开始（跳过 BlockHeader），`ManifestLength` = 加密数据长度。这样 `OpenV4Container` 不需要改。

2. **`internal/v2/physical/file_chunker.go`**：V4 finalize 中的 `ManifestOffset` 同样需要修正。

## 问题 3：文本预览一直加载中

**根因**：移动端用 `<iframe :src="streamUrl">` 加载 `/stream?path=xxx.sccgt`，但 `/stream` 返回的 Content-Type 是 `application/octet-stream`（因为文件扩展名是 `.sccgt`），浏览器不会渲染 `application/octet-stream` 内容，导致 iframe 一直加载或空白。

**桌面端做法**：桌面端使用 `text.html` 预览页面，JS 调用 `/decrypt?file=xxx` 获取解密文本，`response.text()` 读取内容后设置到 `<pre>` 元素。

**修复方案**：移动端文本预览改回 `readFileContent` API 方式（与桌面端思路一致），但需要优化性能：
- 对于加密文件：用 `/api/file?path=xxx` 读取解密后的文本内容（后端已有此 API）
- 对于非加密文件：同样用 `/api/file?path=xxx`

但用户之前说"加载异常久"，那是因为 `readFileContent` 读取大文件很慢。桌面端用 `/decrypt` 端点也是流式的，但用 `fetch().text()` 逐步读取。

**最终方案**：改回 `readFileContent`，但后端 `/api/file` 端点需要支持加密容器解密。检查当前 `/api/file` 是否支持加密文件——如果不支持，需要添加支持。或者直接用 `/stream` 但设置正确的 Content-Type。

**最优方案**：修改 `/stream` 端点，对于加密容器文件，根据原始文件扩展名设置正确的 Content-Type（如 `.txt` → `text/plain`，`.json` → `application/json`，`.md` → `text/markdown`），这样 iframe 就能正确渲染。

具体修改：
1. **`internal/v2/handler/content.go`**：`ServeFile` 中，从 `prov.GetName()` 获取原始文件名，去掉加密扩展名（`.sccgt`/`.sccgv` 等）后获取真实扩展名，用真实扩展名查 Content-Type
2. **`app/encv-mobile/src/views/FilePreview.vue`**：文本预览改回 iframe + `/stream`，但后端返回正确的 Content-Type

---

## 实施步骤

### Step 1: 修复 V4 ManifestOffset/ManifestLength 不匹配

**文件**: `internal/v2/writer/single_file_container_writer.go`
- V4 Close() 中，`ManifestOffset` 应指向加密数据开始（`manifestOffset + blockHeaderSize`），`ManifestLength` 保持为加密数据长度

**文件**: `internal/v2/physical/file_chunker.go`
- V4 finalize 中同样修正

### Step 2: 修复 /stream Content-Type

**文件**: `internal/v2/handler/content.go`
- 添加辅助函数：从加密容器文件名提取原始扩展名（去掉 `.sccgt`/`.sccgv`/`.sccgi`/`.sccga`/`.sccgpdf`/`.sccgwps` 后缀）
- `ServeFile` 中用原始扩展名查 Content-Type

### Step 3: 文本预览恢复 iframe + /stream

**文件**: `app/encv-mobile/src/views/FilePreview.vue`
- 确认文本预览使用 iframe + streamUrl（当前已是，无需改）

### Step 4: 任务卡片增加容器版本

**文件**: `internal/service/task_manager.go`
- `MobileTask` 添加 `ContainerVersion int`
- `processEncrypt` 完成后检测输出文件 Header 版本

**文件**: `app/encv-mobile/src/api/encv.ts`
- `EncvTask` 添加 `containerVersion?: number`

**文件**: `app/encv-mobile/src/views/Tasks.vue`
- completed-info 区域显示容器版本

### Step 5: 验证

- 前端构建 `vue-tsc --noEmit && vite build`
- 后端编译 `go build ./internal/... ./cmd/encv/...`
- 重启后端
- 测试新加密 V4 容器检测
- 测试文本预览
