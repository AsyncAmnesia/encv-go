# Tasks

## Phase 1: CipherMode 体系（AES-128-CTR 默认 + AES-256-CTR 可选）

- [ ] Task 1: 在 v4 crypto 层引入 CipherMode 枚举
  - [ ] SubTask 1.1: 在 `internal/v2/crypto/aes_v2.go` 新增常量 `CipherModeAES128CTR = 0`、`CipherModeAES256CTR = 1`
  - [ ] SubTask 1.2: 新增 `KeySize_v4_128 = 16`、`KeySize_v4_256 = 32` 常量
  - [ ] SubTask 1.3: 改写 `GenerateKey_v4(password, salt, keyLen)` 支持 16/32 字节输出（PBKDF2-SHA256, 100000 iter）
  - [ ] SubTask 1.4: `EncryptStream_v2` / `DecryptStream_v2` 接受变长 key（已支持 16/24/32 字节，验证 16 字节路径走通）
  - [ ] SubTask 1.5: 新增单元测试 `TestGenerateKey_VariableLength` 覆盖 16/32 字节

- [ ] Task 2: v4 Header 增加 CipherMode 字段
  - [ ] SubTask 2.1: 在 `internal/v2/types/header_v4.go` 的 `EnvelopeHeaderV4` 结构体增加 `CipherMode uint16`（偏移 4-6 字节后预留）
  - [ ] SubTask 2.2: 修改 `WriteHeaderV4` / `ReadHeaderV4` 序列化/反序列化 `CipherMode`
  - [ ] SubTask 2.3: 旧 v4 容器（无 CipherMode 字段）按 0 解析，保证向后兼容
  - [ ] SubTask 2.4: 与 `plugin-version-selection-and-password-detection` 的 `PasswordHint`（偏移 20-36）共存不冲突
  - [ ] SubTask 2.5: 单元测试 `TestHeaderV4_CipherMode_RoundTrip`

## Phase 2: HMAC-SHA1-80 实现 + Encrypt-then-MAC

- [ ] Task 3: 实现 HMAC-SHA1-80 crypto 原语
  - [ ] SubTask 3.1: 在 `internal/v2/crypto/` 创建 `mac.go`，实现 `HMACSHA1_80(key, message []byte) [10]byte`
  - [ ] SubTask 3.2: 实现 `VerifyHMACSHA1_80(expected, message []byte, key []byte) bool`（常量时间比较）
  - [ ] SubTask 3.3: 引入 `ErrMACMismatch` 错误类型
  - [ ] SubTask 3.4: 单元测试 `TestHMACSHA1_80_KnownVector`（用 RFC 2202 / WinZip AES 规范向量）

- [ ] Task 4: 重构 Segment 加密为 Encrypt-then-MAC
  - [ ] SubTask 4.1: 改写 `internal/v2/crypto/segment_crypto.go` 的 `EncryptSegment`：
    - 接收 `macKey` 参数
    - 加密后追加 `HMAC-SHA1-80(macKey, nonce || ciphertext)`
  - [ ] SubTask 4.2: 改写 `DecryptSegment`：
    - 接收 `macKey` 参数
    - **先验 MAC**，验失败立即返回 `ErrMACMismatch`
    - 验通过再解 CTR
  - [ ] SubTask 4.3: `EncryptStreamToSegments` 接受 `macKey` 参数
  - [ ] SubTask 4.4: 单元测试 `TestEncryptDecryptSegment_WithMAC`
  - [ ] SubTask 4.5: 负向测试 `TestDecryptSegment_TamperedCiphertext_ReturnsErrMACMismatch`（翻转 1 bit 必失败）
  - [ ] SubTask 4.6: 负向测试 `TestDecryptSegment_WrongMACKey_ReturnsErrMACMismatch`（错误密钥必失败）

- [ ] Task 5: mac_key 派生与 Header 存储
  - [ ] SubTask 5.1: 在 `internal/v2/crypto/` 创建 `mac_key.go`，实现 `DeriveMACKey(password, macSalt []byte) []byte`（PBKDF2-SHA256, 100000, 32 bytes）
  - [ ] SubTask 5.2: 在 `internal/v2/types/header_v4.go` 的 `EnvelopeHeaderV4` 增加 `MacSalt [16]byte`（复用 Reserved 区域，偏移 36-52）
  - [ ] SubTask 5.3: `WriteHeaderV4` 生成随机 `MacSalt` 并写入
  - [ ] SubTask 5.4: `ReadHeaderV4` 提取 `MacSalt` 供 `DeriveMACKey` 使用
  - [ ] SubTask 5.5: 单元测试 `TestHeaderV4_MacSalt_RoundTrip`

## Phase 3: SegmentHeader 扩展（ModeFlags + 压缩字段）

