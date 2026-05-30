# Checklist

## Phase 1: Go 后端算法基础设施 ✅ 已完成

- [x] Cipher 接口定义完整（5 个方法）
- [x] CipherRegistry 使用 RWMutex 并发安全
- [x] Registry init() **仅注册 aesctr**
- [x] 查询 rc4md5/chacha20 → ErrExtensionRequired
- [x] `go vet ./internal/alistencrypt/...` 无 RC4/ChaCha20 import
- [x] AES-128-CTR 密钥派生链与参考实现一致
- [x] incrementIV 与参考实现完全一致
- [x] SetPosition seek 后数据解密正确
- [x] MixBase64 Encode/Decode 往返无损（中文/特殊字符）
- [x] CRC6 校验位计算正确
- [x] V2 AECTR2 magic 检测 + AutoDetectV2 分支正确
- [x] DecryptReader 可流式读取 + seek
- [x] `go test ./internal/alistencrypt/...` 52/52 PASS

## Phase 2: ENCV Plugin 实现 ✅ 已完成

- [x] AlistEncryptPlugin struct 存在，Name()="alist_encrypt"
- [x] GetContainerExtension() 返回 settings.Suffix（默认 ".bin"）
- [x] Initialize() 后缀双重校验（冲突+格式）
- [x] Encrypt() 端到端：原始→AES-CTR→输出+suffix
- [x] CanDecrypt() 扩展名匹配 + AECTR2 magic 增强
- [x] Decrypt() 端到端：加密文件→AutoDetectV2→AES-CTR→明文
- [x] 运行时双重校验（类型+容器碰撞）
- [x] Stream() HTTP Range → 206 Partial Content
- [x] Plugin 其余接口方法有合理默认实现
- [x] 注册到 Plugins 列表，编译通过

## Phase 3: API 层与前端基础集成 ✅ 已完成

- [x] GET /api/alist-encrypt/stream endpoint 存在
- [x] GET /api/alist-encrypt/decode-filename endpoint 存在
- [x] encv.ts streamAlistFile() / decodeAlistFilename() 存在
- [x] Files.vue .bin 文件显示 AE 徽章 + 真实文件名
- [x] Files.vue 长按菜单出现「解密」和「流式预览」
- [x] i18n 中英文翻译 key 已定义

## Phase 4: 移动端 UI 完善 🆕 本次新增

### useAlistEncrypt Composable
- [ ] composables/useAlistEncrypt.ts 文件存在
- [ ] sessionPasswords ref 存在（path→password 映射）
- [ ] decodedNames ref 存在（path→plainName 映射）
- [ ] promptPassword(file) 弹出 IonAlert 密码输入框
- [ ] IonAlert 包含 password 类型 input + 「记住会话」toggle
- [ ] 用户确认后密码存入 sessionPasswords；取消返回 null
- [ ] decodeFilename(file) 调用 API 并缓存结果到 decodedNames
- [ ] getStreamUrl(file) 使用缓存密码构造 stream URL
- [ ] isAlistEncrypted(file) 正确检测 .bin 后缀（大小写不敏感）
- [ ] isAlistEncrypted 对 .sccgv/.encv 返回 false（ENCV 容器排除）
- [ ] 密码不持久化到 localStorage（仅内存）

### Files.vue 交互完善
- [ ] handleAlistStreamPreview 先调 promptPassword 再执行
- [ ] handleAlistDecrypt 先调 promptPassword 再执行
- [ ] 流 URL 构建中显示 loading indicator
- [ ] 流加载失败显示错误信息 + 重试按钮
- [ ] 解密提交时显示 loading toast
- [ ] 密码错误显示特殊样式（红色背景 + lock 图标）
- [ ] 数据损坏/格式无效显示通用 task-error 样式
- [ ] ErrExtensionRequired 显示 task-warning 样式
- [ ] 内联逻辑已迁移至 composable

### 设置页集成
- [ ] 设置页存在「Alist-Encrypt 配置」区域（条件渲染）
- [ ] enabled ion-toggle 控件存在
- [ ] suffix ion-input 存在
- [ ] 输入 .sccgv/.encv 时显示红色警告提示
- [ ] suffix 不以 "." 开头时自动补全
- [ ] default_password ion-input(type=password) 存在
- [ ] enc_type ion-select 存在（MVP 仅 aesctr）

