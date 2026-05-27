# 修复 GoProcessPlugin.kt Kotlin 编译错误

## Why

CI 构建在 `:app:compileReleaseKotlin` 阶段失败，`GoProcessPlugin.kt` 有 17 个编译错误，导致 Release APK 无法生成。FFmpeg 构建已完全通过（资源符号验证 ✅），真正的阻塞点是 Kotlin 编译。

## 根因分析

### 洞察：这不是"偶然遗漏 import"，而是代码膨胀导致的结构性退化

`GoProcessPlugin.kt` 从最初的简单 Capacitor 插件（管理 Go 进程生命周期）逐步膨胀为"万能入口"——集成了进程管理、权限请求、屏幕方向、播放器控制、插件安装、文件路径解析等 6+ 个职责域。每次新增功能时，开发者只关注方法本身的逻辑，忽略了 Kotlin 的 import 和 companion object 约束：

1. **缺失 import 的范式问题**：项目中其他文件（`MainActivity.kt`、`PlayerActivityCapacitor.kt`、`EncvGoService.kt`）都正确 import 了 `BroadcastReceiver` 和 `Context`。`GoProcessPlugin.kt` 缺失这些 import 说明它是在 IDE 环境下编写的——IDE 的自动补全让开发者忽略了 import 的存在，而 CI 的命令行编译器没有这种便利。

2. **重复 companion object 的范式问题**：第 29 行的 `companion object` 包含 `TAG`，第 409 行的 `private companion object` 包含 `REQUEST_CODE_PLUGIN_PICK`。这是典型的"增量开发不回头"模式——新增功能时在文件末尾添加了第二个 companion object，而没有检查是否已有现成的。Kotlin 不允许一个类有多个 companion object。

3. **`Context` 引用的歧义**：`GoProcessPlugin` 继承自 Capacitor `Plugin`，后者提供了 `context` 属性（类型为 `Context`）。但 `Context` 作为**类型**使用时（如 `Context::class.java`、`Context.POWER_SERVICE`、`Context.RECEIVER_NOT_EXPORTED`、`context: Context?` 参数声明），必须显式 import `android.content.Context`。仅 import `ContextCompat` 不够。

### CI vs 本地的差异

这不是 Kotlin 版本差异（CI 和本地都用 2.1.0），也不是 JVM target 差异（都是 JVM_21）。差异在于：
- **IDE 自动 import**：Android Studio 在粘贴代码时自动添加 import，开发者可能没注意到
- **增量编译**：IDE 的增量编译可能缓存了旧的编译结果，掩盖了新增的错误
- **CI 全量编译**：CI 每次从零开始编译，任何 import 缺失都会暴露

## What Changes

- 添加 `import android.content.BroadcastReceiver` 和 `import android.content.Context`
- 合并两个 `companion object`：将 `REQUEST_CODE_PLUGIN_PICK` 移入主 companion object，删除 `private companion object`
- 修复 `String?` nullable 安全调用（第 589 行 `path.isEmpty()` 和第 598 行 `path.removePrefix("/")`）

## Impact

- Affected code: `app/encv-mobile/android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt`
- Affected specs: 无

## ADDED Requirements

### Requirement: GoProcessPlugin.kt 编译通过

`GoProcessPlugin.kt` SHALL 通过 Kotlin 编译，无编译错误。

#### Scenario: import 完整性
- **WHEN** Kotlin 编译器处理 `GoProcessPlugin.kt`
- **THEN** 所有使用的类型（`BroadcastReceiver`、`Context`）SHALL 有对应的 import 声明，与项目中其他文件（`MainActivity.kt`、`PlayerActivityCapacitor.kt`）的 import 模式一致

#### Scenario: 单一 companion object
- **WHEN** Kotlin 编译器处理 `GoProcessPlugin.kt`
- **THEN** 类中 SHALL 只有一个 `companion object`，包含 `TAG` 和 `REQUEST_CODE_PLUGIN_PICK`

#### Scenario: nullable 安全调用
- **WHEN** 对 `String?` 类型调用方法
- **THEN** SHALL 使用安全调用 `?.` 或非空断言 `!!.`

### Requirement: 遵循项目 import 范式

`GoProcessPlugin.kt` 的 import 声明 SHALL 遵循项目中其他 Kotlin 文件的 import 排序范式：
1. `android.*` 包
2. `androidx.*` 包
3. 第三方库（`com.getcapacitor.*`）
4. `java.*` 包

## MODIFIED Requirements

无

## REMOVED Requirements

无
