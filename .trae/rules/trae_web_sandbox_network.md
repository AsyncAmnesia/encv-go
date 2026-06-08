# Trae Web 沙箱网络行为

> 诊断时间：2026-05-27 | 环境：CI=true, non-interactive sandbox

---

## 一、沙箱网络架构总览

```
┌─────────────────────────────────────────────┐
│              Trae Web Sandbox                │
│                                             │
│  ┌──────────┐   ┌─────────────────────┐     │
│  │ curl/wget│──▶│ 出站 TCP 白名单放行  │     │
│  └──────────┘   └─────────────────────┘     │
│                                             │
│  ┌──────────┐   ┌────────────────────┐      │
│  │ Node.js  │──▶│ NODE_OPTIONS 注入  │      │
│  │ + undici │   │ EnvHttpProxyAgent  │      │
│  └──────────┘   │ HTTP CONNECT →     │      │
│                 │ :18080 (MCP代理)    │      │
│                 └────────┬───────────┘      │
│                          │                  │
│                 ┌────────▼───────────┐      │
│                 │ 127.0.0.1:18080    │      │
│                 │ (MCP Proxy)        │      │
│                 │ 本地回环端口开放    │      │
│                 └────────────────────┘      │
│                                             │
│  ┌──────────┐   ┌────────────────────┐      │
│  │ Java/JVM │──▶│ http_proxy 环境变量 │      │
│  │ (任意版本)│   │ → SocksSocketImpl  │      │
│  └──────────┘   │ SOCKS→:18080(不匹配)│     │
│                 └────────┬───────────┘      │
│                          │ 超时             │
│                 ┌────────▼───────────┐      │
│                 │ 直连外网            │      │
│                 │ ❌ TCP 出站被拦截   │      │
│                 └────────────────────┘      │
└─────────────────────────────────────────────┘
```

## 二、进程级网络策略矩阵

| 进程/工具 | 直连外网 | 走 MCP 代理(:18080) | DNS 解析 | localhost |
|-----------|---------|---------------------|----------|-----------|
| **curl** / wget | ✅ 正常（白名单） | — | ✅ | ✅ |
| **Node.js** (默认环境) | ❌ TIMEOUT | ✅ HTTP CONNECT 正常 | ✅ | ✅ |
| **Node.js** (`env -i` 纯净) | ❌ TIMEOUT | — | ✅ | ✅ |
| **Java/JVM** (任意 JDK 版本) | ❌ TIMEOUT | ❌ SOCKS 协议不匹配超时 | ✅ | ✅ |

### 关键测试数据

**DNS 解析正常：**
```
maven.aliyun.com → [183.131.47.194, 121.228.130.68, ...]  (12个IP)
```

**Java `ProxySelector` 返回 DIRECT（无代理），但 TCP 连接仍超时：**
```
Default ProxySelector: proxy: DIRECT → null
NO_PROXY: FAIL: SocketTimeoutException: Connect timed out
    at java.base/sun.nio.ch.NioSocketImpl.connect(NioSocketImpl.java:594)
    at java.base/java.net.SocksSocketImpl.connect(SocksSocketImpl.java:284)
```

**所有 JDK 版本均受影响：**
- JDK 17.0.2 → FAIL: Connect timed out
- JDK 21.0.2 → FAIL: Connect timed out
- JDK 25.0.2 → FAIL: Connect timed out

## 三、自动注入的环境变量

每次执行命令时，沙箱自动注入以下变量（无法通过 `unset` 或 `export` 清除，下一条命令会重新注入）：

```bash
http_proxy=http://127.0.0.1:18080
https_proxy=http://127.0.0.1:18080
HTTP_PROXY=http://127.0.0.1:18080
HTTPS_PROXY=http://127.0.0.1:18080
no_proxy=localhost,127.0.0.1,.svc,.cluster.local,::1
NO_PROXY=localhost,127.0.0.1,.svc,.cluster.local,::1
NODE_OPTIONS=--require /app/mcp_proxy_bootstrap/preload.cjs
PREVIEW_PROXY_PUBLIC_PORT=16000
```

### Node.js 代理注入机制

`NODE_OPTIONS` 通过 preload 脚本让 undici（Node.js 现代 HTTP 客户端）走 HTTP 代理：

