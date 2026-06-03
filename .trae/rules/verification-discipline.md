# 验证纪律规则（Verification Discipline）

> 来自踩坑：曾把 `io.github.lnzz123` 包对应的真实库（com.combo.core.*）幻觉为「Hi-Sillot/ComboLite」仓库，
> 并对不存在的 URL 发起 WebFetch 阻塞等待。此规则强制以下验证纪律。

---

## 一、铁律：核实先于生成

**任何"我以为我知道"的命名（库名 / 仓库 / URL / 包名 / 类名）必须先验证，再用于回复。**

### 1.1 触发"验证"的操作前提

出现以下任一情况，**禁止直接生成**：

- 引用一个第三方库的包名/类名/仓库
- 给出一个 GitHub URL / docs URL
- 引用一个外部 API 的方法签名
- 推荐安装某个 npm/pip/maven 坐标
- 引用一行 CI 输出"我记得是这样"

### 1.2 验证顺序（**本地优先**）

```
1️⃣  Grep / Glob / Read   搜本地仓库（最快、零网络）
2️⃣  cargo test / go test  跑实际构建/单元测试
3️⃣  webfetch + websearch  仅在 1+2 拿不到时；用且只用一次；带短超时
4️⃣  生成结论
```

### 1.3 禁止的反模式

- ❌ 凭印象写出「Hi-Sillot/ComboLite」然后 WebFetch 该 URL（用户原话："根本没有 Hi-Sillot/ComboLite 这个库，哪来的幻觉？"）
- ❌ 给出 Maven 坐标时写 `androidx.core:core-ktx`（无版本）却不验证是否能被 BOM 解析
- ❌ 引用一个方法名而不读 source 验证它存在
- ❌ 用 WebSearch「也许能找到」式发散查询

---

## 二、本地工具速查（优先用这些）

| 想做 | 用这个 | 备注 |
|------|--------|------|
| 找类/函数/常量定义 | `Grep -n` | 1ms 返回 |
| 找文件位置 | `Glob` | 1ms 返回 |
| 读源码 | `Read` | 1ms 返回 |
| 读 CI 日志 | `RunCommand grep` 本地日志副本 | 不要 WebFetch GitHub |
| 验证方法签名 | `Read` 源文件 | 必做 |
| 看依赖坐标 | `Read libs.versions.toml` | 不要 WebFetch Maven Central |
| 验证库存在 | `Grep` 仓库内引用 + 读 `libs.versions.toml` | 不要 WebSearch |
| 验证 URL 可达 | `WebFetch` 1 次 + 短超时 | 30s 兜底 |

---

## 三、WebFetch / WebSearch 红线

### 3.1 必须先 Grep 完本地再 WebFetch

**写代码前先做**：

```bash
# 例：想确认 IPluginEntryClass 的包路径
Grep pattern="IPluginEntryClass"  # 拿到真实包路径
Read  <file with import>           # 看实际 import
```

只有本地确实不存在（全新外部库），才允许 WebFetch。

### 3.2 WebFetch 短超时纪律

WebFetch 是无超时/长超时的工具。本地仓库**永远比外网**准确。

```yaml
禁用: WebFetch 一个不熟悉的 GitHub URL 等待返回
替代: Grep 本地看是否已引用；找不到就停下来问用户
```

### 3.3 WebSearch 红线

WebSearch 用于「补完知识盲区」而非「验证已知」。

- ✅ 用 WebSearch 找新 API 文档（CLI 升级了，文档没读）
- ❌ 用 WebSearch 验证我以为的包路径

---

## 四、CI 失败诊断纪律

### 4.1 找到失败点的流程

```
0. 先看 0_build.txt 的最后 200 行（Post job cleanup / Cache saved → 说明 build 成功）
1. 找 "FAILED" / "exit code 1" / "BUILD FAILED" 行
2. 上溯 30 行，看 "What went wrong" / "Caused by"
3. 再下溯看 Exception stacktrace
4. 把失败定位到具体 task + dependency + source line
```

### 4.2 不要在没读 log 前臆测根因

**反面教材**（用户原话：「更本没读日志就开始改」）：
- 看到 `androidx.core:core-ktx` 报错就猜是版本问题
- 没读 build/0_build.txt 实际行就脑补「Vite 8 不支持 --prod」

