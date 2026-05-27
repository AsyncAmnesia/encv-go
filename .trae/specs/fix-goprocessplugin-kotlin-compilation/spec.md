# 修复 GoProcessPlugin.kt Kotlin 编译错误

## Why

CI 构建在 `:app:compileReleaseKotlin` 阶段失败，`GoProcessPlugin.kt` 有 17 个编译错误，导致 Release APK 无法生成。错误根因是缺少 import 声明和重复的 companion object 声明。

## What Changes

- 添加缺失的 `import android.content.BroadcastReceiver` 和 `import android.content.Context`
- 合并两个 `companion object` 为一个（将 `REQUEST_CODE_PLUGIN_PICK` 移入主 companion object）
- 修复 `String?` 类型的 nullable 安全调用问题

## Impact

- Affected code: `app/encv-mobile/android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt`
- Affected specs: 无其他 spec 受影响

## ADDED Requirements

### Requirement: GoProcessPlugin.kt 编译通过

`GoProcessPlugin.kt` SHALL 通过 Kotlin 编译，无编译错误。

#### Scenario: import 完整性
- **WHEN** Kotlin 编译器处理 `GoProcessPlugin.kt`
- **THEN** 所有使用的类型（`BroadcastReceiver`、`Context`）SHALL 有对应的 import 声明

#### Scenario: 单一 companion object
- **WHEN** Kotlin 编译器处理 `GoProcessPlugin.kt`
- **THEN** 类中 SHALL 只有一个 `companion object`，包含 `TAG` 和 `REQUEST_CODE_PLUGIN_PICK`

#### Scenario: nullable 安全调用
- **WHEN** 对 `String?` 类型调用方法
- **THEN** SHALL 使用安全调用 `?.` 或非空断言 `!!.`

## MODIFIED Requirements

无

## REMOVED Requirements

无
