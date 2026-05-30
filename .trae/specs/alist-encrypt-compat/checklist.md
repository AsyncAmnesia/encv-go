# Checklist (Round 4 — 全面排查补漏)

## REQ 1-15 (Round 1-3) — 已完成 ✅
- [x] REQ 1-10: Round 1-2 全部完成
- [x] REQ 11: isAlistEncrypted 移除 isEncrypted 排除
- [x] REQ 12: 加密文件预览走流式解密路径
- [x] REQ 13: getAlistActions 三分支扩展
- [x] REQ 14: 解密任务 doPredict 降级
- [x] REQ 15: Mock 测试更新

---

## REQ-16: 插件视图 container tab 正确显示所有加密文件（P0）（新增）
- [ ] 16.1: container tab 过滤条件包含 isAlistEncrypted(f)
- [ ] 16.2: origin tab 过滤条件排除 isAlistEncrypted(f)
- [ ] 16.3: alist-encrypt 加密文件在 container tab 可见
- [ ] 16.4: ENCV 容器文件在 container tab 仍可见（回归保护）

## REQ-17: getPluginIcon 消除硬编码映射（P1）（新增）
- [ ] 17.1: 图标获取逻辑支持 PluginMeta.icon 字段（如有）或改进 fallback
- [ ] 17.2: 新插件无需修改 getPluginIcon 即可显示合理图标

## REQ-18: Settings Feature 注册幂等（P1）（新增）
- [ ] 18.1: syncAlistEncryptFeature 内有幂等保护（相同状态不重复操作）
- [ ] 18.2: onMounted + watch 连续调用不会导致状态不一致

## REQ-19: 任务名称包含插件信息（P2）（新增）
- [ ] 19.1: pluginName 非空时格式为 "{basename} [{pluginName}]"
- [ ] 19.2: pluginName 为空时保持原有行为

## REQ-20: Mock 测试覆盖（新增）
- [ ] 20.1: filteredPluginFiles container/origin 双 tab 测试
- [ ] 20.2: getTaskName 格式化测试