**正确做法**：
- 先 `grep -n "FAIL\|Error\|Could not" 0_build.txt`
- 定位行号 → `Read` 那个区段
- 找到根因再改

---

## 五、错误处理流程（用户已踩过的坑）

| 用户反馈 | 对应反模式 | 正确做法 |
|---------|-----------|---------|
| 「哪来的幻觉」 | WebFetch 想象中的 URL | 先 Grep 本地 |
| 「还 curl 阻塞半天」 | WebFetch 默认阻塞 | 本地工具 < 1s |
| 「用本地工具」 | 默认 WebFetch/curl | Grep / Read / Glob |
| 「用 Read 读实际行」 | 凭印象写代码 | Read 文件确认存在 |
| 「不要在没读 log 前瞎改」 | 跳过日志直接改 | 先 `cat`/`grep` 日志 |

---

## 六、强制 checklist（在生成代码/结论前自查）

- [ ] 我引用的库名/包名/类名——在本地仓库里能 Grep 到吗？
- [ ] 我引用的方法签名——Read 过源文件确认存在吗？
- [ ] 我给出的 URL——如果是仓库/源码链接，本地已通过 Grep 验证吗？
- [ ] 我给出的依赖坐标——`libs.versions.toml` 里有版本引用吗？
- [ ] 我跑的 WebFetch——是基于「本地完全没有」的必要调用吗？
- [ ] 我跑的 WebSearch——是补完知识盲区，不是验证已知吗？
- [ ] 我读的 CI 日志——定位到具体 task + line 了吗？

任何一项打勾失败 → 停下来用本地工具补足，再生成。

---

## 七、沙箱可下载范围分析（实战归纳）

> 沙箱里的网络并不全通。盲目 `curl <github-release>` / `curl <google-cdn>` 经常 timeout。**先测后下**。
>
> **Kotlin 编译器准备已工程化** → 直接跑 [`/workspace/.trae/scripts/setup-kotlinc.sh`](../scripts/setup-kotlinc.sh) 一键就绪（自动读 `libs.versions.toml`、从 Maven Central 拉 4 个 jar、写 `/usr/local/bin/kotlinc-<version>` wrapper）。详见 [trae_web_sandbox_network.md §八](trae_web_sandbox_network.md)。

### 7.1 已确认可达的源（Maven 协议族）

| 源 | URL | 用途 |
|----|-----|------|
| **Maven Central** | `https://repo1.maven.org/maven2/` | 任何 JVM 库（Kotlin / AndroidX / ksp 等都镜像到这里） |
| **npm registry** | `https://registry.npmjs.org/` | node 包 |
| **Gradle Plugin Portal** | `https://plugins.gradle.org/m2/` | Gradle 插件 |
| **Gradle distributions** | `https://services.gradle.org/distributions/` | Gradle 二进制 |
| **GitHub 主页** | `https://github.com` | 列表浏览、API |
| **cache-redirector.jetbrains.com** | (slow but reachable) | JetBrains 工具链 |

### 7.2 已确认阻断的源（二进制 CDN）

| 源 | 状态 | 阻断原因 |
|----|------|----------|
| `https://objects.githubusercontent.com` (GitHub Objects CDN) | ❌ 404 | 沙箱代理阻断 |
| `https://github.com/.../releases/download/` (GitHub Releases 下载) | ❌ 404 | 同上 |
| `https://dl.google.com/dl/android/maven2/` | ❌ 404 | Google CDN 阻断 |
| `https://download.jetbrains.com/kotlin/` | ❌ 404 | JetBrains CDN 阻断 |

### 7.3 沙箱里**已经有**的工具（**不要重新装**）

```bash
# 来自 mise 安装
java 17.0.2  → /root/.local/share/mise/installs/java/17.0.2/bin/java
javac 17.0.2
gradle 8.14.4  → /root/.local/share/mise/installs/gradle/8.14.4/gradle-8.14.4/bin/gradle
mvn 3.9.10     → /root/.local/share/mise/installs/maven/3.9.10/apache-maven-3.9.10/bin/mvn

# pnpm 已通过 pnpm/action-setup 装好
pnpm

# 来自系统
apt / apt-get  →  但 apt 仓库**没** kotlin / gradle 包
```

