# Alist-Encrypt 兼容层 Spec

## Why

encv-go 已有完整的 ENCV 容器加密体系（AES-256-CTR），但无法处理由 **alist-encrypt（Node.js/Go）** 加密的文件。这类文件广泛存在于使用 Alist 网盘 + alist-encrypt 加密方案的场景中。需要在 encv-go 中新增对 **alist-encrypt AES-128-CTR** 格式的兼容支持，使移动端（Android）能够解密、加密和流式预览此类文件。

## What Changes

### 架构约束：算法隔离

**核心原则：主应用仅包含 AES-128-CTR，其他算法必须隔离在扩展实现中，不得污染主应用代码。**

```
internal/alistencrypt/           ← 主应用包（仅 AES-128-CTR）
├── cipher.go                    ← Cipher 接口定义（扩展点）
├── aesctr.go                    ← AES-128-CTR 唯一内置实现
├── filename.go                  ← MixBase64 文件名加解密（所有算法共用）
├── content_header.go            ← V2/V1 内容头检测与解析（所有算法共用）
├── flow.go                      ← FlowEnc 调度器（根据 encType 选择 cipher）
└── registry.go                  ← 内置注册表（仅注册 aesctr）

# 扩展实现（未来，不在 MVP 中）
# internal/alistencrypt/ext/
# ├── rc4md5.go                  ← RC4-MD5 扩展（需用户主动引入）
# └── chacha20.go                ← ChaCha20 扩展（需用户主动引入）
```

**具体规则**：
1. `internal/alistencrypt/` 包的 `import` 中不得出现 RC4 或 ChaCha20 相关的实现代码
2. `Cipher` 接口作为扩展点开放，扩展包自行实现并调用 `Register()`
3. Config 中 `enc_type` 非 `aesctr` 时，MVP 阶段返回明确错误「该算法需要扩展支持」
4. MixBase64 / CRC6 / ContentHeader 是所有算法共用的基础设施，放在主包中

### 最小可行范围（MVP）

- **新增 Go 包** `internal/alistencrypt/`：**仅包含 AES-128-CTR** 核心算法 + 共享基础设施（文件名 MixBase64、V2 头检测）
- **Cipher 接口**：定义可扩展的密码器接口，为未来 RC4MD5/ChaCha20 扩展预留
- **扩展 Config**：在 `config.Config` 中新增 `AlistEncrypt` 配置段（密码、后缀名、encType）
- **新增移动端 API**：`/api/alist-encrypt/decrypt`、`/api/alist-encrypt/encrypt`、`/api/alist-encrypt/stream`（流式预览）
- **扩展 TaskManager**：支持 `type=alist-decrypt` / `type=alist-encrypt` 任务类型
- **前端适配**：Files.vue 中识别 alist-encrypt 文件 → 调用新 API 解密/预览

### 不在 MVP 范围内（标记为 TODO）

- **RC4MD5 / ChaCha20 算法实现**：必须通过扩展机制引入，禁止进入主应用包
- **OpenList 代理集成**：将 alist-encrypt 兼容层接入 `internal/openlist/` 代理链
- **内部容器集成**：将 alist-encrypt 作为 ENCV 插件体系的 Plugin 注册到 registry
- **桌面端 UI**：当前仅 Android 移动端

### 关键细节要求

1. **自定义后缀名**：用户可配置 alist-encrypt 加密文件的扩展名（如 `.sccgv`、`.enc`、自定义），用于文件识别
2. **文件名加解密**：完整移植 MixBase64（KSA shuffle alphabet + CRC6 校验）的 EncodeName/DecodeName
3. **V2 内容头兼容**：解析 `AECTR2` magic 头（16 bytes nonce + 8 bytes plainSize），自动区分 V1（裸流）和 V2（带头）格式
4. **Seek 支持**：128-bit CTR counter increment 算法必须与 Node.js aesCTR.js 完全一致
5. **算法隔离**：主应用仅编译链接 AES-128-CTR 实现；enc_type 为 rc4md5/chacha20 时返回「需要扩展支持」错误

