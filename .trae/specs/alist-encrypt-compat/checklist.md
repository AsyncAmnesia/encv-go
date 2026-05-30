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

### Go Handler 层
- [ ] mobile_api_alistencrypt_test.go 编译通过
- [ ] decodeFilename 测试全部通过
- [ ] stream handler 测试全部通过

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

### 前端 API 测试
- [ ] encv.alistencrypt.spec.ts 存在且通过
- [ ] getAlistEncryptStreamUrl URL 构造正确
- [ ] decodeAlistFilename 成功/失败路径正确

### 前端 Composable 测试
- [ ] useAlistEncrypt.spec.ts 存在且通过
- [ ] 密码/文件名缓存生命周期正确

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
