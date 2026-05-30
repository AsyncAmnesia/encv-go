# Alist-Encrypt 兼容层 Spec（Go 基础设施 + ComboLite 业务扩展）

## Why

encv-go 移动端已有 **MPV 播放器** 作为 ComboLite 插件。现在需要新增 **alist-encrypt 解密能力**，使移动端能解密/加密/流式预览 alist-encrypt 格式（AES-128-CTR）文件。

**关键架构决策：算法与业务分离**
- **AES-128-CTR 核心算法 + MixBase64 文件名加解密** → **Go 后端公共包**（`internal/alistencrypt/`），业务无关的基础设施
- **具体业务逻辑**（UI 交互、任务调度、进度展示、播放器集成）→ **ComboLite 插件**（`plugin-alist-decrypt`），通过 Go 后端 API 调用算法

这样做的好处：
1. **单份实现**：Go 后端 + 桌面端 + 移动端共享同一套算法，消除双语言维护风险
2. **测试效率**：Go 单元测试比 Android instrumentation test 快一个数量级
3. **项目惯例**：与 `pkg/encv/`（ENCV 加密公共库）和 `internal/v2/crypto/`（容器加密）的分层模式一致
4. **未来扩展**：OpenList 代理集成、桌面端 UI 可直接复用 Go 包

---

## What Changes — 两层架构

### Layer 1: Go 后端 — 算法基础设施包（新增）

```
internal/alistencrypt/                  ← 新增：业务无关的纯算法包
├── cipher.go                           ← Cipher 接口定义（扩展点）
├── aesctr.go                           ← AES-128-CTR 唯一内置实现（密钥派生+seek+加解密）
├── registry.go                         ← Cipher 注册表（init 仅注册 aesctr）
├── filename.go                         ← MixBase64 文件名加解密（KSA+CRC6）
├── content_header.go                   ← V2 内容头检测与解析（AECTR2 magic）
├── reader.go                           ← DecryptReader io.Reader 包装器（seek 支持）
└── errors.go                           ← ErrExtensionRequired 等
```

**消费方式**：
- **移动端**：通过 `mobile_api.go` 新增 HTTP endpoint → `mobile_service.go` 调用算法包
- **桌面端**：直接 import `internal/alistencrypt` 使用
- **未来 OpenList**：`internal/openlist/handler/` 中引入此包做代理解密

### Layer 2: ComboLite 插件 — 业务编排层（新增）

```
app/encv-mobile/plugin-alist-decrypt/   ← 新增：Android Library 模块
├── build.gradle.kts                    ← id("com.android.library") + aar2apk
├── src/main/
│   ├── AndroidManifest.xml             ← AlistDecryptActivity (exported=false)
│   └── java/com/encvgo/plugin/alistdecrypt/
│       ├── AlistDecryptPluginEntry.kt  ← IPluginEntryClass 入口
│       └── ui/
│           └── AlistDecryptActivity.kt ← Activity（ProxyManager 代理启动）
                                          （UI 层：进度展示、密码输入、操作结果反馈）
```

**插件职责（仅业务，不含算法）**：
1. 接收宿主 Intent extras（action/filePath/password/suffix）
2. 通过 Capacitor bridge 调用 Go 后端 API 执行实际加解密
3. 展示进度条和状态
4. 流式预览时启动 LocalStreamServer 代理 Go 后端的 stream endpoint
5. setResult 返回给宿主

### 已有代码修改清单

| 文件 | 改动 |
|------|------|
| `internal/server/mobile_api.go` | 新增 4 个 alist-encrypt API endpoint |
| `internal/service/mobile_service.go` | 新增解密/加密/流式预览/文件名解码服务方法 |
| `internal/config/config.go` | 新增 AlistEncrypt 配置段 |
| `ExtensionsPage.vue` | 新增 alist-decrypt 扩展卡片 |
| `GoProcessPlugin.kt` | 新增 4 个 @PluginMethod（委托给插件或直接调 API） |
| `PlayerEntry.kt` | 新增 buildAlistDecryptIntent() 路由 |
| `encv.ts`（前端 API） | 新增对应的 Capacitor 调用函数 |
| `Files.vue` | 识别 suffix 匹配文件 → 解密/预览入口 |
| `Tasks.vue` | 展示 alist 任务状态 |