- [ ] Task 6: 扩展 SegmentHeader 结构
  - [ ] SubTask 6.1: 在 `internal/v2/types/segment_v4.go` 修改 `SegmentHeader`：
    ```go
    type SegmentHeader struct {
        SegmentID           uint32
        DataLength          uint64
        NonceSize           uint16
        ModeFlags           uint16  // 新增：bit0=Encrypted, bit1-2=Compression
        MACSize             uint16  // 新增：默认 10
        DataCRC32           uint32
        CompressedBlockSize uint16  // 新增：zstd seekable 块大小
        Reserved            uint16
        SeekTableOffset     uint32  // 新增：zstd 时有效
        SeekTableLength     uint32  // 新增
    }
    ```
  - [ ] SubTask 6.2: `SegmentHeaderSize` 从 18 扩展到 34
  - [ ] SubTask 6.3: `MarshalBinary` / `UnmarshalBinary` 处理新字段
  - [ ] SubTask 6.4: 单元测试 `TestSegmentHeader_Extended_RoundTrip`
  - [ ] SubTask 6.5: 定义 `ModeFlag*` 常量（`ModeFlagEncrypted = 1 << 0`，`ModeFlagCompressionZstd = 1 << 1`）

## Phase 4: zstd 压缩 + seekable 集成

- [ ] Task 7: 引入 zstd-seekable 依赖
  - [ ] SubTask 7.1: `go.mod` 添加 `github.com/saracen/go-zstdseekable` 最新稳定版
  - [ ] SubTask 7.2: `go mod tidy` 验证依赖解析
  - [ ] SubTask 7.3: 简单冒烟测试 `TestZstdSeekable_BasicRoundTrip`

- [ ] Task 8: 实现压缩模块
  - [ ] SubTask 8.1: 在 `internal/v2/crypto/compression/` 创建 `zstd.go`
  - [ ] SubTask 8.2: 实现 `CompressZstdSeekable(src io.Reader) (compressed []byte, seekTable []byte, err error)`
  - [ ] SubTask 8.3: 实现 `DecompressZstdSeekable(compressed, seekTable []byte) (plaintext []byte, err error)`
  - [ ] SubTask 8.4: 块大小配置项 `zstd_block_size`（默认 64KB）
  - [ ] SubTask 8.5: 单元测试 `TestZstdSeekable_LargeFile_CompressDecompress`

- [ ] Task 9: Segment 集成压缩
  - [ ] SubTask 9.1: 在 `internal/v2/crypto/segment_crypto.go` 改写 `EncryptSegment`：
    - 接受 `compressionMode string` 参数
    - `compressionMode == "zstd"` 时先压缩再加密
  - [ ] SubTask 9.2: `EncryptSegment` 设置 `ModeFlags.CompressionZstd` 标记
  - [ ] SubTask 9.3: `DecryptSegment` 根据 `ModeFlags` 决定是否解压
  - [ ] SubTask 9.4: <1KB 数据自动跳过压缩，记 `ModeFlags.Compression = none`
  - [ ] SubTask 9.5: 单元测试 `TestEncryptDecryptSegment_ZstdCompressed`
  - [ ] SubTask 9.6: 单元测试 `TestEncryptDecryptSegment_MixedModes`（一个 Segment 压缩、一个不压缩）

## Phase 5: 加密容器去后缀字节流识别

- [ ] Task 10: 扩展 detector 支持 `io.Reader` 检测
  - [ ] SubTask 10.1: 在 `internal/v2/container/detector/detector.go` 新增 `DetectContainerFromReader(r io.Reader) (DetectResult, error)`
  - [ ] SubTask 10.2: 读取前 4 字节判断 "ENVC" 魔数
  - [ ] SubTask 10.3: 读取 Header 完整 2048 字节（PeekReader 风格）
  - [ ] SubTask 10.4: 返回 `IsENCVContainer / Version / ContainerType / IsSeekable / CipherMode`
  - [ ] SubTask 10.5: 旧 `DetectContainerType(path string)` 包装为 `os.Open` + `DetectContainerFromReader`

