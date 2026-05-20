# 修复：MainActivity.kt 编译错误（2 处）

## 错误列表（CI 日志）

| 行 | 错误 | 原因 |
|----|------|------|
| 116:71 | Too many characters in a character literal | `'unknown'` 是 Char 字面量（只允许单字符），且即使用单引号在双引号字符串插值中也语义不清 |
| 230-276 | Syntax error: Expecting member declaration (×15) | 诊断代码写在函数体外（`return null` + `}` 提前闭合了函数体） |

## 修复

### 修复 1：L116 — 用变量避免引号嵌套问题

```kotlin
// 原来（两种写法都有语法问题）:
notifyFrontend(0, "start_failed:${e.message?.take(200) ?: 'unknown'}")     // 'unknown' = Char 超长
notifyFrontend(0, "start_failed:${e.message?.take(200) ?: "unknown"}")    // "unknown" = 闭合外层字符串

// 修复 — 用局部变量:
val errMsg = e.message?.take(200) ?: "unknown"
notifyFrontend(0, "start_failed:$errMsg")
```

### 修复 2：L221-232 — 诊断代码移入 findExecutableBinary() 函数体内

当前结构（错误）：`return null` 在 L221 → 函数 `}` 在 L222 → 后续 9 行代码（L224-232）全部变成非法顶层语句。

修复：将 L224-232 的诊断代码移到 L221 的 `return null` **之前**（函数体内）。