## Impact

- Affected specs: 无直接依赖现有 spec
- Affected code:
  - `internal/config/config.go` — 新增 AlistEncrypt 配置段
  - `internal/server/mobile_api.go` — 新增 3 个 API endpoint
  - `internal/service/mobile_service.go` — 新增解密/加密/流式逻辑
  - `internal/service/task_manager.go` — 新增任务类型
  - `app/encv-mobile/src/api/encv.ts` — 新增前端 API 调用
  - `app/encv-mobile/src/views/Files.vue` — 文件识别与操作入口
  - **新增** `internal/alistencrypt/` — 核心算法包

## ADDED Requirements

### Requirement: AES-128-CTR 核心算法

系统 SHALL 提供 AES-128-CTR 流密码实现，与 alist-encrypt-go / Node.js alist-encrypt 完全兼容。**这是主应用内置的唯一算法实现。**

#### Cipher 扩展接口（算法隔离契约）

```go
// Cipher 定义可扩展的密码器接口
// 主应用仅提供 AesCtrCipher 实现
// 其他算法（RC4MD5、ChaCha20）必须由扩展包实现此接口并调用 Register()
type Cipher interface {
    SetPosition(position int64) error
    Encrypt(data []byte)
    Decrypt(data []byte)
    Algorithm() string          // 返回 "AES-128-CTR" / "RC4-MD5" / "ChaCha20" 等
    BlockSize() int
}

// CipherFactory 创建 Cipher 实例的工厂函数
type CipherFactory func(password string, fileSize int64) (Cipher, error)

// Register 向注册表注册新的 cipher 工厂（扩展点）
func Register(encType string, factory CipherFactory)
```

**隔离规则**：
- `internal/alistencrypt/` 包内 **禁止 import 或实现 RC4 / ChaCha20 相关代码**
- `registry.go` 的 `init()` 中 **仅注册 aesctr**
- 非 aesctr 的 enc_type 在 MVP 阶段由 `flow.go` 返回 `ErrExtensionRequired` 错误

#### 密钥派生链（必须逐字节兼容）

```
输入: password (string), fileSize (int64)

Step 1: passwdOutward 派生
  └─ if len(password) == 32 → 直接用（已是 hex）
  └─ else → PBKDF2(password, salt="AES-CTR", iter=1000, dkLen=16, hash=SHA256)
         → hex.EncodeToString(key) → 32字符 hex 字符串

Step 2: 密钥 (16 bytes)
  └─ passwdSalt = passwdOutward + strconv.FormatInt(fileSize, 10)
  └─ key = MD5(passwdSalt)[:16]

Step 3: IV (16 bytes)
  └─ iv = MD5(strconv.FormatInt(fileSize, 10))[:16]
```

#### Seek 支持（视频播放必需）

- SetPosition(position) 必须恢复原始 IV → 按 blockCount 递增 128-bit counter → 重建 CTR stream → discard offset bytes
- incrementIV 使用 4 段 32-bit 大端分段进位算法（与 Node.js aesCTR.js 一致）

#### Scenario: 视频文件 seek 播放
- **WHEN** 用户在播放器中拖动进度条到 position=1048576 (1MB)
- **THEN** SetPosition(1048576) 正确计算新 IV，后续读取的数据在该偏移处正确解密

### Requirement: 文件名 MixBase64 加解密

系统 SHALL 提供 MixBase64 文件名编解码，与 alist-encrypt 的 filename.go 完全兼容。

#### 编码流程
1. passwdOutward ← GetPasswdOutward(password, encType) （PBKDF2 或直接使用）
2. mix64 ← NewMixBase64(passwdOutward) （KSA shuffle 生成 64 字符字母表）
3. encoded ← mix64.EncodeString(plainName)
4. crc6Check ← CRC6(encoded + passwdOutward) 取 sourceChars[crc6Bit]
5. result ← encoded + crc6Check （末尾追加 1 字符校验位）

