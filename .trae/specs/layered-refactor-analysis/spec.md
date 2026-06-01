# 分层重构分析 Spec

## Why

经过本轮管线自洽性重构（产物路径从文件系统遍历改为管线返回值），以及时间线可展开详情的实现，暴露出项目在多个层累积的「架构债」。本 spec 旨在**分层梳理需要重构的部位**，为后续每个 PR 提供明确的范围和验收标准，避免一次性大规模改动导致风险不可控。

核心矛盾：
- **后端管线已经自洽**（产品路径由管线返回值），但其他层仍残留「老式遍历/拼接」的反模式
- **前端时间线已可展开**（TaskStep 后端化），但 `Tasks.vue` 仍承担过多职责（725 行 + eventBus 多重订阅）
- **插件接口已统一返回值**（PostEncryptProcessor/Decrypt），但 7 个插件实现仍有大量样板代码
- **phase 名称硬编码**（`analyzing`/`initializing`/`encrypting` 等字符串散落多处），无类型保护
- **CI/Makefile 脚本重复**（test.yml + android.yml 有重叠的 setup 步骤）

## What Changes

新增本 spec 作为**后续多个原子重构任务的入口文档**：
- 不一次性提交所有改动
- 列出每一层的高优先级重构项（含严重程度、范围、影响）
- 每个 Phase 在子任务 spec 中再独立细化（本 spec 不展开具体实现）
- **不改变**任何运行时行为

## Impact

- Affected specs: 无（纯重构元 spec）
- Affected code（按层列出**全部候选**清单，本 spec 不强制全部完成）：

### 后端 (Go) 层

| 层级 | 候选文件 | 现状 |
|------|---------|------|
| Plugin 接口 | `internal/v2/plugins/registry.go` | Plugin 接口的 `PreEncryptProcessor/PreDecryptProcessor/PostDecryptProcessor` 仍是 `error` 返回，产物/状态信息丢失 |
| Plugin 接口 | `internal/v2/plugins/registry.go` | 插件返回值元数据散落：manifest、footer、header 各有不同返回方式 |
| 插件实现 | `internal/v2/plugins/{video,audio,image,pdf,text,wps}/plugin.go` | 6 个 V4 容器插件的 `PostEncryptProcessor` 几乎完全相同：都是 `packParams, err := buildPackParams(...); outputPath, err := packer.StandardPostEncrypt(packParams); return outputPath, nil` |
| Plugin 实现 | `internal/v2/plugins/{video,audio,image,pdf,text,wps}/plugin.go` | 6 个插件的 `Decrypt` 完全相同：构造 `vIndex` → 读 manifest → 调用 `processDecryption` → 返回 `filepath.Join(outputDir, vIndex.GetOriginalFilename())` |
| Task Manager | `internal/service/task_manager.go` | `processEncrypt` / `processDecrypt` 中 phase 字符串硬编码（`"encrypting"`/`"completed"` 等），无类型保护 |
| Task Manager | `internal/service/task_manager.go` | `monitorFileProgress` 还在用裸 `slog` 输出 phase，但 phase 字符串与 `updateProgress` 不同步 |
| Task Manager | `internal/service/task_manager.go` | 大量 `tm.mu.Lock()` + 直接修改 `task.XXX` 字段，缺少结构化的 mutation API（如 `tm.completeTask(id, outputPath)`） |
| HTTP Handler | `internal/server/mobile_api.go` | 多个 `handleAlistXxxGin` 函数有重复的 query param 解码、ServeFile 包装、错误处理模式 |
| HTTP Handler | `internal/server/mobile_api.go` | 路径解析（`utils.DecodeGinQueryParam` + `SafeResolveToAbsPath`）模式重复 |
| Service | `internal/service/mobile_service.go` | `GetFileInfo` 内部可能还有散落的 version==4 分支 |
| Physical 打包 | `internal/v2/physical/file_single.go` vs `file_multi.go` | 两者 `Pack()` 实现有重复 manifest 写盘逻辑 |

### 前端 (Vue/Ionic) 层

