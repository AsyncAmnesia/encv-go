# Checklist (Round 3 — 深度适配)

## REQ 1-10 (Round 1-2) — 已完成 ✅
- [x] REQ 1-5: Feature 架构统一、密码弹窗、字幕防抖、FileInfo 增强
- [x] REQ 6: createTask pluginName 传递
- [x] REQ 7: 防重复提交锁
- [x] REQ 8: 普通文件 encrypt action（isActive 扩大 + 双分支）
- [x] REQ 9: 插件模式 handleRefresh 分叉
- [x] REQ 10: Mock 测试覆盖

---

## REQ-11: isAlistEncrypted 消除 .bin 硬编码，支持多后缀（新增）
- [ ] 11.1: `file.isEncrypted === true` → 返回 true
- [ ] 11.2: 文件名以任意已注册 containerExtension 结尾 → 返回 true
- [ ] 11.3: 配置项 suffix 作为 fallback 仍有效（.bin 默认后缀）
- [ ] 11.4: 目录文件返回 false

## REQ-12: .bin 文件预览走流式解密路径（新增）
- [ ] 12.1: Files.vue handleFileClick 增加 isAlistEncrypted 分支
- [ ] 12.2: 该分支调用 promptPassword → getStreamUrl → player
- [ ] 12.3: ENCV 容器预览路径不受影响（回归保护）

## REQ-13: 其他插件加密文件的解密入口（新增，随 REQ-11 自动解决）
- [ ] 13.1: 非 .bin 加密文件长按显示 decrypt action
- [ ] 13.2: .bin 长按仍显示 decrypt+preview（回归保护）

## REQ-14: 解密任务 doPredict 降级处理（新增）
- [ ] 14.1: 解密任务 predictPlugin 返回空时不卡"分析中"
- [ ] 14.2: UI 允许提交或显示降级提示

## REQ-15: Mock 测试更新（新增）
- [ ] 15.1: isAlistEncrypted(isEncrypted=true) 返回 true
- [ ] 15.2: isAlistEncrypted(多后缀) 返回 true
- [ ] 15.3: getAlistActions(非 .bin 加密文件) 返回 decrypt action
- [ ] 15.4: doPredict 空结果降级测试
