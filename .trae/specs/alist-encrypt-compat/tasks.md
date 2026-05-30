# Tasks

## Phase 1: Go 后端 — 算法基础设施包 ✅ 已完成

- [x] **Task 1.0**: 定义 Cipher 接口与注册表
- [x] **Task 1.1**: 实现 AES-128-CTR 核心密码器
- [x] **Task 1.2**: 实现文件名 MixBase64 加解密
- [x] **Task 1.3**: 实现 V2 内容头检测与流式包装器

## Phase 2: Go 后端 — ENCV Plugin 实现 ✅ 已完成

- [x] **Task 2.1-2.6**: Plugin 骨架/加密/解密/流式预览/接口实现/注册

## Phase 3: API 层与前端基础骨架 ✅ 已完成

- [x] **Task 3.1-3.2**: HTTP endpoint + encv.ts API + Files.vue 基本识别/i18n

## Phase 4: FileFeature 架构（UI 隔离骨架）🆕

- [ ] **Task 4.1**: 定义 FileFeature 接口类型
  - [ ] 4.1.1 创建 `src/types/file-feature.ts`：FileAction / FileBadge / FileSubtitle / FileFeature 接口定义
  - [ ] 4.1.2 每个接口字段附带 JSDoc 注释说明契约语义
  - [ ] 4.1.3 导出所有类型供 features/ 和 Files.vue 使用

- [ ] **Task 4.2**: 实现 useFileFeatures 注册表 composable
  - [ ] 4.2.1 创建 `src/composables/useFileFeatures.ts`
  - [ ] 4.2.2 实现 `registerFileFeature(feature)` / `unregisterFileFeature(id)` （Map 注册表 + 重复检测 warn）
  - [ ] 4.2.3 实现 `useFileFeatures()` 返回 `{ allFeatures, getBadges, getSubtitles, getAllActions }`
  - [ ] 4.2.4 `getBadges(file)`：遍历所有 Feature，并行调用 `getBadge?()` 过滤 null，返回 FileBadge[]
  - [ ] 4.2.5 `getSubtitles(file)`：同上模式，返回 FileSubtitle[]
  - [ ] 4.2.6 `getAllActions(file)`：同上模式，返回 FileAction[]（保持注册顺序）
  - [ ] 4.2.7 所有查询方法支持同步和异步返回值（Promise 兼容）

## Phase 5: AlistEncrypt Feature Module 🆕

- [ ] **Task 5.1**: 创建 features/alist-encrypt/ 目录结构
  - [ ] 5.1.1 创建 `features/alist-encrypt/index.ts` — 导出 `createAlistEncryptFeature(config?)`
  - [ ] 5.1.2 创建 `features/alist-encrypt/useAlistEncrypt.ts` — 密码管理（sessionPasswords map）+ 文件名解码缓存 + API 调用封装
  - [ ] 5.1.3 创建 `features/alist-encrypt/actions.ts` — streamPreviewAction / decryptAction 的 FileAction 定义
  - [ ] 5.1.4 创建 `features/alist-encrypt/badge.ts` — AE 徽章生成逻辑
  - [ ] 5.1.5 创建 `features/alist-encrypt/subtitle.ts` — 异步解码文件名逻辑（带缓存+防抖）
  - [ ] 5.1.6 创建 `features/alist-encrypt/password-dialog.ts` — IonAlert 密码输入弹窗（password 类型 + remember toggle）

- [ ] **Task 5.2**: 实现完整的 AlistEncrypt FileFeature
  - [ ] 5.2.1 `id = 'alist-encrypt'`
  - [ ] 5.2.2 `isActive(file)`: 扩展名匹配 config suffix（默认 .bin），大小写不敏感；排除 .sccgv/.encv
  - [ ] 5.2.3 `getBadge(file)`: 返回 `{text:'AE', color:'var(--ion-color-danger)'}`
  - [ ] 5.2.4 `getSubtitle(file)`: 调用 decode-filename API → 缓存 → 返回 subtitle；失败返回 null
  - [ ] 5.2.5 `getFileActions(file)`: isActive 时返回 [streamPreview, decrypt]，否则返回 []
  - [ ] 5.2.6 action handler 中调用 promptPassword → 获取密码后执行操作
  - [ ] 5.2.7 `onActivate`: 从后端配置读取 suffix/enc_type/defaultPassword
  - [ ] 5.2.8 `onDeactivate`: 清空 sessionPasswords 和 decodedNames 缓存

- [ ] **Task 5.3**: 重构 Files.vue 使用 FileFeature Registry
  - [ ] 5.3.1 移除所有内联的 isAlistEncrypted / handleAlistDecrypt / handleAlistStreamPreview / decodedNames 等代码
  - [ ] 5.3.2 import { useFileFeatures } 替代直接 import
  - [ ] 5.3.3 文件列表渲染中：使用 v-for 遍历 fileBadges[file.path] 渲染徽章
  - [ ] 5.3.4 文件列表渲染中：使用 v-for 遍历 fileSubtitles[file.path] 渲染副标题
  - [ ] 5.3.5 handleLongPress 中：await getAllActions(file) 追加到 buttons 数组
  - [ ] 5.3.6 onMounted / 文件加载时：批量调用 getSubtitles 为可见文件填充副标题

- [ ] **Task 5.4**: 应用启动时注册 FileFeature
  - [ ] 5.4.1 在 App.vue 或 main.ts 中 import 并 registerFileFeature(createAlistEncryptFeature())
  - [ ] 5.4.2 条件注册：仅在 alist_encrypt.enabled=true 时注册（从配置读取）

