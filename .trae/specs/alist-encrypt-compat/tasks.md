# Tasks

## Phase 1: Go 后端 — 算法基础设施包 `internal/alistencrypt/`

- [x] **Task 1.0**: 定义 Cipher 扩展接口与注册表（算法隔离骨架）
  - [x] 1.0.1 创建 `cipher.go`：Cipher 接口 + CipherFactory + Register/Create 函数
  - [x] 1.0.2 创建 `registry.go`：注册表（map+RWMutex）+ init() 仅注册 aesctr
  - [x] 1.0.3 创建 `errors.go`：ErrExtensionRequired 等错误类型
  - [x] 1.0.4 隔离边界测试：RC4MD5/ChaCha20 未注册，调用返回 ErrExtensionRequired
  - [x] 1.0.5 `go vet ./internal/alistencrypt/...` 无 RC4/ChaCha20 import

- [x] **Task 1.1**: 实现 AES-128-CTR 核心密码器
  - [x] 1.1.1 创建 `aesctr.go`：NewAesCtr(password, fileSize) 密钥派生链（PBKDF2→hex→MD5 key+iv）
  - [x] 1.1.2 incrementIV（128-bit 大端分段进位，4×uint32）
  - [x] 1.1.3 SetPosition(position) seek 方法
  - [x] 1.1.4 Encrypt/Decrypt（crypto/cipher NewCTR）
  - [x] 1.1.5 在 registry 注册为 "aesctr"
  - [x] 1.1.6 单元测试：alist-encrypt-go 参考向量验证

- [x] **Task 1.2**: 实现文件名 MixBase64 加解密
  - [x] 1.2.1 创建 `filename.go`：KSA shuffle + MixBase64 Encode/Decode + CRC6 + EncodeName/DecodeName
  - [x] 1.2.2 单元测试：中文文件名往返、CRC6 校验

- [x] **Task 1.3**: 实现 V2 内容头检测与流式包装器
  - [x] 1.3.1 创建 `content_header.go`：AECTR2 magic 检测 + AutoDetectV2 分支
  - [x] 1.3.2 创建 `reader.go`：DecryptReader（io.Reader 包装器，V1/V2 自动分流 + seek）

## Phase 2: Go 后端 — ENCV Plugin 实现 `internal/v2/plugins/alistencrypt/`

- [x] **Task 2.1**: Plugin 骨架与配置
  - [x] 2.1.1 创建目录及 `plugin.go`：AlistEncryptPlugin struct，实现 Name/GetContainerExtension/GetDefaultSettings/GetSettingsSchemaType/Initialize
  - [x] 2.1.2 创建 `types.go`：AlistEncryptPluginConfig（Suffix/DefaultPassword/EncType），默认 Suffix=".bin"
  - [x] 2.1.3 Initialize() 中实现**后缀双重校验**：
    - 冲突校验：suffix 在黑名单 [".sccgv",".encv"] → Error + 回退 ".bin"
    - 格式校验：suffix 不以 "." 开头或 >16 → Warn + 回退 ".bin"
    - enc_type 非 "aesctr" → Warn + ErrExtensionRequired

- [x] **Task 2.2**: 实现加密流程
  - [x] 2.2.1 创建 `encryptor.go`：Encrypt(dataReader) 方法
    - 读取原始数据 → AesCtrCipher 加密 → 写入目标路径+suffix（可选 V2 头）
  - [x] 2.2.2 运行时校验：操作前检查目标路径合法性
  - [x] 2.2.3 端到端测试：加密→解密往返验证

- [x] **Task 2.3**: 实现解密流程
  - [x] 2.3.1 创建 `decryptor.go`：Decrypt(containerPath, outputDir) + CanDecrypt(containerPath) 方法
    - CanDecrypt：扩展名匹配 + 可选 AECTR2 magic 检测
    - Decrypt：打开文件 → AutoDetectV2 → AesCtrCipher 解密 → 写入 outputDir
  - [x] 2.3.2 运行时**双重校验**：
    - 类型校验：扩展名不匹配 → 拒绝
    - 容器碰撞校验：检测 ENCV 容器头 → 拒绝 + 明确提示
  - [x] 2.3.3 错误分类：密码错误（特殊码）、数据损坏、ErrExtensionRequired