### 7.4 拿 Kotlin 编译器的标准做法

**推荐：一键脚本**（自动读 `libs.versions.toml`、Maven Central 拉 4 个 jar、写 wrapper）：

```bash
bash /workspace/.trae/scripts/setup-kotlinc.sh
```

**禁止**用 `curl GitHub releases/download/.../kotlin-compiler-2.3.21.zip`（会超时）
**禁止**用 `curl download.jetbrains.com/kotlin/...`（沙箱 CDN 阻断）
**禁止**用 `curl dl.google.com/dl/android/maven2/...`（沙箱 CDN 阻断）

**手动做法**（不推荐，仅当脚本失败时备查）：用 Maven 拉 `kotlin-compiler-embeddable`，自建包装脚本：

```bash
# /usr/local/bin/kotlinc-2.3.21
KOTLIN_HOME="/tmp/kotlin-home"
mkdir -p "$KOTLIN_HOME/lib"
for art in kotlin-compiler-embeddable kotlin-stdlib kotlin-reflect; do
    if [ ! -f "$KOTLIN_HOME/lib/${art}-2.3.21.jar" ]; then
        curl -sL --max-time 30 -o "$KOTLIN_HOME/lib/${art}-2.3.21.jar" \
            "https://repo1.maven.org/maven2/org/jetbrains/kotlin/${art}/2.3.21/${art}-2.3.21.jar"
    fi
done
exec java -cp "$KOTLIN_HOME/lib/kotlin-compiler-embeddable-2.3.21.jar:$KOTLIN_HOME/lib/kotlin-stdlib-2.3.21.jar:$KOTLIN_HOME/lib/kotlin-reflect-2.3.21.jar" \
    org.jetbrains.kotlin.cli.jvm.K2JVMCompiler -kotlin-home "$KOTLIN_HOME" "$@"
```

**用法**（与原版 kotlinc 兼容）：

```bash
kotlinc-2.3.21 -version
kotlinc-2.3.21 -Xsuppress-version-warnings -no-stdlib -no-reflect <*.kt> -d /tmp/out
```

### 7.5 用 kotlinc 在沙箱里做语法验证的标准流程

```bash
# 步骤 1: 沙箱语法检查（不用 classpath，仅看 syntax）
cd /path/to/plugin
kotlinc-2.3.21 -no-stdlib -no-reflect -Xsuppress-version-warnings src/main/java/**/*.kt -d /tmp/out 2>/tmp/err.log

# 步骤 2: 过滤"真 bug" vs "缺依赖"两类错误
echo "=== 语法/抽象成员/Composable 错误（真 bug）==="
grep -E "Syntax error|Unclosed comment|Missing '|Expecting token|Unexpected token|abstract member|does not implement abstract|Composable invocations can only happen" /tmp/err.log
# 应该为空——如果不为空,说明 source 有真 bug

echo "=== Unresolved references（缺依赖，不是 source bug）==="
grep -c "unresolved reference" /tmp/err.log
# 数字大但都是预期的（android.*、com.combo.*、openlistlib.*、compose.*）
```

### 7.6 CI 错误 vs 沙箱验证的差异

| 维度 | CI（真） | 沙箱验证（本次） |
|------|---------|---------------|
| 是否有 android.jar | ✅ | ❌ |
| 是否有 combolite-core.jar | ✅ | ❌ |
| 是否有 openlist-classes.jar | ✅ | ❌ |
| 是否有 compose runtime | ✅ | ❌ |
| 能抓出 syntax error | ✅ | ✅ |
| 能抓出 abstract member error | ✅ | ✅（仅当 classpath 完整） |
| 能抓出 unresolved reference | ✅ | ❌（沙箱只能报"有 unresolved"，不能定真伪） |

**沙箱验证的定位**：抓 syntax / parse 错误（CI log 的核心 ~60% 错误都是这一类），然后给 CI 跑全量。**不是**替代 CI。

### 7.7 沙箱也能抓 Gradle Kotlin DSL 脚本编译错误

