# 修复计划：加密容器预览 + V3 容器读取 + 通用信息卡片

## 问题概览

| # | 问题 | 根因 |
|---|------|------|
| 1 | 加密容器预览不显示文件通用信息卡片 | FilePreview.vue 的加密容器分支没有渲染文件基本信息（名称、大小、修改时间等） |
| 2 | logcat.txt.sccgt 无法预览 | **所有插件都使用 `HeaderVersion: 3` 写入容器，但 `GetFileInfo` 只调用 `OpenV4Container`**，V3 容器无法被读取 |
| 3 | FFmpeg 构建脚本缓存问题 | 旧缓存 .so 仍含 `ff_graph_css_data` 符号（已在之前修复，此处不再重复） |

---

## 问题 2 详细分析：V3 容器无法被 GetFileInfo 读取

### 根因链路

1. 所有插件（video/audio/image/text/pdf/wps）的 `HeaderVersion` 都是 `3`
2. `mobile_service.go:302` 调用 `reader.OpenV4Container(absPath, s.cfg.Password)`
3. `OpenV4Container` 调用 `types.ReadHeaderV4(f)`，该方法：
   - 读取 2048 字节 header
   - 校验 CRC32
   - 检查 Magic == `ENVC`（通过）
   - **但 V3 header 的字段布局与 V4 不同**：
     - V3 没有 `ContainerType`、`IsSeekable`、`ManifestOffset`、`ManifestLength` 字段
     - V3 的 `Flags` 字段在偏移 6-8，V4 的 `Flags` 也在 6-8 但含义不同
   - CRC32 校验可能通过（因为数据确实被正确写入），但解析出的字段值是错误的
   - `ManifestOffset` 和 `ManifestLength` 被解析为 V3 SpecialID 区域的随机数据
   - `DeobfuscateManifest` 读到的数据太短（< 16 字节 salt），报错 "obfuscated data too short to contain salt"

### 修复方案

修改 `mobile_service.go` 的 `GetFileInfo`，先检测容器版本，再根据版本选择不同的读取方式：

1. 使用 `types.DetectHeaderInfoFromReaderAt` 检测版本
2. V4 容器：使用现有的 `OpenV4Container`
3. V3 容器：使用 `detector.DetectContainerType` 获取 container_type，使用 `reader.NewDecryptReaderFactory` 获取 manifest 信息

```go
if isContainer {
    result.IsEncvContainer = true
    result.IsEncrypted = true
    result.Category = "encrypted"

    // 检测容器版本
    containerVersion, _, detectErr := detector.DetectContainerVersion(absPath)

    if containerVersion == 4 {
        // V4 容器：使用 OpenV4Container
        containerInfo, openErr := reader.OpenV4Container(absPath, s.cfg.Password)
        if openErr != nil {
            // ... error handling
        }
        // ... 现有的 V4 处理逻辑
    } else {
        // V3 容器：使用 DecryptReaderFactory
        factory, factoryErr := reader.NewDecryptReaderFactory(absPath, s.cfg.Password)
        if factoryErr != nil {
            // ... error handling
        }
        defer factory.Close()

        containerType, _ := detector.DetectContainerType(absPath)
        isSeekable := factory.IsSeekable()
        index := factory.GetIndex()
        originalSize := factory.GetOriginalSize()

        // 从 manifest 获取 segments 信息
        // ...
    }
}
```

但更简洁的方案是：**让 `OpenV4Container` 也支持 V3 容器**，或者创建一个通用的 `OpenContainer` 函数。

### 推荐方案：修改 GetFileInfo 使用通用读取方式

修改 `mobile_service.go`，对 V3 容器使用 `DecryptReaderFactory` + `DetectContainerType`：

```go
if isContainer {
    result.IsEncvContainer = true
    result.IsEncrypted = true
    result.Category = "encrypted"

    // 先尝试 V4 容器
    containerInfo, openErr := reader.OpenV4Container(absPath, s.cfg.Password)
    if openErr == nil {
        // V4 容器处理（现有逻辑）
        hdr := containerInfo.Header
        mf := containerInfo.Manifest
        // ... 现有代码
    } else {
        // V4 失败，尝试 V3 通用方式
        slog.Info("GetFileInfo: V4 open failed, trying generic reader", "path", queryPath, "error", openErr)

        containerType, typeErr := detector.DetectContainerType(absPath)
        if typeErr != nil {
            slog.Warn("GetFileInfo: cannot detect container type", "path", queryPath, "error", typeErr)
            result.Container = map[string]interface{}{
                "error": "cannot read container metadata: " + openErr.Error(),
            }
            return result, nil
        }

        isSeekable, seekErr := detector.DetectIsSeekable(absPath)
        if seekErr != nil {
            isSeekable = false
        }

        var containerTypeStr string
        switch containerType {
        case 1:
            containerTypeStr = "video"
        case 2:
            containerTypeStr = "audio"
        case 3:
            containerTypeStr = "image"
        case 4:
            containerTypeStr = "document"
        default:
            containerTypeStr = fmt.Sprintf("unknown(%d)", containerType)
        }

        result.Container = map[string]interface{}{
            "version":        3,
            "container_type": containerTypeStr,
            "is_seekable":    isSeekable,
        }
    }
}
```

---

## 问题 1 详细分析：加密容器预览缺少通用信息卡片

### 当前 FilePreview.vue 的加密容器分支

加密容器进入 `if (isEncrypted)` 分支后，只获取了 `containerInfo` 和 `manifestJson`，但没有显示文件基本信息（名称、大小、修改时间、MIME 类型等）。

### 修复方案

在 `loadFile()` 的加密分支中，也设置文件基本信息。在模板中，对加密容器也显示通用信息卡片（类似 FileInfo.vue 的第一个 section-card）。

---

## 执行步骤

### Step 1：修改 mobile_service.go 支持读取 V3 容器信息

1. 在 `OpenV4Container` 失败时，使用 `detector.DetectContainerType` 和 `detector.DetectIsSeekable` 获取 V3 容器信息
2. 返回基本的 container_type、is_seekable 等字段

### Step 2：修改 FilePreview.vue 显示通用信息卡片

1. 在加密容器预览中添加文件基本信息显示（名称、大小、修改时间等）
2. 在 API 响应中已有这些字段（name、size、modified、mime_type、category），只需在模板中渲染

### Step 3：验证

1. 重启后端
2. 测试 V3 容器（logcat.txt.sccgt）的预览
3. 测试各种类型加密容器的预览

---

## 文件修改清单

| 文件 | 修改内容 |
|------|----------|
| `internal/service/mobile_service.go` | `GetFileInfo` 支持 V3 容器：V4 失败后降级到 `DetectContainerType` + `DetectIsSeekable` |
| `src/views/FilePreview.vue` | 加密容器预览添加文件通用信息卡片 |