| 层级 | 候选文件 | 现状 |
|------|---------|------|
| View | `app/encv-mobile/src/views/Tasks.vue` | 725 行，承担：列表渲染、过滤/搜索/排序、刷新、eventBus 多重订阅、modal 触发、UI 状态机 |
| View | `app/encv-mobile/src/views/Tasks.vue` | `onMounted` 中注册 5+ 个 eventBus 监听器，部分可能在跨 tab 场景下违反 §2.1 铁律 |
| View | `app/encv-mobile/src/views/Files.vue` | 仍然使用 `eventBus.emit` 触发跨组件操作（之前修复了 modal 跨 tab，但其他事件流是否还有反模式？） |
| Component | `app/encv-mobile/src/components/TaskDetailModal.vue` | 440+ 行，承担：基本信息、时间线、产物展示、错误展示、警告展示、操作按钮。**已完成的 steps 展开逻辑**埋藏在 computed 中 |
| Component | `app/encv-mobile/src/components/TaskDetailModal.vue` | `phaseLabel` 映射函数 7 行 case 散落在 script 块中，可独立为 `usePhaseLabel.ts` composable |
| Component | `app/encv-mobile/src/components/NewTaskModal.vue` | 加密/解密双模式 if-else 大量重复，可考虑拆为 `<EncryptBody>` + `<DecryptBody>` 子组件 |
| Composable | `app/encv-mobile/src/composables/useTaskDetail.ts` | 文件存在但 30 行内只暴露几个简单函数，可能未被使用或职责不清 |
| Composable | `app/encv-mobile/src/composables/useTaskForm.ts` | `doPredict` 内部有 500ms 防抖 + API 调用，但外层 `useNewTaskModal` 不知道这个时序（依赖外层 await），存在时序耦合 |
| Feature/action | `app/encv-mobile/src/features/alist-encrypt/actions.ts` | actions.ts 还在用 `router.push` 做导航（与 §1.4 modal 铁律相关） |
| API client | `app/encv-mobile/src/api/encv.ts` | 仍有 `fetch` 直接调用，未统一走 axios/ofetch |

### 横切关注点

| 层级 | 候选文件 | 现状 |
|------|---------|------|
| Plugin 元数据 | `internal/v2/plugins/registry.go` | `GetAllRegisteredContainerExtensions()` 和 `IsContainer(name)` 还有别的硬编码 fallback？ |
| Phase 类型 | 后端 + 前端双份 | 后端字符串 → 前端字符串，无共享枚举/常量 |
| CI | `.github/workflows/test.yml` + `android.yml` | 都有 `setup-go`/`setup-node`/`mise` 步骤，重复 |
| CI | `Makefile` + `hack/hack.mk` + `hack/hack-cli.mk` | 三处都有 `test`/`build` 入口，定义分散 |
| Test | `internal/*/mock_broadcaster_test.go` | 仅 broadcaster 有独立 mock，task_manager 内部 mock 散落在多个测试文件 |
| i18n | `app/encv-mobile/src/composables/useI18n.ts` | 1299 行，所有翻译键集中在一个文件，查找/修改代价大 |
| Error code | 整个项目 | 后端用字符串 error，前端用 `t('error.xxx')` 字符串映射，无共享 schema |

---

## ADDED Requirements

### Requirement: 分层重构 spec 入口

本 spec SHALL 作为后续每个原子重构任务的**索引入口**，每个 Phase 完成后**勾选 checklist 对应项**。

#### Scenario: 用户批准 Phase 1 后开始
- **WHEN** 用户批准本 spec 并指定"开始 Phase 1"
- **THEN** 实施对应原子任务（见 tasks.md 各项）
- **AND** 完成后**只勾选 Phase 1 对应的 checklist 项**
- **AND** 不擅自开始后续 Phase

#### Scenario: 跨 Phase 依赖
- **WHEN** Phase N 的原子任务完成且 checklist 全部勾选
- **THEN** 才允许开始 Phase N+1
- **AND** Phase 1 完成后，必须先在 test 环境验证再进入 Phase 2

### Requirement: 重构优先级评估准则