## Phase 6: 设置页与 ExtensionsPage 集成 🆕

- [ ] **Task 6.1**: 设置页 alist_encrypt 配置区域
  - [ ] 6.1.1 动态读取 schema 中的 alist_encrypt 字段渲染配置 UI
  - [ ] 6.1.2 suffix 输入框 + 冲突校验提示（.sccgv/.encv 红色警告）
  - [ ] 6.1.3 default_password (type=password)
  - [ ] 6.1.4 enc_type ion-select
  - [ ] 6.1.5 enabled 切换时触发 register/unregister FileFeature

- [ ] **Task 6.2**: ExtensionsPage.vue 集成
  - [ ] 6.2.1 新增 alist-decrypt 卡片
  - [ ] 6.2.2 COMBO_LITE_ID_MAP 增加 mapping

## Phase 7: Mock 测试 🆕

### Go 后端测试

- [ ] **Task 7.1**: ENCV Plugin 层单元测试 (`plugin_test.go`)
  - [ ] 7.1.1 TestPluginInitialization（成功/冲突回退/格式修正/enc_type 无效）
  - [ ] 7.1.2 TestCanDecrypt（匹配/不匹配/AECTR2 增强/ENCV 碰撞拒绝）
  - [ ] 7.1.3 TestEncryptDecryptRoundtrip（V1/V2 往返一致性）
  - [ ] 7.1.4 TestEncryptWithV2Header（头结构正确/解密自动跳过）
  - [ ] 7.1.5 TestStreamRange（206/200/416/边界 offset）

- [ ] **Task 7.2**: API Handler 测试 (`mobile_api_alistencrypt_test.go`)
  - [ ] 7.2.1 handleAlistDecodeFilenameGin 正常/异常/特殊字符
  - [ ] 7.2.2 handleAlistEncryptStreamGin 正常/Range/404/400

### 前端 Vitest 测试

- [ ] **Task 7.3**: FileFeature 架构测试 (`file-feature.registry.spec.ts`)
  - [ ] 7.3.1 register/unregister 生命周期
  - [ ] 7.3.2 重复注册保护
  - [ ] 7.3.3 getBadges/getSubtitles/getAllActions 聚合多个 Feature
  - [ ] 7.3.4 空 registry 行为

- [ ] **Task 7.4**: AlistEncrypt Feature 模块测试 (`features.alist-encrypt.spec.ts`)
  - [ ] 7.4.1 isActive 检测逻辑（.bin/.mp4/.sccgv/大小写）
  - [ ] 7.4.2 getBadge 返回值
  - [ ] 7.4.3 getSubtitle mock API 成功/失败/缓存
  - [ ] 7.4.4 getFileActions 返回正确的 action 数组
  - [ ] 7.4.5 promptPassword 缓存/取消/二次调用

- [ ] **Task 7.5**: API 函数 mock 测试 (`encv.alistencrypt.spec.ts`)
  - [ ] 7.5.1 getAlistEncryptStreamUrl URL 构造
  - [ ] 7.5.2 decodeAlistFilename 成功/失败路径

- [ ] **Task 7.6**: useAlistEncrypt composable 测试 (`useAlistEncrypt.spec.ts`)
  - [ ] 7.6.1 密码缓存生命周期
  - [ ] 7.6.2 文件名解码缓存
  - [ ] 7.6.3 Stream URL 构造

## Phase 8: CI 与覆盖率 🆕

- [ ] **Task 8.1**: CI workflow 更新
  - [ ] 8.1.1 Go plugin test + coverage
  - [ ] 8.1.2 Go handler test + coverage
  - [ ] 8.1.3 Frontend vitest --coverage
  - [ ] 8.1.4 隔离性 grep 验证

- [ ] **Task 8.2**: 覆盖率达标验证
  - [ ] 8.2.1 useFileFeatures ≥ 95%
  - [ ] 8.2.2 AlistEncrypt Feature ≥ 90%
  - [ ] 8.2.3 encv.ts API ≥ 90%
  - [ ] 8.2.4 Go Plugin ≥ 80%
  - [ ] 8.2.5 Go Handler ≥ 80%

## Phase 9: TODO（后续迭代）

- [ ] **[TODO] Task 9.1**: OpenList 代理集成
- [ ] **[TODO] Task 9.2**: 桌面端 UI
- [ ] **[TODO] Task 9.3**: RC4MD5 / ChaCha20 扩展包（禁止进主包）

# Task Dependencies
- [Task 4.1] depends on [Task 3.2] （类型定义依赖基础架构就绪）
- [Task 4.2] depends on [Task 4.1]
- [Task 5.1] depends on [Task 4.1], [Task 4.2] （Feature 模块依赖接口和注册表）
- [Task 5.2] depends on [Task 5.1]
- [Task 5.3] depends on [Task 4.2], [Task 5.2] （Files.vue 重构依赖注册表和 Feature 就绪）
- [Task 5.4] depends on [Task 5.2]
- [Task 6.1] depends on [Task 5.2] （设置页依赖 Feature 配置接口）
- [Task 6.2] depends on [Task 3.2]
- [Task 7.1] depends on [Task 2.6] （Go 测试依赖 Plugin 实现）
- [Task 7.2] depends on [Task 3.1]
- [Task 7.3] depends on [Task 4.2]
- [Task 7.4] depends on [Task 5.2]
- [Task 7.5] depends on [Task 3.2]
- [Task 7.6] depends on [Task 5.1]
- [Task 8.1] depends on [Task 7.1]..[Task 7.6]
- [Task 8.2] depends on [Task 8.1]
