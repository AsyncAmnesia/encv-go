# Checklist (Borrow Nuclear-Boy 2026Q2)

## Stage 0: 仓库深读 + 借鉴点设计文档

- [ ] 仓库已克隆到 `/tmp/nuclear-boy`
- [ ] HANDOVER.md + HANDOVER2.0.md 完整内容已读
- [ ] agent-core 8 个核心 .kt 文件已深读
- [ ] memory 3 个文件已深读
- [ ] skills 4 个文件已深读
- [ ] python-bridge 3 个文件已深读
- [ ] tools-docgen 2 个文件已深读
- [ ] ui-chat 4 个文件已深读
- [ ] CLAUDE.md / INFO.md 已读
- [ ] `/workspace/.trae/documents/nuclear-boy-borrowing-design.md` 已创建（≥300 行）
- [ ] `/workspace/.trae/specs/borrow-nuclear-boy-2026q2/borrowing-points.md` 已创建

## Stage 1: System Prompt 工程化

- [ ] `/workspace/internal/agent/prompt.go` 已创建
- [ ] Build() 方法遵循 5 大原则
- [ ] 动态内容后置
- [ ] prompt_test.go 单测通过
- [ ] 5 大原则校验通过
- [ ] 实际生成的 prompt 长度 < 1500 字
- [ ] `go build ./cmd/encv` 0 错误

## Stage 2: ToolCallAccumulator 流式累积模式

- [ ] `/workspace/app/encv-mobile/src/composables/useToolCallAccumulator.ts` 已创建
- [ ] 4 状态（pending / accumulating / complete / executed）实现
- [ ] clear() 在 ReAct 开始时调用
- [ ] tool_call_start / delta / end 三个事件处理
- [ ] 与 useAgent.send() 集成
- [ ] 4 个单测场景通过
- [ ] `vue-tsc --noEmit` 0 错误

## Stage 3: buildHistoryMessages 去重 + 过滤

- [ ] agent_api.go buildHistoryMessages 实现
- [ ] 跳过 MessageRole.SYSTEM
- [ ] 预算控制 100,000 tokens
- [ ] 按 toolCallId 去重
- [ ] completedCalls 过滤
- [ ] completedCalls 为空 → toolCalls=null
- [ ] 前端 useAgent.ts 按 toolCallId 去重
- [ ] 3 个单测场景通过
- [ ] `go build` + `vue-tsc` 双 0 错误

## Stage 4: 参数别名容错层

- [ ] ToolDef 加 ArgAliases 字段
- [ ] resolveArg helper 实现
- [ ] 现有 10 个工具 description 标注别名
- [ ] 错误信息自动附加别名提示
- [ ] 4 个单测通过
- [ ] `go build` 0 错误

## Stage 5: 工具 JSON Schema description 优化

- [ ] description_lint.go 校验工具
- [ ] 现有 10 个工具按 4 要素重写
- [ ] 单测：含 4 要素 / 缺要素警告
- [ ] `go build` 0 错误

## Stage 6: AppResult<T> + AppError.humanMessage

- [ ] `/workspace/internal/common/result.go` 已创建
- [ ] Result[T] + AppError 类型定义
- [ ] 工具 handler 返回值扩展
- [ ] 前端 i18n error.humanMessage 映射
- [ ] 至少 5 个常见 error code 本地化
- [ ] 单测通过
- [ ] `go build` + `vue-tsc` 双 0 错误

## Stage 7: Skills 生态

- [ ] skill.yaml schema 定义
- [ ] `/workspace/internal/skills/manager.go` 已创建
- [ ] `/workspace/internal/skills/manifest.go` 已创建
- [ ] 3 个预置 skill 移植（skill-creator / file-organizer / code-formatter）
- [ ] runtime: python 共用 python-bridge
- [ ] 3 个单测场景通过
- [ ] `go build` 0 错误

## Stage 8: 三层记忆系统

- [ ] `/workspace/internal/memory/short.go` 内存 20 轮 FIFO
- [ ] `/workspace/internal/memory/medium.go` SQLite events
- [ ] `/workspace/internal/memory/long.go` SQLite preferences + project_meta
- [ ] 触发条件正确
- [ ] 3 个单测场景通过
- [ ] `go build` 0 错误

## Stage 9: 文档生成

- [ ] `/workspace/internal/tools/docgen.go` 已创建
- [ ] generate_docx / xlsx / pptx 三个工具
- [ ] 与 Stage 7 共用 python-bridge
- [ ] Python 沙箱注入 __main__ 模式
- [ ] 3 个端到端单测通过
- [ ] `go build` 0 错误

## Stage 10: HUD 栏

- [ ] `/workspace/app/encv-mobile/src/components/agent/TokenHudBar.vue` 已创建
- [ ] 6 元素全部实现
- [ ] 数据从 AG-UI usage / cost 事件读
- [ ] 黄/红预警工作
- [ ] 集成到 AgentChat.vue
- [ ] 单测：6 元素 / 70% 黄 / 90% 红
- [ ] `vue-tsc` 0 错误

## Stage 11: 凌晨轻声模式

- [ ] `/workspace/app/encv-mobile/src/composables/useNotificationTone.ts` 已创建
- [ ] 时间检测逻辑
- [ ] i18n key 齐全
- [ ] 通知音量切换
- [ ] 单测：边界值
- [ ] `vue-tsc` 0 错误

## Stage 12: 错误处理哲学

- [ ] AppError.HumanMessage 应用模板
- [ ] 5+ 个常见错误码友好文案
- [ ] Settings 切换"技术化错误"模式
- [ ] i18n 中英双语
- [ ] 单测通过
- [ ] `vue-tsc` 0 错误

## 端到端验证

- [ ] Stage 0..12 全部完成
- [ ] 所有单测通过
- [ ] `go build` + `vue-tsc` 双 0 错误
- [ ] 集成测试：实际启动 LLM + 调用 10 工具 + Skills + 记忆 + HUD 全链路
- [ ] 跨 Stage 一致性（Stage 6 AppError 在 Stage 7 Skills / Stage 9 docgen 都生效）
