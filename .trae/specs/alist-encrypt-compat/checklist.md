# Checklist

## Phase 1-3: 已完成 ✅
（见 tasks.md Phase 1-3 全部 [x]）

## Phase 4: FileFeature 架构（UI 隔离骨架）🆕

### 类型定义
- [ ] `src/types/file-feature.ts` 存在
- [ ] FileAction 接口定义完整（id/text/icon/color/visible/handler）
- [ ] FileBadge 接口定义完整（text/color/icon）
- [ ] FileSubtitle 接口定义完整（text/color）
- [ ] FileFeature 接口定义完整（id/isActive/getBadge/getSubtitle/getFileActions/onActivate/onDeactivate）
- [ ] 所有接口字段有 JSDoc 注释

### useFileFeatures 注册表
- [ ] `src/composables/useFileFeatures.ts` 存在
- [ ] registerFileFeature() 正确写入 Map，重复注册 console.warn 不覆盖
- [ ] unregisterFileFeature() 正确删除 + 调用 onDeactivate
- [ ] useFileFeatures() 返回 allFeatures computed ref
- [ ] getBadges(file) 并行调用所有 Feature.getBadge?() → 过滤 null → 返回数组
- [ ] getSubtitles(file) 同上模式
- [ ] getAllActions(file) 同上模式，保持注册顺序
- [ ] 查询方法支持同步和异步返回值
- [ ] **🆕** registry 并发安全（RWMutex 或等效机制）
- [ ] **🆕** getBadge 强制同步（无 await），耗时 < 0.05ms/文件

## Phase 5: AlistEncrypt Feature Module 🆕

### 目录结构
- [ ] `features/alist-encrypt/` 目录存在
- [ ] index.ts 导出 createAlistEncryptFeature()
- [ ] useAlistEncrypt.ts 存在（密码管理/解码缓存/API 封装）
- [ ] actions.ts 存在（streamPreview + decrypt action 定义）
- [ ] badge.ts 存在
- [ ] subtitle.ts 存在（异步解码+缓存）
- [ ] password-dialog.ts 存在（IonAlert 弹窗）

### FileFeature 实现
- [ ] id === 'alist-encrypt'
- [ ] isActive('.bin') === true
- [ ] isActive('.BIN') === true（大小写不敏感）
- [ ] isActive('.mp4') === false
- [ ] isActive('.sccgv') === false（ENCV 容器排除）
- [ ] isActive('.encv') === false（ENCV 容器排除）
- [ ] getBadge 返回 {text:'AE', color:'var(--ion-color-danger)'}
- [ ] getSubtitle 调用 API 成功时返回解码名称
- [ ] getSubtitle API 失败时返回 null
- [ ] getSubtitle 结果被缓存（二次调用不重复请求）
- [ ] getFileActions(.bin 文件) 返回 2 个 action
- [ ] getFileActions(.mp4 文件) 返回空数组
- [ ] action handler 中先调 promptPassword 再执行操作
- [ ] onActivate 读取后端配置
- [ ] onDeactivate 清空缓存
- [ ] **🆕** subtitle.ts 实现请求合并（300ms 窗口批量收集 → 单次 API）
- [ ] **🆕** sessionPasswords LRU 上限 500 条，超出淘汰最旧
- [ ] **🆕** decodedNames LRU 上限 500 条，超出淘汰最旧

### Files.vue 重构
- [ ] Files.vue 中无 isAlistEncrypted / handleAlistDecrypt / handleAlistStreamPreview / decodedNames 内联代码
- [ ] Files.vue 仅 import { useFileFeatures }
- [ ] 文件列表中徽章通过 v-for fileBadges 渲染
- [ ] 文件列表中副标题通过 v-for fileSubtitles 渲染
- [ ] handleLongPress 通过 getAllActions 追加按钮
- [ ] 文件加载时批量填充副标题

### 应用启动注册
- [ ] App.vue 或 main.ts 中有 registerFileFeature(createAlistEncryptFeature())
- [ ] 条件注册逻辑存在（enabled 判断）

## Phase 6: 设置页与 ExtensionsPage 🆕
- [ ] 设置页 alist_encrypt 配置区域存在
- [ ] suffix 输入框 + 冲突校验提示
- [ ] default_password (type=password)
- [ ] enc_type ion-select
- [ ] enabled 切换触发 register/unregister
- [ ] ExtensionsPage alist-decrypt 卡片存在
- [ ] COMBO_LITE_ID_MAP 包含 mapping

## Phase 7: Mock 测试 🆕

### Go Plugin 层
- [ ] plugin_test.go 编译通过
- [ ] TestPluginInitialization 全部场景通过
- [ ] TestCanDecrypt 全部场景通过
- [ ] TestEncryptDecryptRoundtrip V1/V2 通过
- [ ] TestEncryptWithV2Header 通过
- [ ] TestStreamRange 通过
- [ ] **🆕** TestBoundaryEmptyFile: 0字节 → ErrInvalidFormat
- [ ] **🆕** TestBoundaryTooSmallFile: <最小尺寸 → ErrInvalidFormat
- [ ] **🆕** TestBoundaryV2ZeroPlainSize: PlainSize=0 → ErrInvalidFormat
- [ ] **🆕** TestBoundarySizeMismatch: 解密大小≠PlainSize → DecryptionError
- [ ] **🆕** TestBoundaryPasswordHeuristic: 垃圾数据 → ErrInvalidPassword