```javascript
// /app/mcp_proxy_bootstrap/preload.cjs
const { setGlobalDispatcher, EnvHttpProxyAgent } = require("undici");
if (process.env.http_proxy || process.env.https_proxy) {
    setGlobalDispatcher(new EnvHttpProxyAgent());
}
```

这就是为什么 **npm install 能下载 node_modules** —— 它通过 `EnvHttpProxyAgent` 以正确的 **HTTP CONNECT** 隧道方式经过 `127.0.0.1:18080` 出去。

### Java 代理失败原因

JDK 读到 `http_proxy=http://127.0.0.1:18080` 后，使用 `SocksSocketImpl`（SOCKS 协议）去连接该地址。但 `:18080` 是一个 **HTTP 代理**，不是 SOCKS 代理。协议不匹配导致连接必然超时。

即使使用以下方法也无法绕过：
- `env -i` 清空环境变量 → 直连被沙箱拦截
- `-DproxyHost= -Djava.net.useSystemProxies=false` → 无效
- `Proxy.NO_PROXY` → 直连被沙箱拦截
- 自定义 `ProxySelector` → 直连被沙箱拦截

## 四、Maven 仓库可达性（curl 测试）

所有镜像在沙箱中均可通过 curl 访问：

| 仓库 | URL | 状态 |
|------|-----|------|
| Maven Central | repo.maven.org | ✅ 200 |
| Aliyun Google | maven.aliyun.com/repository/google | ✅ 200 |
| Aliyun Central | maven.aliyun.com/repository/central | ✅ 200 |
| Aliyun Gradle Plugin | maven.aliyun.com/repository/gradle-plugin | ✅ 200 |
| Aliyun Public | maven.aliyun.com/repository/public | ✅ 200 |
| Tencent Tencent | mirrors.tencent.com/nexus/repository/maven-tencent | ✅ 200 |
| Tencent Public | mirrors.tencent.com/nexus/repository/maven-public | ✅ 200 |
| Google Maven | maven.google.com | ✅ 200 |
| Gradle Plugin Portal | plugins.gradle.org | ✅ 200 |

Kotlin 2.3.21 的 plugin marker POM 在以上所有源中均返回 200。

## 五、对构建的影响

### CI 环境 vs 本地沙箱

| 场景 | Java 网络 | Gradle 构建 |
|------|----------|------------|
| **GitHub Actions CI** | ✅ 正常出站 | ✅ 可正常运行 |
| **Trae Web 沙箱本地** | ❌ 出站 TCP 被拦截 | ❌ 无法下载依赖 |

### Go 构建（mise 管理）

