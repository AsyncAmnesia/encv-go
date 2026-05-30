# Tasks

## Phase 1: Go 后端 — 算法基础设施包 `internal/alistencrypt/`

- [ ] **Task 1.0**: 定义 Cipher 扩展接口与注册表（算法隔离骨架）
  - [ ] 1.0.1 创建 `cipher.go`：定义 Cipher 接口（SetPosition/Encrypt/Decrypt/Algorithm/BlockSize）+ CipherFactory 类型 + Register/Create 函数
  - [ ] 1.0.2 创建 `registry.go`：实现注册表（map + RWMutex）+ init() 仅注册 aesctr
  - [ ] 1.0.3 创建 `errors.go`：定义 ErrExtensionRequired 等错误类型
  - [ ] 1.0.4 编写隔离边界测试：确认 RC4MD5/ChaCha20 未注册，调用返回 ErrExtensionRequired
  - [ ] 1.0.5 `go vet ./internal/alistencrypt/...` 无 RC4/ChaCha20 相关 import

- [ ] **Task 1.1**: 实现 AES-128-CTR 核心密码器（唯一内置实现）
  - [ ] 1.1.1 创建 `aesctr.go`：NewAesCtr(password, fileSize) 密钥派生链（PBKDF2→hex→MD5 key + MD5 iv）
  - [ ] 1.1.2 实现 incrementIV（128-bit 大端分段进位，4×uint32）
  - [ ] 1.1.3 实现 SetPosition(position) seek 方法（恢复IV→increment→重建CTR→discard offset）
  - [ ] 1.1.4 实现 Encrypt/Decrypt（crypto/cipher NewCTR XORKeyStream）
  - [ ] 1.1.5 在 registry 中注册为 "aesctr"
  - [ ] 1.1.6 单元测试：使用 alist-encrypt-go 参考向量验证密钥派生 + seek 解密正确性

- [ ] **Task 1.2**: 实现文件名 MixBase64 加解密
  - [ ] 1.2.1 创建 `filename.go`：KSA shuffle（passwdOutward → 64字符字母表）+ MixBase64 Encode/Decode
  - [ ] 1.2.2 实现 CRC6 校验（多项式 x^6+x+1, 反射模式, 6-bit → sourceChars 映射）
  - [ ] 1.2.3 实现 EncodeName / DecodeName / ConvertShowName / ConvertRealName
  - [ ] 1.2.4 单元测试：中文文件名往返、特殊字符、长文件名、CRC6 校验

- [ ] **Task 1.3**: 实现 V2 内容头检测与流式包装器
  - [ ] 1.3.1 创建 `content_header.go`：AECTR2 magic 检测 + NonceField/PlainSize 解析 + AutoDetectV2 分支逻辑
  - [ ] 1.3.2 创建 `reader.go`：DecryptReader（io.Reader 包装器，支持自动 V1/V2 分流 + seek）

## Phase 2: Go 后端 — API 与服务层

- [ ] **Task 2.1**: 扩展 Config 结构体
  - [ ] 2.1.1 在 `internal/config/config.go` 新增 AlistEncrypt 配置段（enabled/suffix/default_password/enc_type）
  - [ ] 2.1.2 **后缀冲突校验**：suffix 为 `.sccgv`/`.encv` 时 ERROR + 功能禁用或回退到 `.bin`
  - [ ] 2.1.3 **后缀格式校验**：suffix 不以 `.` 开头或长度 >16 时 WARNING + 回退 `.bin`
  - [ ] 2.1.4 更新 config.schema.json + config.user.json 示例（默认 suffix 为 `.bin`）

- [ ] **Task 2.2**: 新增移动端 API endpoint
  - [ ] 2.2.1 `POST /api/alist-encrypt/decrypt` — 发起解密任务（sourcePath+password → targetDir）
  - [ ] 2.2.2 `POST /api/alist-encrypt/encrypt` — 发起加密任务（sourcePath+password → targetDir+suffix）
  - [ ] 2.2.3 `GET /api/alist-encrypt/stream` — 流式解密预览（HTTP Range 支持）
  - [ ] 2.2.4 `GET /api/alist-encrypt/decode-filename` — 文件名在线解码

- [ ] **Task 2.3**: 实现业务 Service 层
  - [ ] 2.3.1 在 `mobile_service.go` 新增解密方法：读取加密文件 → AutoDetectV2 → AesCtrCipher.DecryptReader → 写入目标路径
  - [ ] 2.3.2 新增加密方法：读取原始文件 → AesCtrCipher.EncryptWriter → 写入目标路径+suffix（可选V2头）
  - [ ] 2.3.3 新增 stream 方法：DecryptReader → HTTP Range 响应（206 Partial Content）
  - [ ] 2.3.4 新增 decode-filename 方法：DecodeName 同步返回
  - [ ] 2.3.5 **运行时双重校验**：
    - 操作前检查文件扩展名是否匹配 config.suffix（不匹配→拒绝）
    - 解密前可选容器碰撞校验（检测 ENCV 容器头→拒绝+明确提示）

- [ ] **Task 2.4**: 扩展 TaskManager 支持新任务类型
  - [ ] 2.4.1 注册 alist-decrypt / alist-encrypt 任务处理器
  - [ ] 2.4.2 异步执行解密/加密，进度通过 WebSocket 推送
  - [ ] 2.4.3 错误分类：密码错误（特殊码）、数据损坏、ErrExtensionRequired

## Phase 3: ComboLite 插件 — 业务编排层 `plugin-alist-decrypt`

