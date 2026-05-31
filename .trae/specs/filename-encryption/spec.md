# alist-encrypt 插件 + ENCV v4 容器文件名加密 Spec

## Why

当前 ENCV 系统需要两个独立的文件名加密能力：
1. **alist-encrypt 插件**：与上游 [alist-encrypt-go](https://github.com/qingwo1991-debug/alist-encrypt-go) 保持一致的 mixBase64 + CRC6 文件名加密
2. **ENCV v4 容器**：基于 Manifest 清单的文件名加密——核心原则是**文件名为展示层抽象**，物理文件名可被任意破坏，原始文件名始终可从容器清单恢复；支持运行时修改原始文件名并实时反映到显示层；需妥善处理超长/超短/特殊字符等边界情况

## What Changes

### A. alist-encrypt 插件文件名加密（与 alist-encrypt-go 对齐）

- **Go 后端** (`internal/alistencrypt/`)：确保 EncryptedName 的 mixBase64 编码 + CRC6 校验 + AES-CTR 加密逻辑与参考实现二进制兼容
- **前端** (`features/alist-encrypt/useAlistEncrypt.ts`)：decodeAlistFilename 函数同步更新

### B. ENCV v4 容器文件名加密（Manifest 驱动的展示层抽象）

核心设计原则：**文件名是展示层元数据，不是存储标识**。

- **Manifest_v4 扩展**：`original_name` 字段存储 UTF-8 原始文件名（明文或密文取决于策略）
- **Header Flags**：`FlagFilenameEncrypted` (0x0010) 标志位表示 original_name 已加密
- **文件名解析优先级**：`Manifest.original_name（解密后） > 物理文件名`
- **原始文件名修改 API**：`PATCH /api/v1/file/rename` 修改 Manifest 中的 original_name 并立即反映到显示层
- **边界处理**：超长文件名（>255 字节截断/分片）、空文件名、Unicode/特殊字符全量支持

## Impact

- Affected code:
  - `internal/alistencrypt/filename.go` — alist-encrypt 插件 EncryptedName 加解密
  - `internal/v2/types/segment_v4.go` — Manifest_v4 新增 original_name / filename_alg 字段
  - `internal/v2/types/header_v4.go` — FlagFilenameEncrypted 标志位
  - `app/encv-mobile/src/features/alist-encrypt/useAlistEncrypt.ts` — 前端解码函数
  - `app/encv-mobile/src/views/Files.vue` — 文件列表显示逻辑（优先使用 Manifest original_name）
  - 后端 API 层 — 新增 rename 端点 + 文件列表返回 original_name 元数据

## ADDED Requirements

### Requirement: alist-encrypt 文件名加密对齐

系统 SHALL 提供 alist-encrypt 插件的文件名加解密能力，且与 alist-encrypt-go 参考实现保持二进制兼容。

#### Scenario: 加密文件名生成
- **WHEN** 后端执行 EncryptName(plainName, password)
- **THEN** 输出格式：`mixBase64(AES-CTR(plaintext, key)) + "_" + hex(CRC6(ciphertext))`

#### Scenario: 解密文件名还原
- **WHEN** 执行 DecryptName(encName, password)
- **THEN** 先校验 CRC6，再 AES-CTR 解密，返回 UTF-8 明文
- **AND** CRC6 失败时返回明确错误而非乱码

### Requirement: ENCV v4 容器文件名作为展示层抽象

系统 SHALL 将 v4 容器的文件名视为**可恢复的展示层元数据**，而非不可变的存储标识。

#### Scenario: 物理文件名被破坏时从 Manifest 恢复
- **WHEN** v4 容器的物理文件名被重命名为任意字符串（如 UUID、乱码、超长名称）
- **AND** Manifest.original_name 存在有效值
- **THEN** 系统（API 返回 / 前端列表）始终显示 Manifest 中的原始文件名
- **AND** 所有文件操作（打开、预览、解密等）均基于 Manifest 的 original_name 进行

#### Scenario: 启用文件名加密时的显示行为
- **WHEN** 创建 v4 容器时设置了 FlagFilenameEncrypted 且指定了密码
- **THEN** Manifest.original_name 存储的是**加密后的**原始文件名
- **AND** 前端/API 层解密后显示明文原始文件名
- **AND** 若解密失败（无密码/密码错误），则显示物理文件名或占位符

#### Scenario: 修改原始文件名
- **WHEN** 用户通过 API 或 UI 修改某个 v4 容器的原始文件名
- **THEN** Manifest.original_name 被更新为新值
- **AND** 若启用了文件名加密，新值先经加密算法处理后存入 Manifest
- **AND** 文件列表立即反映修改后的文件名
- **AND** 物理文件名不受影响（除非用户显式触发重命名）

#### Scenario: 超长文件名处理
- **WHEN** 原始文件名超过 255 字节（常见文件系统限制）
- **THEN** 系统不截断原始文件名，完整存储于 Manifest.original_name（JSON string 无长度限制）
- **AND** 物理文件名自动缩短为安全长度（如取哈希前缀或截断）
- **AND** 展示层始终显示完整的原始文件名

#### Scenario: 超短/空文件名处理
- **WHEN** 原始文件名为空字符串或仅含空白字符
- **THEN** Manifest.original_name 存储该值（不拒绝）
- **AND** 展示层回退显示物理文件名或 "(unnamed)" 占位符

#### Scenario: 任意 Unicode 字符文件名
- **WHEN** 原始文件名包含 emoji、CJK、RTL 文本、控制字符等
- **THEN** Manifest.original_name 以 UTF-8 存储，不做任何字符过滤或转义
- **AND** 加密操作作用于 UTF-8 字节序列，不依赖字符语义
- **AND** 展示层正确渲染 Unicode 字符

## MODIFIED Requirements

### Requirement: Manifest_v4 数据结构

```go
type Manifest_v4 struct {
    // ... 已有字段 ...
    OriginalName      string `json:"original_name,omitempty"`       // 原始文件名（明文或密文）
    FilenameAlgorithm string `json:"filename_alg,omitempty"`        // "none" | "aes-gcm" | ...
}
```

### Requirement: Header Flags 位域

```go
const FlagFilenameEncrypted uint16 = 0x0010 // Manifest.original_name 是加密存储的
```

### Requirement: 文件名解析优先级

后端文件列表 API 和前端 Files.vue 的文件名解析 SHALL 遵循以下优先级：

```
1. 若容器是 v4 格式且 Manifest.original_name 非空：
   a. 若 FlagFilenameEncrypted 未设置 → 直接使用 original_name 作为显示名
   b. 若 FlagFilenameEncrypted 已设置 → 解密 original_name 后作为显示名（失败则 fallback 到步骤 2）
2. 否则 → 使用物理文件系统中的文件名
```

### Requirement: 前端 decodeAlistFilename

SHALL 支持完整 EncryptedName 格式：分离 mixBase64 密文和 CRC6 → 校验 → AES-CTR 解密 → 返回 UTF-8 明文

## REMOVED Requirements

无。
