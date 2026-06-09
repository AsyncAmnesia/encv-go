# Checklist

## Phase 1: CipherMode 体系

- [ ] `CipherModeAES128CTR` 和 `CipherModeAES256CTR` 常量在 `internal/v2/crypto/aes_v2.go` 中定义
- [ ] `KeySize_v4_128 = 16` 和 `KeySize_v4_256 = 32` 常量已定义
- [ ] `GenerateKey_v4` 支持 16 字节和 32 字节输出
- [ ] PBKDF2 派生参数：SHA-256、100000 iter、salt 16+ bytes
- [ ] `TestGenerateKey_VariableLength` 单元测试通过（16 字节 / 32 字节两路径）
- [ ] `EncryptStream_v2` / `DecryptStream_v2` 16 字节密钥走通 AES-128
- [ ] v4 Header `CipherMode` 字段在 `internal/v2/types/header_v4.go` 中定义
- [ ] `WriteHeaderV4` / `ReadHeaderV4` 序列化 `CipherMode` 字段
- [ ] 旧 v4 容器（无 CipherMode）按 0 解析不报错
- [ ] `PasswordHint`（偏移 20-36）与 `CipherMode` 共存不冲突
- [ ] `TestHeaderV4_CipherMode_RoundTrip` 通过

## Phase 2: HMAC-SHA1-80 + Encrypt-then-MAC

- [ ] `internal/v2/crypto/mac.go` 文件存在
- [ ] `HMACSHA1_80(key, message []byte) [10]byte` 函数实现
- [ ] `VerifyHMACSHA1_80(expected, message []byte, key []byte) bool` 常量时间比较
- [ ] `ErrMACMismatch` 错误类型在 crypto 包中导出
- [ ] `TestHMACSHA1_80_KnownVector` 通过（RFC 2202 标准向量）
- [ ] `EncryptSegment` 接受 `macKey` 参数并在密文后追加 10 字节 MAC
- [ ] `DecryptSegment` 接受 `macKey` 参数并先验 MAC
- [ ] MAC 校验失败时不解密、立即返回 `ErrMACMismatch`
- [ ] `TestEncryptDecryptSegment_WithMAC` 通过
- [ ] `TestDecryptSegment_TamperedCiphertext_ReturnsErrMACMismatch` 通过（翻转 1 bit 必失败）
- [ ] `TestDecryptSegment_WrongMACKey_ReturnsErrMACMismatch` 通过
- [ ] `DeriveMACKey` 函数存在（PBKDF2-SHA256, 100000, 32 bytes）
- [ ] v4 Header `MacSalt [16]byte` 字段存在（偏移 36-52）
- [ ] `TestHeaderV4_MacSalt_RoundTrip` 通过

## Phase 3: SegmentHeader 扩展

- [ ] `SegmentHeader` 结构体增加 `ModeFlags` / `MACSize` / `CompressedBlockSize` / `SeekTableOffset` / `SeekTableLength` 字段
- [ ] `SegmentHeaderSize` 从 18 扩展到 34
- [ ] `MarshalBinary` / `UnmarshalBinary` 处理新字段
- [ ] `ModeFlagEncrypted = 1 << 0` 常量定义
- [ ] `ModeFlagCompressionZstd = 1 << 1` 常量定义
- [ ] `TestSegmentHeader_Extended_RoundTrip` 通过

## Phase 4: zstd + seekable

- [ ] `go.mod` 包含 `github.com/saracen/go-zstdseekable`
- [ ] `go mod tidy` 无错误
- [ ] `TestZstdSeekable_BasicRoundTrip` 通过
- [ ] `internal/v2/crypto/compression/zstd.go` 存在
- [ ] `CompressZstdSeekable` / `DecompressZstdSeekable` 函数实现
- [ ] 默认块大小 64KB 可配
- [ ] `TestZstdSeekable_LargeFile_CompressDecompress` 通过（10MB 文本）
- [ ] `EncryptSegment` 接受 `compressionMode` 参数
- [ ] `compressionMode == "zstd"` 时先压缩后加密
- [ ] `ModeFlags.CompressionZstd` 正确设置
- [ ] `DecryptSegment` 根据 `ModeFlags` 决定是否解压
- [ ] <1KB 数据自动跳过压缩
- [ ] `TestEncryptDecryptSegment_ZstdCompressed` 通过
- [ ] `TestEncryptDecryptSegment_MixedModes` 通过（一个压缩 + 一个不压缩）

