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

### 反模式 D：`pnpm build` + `pnpm preview` 制造孤儿前端（2026-06-07 mobile mock preset 验证事故）

**症状**：用户说"重建前端并给我预览链接"时，擅自跑：
1. `pnpm build` — 跑 vue-tsc + vite build 完整构建
2. `pnpm preview --port 4173` — 启动 vite 自带静态预览服务

**根因（连环误判）**：
1. 错把 `pnpm build` 当成"重建前端"标准做法 — 实际 `build` 产物 `dist/` 是给 **Android 离线包**用的（Capacitor sync → android/app/src/main/assets/public/），**不是 dev 预览**
2. 错把 vite 自带 `preview` 当成"预览链接"通用方案 — vite preview 是**纯静态服务**，不走 vite dev 插件链（@ionic/vue、Capacitor polyfill、HMR 客户端）→ 前端能打开但**调不到任何 `/api/`** → 用户看到的"无错白屏 + 工具调用失败"是孤儿前端症状
3. 错以为"vite preview"是 Capacitor + Ionic Vue 项目的合法入口 — 实际本项目 preview 链路是**已 pm2 守护的 2 件套**（preview-gateway:16666 内部统一管理 Go 后端 :2025 + Vite :8100），OpenPreview 直接打 16666 就行
4. 思考过程没核对 §二 沙箱 dev 服务拓扑表，假设 vite 自己 serve 出来就等于项目预览

**禁止**：
- ❌ `cd app/encv-mobile && pnpm build`（除非用户明确说"打 Android 包 / Capacitor sync"）
- ❌ `cd app/encv-mobile && pnpm preview`（vite 自带静态预览，绕开项目管控链路）
- ❌ 任何 `vite --port 4xxx` / `vite preview --port 4xxx`（非 8100 端口都脱离项目管控）
- ❌ 任何 `npx serve dist` / `python3 -m http.server dist` / `npx http-server dist`（同样孤儿）

**正确做法（用户说"重建前端 / 给我预览链接"时）**：
- ✅ **不** build：vite dev (8100) 跑源码 + HMR，源码改动已自动热重载
- ✅ **不**启新进程：preview-gateway (16666) 已在 pm2 守护（除非 `pm2 list` 显示 offline 才需要 `pm2 restart`）
- ✅ Preview 链接直接用：**http://localhost:16666/**
- ✅ 调用 OpenPreview 工具展示 16666 链接（command_id 取最近一次 `curl -sI :16666/` 健康检查的 RunCommand 即可）
- ✅ 如真要"完整重打"前端（如 Capacitor sync 场景），必须先和用户确认目标，并明确告知 `pnpm build` 不会重启 dev 服务

**反查清单（用户说 preview/build/dev 时，先问自己）**：
- [ ] 用户说的"预览"是 web dev 预览还是 Android 离线包？
- [ ] pm2 list 看过吗？2 件套 (preview-gateway + openpreview-stub) 是否都在 online？
- [ ] 我要启的端口在 §二 拓扑表里吗？（16666 / 15003；2025/8100/5174/5244 由 gateway 按需拉起）
- [ ] 我要跑的命令在 §四 禁止命令清单里吗？

---

## 二、pm2 联动启动标准流程（方案 C：网关合一，2026-06-08 大改）

### 2.1 沙箱 dev 服务拓扑（pm2 监管 2 个 + gateway 内部管理 4 个子进程）

| pm2 app | 端口 | 必备 | 角色 |
|---------|------|------|------|
| `preview-gateway` | :16666 | ✅ 必备 | 统一预览网关，**唯一对外入口 + 唯一进程管理者** |
| `openpreview-stub` | :15003 | ✅ 必备 | OpenPreview web_server command_id 源（无法绕开，详见 §三） |

**gateway 内部子进程**（由 `preview-gateway` 自己 `child_process.spawn` 管理，**不需独立 pm2 app**）：

| 子进程 | 端口 | 默认 | 角色 | 开关 env |
|--------|------|------|------|----------|
| `encv-go` (air) | :2025 | ✅ 启用 | Go 后端（mobile overlay 关键） | `SPAWN_GO=0` 关闭 |
| `encv-mobile-vite` | :8100 | ✅ 启用 | 主 app Vite（被 :16666/ 代理） | `SPAWN_VITE=0` 关闭 |
| `plugin-openlist-vite` | :5174 | ❌ 按需 | OpenList 管理 UI Vite | `SPAWN_PLUGIN_VITE=1` 启用 |
| `openlist` | :5244 | ❌ 按需 | OpenList 真实 fork Go 服务 | `SPAWN_OPENLIST=1` 启用 |

