# Tasks

## Phase 1: Go 后端 — 算法基础设施包 `internal/alistencrypt/` ✅ 已完成

- [x] **Task 1.0**: 定义 Cipher 扩展接口与注册表（算法隔离骨架）
- [x] **Task 1.1**: 实现 AES-128-CTR 核心密码器
- [x] **Task 1.2**: 实现文件名 MixBase64 加解密
- [x] **Task 1.3**: 实现 V2 内容头检测与流式包装器

## Phase 2: Go 后端 — ENCV Plugin 实现 `internal/v2/plugins/alistencrypt/` ✅ 已完成

- [x] **Task 2.1**: Plugin 骨架与配置（含后缀双重校验）
- [x] **Task 2.2**: 实现加密流程
- [x] **Task 2.3**: 实现解密流程（含运行时双重校验）
- [x] **Task 2.4**: 实现流式预览
- [x] **Task 2.5**: 实现 Plugin 接口其余方法（默认实现）
- [x] **Task 2.6**: 注册到 Plugins 列表

## Phase 3: API 层与前端基础集成 ✅ 已完成（骨架）

- [x] **Task 3.1**: 新增 HTTP endpoint（stream + decode-filename）
- [x] **Task 3.2**: 前端基础适配（encv.ts API + Files.vue 基本识别/菜单 + i18n）

## Phase 4: 移动端 UI 完善 🆕 本次新增

- [ ] **Task 4.1**: 创建 `useAlistEncrypt` Composable
  - [ ] 4.1.1 创建 `composables/useAlistEncrypt.ts`：sessionPasswords 缓存、decodedNames 缓存
  - [ ] 4.1.2 实现 `promptPassword(file)`：弹出 IonAlert 密码输入框（password 类型 + 记住会话开关）
  - [ ] 4.1.3 实现 `decodeFilename(file)`：调用 API 解码文件名并缓存结果
  - [ ] 4.1.4 实现 `getStreamUrl(file)`：使用缓存密码构造 stream URL
  - [ ] 4.1.5 实现 `isAlistEncrypted(file)` / `cachedPassword(file)` 纯逻辑函数
  - [ ] 4.1.6 密码仅内存缓存，不持久化到 localStorage

- [ ] **Task 4.2**: 完善 Files.vue 交互流程
  - [ ] 4.2.1 重构 `handleAlistStreamPreview()` 和 `handleAlistDecrypt()`：先调用 `promptPassword()` 获取密码再执行操作
  - [ ] 4.2.2 流式预览：URL 构建中显示 loading indicator；加载失败显示错误+重试按钮
  - [ ] 4.2.3 解密提交：显示 loading toast「正在提交任务...」；成功后跳转 Tasks.vue
  - [ ] 4.2.4 错误展示遵循 frontend-design.md 规范（密码错误特殊样式 vs 数据损坏通用样式）
  - [ ] 4.2.5 将 Files.vue 中内联的 alistencrypt 逻辑迁移到 useAlistEncrypt composable

- [ ] **Task 4.3**: 设置页集成
  - [ ] 4.3.1 在设置页面增加「Alist-Encrypt 配置」区域（条件渲染：后端 schema 包含 alist_encrypt 字段时显示）
  - [ ] 4.3.2 enabled: ion-toggle 控件
  - [ ] 4.3.3 suffix: ion-input + 实时校验（输入 .sccgv/.encv 时红色警告提示）
  - [ ] 4.3.4 default_password: ion-input(type=password)
  - [ ] 4.3.5 enc_type: ion-select（MVP 仅 aesctr 可选）

- [ ] **Task 4.4**: ExtensionsPage.vue 集成
  - [ ] 4.4.1 新增 alist-decrypt 扩展卡片（id/name/description/sizeDisplay）
  - [ ] 4.4.2 COMBO_LITE_ID_MAP 增加 `'alist-decrypt' → 'com.encvgo.plugin.alistdecrypt'`
  - [ ] 4.4.3 安装/启用/禁用状态正确展示

## Phase 5: Mock 测试 🆕 本次新增

### Go 后端测试

- [ ] **Task 5.1**: ENCV Plugin 层单元测试
  - [ ] 5.1.1 创建 `internal/v2/plugins/alistencrypt/plugin_test.go`
  - [ ] 5.1.2 TestPluginInitialization: Initialize() 成功场景、冲突后缀回退(.sccgv/.encv)、格式修正(无点/>16)、enc_type 无效 Warn
  - [ ] 5.1.3 TestCanDecrypt: .bin 匹配 true / .mp4 匹配 false / AECTR2 magic 增强 / ENCV 容器碰撞拒绝
  - [ ] 5.1.4 TestEncryptDecryptRoundtrip: 内存数据 Encrypt→Decrypt 往返一致性（V1 裸流 + V2 带 AECTR2 头两种模式）
  - [ ] 5.1.5 TestEncryptWithV2Header: V2 头 32 bytes 正确写入(magic+version+nonce+plainSize)；解密时自动跳过 32 bytes
  - [ ] 5.1.6 TestStreamRange: ServeStream() Range=bytes=0- 返回 206 / Range=bytes=100-200 返回正确段 / Range 超出范围返回 416 / 无 Range 返回 200
  - [ ] 5.1.7 使用 bytes.Buffer 替代真实文件系统，固定 test password="test123"