> **Phase 16 踩坑**：CI 报 `Unresolved reference 'foundation'.`（在 `build.gradle.kts:81-82`），
> Phase 14 修复时加 `implementation(libs.compose.foundation)` 但忘了在 `libs.versions.toml` 声明。
> **Gradle Kotlin DSL 脚本编译期就崩了**，不进入 source 编译期。
>
> **沙箱检测方法**：写一个 `bash` 脚本遍历所有 `build.gradle.kts`，
> 提取 `libs.X.Y` 引用，对照 toml alias（toml `-` → 访问器 `.`），**总失败数应 0**。

```bash
# 1. 提取 toml 全部 alias
TOML_LIBS=$(awk '/\[libraries\]/{flag=1; next} /^\[/{flag=0} flag && /^[a-zA-Z]/' \
    android/gradle/libs.versions.toml | sed -E 's/^([a-zA-Z0-9._-]+).*/\1/')
TOML_PLUGINS=$(awk '/\[plugins\]/{flag=1; next} /^\[/{flag=0} flag && /^[a-zA-Z]/' \
    android/gradle/libs.versions.toml | sed -E 's/^([a-zA-Z0-9._-]+).*/\1/')

# 2. 遍历所有 build.gradle.kts
FAIL=0
for f in $(find . -name "build.gradle.kts"); do
    for ref in $(grep -oE 'libs\.(plugins|[a-zA-Z0-9._-]+)\.[a-zA-Z0-9._-]+' "$f" | sort -u); do
        [[ "$ref" == libs.versions.* ]] && continue
        if [[ "$ref" == libs.plugins.* ]]; then
            alias="${ref#libs.plugins.}"; alias="${alias//./-}"
            echo "$TOML_PLUGINS" | grep -qx "$alias" || { echo "✗ $f: $ref"; FAIL=$((FAIL+1)); }
        else
            alias="${ref#libs.}"; alias="${alias//./-}"
            echo "$TOML_LIBS" | grep -qx "$alias" || { echo "✗ $f: $ref"; FAIL=$((FAIL+1)); }
        fi
    done
done
[ "$FAIL" -eq 0 ] && echo "✅ 所有 libs.* 引用都能在 toml 找到"
```

**根因 / 教训**：
- 任何 `libs.X.Y` 引用都必须有 toml alias 对应（`toml alias 用 '-' 分隔, 访问器用 '.'`）
- 改 build.gradle.kts 加新 `libs.*` 引用时,必须同时改 `libs.versions.toml` 声明
- Gradle Kotlin DSL 脚本编译错误会污染整个 multi-project（**任何子项目配置期失败 → 其他子项目的任务也死**）

### 7.8 gomobile Java 命名规则（**沙箱也能诊断**）

> **Phase 17 踩坑**：CI 报 `Unresolved reference 'forceDbSync'.`（`OpenListBridge.kt:306`），
> 一直猜是 gomobile 暴露了错误的方法名。其实**沙箱里能 100% 验证**：
> 读 gomobile 源码里的 `lowerFirst()` 函数 + 读 fork 实际 Go 函数名 → 直接推导出 Java 名。

**核心规则**（gomobile `cmd/gobind/gen.go:527 lowerFirst`）：

```go
func lowerFirst(s string) string {
    // 逐 rune 走,遇到非大写就停
    // 只 lowercase 第 1 个 rune,后面保留原样
    // 例: "ForceDBSync" → "forceDBSync"   (DBSync 作为一个子词保留)
    // 例: "SetConfigData" → "setConfigData"
    // 例: "IsRunning" → "isRunning"
}
```

**Go 函数名 → Java 方法名 速查**（fork `openlistlib/` 实际导出）：