**为什么只有 2 个 pm2 app？**（方案 C 解决的问题）
- **历史 7 app 架构**：`preview-gateway` + `encv-mobile-vite` + `preview-helper` + `openpreview-stub` + `start-preview` + `plugin-openlist-vite` + `openlist`
- **核心 bug**：`start-preview` 内部 & 启 vite :8100 与 `encv-mobile-vite` 抢同一端口；`preview-helper` 与 `openpreview-stub` 功能完全重复
- **方案 C**：所有 dev 进程下放到 `preview-gateway` 内部（src/children.ts）；pm2 只管 2 个真正长跑的 app
- **子进程死 → gateway 死 → pm2 重启整套**（避免出现 "vite 死、Go 活、gateway 200、用户看到白屏" 的鬼状态）

### 2.2 启动标准命令（**只此一条**）

```bash
# 1. 装 pm2（一次性，setup-sandbox-env.sh 已做）
which pm2 || npm install -g pm2

# 2. 一行启动全部（2 个 app；其余由 preview-gateway 内部 spawn）
pm2 start /workspace/ecosystem.config.cjs

# 3. 验证
pm2 list                                              # 看到 2 个 online
curl -s http://localhost:16666/__gateway/health       # ok:true
curl -sI http://localhost:16666/                      # HTTP/1.1 200 OK
curl -s http://localhost:16666/api/service-guard | jq '.context.envDevPreview'   # true
```

**如果需要 plugin-vite / openlist**（按需）：
```bash
# 临时：单次启动前注入 env
SPAWN_PLUGIN_VITE=1 pm2 restart preview-gateway
SPAWN_OPENLIST=1    pm2 restart preview-gateway

# 永久：编辑 ecosystem.config.cjs 把对应 env 改 '1'
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
pm2 start /workspace/ecosystem.config.cjs       # 启全部
pm2 restart preview-gateway                     # 改 gateway 配置（含 spawn 子进程重启）
pm2 restart openpreview-stub                    # 改 stub
pm2 stop all && pm2 delete all                  # 全清
pm2 start <script> --name <name>                # 临时启单 app（不推荐，破坏单一管理）

# 配置
pm2 save                                        # 写 /root/.pm2/dump.pm2
pm2 resurrect                                   # 从 dump 恢复
pm2 reload ecosystem                            # 0 秒重载
```

### 2.4 验证命令

```bash
# 端口在线（只需检查 16666 / 15003；2025/8100 由 gateway 内部管理）
lsof -i :16666 -i :15003 | grep LISTEN

# 主 app 入口可达
curl -sI http://localhost:16666/ | head -1
# 期望: HTTP/1.1 200 OK

# preview-gateway health（含子进程状态）
curl -s http://localhost:16666/__gateway/health | jq '{ok, children: [.children[].name], optionalDown: .optionalDown | length}'
# 期望: {"ok": true, "children": ["encv-go", "encv-mobile-vite"], "optionalDown": 2}
#  optionalDown 数 = 2 是因为 plugin-vite + openlist 按需未启（这是预期）
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
| `curl :16666/` 返回 502 | preview-gateway 子进程（encv-go / encv-mobile-vite）未就绪 | `curl :16666/__gateway/health | jq .children` 看哪个没 ready；`pm2 logs preview-gateway --lines 100` |
| `curl :16666/` 200 但浏览器白屏 | encv-mobile-vite 子进程死了 | `pm2 restart preview-gateway`（整组重启） |

---

## 四、禁止命令清单（速查）

| 模式 | 反例 | 替代 |
|------|------|------|
| `sleep N` (N>60s) blocking | `sleep 86400` | `pm2 start xxx` 后立刻返回 |
| `tail -f xxx` blocking | `tail -f /tmp/x.log` | `pm2 logs xxx --lines 100` |
| `nohup xxx &` | `nohup node s.js &` | `pm2 start s.js --name xxx` |
| `setsid xxx` | `setsid vite &` | `pm2 start vite --name xxx` |
| `node server.js` blocking | 直接跑 node | `pm2 start server.js --name xxx` |
| `vite --port N &` blocking | `vite --port 8100 &` | `pm2 restart preview-gateway`（vite 由 gateway 内部 spawn，不需手起） |
| 任何 `&` 启后台 | `cmd &` | `pm2 start` |
| **`pnpm build`（误当作 dev 预览）** | `pnpm build` | vite dev (8100) 跑源码 + HMR，**不 build** |
| **`pnpm preview`（vite 静态预览，孤儿前端）** | `pnpm preview --port 4173` | preview-gateway (16666) → vite dev (8100) → Go 后端 |
| **任意 `npx serve dist` / `http-server dist`** | 静态文件服务绕开 API 反代 | preview-gateway (16666) 是唯一合法入口 |

---

## 五、env 注入铁律（2026-06-05 mobile overlay 触发失败事故后写入；2026-06-08 方案 C 重写）

> **核心原则：`ApplyMobileOverlay` 由 `ENCV_MOBILE=1` 或 `ENCV_DEV_PREVIEW=1` 触发，缺失则 servingDir 退回 `/workspace`（用户看到 `.md` / `.gitignore`，不是 mock 媒体）。**

### 5.1 三层注入（缺一不可）

| 层 | 文件 | 作用 |
|----|------|------|
| **L1 pm2 → gateway** | `ecosystem.config.cjs` `preview-gateway` 块 `env` | pm2 fork 时注入到 gateway Node 进程 |
| **L2 gateway → air 子进程** | `app/preview-gateway/src/server.ts` `buildChildSpecs()` | gateway `child_process.spawn` air 时透传 env |
| **L3 air → encv 传递** | `.air-run.sh` `export ${X:-1}` 兜底 | air rebuild 重启 ./tmp/encv 时不会丢 env |

**数据流**：
```
ecosystem.config.cjs  (L1: ENCV_DEV_PREVIEW=1 / ENCV_MOBILE=1)
  ↓ pm2 start
