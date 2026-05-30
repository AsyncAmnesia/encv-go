# Alist-Encrypt 兼容层 Spec（ENCV Plugin 架构）

## Why

encv-go 已有完善的 **ENCV Plugin 体系**（`internal/v2/plugins/`），包含 video/image/audio/text/pdf/wps 等插件，每个插件实现统一的 `Plugin` 接口并通过 `registry.go` 统一管理。现在需要新增一个 **alist-encrypt 插件**，使系统能够：
- 解密 alist-encrypt 格式（AES-128-CTR）的加密文件
- 加密文件为 alist-encrypt 格式
- 流式预览加密视频（支持 seek）
- 在线解码显示真实文件名（MixBase64）

**架构决策：三层分离**
1. **算法层** (`internal/alistencrypt/`) — 业务无关的基础设施（AES-128-CTR + MixBase64 + V2头检测），纯 Go 包
2. **插件层** (`internal/v2/plugins/alistencrypt/`) — 实现 ENCV `Plugin` 接口，编排加解密流程，注册到 Plugins 列表
3. **UI 层**（已有通道）— 通过 `mobile_api.go` + 前端复用现有任务/预览机制，无需 ComboLite 插件

---

## What Changes

### 新增模块结构

```
internal/alistencrypt/                          ← Layer 1: 算法基础设施包（业务无关）
├── cipher.go                                   ← Cipher 接口 + CipherFactory + Register/Create
├── aesctr.go                                   ← AES-128-CTR 唯一内置实现
├── registry.go                                 ← Cipher 注册表（init 仅注册 aesctr）
├── filename.go                                 ← MixBase64 文件名加解密（KSA+CRC6）
├── content_header.go                           ← V2 内容头检测与解析（AECTR2 magic）
├── reader.go                                   ← DecryptReader io.Reader 包装器（seek 支持）
└── errors.go                                   ← ErrExtensionRequired 等

internal/v2/plugins/alistencrypt/                ← Layer 2: ENCV Plugin 实现（业务编排）
├── plugin.go                                   ← AlistEncryptPlugin struct，实现 Plugin 接口
├── types.go                                    ← AlistEncryptPluginConfig 配置结构体
├── encryptor.go                                ← 加密流程编排（原始文件 → AES-CTR → 输出+后缀）
├── decryptor.go                                ← 解密流程编排（加密文件 → DetectV2 → AES-CTR → 输出）
└── streamer.go                                 ← 流式预览（DecryptReader → HTTP Range 响应）
```

### 已有代码修改清单

| 文件 | 改动 |
|------|------|
| `internal/v2/plugins/registry.go` | Plugins 列表中注册 `&AlistEncryptPlugin{}` |
| `internal/server/mobile_api.go` | 新增 `/api/alist-encrypt/stream` 和 `/api/alist-encode/decode-filename`（如果现有 task API 不能覆盖） |
| `internal/service/mobile_service.go` | 适配新插件类型的任务处理 |
| `internal/config/config.go` | alist_encrypt 插件配置会通过 Plugin.GetDefaultSettings() 自动纳入，可能无需额外改动 |
| `app/encv-mobile/src/views/Files.vue` | 识别 .bin（或自定义 suffix）文件 → 显示解密/预览入口 |
| `app/encv-mobile/src/api/encv.ts` | 新增 streamAlistFile / decodeAlistFilename 调用 |
| `app/encv-mobile/src/views/Tasks.vue` | alist-encrypt 类型的任务自动被现有任务系统展示（无需特殊处理） |

### 不在 MVP 范围内（TODO）

- **OpenList 代理集成**：接入 internal/openlist/ 代理链
- **桌面端 UI**：openlist 桌面客户端适配
- **RC4MD5 / ChaCha20 扩展**：必须通过 Cipher 接口 + Register() 引入，禁止进入主包

---

## ADDED Requirements

### Requirement: 算法隔离架构（铁律）

`internal/alistencrypt/` 包 **仅包含 AES-128-CTR 实现**，其他算法必须通过 Cipher 接口隔离。