| Go 函数（PascalCase） | Java 方法（camelCase） | 本项目调用点 |
|----------------------|----------------------|-------------|
| `GetOutboundIP()` | `getOutboundIP()` | 未用 |
| `GetOutboundIPString()` | `getOutboundIPString()` | 未用 |
| `SetConfigData(path)` | `setConfigData(String)` | [OpenListBridge.kt:101](file:///workspace/app/encv-mobile/plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListBridge.kt) ✅ |
| `SetConfigLogStd(b)` | `setConfigLogStd(boolean)` | 未用 |
| `SetConfigDebug(b)` | `setConfigDebug(boolean)` | 未用 |
| `SetConfigNoPrefix(b)` | `setConfigNoPrefix(boolean)` | 未用 |
| `SetAdminPassword(pwd)` | `setAdminPassword(String)` | [OpenListBridge.kt:330](file:///workspace/app/encv-mobile/plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListBridge.kt) ✅ |
| `Init(e Event, cb LogCallback)` | `init(Event, LogCallback)` | [OpenListBridge.kt:111](file:///workspace/app/encv-mobile/plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListBridge.kt) ✅ |
| `IsRunning(t)` | `isRunning(String)` | [OpenListBridge.kt:294](file:///workspace/app/encv-mobile/plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListBridge.kt) ✅ |
| `Start()` | `start()` | [OpenListBridge.kt:233](file:///workspace/app/encv-mobile/plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListBridge.kt) ✅ |
| `Shutdown(timeout)` | `shutdown(long)` | [OpenListBridge.kt:269](file:///workspace/app/encv-mobile/plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListBridge.kt) ✅ |
| **`ForceDBSync()`** | **`forceDBSync()`** | [OpenListBridge.kt:309](file:///workspace/app/encv-mobile/plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListBridge.kt) ✅（Phase 17 修） |

**沙箱自检脚本**（验证任意 Go 名字 → Java 名）：

```bash
# 用 gomobile 源码里的 lowerFirst 跑（直接复用其逻辑）
cat > /tmp/lowerFirst.go <<'EOF'
package main
import ("fmt"; "unicode"; "unicode/utf8")
func lowerFirst(s string) string {
    if s == "" { return "" }
    var conv []rune
    for len(s) > 0 {
        r, n := utf8.DecodeRuneInString(s)
        if !unicode.IsUpper(r) {
            if l := len(conv); l > 1 { conv[l-1] = unicode.ToUpper(conv[l-1]) }
            return string(conv) + s
        }
        conv = append(conv, unicode.ToLower(r))
        s = s[n:]
    }
    return string(conv)
}
func main() { fmt.Println(lowerFirst("ForceDBSync")) }
EOF
go run /tmp/lowerFirst.go
# 期望输出: forceDBSync
```

**特别提醒**：
- `DBSync` / `HTTPClient` 这种**全大写子词**会被保留（`lowerFirst` 只动首字符）
- 写 Kotlin 包装函数时**注意 A2 fallback**：[`build-openlist-aar.sh:381`](file:///workspace/scripts/build-openlist-aar.sh) 只在 fork 缺 `openlistlib/event.go` 时才注入。Hi-Sillot/OpenList@`404daf0` 已自带 event.go（`OnProcessExit(code int)`）→ A2 fallback 被跳过 → gomobile 生成 `onProcessExit(int)`，**和现有 `code: Long` 不匹配**（下一轮 AAR 重构时会爆）。Phase 17 不动它，留作 Phase 18 风险登记。

### 7.9 守卫也要被 test 验证（Guard self-test discipline）

> **Phase 18 元教训**：写好的 guard 自己也要 smoke test，否则可能比没 guard 还糟。

#### 7.9.1 反面教材（Guard A 盲点）

**事件**：[`/workspace/.github/workflows/android.yml`](file:///workspace/.github/workflows/android.yml) Guard A（TOML alias guard）第一版：

```bash
# ❌ 第一版
for f in $(find android -name "build.gradle.kts"); do
    for ref in $(grep -oE 'libs\.(plugins|[a-zA-Z0-9._-]+)\.[a-zA-Z0-9._-]+' "$f" | sort -u); do
        ...
    done
done
```

**症状**：
- 破坏 `libs.versions.toml`（删 `compose-foundation` 2 行）后，guard **没报错**
- 实际只检查了 22 个 `libs.*` refs，漏掉 14 个
- 漏的是 `plugin-openlist/build.gradle.kts` 和 `plugin-mpv-player/build.gradle.kts`（**不在 `android/` 目录下**，是 `android/` 的 sibling）

**根因链路**：
```
Guard A 期望: 遍历整个 monorepo 的所有 build.gradle.kts
实际遍历:   find android -name "build.gradle.kts"
              ↑ 范围被限制在 android/ 子树
              ↑ plugin-openlist/ 在 app/encv-mobile/ 下, 不在 android/ 下
              ↑ 漏 14/36 个 refs
              ↑ 静默 false-pass
              ↑ CI 仍然在 3 min Gradle 配置期才报 unresolved
```

**为什么 smoke test 没抓到**：smoke test 在 `app/encv-mobile/` 目录下跑，
本身 `cd` 路径就有歧义 → 测了但被 `cd` 逻辑吞了，**没暴露**真正的盲点。

#### 7.9.2 修复：故意坏掉（Break-it-back test）

**任何新增的"自动化守卫"都必须经过「故意坏掉」的反向测试**：

```bash
# === 守卫自检模板 ===

# 1. 在干净仓库上跑 → 期望 0 错
bash <guard-script>
# 预期: "✅ 所有 libs.* 引用都能在 toml 找到" / EXIT=0

# 2. 故意引入已知 bug → 期望抓出
cp android/gradle/libs.versions.toml /tmp/toml.bak
# 删除 toml 中某个 alias (模拟 Phase 16 错)
sed -i '/^compose-foundation =/d' android/gradle/libs.versions.toml
bash <guard-script>
# 预期: "✗ plugin-openlist/build.gradle.kts:81: libs.compose.foundation" / EXIT≠0
# 验证抓到了! ✓

# 3. 还原
cp /tmp/toml.bak android/gradle/libs.versions.toml
bash <guard-script>
# 预期: 0 错 / EXIT=0
```

**对 Guard A 的实际修复**：
```bash
# ✅ 修复后
for f in $(find . -name "build.gradle.kts"); do  # ← 从 repo root, 不限子目录
    for ref in $(grep -oE 'libs\.(plugins|[a-zA-Z0-9._-]+)\.[a-zA-Z0-9._-]+' "$f" | sort -u); do
        ...
    done
done
# 验证: 36 refs (之前 22), 破坏后精准抓 2 错
```

#### 7.9.3 Guard self-test checklist

写完任何 guard（bash 脚本 / GitHub Action / gradle task）后, **必做**：

- [ ] **正向测试**：干净仓库跑 guard → 0 错, EXIT=0
- [ ] **反向测试（必做）**：故意制造 guard 想抓的 bug → guard 必须报错并定位到具体文件 + 行
- [ ] **回归测试**：还原 bug → guard 重新 0 错
- [ ] **路径覆盖测试**：`find` / `grep` 范围从 **repo root** 开始, 不限子目录（除非 guard 明确只针对某子项目）
- [ ] **silent-fail 排查**：guard 跑通但**实际没遍历到目标**也算失败（用 `wc -l` 或计数变量验证扫描量）

#### 7.9.4 守卫与 CI 反馈链的金字塔

```
   ┌──────────────────┐
   │   CI 编译 (3-5min)│   ← 最晚发现, 但最准确
   ├──────────────────┤
   │  Guard B (10s)   │   ← kotlinc pre-flight, 抓真 unresolved
   ├──────────────────┤
   │  Guard A + C (<1s)│  ← grep 静态扫描, 零网络
   └──────────────────┘
   越下越早发现, 越上越准. 守卫 = 把错误往下推, 早发现早治.
```

**关键认知**：guard 不是要替代 CI 编译, 是要把错误**往前推**——从 3 min 推到 1 s。
但 guard **必须**经过 §7.9.3 的 self-test, 否则会把"假绿"伪装成"真绿",
反而**增加**调试时间（你以为有 guard 罩着, 实际它在睡觉）。

#### 7.9.5 历史教训时间线

| Phase | 错类型 | Guard 抓到? | 耗时 |
|-------|--------|------------|------|
| 14 | `dist/* to` 嵌套块注释 | ❌（当时无 Guard B） | CI 4 min |
| 15 | setup-kotlinc.sh 脚本 bug | ❌（脚本本身无 self-test） | 手动排错 20 min |
| 16 | `libs.compose.foundation` 缺 toml | ❌（当时无 Guard A） | CI 3 min |
| 17 | `forceDbSync` 命名错 | ❌（当时无 Guard B） | CI 3 min |
| 17 | `snapshot.running` Map property | ❌（当时无 Guard C） | CI 3 min |
| **18+** | **任何同类** | **✅ Guard A/B/C 抓** | **< 10s** |

#### 7.9.6 反向测试在沙箱里的标准做法

```bash
# === 在 /workspace (CI repo root) 跑 ===

# Guard A 范例
cd /workspace
TOML=app/encv-mobile/android/gradle/libs.versions.toml
[ -f "$TOML" ] || TOML=android/gradle/libs.versions.toml

# 1. 正向
for f in $(find . -name "build.gradle.kts"); do
    refs=$(grep -oE 'libs\.[a-zA-Z0-9._-]+\.[a-zA-Z0-9._-]+' "$f" | sort -u | wc -l)
    [ "$refs" -gt 0 ] && echo "  $f: $refs refs"
done
# 预期: 总 refs = 36 (含 plugin-openlist + plugin-mpv-player)

# 2. 反向
cp "$TOML" /tmp/toml.bak
sed -i '/^compose-foundation =/d' "$TOML"
<run guard>
# 预期: EXIT≠0, 报错至少 1 个 line
# 验证: guard 真能抓!

# 3. 还原
cp /tmp/toml.bak "$TOML"
<run guard>
# 预期: EXIT=0
```

**关键纪律**：
- 沙箱 smoke test **必须从 CI repo root 跑**（不能 `cd app/encv-mobile` 再 `cd app/encv-mobile` 套两层）
- 破坏测试**至少跑 1 次正向 + 1 次反向**, 不能省
- guard 写完 24 小时内必须 self-test, 否则遗忘成本 > 测试成本

#### 7.9.7 guard 写完后的强制 checklist（commit 前）

- [ ] guard 脚本 bash -n 通过（无语法错）
- [ ] YAML 嵌入的 guard 用 `python3 -c "import yaml; yaml.safe_load(open('android.yml'))"` 验证
- [ ] **正向测试**跑过且 EXIT=0
- [ ] **反向测试**跑过且 guard 真报错（不是 silent pass）
- [ ] 路径范围从 repo root 起（不限子目录）
- [ ] 已记录在 `tasks.md` Phase N 的子项
- [ ] **§7.9 引用过**, 元教训已写下来

任何一项没做 → **不允许 commit**。

#### 7.9.8 Go 改动 commit 前必跑三件套（Phase 19 教训）

> **Phase 19 教训**：以为只改了 Go 格式（gofmt 整库），commit 前只跑 `gofmt -l` + `go build` 验证，
> 没跑 `go test` → CI 立刻爆 3 个 pre-existing Go test 失败 + 1 个安全 bug。
> 浪费一整次 CI 时间。

**Go 改动 commit 前必跑**（顺序无关，但全要跑）：

```bash
# 1. 格式
gofmt -l ./internal ./cmd ./pkg 2>/dev/null  # 应输出空
[ $? -eq 0 ] && echo "✅ gofmt"

# 2. 类型 / 静态检查
go vet ./internal/... ./cmd/... 2>&1
[ $? -eq 0 ] && echo "✅ go vet"

# 3. 单元测试
go test ./internal/... ./cmd/... -count=1 -timeout 120s 2>&1 | tee /tmp/gotest.log
[ $? -eq 0 ] && echo "✅ go test"
```

**任何一项不过** → 不允许 commit。这是从"gofmt 整库"commit 把 3 个 pre-existing test 失败 + 1 个安全漏洞带进 CI 后总结的铁律。

**Phase 19 真实战果**：
| 检查 | 之前 | 之后 |
|------|------|------|
| `gofmt -l` | ✅ | ✅ |
| `go build` | ✅ | ✅ |
| **`go test`** | **❌ 漏跑** | **✅ 必跑** |
| `go vet` | ❌ 漏跑 | ✅ 必跑 |

新增的两个 test failure 类型（§7.9 guard 抓不到，因为是运行时不是编译时）：
1. **架构漂移型** — 旧 contract test 期望老 API（PluginManager.isInitialized），代码已重构走新封装（EncvComboLiteHost）
2. **silent-fallback 型** — alist_encrypt plugin 静默把 `.sccgv` 改为 `.bin`，掩盖真实冲突

**CI 守卫不会抓运行时 test 失败**（gofmt guard / kotlinc guard / toml guard 都是编译期守卫），
所以沙箱的 `go test ./internal/...` 是**唯一**提前发现这类问题的环节。
