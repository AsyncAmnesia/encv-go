# Spec: Go Agent 独立服务 + OpenList 定制接口 + Vue 渲染壳

> **核心思路**：Agent 是独立的 Go 微服务，通过 OpenList **定制开放的 HTTP 接口**执行工具调用。前端是个极薄 Vue 渲染壳，**首个集成入口直接嵌在 encv-mobile 主应用首页**。
> **架构三段式**：`agent` Go 服务（SSE 流式对话 + 4-决策确认 + 内存缓存续传）↔ OpenList（仅暴露 `/api/ext/list_files`、`/api/ext/delete_file` 等定制接口）↔ encv-mobile 主应用首页（Vue 渲染壳）。
> **UI 参考**：[codex_web](https://github.com/shopkeeper2020/codex_web) 的 `MessageBlocks`/`ApprovalCard`/`renderTurnItems` 模式 —— `ApprovalDecision` 4 选 1、`MessageAuthor` 作者头、`BlockHeader` 块头、`GroupedOperationMessage` 操作分组、消息列表虚拟化（>120 触发）。
> **OpenList 集成边界（铁律）**：**不在 OpenList 源码里集成 agent 库**（避免 fork 维护成本、避免污染 upstream）。OpenList 只负责「定制接口开放」—— 在 Hi-Sillot/OpenList 提 PR 添加 5-8 个 `/api/ext/*` 端点，agent 服务通过 HTTP 调用这些端点实现工具执行。

---

## Why

当前 ENCV + OpenList 的 AI 集成方式（如果有）是外部 HTTP/MCP 桥接，AI 与应用本体之间有网络边界，无法直接调用 Go 方法、无法利用进程内缓存、无法保证调用安全性。

**新方案核心价值**：

1. **零 OpenList 源码污染**：agent 库独立部署为 Go 微服务，OpenList 只暴露最小化定制接口（5-8 个 `/api/ext/*` 端点），后续 OpenList 升级 / 切换 fork 都不影响 agent。
2. **绝对安全可控**：agent → OpenList 走 HTTP 调用，所有调用可审计、可限流、可熔断；OpenList 自己的鉴权机制（账号 / Token）继续生效，不会因为 AI 调用绕过权限检查。
3. **天然支持断点续传**：Go 进程不挂，内存缓存就在。前端 WebView 刷新、App 重启，/resume 接口瞬间追平进度。
4. **可复用**：agent 库是独立 Go module，可被 encv-go、任何需要 AI 工具调用的 Go 应用 import。
5. **前端解耦**：Vue 不知道后端有什么工具，只根据 `Event` 流渲染 UI。换后端、换工具，前端零改动。
6. **经过验证的 UX**：UI 直接复用 `codex_web` 的高保真组件（基于官方 Codex Desktop 截图对齐），避开了从零设计带来的视觉/交互缺陷。
7. **集成入口直达主应用**：首个入口嵌在 encv-mobile 主应用首页，无需先进入 plugin-openlist 子模块才能用 AI。

---

## What Changes

### 新增

- `agent/` 顶层 Go module —— 独立可部署的 Agent 微服务（**不是 OpenList 的 in-process 库**）
  - `types.go` — `Event` / `EventType` / `ToolCallData` / `ToolResultData` / `MessageData` / **`Decision`**（4 选 1 确认决策）
  - `registry.go` — `ToolRegistry`（注册中心，线程安全）
  - `agent.go` — `Agent` 核心（`Chat` / `ConfirmTool` / `Resume` + `SessionCache` + `sessionGrants`）
  - `openai.go` — OpenAI 流式客户端（tool_calls 处理 + 多轮递归）
  - `http.go` — HTTP/SSE handlers（`/api/chat` / `/api/resume` / `/api/confirm`）—— 暴露给前端
  - `openlist_client.go` — **OpenList 定制接口 HTTP 客户端**（调用 `OPENLIST_BASE_URL/api/ext/*`）
  - `cmd/agent-demo/main.go` — 演示程序（注册 list_files / delete_file / exec_command → 转调 OpenList 定制接口）
  - `go.mod` / `README.md`
- `app/encv-mobile/src/composables/useAgent.ts` — **encv-mobile 主应用**复合式（reactive state + SSE 解析 + 断点续传 + 4-决策决策）
- `app/encv-mobile/src/components/agent/` — **encv-mobile 主应用**Vue 组件库
  - `MessageAuthor.vue` — 作者头（icon + label + meta）
  - `BlockHeader.vue` — 块头（icon + title + status badge + copy + expand）
  - `StatusBadge.vue` — 状态徽章（`ready` / `warn` / `idle` 三种 tone）
  - `CollapsedMessageToggle.vue` — 折叠消息切换器
  - `ApprovalCard.vue` — 审批卡（4 决策按钮组）
  - `GroupedOperationMessage.vue` — 连续 command/fileChange/toolOutput 折叠摘要
  - `FileChangeSummaryMessage.vue` — 文件变更摘要
  - `WebSearchSummaryMessage.vue` — Web 搜索摘要
  - `UserMessageBubble.vue` — 用户消息气泡（长文本自动折叠）
  - `MessageVirtualList.vue` — 虚拟化消息列表（>120 触发）
  - `MarkdownStream.vue` — 流式 Markdown 渲染（封装 markstream-vue）
- `app/encv-mobile/src/components/agent/AgentEntry.vue` — **首页 AI 助手入口按钮 + 弹出式 AgentChat**
- `app/encv-mobile/src/components/agent/AgentChat.vue` — 顶层聊天视图（renderTurnItems 等价实现）
- `app/encv-mobile/src/views/Home.vue` — **首页改造**：把 `AgentEntry` 放在头部 / 浮动按钮位置
- `app/encv-mobile/src/locales/{zh-CN,en-US}.json` — 新增 `agent.*` / `modals.approve*` i18n key

### 修改

- `app/encv-mobile/package.json` — 新增 `markstream-vue`、`vue-virtual-scroller` 依赖
- `app/encv-mobile/src/views/Home.vue` — 集成 `AgentEntry` 浮动按钮
- `app/encv-mobile/src/locales/{zh-CN,en-US}.json` — 新增 i18n key

### 外部仓库 PR（不在本仓库）

- `Hi-Sillot/OpenList`（独立 PR 仓库）— 添加 `/api/ext/*` 定制接口
  - `POST /api/ext/list_files` — body `{path: string}`，return `{files: [{name, size, mtime, isDir}]}`
  - `POST /api/ext/read_file` — body `{path: string, maxBytes?: int}`，return `{content: string, mime: string}`
  - `POST /api/ext/write_file` — body `{path: string, content: string}`，return `{success: bool}`
  - `POST /api/ext/delete_file` — body `{paths: string[]}`，return `{deleted: int, failed: []string}`
  - `POST /api/ext/rename` — body `{from: string, to: string}`，return `{success: bool}`
  - `POST /api/ext/exec_command` — body `{command: string, cwd: string, timeout: int}`，return `{stdout, stderr, exitCode, durationMs}`
  - `POST /api/ext/get_storage_info` — body `{}`，return `{total, used, free, files, drives}`
  - 全部走 OpenList 现有 `Authorization` token 鉴权
  - 全部加 `/api/ext/` 路径前缀方便 firewall 规则

### 影响的现有 spec

- `wire-openlist-runtime-and-ui-v2` — OpenList 集成与 UI（受影响：OpenList 需新增 /api/ext/* 端点）
- `unify-sandbox-preview-port` — preview-gateway 路由（未来加 `/agent-api/` upstream 指向 :5245）

---

## ADDED Requirements

### Requirement: Go Agent 库核心类型

`agent` 包 SHALL 提供标准化的、与前端解耦的事件流类型。

#### Scenario: Event 类型契约

- **WHEN** Agent 向 SSE channel 推事件
- **THEN** 每个事件都是 `*Event{Type: EventType, Data: string (JSON)}`
- **AND** `EventType` 取值限于：`text_delta` | `tool_call` | `tool_result` | `stream_end` | **`reasoning_delta`** | **`tool_status`**（运行中/已完成/失败三态）
- **AND** `Data` 字段是 JSON 字符串，前端按 `Type` 自行反序列化

#### Scenario: ToolCallData 字段

- **WHEN** LLM 返回 `tool_calls`
- **THEN** Agent 推 `EventToolCall`，`Data` 包含 `ToolCallData{ID, Name, Args, AutoRun, Kind}`
- **AND** `AutoRun = !def.NeedConfirm`
- **AND** `Kind` ∈ `command` | `fileChange` | `readOnly` | `unknown`（用于 ApprovalCard 图标选择）

#### Scenario: ToolResultData 字段

- **WHEN** 工具执行完成（自动或确认后）
- **THEN** Agent 推 `EventToolResult`，`Data` 包含 `ToolResultData{ID, Name, Result, IsError, Status, DurationMs}`
- **AND** `Status` ∈ `success` | `failed` | `cancelled` | `running`

#### Scenario: 4-决策 ConfirmRequest

- **WHEN** 前端发起 `POST /api/confirm`
- **THEN** body 包含 `Decision` 字段
- **AND** `Decision` 取值限于：**`accept`**（批准本次）/ **`accept_for_session`**（本轮批准，所有同类 tool 都自动放行）/ **`decline`**（拒绝并继续 LLM）/ **`cancel`**（拒绝并停止本轮）
- **AND** 与 codex_web `approvalDecisionSchema` 一一对应，避免后续集成时再做映射

---

### Requirement: OpenList 定制接口契约（Hi-Sillot/OpenList 外部 PR）

**核心原则**：OpenList 不集成 agent 库，只暴露 `/api/ext/*` 端点供 agent 服务调用。所有工具能力由 agent 服务通过 HTTP 调用 OpenList 实现。

#### Scenario: 接口清单（8 个端点）

- **WHEN** agent 服务注册工具时
- **THEN** 调用 OpenList 下列端点执行实际操作：

| 工具名 | OpenList 端点 | 入参 | 返回 | Kind |
|--------|---------------|------|------|------|
| `list_files` | `POST /api/ext/list_files` | `{path: string}` | `{files: [{name, size, mtime, isDir}]}` | `readOnly` |
| `read_file` | `POST /api/ext/read_file` | `{path, maxBytes?}` | `{content, mime}` | `readOnly` |
| `write_file` | `POST /api/ext/write_file` | `{path, content}` | `{success}` | `fileChange` |
| `delete_file` | `POST /api/ext/delete_file` | `{paths: string[]}` | `{deleted, failed}` | `fileChange` |
| `rename` | `POST /api/ext/rename` | `{from, to}` | `{success}` | `fileChange` |
| `exec_command` | `POST /api/ext/exec_command` | `{command, cwd, timeout}` | `{stdout, stderr, exitCode, durationMs}` | `command` |
| `get_storage_info` | `POST /api/ext/get_storage_info` | `{}` | `{total, used, free, files, drives}` | `readOnly` |
| `search_files` | `POST /api/ext/search_files` | `{path, pattern, maxResults?}` | `{matches: [{path, score}]}` | `readOnly` |

- **AND** 所有端点接受 OpenList 标准 `Authorization: Bearer <token>` header（沿用 OpenList 现有用户 / 管理员 token 体系）
- **AND** 所有端点返回标准 JSON，错误用 HTTP 4xx/5xx + body `{error, code, message}`

#### Scenario: agent 服务调用方式

- **WHEN** agent 收到 LLM 的 `tool_call`（如 `list_files`）
- **THEN** 调 `OpenListClient.ListFiles(path)` → 内部 `POST OPENLIST_BASE_URL/api/ext/list_files`
- **AND** 把 OpenList 返回值序列化为 JSON string 作为 tool_result
- **AND** OpenList 端点失败时返回 4xx/5xx → agent 包装为 `ToolResultData{IsError: true, Status: "failed"}`

#### Scenario: 鉴权传递

- **WHEN** agent 启动
- **THEN** 从环境变量 `OPENLIST_BASE_URL` + `OPENLIST_TOKEN` 读取配置
- **AND** 每次调 OpenList 端点时把 `OPENLIST_TOKEN` 放入 Authorization header
- **AND** 端到端鉴权链：用户 → agent 服务 → OpenList（同用户身份，OpenList 走自己的 ACL）

#### Scenario: 接口版本兼容

- **WHEN** OpenList 升级导致 `/api/ext/*` 端点契约变化
- **THEN** agent 服务可通过 `OPENLIST_API_VERSION` 环境变量选择目标版本
- **AND** 默认 v1，新接口可在 `/api/ext/v2/*` 部署

---

### Requirement: 加解密容器插件系统适配 agent

**核心原则**：加解密容器插件（encv format）通过 manifest 声明能力，agent 在启动时扫描已安装插件，把插件能力自动注册为 agent 工具。用户授权后 agent 可对加密容器进行 list / read / write / decrypt / encrypt 操作。

#### Scenario: 插件 manifest 声明 tools

- **WHEN** 加解密容器插件（如 `encv-enc-aes256`、`encv-enc-sm4`、`encv-enc-chacha20`）安装到 encv-mobile 插件目录
- **THEN** 插件 manifest（`plugin.json`）包含 `tools` 字段，数组形式声明该插件暴露给 agent 的工具
- **AND** 每个 tool 条目形如：
  ```json
  {
    "name": "encv_list_files",
    "kind": "readOnly",
    "needConfirm": false,
    "schema": { "type": "function", "function": { "name": "encv_list_files", "description": "列出加密容器内文件", "parameters": { ... } } },
    "handler": "encv.ListFiles",
    "containerTypes": ["encv-aes256", "encv-sm4"]
  }
  ```
- **AND** `handler` 是插件导出的 Go 函数（agent 通过 gRPC / IPC 调插件）

#### Scenario: 容器内文件操作工具集

- **WHEN** 插件 manifest 注册到 agent
- **THEN** agent 注册以下 5 类工具（每个插件可裁剪）：
  - `encv_list_files(containerPath, prefix?)` — 列出加密容器内文件 → `readOnly` 自动执行
  - `encv_read_file(containerPath, innerPath)` — 解密读取单个文件 → `readOnly` 自动执行
  - `encv_write_file(containerPath, innerPath, content)` — 加密写入文件 → `fileChange` **需确认**
  - `encv_decrypt_to_openlist(containerPath, innerPath, destPath)` — 解密到 OpenList 路径 → `fileChange` **需确认**
  - `encv_encrypt_from_openlist(srcPath, containerPath, innerPath)` — 从 OpenList 加密入库 → `fileChange` **需确认**

#### Scenario: agent 启动时自动注册插件工具

- **WHEN** agent 服务启动
- **THEN** agent 扫描 `OPENCV_PLUGIN_DIR` 目录加载所有插件 manifest
- **AND** 对每个 `tool` 条目，调用 `registry.Register(name, schema, handlerBridge, needConfirm, kind)`
- **AND** `handlerBridge` 是统一 adapter，agent 调用时通过 IPC 转发到对应插件
- **AND** 工具名以 `encv_` 前缀避免与 OpenList 工具重名

#### Scenario: 危险操作的 NeedConfirm 强制

- **WHEN** 插件 manifest 中 `needConfirm` 字段缺失或为 false，但 tool.kind ∈ `fileChange | command`
- **THEN** agent **强制覆盖**为 `needConfirm = true`（防止插件误配置导致破坏性操作）
- **AND** `readOnly` 类允许保持 `needConfirm = false`

#### Scenario: 插件错误隔离

- **WHEN** 插件 IPC 失败（进程崩溃、超时、返回错误）
- **THEN** agent 包装为 `ToolResultData{IsError: true, Status: "failed", Result: "{\"error\":\"plugin_unavailable\"}"}`
- **AND** 错误消息不暴露插件内部堆栈
- **AND** 单一插件故障不影响其他工具调用

#### Scenario: 工具 schema 描述容器能力

- **WHEN** LLM 收到插件工具的 schema
- **THEN** `description` 字段用中文描述工具行为 + 容器格式要求
- **AND** 例如 `encv_list_files` 的 description：「列出 encv 加密容器内的文件。容器路径形如 `/MyDrive/secrets.encv`。支持 AES-256/SM4/ChaCha20 三种格式。返回文件名、大小、修改时间。」
- **AND** LLM 据此能正确推断何时调用容器工具

---

### Requirement: 工具注册中心（ToolRegistry）

`agent` 包 SHALL 提供线程安全的工具注册中心，让应用能注册自己的 Go 原生能力。

#### Scenario: 注册工具

- **WHEN** 应用调用 `registry.Register(name, schema, handler, needConfirm, kind)`
- **THEN** 工具被存入 `tools map[string]ToolDefinition`
- **AND** `ToolDefinition = {Schema, Handler, NeedConfirm, Kind}`
- **AND** `Kind` 用于前端选择 ApprovalCard 图标（`TerminalSquare` / `FileCode2` / `ShieldCheck`）

#### Scenario: Session 级放行（accept_for_session 决策）

- **WHEN** Agent 收到 `Decision = accept_for_session`
- **THEN** 把 `(toolName, sessionID)` 加入 `sessionGrants sync.Map`
- **AND** 后续同 session 内同 `toolName` 的 tool_call 自动通过（不弹 ApprovalCard）
- **AND** 切到新 session 后清空

#### Scenario: 获取所有 Schema

- **WHEN** Agent 调用 `registry.GetAllSchemas()`
- **THEN** 返回所有注册工具的 `Schema` 切片

#### Scenario: 查找工具

- **WHEN** LLM 返回 `tool_calls` 中某个 name
- **THEN** `registry.Get(name)` 返回 `ToolDefinition, bool`
- **AND** `bool == false` 时 Agent 跳过该 tool_call（不报错，继续流）

---

### Requirement: Agent 核心（Chat 流式对话）

`agent` 包 SHALL 提供流式对话能力，支持 tool_calls 自动执行与确认挂起。

#### Scenario: 发起对话

- **WHEN** 应用调用 `agent.Chat(sessionID, messages)`
- **THEN** 返回 `(<-chan *Event, error)`
- **AND** 内部创建 `SessionCache{Events: [], IsFinished: false}` 并存入 `sessions sync.Map`
- **AND** 后台 goroutine 启动：调 OpenAI 流 → 解析 delta → 转 Event → 推 channel + 写 cache

#### Scenario: 自动执行工具（needConfirm=false）

- **WHEN** LLM delta 包含 tool_call 且 `def.NeedConfirm == false`
- **THEN** Agent 推 `EventToolCall`（AutoRun=true）
- **AND** 推 `EventToolStatus` 携带 `Status=running`
- **AND** 立即同步执行 `def.Handler(args)`，推 `EventToolStatus` 携带 `Status=success|failed`
- **AND** 推 `EventToolResult`（带 DurationMs）
- **AND** 把 tool_result 追加到 messages，递归发起下一轮 LLM 调用

#### Scenario: 需确认工具（needConfirm=true）挂起

- **WHEN** LLM delta 包含 tool_call 且 `def.NeedConfirm == true`
- **AND** 同 `(toolName, sessionID)` 不在 `sessionGrants` 中
- **THEN** Agent 推 `EventToolCall`（AutoRun=false）
- **AND** **不执行** Handler，**不追加** tool_result 到 messages
- **AND** 推 `EventStreamEnd` 结束当前 stream（挂起）
- **AND** 后续由前端调用 `ConfirmTool` 恢复

#### Scenario: 文本增量

- **WHEN** LLM delta 包含 `Content`
- **THEN** Agent 推 `EventTextDelta`，`Data` 包含 `{content: string}`
- **AND** 多个 text_delta 累积形成完整消息

#### Scenario: 推理增量（reasoning_delta）

- **WHEN** LLM delta 包含 `ReasoningContent`（OpenAI o1 系列特有字段）
- **THEN** Agent 推 `EventReasoningDelta`，`Data` 包含 `{content: string}`
- **AND** 前端用 `CollapsedMessageToggle` 默认折叠，活跃时显示动效

---

### Requirement: Agent 工具确认（ConfirmTool）— 4 决策

`agent` 包 SHALL 提供 4-决策确认恢复能力，对齐 codex_web `approvalDecisionSchema`。

#### Scenario: 确认执行（accept）

- **WHEN** 应用调用 `agent.ConfirmTool(sessionID, toolCallID, Decision=accept)`
- **THEN** 找到挂起的 `ToolDefinition`
- **AND** 同步执行 `def.Handler(args)`
- **AND** 推 `EventToolResult`（Status=success|failed）
- **AND** 追加 tool_result 到 messages，递归发起下一轮 LLM 调用
- **AND** 返回新的 `<-chan *Event`

#### Scenario: 本轮批准（accept_for_session）

- **WHEN** 应用调用 `agent.ConfirmTool(..., Decision=accept_for_session)`
- **THEN** **同时**执行 Handler **并**把 `(toolName, sessionID)` 加入 `sessionGrants`
- **AND** 后续同类 tool_call 自动放行
- **AND** 递归发起下一轮 LLM 调用

#### Scenario: 拒绝并继续（decline）

- **WHEN** 应用调用 `agent.ConfirmTool(..., Decision=decline)`
- **THEN** **不执行** Handler
- **AND** 推 `EventToolResult`（Result=`{"error":"user_rejected"}`, IsError=true, Status=cancelled）
- **AND** 追加 tool_result 到 messages（让 LLM 知道用户拒绝）
- **AND** 递归发起下一轮 LLM 调用
- **AND** 返回新的 `<-chan *Event`

#### Scenario: 拒绝并停止（cancel）

- **WHEN** 应用调用 `agent.ConfirmTool(..., Decision=cancel)`
- **THEN** **不执行** Handler
- **AND** 推 `EventToolResult`（Status=cancelled, IsError=true）
- **AND** **不递归**发起下一轮 LLM 调用（本轮终止）
- **AND** 推 `EventStreamEnd`
- **AND** 返回新的 `<-chan *Event`

---

### Requirement: 断点续传（Resume）

`agent` 包 SHALL 提供从 offset 重放事件流的能力（内存缓存，不依赖外部存储）。

#### Scenario: 续传未结束的会话

- **WHEN** 应用调用 `agent.Resume(sessionID, offset)`
- **THEN** 找到 `SessionCache`
- **AND** 启动 goroutine：从 `cache.Events[offset]` 开始，依次 push 到 channel
- **AND** 如果追到 `cache.IsFinished == true`，推 `EventStreamEnd` 后退出
- **AND** 如果还有事件在生成中（offset == len(Events)），阻塞等待，每 50ms 重试

#### Scenario: 会话不存在

- **WHEN** `sessionID` 不在 `sessions` map 中
- **THEN** 返回 `error`，前端应重置并发起新对话

---

### Requirement: HTTP/SSE 端点

`agent` 包 SHALL 提供标准化的 HTTP handlers，让任何 `net/http` 应用能挂载。

#### Scenario: POST /api/chat

- **WHEN** 前端 POST `/api/chat`，body=`{messages: [...], sessionId?: string}`
- **AND** 没有 sessionId → 生成新 UUID
- **THEN** Handler 调用 `agent.Chat(sessionId, messages)`
- **AND** 设置 `Content-Type: text/event-stream`、`Cache-Control: no-cache`、`Connection: keep-alive`
- **AND** 遍历 channel，每个 Event 序列化为 `data: {json}\n\n` 写入 ResponseWriter
- **AND** channel 关闭时关闭 ResponseWriter

#### Scenario: POST /api/resume

- **WHEN** 前端 POST `/api/resume`，body=`{sessionId, offset}`
- **THEN** Handler 调用 `agent.Resume(sessionId, offset)`
- **AND** 同 /api/chat 的 SSE 写入逻辑

#### Scenario: POST /api/confirm（4-决策）

- **WHEN** 前端 POST `/api/confirm`，body=`{sessionId, toolCallId, decision: "accept"|"accept_for_session"|"decline"|"cancel"}`
- **THEN** Handler 内部把 `accept_for_session` 映射为 `Decision = accept_for_session`
- **AND** 调用 `agent.ConfirmTool(sessionId, toolCallId, decision)`
- **AND** 同 /api/chat 的 SSE 写入逻辑

---

### Requirement: Vue 复合式（useAgent）

`useAgent.ts` SHALL 提供 reactive 状态 + SSE 解析 + 断点续传 + 4-决策决策。

#### Scenario: send() 发起对话

- **WHEN** 用户在输入框敲回车
- **THEN** `send(text)` 推 user 消息 + 空 assistant 消息到 `messages[]`
- **AND** 生成新 `sessionId = crypto.randomUUID()`
- **AND** 持久化 `{sessionId, eventOffset: 0, messages}` 到 localStorage
- **AND** `status = 'streaming'`
- **AND** fetch POST `/api/chat`，processSSE 处理响应流

#### Scenario: processSSE 解析事件

- **WHEN** SSE 流到达
- **THEN** 用 `ReadableStream.getReader()` + `TextDecoder` 按行解析
- **AND** 每行 `data: {json}` → `JSON.parse` → `Event` 对象
- **AND** `eventOffset++` 每次推后
- **AND** 按 type 分发：
  - `text_delta` → append to last assistant.content
  - `reasoning_delta` → push to last assistant.reasoning
  - `tool_call` → push to tool_calls（带 `kind` 用于图标选择）
  - `tool_status` → 标记 tool_call 的运行态（running/success/failed）
  - `tool_result` → push to tool_results
  - `stream_end` → status='idle'

#### Scenario: confirmTool 4-决策

- **WHEN** 用户在 ApprovalCard 点击 4 个按钮之一
- **THEN** `confirmTool(toolCallId, decision: 'accept' | 'accept_for_session' | 'decline' | 'cancel')` 立即调用
- **AND** `status = 'streaming'`（cancel 后会立刻收到 stream_end → status='idle'）
- **AND** fetch POST `/api/confirm`，processSSE 继续

#### Scenario: 启动时自动续传

- **WHEN** 组件 mount
- **THEN** `resume()` 从 localStorage 读 `{sessionId, eventOffset, messages}`
- **AND** 如果上次 status === 'streaming'，fetch POST `/api/resume`
- **AND** 追平进度

#### Scenario: 持久化

- **WHEN** `processSSE` 推完一个事件
- **THEN** `saveState()` 写 localStorage
- **AND** key: `agent:session:{sessionId}`

---

### Requirement: 消息作者头（MessageAuthor）

参照 codex_web `MessageAuthor({icon, label, meta})`，所有 assistant 消息 SHALL 显示统一的作者头。

#### Scenario: 作者头字段

- **WHEN** 渲染 assistant 消息
- **THEN** 顶部显示 `<MessageAuthor icon={...} label="Codex" meta={...} />`
- **AND** `icon` 视消息状态切换：默认 `<Bot size=16>`、streaming `<Brain size=16>`、error `<X size=16>`、approval `<ShieldCheck size=16>`、tool `<TerminalSquare size=16>`
- **AND** `label` 视消息类型切换：普通 `"Codex"`、plan `"计划"`、approval `"审批"`、tool `"工具"`、error `"出错"`
- **AND** `meta` 是状态文案（中文）：`正在思考` / `正在运行` / `正在编辑` / `已完成` / `失败` / `已取消`

---

### Requirement: 块头（BlockHeader）

参照 codex_web `BlockHeader({icon, title, status, statusTone, copyText, expanded, onToggleExpanded})`，所有 tool/plan/approval/error 块 SHALL 顶部带统一块头。

#### Scenario: 块头字段

- **WHEN** 渲染 tool/plan/approval/error 块
- **THEN** 顶部显示 `<BlockHeader icon={...} title={...} status={...} statusTone={...} ... />`
- **AND** 右侧 BlockActions 行包含 `CopyButton`（复制 tool 名称+结果）和 `ExpandButton`（折叠/展开详情）
- **AND** 默认折叠，只显示摘要（icon + title + status badge）
- **AND** 展开后显示完整内容（命令、diff、文件路径、输出）

#### Scenario: StatusBadge 三 tone

- **WHEN** 块头需要显示 status
- **THEN** 使用 `<StatusBadge label={...} tone="ready"|"warn"|"idle" />`
- **AND** `ready`（绿）= success / completed
- **AND** `warn`（橙）= failed / error
- **AND** `idle`（灰）= pending / unknown

---

### Requirement: 折叠消息切换器（CollapsedMessageToggle）

参照 codex_web，所有非流式输出的 assistant 消息 SHALL 默认可折叠。

#### Scenario: 折叠行为

- **WHEN** 渲染 assistant 消息 / tool 块 / plan 块 / error 块
- **THEN** 顶部使用 `<CollapsedMessageToggle icon={...} label={...} meta={...} expanded={...} onToggle={...} />`
- **AND** 默认 `expanded = false`（折叠）
- **AND** 点击切换展开/折叠
- **AND** 展开后显示完整内容

#### Scenario: 活跃动效

- **WHEN** tool 处于 `running` 状态
- **THEN** `CollapsedMessageToggle` 应用 `active` class（CSS 动画，浅灰脉冲）
- **AND** 完成/失败后移除 active class

---

### Requirement: 操作分组（GroupedOperationMessage）

参照 codex_web `GroupedOperationMessage`，连续相邻的 `command` + `fileChange` + `toolOutput` SHALL 合并为单一可折叠摘要。

#### Scenario: 分组条件

- **WHEN** 渲染 `turn.items` 时遇到连续 `command` / `fileChange` / `toolOutput`
- **THEN** 不立即渲染各 item
- **AND** 累积到 `operationGroup[]`
- **AND** 遇到非操作 item / turn 结束时调用 `flushOperationGroup(forceComplete)`
- **AND** 把整个 group 渲染为单个 `<GroupedOperationMessage :items="operationGroup" />`

#### Scenario: 文件变更特化分组

- **WHEN** `operationGroup` 全是 `fileChange` item
- **THEN** 渲染为 `<FileChangeSummaryMessage>` 而非通用 GroupedOperationMessage
- **AND** 默认折叠显示「已编辑 N 个文件」摘要
- **AND** 展开后列出文件路径 + 各文件 diff

#### Scenario: 摘要文本

- **WHEN** 渲染 group 摘要
- **THEN** 文本遵循 codex_web 规范：
  - 全是 command：`已运行 N 条命令，Xms`
  - 全是 fileChange：`已编辑 N 个文件`
  - 混合 command + fileChange：`已执行 N 个操作（X 命令 + Y 文件变更）`
  - 全是 toolOutput：`已执行 N 个工具`
- **AND** `active` class 跟随最末 item 的状态

---

### Requirement: 审批卡（ApprovalCard）— 4 决策按钮组

参照 codex_web `ApprovalCard`，所有 `needConfirm=true` 的 tool_call SHALL 通过 `ApprovalCard` 渲染（不再用「确认/取消」2 按钮）。

#### Scenario: 卡片结构

- **WHEN** 渲染 pending tool_call
- **THEN** 渲染 `<ApprovalCard :toolCall="..." :onDecide="confirmTool" />`
- **AND** 卡片结构（按 codex_web 规范）：
  - `approvalHeader`：`icon`（按 Kind 选 TerminalSquare/FileCode2/ShieldCheck）+ `title` + `reason`
  - `approvalBody`：`command` / `cwd` / `changedFiles` / `permissions` 摘要
  - `approvalFiles`（可选）：`changedFiles` 前 6 个文件路径 chip
  - `approvalDiff`（fileChange 时）：可折叠的 diff 块 + CopyButton + ExpandButton
  - `approvalActions`：**4 个决策按钮**

#### Scenario: 4-决策按钮

- **WHEN** 渲染 `approvalActions`
- **THEN** 显示 4 个按钮（顺序固定）：
  1. **`批准`**（accept）— 蓝色主按钮
  2. **`本轮批准`**（accept_for_session）— 仅当 `amendmentCount > 0`（即有 session 级授权建议）时显示
  3. **`拒绝`**（decline）— 灰色次按钮
  4. **`拒绝并停止`**（cancel）— 红色危险按钮
- **AND** 点击任一按钮时该按钮显示「处理中」并禁用其他按钮
- **AND** 按钮文案来自 i18n：`modals.approve` / `modals.approveForSession` / `modals.decline` / `modals.cancel`

---

### Requirement: 用户消息气泡（UserMessageBubble）

参照 codex_web `UserPlainText`，用户消息 SHALL 渲染为右侧纯文本气泡，不解析 Markdown。

#### Scenario: 气泡结构

- **WHEN** 渲染 user 消息
- **THEN** 右对齐 + 蓝色背景 + 圆角
- **AND** 内容为纯文本（保留换行，不解析 `**bold**`/代码块/列表）
- **AND** 不显示 MarkdownBody

#### Scenario: 长消息折叠

- **WHEN** user 消息字符数 > `USER_MESSAGE_COLLAPSE_CHAR_COUNT` (560) **或** 行数 > `USER_MESSAGE_COLLAPSE_LINE_COUNT` (9)
- **THEN** 默认折叠，显示前 N 行 + 「显示更多」toggle
- **AND** 展开后显示完整内容 + 「收起」toggle

---

### Requirement: 消息列表虚拟化（MessageVirtualList）

参照 codex_web `MESSAGE_VIRTUALIZATION_THRESHOLD = 120`，长消息列表 SHALL 自动启用虚拟滚动。

#### Scenario: 虚拟化触发

- **WHEN** `messages.length > 120`
- **THEN** 使用 `<MessageVirtualList :messages="..." :estimateSize="112" :overscan="12" />` 渲染
- **AND** Vue 等价方案：`vue-virtual-scroller` 的 `RecycleScroller`（`itemSize=112`, `minItemSize=80`, `buffer=600`）

#### Scenario: 自动滚动到底部

- **WHEN** 流式新增事件 / `status === 'streaming'`
- **THEN** `scrollToLatest('auto')` 调用 `messageVirtualizerRef.scrollToIndex(messages.length - 1, { align: 'end' })`
- **AND** 用户向上滚动查看历史时**不**打断，只在 `nearBottom === true` 时跟随

---

### Requirement: 顶层聊天视图（AgentChat.vue）

#### Scenario: 渲染流程

- **WHEN** 组件 mount / `messages` reactive 变化
- **THEN** 调用 `renderTurnItems(messages, status, options)` 产出渲染块数组
- **AND** 渲染流程与 codex_web 一致：
  1. 遍历 `messages[]`
  2. 累积相邻的 `command` / `fileChange` / `toolOutput` 到 `operationGroup`
  3. 累积相邻的 `webSearch` 到 `webSearchGroup`
  4. 遇到非操作 / 流结束时 flush group
  5. 其他类型直接渲染（user / assistant / reasoning / approval / plan / image / error / unknown）
- **AND** 输出送入 `<MessageVirtualList>` 或普通 `<v-for>`

#### Scenario: 流式 Markdown 渲染

- **WHEN** 渲染 assistant 消息正文
- **THEN** 使用 `<MarkdownStream :source="msg.content" :streaming="msg.isStreaming" />`
- **AND** 底层封装 `markstream-vue` 的 `<MarkStream>` + `dist/style.css`
- **AND** `streaming=true` 时启用代码块/表格渐进渲染

#### Scenario: 输入框

- **WHEN** `status === 'idle'`
- **THEN** 显示 ➤ 发送按钮，可输入
- **WHEN** `status === 'streaming'`
- **THEN** 输入框 disabled，显示 ⏹ 停止按钮（停止 = abort controller 关闭 SSE 连接）

---

### Requirement: 应用集成示例（OpenList 定制接口 + 独立 agent 服务）

**铁律**：OpenList 不集成 agent 库，agent 是独立 Go 微服务，通过 HTTP 调用 OpenList 暴露的 `/api/ext/*` 端点。

#### Scenario: agent 服务独立部署（:5245）

```go
// cmd/agent-demo/main.go —— 独立 Go 二进制，独立进程
package main

import (
    "os"
    "github.com/encv/agent"
)

func main() {
    // 1. 创建 OpenList 客户端
    ol := agent.NewOpenListClient(
        os.Getenv("OPENLIST_BASE_URL"),  // http://localhost:5244
        os.Getenv("OPENLIST_TOKEN"),     // admin token
    )

    // 2. 创建工具注册中心
    registry := agent.NewRegistry()
    registry.Register("list_files", schema, func(args string) (string, error) {
        var p struct{ Path string `json:"path"` }
        json.Unmarshal([]byte(args), &p)
        return ol.ListFiles(p.Path)  // HTTP POST /api/ext/list_files
    }, false, agent.KindReadOnly)
    registry.Register("delete_file", schema, func(args string) (string, error) {
        var p struct{ Paths []string `json:"paths"` }
        json.Unmarshal([]byte(args), &p)
        return ol.DeleteFiles(p.Paths)  // HTTP POST /api/ext/delete_file
    }, true, agent.KindFileChange)
    registry.Register("exec_command", schema, func(args string) (string, error) {
        var p struct {
            Command, Cwd string
            Timeout int
        }
        json.Unmarshal([]byte(args), &p)
        return ol.ExecCommand(p.Command, p.Cwd, p.Timeout)  // HTTP POST /api/ext/exec_command
    }, true, agent.KindCommand)

    // 3. 启动 Agent
    ag := agent.NewAgent(os.Getenv("OPENAI_API_KEY"), registry)

    // 4. mount HTTP handlers
    http.HandleFunc("/api/chat", ag.HandleChat)
    http.HandleFunc("/api/resume", ag.HandleResume)
    http.HandleFunc("/api/confirm", ag.HandleConfirm)
    http.ListenAndServe(":5245", nil)  // 独立端口
}
```

#### Scenario: OpenList 端只暴露接口（Hi-Sillot/OpenList PR）

```go
// Hi-Sillot/OpenList/server/handles/ext.go —— OpenList 端
package handles

// POST /api/ext/list_files
func ListFiles(c *gin.Context) {
    var req struct{ Path string `json:"path"` }
    if err := c.ShouldBindJSON(&req); err != nil { ... }
    user := c.MustGet("user").(*model.User)  // 走 OpenList 鉴权
    files, err := op.ListFiles(user, req.Path)
    c.JSON(200, gin.H{"files": files})
}
// 其他 7 个端点类似...
```

#### Scenario: 集成入口（encv-mobile 主应用首页）

```vue
<!-- app/encv-mobile/src/views/Home.vue -->
<template>
  <IonPage>
    <IonHeader>...</IonHeader>
    <IonContent>
      <!-- 现有首页内容（文件浏览 / 任务列表 等） -->
      <FileBrowser />

      <!-- 🆕 AI 助手浮动按钮 -->
      <AgentEntry />  <!-- 点击 → 弹出 AgentChat -->
    </IonContent>
  </IonPage>
</template>
```

```vue
<!-- app/encv-mobile/src/components/agent/AgentEntry.vue -->
<template>
  <button class="agent-fab" @click="openAgent">
    <IonIcon :icon="sparklesOutline" />  <!-- AI 闪亮图标 -->
  </button>
</template>

<script setup>
function openAgent() {
  // 不走路由！用 modalController 在首页弹出 AgentChat 全屏 overlay
  modalController.create({
    component: AgentChat,
    componentProps: { apiBase: '/agent-api' },  // → preview-gateway → :5245
  }).then(m => m.present())
}
</script>
```

#### Scenario: 关键约束

- **THEN** agent 服务、OpenList、encv-mobile 是 **三个独立进程**，通过 HTTP 互通
- **AND** OpenList 不感知 agent 存在，只暴露 `Authorization` 鉴权下的 `/api/ext/*` 端点
- **AND** agent 服务可热升级、灰度、AB test，与 OpenList 解耦
- **AND** encv-mobile 端只看得到 agent 服务的 `/api/chat` `/api/resume` `/api/confirm` 三个 SSE 端点
- **AND** 未来切到其他后端（如自研 encv-go），agent 服务注册表换工具实现即可，前端 0 改动

---

## MODIFIED Requirements

无（这是全新模块，不修改现有能力）

---

## REMOVED Requirements

无（保留现有 OpenList 真实后端 WebView 集成作为 fallback）

---

## 约束与限制

1. **OpenAI 依赖**：库使用 `github.com/sashabaranov/go-openai` 或类似。需要在 agent 库的 go.mod 明确。
2. **OpenList 集成边界（铁律）**：
   - **禁止**在 OpenList 源码里集成 agent 库（vendor、go.mod 引入、复制 types.go 都不允许）
   - **只**在 Hi-Sillot/OpenList 提外部 PR，添加 8 个 `/api/ext/*` 定制接口（5-7 个文件、≤ 300 行改动）
   - OpenList 端改动独立版本化（branch: `feat/ext-api-for-encv-agent`），与 encv-mobile 主仓库解耦
3. **进程边界（铁律）**：
   - `agent` 服务 = 独立 Go 二进制，端口 **:5245**（demo），生产可换
   - OpenList = 端口 **:5244**（沿用）
   - encv-mobile = 端口 **:5173**（vite dev）
   - preview-gateway = 端口 **:16000**，转发 `/agent-api/*` → `127.0.0.1:5245`
4. **session 内存缓存**：当前版本 session 存在 Go 进程内存，进程重启即丢。生产环境建议加 Redis / SQLite 持久化（v2 任务）。
5. **单个 session 只能串行**：ConfirmTool 调用时假设 session 处于挂起态，并发调用需加锁。
6. **沙箱 dev 阶段**：agent demo（`cmd/agent-demo/main.go`）独立运行在 :5245，便于前端联调。
7. **UI 必须 1:1 复用 codex_web 模式**：所有组件命名、props 形状、视觉 token 与 codex_web 一致（中文文案、状态文案、按钮顺序固定），便于未来 codex_web → ENCV 组件库复用代码。
8. **集成入口（铁律）**：AI 助手入口**只在 encv-mobile 主应用首页**（`Home.vue`）首次出现，不在 plugin-openlist 路由下，不在 Tasks tab 下，不在单独路由。入口是 `AgentEntry` 浮动按钮 → 弹 modal → 全屏 AgentChat。

---

## 与 codex_web 的对应关系速查表

| codex_web 概念 | 本 spec 对应 | 备注 |
|----------------|--------------|------|
| `ApprovalDecision = "accept" \| "acceptForSession" \| "decline" \| "cancel"` | `Decision` 4 选 1（`accept` / `accept_for_session` / `decline` / `cancel`） | Go 字段用 snake_case 兼容 OpenAPI |
| `MessageAuthor({icon, label, meta})` | `<MessageAuthor>` | 完全相同的 props |
| `BlockHeader({icon, title, status, statusTone, copyText, expanded, onToggleExpanded})` | `<BlockHeader>` | 完全相同的 props |
| `StatusBadge({label, tone})` tone ∈ `ready`/`warn`/`idle` | `<StatusBadge>` | 完全相同 |
| `CollapsedMessageToggle({icon, label, meta, expanded, active, onToggle})` | `<CollapsedMessageToggle>` | 完全相同 |
| `GroupedOperationMessage({items, forceComplete, ...})` | `<GroupedOperationMessage>` | 完全相同 |
| `FileChangeSummaryMessage` | `<FileChangeSummaryMessage>` | 完全相同 |
| `WebSearchSummaryMessage` | `<WebSearchSummaryMessage>` | 完全相同 |
| `renderTurnItems(items, turnStatus, options)` | `renderTurnItems()` 组合式 | 行为完全一致 |
| `MESSAGE_VIRTUALIZATION_THRESHOLD = 120` | `messages.length > 120` | 完全相同 |
| `USER_MESSAGE_COLLAPSE_CHAR_COUNT = 560` | 560 字符 | 完全相同 |
| `USER_MESSAGE_COLLAPSE_LINE_COUNT = 9` | 9 行 | 完全相同 |
| `useVirtualizer` from `@tanstack/react-virtual` | `RecycleScroller` from `vue-virtual-scroller` | 等价能力 |
| `i18n` zh-CN / en-US | zh-CN / en-US | 完全相同 |
| `approvalCard` / `approvalHeader` / `approvalBody` / `approvalActions` | 同名 CSS class | 完全相同 |
| token `var(--color-bg-elevated)` 等 | 同样的 CSS variable | 在 `app/encv-mobile/src/styles/tokens.css` 中定义 |
| codex_web 路由 `/` → App.tsx | encv-mobile `Home.vue` → 浮动按钮 + modal | 入口位置不同（首页 pop modal） |

---

## 三个进程的全景图

```
┌────────────────────────────────────────────────────────────────────┐
│  encv-mobile 主应用（:5173 vite dev）                              │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  Home.vue（主应用首页）                                       │  │
│  │  ┌──────────────────┐  ┌──────────────────────────────────┐  │  │
│  │  │ FileBrowser      │  │ <AgentEntry />  ← 浮动 AI 按钮   │  │  │
│  │  │ （现有内容）      │  │                                  │  │  │
│  │  │                  │  │ 点击 → modalController.create(   │  │  │
│  │  │                  │  │   AgentChat,                     │  │  │
│  │  │                  │  │   {apiBase: '/agent-api'}        │  │  │
│  │  │                  │  │ )                                │  │  │
│  │  └──────────────────┘  └──────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────────────────┘  │
│         │ fetch POST /agent-api/api/chat (SSE)                     │
│         │ fetch POST /agent-api/api/confirm (SSE)                  │
│         ▼                                                           │
│  preview-gateway（:16000）                                          │
│         │ 路径 /agent-api/* → proxy_pass http://127.0.0.1:5245      │
│         ▼                                                           │
│  agent 服务（:5245，独立 Go 二进制）                                │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  POST /api/chat  → agent.Chat(sessionId, messages)          │  │
│  │  POST /api/resume → agent.Resume(sessionId, offset)         │  │
│  │  POST /api/confirm → agent.ConfirmTool(sessionId, tcID, dec)│  │
│  └──────────────────────────────────────────────────────────────┘  │
│         │ Tool handler: OpenListClient.ListFiles(path)             │
│         │ HTTP POST http://127.0.0.1:5244/api/ext/list_files        │
│         │ Header: Authorization: Bearer ${OPENLIST_TOKEN}           │
│         ▼                                                           │
│  OpenList（:5244，Hi-Sillot fork，外部 PR 加 8 个端点）            │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  POST /api/ext/list_files  ──┐ 走 OpenList 现有 ACL          │  │
│  │  POST /api/ext/read_file    │ 用户鉴权 → 业务处理            │  │
│  │  POST /api/ext/write_file   │                               │  │
│  │  POST /api/ext/delete_file  │ 改 ≤ 300 行                    │  │
│  │  POST /api/ext/rename       │ 5-7 个新文件                   │  │
│  │  POST /api/ext/exec_command │ 不引入 agent 依赖              │  │
│  │  POST /api/ext/get_storage_info                              │  │
│  │  POST /api/ext/search_files ─┘                               │  │
│  └──────────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────┘
```

**关键不变量**：
- encv-mobile **不**直接连 OpenList 的 `/api/ext/*`（统一走 agent 服务中转）
- encv-mobile **不**直连 OpenAI（agent 服务代理，apiKey 不下发前端）
- OpenList **不**感知 agent 服务存在（无耦合，可独立升级）
- agent 服务是**唯一**连接 OpenAI 和 OpenList 的组件
