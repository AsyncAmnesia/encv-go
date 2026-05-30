# Tasks

## Phase 1: 核心算法层（Go 后端，仅 AES-128-CTR）

- [ ] **Task 1.0**: 定义 Cipher 扩展接口与注册表（算法隔离骨架）
  - [ ] 1.0.1 在 `cipher.go` 中定义 `Cipher` 接口（SetPosition/Encrypt/Decrypt/Algorithm/BlockSize）
  - [ ] 1.0.2 在 `cipher.go` 中定义 `CipherFactory` 类型和 `Register(encType, factory)` 函数
  - [ ] 1.0.3 在 `registry.go` 中实现注册表（map[string]CipherFactory + mutex 保护）
  - [ ] 1.0.4 在 `registry.go` 的 init() 中 **仅注册 aesctr**
  - [ ] 1.0.5 定义 `ErrExtensionRequired` 错误类型（enc_type 不支持时返回）
  - [ ] 1.0.6 编写验证测试：确认 RC4MD5 / ChaCha20 **未** 被注册，调用时返回 ErrExtensionRequired

- [ ] **Task 1.1**: 实现 AES-128-CTR 核心密码器（唯一内置实现）
  - [ ] 1.1.1 实现 `AesCtrCipher` 结构体：NewAesCtr(password, fileSize) 密钥派生（PBKDF2+MD5 链）
  - [ ] 1.1.2 实现 `SetPosition(position)` seek 方法（128-bit counter increment）
  - [ ] 1.1.3 实现 `Encrypt(data)` / `Decrypt(data)` XORKeyStream 操作
  - [ ] 1.1.4 确认 AesCtrCipher 满足 Cipher 接口并在 registry 中注册为 `"aesctr"`
  - [ ] 1.1.5 编写单元测试：密钥派生与 alist-encrypt-go 参考实现对比（使用已知向量验证）

- [ ] **Task 1.2**: 实现共享基础设施 — 文件名 MixBase64 加解密
  - [ ] 1.2.1 实现 KSA (Key Scheduling Algorithm) shuffle：initKSA(passwd) → 64 字符字母表
  - [ ] 1.2.2 实现 MixBase64 Encode/Decode（自定义 alphabet 的 Base64）
  - [ ] 1.2.3 实现 CRC6 校验（6-bit CRC，多项式 x^6+x+1, 反射输入输出）
  - [ ] 1.2.4 实现 EncodeName / DecodeName / ConvertShowName / ConvertRealName
  - [ ] 1.2.5 编写单元测试：文件名往返编解码 + CRC6 校验

- [ ] **Task 1.3**: 实现共享基础设施 — V2 内容头检测与解析
  - [ ] 1.3.1 实现 ContentHeader 解析（magic AECTR2 检测、NonceField 提取、PlainSize 读取）
  - [ ] 1.3.2 实现 AutoDetectV2 格式（前缀 peek → 自动分支 V1/V2 路径）
  - [ ] 1.3.3 实现 DecryptReader 包装器（io.Reader → 自动解密流）

- [ ] **Task 1.4**: 实现 FlowEnc 调度器（根据 encType 分发到对应 Cipher）
  - [ ] 1.4.1 实现 NewFlowEnc(password, encType, fileSize) → 查询 registry → 创建 Cipher
  - [ ] 1.4.2 enc_type 不在 registry 中时返回 ErrExtensionRequired（**不 fallback 到 aesctr**）
  - [ ] 1.4.3 封装 EncryptReader / DecryptReader / SetPosition 委托给底层 Cipher

## Phase 2: 配置与 API 层

- [ ] **Task 2.1**: 扩展 Config 结构体
  - [ ] 2.1.1 在 `internal/config/config.go` 中新增 `AlistEncrypt` 配置段
  - [ ] 2.1.2 enc_type 校验：非 aesctr 时 **警告日志** + 返回配置错误或标记需扩展
  - [ ] 2.1.3 更新 `config.schema.json` 添加 alist_encrypt 字段定义
  - [ ] 2.1.4 在 `config.user.json` 中添加默认配置示例