每个原子重构任务 SHALL 包含以下字段（在具体子 spec 中填写）：

| 字段 | 含义 |
|------|------|
| **严重程度** | P0 (崩溃/数据丢失) / P1 (重复代码 >200行) / P2 (可读性/可测试性) |
| **改动行数预估** | 净增/净减行数 |
| **影响面** | 修改的包/文件数 + 公共 API 变更数 |
| **验证方式** | 编译 + 单元测试 + 集成测试 + 端到端测试 |
| **回滚成本** | git revert 即可 / 涉及数据迁移 |

#### Scenario: 优先级判定
- **WHEN** 一个候选重构项被评估
- **THEN** P0 项必须立即处理
- **AND** P1 项列入下个 Phase
- **AND** P2 项可累积到季度重构窗口

---

## MODIFIED Requirements

### Requirement: Plugin 接口返回值一致性

`Plugin` 接口的所有处理器方法 SHALL 返回**结构化结果**而非纯 `error`：
- `PreEncryptProcessor(index, inputPath, inputRootDir, outputDir) error` — 暂不变（pre 阶段无产物）
- `PreDecryptProcessor(containerPath, outputDir) error` — 暂不变
- `PostEncryptProcessor(result) (string, error)` — **已修改** ✅
- `Decrypt(containerPath, outputDir) (string, error)` — **已修改** ✅
- `PostDecryptProcessor(containerPath) error` — 暂不变（post 阶段无产物）

> **本次 spec 不强制**继续修改 `Pre*` 处理器，因它们不产生产物；但应在下个 Phase 评估是否需要返回结构化状态。

### Requirement: Task Manager mutation API

`TaskManager` SHALL 提供结构化的 mutation 方法，避免在外部直接 `tm.mu.Lock()` 后修改字段：

| 当前直接修改 | 建议封装方法 |
|-------------|------------|
| `task.Status = "completed"; task.Progress = 100; ...; task.CompletedAt = &now` | `tm.completeTask(id, outputPath)` |
| `task.Status = "cancelling"` | `tm.markCancelling(id)` |
| `task.Status = "failed"; task.Error = msg` | `tm.failTaskWithDetail(id, errMsg, errDetail)` |
| `task.Steps = append(...)` | `tm.appendStep(id, phase, detail)` |
| `tm.mu.Unlock(); tm.saveTasks()` | 封装在 mutation 方法内部 |

#### Scenario: 调用方使用结构化 API
- **WHEN** 业务代码需要更新任务状态
- **THEN** 调用 `tm.completeTask(id, outputPath)` 等方法
- **AND** 内部统一加锁 + 持久化 + 广播
- **AND** 调用方不再持有 `task` 指针（避免锁外修改）

### Requirement: 插件实现去重

6 个 V4 容器插件的 `PostEncryptProcessor` 和 `Decrypt` SHALL 提取为**基类/组合 helper**，消除样板代码。

#### Scenario: 视频/音频/图片/PDF/Text/WPS 插件
- **WHEN** 6 个 V4 插件实现 `PostEncryptProcessor`
- **THEN** 全部委托给一个公共函数 `plugins.StandardPostEncryptForContainer(plugin, params)`
- **AND** 6 个插件的 `Decrypt` 全部委托给 `plugins.StandardDecryptForContainer(plugin, containerPath, outputDir)`

#### Scenario: 插件间差异
- **WHEN** 某插件需要特化处理（如 `video` 的 verify、`image` 的 EXIF 保留）
- **THEN** 通过 plugin 自己的 hook 函数扩展，**不**改基类函数签名
- **AND** 公共代码逻辑保持一致

### Requirement: Phase 名称类型化

后端和前端的 phase 字符串 SHALL 由**共享类型**约束：

#### Scenario: 后端 Go
- **WHEN** 代码中引用 phase 字符串（`"encrypting"`/`"completed"` 等）
- **THEN** 使用 `const` 块或 `iota` 类型化（如 `type Phase int; const PhaseEncrypting Phase = iota + 1`）
- **AND** JSON 序列化时仍输出小写字符串以保持 API 兼容