- [ ] **Task 3.1**: 创建插件模块骨架
  - [ ] 3.1.1 创建 `app/encv-mobile/plugin-alist-decrypt/` 目录及 build.gradle.kts（复用 mpv-player 模板）
  - [ ] 3.1.2 配置 compileOnly(libs.combolite.core) + Kotlin stdlib/reflect（遵循 compileOnly 共享依赖模式）
  - [ ] 3.1.3 AndroidManifest.xml 声明 AlistDecryptActivity（exported=false）
  - [ ] 3.1.4 创建 AlistDecryptPluginEntry.kt（IPluginEntryClass 入口）

- [ ] **Task 3.2**: 实现 AlistDecryptActivity（纯 UI + API 调度，不含算法）
  - [ ] 3.2.1 从 Intent extras 读取 action + filePath + password + suffix
  - [ ] 3.2.2 decrypt action: 调用 Go 后端 POST /api/alist-encrypt/decrypt → 订阅 WebSocket 进度 → 显示进度 UI
  - [ ] 3.2.3 encrypt action: 调用 Go 后端 POST /api/alist-encrypt/encrypt → 进度展示
  - [ ] 3.2.4 stream action: 构造 Go 后端 stream URL → setResult 返回给宿主传给 MPV
  - [ ] 3.2.5 decode-filename action: 调用 Go 后端 GET decode-filename API → setResult 返回
  - [ ] 3.2.6 遵循 EncvHostActivity 四层超时防御规范

## Phase 4: 宿主端集成（Android + 前端）

- [ ] **Task 4.1**: GoProcessPlugin 新增方法
  - [ ] 4.1.1 decryptAlistFile: 直接调 Go 后端 API（或启动插件 Activity）
  - [ ] 4.1.2 encryptAlistFile: 同上
  - [ ] 4.1.3 streamAlistFile: 返回 Go 后端 stream URL 或启动插件 Activity
  - [ ] 4.1.4 decodeAlistFilename: 直接调 Go 后端 API（同步，无需插件参与）
  - [ ] 4.1.5 未安装后端服务时返回友好提示

- [ ] **Task 4.2**: PlayerEntry + 前端 API
  - [ ] 4.2.1 PlayerEntry 新增 buildAlistDecryptIntent()（如需启动插件 Activity 时用）
  - [ ] 4.2.2 encv.ts 新增 decryptAlistFile/encryptAlistFile/streamAlistFile/decodeAlistFilename 函数

- [ ] **Task 4.3**: Files.vue 文件识别与操作
  - [ ] 4.3.1 检测 suffix 匹配 → 加密标记 + decodeAlistFilename 显示真实名称
  - [ ] 4.3.2 长按菜单增加「解密」和「流式预览」
  - [ ] 4.3.3 流式预览 URL 传给 MPV/ArtPlayer 播放

- [ ] **Task 4.4**: ExtensionsPage.vue + Tasks.vue + i18n
  - [ ] 4.4.1 ExtensionsPage 新增 alist-decrypt 卡片
  - [ ] 4.4.2 COMBO_LITE_ID_MAP 增加 'alist-decrypt' → 'com.encvgo.plugin.alistdecrypt'
  - [ ] 4.4.3 Tasks.vue 支持 alist-decrypt/alist-encrypt 任务状态展示
  - [ ] 4.4.4 i18n 新增翻译 key（中英文）

## Phase 5: CI 构建

- [ ] **Task 5.1**: settings.gradle.kts 包含 :plugin-alist-decrypt
- [ ] **Task 5.2**: CI workflow 新增 plugin-alist-decrypt 的 aar2apk 构建
- [ ] **Task 5.3**: Go 测试 `go test ./internal/alistencrypt/...` 加入 CI

## Phase 6: TODO（后续迭代）

- [ ] **[TODO] Task 6.1**: OpenList 代理集成 — 接入 internal/openlist/ 代理链
- [ ] **[TODO] Task 6.2**: ENCV Plugin 注册 — 注册到 ENCV v2 plugins.Registry
- [ ] **[TODO] Task 6.3**: 桌面端 UI — openlist 桌面客户端适配
- [ ] **[TODO] Task 6.4**: RC4MD5 / ChaCha20 **扩展包** — 独立包实现 Cipher 接口。**禁止引入 internal/alistencrypt/**

# Task Dependencies
- [Task 1.1] depends on [Task 1.0] （AES-CTR 需要 Cipher 接口和 Registry）
- [Task 1.2] depends on [Task 1.1] （MixBase64 复用 passwdOutward 派生逻辑）
- [Task 1.3] depends on [Task 1.1] （V2 头解析依赖 AesCtrCipher 初始化）
- [Task 2.2] depends on [Task 1.1], [Task 1.2], [Task 1.3] （API 依赖核心算法完成）
- [Task 2.3] depends on [Task 2.1], [Task 2.2] （Service 依赖配置和 API）
- [Task 2.4] depends on [Task 2.3] （TaskManager 依赖 Service 就绪）
- [Task 3.1] depends on [Task 2.2] （插件骨架依赖后端 API 存在）
- [Task 3.2] depends on [Task 3.1] （Activity 依赖骨架就绪）
- [Task 4.1] depends on [Task 3.2], [Task 2.2] （GoProcessPlugin 依赖插件和 API 都就绪）
- [Task 4.2] depends on [Task 4.1]
- [Task 4.3] depends on [Task 4.2]
- [Task 4.4] depends on [Task 4.2]
- [Task 5.1] depends on [Task 3.1]
- [Task 5.3] depends on [Task 1.1] （Go 测试仅依赖算法层）
