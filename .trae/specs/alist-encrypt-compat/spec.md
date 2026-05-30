# 移动端插件系统适配修复 Spec

## Why

移动端 alist-encrypt 插件存在 4 大适配问题：加密/解密操作路径不一致（Feature actions 用 router.push 反模式 vs Files.vue 内联用 openNewTask）、预览流程绕过插件系统、密码弹窗交互缺陷、FileInfo 缺少元信息展示。根因是 FileFeature 架构骨架已搭建但未真正接管核心操作路径，导致 Files.vue 内联代码与 Feature 模块功能重叠且行为不一致。

## What Changes

### 核心原则：Files.vue 不再内联任何插件特定逻辑，全部委托给 FileFeature 架构

- **统一解密/加密操作路径**：删除 Files.vue Section 2 的内联 encrypt/decrypt 按钮，改由 `getAllActions()` 返回的 Feature Action 提供
- **修复 actions.ts 反模式**：`createDecryptAction` / `createEncryptAction` 从 `router.push` 改为调用 `useNewTaskModal().openNewTask()`
- **修复密码弹窗**：confirm handler 正确关闭
- **修复字幕防抖竞态**：重复调用返回已有 Promise
- **增强 FileInfo**：展示加密文件解码后名称和元信息

## Impact

- Affected code:
  - `src/features/alist-encrypt/actions.ts` — 重写 handler 为 useNewTaskModal 调用
  - `src/features/alist-encrypt/password-dialog.ts` — 修复 confirm 关闭
  - `src/features/alist-encrypt/subtitle.ts` — 修复防抖竞态
  - `src/views/Files.vue` — 删除 Section 2 内联加解密代码，依赖 getAllActions()
  - `src/views/FileInfo.vue` — 增强加密文件信息展示
  - `src/views/Preview.vue` — 确认加密文件预览路径正确

## Requirements

### REQ-1: 统一操作入口（Feature Actions 唯一性）

系统 SHALL 通过 `useFileFeatures().getAllActions(file)` 作为文件操作的唯一扩展点。Files.vue SHALL NOT 包含任何插件特定的加密/解密/预览逻辑。

#### 场景 1.1: 加密文件长按菜单
- **WHEN** 用户长按一个 .enc 文件
- **THEN** 菜单中 SHALL 显示「解密」action（来自 alist-encrypt Feature），点击后调用 `openNewTask(path, 'decrypt')`
- **AND** 菜单中 SHALL NOT 显示 Files.vue Section 2 的重复「解密」按钮

#### 场景 1.2: 普通文件长按菜单
- **WHEN** 用户长按一个非加密文件
- **THEN** 菜单中 SHALL 显示「加密」action（来自 alist-encrypt Feature），点击后调用 `openNewTask(path, 'encrypt')`

#### 场景 1.3: 操作路径一致性
- **WHEN** 无论从哪个 tab 触发加密/解密操作
- **THEN** 所有路径 SHALL 统一通过 `modalController.create(NewTaskModal)` 打开全局 overlay

### REQ-2: 密码弹窗正确交互

- **WHEN** 用户在密码输入框输入密码并点击确认
- **THEN** 弹窗 SHALL 自动关闭并 resolve 密码字符串
- **WHEN** 用户点击取消
- **THEN** 弹窗 SHALL 关闭并 resolve null

### REQ-3: 字幕查询稳定性

- **WHEN** 同一加密文件的 getAlistSubtitle 在防抖窗口内被多次调用
- **THEN** SHALL 返回已有 Promise（而非 null），确保最终回调触发一次

### REQ-4: FileInfo 元信息完整

- **WHEN** 用户查看加密文件的 FileInfo 页面
- **THEN** SHALL 显示：原始文件名（解码后）、加密状态 badge、文件大小

### REQ-5: 编译与测试通过

- vue-tsc 零错误
- vitest 全部通过
- vite build 成功
