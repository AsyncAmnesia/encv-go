# Tasks

## Phase 1: 插件模块骨架（Android Library）

- [ ] **Task 1.1**: 创建 `plugin-alist-decrypt` Android Library 模块
  - [ ] 1.1.1 创建 `app/encv-mobile/plugin-alist-decrypt/` 目录及 `build.gradle.kts`
  - [ ] 1.1.2 配置 aar2apk 插件（复用 plugin-mpv-player 的配置模板）
  - [ ] 1.1.3 配置 `compileOnly(libs.combolite.core)` + Kotlin stdlib/reflect（遵循 compileOnly 共享依赖模式）
  - [ ] 1.1.4 创建 `AndroidManifest.xml`，声明 AlistDecryptActivity（exported=false）+ AlistDecryptService
  - [ ] 1.1.5 创建 `AlistDecryptPluginEntry.kt` 实现 `IPluginEntryClass`

- [ ] **Task 1.2**: 实现算法隔离骨架
  - [ ] 1.2.1 创建 `Cipher.kt` 接口（setPosition/encrypt/decrypt/algorithm/blockSize）
  - [ ] 1.2.2 创建 `CipherRegistry.kt` 单例（init 仅注册 aesctr + register 扩展点 + ErrExtensionRequired 错误类）
  - [ ] 1.2.3 编写隔离边界测试：确认 RC4MD5/ChaCha20 未注册且调用返回 ErrExtensionRequired

## Phase 2: 核心算法实现

- [ ] **Task 2.1**: AES-128-CTR 密码器
  - [ ] 2.1.1 创建 `AesCtrCipher.kt`：PBKDF2 密钥派生链（passwdOutward → MD5 key + MD5 iv）
  - [ ] 2.1.2 创建 `CounterIncrement.kt`：128-bit 大端分段进位算法（4×uint32）
  - [ ] 2.1.3 实现 `setPosition(position)` seek 方法（恢复IV→incrementIV→重建Cipher→discard offset）
  - [ ] 2.1.4 实现 encrypt/decrypt（javax.crypto.Cipher AES/CTR/NoPadding）
  - [ ] 2.1.5 在 CipherRegistry 中注册为 "aesctr"
  - [ ] 2.1.6 单元测试：使用 alist-encrypt-go 参考向量验证密钥派生 + seek 解密正确性

- [ ] **Task 2.2**: 文件名 MixBase64 加解密
  - [ ] 2.2.1 创建 `MixBase64.kt`：KSA shuffle（passwdOutward → 64字符字母表）+ Encode/Decode
  - [ ] 2.2.2 创建 `CRC6.kt`：6-bit CRC 校验（多项式 x^6+x+1, 反射模式）
  - [ ] 2.2.3 实现 EncodeName / DecodeName / ConvertShowName / ConvertRealName
  - [ ] 2.2.4 单元测试：中文文件名往返、特殊字符、长文件名

- [ ] **Task 2.3**: V2 内容头检测
  - [ ] 2.3.1 创建 `ContentHeader.kt`：AECTR2 magic 检测 + NonceField/PlainSize 解析
  - [ ] 2.3.2 实现 AutoDetectV2（前缀 peek → 分支 V1/V2 路径）
  - [ ] 2.3.3 单元测试：V1 裸流和 V2 带头格式均能正确识别

- [ ] **Task 2.4**: 流式解密包装器
  - [ ] 2.4.1 创建 `DecryptInputStream.kt`：InputStream 包装器（支持 skip/seek + 自动 V1/V2 分流）
  - [ ] 2.4.2 单元测试：读取完整文件 + 从中间位置 seek 后继续读取

## Phase 3: 插件功能层

- [ ] **Task 3.1**: AlistDecryptActivity（插件 UI 入口）
  - [ ] 3.1.1 从 Intent extras 读取 action（decrypt/encrypt/stream/decode-filename）+ filePath + password
  - [ ] 3.1.2 decrypt action：启动 AlistDecryptService 执行解密任务，显示进度
  - [ ] 3.1.3 encrypt action：启动 AlistDecryptService 执行加密任务，显示进度
  - [ ] 3.1.4 stream action：启动 LocalStreamServer → 通过 setResult 返回 localhost URL 给宿主
  - [ ] 3.1.5 decode-filename action：同步解码文件名 → setResult 返回给宿主
  - [ ] 3.1.6 遵循 EncvHostActivity 透明主题防御规范（L1-L4 四层超时机制）

- [ ] **Task 3.2**: AlistDecryptService（后台任务执行）
  - [ ] 3.2.1 继承 IntentService，处理 decrypt/encrypt action
  - [ ] 3.2.2 解密流程：打开文件 → AutoDetectV2 → AesCtrCipher.DecryptReader → 写入目标路径
  - [ ] 3.2.3 加密流程：打开原始文件 → AesCtrCipher.EncryptWriter → 写入目标路径+suffix（可选 V2 头）
  - [ ] 3.2.4 进度报告：通过 LocalBroadcast 发送进度百分比到 Activity
  - [ ] 3.2.5 错误处理：密码错误（特殊错误码）、数据损坏、磁盘空间不足等