#### 解码流程
1. 从 encodedName 末尾剥离 CRC6 校验字符
2. CRC6 验证（encoded + passwdOutward）
3. mix64.DecodeString(subEncName) → 原始文件名
4. 验证失败时返回空字符串（非异常）

#### Scenario: 文件名正确往返
- **WHEN** EncodeName("测试视频.mp4", "mypassword", "aesctr") 后 DecodeName 回来
- **THEN** 得到 "测试视频.mp4"（完全一致）

### Requirement: V2 内容头检测与解析

系统 SHALL 自动检测并解析 alist-encrypt V2 格式的内容头（magic `AECTR2`）。

#### V2 头结构（32 bytes）
| Offset | Length | Content |
|--------|--------|---------|
| 0 | 6 | Magic `"AECTR2"` |
| 6 | 1 | Version (`0x02`) |
| 7 | 1 | Reserved |
| 8 | 16 | NonceField |
| 24 | 8 | PlainSize (big-endian uint64) |

#### AutoDetect 行为
1. 读取前 32 bytes
2. 如果 magic 匹配 → V2 模式（使用 NonceField 初始化 cipher）
3. 如果 magic 不匹配 → V1/Legacy 模式（裸流，fileSize 即 plainSize）

#### Scenario: 混合格式处理
- **WHEN** 打开一个 V2 格式的加密文件（带 AECTR2 头）
- **THEN** 自动跳过 32 字节头，使用头内 NonceField + PlainSize 进行解密

### Requirement: 流式解密预览（HTTP Range 支持）

系统 SHALL 提供 HTTP 流式端点，支持 Range 请求，使 MPV 等播放器可以直接播放解密后的流。

- **Endpoint**: `GET /api/alist-encrypt/stream?path=<path>&password=<pwd>`
- **行为**:
  1. 打开加密文件，检测 V1/V2 格式
  2. 根据 Range header 计算上游偏移（V2 时 +32 bytes header）
  3. SetPosition(rangeStart) → 读取 rangeLength 字节 → XORKeyStream 解密
  4. 返回 `Content-Type` 基于原始文件扩展名推断
  5. 支持 `Content-Length: plainSize`

#### Scenario: MPV 通过 URL 播放加密视频
- **WHEN** MPV 请求 `GET /api/alist-encrypt/stream?path=/sdcard/video.sccgv&Range: bytes=0-`
- **THEN** 返回 206 Partial Content，body 为解密后的视频数据

### Requirement: 可配置后缀名

用户 SHALL 能够通过配置指定 alist-encrypt 加密文件的扩展名，用于文件识别。

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

- `suffix`: 默认 `.sccgv`，可为空（表示不依赖扩展名识别）
- `enc_type`: MVP 阶段 **仅接受 `"aesctr"`**；传入其他值（`"rc4md5"` / `"chacha20"`）时：
  - 配置加载阶段：**警告日志**「enc_type 'xxx' 需要扩展支持，MVP 阶段将回退到 aesctr 或拒绝操作」
  - 运行时调用：返回错误 `ErrExtensionRequired{EncType: "rc4md5", Message: "该算法需要扩展包支持，当前仅内置 AES-128-CTR"}`

### Requirement: 移动端解密/加密任务

系统 SHALL 在现有 TaskManager 框架下支持 alist-encrypt 类型的解密和加密任务。

- **任务类型**: `alist-decrypt`、`alist-encrypt`
- **状态流转**: queued → running → completed / failed（复用现有状态机）
- **进度报告**: 基于已处理字节数 / 总字节数
- **WebSocket 推送**: 复用现有 WSBroadcaster 推送进度更新

#### Scenario: 移动端发起解密任务
- **WHEN** 前端 POST `/api/tasks` body=`{type:"alist-decrypt", sourcePath:"/sdcard/video.sccgv", password:"xxx"}`
- **THEN** 创建任务 → 异步执行解密 → WebSocket 推送进度 → 完成后输出到目标目录

## MODIFIED Requirements

无（纯新增功能，不修改现有 ENCV 容器逻辑）。

## REMOVED Requirements

无。