- 项目使用 [mise](https://mise.jdx.dev/) 管理工具链，配置文件：`/workspace/mise.toml`
- `go.mod` 要求 **Go 1.25.1**，`mise.toml` 必须匹配：`go = "1.25.1"`
- mise 已安装 Go 1.25.1（路径：`~/.local/share/mise/installs/go/1.25.1/bin/go`）
- 编译命令：`cd /workspace && mise exec -- go build ./cmd/encv/`
- ⚠️ **不要** 使用 `go build`（可能指向系统旧版本），必须通过 `mise exec --` 执行

### 沙箱内可行的替代方案

**方案 A：curl 预下载依赖 + gradle --offline**

```bash
# 用 curl 下载所需依赖到 ~/.gradle/caches/
curl -o ~/.gradle/caches/modules-2/files-x.x.x/<path> <mirror-url>
# 然后
gradle build --offline
```

**方案 B：利用 MCP 代理的 HTTP CONNECT 隧道**

如果 `127.0.0.1:18080` 支持 HTTP CONNECT 方法，可以配置 Java 使用 HTTP 代理而非 SOCKS：

```bash
GRADLE_OPTS="-Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=18080 \
             -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=18080 \
             -Dhttps.nonProxyHosts='localhost|127.*'"
```

注意：这需要确认 18080 端点支持标准 HTTP CONNECT 隧道协议。

**方案 C：不在沙箱内跑 Java 构建**

将 Gradle 构建交给 CI 执行，沙箱只负责代码编辑和非 Java 工具链操作。

## 六、DNS 特殊情况

沙箱 DNS 对部分域名返回 NXDOMAIN（但非全部）：
- `repo.maven.org` → NXDOMAIN（需用镜像代替）
- `maven.aliyun.com` → 正常解析
- `mirrors.tencent.com` → 正常解析
- `maven.google.com` → 正常解析

建议始终优先使用国内镜像（阿里云/腾讯云），避免 DNS 问题。

---

## 七、CDN 阻断清单（**禁止** curl 直连这些源）

> 沙箱的出站 TCP 走白名单，但**部分主流 CDN 即使能 DNS 解析也会被 404 / 阻断**。
> 盲目 `curl <github-release>` / `curl <google-cdn>` 经常卡 30s+ 然后 exit 28 (timeout)。
> **先看这张表再用 curl**。

| 源 | 用途 | 状态 | 替代方案 |
|----|------|------|----------|
| `https://objects.githubusercontent.com` | GitHub Release 资产 CDN | ❌ 404 | **走 gh-proxy.com 镜像**（实测 18 MB / 10s OK） |
| `https://github.com/.../releases/download/...` | GitHub Releases 下载 | ❌ 404 | **走 gh-proxy.com / mirror.ghproxy.com 镜像** |
| `https://dl.google.com/dl/android/maven2/...` | Android Maven 仓库 | ❌ 404 | `maven.google.com` 或阿里云 Google 镜像 |
| `https://download.jetbrains.com/kotlin/...` | JetBrains Kotlin 二进制 | ❌ 404 | Maven Central `kotlin-compiler-embeddable` |
| `https://services.gradle.org/distributions/...` | Gradle 二进制 | ✅ 200 | 已是上游源，不需要替代 |

### 7.0 GitHub Release 镜像（2026-06-04 实战验证）

> GitHub `releases/download` 直链沙箱 60s 超时（仅收到 ~800 KB / 18 MB）。
> 但 **gh-proxy 类镜像通过 :18080 MCP 代理可达**，10s 内下完 18 MB tarball。
> **优先用镜像而非直连。**

| 镜像 | URL 模板 | 实测 |
|------|---------|------|
| `gh-proxy.com` | `https://gh-proxy.com/<原 URL>` | ✅ 18 MB / 10s（OpenList 4.2.2 dist） |
| `mirror.ghproxy.com` | `https://mirror.ghproxy.com/<原 URL>` | ✅ 30s 内 |
| `ghproxy.net` | `https://ghproxy.net/<原 URL>` | ✅ 30s 内 |
| `ghfast.top` | `https://ghfast.top/<原 URL>` | ✅ 30s 内 |

**用法**（以 OpenList-Frontend rolling dist 为例）：

```bash
# 1. 解析 release 元数据（API 走 MCP 代理可达）
url=$(curl -fsSL --max-time 10 -H "Authorization: Bearer $GITHUB_TOKEN" \
  -H "Accept: application/vnd.github.v3+json" \
  "https://api.github.com/repos/OpenListTeam/OpenList-Frontend/releases/tags/rolling" \
  | jq -r '.assets[] | select(.name | endswith(".tar.gz")) | .browser_download_url' \
  | grep -v "lite" | head -1)

# 2. 走 gh-proxy 下载（10s 内）
curl -fsSL --max-time 60 \
  "https://gh-proxy.com/$url" \
  -o /tmp/dist.tar.gz
```

**原理**：gh-proxy 类服务把 `<原 URL>` 路径透传到 GitHub，
通过 :18080 走 HTTP CONNECT 隧道（沙箱 MCP 代理支持 CONNECT），
绕开 `objects.githubusercontent.com` 的 404 / 阻断。

**注意**：
- **不要**用 `https://ghproxy.com/...`（缺 `-`）会 404
- 镜像偶有 502 → `--max-time 10` 失败立即换下一个
- 镜像不需要 GITHUB_TOKEN（GitHub 直连才需要）

### 7.1 识别 CDN 阻断的快速诊断

```bash
# 1. DNS 解析（看是否能拿到 IP）
getent hosts github.com           # 正常返回 IP
getent hosts objects.githubusercontent.com   # 返回 IP 但实际连不上

# 2. curl 测连通性（带短超时）
curl -sI --max-time 10 https://objects.githubusercontent.com/foo  # exit 28 / 404
curl -sI --max-time 10 https://repo1.maven.org/maven2/           # exit 0 / 200

# 3. 走 MCP 代理尝试（只对 HTTP CONNECT 友好的源有效）
http_proxy=http://127.0.0.1:18080 curl -sI --max-time 10 https://github.com  # 通常 OK
```

### 7.2 反模式（**禁止**）

```bash
# ❌ 错：curl GitHub Releases 直链（会卡 30s+）
curl -L https://github.com/JetBrains/kotlin/releases/download/v2.3.21/kotlin-compiler-2.3.21.zip

# ❌ 错：curl JetBrains CDN（被沙箱阻断）
curl -L https://download.jetbrains.com/kotlin/native/builds/

# ❌ 错：curl Google Android Maven（被沙箱阻断）
curl -L https://dl.google.com/dl/android/maven2/androidx/core/core-ktx/1.13.0/core-ktx-1.13.0.aar

# ✅ 对：Maven Central（沙箱白名单，已确认 200）
curl -L https://repo1.maven.org/maven2/org/jetbrains/kotlin/kotlin-compiler-embeddable/2.3.21/kotlin-compiler-embeddable-2.3.21.jar
```

### 7.3 决策树：要拉二进制时

```
1. 这个二进制有 Maven 坐标吗？
   → 有：Maven Central 走起
   → 没有：继续 ↓

2. 是 Gradle 工具吗？
   → 是：services.gradle.org 可达
   → 否：继续 ↓

3. 是 npm 包吗？
   → 是：npm registry 可达
   → 否：继续 ↓

4. 是 Go module 吗？
   → 是：proxy.golang.org 可达
   → 否：继续 ↓

5. 是 GitHub Release 资产？
   → **首选 gh-proxy.com / mirror.ghproxy.com 镜像**（10s 内，2026-06 验证）
   → 镜像失败再试 github.com API 看是否有 redirect 到 S3
   → 若只有 S3 / objects.githubusercontent.com → 阻断，换源
```

---

## 八、Kotlin 编译器一键拉取方案

> 沙箱**无** kotlinc，每次新会话都要重装。手动 curl 4 个 jar + 写 wrapper 易错。
> **统一脚本入口**：[setup-kotlinc.sh](../scripts/setup-kotlinc.sh)

### 8.1 一键命令

```bash
bash /workspace/.trae/scripts/setup-kotlinc.sh
```

**输出预期**（6 步骤全部走完）：

```
==> 0/6 前置检查（java / curl / libs.versions.toml）
    libs.versions.toml: /workspace/app/encv-mobile/android/gradle/libs.versions.toml
    Kotlin version: 2.3.21
==> 1/6 创建 KOTLIN_HOME=/tmp/kotlin-home/lib
==> 2/6 从 Maven Central 拉 4 个 jar（每个 --max-time 60s, curl -f 失败立即 abort）
    [skip] kotlin-compiler-embeddable-2.3.21.jar 已有
    [skip] kotlin-stdlib-2.3.21.jar 已有
    [skip] kotlin-reflect-2.3.21.jar 已有
    [skip] kotlinx-coroutines-core-jvm-1.10.2.jar 已有
==> 3/6 写 /usr/local/bin/kotlinc-2.3.21 包装脚本
==> 4/6 验证 kotlinc-2.3.21 -version
    info: kotlinc-jvm 2.3.21 (JRE 17.0.2+8-86)
==> 5/6 ✅ Kotlin 编译器就绪
==> 6/6 退出码 0（环境就绪，可开始 Kotlin 调试）
```

### 8.2 脚本设计要点

| 维度 | 实现 |
|------|------|
| **版本检测** | 自动 `grep` `libs.versions.toml` 中 `kotlin = "X.Y.Z"`，不硬编码 |
| **路径兼容** | monorepo 多种布局都试（app/encv-mobile/android/gradle/、android/gradle/、gradle/） |
| **下载源** | 100% 走 `https://repo1.maven.org/maven2/`，**绝不**走 GitHub / Google / JetBrains CDN |
| **超时** | 每个 jar `curl --max-time 60`，失败立即 `exit 2`（不重试无限循环） |
| **幂等** | 已有 jar + size 合理（compiler ≥50MB，其余 ≥100KB）就 skip；否则重拉 |
| **包装脚本** | `/usr/local/bin/kotlinc-<version>`，内部 `exec java -cp ... K2JVMCompiler` |
| **校验** | 跑 `kotlinc-<version> -version` 确认退出码 0 + 输出含 `kotlinc-jvm` |
| **退出码** | 0=OK / 1=前置缺 / 2=网络失败 / 3=版本校验失败 |
| **风格** | 仿照 `app/encv-mobile/scripts/start-preview.sh` 的 `set -euo pipefail` + `step()` + 状态报告 |

### 8.3 沙箱 Kotlin 调试的标准流程

> 详见 [.trae/rules/verification-discipline.md §7](verification-discipline.md)

**第一步：跑 setup（每会话一次）**

```bash
bash /workspace/.trae/scripts/setup-kotlinc.sh
```

**第二步：语法检查（不依赖 android.jar / compose.jar）**

```bash
cd /workspace/app/encv-mobile/plugin-openlist
/usr/local/bin/kotlinc-2.3.21 \
    -no-stdlib -no-reflect -Xsuppress-version-warnings \
    src/main/java/com/encvgo/plugin/openlist/*.kt \
    -d /tmp/out 2>/tmp/err.log
```

**第三步：过滤真 bug vs 缺依赖**

```bash
# 真 bug（应该为空）
grep -E "Syntax error|abstract member|Composable invocations|Unclosed comment" /tmp/err.log

# unresolved reference 是预期（沙箱无 android.jar / combolite-core.jar）
grep -c "unresolved reference" /tmp/err.log
```

### 8.4 注意事项

- **`/tmp` 在新沙箱会话清空** → 每次新会话都需重跑 `setup-kotlinc.sh`
- **kotlin 版本升级** → 改 `libs.versions.toml` 后重跑脚本即可（旧 wrapper 留着也无害，新 wrapper 叫 `kotlinc-<新版本>`）
- **不要手动改 `/usr/local/bin/kotlinc-*`** → 应改 setup 脚本模板（占位符 `__VERSION__` / `__KOTLIN_HOME__`）
- **不走 GitHub Releases** → 沙箱阻断 `objects.githubusercontent.com`（见 §七）

---

## 八、Preview Proxy 路由必须由 OpenPreview 工具注册（2026-06-04 实战踩坑）

> **铁律：起完 dev server 必须调用 `OpenPreview` 工具，否则 16000 代理一律 400。**

### 8.1 症状

```
curl http://127.0.0.1:16000/openlist-ui/   → 400 Service Unavailable
```

服务（vite、air、openlist vite）明明都在 5173/2025/3000 活着，但外部预览代理 16000 一个请求都不通。

### 8.2 根因

`agent-tool-host`（PID 由 `supervisor` 启动的 `/app/bin/agent-tool-host`）在 16000 暴露 preview proxy。
它**不会自动发现**沙箱里监听的 dev server 端口——需要 agent 显式调用 `OpenPreview` 工具注册。

agent-tool-host 日志会爆：
```
ERROR preview_proxy::proxy: [preview-proxy] No valid port found for request: / 
  (default_port: None, registered_ports: [])
```

`registered_ports: []` = 路由表是空的。

### 8.3 修复

1. 起服务（`start-preview.sh` 或类似脚本），拿到**该命令的 `command_id`**
2. 调用 `OpenPreview(command_id=..., preview_url="http://localhost:<vite-port>/")`
3. 验证：日志出现 `Proxying / to port 5173`，16000 返回 200

```bash
# 验证脚本（OpenPreview 调用之后）
for path in / /api/service-guard /api/preview/openlist-ui /openlist-ui/; do
  code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:16000$path")
  echo "  16000$path → $code"
done
# 全部应该 200 或 302，不再 400
```

### 8.4 注意事项

- **OpenPreview 必须用启动 dev server 的那条命令的 `command_id`**——不是后续 curl 的 ID
- **代理路由有 session/TTL**——长时间不活动 / 重启 agent-tool-host 后可能再次掉线，需要重新调用 OpenPreview
- **多个 dev server 同时跑**时，OpenPreview 只能注册**一个**端口（默认 / 路径）。
  - 当前 ENCV 架构：只注册 5173（encv-mobile vite），其它（3000/2025）通过 5173 的 proxy 间接访问
  - 不要试图注册多个 OpenPreview——agent-tool-host 会用最后一次调用的端口
- **检测掉线**：`grep "No valid port found" /var/log/tool/agent-tool-host.stdout.log` 是最快信号

### 8.5 为什么 start-preview.sh 不自己调 OpenPreview

start-preview.sh 是个普通 bash 脚本，没有 `OpenPreview` 工具的访问能力。
它**只能**在最后打印 `OpenPreview(preview_url="...")` 提示，由 agent（运行在 MCP 那一侧）来真正调用工具。
所以**每次重启预览都得 agent 收尾调一次 OpenPreview**，这是沙箱架构的硬约束。

---

## 九、Preview 链路实际可达性矩阵（2026-06-08 实战实测）

> 沙箱 dev 链路：外网浏览器 → `:16000` (agent-tool-host) → `:16666` (preview-gateway) → `:8100` (vite) / `:2025` (encv-go)
> 本节是 §八 "只能注册一个端口 / 路径" 的**实测补充**——明确告诉 agent "哪些能成 / 哪些必失败"。

### 9.1 实测可达性矩阵

| 调用方 | 目标路径 | `:16000` 是否转发到 `:16666` | 经 gateway 后真正落点 | 状态 |
|--------|----------|----------------------------|---------------------|------|
| 浏览器加载 SPA | `/` | ✅ | `:8100` index.html | ✅ 200 |
| Vite 拉模块 | `/@vite/client` | ✅ | `:8100` /@vite/client | ✅ 200（hmr:false 也注入脚本） |
| Vite 拉源码 | `/src/main.ts` | ✅ | `:8100` /src/main.ts | ✅ 200 |
| 前端 fetch API | `/api/config` | ❌ **不转发** | — | ❌ CORS/404/网络错 |
| 前端 fetch API | `/api/service-guard` | ❌ **不转发** | — | ❌ 同上 |
| WebSocket | `/ws` | ❌ **不支持升级** | — | ❌ readyState=3 |
| Vite HMR client | `@vite/client` 内的 WS URL | ❌ | — | ❌ `[vite] failed to connect to websocket` |

**关键发现**：
- **agent-tool-host (:16000) 只把注册时声明的根路径 `/` 转发到注册端口**，子路径 `/api/*` `/openlist-ui/*` 等都**不转发**——这是 §八 "OpenPreview 只能注册一个端口（默认 / 路径）" 的实操结果。
- **WebSocket 升级不支持**——规则 §三 + [useFrontendLogs.ts:75-78](file:///workspace/app/encv-mobile/src/composables/useFrontendLogs.ts#L75-L78) 已记录。
- **vite.config.ts 的 `server.hmr = false`** + htmlRewritePlugin 移除 `@vite/client` 脚本——Vite HMR client 的 WS 也连不上，但这是预期降级，**不影响应用运行**。

### 9.2 后果：useApiBaseProbe 全失败

`useApiBaseProbe.ts` 的探测链：
1. `[1] cached` —— localStorage 缓存
2. `[1.5] current origin` —— `window.location.origin`（`<public-url>/api/config`）
3. `[2] loopback` —— `http://127.0.0.1:2025`
4. `[3] LAN` —— 通过 baseUrl 拉 `/api/network/lan-access`

在沙箱浏览器里：
- `[1]` 如果用户上次缓存的 URL 是有效 trae 域名，**可能** 命中
- `[1.5]` `<public-url>/api/config` → **不转发** → fetch 报 `TypeError: Failed to fetch` 或 CORS 错
- `[2]` `127.0.0.1:2025` 在用户机器上根本不存在 → fetch 报 `net::ERR_CONNECTION_REFUSED`
- `[3]` 同 [2]，LAN 候选为 null

→ `all-candidates-failed` 抛出 → `App.vue:33 mounted hook` 捕获 → `[error] [App] Vue error captured: Error: all-candidates-failed`。

### 9.3 备用方案（按改动量从小到大）

#### 方案 X：把 :2025 单独注册成外网入口（**最简单**）

让 OpenPreview 注册 `:2025` 而非 `:16666`。外网用户访问 trae 域名 → :16000 → :2025 (encv-go)。然后前端用绝对 URL `https://run-agent-...trae.cn/api/...`（这时代理自动适配到 :2025）。

**代价**：encv-go 必须自带静态文件 serve（embed 前端 dist）—— 但**沙箱 dev 模式不 build**，所以 vite :8100 的 HMR 体验就没了。

#### 方案 Y：方案 C 改造 —— 把 :2025 (encv-go) 注册，gateway (:16666) 只在沙箱内做反代（**不推荐**）

让 preview-gateway 仅作"沙箱内部使用"的反向代理，外部通过注册的 :2025 走 API。浏览器 fetch `https://.../api/...` 实际代理到 :2025。但 SPA 静态资源要由 encv-go serve（Vite 不参与），失去 HMR。

#### 方案 Z：accept & document 当前行为（**当前做法**）

承认沙箱浏览器模式下 API + WS 不可用，**让前端在 probe 失败时优雅降级**：
- 不要 throw 阻塞 `App.vue mounted`
- 改用 `useUiErrorBanner` 显示 "API 连接失败，请到 Settings 手动设置服务器地址"
- WS 失败时静默重试（已部分实现）

**代价**：浏览器模式下 `Files` / `Tasks` / `Settings` 等所有依赖 API 的功能不可用。用户要么切到本地 dev (`pm2 start ...` + `vite --port 8100` + 自访问 `:8100`)，要么用 APK 真机调试。

### 9.4 调试手段：mock 浏览器（**无 Network 面板**）

> **沙箱架构硬约束（2026-06-08 用户承认）：**
> 用户**只能在 agent-tool-host 提供的指定模拟浏览器里**预览（OpenPreview 工具激活）。
> 该浏览器：
> - ✅ **可以** 刷新页面（Cmd+R / Ctrl+R）
> - ✅ **可以** 查看 console 日志（DevLogs tab 可见）
> - ❌ **没有** 完整 DevTools
> - ❌ **没有** Network 面板（看不到 fetch 请求/响应 / status / header / CORS 错）
> - ❌ **没有** Application / Sources / Performance 面板
>
> → **诊断时只能靠 console 日志**，所有关键路径必须把 trace 打到 console。

#### 9.4.1 前端必做的日志规范

> 凡是 agent 排查时需要看到的链路，**必须** `console.info` 一行带 `[probe]` 前缀的日志。
> 全部走 `console.info`（**不**用 debug——DevLogs 默认隐藏 debug）。

`useApiBaseProbe` 每次探测的日志模板（已落到 `useApiBaseProbe.ts`）：

| 时机 | console.info 模板 | 示例 |
|------|------------------|------|
| 入口 | `[probe] start origin=<o> cached=<c> force=<f>` | `[probe] start origin=https://run-agent-xxx.trae.cn cached=(empty) force=false` |
| [1] 缓存 | `[probe] step [1] try cached: <url>` / `[probe] step [1] result: ok=false err=...` | `[probe] step [1] no cached URL, skip` |
| [1.5] current origin | `[probe] step [1.5] try current origin: <o>` / `result: ok=false err=non-JSON response (content-type: "text/html")` | — |
| [2] loopback | `[probe] step [2] try loopback: <url>` / `result: ok=false err=...` | `[probe] step [2] result: ok=false latency=12ms err=TypeError: Failed to fetch` |
| [expand] | `[probe] step [expand] try N lan candidates (port P)` | — |
| [lan] 每个候选 | `[probe] step [lan] try <url>` / `result: ok=false err=...` | — |
| 命中 / 提交 | `[probe] commit baseUrl=<u> source=<s> latency=<L>ms` | `[probe] commit baseUrl=https://run-agent-xxx.trae.cn source=current-origin latency=143ms` |
| 全部失败 | `[probe] step [4] no candidates available, all-failed` + `[probe] FAIL all-candidates-failed \| trace: ...` | — |

**用户在 mock 浏览器看到 N 条 `[probe]` 日志（典型 6-13 条）= 一次完整探测链的 trace。**

#### 9.4.2 错误抛出规范

**❌ 错误**：只 throw 一个 `"all-candidates-failed"`——mock 浏览器没有 Network 面板，agent 拿不到 fetch 细节。

**✅ 正确**：把整条 `log[]` 数组串成单行 error message 抛出：

```ts
const trace = log.join(' | ')
throw new Error(`all-candidates-failed | trace: ${trace}`)
```

这样上游 `App.vue` 捕获后能直接看到 `[1] no cached URL, skip | [1.5] result: ok=false err=non-JSON... | [2] result: ok=false err=TypeError: Failed to fetch | [4] no candidates available, all-failed`。

#### 9.4.3 其它 API 调用失败的日志规范

| 场景 | 日志模板 | 等级 |
|------|---------|------|
| 关键 API 失败 | `[api] POST /api/chat failed: status=502 content-type=text/html body=<前 200 字符>` | `console.error` |
| WebSocket 断连 | `[ws] disconnect code=1006 reason= wsUrl=<url> readyState=3` | `console.warn` |
| WebSocket 升级失败 | `[ws] upgrade failed: 沙箱 agent-tool-host (:16000) 不支持 WS 升级（见 §九.1）` | `console.warn` |
| mock 浏览器无 network 警告 | `[browser] mock browser has no DevTools Network panel — see trae_web_sandbox_network.md §9.4` | `console.info`（首次启动时打一次） |

#### 9.4.4 agent 必做

- 看到 `[error]` 开头的日志 → 在用户报告里贴完整堆栈 + 上下文 `[probe]` 日志
- 不要假设 "看到 502 = 反代挂了"——可能是 CORS / 网络错 / Content-Type 不符 / 401 鉴权
- 如果 `useApiBaseProbe` 没打 `[probe]` 日志 → 用户浏览器是**旧代码**，让用户硬刷新（Cmd+Shift+R / Ctrl+F5）
- mock 浏览器报错时**不要**让用户"开 DevTools 看 Network"——**没有 DevTools**，只能让他把 console 日志贴过来
- 用户报"preview 不工作"时第一反应应是"在 mock 浏览器 DevLogs tab 找到 [probe] 行，贴过来"

### 9.5 沙箱外 / 本地 dev 的真链路

| 环境 | 前端访问地址 | API 地址 | WS 地址 |
|------|--------------|----------|---------|
| **沙箱浏览器（OpenPreview）** | `https://run-agent-...trae.cn/` | ⚠️ 同源 `/api/*` 不可达（见 9.1） | ⚠️ `/ws` 不可达 |
| **沙箱本地（PM2 启 :16666）** | `http://localhost:16666/` | `http://localhost:16666/api/*` ✅ | `ws://localhost:16666/ws` ✅ |
| **沙箱 dev 直连 vite** | `http://localhost:8100/` | ❌ vite 不代理 /api（设计决策 D9） | ❌ vite HMR 关 |
| **APK 真机 + adb reverse** | `capacitor://localhost` | `http://127.0.0.1:2025` ✅ | `ws://127.0.0.1:2025/ws` ✅ |

→ **沙箱 dev 模式下唯一完整的端到端调试链路是「沙箱本地访问 :16666」**——agent 在沙箱内跑 `curl http://localhost:16666/__gateway/health` 等价于"完整功能可用"。**OpenPreview 仅用于给用户看 UI 截图 / 视觉验收，不能用于功能调试**。

### 9.6 绝对禁止

- ❌ 假设 OpenPreview 模式下能完整功能调试——见 9.5
- ❌ 让前端 throw 阻塞 `mounted` hook 不带 console.info 兜底——见 9.4
- ❌ 试图注册多个 OpenPreview——§八.4 已禁止，agent-tool-host 用最后一次的端口
- ❌ 让 vite 加回 `/api` proxy——决策 D9 已固化
- ❌ 在 mock 浏览器里让用户"开 DevTools 看 Network"——**没有 DevTools**

---

## 十、跨文档引用

| 主题 | 文档 |
|------|------|
| 沙箱可下载范围 + kotlinc 拉取细节 + 沙箱验证流程 | [verification-discipline.md §7](verification-discipline.md) |
| 一键准备脚本源码 + 退出码规范 | [../scripts/setup-kotlinc.sh](../scripts/setup-kotlinc.sh) |
| Capacitor 预览一键启动脚本（同样模式） | [../../app/encv-mobile/scripts/start-preview.sh](../../app/encv-mobile/scripts/start-preview.sh) |
| CI 端 Gradle 仓库配置 + isMinifyEnabled 硬约束 | [android.md](android.md) |