#### Scenario: 前端 TS
- **WHEN** 前端组件引用 phase 字符串
- **THEN** 使用 `import { Phase } from '@/types/phase'` 枚举类型
- **AND** `i18n` 键名也由 phase 类型派生，避免散落字符串

### Requirement: 任务步骤类型化

后端 `TaskStep` 和前端 `TaskStep` SHALL 字段保持完全一致：

| 字段 | Go | TS | 用途 |
|------|-----|-----|------|
| `phase` | `string` | `string` | 阶段名（来自 Phase 枚举） |
| `startedAt` | `time.Time` | `string` (ISO) | 开始时间 |
| `completedAt` | `*time.Time` | `string?` | 完成时间（可空表示进行中） |
| `detail` | `string` | `string?` | 阶段详情（如产物路径） |

> **本轮已添加 Steps 字段** ✅，本 spec 不重复实现，只确保后续修改保持类型一致。

### Requirement: HTTP Handler 模板化

`internal/server/mobile_api.go` 中所有 `handleXxxGin` 函数 SHALL 使用统一模板：

```go
func (s *Server) handleGinTemplate(c *gin.Context, opts HandlerOpts) {
    // 1. 解析 path/password/extra
    // 2. SafeResolveToAbsPath
    // 3. 委托给 service 层
    // 4. 包装 ServeFile/JSON
    // 5. 统一错误处理
}

type HandlerOpts struct {
    RequirePassword bool
    PathParam       string
    ExtraParamKeys  []string
    Handler         func(absPath string, opts ResolvedOpts) (provider.FileContentProvider, error)
}
```

#### Scenario: 现有 handler 迁移
- **WHEN** 重构时
- **THEN** `handleAlistEncryptStreamGin` 等作为模板参考
- **AND** 逐个迁移到统一模板

### Requirement: Tasks.vue 拆分为 store + 视图

`Tasks.vue` 725 行的状态管理逻辑 SHALL 迁移到 Pinia store（如果项目还未引入）/ composable：

#### Scenario: 拆分目标
- **WHEN** 重构时
- **THEN** 提取 `useTasksStore()`：负责列表、过滤、刷新、eventBus 订阅
- **AND** `Tasks.vue` 只剩 UI 渲染 + 用户事件分发
- **AND** `<template>` 部分行数 < 200

#### Scenario: eventBus 审查
- **WHEN** 拆分 store 时
- **THEN** 重新审查 `Tasks.vue` 中注册的 5+ 个 eventBus 监听
- **AND** 跨 tab 的迁移到 composable 调用（同 §1.4 铁律）
- **AND** 同 tab 自消费的保留在 store 内

### Requirement: TaskDetailModal 子组件化

`TaskDetailModal.vue` 440+ 行 SHALL 拆分为：
- `<TaskBasicInfo>` — 文件名/类型/插件/容器版本
- `<TaskTimeline>` — 时间线（含已完成的 steps 展开逻辑）
- `<TaskOutputInfo>` — 产物展示
- `<TaskErrorSection>` — 错误展示
- `<TaskWarningSection>` — 警告展示
- `<TaskActionButtons>` — 取消/重试/移除

#### Scenario: 拆分后
- **WHEN** 拆分完成
- **THEN** `TaskDetailModal.vue` < 100 行，只做组合
- **AND** 每个子组件独立可测
- **AND** i18n 键名仍统一在 `useI18n.ts`

### Requirement: CI/Makefile 去重

`.github/workflows/test.yml` 和 `android.yml` 的公共步骤 SHALL 抽取为 composite action 或 reusable workflow。

#### Scenario: 复用步骤
- `actions/checkout` + `setup-go` + `mise install` 抽取为 `/.github/actions/setup-env`
- `npm ci` + `vue-tsc --noEmit` 抽取为 `/.github/actions/frontend-check`

#### Scenario: Makefile 入口统一
- `Makefile` 主入口覆盖所有场景
- `hack/hack.mk` 和 `hack/hack-cli.mk` 简化为工具函数库

---

## REMOVED Requirements

无。本 spec 是元 spec，不删除任何功能，只是规划后续原子重构任务。