- [x] **Task 2.4**: 实现流式预览
  - [x] 2.4.1 创建 `streamer.go`：Stream(path, password) → DecryptReader + HTTP Range 处理逻辑
  - [x] 2.4.2 支持 206 Partial Content + Content-Type 推断 + Content-Length

- [x] **Task 2.5**: 实现 Plugin 接口其余方法（默认实现）
  - [x] 2.5.1 SupportedMimePrefixes() → nil（不参与自动 MIME 匹配）
  - [x] 2.5.2 SupportedExtensions() → nil（不参与自动扩展名匹配）
  - [x] 2.5.3 ShouldProcess() → 基于 explicit user action
  - [x] 2.5.4 其余接口方法提供合理的默认/空实现

- [x] **Task 2.6**: 注册到 Plugins 列表
  - [x] 2.6.1 在 `internal/v2/plugins/registry.go` 的 Plugins 切片中追加 `&alistencrypt.AlistEncryptPlugin{}`
  - [x] 2.6.2 验证 InitializePlugins() 不报错
  - [x] 2.6.3 验证 FindDecryptingPlugin() 能发现 alist-encrypt 格式文件

## Phase 3: API 层与前端集成

- [x] **Task 3.1**: 新增 HTTP endpoint
  - [x] 3.1.1 `GET /api/alist-encrypt/stream` — 流式解密预览（HTTP Range 支持）
  - [x] 3.1.2 `GET /api/alist-encrypt/decode-filename` — 文件名在线解码
  - [x] 3.1.3 （评估现有 task API 是否可直接复用 encrypt/decrypt，如需调整则修改）

- [x] **Task 3.2**: 前端适配
  - [x] 3.2.1 `encv.ts` 新增 streamAlistFile() / decodeAlistFilename()
  - [x] 3.2.2 Files.vue：识别 suffix 匹配文件 → 显示加密标记 + decodeAlistFilename 显示真实名称
  - [x] 3.2.3 Files.vue 长按菜单增加「解密」和「流式预览」
  - [x] 3.2.4 Tasks.vue：alist-encrypt 任务状态展示（应自动被现有任务系统覆盖）
  - [x] 3.2.5 i18n 新增翻译 key（中英文）

## Phase 4: 测试与 CI

- [ ] **Task 4.1**: Go 单元测试完善
  - [x] 4.1.1 `go test ./internal/alistencrypt/...` 全部通过（52 个子测试覆盖 7 个测试组）
  - [ ] 4.1.2 `go test ./internal/v2/plugins/alistencrypt/...` 全部通过
  - [ ] 4.1.3 端到端集成测试：加密文件 → 解密 → 数据一致性验证
  - [ ] 4.1.4 流式预览 Range 请求测试

- [ ] **Task 4.2**: CI 集成
  - [ ] 4.2.1 Go 测试步骤加入 CI workflow
  - [ ] 4.2.2 隔离性验证加入 CI（grep 确认无 RC4/ChaCha20）

## Phase 5: TODO（后续迭代，不在 MVP）

- [ ] **[TODO] Task 5.1**: OpenList 代理集成 — 接入 internal/openlist/ 代理链
- [ ] **[TODO] Task 5.2**: 桌面端 UI — openlist 桌面客户端适配
- [ ] **[TODO] Task 5.3**: RC4MD5 / ChaCha20 **扩展包** — 独立包实现 Cipher 接口。**禁止引入 internal/alistencrypt/**

# Task Dependencies
- [Task 1.1] depends on [Task 1.0]
- [Task 1.2] depends on [Task 1.1]
- [Task 1.3] depends on [Task 1.1]
- [Task 2.1] depends on [Task 1.1], [Task 1.2], [Task 1.3]
- [Task 2.2] depends on [Task 2.1]
- [Task 2.3] depends on [Task 2.1], [Task 1.3]
- [Task 2.4] depends on [Task 2.3], [Task 1.3]
- [Task 2.5] depends on [Task 2.1]
- [Task 2.6] depends on [Task 2.1]..[Task 2.5]
- [Task 3.1] depends on [Task 2.4], [Task 2.3]
- [Task 3.2] depends on [Task 3.1]
- [Task 4.1] depends on [Task 2.6], [Task 3.1]
- [Task 4.2] depends on [Task 4.1]
