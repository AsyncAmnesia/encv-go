# Checklist (Round 4 — 全面排查补漏)

## REQ 1-15 (Round 1-3) — 已完成 ✅
- [x] REQ 1-10: Round 1-2 全部完成
- [x] REQ 11: isAlistEncrypted 移除 isEncrypted 排除
- [x] REQ 12: 加密文件预览走流式解密路径
- [x] REQ 13: getAlistActions 三分支扩展
- [x] REQ 14: 解密任务 doPredict 降级
- [x] REQ 15: Mock 测试更新

---

## REQ-16: 插件视图 container/origin tab 过滤委托给插件系统（P0）
- [x] 16.1: 创建 `isContainerFile(file)` 辅助函数（组合 `file.isEncrypted || isAlistEncrypted(file)`） ✅
- [x] 16.2: container tab 过滤条件改为 `isContainerFile(f)` ✅
- [x] 16.3: origin tab 过滤条件改为 `!isContainerFile(f)` ✅
- [x] 16.4: ENCV 容器文件在 container tab 仍可见（回归保护） ✅

## REQ-17: getPluginIcon 消除硬编码映射（P1）
- [x] 17.1: fallback 图标从 cubeOutline 改为 lockClosed ✅
- [x] 17.2: 清理未使用的 cubeOutline import ✅

## REQ-18: Settings Feature 注册幂等（P1）
- [x] 18.1: syncAlistEncryptFeature 内有幂等保护（alistFeatureRegistered 状态变量） ✅
- [x] 18.2: onMounted + watch 连续调用不会导致重复操作 ✅

## REQ-19: 任务名称包含插件信息（P2）
- [x] 19.1: pluginName 非空时格式为 "{basename} [{pluginName}]" ✅
- [x] 19.2: pluginName 为空时保持原有行为 ✅

## REQ-20: Mock 测试覆盖
- [x] 20.1: 现有 208 测试全部通过，回归验证完整 ✅