### 不在 MVP 范围内（TODO）

- **OpenList 代理集成**：接入 internal/openlist/ 代理链
- **内部容器集成**：注册到 ENCV v2 plugins.Registry
- **桌面端 UI**：openlist 桌面客户端适配
- **RC4MD5 / ChaCha20 扩展包**：必须通过 Cipher 接口 + Register() 引入，禁止进入主包

---

## ADDED Requirements

### Requirement: Go 后端算法隔离架构（铁律）

`internal/alistencrypt/` 包 **仅包含 AES-128-CTR 实现**，其他算法必须通过 Cipher 接口隔离。

```go
// Cipher 接口 — 所有密码器的统一抽象
type Cipher interface {
    SetPosition(position int64) error
    Encrypt(data []byte)
    Decrypt(data []byte)
    Algorithm() string          // "AES-128-CTR" / "RC4-MD5" / "ChaCha20"
    BlockSize() int
}

// CipherFactory 创建 Cipher 实例
type CipherFactory func(password string, fileSize int64) (Cipher, error)

// Register 向注册表注册新的 cipher 工厂（扩展点）
func Register(encType string, factory CipherFactory)

// Create 根据 encType 创建 Cipher 实例
func Create(password string, encType string, fileSize int64) (Cipher, error)
```

**隔离规则**：
1. `internal/alistencrypt/` 包内 **禁止出现 RC4 / ChaCha20 实现代码**
2. `registry.go` 的 `init()` 中 **仅注册 aesctr**
3. 非 aesctr 的 enc_type 返回 `ErrExtensionRequired`
4. MixBase64 / CRC6 / ContentHeader / DecryptReader 是所有算法共用的基础设施，放在主包中

### Requirement: AES-128-CTR 核心算法（Go 实现，唯一内置）

#### 密钥派生链（必须逐字节兼容 alist-encrypt-go）

```
输入: password (string), fileSize (int64)

Step 1: passwdOutward
  ├─ len == 32 → 直接用（已是 hex）
  └─ else → PBKDF2(pwd="AES-CTR", salt=password, iter=1000, dkLen=16, HmacSHA256)
         → hex.EncodeToString(key) → 32字符 hex 字符串

Step 2: Key (16 bytes) = MD5(passwdOutward + strconv.FormatInt(fileSize, 10))[:16]
Step 3: IV  (16 bytes) = MD5(strconv.FormatInt(fileSize, 10))[:16]

输出: AES-128-CTR(Key, IV) via crypto/cipher NewCTR
```

#### Seek 支持（视频流必需）

```
SetPosition(position):
  1. iv = copy(originalIv)
  2. blockCount = position / 16
  3. incrementIV(blockCount)     // 128-bit 大端分段进位（4×uint32）
  4. stream = cipher.NewCTR(block, iv)
  5. stream.XORKeyStream(discard[:offset], discard[:offset])  // offset = position % 16
```

#### Scenario: 视频文件 seek
- **WHEN** SetPosition(1048576) 后读取数据
- **THEN** 数据在该偏移处正确解密（与 Node.js aesCTR.js 输出逐字节一致）

### Requirement: 文件名 MixBase64 加解密（Go 实现）

完整移植 alist-encrypt-go 的 `filename.go`：

- **KSA shuffle**: passwdOutward → 64字符自定义字母表
- **MixBase64 Encode/Decode**: 基于自定义 alphabet 的 Base64 变种
- **CRC6 校验**: 多项式 x^6+x+1, 反射输入输出, 6-bit 结果映射到 sourceChars
- **EncodeName**: plainName → KSA→Encode→CRC6→encoded+crcChar
- **DecodeName**: encodedName → stripCRC6→Verify→Decode→plainName（验证失败返回空串）

### Requirement: V2 内容头自动检测（Go 实现）

| Offset | Len | Content |
|--------|-----|---------|
| 0 | 6 | Magic `"AECTR2"` |
| 6 | 1 | Version (`0x02`) |
| 7 | 1 | Reserved |
| 8 | 16 | NonceField |
| 24 | 8 | PlainSize (BE uint64) |

