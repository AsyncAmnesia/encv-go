# alist-encrypt 插件 + ENCV v4 容器文件名加密 Spec

## Why

当前 ENCV 系统中，alist-encrypt 插件的文件名加密实现（mixBase64 + CRC6）需要与上游 [alist-encrypt-go](https://github.com/qingwo1991-debug/alist-encrypt-go) 保持完全一致；同时 ENCV v4 容器（Manifest_v4 / EnvelopeHeaderV4）尚无文件名加密能力，需要从整体架构角度设计更先进的方案（如将原始文件名存入 Manifest 元数据、支持可配置的混淆算法等）。两者是**独立并行**的功能。

## What Changes

### A. alist-encrypt 插件文件名加密（与 alist-encrypt-go 对齐）

- **Go 后端** (`internal/alistencrypt/`)：确保 EncryptedName 的 mixBase64 编码 + CRC6 校验 + AES-CTR 加密逻辑与 alist-encrypt-go 一致
- **前端** (`features/alist-encrypt/useAlistEncrypt.ts`)：decodeAlistFilename 函数需同步更新
- **API 层**：确认后端 API 返回的加密文件名字段格式

### B. ENCV v4 容器文件名加密（架构级新功能）

- **Manifest_v4 扩展**：新增 `OriginalName` 字段存储原始文件名
- **Header Flags**：新增 `FlagFilenameEncrypted` 标志位表示文件名已混淆
- **文件名加密接口**：定义 FilenameObfuscator 接口，支持多种策略
- **默认策略**：AES-GCM-SIV 或 XChaCha20-Poly1305（比 mixBase64 更安全）
- **前端集成**：Files.vue 列表显示时自动解密 v4 容器的原始文件名
- **API 层**：新增 `GET /api/v1/file/original-name` 端点

## Impact

- Affected specs: 无已有 spec 直接关联（encv-v4-container-architecture 是容器通用架构，本 spec 是其子集）
- Affected code:
  - `internal/alistencrypt/filename.go` — alist-encrypt 插件文件名加解密
  - `internal/alistencrypt/cipher.go`, `aesctr.go`, `reader.go` — 加密原语
  - `internal/v2/types/header_v4.go` — v4 Header 结构扩展
  - `internal/v2/types/segment_v4.go` — Manifest_v4 新增字段
  - `internal/v2/container/handle/handle.go` — Handle 接口可能扩展
  - `app/encv-mobile/src/features/alist-encrypt/useAlistEncrypt.ts` — 前端解码
  - `app/encv-mobile/src/views/Files.vue` — 文件列表显示

## ADDED Requirements

### Requirement: alist-encrypt 文件名加密对齐

系统 SHALL 提供 alist-encrypt 插件的文件名加解密能力，且与 alist-encrypt-go 参考实现的编码格式、校验方式、加密算法保持二进制兼容。

#### Scenario: 加密文件名生成
- **WHEN** 后端使用密码对原始文件名执行 EncryptName(plainName, password)
- **THEN** 输出符合 EncryptedName 格式：`mixBase64(AES-CTR(plainName, key)) + "_" + hex(CRC6(ciphertext))`
- **AND** 前端 decodeAlistFilename 能正确还原

#### Scenario: 解密文件名还原
- **WHEN** 前端调用 decodeAlistFilename(encryptedName, password) 或后端 DecryptName()
- **THEN** 正确返回原始文件名明文
- **AND** CRC6 校验失败时返回错误而非乱码

### Requirement: ENCV v4 容器文件名混淆

系统 SHALL 在 v4 容器中提供可选的文件名混淆功能，将原始文件名安全地嵌入容器元数据，并在展示时自动还原。

#### Scenario: v4 容器写入时携带原始文件名
- **WHEN** 创建 v4 容器时启用文件名混淆（FlagFilenameEncrypted）
- **THEN** Manifest_v4.OriginalName 存储加密后的原始文件名
- **AND** Header.Flags 包含 FlagFilenameEncrypted 标志
- **AND** 外部可见的文件名为混淆后的名称（如 UUID 或哈希前缀）

#### Scenario: v4 容器读取时还原原始文件名
- **WHEN** 前端请求 v4 容器文件列表或详情
- **THEN** 自动从 Manifest 还原并显示原始文件名
- **AND** 若无 FlagFilenameEncrypted 标志则直接使用物理文件名

#### Scenario: 可插拔的混淆策略
- **WHEN** 配置指定不同的 filename_obfuscation_strategy
- **THEN** 系统支持至少两种策略：
  - `none`（默认）：不混淆，保留原始文件名
  - `aes-gcm`：使用 AES-256-GCM-SIV 加密原始文件名存入 Manifest

## MODIFIED Requirements

### Requirement: Manifest_v4 数据结构

Manifest_v4 结构体 SHALL 新增以下字段：

```go
type Manifest_v4 struct {
    // ... 已有字段 ...
    OriginalName      string              `json:"original_name,omitempty"`       // 加密后的原始文件名
    FilenameAlgorithm string              `json:"filename_alg,omitempty"`        // 使用的混淆算法标识
}
```

### Requirement: Header Flags 位域

EnvelopeHeaderV4.Flags SHALL 新增标志位：

```go
const (
    // ... 已有标志 ...
    FlagFilenameEncrypted uint16 = 0x0010 // 文件名已混淆
)
```

### Requirement: 前端 decodeAlistFilename 函数

`useAlistEncrypt.ts` 中的 decodeAlistFilename SHALL 支持完整的 EncryptedName 格式解析：
- 分离 mixBase64 密文和 CRC6 校验值
- 验证 CRC6
- 使用 password 派生密钥执行 AES-CTR 解密
- 返回 UTF-8 明文文件名

## REMOVED Requirements

无。
