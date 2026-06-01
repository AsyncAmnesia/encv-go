# 二次解码路径修复 — 分支完整性排查

## 用户反馈

> "二次解码路径修复不在 trae/solo-agent-bBrT3z 分支中"

## 排查结果

### 1. DecodePathParam 代码状态

**✅ 存在于 bBrT3z 分支中**

| 文件 | 行号 | 内容 | 来源 |
|------|------|------|------|
| [path.go](internal/utils/path.go#L11-L20) | L11-20 | `DecodePathParam()` 函数定义（双重 url.QueryUnescape） | PR #29 (`8b4fa77`) |
| [path.go](internal/utils/path.go#L80) | L80 | `SafeURLToAbsPath()` 内调用 `DecodePathParam(decodedPath)` | PR #29 (`8b4fa77`) |
| [path_test.go](internal/utils/path_test.go) | L120-372 | 完整测试套件（Idempotent, DoubleEncoding, RoundTrip 等 8 个测试函数） | PR #29 (`8b4fa77`) |

### 2. 三分支对比

```
dev / main / bBrT3z 的 path.go 完全一致 ✅
```

三个分支的 `DecodePathParam` 实现**完全相同**，无差异：

```go
func DecodePathParam(raw string) string {
    s, err := url.QueryUnescape(raw)
    if err != nil { return raw }
    s2, err := url.QueryUnescape(s)
    if err != nil { return s }
    return s2
}
```

### 3. 调用链完整

```
manifest_v2.go:OpenContainer()
  → utils.DecodePathParam(containerPath)     ← path.go:11
  → utils.SafeURLToAbsPath(baseDir, decoded) ← path.go:43 (内部也调用 DecodePathParam)
```

### 4. bBrT3z 分支提交历史（28 个提交）

```
d6609df fix: remove boolean comparison in toggle checked binding (TS2367)  ← 最新
d65d731 cherry-pick: NewTaskModal.vue 类型分支渲染修复 + 组件导入补全
a42936a feat: Capacitor前后端预览与Go运行                              ← 第 25 个 (bBrT3z 原始)
... (22 个中间提交)
5abab3c feat: Capacitor前后端预览与Go运行                              ← 第 1 个 (基于 8b4fa77)
8b4fa77 Merge pull request #29 from AsyncAmnesia/trae/solo-agent-PZqWlc ← 基础
```

## 需要用户确认

代码层面 `DecodePathParam` **确实存在于 bBrT3z**。可能的情况：

1. **用户指的是某个特定的 bug 场景修复** — 不是函数本身是否存在，而是某个调用点的修改？
2. **用户期望的是某个尚未实现的改进** — 比如 manifest_v2.go 中某处应该调用但没调用的地方？
3. **用户看到的是其他分支上的改动** — 可能在 PZqWlc 合并前有某个本地修改？

**请用户提供更多线索：**
- 具体是哪个文件的哪行代码缺失？
- 或者描述一下"二次解码路径修复"具体解决了什么问题？