- [ ] **Task 5.2**: API Handler 测试
  - [ ] 5.2.1 创建 `internal/server/mobile_api_alistencrypt_test.go`
  - [ ] 5.2.2 handleAlistDecodeFilenameGin: 正常解码返回 plain_name / 空参数返回 400 / 特殊字符编码正确处理
  - [ ] 5.2.3 handleAlistEncryptStreamGin: 正常流返回 200+解密数据 / Range 请求返回 206 / 文件不存在返回 404 / 密码错误返回 400
  - [ ] 5.2.4 使用 httptest.NewRecorder + gin.CreateTestContext；临时目录创建测试加密文件；t.Cleanup 清理

### 前端 Vitest 测试

- [ ] **Task 5.3**: encv.ts API 函数 mock 测试
  - [ ] 5.3.1 创建 `src/__tests__/encv.alistencrypt.spec.ts`
  - [ ] 5.3.2 mock Capacitor.Http.request：验证 getAlistEncryptStreamUrl 构造正确的 URL（path/password 编码）
  - [ ] 5.3.3 mock Capacitor.Http.request：验证 DEV 模式使用正确 base URL
  - [ ] 5.3.4 mock Capacitor.Http.request：验证 decodeAlistFilename 成功返回 plain_name + success:true
  - [ ] 5.3.5 mock Capacitor.Http.request：验证 decodeAlistFilename 失败返回空 plain_name + success:false
  - [ ] 5.3.6 mock Capacitor.Http.request：验证网络超时/断网的优雅降级

- [ ] **Task 5.4**: 文件检测逻辑单元测试
  - [ ] 5.4.1 创建 `src/__tests__/Files.alistencrypt.spec.ts`
  - [ ] 5.4.2 isAlistEncrypted('.bin') === true
  - [ ] 5.4.3 isAlistEncrypted('.BIN') === true（大小写不敏感）
  - [ ] 5.4.4 isAlistEncrypted('.sccgv') === false（ENCV 容器，不是 alist-encrypt）
  - [ ] 5.4.5 isAlistEncrypted('.mp4') === false
  - [ ] 5.4.6 isAlistEncrypted('') === false（无扩展名）

- [ ] **Task 5.5**: useAlistEncrypt composable 测试
  - [ ] 5.5.1 创建 `src/__tests__/useAlistEncrypt.spec.ts`
  - [ ] 5.5.2 promptPassword 成功→密码被缓存到 sessionPasswords
  - [ ] 5.5.3 promptPassword 用户取消→返回 null，无缓存
  - [ ] 5.5.4 第二次调用 promptPassword（同一文件）且已缓存→跳过弹窗直接返回缓存值
  - [ ] 5.5.5 decodeFilename 成功→结果缓存到 decodedNames
  - [ ] 5.5.6 getStreamUrl 使用缓存密码构造 URL
  - [ ] 5.5.7 mock IonAlert controller（需要 @ionic/vue 的 mock 或 shallow mount）

## Phase 6: CI 与覆盖率 🆕

- [ ] **Task 6.1**: CI 集成
  - [ ] 6.1.1 CI workflow 加入 `go test ./internal/v2/plugins/alistencrypt/... -cover`
  - [ ] 6.1.2 CI workflow 加入 `go test ./internal/server/... -run TestAlist -cover`
  - [ ] 6.1.3 CI workflow 加入 `cd app/encv-mobile && npm run test:run -- --coverage`
  - [ ] 6.1.4 CI workflow 加入隔离性 grep 验证

- [ ] **Task 6.2**: 覆盖率达标验证
  - [ ] 6.2.1 internal/alistencrypt 覆盖率 ≥ 90%
  - [ ] 6.2.2 internal/v2/plugins/alistencrypt 覆盖率 ≥ 80%
  - [ ] 6.2.3 mobile_api handler 覆盖率 ≥ 80%
  - [ ] 6.2.4 前端 alistencrypt 相关函数覆盖率 ≥ 85%

## Phase 7: TODO（后续迭代，不在本次范围）

- [ ] **[TODO] Task 7.1**: OpenList 代理集成 — 接入 internal/openlist/ 代理链
- [ ] **[TODO] Task 7.2**: 桌面端 UI — openlist 桌面客户端适配
- [ ] **[TODO] Task 7.3**: RC4MD5 / ChaCha20 **扩展包** — 独立包实现 Cipher 接口。**禁止引入 internal/alistencrypt/**

# Task Dependencies
- [Task 4.1] depends on [Task 3.2] （Composable 依赖基础 API 就绪）
- [Task 4.2] depends on [Task 4.1] （Files.vue 依赖 Composable）
- [Task 4.3] depends on [Task 4.1] （设置页依赖 Composable 中的配置读取逻辑）
- [Task 4.4] depends on [Task 3.2]
- [Task 5.1] depends on [Task 2.6] （Plugin 测试依赖实现完成）
- [Task 5.2] depends on [Task 3.1] （Handler 测试依赖 API 就绪）
- [Task 5.3] depends on [Task 3.2] （前端 API 测试依赖函数就绪）
- [Task 5.4] depends on [Task 3.2]
- [Task 5.5] depends on [Task 4.1] （Composable 测试依赖其实现）
- [Task 6.1] depends on [Task 5.1], [Task 5.2], [Task 5.3], [Task 5.4], [Task 5.5]
- [Task 6.2] depends on [Task 6.1]