### ExtensionsPage 集成
- [ ] alist-decrypt 扩展卡片显示正确
- [ ] COMBO_LITE_ID_MAP 包含 'alist-decrypt' → 'com.encvgo.plugin.alistdecrypt'
- [ ] 安装/启用/禁用状态展示正确

## Phase 5: Mock 测试 🆕

### Go Plugin 层测试
- [ ] plugin_test.go 文件存在且编译通过
- [ ] TestPluginInitialization: 成功场景通过
- [ ] TestPluginInitialization: .sccgv 冲突回退 .bin 通过
- [ ] TestPluginInitialization: .encv 冲突回退 .bin 通过
- [ ] TestPluginInitialization: 无点 suffix 自动补全通过
- [ ] TestPluginInitialize: >16 长度 suffix 回退通过
- [ ] TestPluginInitialize: 非 aesctr enc_type Warn 通过
- [ ] TestCanDecrypt: .bin 匹配返回 true
- [ ] TestCanDecrypt: .mp4 不匹配返回 false
- [ ] TestCanDecrypt: AECTR2 magic 增强置信度
- [ ] TestCanDecrypt: ENCV 容器碰撞拒绝
- [ ] TestEncryptDecryptRoundtrip: V1 裸流往返一致
- [ ] TestEncryptDecryptRoundtrip: V2 带 AECTR2 头往返一致
- [ ] TestEncryptWithV2Header: 头 32 bytes 结构正确
- [ ] TestEncryptWithV2Header: 解密自动跳过头
- [ ] TestStreamRange: Range=bytes=0- → 206
- [ ] TestStreamRange: Range=bytes=100-200 → 正确段
- [ ] TestStreamRange: Range 超出范围 → 416
- [ ] TestStreamRange: 无 Range → 200

### Go API Handler 测试
- [ ] mobile_api_alistencrypt_test.go 编译通过
- [ ] handleAlistDecodeFilenameGin: 正常解码返回 plain_name
- [ ] handleAlistDecodeFilenameGin: 空/缺失参数返回 400
- [ ] handleAlistDecodeFilenameGin: 特殊字符编码正确
- [ ] handleAlistEncryptStreamGin: 正常流返回 200+解密数据
- [ ] handleAlistEncryptStreamGin: Range → 206
- [ ] handleAlistEncryptStreamGin: 文件不存在 → 404
- [ ] handleAlistEncryptStreamGin: 密码错误 → 400

### 前端 Vitest 测试
- [ ] __tests__/ 目录存在
- [ ] encv.alistencrypt.spec.ts: getAlistEncryptStreamUrl URL 构造正确
- [ ] encv.alistencrypt.spec.ts: DEV 模式 base URL 正确
- [ ] encv.alistencrypt.spec.ts: decodeAlistFilename 成功路径
- [ ] encv.alistencrypt.spec.ts: decodeAlistFilename 失败路径
- [ ] encv.alistencrypt.spec.ts: 网络错误降级
- [ ] Files.alistencrypt.spec.ts: .bin → true
- [ ] Files.alistencrypt.spec.ts: .BIN → true（大小写）
- [ ] Files.alistencrypt.spec.ts: .sccgv → false（ENCV 排除）
- [ ] Files.alistencrypt.spec.ts: .mp4 → false
- [ ] Files.alistencrypt.spec.ts: '' (无扩展名) → false
- [ ] useAlistEncrypt.spec.ts: promptPassword 缓存成功
- [ ] useAlistEncrypt.spec.ts: promptPassword 取消无缓存
- [ ] useAlistEncrypt.spec.ts: 二次调用跳过弹窗用缓存
- [ ] useAlistEncrypt.spec.ts: decodeFilename 缓存结果
- [ ] useAlistEncrypt.spec.ts: getStreamUrl 用缓存密码
- [ ] npm run test:run 全部通过

## Phase 6: CI 与覆盖率 🆕

- [ ] CI workflow 包含 go test plugin 层 -cover
- [ ] CI workflow 包含 go test handler 层 -cover
- [ ] CI workflow 包含 npm run test:run --coverage
- [ ] CI workflow 包含隔离性 grep 验证
- [ ] internal/alistencrypt 覆盖率 ≥ 90%
- [ ] internal/v2/plugins/alistencrypt 覆盖率 ≥ 80%
- [ ] mobile_api handler 覆盖率 ≥ 80%
- [ ] 前端 alistencrypt 函数覆盖率 ≥ 85%

## 隔离性验证
- [x] `go build ./internal/alistencrypt/` 成功
- [x] ComboLite 插件目录不存在（本方案不需要）