preview-gateway 进程  (Node，环境变量在 process.env)
  ↓ buildChildSpecs() spread process.env + defaults  (L2)
air 子进程  (bash，环境变量在 air shell)
  ↓ air exec
.air-run.sh  (export ${X:-1} 兜底  L3)
  ↓ exec
./tmp/encv start  (Go，env 在 os.Getenv)
  ↓ config.Load() → ApplyMobileOverlay 触发
```

**为什么 L2 显式 spread 不省略**：
- `process.env` 在 Node 里有但**不会自动**被子进程继承 — 必须用 `spawn(cmd, args, { env: ... })` 显式传递
- `buildChildSpecs` 里 `env: { ...process.env, ENCV_*: '1' }` 强制覆盖，避免用户在父 shell 里设了 `ENCV_MOBILE=0` 导致冲突

**为什么不在 `start-preview.sh` 设 inline env `ENCV_DEV_PREVIEW=1 air &`**：
- start-preview.sh 已**删除**（方案 C 大改，脚本退化为只跑 mock 生成）
- inline env 只对 `air` 进程有效，但 air rebuild 时**不一定**透传给新的 `./tmp/encv`（air 0.x 行为）
- L1 + L2 + L3 三层防御才是稳定来源

### 5.2 自检命令

```bash
# 1. service-guard 必须返 envDevPreview=true
curl -s http://localhost:16666/api/service-guard | jq '.context.envDevPreview'
# 期望：true

# 2. servingDir 必须是 /storage/emulated/0
curl -s http://localhost:16666/api/service-guard | jq '.context.servingDir'
# 期望："/storage/emulated/0"

# 3. mock 数据落地
ls /storage/emulated/0/01-plain-media/ | head
# 期望：01.mp4 02.mp3 03.png 04.pdf 05.txt ...