```go
type Cipher interface {
    SetPosition(position int64) error
    Encrypt(data []byte)
    Decrypt(data []byte)
    Algorithm() string
    BlockSize() int
}

type CipherFactory func(password string, fileSize int64) (Cipher, error)

func Register(encType string, factory CipherFactory)
func Create(password string, encType string, fileSize int64) (Cipher, error)
```

**隔离规则**：
1. `internal/alistencrypt/` 内 **禁止 RC4 / ChaCha20 实现代码**
2. `registry.go init()` **仅注册 aesctr**
3. 非 aesctr enc_type → `ErrExtensionRequired`

### Requirement: AES-128-CTR 核心算法（Go 实现）

#### 密钥派生链（逐字节兼容 alist-encrypt-go）

```
输入: password (string), fileSize (int64)

Step 1: passwdOutward
  ├─ len == 32 → 直接用（已是 hex）
  └─ else → PBKDF2(pwd="AES-CTR", salt=password, iter=1000, dkLen=16, HmacSHA256)
         → hex.EncodeToString(key) → 32字符 hex 字符串

Step 2: Key (16 bytes) = MD5(passwdOutward + strconv.FormatInt(fileSize, 10))[:16]
Step 3: IV  (16 bytes) = MD5(strconv.FormatInt(fileSize, 10))[:16]

输出: AES-128-CTR(Key, IV)
```

#### Seek 支持

```
SetPosition(position):
  1. iv = copy(originalIv)
  2. blockCount = position / 16
  3. incrementIV(blockCount)     // 128-bit 大端分段进位（4×uint32）
  4. stream = cipher.NewCTR(block, iv)
  5. discard offset = position % 16 bytes
```

### Requirement: 文件名 MixBase64 加解密（Go 实现）

完整移植 `filename.go`：KSA shuffle → MixBase64 Encode/Decode → CRC6 校验 → EncodeName/DecodeName。

### Requirement: V2 内容头自动检测（Go 实现）

| Offset | Len | Content |
|--------|-----|---------|
| 0 | 6 | Magic `"AECTR2"` |
| 6 | 1 | Version (`0x02`) |
| 7 | 1 | Reserved |
| 8 | 16 | NonceField |
| 24 | 8 | PlainSize (BE uint64) |

AutoDetectV2: peek 前 32 bytes → magic 匹配则 V2，否则 V1/Legacy。

### Requirement: ENCV Plugin 接口实现

`AlistEncryptPlugin` 必须实现 `Plugin` 接口的关键方法：

```go
type AlistEncryptPlugin struct {
    ctx      context.Context
    cfg      *config.Config
    settings AlistEncryptPluginConfig
}

func (p *AlistEncryptPlugin) Name() string                          { return "alist_encrypt" }
func (p *AlistEncryptPlugin) GetContainerExtension() string         { return p.settings.Suffix }  // 默认 ".bin"
func (p *AlistEncryptPlugin) GetDefaultSettings() json.RawMessage   { /* 默认配置 */ }
func (p *AlistEncryptPlugin) GetSettingsSchemaType() interface{}    { return AlistEncryptPluginConfig{} }
func (p *AlistEncryptPlugin) Initialize(ctx context.Context) error  { /* 读取配置 */ }
func (p *AlistEncryptPlugin) SupportedMimePrefixes() []string       { return nil }  // 不按 MIME 匹配
func (p *AlistEncryptPlugin) SupportedExtensions() []string         { return nil }  // 不按扩展名匹配
func (p *AlistEncryptPlugin) ShouldProcess(inputPath string) bool   { /* 检查是否需要加密 */ }
func (p *AlistEncryptPlugin) CanDecrypt(containerPath string) bool  { /* 检测 AECTR2 magic 或扩展名匹配 */ }
func (p *AlistEncryptPlugin) Encrypt(dataReader io.Reader) (*crypto.EncryptionResult, error)  { /* 加密流程 */ }
func (p *AlistEncryptPlugin) Decrypt(containerPath, outputDir string) error               { /* 解密流程 */ }
// ... 其他方法提供默认实现
```

