# 多问题修复计划

## 问题清单

| # | 问题 | 严重程度 | 根因 |
|---|------|----------|------|
| 1 | video.go 缺少 `!android` build tag | 🔴 CI 编译失败 | 无平台约束，引用 Android 特有函数 |
| 2 | 加密/解密任务 UI 不统一 | 🟡 功能缺失 | 解密用 alertController，加密用 ion-modal |
| 3 | v4 容器图片信息乱码（版本/ContainerID/Manifest） | 🔴 数据错误 | ContainerID 来源是 SpecialID（所有插件都传 nil→空）；KVI 原始 JSON 直接暴露给前端 |
| 4 | config.user.json 初始化内容丢失 | 🔴 数据丢失 | PUT /api/config 全量替换；前端 Settings 页 schema-driven 编辑可能丢弃未知字段 |

---

## 问题 1: video.go 缺少 android build tag

### 根因
[video.go](file:///workspace/internal/utils/video.go) 无 build tag 约束，直接引用 Android 特有类型和函数：`NativeError`, `callFFprobeNative()`, `callFFmpegNative()` 等。这些定义在 `ffmpeg_dlopen.go` (android) / `ffmpeg_dlopen_stub.go` (!android) 中。

### 修复
在 `/workspace/internal/utils/video.go` 顶部添加 `//go:build !android`

---

## 问题 2: 加密/解密任务统一为模态框

### 当前状态
- **加密** (`handleEncryptFile`): ✅ 已用 `ion-modal` + `ContainerVersionSelector`
- **解密** (`handleDecryptFile`): ❌ 使用 `alertController.create()` 弹出表单

### 修复方案

**修改文件**: `/workspace/app/encv-mobile/src/views/Files.vue`

1. 新增解密模态框状态变量:
   - `showDecryptModal`, `decryptSourcePath`, `decryptTargetPath`, `decryptPassword`
   
2. 新增 `<ion-modal>` 解密模板（与加密 modal 并列），包含:
   - 目标路径输入 (ion-input)
   - 密码输入 (ion-input type=password)
   - 提交按钮

3. 修改 `handleDecryptFile()`:
   - 从 alertController 改为设置状态变量并打开 `showDecryptModal = true`
   - 预填充目标路径和密码（从 fetchConfig 获取全局密码）

4. 新增 `handleDecryptSubmit()`:
   - 调用 `doCreateTask('decrypt', ...)`
   - 包含覆盖确认逻辑（与 `handleEncryptSubmit` 一致）

---

## 问题 3: v4 容器图片信息乱码

### 根因分析（已确认）

**ContainerID 为空的链路**:

1. Image plugin [PostEncryptProcessor](file:///workspace/internal/v2/plugins/image/plugin.go#L293-L294) 设置 `SpecialID: nil`
2. [writeManifestV4](file:///workspace/internal/v2/writer/single_file_container_writer.go#L172) 中:
   ```go
   ContainerID: string(w.v4Header.SpecialID[:w.v4Header.IDLength])
   // SpecialID=nil → IDLength=0 → ContainerID=""
   ```
3. Video plugin 同样 `SpecialID: nil`（[plugin.go:582](file:///workspace/internal/v2/plugins/video/plugin.go#L582)）

**KVI 字段暴露原始内容**:

- [GetFileInfo](file:///workspace/internal/service/mobile_service.go#L342) 直接将整个 `mf` (Manifest_v4) 作为 `manifest` 返回给前端
- `Manifest_v4.KVI` 是 `json.RawMessage` 类型，包含 base64 编码的 salt/iv 等
- 前端 [FileInfo.vue](file:///workspace/app/encv-mobile/src/views/FileInfo.vue#L108) 用 `JSON.stringify(data.container.manifest, null, 2)` 直接序列化显示
- 如果 XOR deobfuscation 失败 → legacy fallback → 数据不完整或格式异常

**"乱码"的真正含义**:
- ContainerID 显示空字符串或 `-`
- Manifest JSON 中的 KVI 字段显示为一长串 base64 字符串（看起来像乱码）
- 版本号应该正确（来自 Header，不依赖 Manifest）

### 修复方案

**Step 1: 后端 GetFileInfo 增强** ([mobile_service.go](file:///workspace/internal/service/mobile_service.go#L329-L346))

```go
// 在返回 container 数据前清理敏感/无用字段
if h.Version() == 4 && h.ManifestV4() != nil {
    mf := h.ManifestV4()
    // 清理 KVI（含敏感信息且很长）
    cleanManifest := make(map[string]interface{})
    data, _ := json.Marshal(mf)
    json.Unmarshal(data, &cleanManifest)
    delete(cleanManifest, "kvi") // 不向前端暴露 KVI
    
    result.Container["container_id"] = mf.ContainerID
    if mf.ContainerID == "" {
        result.Container["container_id"] = "(auto-generated)" // 或从 Header 生成
    }
    result.Container["manifest"] = cleanManifest
}
```

**Step 2: 前端 FileInfo.vue 容错**

- ContainerID 为空时显示 `(none)` 而非空白
- 添加 manifest 解析错误捕获
- 对 manifest 显示区域添加最大高度限制和折叠支持（已有）

**Step 3: (可选改进) ContainerID 自动生成**

如果希望 ContainerID 有意义，可以在 `writeManifestV4` 中当 SpecialID 为 nil 时自动生成一个 UUID 作为 ContainerID：

```go
if w.v4Header.IDLength == 0 {
    mf.ContainerID = generateShortUUID() // 如 "img_a1b2c3d4"
}
```

---

## 问题 4: config.user.json 初始化内容丢失

### 根因分析（已确认）

**PUT /api/config 全量替换** ([server_config_api.go:36-122](file:///workspace/internal/server/server_config_api.go#L36-L122)):

```go
body, _ := c.GetRawData()
var raw map[string]interface{}
json.Unmarshal(body, &raw)
indented, _ := json.MarshalIndent(raw, "", "  ")
os.WriteFile(s.configPath, indented, 0644) // ← 全量写入！
```

**问题场景**：
1. 用户首次安装 app → 后端初始化生成默认 config.user.json（完整）
2. 用户打开 Settings 页面 → `fetchConfig()` 获取完整配置
3. 用户修改某个字段（如密码）→ `saveConfig()` → `updateConfig()`
4. **关键**：`useConfig` composable 的 `saveConfig` 是否发送了**完整配置**还是**仅修改的字段**？
5. 如果只发送部分字段 → PUT API 全量替换 → **其他字段丢失**

**另一个可能**：移动端首次启动时的 `ensureConfigExists()` 逻辑是否用默认值覆盖了已有文件？

### 排查/修复方向

**Step 1: 检查 useConfig composable 的 saveConfig 实现**
- 文件: `/workspace/app/encv-mobile/src/composables/useConfig.ts`
- 确认 `saveConfig` 发送的是完整 Config 对象还是仅 dirty fields

**Step 2: 检查移动端初始化逻辑**
- 搜索 `ensureConfigExists` / `initConfig` / `copyDefaultConfig`
- 确认不会在已有配置文件存在时覆盖

**Step 3: (防御性修复) PUT API 增加 merge 逻辑**
- 修改 `handlePutConfigGin`: 先读取现有配置 → deep merge → 写入
- 或者在 API 层不做改变，而是确保前端始终发送完整配置

**Step 4: DefaultConfig() 补全缺失字段**
- 确保 `proxy`, `admin`, `mobile` 等字段有合理的默认值
- 这样即使发生替换，至少不会丢失结构

---

## 实施顺序

### Phase 1: 编译修复（立即阻断）⚡
1. [ ] **Fix 1 - video.go**: 添加 `//go:build !android`

### Phase 2: 功能修复
2. [ ] **Fix 2 - 统一加解密模态框**: Files.vue 解密改为 ion-modal
3. [ ] **Fix 3a - 后端 GetFileInfo 增强**: 清理 KVI、容错 ContainerID
4. [ ] **Fix 3b - 前端 FileInfo 容错**: 空值处理、错误捕获
5. [ ] **Fix 4a - 排查 config 丢失**: 检查 useConfig saveConfig + 移动端初始化
6. [ ] **Fix 4b - 防御性修复**: 确保 saveConfig 发送完整配置 / PUT API merge

### Phase 3: 验证
7. [ ] `CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build ./internal/...`
8. [ ] `go build ./internal/...` (桌面端)
9. [ ] `cd app/encv-mobile && npm run build`
10. [ ] `go test ./internal/v2/...` (回归测试)

## 任务依赖关系
- Fix 1 可独立执行
- Fix 2 可独立执行
- Fix 3a/3b 可并行
- Fix 4a 必须在 Fix 4b 之前（先排查再修复）
- 所有 Fix 完成后统一验证
