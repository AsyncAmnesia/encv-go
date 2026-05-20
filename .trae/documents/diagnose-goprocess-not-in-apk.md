# 修复：MainActivity.kt 编译错误（2 处）

## 错误列表（CI 日志）

| 行 | 错误 | 原因 |
|----|------|------|
| 116:71 | Too many characters in a character literal | `'unknown'` 是 Char 字面量，只允许单字符 |
| 230-276 | Syntax error: Expecting member declaration (×15) | 诊断代码写在函数体外，导致解析器混乱 |

## 修复

### 修复 1：L116 — Char → String

```kotlin
// 原来:
notifyFrontend(0, "start_failed:${e.message?.take(200) ?: 'unknown'}")

// 修复:
notifyFrontend(0, "start_failed:${e.message?.take(200) ?: "unknown"}")
```

### 修复 2：L221-232 — 诊断代码移入函数体内

当前代码结构（错误）：
```kotlin
private fun findExecutableBinary(): File? {
    // ... for 循环 ...
    return null          // ← L221: 函数在这里结束
}                        // ← L222: 函数闭合

Log.e(TAG, "...")       // ← L224: 函数体外！语法错误
for (...) { ... }       // ← L225: 函数体外！
notifyFrontend(...)     // ← L231: 函数体外！
return null             // ← L232: 函数体外！

private fun ensureConfigExists() {  // ← L235
```

修复后：
```kotlin
private fun findExecutableBinary(): File? {
    // ... for 循环 ...

    // 诊断输出（在 return null 之前，函数体内）
    Log.e(TAG, "=== Binary diagnosis: ALL locations failed ===")
    for ((dir, name) in candidateDirs) { ... }
    Log.e(TAG, "SDK_INT=...")
    notifyFrontend(0, "no_binary")

    return null          // ← 函数在这里结束
}                        // ← 唯一闭合

private fun ensureConfigExists() {  // ← 正常
```