#### 关键设计决策

| 方法 | 实现策略 | 说明 |
|------|---------|------|
| `GetContainerExtension()` | 返回 `settings.Suffix`（默认 `.bin`） | 用户可自定义 |
| `CanDecrypt()` | 检测扩展名匹配 + 可选 AECTR2 magic 检测 | 与 ENCV 容器区分开 |
| `Encrypt()` | 读取原始数据 → AesCtrCipher 加密 → 写入目标路径+suffix（可选 V2 头） | **不使用 ENCV 容器包装** |
| `Decrypt()` | 打开文件 → AutoDetectV2 → AesCtrCipher 解密 → 写入目标路径 | **不使用 ENCV 容器解包** |
| `SupportedMimePrefixes()` | 返回 `nil` | 不参与 MIME 自动匹配（避免误拦截） |
| `SupportedExtensions()` | 返回 `nil` | 同上 |
| `ShouldProcess()` | 基于 explicit user action | 仅用户主动选择时触发 |

### Requirement: 后缀安全校验（冲突 + 无效双重校验）

#### 默认值

```go
type AlistEncryptPluginConfig struct {
    Suffix         string `json:"suffix"`          // 默认 ".bin"（**禁止 .sccgv/.encv**）
    DefaultPassword string `json:"default_password"` // 默认 ""
    EncType        string `json:"enc_type"`        // 默认 "aesctr"
}
```

#### 第一层：配置加载时校验

```
Initialize() 中:
  1. suffix 冲突检查：黑名单 [".sccgv", ".encv"]
     ├─ 命中 → slog.Error + 回退到 ".bin" + 禁用功能
  2. suffix 格式检查：以 "." 开头且长度 ≤ 16
     ├─ 不合法 → slog.Warn + 回退到 ".bin"
  3. enc_type 检查：MVP 仅允许 "aesctr"
     ├─ 其他值 → slog.Warn + ErrExtensionRequired
```

#### 第二层：运行时操作前校验

```
Decrypt() / Encrypt() 操作前:
  1. 类型校验：目标文件扩展名 == config.suffix？
     ├─ 不匹配 → 返回错误「非 alist-encrypt 加密文件」
  2. 容器碰撞校验（可选）：peek 文件头
     ├─ 检测到 ENCV 容器 magic → 返回错误「检测到 ENCV 容器格式，请使用主应用解密」
     ├─ AECTR2 magic → V2 格式 ✓
     └─ 无 magic → V1/Legacy（继续，记录日志）
```

### Requirement: 流式预览

通过 `mobile_api.go` 提供 HTTP endpoint：

```
GET /api/alist-encrypt/stream?path=<uri>&password=<pwd>
  → AutoDetectV2(path) → DecryptReader(支持 SetPosition)
  → HTTP Range 支持 → 206 Partial Content → Content-Type 推断
```

MPV/ArtPlayer 直接请求此 URL 即可播放加密视频。

### Requirement: 文件名解码 API

```
GET /api/alist-encrypt/decode-filename?encoded=<name>&password=<pwd>
  → DecodeName(encoded, password) → plainName JSON
```

Files.vue 在列表中调用此 API 显示真实文件名。

### Requirement: 注册与发现

在 `registry.go` 中注册：

```go
var Plugins = []Plugin{
    &video.VideoPlugin{},
    &audio.AudioPlugin{},
    &image.ImagePlugin{},
    &wps.WPSPlugin{},
    &pdf.PDFPlugin{},
    &text.TextPlugin{},
    &alistencrypt.AlistEncryptPlugin{},  // ← 新增
}
```

**发现机制**：
- **加密**：不通过 `FindEncryptingPlugin()` 自动发现（`SupportedMimePrefixes()`/`SupportedExtensions()` 返回 nil），由前端显式指定 `"use_plugin": "alist_encrypt"` 触发
- **解密**：通过 `FindDecryptingPlugin()` 发现——`CanDecrypt()` 检测扩展名 + AECTR2 magic

---

## MODIFIED Requirements

无（纯新增功能）。

## REMOVED Requirements

无。
