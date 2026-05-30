# Checklist (Round 3 — 深度适配)

## REQ 1-10 (Round 1-2) — 已完成 ✅
- [x] REQ 1-5: Feature 架构统一、密码弹窗、字幕防抖、FileInfo 增强
- [x] REQ 6: createTask pluginName 传递
- [x] REQ 7: 防重复提交锁
- [x] REQ 8: 普通文件 encrypt action（isActive 扩大 + 双分支）
- [x] REQ 9: 插件模式 handleRefresh 分叉
- [x] REQ 10: Mock 测试覆盖

---

## REQ-11: isAlistEncrypted 移除 isEncrypted 排除（新增）
- [ ] 11.1: 移除 `|| file.isEncrypted` 条件
- [ ] 11.2: .bin 文件仍返回 true（回归保护）
- [ ] 11.3: isEncrypted=true 的非 .bin 文件返回 false（正确行为）
- [ ] 11.4: 目录文件返回 false

## REQ-12: .bin 文件预览走流式解密路径（新增）
- [ ] 12.1: Files.vue handleFileClick 增加 isAlistEncrypted 分支
- [ ] 12.2: promptPassword → getStreamUrl → player 流程
- [ ] 12.3: ENCV 容器预览路径不受影响

## REQ-13: getAlistActions 三分支扩展（新增）
- [ ] 13.1: 分支 A — isAlistEncrypted(.bin) → decrypt + preview（不变）
- [ ] 13.2: 分支 B — isEncrypted=true → decrypt action（新增）
- [ ] 13.3: 分支 C — 普通文件 → encrypt action（不变）

## REQ-14: 解密任务 doPredict 降级（新增）
- [ ] 14.1: 解密任务 predictPlugin 空结果时不卡"分析中"
- [ ] 14.2: UI 允许正常提交

## REQ-15: Mock 测试更新（新增）
- [ ] 15.1: isAlistEncrypted 移除排除后的行为验证
- [ ] 15.2: getAlistActions 三分支全覆盖测试
- [ ] 15.3: doPredict 降级测试
