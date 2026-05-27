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
