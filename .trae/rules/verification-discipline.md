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
