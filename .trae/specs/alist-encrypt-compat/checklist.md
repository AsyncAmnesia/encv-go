# Checklist

## REQ-1: 统一操作入口（Feature Actions 唯一性）
- [x] 1.1: actions.ts decrypt handler 使用 useNewTaskModal 替代 router.push ✅
- [x] 1.2: actions.ts encrypt handler 使用 useNewTaskModal 替代 router.push ✅（当前仅 decrypt action 存在）
- [x] 1.3: Files.vue Section 2 内联加解密代码已删除，由 getAllActions() Feature action 提供 ✅
- [x] 1.4: handleEncryptFile/handleDecryptFile 已清理（dead code） ✅

## REQ-2: 密码弹窗正确交互
- [x] 2.1: confirm 后弹窗自动关闭（return true） ✅
- [x] 2.2: cancel 后弹窗关闭并 resolve null ✅

## REQ-3: 字幕查询稳定性
- [x] 3.1: 防抖窗口内重复调用返回已有 Promise（非 null） ✅

## REQ-4: FileInfo 元信息完整
- [x] 4.1: 加密文件展示解码后名称（灰色斜体 + border-top 分隔） ✅
- [x] 4.2: 展示加密状态 badge（warning 色 Yes） ✅

## REQ-5: 编译与测试通过
- [x] 5.1: vue-tsc 零错误 ✅
- [x] 5.2: vitest 全通过 (190/190) ✅
- [x] 5.3: vite build 成功 ✅