- [ ] **Task 3.3**: LocalStreamServer（本地 HTTP 代理）
  - [ ] 3.3.1 基于NanoHTTPD或自实现简易 HTTP server（单连接，随机端口）
  - [ ] 3.3.2 GET /stream 端点：解析 path+password 参数 → DecryptInputStream → HTTP Range 支持
  - [ ] 3.3.3 返回 206 Partial Content + 正确 Content-Type + Content-Length
  - [ ] 3.3.4 生命周期绑定到 Activity（onDestroy 时停止 server）

## Phase 4: 宿主端集成

- [ ] **Task 4.1**: GoProcessPlugin 新增方法
  - [ ] 4.1.1 新增 `@PluginMethod decryptAlistFile(call)` — 构建代理 Intent 启动解密
  - [ ] 4.1.2 新增 `@PluginMethod encryptAlistFile(call)` — 构建代理 Intent 启动加密
  - [ ] 4.1.3 新增 `@PluginMethod streamAlistFile(call)` — 构建 Intent 启动流式预览，返回 localhost URL
  - [ ] 4.1.4 新增 `@PluginMethod decodeAlistFilename(call)` — 同步调用插件解码文件名
  - [ ] 4.1.5 所有方法先检查 `EncvComboLiteHost.isPluginAvailable("com.encvgo.plugin.alistdecrypt")`，未安装时返回友好提示

- [ ] **Task 4.2**: PlayerEntry 新增路由
  - [ ] 4.2.1 新增 `buildAlistDecryptIntent()` 方法（参照 buildMpvIntent 模式）
  - [ ] 4.2.2 定义 REQUEST_CODE_ALIST_DECRYPT 常量
  - [ ] 4.2.3 onActivityResult 处理解密/加密结果回调

- [ ] **Task 4.3**: 前端 API 层（encv.ts）
  - [ ] 4.3.1 新增 `decryptAlistFile()`, `encryptAlistFile()`, `streamAlistFile()`, `decodeAlistFilename()` 函数
  - [ ] 4.3.2 在 GoProcess 插件定义中注册新方法

- [ ] **Task 4.4**: Files.vue 文件识别与操作
  - [ ] 4.4.1 检测文件扩展名匹配 suffix 时显示加密标记 + 调用 decodeAlistFilename 显示真实名称
  - [ ] 4.4.2 长按菜单增加「解密」和「流式预览」选项（条件显示：插件已安装时）
  - [ ] 4.4.3 流式预览返回的 localhost URL 传给 MPV/ArtPlayer 播放

- [ ] **Task 4.5**: ExtensionsPage.vue + Tasks.vue + i18n
  - [ ] 4.5.1 ExtensionsPage 新增 alist-decrypt 扩展卡片（id/name/description/sizeDisplay）
  - [ ] 4.5.2 COMBO_LITE_ID_MAP 增加 `'alist-decrypt' → 'com.encvgo.plugin.alistdecrypt'`
  - [ ] 4.5.3 Tasks.vue 支持 type=alist-decrypt/alist-encrypt 的状态展示
  - [ ] 4.5.4 i18n 新增相关翻译 key（中英文）

## Phase 5: CI 构建集成

- [ ] **Task 5.1**: settings.gradle.kts 包含新模块
- [ ] **Task 5.2**: CI workflow 新增 plugin-alist-decrypt 的 aar2apk 构建步骤
- [ ] **Task 5.3**: 验证插件 APK 可正常安装/加载/卸载

## Phase 6: TODO（后续迭代，不在 MVP）

- [ ] **[TODO] Task 6.1**: OpenList 代理集成 — 接入 internal/openlist/ 代理链
- [ ] **[TODO] Task 6.2**: ENCV Plugin 注册 — 注册到 ENCV v2 plugins.Registry
- [ ] **[TODO] Task 6.3**: 桌面端 UI — openlist 桌面客户端适配
- [ ] **[TODO] Task 6.4**: RC4MD5 / ChaCha20 **扩展包** — 独立包实现 Cipher 接口，通过 Register() 注册。**禁止引入主包**

# Task Dependencies
- [Task 1.2] depends on [Task 1.1] （骨架需要模块先创建）
- [Task 2.1] depends on [Task 1.2] （AES-CTR 需要 Cipher 接口和 Registry）
- [Task 2.2] depends on [Task 2.1] （MixBase64 复用 passwdOutward 派生逻辑）
- [Task 2.3] depends on [Task 2.1] （V2 头解析依赖 AesCtrCipher 初始化）
- [Task 2.4] depends on [Task 2.1], [Task 2.3] （DecryptInputStream 依赖 cipher 和头检测）
- [Task 3.1] depends on [Task 2.1], [Task 2.2], [Task 2.3], [Task 2.4] （Activity 依赖全部核心算法）
- [Task 3.2] depends on [Task 2.4] （Service 依赖 DecryptInputStream）
- [Task 3.3] depends on [Task 2.4] （LocalStreamServer 依赖 DecryptInputStream）
- [Task 4.1] depends on [Task 3.1] （GoProcessPlugin 依赖 Activity 就绪）
- [Task 4.2] depends on [Task 4.1] （PlayerEntry 依赖 GoProcessPlugin 方法）
- [Task 4.3] depends on [Task 4.1] （前端 API 依赖原生方法注册）
- [Task 4.4] depends on [Task 4.3] （Files.vue 依赖前端 API）
- [Task 4.5] depends on [Task 4.3] （UI 组件依赖前端 API）
- [Task 5.1] depends on [Task 1.1] （CI 依赖模块存在）
