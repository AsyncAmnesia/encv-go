# Checklist

## Phase 6: 适配问题修复
- [ ] 6.1: actions.ts 解密/加密操作使用 useNewTaskModal 替代 router.push（符合 capacitor.md §1.4）
- [ ] 6.2: password-dialog.ts 确认后自动关闭（return true 或手动 dismiss）
- [ ] 6.3: subtitle.ts 防抖竞态优化（重复调用返回已有 Promise）
- [ ] 6.4: FileInfo.vue 加密文件元信息展示正确
- [ ] 6.5: 加密文件预览流程走正确的 StreamPreview 路径

## Phase 7: 端到端验证
- [ ] 7.1: 文件列表 badge/subtitle 渲染正确
- [ ] 7.2: 长按解密 action → NewTaskModal(decrypt) 正确打开
- [ ] 7.3: 解密任务完整执行流程正常
- [ ] 7.4: 长按加密 action → NewTaskModal(encrypt) 正确打开
- [ ] 7.5: 加密任务完整执行流程正常
- [ ] 7.6: 密码弹窗交互正确（输入/取消/确认/关闭）
- [ ] 7.7: vue-tsc + vitest + vite build 全通过