# 4. gateway children 状态（确认 air 在跑）
curl -s http://localhost:16666/__gateway/health | jq '.children[].name'
# 期望：["encv-go", "encv-mobile-vite"]
```

**自检失败排查表**：

| 现象 | 原因 | 修复 |
|------|------|------|
| `envDevPreview: false` | env 没传到 encv | 检查 L1（pm2 env）+ L2（gateway buildChildSpecs）+ L3（.air-run.sh） |
| `servingDir: "/workspace"` | mobile overlay 没触发 | 同上 |
| `servingDirExists: false` | mock 没生成 | `gateway src/preflight.ts:ensureMockData` 兜底 |
| air rebuild 后 env 变 false | L3 兜底缺失 | 检查 `.air-run.sh` 末尾 export |
| gateway children 缺 encv-go | air 启动失败 / 90s 就绪超时 | `pm2 logs preview-gateway --lines 100` 看错误 |

### 5.3 绝对禁止

- ❌ 移除 `.air-run.sh` 的 `export ${X:-1}` 兜底（被前人坑过：air rebuild 丢 env）
- ❌ 在 `ecosystem.config.cjs` `preview-gateway` 块 env 里**不设** `ENCV_DEV_PREVIEW` / `ENCV_MOBILE`
- ❌ 在 `src/children.ts` `buildChildSpecs` 里删掉 `{ ...process.env, ENCV_*: ... }` 显式 spread
- ❌ 让 `./tmp/encv` 直接以 pm2 启动，绕过 air 监视
- ❌ 复活 start-preview.sh 里的 inline env 注入（方案 C 已删）

---

## 六、强制自检清单

每次启动 dev 服务前必须确认：

- [ ] pm2 已装（`which pm2`）—— 未装则 `npm install -g pm2`
- [ ] ecosystem.config.cjs 已存在（`/workspace/ecosystem.config.cjs`）且 `preview-gateway/dist/server.js` 已构建（`pnpm build`）
- [ ] 启动命令是 `pm2 start /workspace/ecosystem.config.cjs`，**不是** `nohup`、`&`、`sleep` 阻塞
- [ ] 启动后 `pm2 list` 看到 **2 个** app `online`（preview-gateway + openpreview-stub）
- [ ] **`curl :16666/__gateway/health | jq .ok` = true**（必检 upstream 全 alive）
- [ ] `curl :16666/` 返回 200（fallthrough 到 vite :8100）
- [ ] **`curl :16666/api/service-guard | jq .context.envDevPreview` = true**（mobile overlay 生效）
- [ ] **`curl :16666/api/service-guard | jq .context.servingDir` = /storage/emulated/0**
- [ ] `pm2 save` 持久化（sandbox 会话重置可 `pm2 resurrect`）

**当用户说"重建前端 / 给我预览链接 / dev server 起一下"时，先过这一关**：

- [ ] 我**没有**在用 `pnpm build` 吗？（除非用户明确说"打 Android 包 / Capacitor sync"）
- [ ] 我**没有**在用 `pnpm preview` / `vite preview` / `npx serve dist` 吗？
- [ ] 我要启的端口在 §二 拓扑表（16666 / 8100 / 15002 / 15003 / 2025）吗？
- [ ] `pm2 list` 显示 preview-gateway + encv-mobile-vite + Go 后端都 online 吗？
- [ ] 如果都已 online，我**根本不需要启任何进程** — 直接给链接 http://localhost:16666/
- [ ] OpenPreview 调用过了吗？（不是只口头说链接，要真正调工具）

---

## 七、相关 spec / 文件

| 文件 | 作用 |
|------|------|
| [/workspace/ecosystem.config.cjs](file:///workspace/ecosystem.config.cjs) | pm2 完整配置（4 主 + 3 辅） |
| [/workspace/.air-run.sh](file:///workspace/.air-run.sh) | air rebuild 时 env 兜底 export |
| [/workspace/scripts/previews.sh](file:///workspace/scripts/previews.sh) | pm2 启停包装（start/stop/restart/status/logs/monit/kill） |
| [/workspace/scripts/openpreview-stub.js](file:///workspace/scripts/openpreview-stub.js) | OpenPreview web_server command_id 源 |
| [/workspace/.preview-helper.js](file:///workspace/.preview-helper.js) | 早期占位（被 openpreview-stub 取代） |
| [/workspace/app/encv-mobile/scripts/start-preview.sh](file:///workspace/app/encv-mobile/scripts/start-preview.sh) | 主预览脚本（mock 生成 + air 启动） |
| [/workspace/app/preview-gateway/README.md](file:///workspace/app/preview-gateway/README.md) | 网关 + 路由 + 健康检查文档 |
| [/workspace/.trae/specs/unify-sandbox-preview-port/spec.md](file:///workspace/.trae/specs/unify-sandbox-preview-port/spec.md) | 端口决策 D1-D9 原始 spec |
| [/workspace/.trae/documents/plan-fix-sandbox-preview-env-injection.md](file:///workspace/.trae/documents/plan-fix-sandbox-preview-env-injection.md) | 本次 env 注入修复 plan（2026-06-05） |
| [/workspace/internal/config/config.go](file:///workspace/internal/config/config.go) | `ApplyMobileOverlay` 触发条件：`ENCV_MOBILE=1 \|\| ENCV_DEV_PREVIEW=1`（L292-294） |
