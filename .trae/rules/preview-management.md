# 沙箱预览服务管理铁律

> **核心原则：所有 dev/preview 进程必须由 pm2 监管，禁止在工具调用中用 blocking / nohup & / sleep 长跑方式启动。**
> **任何"等用户访问"的端口都必须先 pm2 守护，工具调用立即返回。**

---

## 一、三大反模式（2026-06-05 用户痛批后写入规则）

### 反模式 A：`sleep 86400` / 任意大数 sleep 阻塞会话

**症状**：用 `RunCommand blocking=true` 跑 `sleep 86400`、`sleep infinity`、`tail -f xxx` 之类命令"占位"。

**根因**：把"长跑进程"和"工具命令生命周期"混为一谈。RunCommand 一旦 blocking 就会等命令完成，阻塞整个会话，浪费 token 配额。

**禁止**：
- ❌ `sleep 86400`、`sleep 999999`、`sleep infinity` —— **任何大于 60s 的 sleep 都不允许出现在 blocking 命令中**
- ❌ `tail -f /var/log/xxx` 阻塞等待
- ❌ `node server.js` 阻塞运行（无 daemon 化）

**正确做法**：
- ✅ 用 `pm2 start xxx.js --name xxx` 守护（fork 模式，立刻 daemon 化返回）
- ✅ 如需长跑日志观察，用 `pm2 logs <name>` 短时查看（30s 内 kill）
- ✅ 如需让某个 web 端口"保持在线"以供 OpenPreview，**先 pm2 守护** → 再让工具调用 `curl` 健康检查

---

### 反模式 B：`nohup xxx > /tmp/log 2>&1 &` 启动后台进程

**症状**：用 `nohup ... &` 在 RunCommand 末尾 `&` 一扔，制造"无主"的后台进程。

**根因**：bash 父进程退出后，子进程变孤儿由 init 收养；无人监管、死了无人重启、日志散落、端口冲突无法溯源。

**禁止**：
- ❌ `nohup node server.js > /tmp/x.log 2>&1 &`
- ❌ `nohup ./node_modules/.bin/vite ... &`
- ❌ `setsid xxx &`

**正确做法**：
- ✅ 全部走 `pm2 start <script> --name <name>`，自带日志文件（`/tmp/pm2-<name>.log`）、自动重启、内存监控、状态查询

---

### 反模式 C：阻塞 + web_server 类型假装常驻

**症状**：用 `command_type: web_server, blocking: true` 跑 `node server.js`，命令一直不返回。

**根因**：web_server 命令是给 OpenPreview 工具注册端口用的「在线占位」。但**真正提供 web 服务的进程**必须由 pm2 守护，**不能用 RunCommand 自身当 server**。

**禁止**：
- ❌ `node server.js` (blocking, command_type=web_server)
- ❌ 任何 RunCommand 自身的进程扮演"长跑 web server"

**正确做法**：
- ✅ 真实 server 进程：先 `pm2 start` 起来
- ✅ OpenPreview 注册：另起一个 `command_type=web_server, blocking=true` 的 RunCommand（如 `while true; do curl -s http://localhost:15003/health; sleep 30; done`），它**循环**健康检查**已经在跑的** server → 工具会把这个命令的 command_id 当作 web_server 来源

---

## 二、pm2 联动启动标准流程

### 2.1 沙箱 dev 服务拓扑（本项目固定 4 上游）

| pm2 app | 端口 | 必备 | 角色 |
|---------|------|------|------|
| `preview-gateway` | :16666 | ✅ 必备 | 统一预览网关，对外唯一入口 |
| `encv-mobile-vite` | :8100 | ✅ 必备 | 主 app Vite（被 :16666/ 代理） |
| `preview-helper` | :15002 | ✅ 必备 | OpenPreview 占位 |
| `openpreview-stub` | :15003 | ✅ 必备 | OpenPreview web_server command_id 源 |
| `start-preview` | :2025 + :5173 | ⚠️ 可选 | Go 后端 air + 旧 vite；现已拆出独立进程 |
| `openlist` | :5244 | ⚠️ 可选 | OpenList 真实 fork Go 服务 |
| `plugin-openlist-vite` | :5174 | ⚠️ 可选 | OpenList 管理 UI Vite（被 :16666/openlist-ui/ 代理） |

**主 app 入口最少依赖**：`preview-gateway` + `encv-mobile-vite` + `preview-helper` + `openpreview-stub`，其余按需。

### 2.2 启动标准命令

```bash
# 1. 装 pm2（一次性）
npm install -g pm2

# 2. 启动主 app 4 件套（来自 ecosystem.config.cjs）
pm2 start /workspace/ecosystem.config.cjs \
  --only preview-gateway,encv-mobile-vite,preview-helper

# 3. 额外启 openpreview-stub（该 app 不在 ecosystem 里，独立维护）
PORT=15003 pm2 start /workspace/scripts/openpreview-stub.js \
  --name openpreview-stub

# 4. 保存状态（sandbox 会话重置后可 pm2 resurrect 恢复）
pm2 save
```

### 2.3 完整命令参考