- [ ] **Task 2.2**: 新增移动端 API 端点
  - [ ] 2.2.1 `POST /api/alist-encrypt/decrypt` — 发起解密任务（sourcePath + password → targetDir）
  - [ ] 2.2.2 `POST /api/alist-encrypt/encrypt` — 发起加密任务（sourcePath + password → targetDir + suffix）
  - [ ] 2.2.3 `GET /api/alist-encrypt/stream` — 流式解密预览（支持 HTTP Range，返回解密后的数据流）
  - [ ] 2.2.4 `GET /api/alist-encrypt/decode-filename` — 文件名在线解码工具（encodedName + password → plainName）

- [ ] **Task 2.3**: 扩展 TaskManager 支持新任务类型
  - [ ] 2.3.1 在 TaskManager 中注册 `alist-decrypt` / `alist-encrypt` 任务处理器
  - [ ] 2.3.2 实现异步解密逻辑（读取加密文件 → FlowEnc → AesCtrCipher 解密 → 写入目标路径）
  - [ ] 2.3.3 实现异步加密逻辑（读取原始文件 → FlowEnc → AesCtrCipher 加密 → 写入目标路径 + 后缀）
  - [ ] 2.3.4 实现进度报告（已处理字节 / 总字节数百分比）
  - [ ] 2.3.5 复用 WebSocket 推送进度到前端

## Phase 3: 移动端集成（Android）

- [ ] **Task 3.1**: 前端 API 层适配
  - [ ] 3.1.1 在 `app/encv-mobile/src/api/encv.ts` 中新增 `decryptAlistEncrypt()` / `encryptAlistEncrypt()` / `getAlistEncryptStreamUrl()` 函数
  - [ ] 3.1.2 新增 `decodeAlistFilename()` API 调用函数

- [ ] **Task 3.2**: Files.vue 文件识别与操作入口
  - [ ] 3.2.1 检测文件扩展名匹配 `config.alist_encrypt.suffix` 时显示「alist-encrypt 加密」标记
  - [ ] 3.2.2 文件列表中自动调用 decode-filename API 显示真实文件名
  - [ ] 3.2.3 长按菜单增加「解密」和「流式预览」选项
  - [ ] 3.2.4 流式预览时将 stream URL 传给播放器（MPV 或 ArtPlayer）

- [ ] **Task 3.3**: Tasks.vue 任务展示适配
  - [ ] 3.3.1 `type=alist-decrypt` / `type=alist-encrypt` 任务正确显示在任务列表
  - [ ] 3.3.2 错误信息展示（密码错误特殊提示、数据损坏提示、ErrExtensionRequired 提示）

## Phase 4: TODO（后续迭代，不在 MVP 范围）

- [ ] **[TODO] Task 4.1**: OpenList 代理集成 — 将 alist-encrypt 兼容接入 `internal/openlist/` 代理链，支持通过 OpenList 网盘 URL 直接代理解密
- [ ] **[TODO] Task 4.2**: ENCV Plugin 注册 — 将 alist-encrypt 作为 ENCV v2 Plugin 接口的实现注册到 plugins.Registry（需评估接口兼容性）
- [ ] **[TODO] Task 4.3**: 桌面端 UI 支持 — openlist 桌面客户端的 alist-encrypt 文件操作界面
- [ ] **[TODO] Task 4.4**: RC4MD5 / ChaCha20 **扩展包** — 在独立包中实现 Cipher 接口，通过 Register() 注册。**禁止引入主应用 `internal/alistencrypt/`**

# Task Dependencies
- [Task 1.1] depends on [Task 1.0] （AES-CTR 实现依赖 Cipher 接口和 Registry 先就绪）
- [Task 1.2] depends on [Task 1.1] （MixBase64 依赖 passwdOutward 派生，复用 PBKDF2 逻辑）
- [Task 1.3] depends on [Task 1.1] （V2 头解析依赖 AesCtrCipher V2 初始化）
- [Task 1.4] depends on [Task 1.0], [Task 1.1] （FlowEnc 依赖 Registry 和具体 Cipher 实现）
- [Task 2.2] depends on [Task 1.1], [Task 1.2], [Task 1.3], [Task 1.4] （API 依赖核心算法完成）
- [Task 2.3] depends on [Task 2.1], [Task 2.2] （Task管理依赖配置和 API 就绪）
- [Task 3.1] depends on [Task 2.2] （前端 API 依赖后端 API 完成）
- [Task 3.2] depends on [Task 3.1] （Files.vue 依赖 API 层）
- [Task 3.3] depends on [Task 2.3], [Task 3.1] （Tasks.vue 依赖任务系统和 API）
