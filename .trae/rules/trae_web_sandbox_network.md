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
| `https://objects.githubusercontent.com` | GitHub Release 资产 CDN | ❌ 404 | 改走 GitHub API + 直链解析；或换镜像源 |
| `https://github.com/.../releases/download/...` | GitHub Releases 下载 | ❌ 404 | Maven Central 镜像（如 combolite-core） |
| `https://dl.google.com/dl/android/maven2/...` | Android Maven 仓库 | ❌ 404 | `maven.google.com` 或阿里云 Google 镜像 |
| `https://download.jetbrains.com/kotlin/...` | JetBrains Kotlin 二进制 | ❌ 404 | Maven Central `kotlin-compiler-embeddable` |
| `https://services.gradle.org/distributions/...` | Gradle 二进制 | ✅ 200 | 已是上游源，不需要替代 |

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
   → 试试 github.com API 看是否有 redirect 到 S3
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

## 九、跨文档引用

| 主题 | 文档 |
|------|------|
| 沙箱可下载范围 + kotlinc 拉取细节 + 沙箱验证流程 | [verification-discipline.md §7](verification-discipline.md) |
| 一键准备脚本源码 + 退出码规范 | [../scripts/setup-kotlinc.sh](../scripts/setup-kotlinc.sh) |
| Capacitor 预览一键启动脚本（同样模式） | [../../app/encv-mobile/scripts/start-preview.sh](../../app/encv-mobile/scripts/start-preview.sh) |
| CI 端 Gradle 仓库配置 + isMinifyEnabled 硬约束 | [android.md](android.md) |