- [ ] Task 11: 边界测试套件（**核心验证**）
  - [ ] SubTask 11.1: 在 `internal/v2/container/detector/` 创建 `stripped_suffix_test.go`
  - [ ] SubTask 11.2: `TestDetect_StrippedSuffix_Plain`（`mydocument` 无扩展名）
  - [ ] SubTask 11.3: `TestDetect_StrippedSuffix_Dotfile`（`.encv` 隐藏文件）
  - [ ] SubTask 11.4: `TestDetect_StrippedSuffix_WrongExtension`（`mydocument.zip` 应识别为非 ENCV）
  - [ ] SubTask 11.5: `TestDetect_StrippedSuffix_Boundary_Magic`（恰好 4 字节 "ENVC"）
  - [ ] SubTask 11.6: `TestDetect_StrippedSuffix_Boundary_HeaderMinus1`（2047 字节，差 1 字节完整 Header）
  - [ ] SubTask 11.7: `TestDetect_StrippedSuffix_TruncatedAt5Bytes`（5 字节，"ENVC" + 1 字节）
  - [ ] SubTask 11.8: `TestDetect_StrippedSuffix_NonENCVMagic`（"ZIP\x03\x04" 应返回 `IsENCVContainer=false`）
  - [ ] SubTask 11.9: `TestDetect_StrippedSuffix_EmptyFile`（0 字节返回明确错误）
  - [ ] SubTask 11.10: `TestDetect_StrippedSuffix_CipherMode_0` 与 `TestDetect_StrippedSuffix_CipherMode_1`

## Phase 6: writer/reader 集成新能力

- [ ] Task 12: container_writer_v4 集成新能力
  - [ ] SubTask 12.1: 在 `internal/v2/writer/container_writer_v4.go` 改写写入流程：
    - 接受 `CipherMode` / `CompressionMode` / `EnableHMAC` 参数
    - 按 Phase 1+2+4 写入 Header → Segments（每 Segment 独立 nonce+mac_key+可选压缩）→ Manifest → Footer
  - [ ] SubTask 12.2: 写入空 Segment 时不写 MAC（ModeFlags.Encrypted=0 跳过 mac_key 派生）
  - [ ] SubTask 12.3: 集成测试 `TestWriterV4_AES128_WithMAC_WithZstd`
  - [ ] SubTask 12.4: 集成测试 `TestWriterV4_AES256_WithMAC_NoCompression`
  - [ ] SubTask 12.5: 集成测试 `TestWriterV4_MixedSegments_EncryptedAndPlain`

- [ ] Task 13: segment_reader 集成 MAC 校验前置
  - [ ] SubTask 13.1: 在 `internal/v2/reader/segment_reader.go` 改写解密流程：
    - 接受 `macKey` 参数
    - **强制先验 MAC**，验失败立即 `ErrMACMismatch`
  - [ ] SubTask 13.2: 处理 `ModeFlags.Encrypted=0` 的明文 Segment（不验 MAC）
  - [ ] SubTask 13.3: 处理 `ModeFlags.Compression=zstd` 的解压路径
  - [ ] SubTask 13.4: 集成测试 `TestReaderV4_DetectTamperedMAC_ReturnsErrMACMismatch`
  - [ ] SubTask 13.5: 集成测试 `TestReaderV4_DecompressZstd_OnTheFly`

- [ ] Task 14: 配置文件 schema 更新
  - [ ] SubTask 14.1: 在 `config.schema.json` 增加 `v4_cipher_mode` (integer enum [0,1], default 0)
  - [ ] SubTask 14.2: 增加 `v4_compression_mode` (string enum ["none", "zstd"], default "none")
  - [ ] SubTask 14.3: 增加 `v4_enable_hmac` (bool, default true)
  - [ ] SubTask 14.4: 增加 `v4_zstd_block_size` (integer, default 65536)

- [ ] Task 15: 前端 UI 适配
  - [ ] SubTask 15.1: 在 `app/encv-mobile/src/components/EncryptDialog.vue` 新增 "加密强度" 选择（128/256）
  - [ ] SubTask 15.2: 新增 "压缩" 选择（无 / zstd）
  - [ ] SubTask 15.3: 默认值：128 + 无压缩
  - [ ] SubTask 15.4: 选 256 时显示提示 "更慢，强度更高"
  - [ ] SubTask 15.5: 选 zstd 时显示提示 "纯文本/重复二进制可节省 30-70% 空间"

## Task Dependencies

- [Task 3] depends on [Task 1] (HMAC 测试需 CipherMode 体系)
- [Task 4] depends on [Task 3] (Segment 加密需 MAC 原语)
- [Task 6] depends on [Task 4] (SegmentHeader 扩展需 MAC 设计落地)
- [Task 8] depends on [Task 7] (压缩模块需 zstd 依赖)
- [Task 9] depends on [Task 6, 8] (Segment 集成需 Header 扩展 + 压缩模块)
- [Task 12] depends on [Task 1, 4, 6, 9] (writer 集成全部前置)
- [Task 13] depends on [Task 12] (reader 镜像 writer)
- [Task 15] depends on [Task 14] (前端需 schema 落地)

## Parallelization

- Task 1, 3, 7 可并行启动（独立模块）
- Task 11（detector 边界测试）可与 Task 2/5 并行（独立子系统）
- Task 15（前端 UI）可与 Task 12/13 并行（独立代码库）