```bash
# 状态
pm2 list
pm2 jlist                                       # JSON 格式
pm2 prettylist                                  # 含 cwd/args/内存/重启次数

# 日志
pm2 logs preview-gateway --lines 100
pm2 logs --nostream --raw 0 0 100               # 拉最近 100 行非流

# 启停
pm2 start <script> --name <name>                # 启
pm2 restart <name>                              # 重启
pm2 stop <name>                                 # 停（保留 pm2 注册表）
pm2 delete <name>                               # 删（出注册表）
pm2 delete all                                  # 全删

# 配置
pm2 save                                        # 写 /root/.pm2/dump.pm2
pm2 resurrect                                   # 从 dump 恢复
pm2 reload ecosystem                            # 0 秒重载
```

### 2.4 验证命令

```bash
# 端口在线
lsof -i :16666 -i :8100 -i :15003 | head

# 主 app 入口可达
curl -sI http://localhost:16666/ | head -1
# 期望: HTTP/1.1 200 OK

# preview-gateway health（如果其他 upstream 未启，会 503；不影响 / 路径）
curl -s http://localhost:16666/__gateway/health | head -c 200
```

---

## 三、OpenPreview 激活（拿到外网链接）

### 3.1 原理

```
外网用户浏览器
  ↓
:16000 (agent-tool-host)  ← 唯一外网入口
  ↓ 内部 preview-proxy
:16666 (preview-gateway)   ← 沙箱内统一入口
  ↓
:8100 (encv-mobile-vite)   ← 主 app
```

**首次访问 :16666** → agent-tool-host 内部 `:80` register 端点 `requires_auth=true`，**普通 HTTP 请求 401 拒绝**。
只有 `OpenPreview` 工具（IDE 内部命令）能完成 register。

### 3.2 标准激活流程

```bash
# Step 1: 准备 web_server 类型 command 来源
#   openpreview-stub 已经在 :15003（pm2 守护），但工具历史里需要
#   一个 command_type=web_server 的命令作为「来源」。

# Step 2: 启动 web_server 类型 RunCommand（阻塞但**不浪费**——它循环探测已运行的 server）
#   命令示例:
#     while true; do
#       curl -s http://localhost:15003/ > /dev/null || exit 1
#       sleep 5
#     done
#   command_type=web_server, blocking=true
#   工具会把这个命令当作 web_server，拿到 command_id

# Step 3: 调用 OpenPreview 工具
#   OpenPreview(
#     command_id="<Step 2 的 command_id>",
#     preview_url="http://localhost:16666/"
#   )
#   完成 :16666 在 agent-tool-host 的注册
```

### 3.3 错误模式

| 错误 | 原因 | 修复 |
|------|------|------|
| `OpenPreview` 返回 401 | 命令不是 web_server 类型 | 用 `command_type: web_server` 重启 |
| `OpenPreview` 返回 port already registered | 上一次注册过；先取消 | 检查 agent-tool-host 内部 preview-proxy 状态 |
| `curl :16666/` 返回 502 | preview-gateway 上游不可达 | `pm2 list` 看 vite 是否 online；`curl :8100` 直连验证 |
| `curl :16666/` 200 但浏览器白屏 | vite :8100 死了 | `pm2 restart encv-mobile-vite` |

---

## 四、禁止命令清单（速查）

| 模式 | 反例 | 替代 |
|------|------|------|
| `sleep N` (N>60s) blocking | `sleep 86400` | `pm2 start xxx` 后立刻返回 |
| `tail -f xxx` blocking | `tail -f /tmp/x.log` | `pm2 logs xxx --lines 100` |
| `nohup xxx &` | `nohup node s.js &` | `pm2 start s.js --name xxx` |
| `setsid xxx` | `setsid vite &` | `pm2 start vite --name xxx` |
| `node server.js` blocking | 直接跑 node | `pm2 start server.js --name xxx` |
| `vite --port N &` blocking | `vite --port 8100 &` | `pm2 start vite.js --name encv-mobile-vite -- --port 8100` |
| 任何 `&` 启后台 | `cmd &` | `pm2 start` |

---

## 五、强制自检清单

每次启动 dev 服务前必须确认：

- [ ] pm2 已装（`which pm2`）—— 未装则 `npm install -g pm2`
- [ ] ecosystem.config.cjs 已存在（`/workspace/ecosystem.config.cjs`）
- [ ] 启动命令是 `pm2 start ...`，**不是** `nohup`、`&`、`sleep` 阻塞
- [ ] 启动后 `pm2 list` 看到目标 app `online`
- [ ] `curl :16666/` 返回 200（fallthrough 到 vite）
- [ ] `pm2 save` 持久化（sandbox 会话重置可 `pm2 resurrect`）

---

## 六、相关 spec / 文件

| 文件 | 作用 |
|------|------|
| [/workspace/ecosystem.config.cjs](file:///workspace/ecosystem.config.cjs) | pm2 完整配置（4 主 + 3 辅） |
| [/workspace/scripts/previews.sh](file:///workspace/scripts/previews.sh) | pm2 启停包装（start/stop/restart/status/logs/monit/kill） |
| [/workspace/scripts/openpreview-stub.js](file:///workspace/scripts/openpreview-stub.js) | OpenPreview web_server command_id 源 |
| [/workspace/.preview-helper.js](file:///workspace/.preview-helper.js) | 早期占位（被 openpreview-stub 取代） |
| [/workspace/app/preview-gateway/README.md](file:///workspace/app/preview-gateway/README.md) | 网关 + 路由 + 健康检查文档 |
| [/workspace/.trae/specs/unify-sandbox-preview-port/spec.md](file:///workspace/.trae/specs/unify-sandbox-preview-port/spec.md) | 端口决策 D1-D9 原始 spec |
