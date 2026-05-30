# Tasks

## Task 1: 修复 actions.ts — 统一为 useNewTaskModal 路径
- [x] 1.1: actions.ts `createDecryptAction` handler 从 `router.push('/tabs/tasks')` 改为 import + 调用 `useNewTaskModal().openNewTask(path, 'decrypt')`
- [x] 1.2: actions.ts `createEncryptAction` handler 同样改为 `openNewTask(path, 'encrypt')`（注：当前仅存在 decrypt action 使用 router.push）
- [x] 1.3: 移除 actions.ts 中不再需要的 `useRouter` 导入

## Task 2: 修复 password-dialog.ts — confirm 自动关闭
- [x] 2.1: confirm handler 中 `return false` → 改为 resolve 后 return true 让 Ionic 自动 dismiss

## Task 3: 修复 subtitle.ts — 防抖竞态
- [x] 3.1: 防抖窗口内对同一文件的重复调用返回已有 pending Promise 而非 null；移除未使用的 timer 变量

## Task 4: Files.vue 清理内联加解密代码
- [x] 4.1: 删除 handleLongPress 中 Section 2（L910-L940）的内联 encrypt/decrypt 按钮（31 行）
- [x] 4.2: 确认 getAllActions() 返回的 Feature action 能正确覆盖加密/解密操作 ✅
- [x] 4.3: 删除 dead code：handleEncryptFile/handleDecryptFile/resolveFileItem/openNewTask/useNewTaskModal/usePathResolver（共 45 行）

## Task 5: FileInfo.vue 增强加密文件展示
- [x] 5.1: 加密文件展示解码后文件名（通过 isAlistEncrypted + loadDecodedName API）
- [x] 5.2: 展示加密状态 badge + 灰色斜体原始文件名行 + 优雅降级

## Task 6: 验证
- [x] 6.1: vue-tsc --noEmit 零错误 ✅
- [x] 6.2: vitest run 全部通过 (190/190) ✅（含回归测试更新为 Feature 架构断言）
- [x] 6.3: vite build 成功 ✅

# Dependencies
- [Task 1] 无依赖，最高优先级（核心功能路径）✅
- [Task 2] 无依赖，可与 Task 1 并行 ✅
- [Task 3] 无依赖，可与 Task 1 并行 ✅
- [Task 4] 依赖 Task 1 完成（先确保 Feature action 路径正确再移除内联代码）✅
- [Task 5] 可与 Task 1-3 并行 ✅
- [Task 6] 依赖 Task 1-5 全部完成 ✅