- AutoDetectV2: peek 前 32 bytes → magic 匹配则 V2（跳过头+用 NonceField），否则 V1（裸流）

### Requirement: 移动端 API（Go 后端提供）

通过 `mobile_api.go` 新增以下 endpoint，供 ComboLite 插件和前端直接调用：

| Method | Path | 用途 |
|--------|------|------|
| POST | `/api/alist-encrypt/decrypt` | 发起异步解密任务（sourcePath+password → targetDir） |
| POST | `/api/alist-encrypt/encrypt` | 发起异步加密任务（sourcePath+password → targetDir+suffix） |
| GET | `/api/alist-encrypt/stream` | 流式解密预览（HTTP Range 支持，返回解密数据） |
| GET | `/api/alist-encrypt/decode-filename` | 同步解码文件名（encodedName+password → plainName） |

#### Scenario: 完整解密流程
- **WHEN** 前端 POST `/api/alist-encrypt/decrypt` body=`{sourcePath:"/sdcard/video.sccgv", password:"xxx"}`
- **THEN** Go 后端创建 TaskManager 任务 → 异步执行 AES-CTR 解密 → WebSocket 推送进度 → 完成后输出到目标目录

#### Scenario: MPV 流式播放
- **WHEN** MPV 请求 `GET /api/alist-encrypt/stream?path=/sdcard/video.sccgv&Range: bytes=0-`
- **THEN** 返回 206 Partial Content，body 为解密后的视频流

### Requirement: 配置管理

```jsonc
{
  "alist_encrypt": {
    "enabled": true,
    "suffix": ".sccgv",
    "default_password": "",
    "enc_type": "aesctr"
  }
}
```

- `enc_type` MVP 仅支持 `"aesctr"`；其他值返回 `ErrExtensionRequired`
- `suffix`: 用户可自定义加密文件识别后缀

### Requirement: ComboLite 插件 — 业务编排层（纯 UI + 调度）

插件 **不包含任何算法代码**，职责为：

1. **AlistDecryptActivity**:
   - 从 Intent extras 读取 action（decrypt/encrypt/stream/decode-filename）+ filePath + password
   - decrypt/encrypt action: 调用 Go 后端 API 启动任务 → 订阅 WebSocket 进度 → 显示进度 UI
   - stream action: 将 Go 后端 stream URL 透传给 MPV（或通过 LocalStreamServer 二次代理）
   - decode-filename action: 同步调用 Go 后端 API → setResult 返回
   - 遵循 EncvHostActivity 四层超时防御规范

2. **AlistDecryptPluginEntry**: IPluginEntryClass 入口（onLoad/onUnload）

### Requirement: 宿主集成路径

```
Files.vue (长按→解密/预览)
  → encv.ts: decryptAlistFile({path, password})
    → GoProcessPlugin.decryptAlistFile(call)
      ├─ 方案A（推荐）：直接调用 Go 后端 POST /api/alist-encrypt/decrypt
      │   → 复用现有 mobile_api 通道，无需启动插件 Activity
      │   → TaskManager 管理 → WebSocket 推进 → Tasks.vue 展示
      └─ 方案B（流式预览）：启动 plugin-alist-decrypt Activity
          → Activity 内部调用 Go 后端 stream API → 返回 URL 给 MPV
```

> **注意**：解密/加密任务可以直接走 Go 后端 API（不需要经过插件），因为它们是纯后端计算。
> **流式预览**可能需要插件参与（如果需要 LocalStreamServer 做 URL 转换），也可能直接走 Go 后端 stream endpoint。

### Requirement: 扩展管理页集成

ExtensionsPage.vue:
```typescript
{
  id: 'alist-decrypt',
  name: 'Alist-Encrypt 解密',
  description: '支持 AES-128-CTR 加密文件的解密、加密和流式预览',
  installed: boolean,
  enabled: boolean,
  sizeDisplay: '~150 KB', // 纯 Kotlin UI 层，无 native 库
}
```
COMBO_LITE_ID: `'alist-decrypt' → 'com.encvgo.plugin.alistdecrypt'`

---

## MODIFIED Requirements

无（纯新增功能）。

## REMOVED Requirements

无。
