# 移动端插件系统适配修复 Spec（Round 2 — 4 Bug 修复）

## Why

上一轮（REQ 1-5）完成了 FileFeature 架构统一、密码弹窗修复、字幕防抖、FileInfo 增强。实测发现 4 个新 bug：
1. **新建加密任务选择 alist-encrypt 插件没生效** — `useNewTaskModal.onSubmit` 调用 `createTask()` 时**完全没有传递用户选择的 pluginName**
2. **重复创建两个任务** — onSubmit 无防重入保护 + doPredict 双重调用路径
3. **文件长按菜单没有加密操作** — `getAlistActions()` 只对 `.enc` 文件返回 actions（decrypt/preview），普通文件返回空数组；上轮 Task 4 删除了 Files.vue 内联加密代码后，Feature 系统未提供 encrypt 入口
4. **插件类型文件列表空状态时下滑刷新导致空状态下方出现文件列表** — `handleRefresh` 刷新了主 `files` 数组而非 `pluginFiles`，导致主列表渲染在 plugin-view 下方

## What Changes

### 核心原则：从插件系统本质上解决问题，非打补丁

- **Bug 1 修复（本质）**：`createTask()` API 需要支持 `pluginName` 参数；`useNewTaskModal.onSubmit` 必须将用户选择的插件名传递到后端
- **Bug 2 修复（本质）**：onSubmit 添加 submitting 锁防止双击重复提交
- **Bug 3 修复（本质）**：`getAlistActions()` 对可加密的普通文件也返回 encrypt action；Feature 的 `isActive` 判定需要扩展（或增加独立的 encrypt action 来源）
- **Bug 4 修复（本质）**：插件模式下 `handleRefresh` 应刷新 `pluginFiles` 而非主 `files` 数组

## Impact

- Affected specs: REQ-1（统一操作入口）需扩展为双向（encrypt + decrypt）
- Affected code:
  - `src/api/encv.ts` — `createTask()` 增加 `pluginName` 参数
  - `src/composables/useNewTaskModal.ts` — onSubmit 传递 pluginName + 防重入锁
  - `src/features/alist-encrypt/actions.ts` — 增加普通文件的 encrypt action
  - `src/features/alist-encrypt/index.ts` — isActive 或 getFileActions 逻辑调整
  - `src/views/Files.vue` — handleRefresh 区分 plugin mode / normal mode
  - `__tests__/` — 补充 useNewTaskModal / actions / Files refresh 的 mock 测试

---

## ADDED Requirements

### REQ-6: 新建任务必须传递 pluginName 到后端

系统 SHALL 将用户新建任务时选择的插件名称传递给后端 `createTask` API。

#### 场景 6.1: 用户选择特定插件后提交
- **WHEN** 用户在 NewTaskModal 中从下拉框选择了 `alist-encrypt` 插件并点击「创建任务」
- **THEN** `createTask()` API 请求 body SHALL 包含 `pluginName: 'alist-encrypt'`
- **AND** 后端 SHALL 使用指定插件执行加密任务

#### 场景 6.2: 自动预测的单插件场景
- **WHEN** 后端只返回 1 个候选插件（无需用户手动选择）
- **THEN** `createTask()` SHALL 自动使用该预测插件的 name 作为 `pluginName`

#### 场景 6.3: 多插件候选时用户切换选择
- **WHEN** 用户在下拉框中切换了插件选择（如从 plugin-a 切到 plugin-b）
- **THEN** 最终提交 SHALL 使用最后选中的 `candidates[selectedPluginIndex].name`

### REQ-7: 任务创建防重复提交

系统 SHALL 防止用户新建任务时的重复提交。

#### 场景 7.1: 快速双击提交按钮
- **WHEN** 用户快速连续点击两次「创建任务」按钮
- **THEN** 系统 SHALL 只创建 1 个任务（第一次点击触发，第二次被忽略）

### REQ-8: 普通文件长按菜单显示加密操作（插件系统架构内解决）

系统 SHALL 通过调整 alist-encrypt Feature 的 `isActive` 范围，让其对所有非目录文件激活，在 `getFileActions` 内部按文件加密状态分支返回不同 actions。**不得脱离 Feature 架构在 Files.vue 或其他位置硬编码加密入口。**

#### 架构约束

`useFileFeatures.collectActions()` 在 L63 有 gatekeeper：`if (!feature.isActive(file) || !feature.getFileActions) continue`。
这意味着 **`isActive` 返回 false 时 `getFileActions` 根本不会被调用**。因此：
- ❌ 错误做法：保持 `isActive = isAlistEncrypted`，在 `getAlistActions` 里给普通文件返回 encrypt action（死代码，永远不会执行）
- ✅ 正确做法：扩大 `isActive` 范围 → 让 `getAlistActions` 内部分支

#### 场景 8.1: 非 .bin 普通文件长按
- **WHEN** 用户长按一个非加密的普通文件（如 `.mp4`, `.pdf` 等非目录文件）
- **THEN** alist-encrypt Feature 的 `isActive` SHALL 返回 true
- **AND** `getAlistActions(file)` SHALL 返回「加密」action（调用 `openNewTask(path, 'encrypt')`）
- **AND** 长按菜单通过 `getAllActions()` 收集到该 action 并展示

#### 场景 8.2: .bin 加密文件长按（回归保护）
- **WHEN** 用户长按一个 `.bin` 加密文件
- **THEN** `isActive` SHALL 返回 true
- **AND** `getAlistActions(file)` SHALL 返回「解密」+「流预览」（行为不变）

#### 场景 8.3: 目录文件不显示加密相关 action
- **WHEN** 用户长按一个目录
- **THEN** `isActive` SHALL 返回 false
- **AND** 不返回任何 alist-encrypt 相关 action

### REQ-9: 插件模式空状态下拉刷新不泄漏主列表

系统 SHALL 在插件模式下正确刷新插件文件列表。

#### 场景 9.1: 插件视图空状态 + 下拉刷新
- **WHEN** 用户进入某插件类型视图且该类型下无文件（显示空状态），然后下拉刷新
- **THEN** 刷新完成后 SHALL 仍显示插件视图空状态（或刷新后的插件文件列表）
- **AND** 主文件列表 SHALL NOT 出现在插件空状态下方

#### 场景 9.2: 插件视图有文件 + 下拉刷新
- **WHEN** 用户在插件文件列表中下拉刷新
- **THEN** 刷新后 SHALL 更新插件文件列表（而非主文件列表）

### REQ-10: Mock 测试覆盖新增场景

测试套件 SHALL 覆盖以上 4 个 bug 的修复场景。

- [ ] useNewTaskModal: onSubmit 传递正确的 pluginName
- [ ] useNewTaskModal: 双击提交只创建 1 个任务
- [ ] getAlistActions: 普通文件（isActive=true）返回 encrypt action
- [ ] getAlistActions: .bin 文件仍返回 decrypt + preview（回归保护）
- [ ] isActive: 目录文件返回 false，非目录文件返回 true
- [ ] Files.vue: plugin mode 下 handleRefresh 刷新 pluginFiles