### Go Handler 层
- [ ] mobile_api_alistencrypt_test.go 编译通过
- [ ] decodeFilename 测试全部通过
- [ ] stream handler 测试全部通过
- [ ] **🆕** Range end > fileSize → 截断 206 + 正确 Content-Range
- [ ] **🆕** 负数 Range → 400 Bad Request
- [ ] **🆕** start > fileSize → 416 Range Not Satisfiable
- [ ] **🆕** 密码错误 → 400 + error JSON
- [ ] **🆕** 第 4 并发连接 → 429 Too Many Requests

### 前端 FileFeature 架构测试
- [ ] file-feature.registry.spec.ts 存在且通过
- [ ] register/unregister 生命周期正确
- [ ] 重复注册保护有效
- [ ] 多 Feature 聚合查询正确
- [ ] 空 registry 安全

### 前端 AlistEncrypt Feature 测试
- [ ] features.alist-encrypt.spec.ts 存在且通过
- [ ] isActive 所有边界 case 通过
- [ ] getBadge / getSubtitle / getFileActions 正确
- [ ] promptPassword 缓存行为正确
- [ ] **🆕** getSubtitle 超时返回 null + 可重试状态
- [ ] **🆕** LRU 缓存淘汰: 501 条时最旧条目被移除

### 前端 API 测试
- [ ] encv.alistencrypt.spec.ts 存在且通过
- [ ] getAlistEncryptStreamUrl URL 构造正确
- [ ] decodeAlistFilename 成功/失败路径正确
- [ ] **🆕** 网络超时不抛异常，返回安全默认值
- [ ] **🆕** 网络断开不抛异常，返回安全默认值 + 重试标记

### 前端 Composable 测试
- [ ] useAlistEncrypt.spec.ts 存在且通过
- [ ] 密码/文件名缓存生命周期正确
- [ ] **🆕** 并发 promptPassword 各自独立缓存不冲突

## Phase 8: CI 与覆盖率 🆕
- [ ] CI workflow 包含所有新增测试步骤
- [ ] useFileFeatures 覆盖率 ≥ 95%
- [ ] AlistEncrypt Feature 覆盖率 ≥ 90%
- [ ] encv.ts API 覆盖率 ≥ 90%
- [ ] Go Plugin 覆盖率 ≥ 80%
- [ ] Go Handler 覆盖率 ≥ 80%

## 架构隔离性验证
- [x] internal/alistencrypt/ 无 RC4/ChaCha20 实现
- [ ] Files.vue 无业务特性直接 import（仅 useFileFeatures）
- [ ] features/alist-encrypt/ 是自包含模块（不依赖 Files.vue 内部状态）

## 性能与边界异常实现检查 🆕

### Go Stream 内存安全
- [ ] **🆕** stream handler 使用 io.LimitedReader 限制单次读取 ≤ 1MB
- [ ] **🆕** stream handler 并发连接数限制 ≤ 3（semaphore 或 channel）
- [ ] **🆕** 超过并发限制返回 429 + Retry-After header
- [ ] **🆕** 每个 connection 创建独立 Cipher 实例（无共享状态）
- [ ] **🆕** Range start > fileSize → 416 + Content-Range: bytes */N
- [ ] **🆕** Range end > fileSize → 截断到 fileSize-1，返回 206
- [ ] **🆕** 负数 Range → 400 Bad Request
- [ ] **🆕** 文件读取中 ENOENT → 500 + 日志记录

### Go 解密/加密边界处理
- [ ] **🆕** 源文件不存在 → TaskManager failed + "source file not found"
- [ ] **🆕** 目标目录不存在 → 自动 mkdir-p；失败 → failed + 错误信息
- [ ] **🆕** 磁盘空间不足 → 写入前检查；不足 → failed + "insufficient disk space"
- [ ] **🆕** 加密文件 = 0 字节 → ErrInvalidFormat "empty encrypted file"
- [ ] **🆕** 加密文件 < 最小尺寸 → ErrInvalidFormat "file too small to be valid"
- [ ] **🆕** V2 PlainSize = 0 → ErrInvalidFormat "V2 header declares zero plain size"
- [ ] **🆕** 解密后大小 ≠ V2 PlainSize → DecryptionError "size mismatch"
- [ ] **🆕** 密码错误启发式检测（前 1KB 全不可打印字节）→ ErrInvalidPassword

### 前端网络异常恢复
- [ ] **🆕** decode-filename 超时 (>5s) → 副标题显示重试按钮
- [ ] **🆕** decode-filename 500 → 同上
- [ ] **🆕** Capacitor Http.request catch 不抛异常到 console
- [ ] **🆕** 网络断开时 toast 提示 + 表单输入保留
