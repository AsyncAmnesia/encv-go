# Checklist

## Phase 1: Go 后端算法基础设施 `internal/alistencrypt/`

### 算法隔离骨架
- [x] Cipher 接口定义完整（5 个方法：SetPosition/Encrypt/Decrypt/Algorithm/BlockSize）
- [x] CipherRegistry 使用 RWMutex 并发安全
- [x] Registry init() **仅注册 aesctr**
- [x] 查询 rc4md5/chacha20 → ErrExtensionRequired（非 panic、非 fallback）
- [x] `go vet ./internal/alistencrypt/...` 无 RC4/ChaCha20 import
- [x] `grep -r "rc4\|RC4\|chacha\|ChaCha" internal/alistencrypt/` **无匹配**

### AES-128-CTR
- [x] 密钥派生链与 alist-encrypt-go 逐字节一致
- [x] incrementIV 与 Node.js aesCTR.js 完全一致（含大数溢出）
- [x] SetPosition(0/mid/nearEnd) 后数据解密正确
- [x] 满足 Cipher 接口，Registry 中注册为 "aesctr"

### MixBase64 文件名
- [x] KSA shuffle 输出与参考实现一致
- [x] Encode/Decode 往返无损（中文/特殊字符/长文件名）
- [x] CRC6 校验位计算正确
- [x] EncodeName → DecodeName 往返得到原始文件名

### V2 内容头 + DecryptReader
- [x] AECTR2 magic 正确检测，NonceField/PlainSize 正确提取
- [x] AutoDetectV2 在 V1/V2 下均能正确分支
- [x] DecryptReader 可完整流式读取 + seek 到中间位置

## Phase 2: ENCV Plugin 实现 `internal/v2/plugins/alistencrypt/`

### Plugin 骨架与配置
- [x] AlistEncryptPlugin struct 存在，Name() 返回 "alist_encrypt"
- [x] GetContainerExtension() 返回 settings.Suffix（默认 ".bin"）
- [x] GetDefaultSettings() 返回合法 JSON（Suffix=".bin", EncType="aesctr"）
- [x] Initialize() 正确读取配置并执行校验

### 后缀安全校验
- [x] **冲突校验**：suffix=".sccgv" → Error + 回退 ".bin"
- [x] **冲突校验**：suffix=".encv" → Error + 回退 ".bin"
- [x] **格式校验**：suffix 不以 "." 开头 → Warn + 回退 ".bin"
- [x] **格式校验**：suffix 长度 >16 → Warn + 回退 ".bin"
- [x] **enc_type 校验**：非 "aesctr" → Warn + ErrExtensionRequired

### 加密流程
- [x] Encrypt(dataReader) 端到端：原始数据 → AES-CTR 加密 → 输出文件+suffix
- [x] 可选 V2 头写入（AECTR2 magic + NonceField + PlainSize）

### 解密流程
- [x] CanDecrypt(containerPath)：扩展名匹配返回 true，AECTR2 magic 增强置信度
- [x] Decrypt(containerPath, outputDir) 端到端：加密文件 → AutoDetectV2 → AES-CTR 解密 → 明文输出
- [x] **运行时类型校验**：扩展名不匹配 → 拒绝操作
- [x] **运行时容器碰撞校验**：检测 ENCV 容器头 → 拒绝 + 明确提示「请使用主应用解密」
- [x] 密码错误返回特殊错误码（区别于数据损坏）

### 流式预览
- [x] Stream(path, password) 返回支持 HTTP Range 的数据流
- [x] Range 请求 → 206 Partial Content
- [x] Content-Type 根据扩展名正确推断

### Plugin 接口其余方法
- [x] SupportedMimePrefixes() == nil（不参与自动 MIME 匹配）
- [x] SupportedExtensions() == nil（不参与自动扩展名匹配）
- [x] ShouldProcess() 仅在用户显式选择时返回 true
- [x] 其余接口方法有合理默认实现

### 注册
- [x] registry.go Plugins 切片包含 &alistencrypt.AlistEncryptPlugin{}
- [x] InitializePlugins() 成功初始化无报错
- [x] FindDecryptingPlugin() 能发现 .bin（或自定义 suffix）的 alist-encrypt 文件
- [x] FindEncryptingPlugin() **不会**自动匹配到 alist-encrypt（SupportedMimePrefixes/Extensions 返回 nil）

## Phase 3: API 层与前端集成

### HTTP Endpoint
- [x] GET /api/alist-encrypt/stream 支持 Range → 206 + 解密数据
- [x] GET /api/alist-encrypt/decode-filename 返回解码后的真实文件名

### 前端
- [x] encv.ts streamAlistFile() / decodeAlistFilename() 可调用
- [x] Files.vue 匹配 suffix 文件显示加密标记+真实文件名
- [x] Files.vue 长按菜单出现「解密」和「流式预览」
- [x] 流式预览 URL 可被 MPV/ArtPlayer 加载播放
- [x] Tasks.vue 展示 alist-encrypt 任务状态和错误信息
- [x] i18n 中英文翻译 key 已定义

## Phase 4: 测试与 CI
- [x] `go test ./internal/alistencrypt/...` 全部通过（52 个子测试，7 个测试组）
- [ ] `go test ./internal/v2/plugins/alistencrypt/...` 全部通过
- [ ] 端到端：加密→解密→数据一致性验证通过
- [ ] Range seek 测试通过
- [ ] CI workflow 包含 Go 测试步骤
- [ ] CI 包含隔离性 grep 验证

## 隔离性验证（CI 必须通过）
- [ ] `go build ./internal/alistencrypt/` 成功，产物不含 RC4/ChaCha20 符号
- [x] ComboLite 插件目录不存在（本方案不需要）