## Phase 5: 去后缀字节流识别

- [ ] `DetectContainerFromReader(io.Reader) (DetectResult, error)` 函数实现
- [ ] 前 4 字节魔数检查 "ENVC"
- [ ] 完整 2048 字节 Header 解析
- [ ] 返回字段含 `IsENCVContainer / Version / ContainerType / IsSeekable / CipherMode`
- [ ] 旧 `DetectContainerType(path string)` API 保留并复用新函数
- [ ] `internal/v2/container/detector/stripped_suffix_test.go` 文件存在
- [ ] `TestDetect_StrippedSuffix_Plain` 通过
- [ ] `TestDetect_StrippedSuffix_Dotfile` 通过
- [ ] `TestDetect_StrippedSuffix_WrongExtension` 通过
- [ ] `TestDetect_StrippedSuffix_Boundary_Magic`（恰好 4 字节）通过
- [ ] `TestDetect_StrippedSuffix_Boundary_HeaderMinus1`（2047 字节）通过
- [ ] `TestDetect_StrippedSuffix_TruncatedAt5Bytes` 通过
- [ ] `TestDetect_StrippedSuffix_NonENCVMagic`（"PK\x03\x04"）通过
- [ ] `TestDetect_StrippedSuffix_EmptyFile` 通过
- [ ] `TestDetect_StrippedSuffix_CipherMode_0` 通过
- [ ] `TestDetect_StrippedSuffix_CipherMode_1` 通过

## Phase 6: writer/reader/前端集成

- [ ] `container_writer_v4` 接受 `CipherMode` / `CompressionMode` / `EnableHMAC` 参数
- [ ] `TestWriterV4_AES128_WithMAC_WithZstd` 集成测试通过
- [ ] `TestWriterV4_AES256_WithMAC_NoCompression` 集成测试通过
- [ ] `TestWriterV4_MixedSegments_EncryptedAndPlain` 集成测试通过
- [ ] `segment_reader` 强制先验 MAC，验失败返回 `ErrMACMismatch`
- [ ] `TestReaderV4_DetectTamperedMAC_ReturnsErrMACMismatch` 通过
- [ ] `TestReaderV4_DecompressZstd_OnTheFly` 通过
- [ ] `config.schema.json` 含 `v4_cipher_mode` / `v4_compression_mode` / `v4_enable_hmac` / `v4_zstd_block_size`
- [ ] `app/encv-mobile/src/components/EncryptDialog.vue` 含"加密强度"选择（128/256）
- [ ] 前端默认 128 + 无压缩
- [ ] 选 256 显示 "更慢，强度更高" 提示
- [ ] 选 zstd 显示 "纯文本/重复二进制可节省 30-70% 空间" 提示

## 全局回归

- [ ] `go test ./internal/v2/... -count=1` 全绿
- [ ] `go test ./internal/v2/crypto/... -count=1` 全绿
- [ ] `go test ./internal/v2/container/detector/... -count=1` 全绿
- [ ] `go test ./internal/v2/writer/... -count=1` 全绿
- [ ] `go test ./internal/v2/reader/... -count=1` 全绿
- [ ] `go vet ./internal/v2/...` 无错误
- [ ] `golangci-lint run ./internal/v2/...` 无错误（若配置）
- [ ] 不破坏现有 v2/v3 容器读写（向后兼容测试通过）
- [ ] 旧 v4 容器（无 CipherMode / 无 MacSalt）能正常读取
