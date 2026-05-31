# alist-encrypt 插件 + ENCV v4 容器文件名加密 Spec

## Why

当前 ENCV 系统需要两个独立的文件名加密能力：
1. **alist-encrypt 插件**：与上游 [alist-encrypt-go](https://github.com/qingwo1991-debug/alist-encrypt-go) 保持一致的 mixBase64 + CRC6 文件名加密
2. **ENCV v4 容器**：基于 Manifest 清单的**深度定制的文件名混淆编码方案**——核心原则是**文件名为展示层抽象**，物理文件名可被任意破坏，原始文件名始终可从容器清单恢复；支持运行时修改原始文件名并实时反映到显示层；需妥善处理超长/超短/特殊字符等边界情况

> **关键设计决策**：v4 容器的文件名混淆**不能直接套用通用加密算法**（AES-GCM 等），而必须是一套**深度定制的确定性编码方案**，具备以下独特属性：
> - **人类可读性**：输出是结构化的编码串而非纯乱码
> - **规律性**：确定性算法，同输入+同密钥→相同输出；编码结构可分析
> - **长短双方案**：短模式（紧凑）和长模式（结构化丰富）两种输出策略
> - **字符集可选**：支持多种输出字符集配置

## What Changes

### A. alist-encrypt 插件文件名加密（与 alist-encrypt-go 对齐）

- **Go 后端** (`internal/alistencrypt/`)：确保 EncryptedName 的 mixBase64 编码 + CRC6 校验 + AES-CTR 加密逻辑与参考实现二进制兼容
- **前端** (`features/alist-encrypt/useAlistEncrypt.ts`)：decodeAlistFilename 函数同步更新

### B. ENCV v4 容器文件名混淆 — 深度定制的 ENC-FN 编码方案

核心设计原则：**文件名是展示层元数据，不是存储标识**。

- **Manifest_v4 扩展**：`original_name` 字段存储 UTF-8 原始文件名（明文或 ENC-FN 编码后）
- **Header Flags**：`FlagFilenameEncrypted` (0x0010) 标志位表示 original_name 已编码
- **ENC-FN 编码器** (`internal/v2/filename/encfn.go`)：全新实现的深度定制文件名混淆方案
- **文件名解析优先级**：`Manifest.original_name（解码后） > 物理文件名`
- **原始文件名修改 API**：`PATCH /api/v1/file/rename`
- **边界处理**：超长/空/Unicode 全量支持

## Impact

- Affected code:
  - `internal/alistencrypt/filename.go` — alist-encrypt 插件 EncryptedName 加解密
  - `internal/v2/filename/encfn.go` — **新增** ENC-FN 深度定制编码器
  - `internal/v2/filename/charset.go` — **新增** 可选字符集定义
  - `internal/v2/types/segment_v4.go` — Manifest_v4 新增 original_name / filename_alg 字段
  - `internal/v2/types/header_v4.go` — FlagFilenameEncrypted 标志位
  - `app/encv-mobile/src/features/alist-encrypt/useAlistEncrypt.ts` — 前端解码函数
  - `app/encv-mobile/src/views/Files.vue` — 文件列表显示逻辑

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

---

### Requirement: ENC-FN 深度定制文件名编码方案

系统 SHALL 为 v4 容器提供一套**深度定制的确定性文件名编码方案**（代号 ENC-FN），具备人类可读性、规律性、长短双模式和可选字符集。此方案**不直接复用任何现成的通用加密或编码库**，而是从零设计的专用算法。

#### ENC-FN 算法概要

```
编码流程 (Encode):
  plaintext(UTF-8 bytes) → KDF(password) → S-box生成 → 字节置换 → Feistel轮变换 → 目标字符集映射 → 输出编码串

解码流程 (Decode):
  编码串 → 字符集逆映射 → Feistel逆轮变换 → 字节逆置换 → UTF-8 → 明文
```

#### ENC-FN 核心组件

| 组件 | 说明 | 设计要点 |
|------|------|---------|
| **KDF** | 密钥派生函数 | 基于 HKDF-SHA256 从密码派生主密钥，再派生 S-box 种子和轮密钥 |
| **S-box** | 256 字节置换表 | 由 KDF 输出的种子确定性生成，每次编码/解码重建相同的 S-box |
| **Feistel 网络** | 多轮混淆变换 | 4-8 轮（可配置），每轮使用不同轮密钥，保证雪崩效应 |
| **字符集映射器** | 将字节序列映射到目标字符集 | 支持多种字符集，决定输出的视觉特征 |

#### ENC-FN 长短双方案

| 模式 | 标识 | 输出特征 | 适用场景 |
|------|------|---------|---------|
| **短模式 (compact)** | `C` | 无前缀，纯编码体，最大压缩率 | 文件名较短（<50 字节）、需要最小长度 |
| **长模式 (structured)** | `S` | 带 `[S]` 前缀 + 长度标记 + 编码体 + 校验后缀 | 文件名较长、需要自描述和完整性校验 |

**短模式输出示例**：
```
原始文件名: "report.pdf"
ENC-FN(compact, alnum): "xK7mPq2RnWv"
ENC-FN(compact, hex):    "a3f7b2c91e04d5"
```

**长模式输出示例**：
```
原始文件名: "2024年度财务报表_Q3_final_version.pdf"
ENC-FN(structured, alnum): "[S]44:xK7mPq2RnWvLsT9uYzAbCdEfGhIjKlMnOpQrStUvWxYzAbCdEfGhIjKlMn:p3"
```

其中 `[S]` = 结构化模式前缀，`44` = 原始字节长度（十进制），`:p3` = 截断校验（若启用）

#### ENC-FN 字符集选项

| 字符集 ID | 字符 | 大小 | 特征 |
|-----------|------|------|------|
| `alnum` | `a-z A-Z 0-9` | 62 | 全字母数字，最大信息密度 |
| `safe` | 去掉 `0Oo1lI` 的 alnum | 56 | 排除易混淆字符，适合人工抄写 |
| `hex` | `0-9 a-f` | 16 | 最安全但最长，适合技术环境 |
| `alpha` | `a-z A-Z` | 52 | 纯字母，避免数字混淆 |
| `custom` | 用户自定义字符串 | 可变 | 最大灵活性 |

#### ENC-FN 配置接口

```go
type FNConfig struct {
    Mode     FNMode     // FNCompact | FNStructured
    Charset FNCharset  // FNAlnum | FNSafe | FNHex | FNAlpha | FNCustom(...)
    Rounds   int        // Feistel 轮数 (默认 6, 范围 4-12)
    Truncate bool       // 是否在长模式下截断并附加校验 (默认 true)
}

type FNMode string
const (
    FNCompact    FNMode = "compact"
    FNStructured FNMode = "structured"
)

type FNCharset string
const (
    FNAlnum   FNCharset = "alnum"
    FNSafe    FNCharset = "safe"
    FNHex     FNCharset = "hex"
    FNAlpha   FNCharset = "alpha"
)
```

#### Scenario: ENC-FN 短模式编码
- **WHEN** 使用 ENC-FN compact 模式编码文件名 "video.mp4"，密码 "pass123"，字符集 almun
- **THEN** 输出一个 ~10-16 字符的 alnum 编码串（无前缀无后缀）
- **AND** 同一输入+同一密码始终产生相同输出（确定性）
- **AND** 不同密码产生完全不同的输出（密钥敏感性）

#### Scenario: ENC-FN 长模式编码
- **WHEN** 使用 ENC-FN structured 模式编码文件名 "非常长的中文文件名_包含emoji🎉.txt"
- **THEN** 输出以 `[S]` 开头，包含原始字节长度、编码体、可选校验后缀
- **AND** 解码时可验证完整性和正确性

#### Scenario: ENC-FN 解码失败处理
- **WHEN** 编码串被篡改（字符缺失/多余/错误字符集）
- **THEN** Decode 返回明确的错误（ErrFNInvalidFormat / ErrFNCorrupt / ErrFNChecksumMismatch）
- **AND** 不返回部分乱码结果

#### Scenario: 不同字符集的输出对比
- **WHEN** 同一文件名分别用 alnum / safe / hex 字符集编码
- **THEN** hex 输出约是 alnum 的 1.55 倍长度（log(62)/log(16) ≈ 1.55）
- **AND** safe 输出比 alnum 略长（因基数更小）
- **AND** 三种输出均可被各自的 Decode 正确还原

---

### Requirement: ENCV v4 容器文件名作为展示层抽象

系统 SHALL 将 v4 容器的文件名视为**可恢复的展示层元数据**，而非不可变的存储标识。

#### Scenario: 物理文件名被破坏时从 Manifest 恢复
- **WHEN** v4 容器的物理文件名被重命名为任意字符串
- **AND** Manifest.original_name 存在有效值
- **THEN** 系统（API 返回 / 前端列表）始终显示 Manifest 中的原始文件名（ENC-FN 解码后）

#### Scenario: 启用文件名编码时的显示行为
- **WHEN** 创建 v4 容器时设置了 FlagFilenameEncrypted 且指定了密码和 FNConfig
- **THEN** Manifest.original_name 存储的是 ENC-FN 编码后的文件名
- **AND** 前端/API 层通过 ENC-FN.Decode 还原明文文件名显示
- **AND** 若解码失败（无密码/密码错误），则显示物理文件名或占位符

#### Scenario: 修改原始文件名
- **WHEN** 用户通过 API 或 UI 修改某个 v4 容器的原始文件名
- **THEN** Manifest.original_name 被 ENC-FN.Encode(newName) 更新
- **AND** 文件列表立即反映修改后的文件名
- **AND** 物理文件名不受影响

#### Scenario: 超长文件名处理
- **WHEN** 原始文件名超过 255 字节
- **THEN** Manifest.original_name 完整存储 ENC-FN 编码结果
- **AND** 物理文件名自动缩短为安全长度
- **AND** 展示层始终显示完整的原始文件名

#### Scenario: 空/Unicode 文件名处理
- **WHEN** 原始文件名为空或含 emoji/CJK/控制字符
- **THEN** UTF-8 字节全量传入 ENC-FN 编码，不做过滤
- **AND** 展示层正确渲染

## MODIFIED Requirements

### Requirement: Manifest_v4 数据结构

```go
type Manifest_v4 struct {
    // ... 已有字段 ...
    OriginalName      string `json:"original_name,omitempty"`       // 原始文件名（明文或 ENC-FN 编码）
    FilenameAlgorithm string `json:"filename_alg,omitempty"`        // "none" | "enc-fn:compact:alnum" | "enc-fn:structured:safe" | ...
}
```

`FilenameAlgorithm` 格式：`enc-fn:{mode}:{charset}`，例如 `"enc-fn:compact:alnum"` 或 `"enc-fn:structured:safe"`

### Requirement: Header Flags 位域

```go
const FlagFilenameEncrypted uint16 = 0x0010 // Manifest.original_name 是 ENC-FN 编码存储的
```

### Requirement: 文件名解析优先级

```
1. 若 v4 容器 Manifest.original_name 非空：
   a. FlagFilenameEncrypted 未设置 → 直接作为显示名
   b. FlagFilenameEncrypted 已设置 → ENC-FN.Decode(original_name, password) → 显示（失败 fallback 到步骤 2）
2. 否则 → 物理文件名
```

### Requirement: 前端 decodeAlistFilename

alist-encrypt 插件的 decodeAlistFilename SHALL 支持 EncryptedName 格式（mixBase64 + CRC6 + AES-CTR），与 v4 容器的 ENC-FN 方案独立。

## REMOVED Requirements

无。
