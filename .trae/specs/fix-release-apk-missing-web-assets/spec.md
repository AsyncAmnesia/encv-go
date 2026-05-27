# 修复 Release APK 前端资源缺失 + 清除 config.mobile.json 幽灵改动

## Why
Release APK 启动后 WebView 显示 `net::ERR_CONNECTION_REFUSED`，因为前端 Web 资源（dist/）未被复制到 APK 的 assets/public/ 目录。同时 android-overlay 中的 EncvGoService.kt 仍残留 `config.mobile.json` 引用（幽灵改动），若 post-cap-sync.mjs 执行会将已修复的版本覆盖回错误版本。

## What Changes
- CI workflow 为 release 构建添加 `npx cap copy android` 步骤，确保 Web 资源进入 APK
- 修复 `android-overlay/EncvGoService.kt` 中 `config.mobile.json` → `config.user.json`
- 在 `sync-native.mjs` 中添加 `config.user.json` 复制到 assets 的逻辑
- CI 添加 APK 内 Web 资源存在性验证（检查 `public/index.html`）

## Impact
- Affected code:
  - `.github/workflows/android.yml` — 添加 cap copy 步骤 + 验证
  - `app/encv-mobile/android-overlay/app/src/main/java/com/encvgo/app/EncvGoService.kt` — 修复 config.mobile → config.user
  - `app/encv-mobile/scripts/sync-native.mjs` — 添加 config.user.json 复制

## ADDED Requirements

### Requirement: Release APK 必须包含前端 Web 资源
CI 构建的 Release APK 必须包含完整的前端 Web 资源（index.html 及关联 JS/CSS），确保 WebView 能正常加载页面。

#### Scenario: Release 构建后 APK 包含 Web 资源
- **WHEN** CI 执行 release 构建（version 参数非空）
- **THEN** APK 的 `assets/public/index.html` 存在且非空
- **AND** WebView 能正常加载 `https://localhost/` 而非显示 `ERR_CONNECTION_REFUSED`

### Requirement: overlay 源文件不得引用 config.mobile.json
android-overlay 目录中的所有 Kotlin 源文件必须引用 `config.user.json`，不得出现 `config.mobile.json`。

#### Scenario: post-cap-sync 执行后 EncvGoService.kt 仍使用 config.user.json
- **WHEN** `post-cap-sync.mjs` 或 `sync-native.mjs` 执行覆盖 Kotlin 文件
- **THEN** `EncvGoService.kt` 中的 `assets.open()` 调用使用 `"config.user.json"` 而非 `"config.mobile.json"`

### Requirement: sync-native.mjs 必须复制 config.user.json 到 assets
`sync-native.mjs`（capacitor:copy:after hook）必须将项目根目录的 `config.user.json` 复制到 `android/app/src/main/assets/`。

#### Scenario: cap copy 执行后 assets 目录包含 config.user.json
- **WHEN** `npx cap copy android` 执行完毕
- **THEN** `android/app/src/main/assets/config.user.json` 存在

## MODIFIED Requirements

### Requirement: CI release 构建流程
CI 的 release 构建必须在 Gradle 构建前执行 `npx cap copy android`，确保 Web 资源和原生配置同步到 Android 项目。原有的 debug-only 限制需移除。

## REMOVED Requirements

### Requirement: config.mobile.json 配置文件
**Reason**: 项目规则明确禁止创建独立的 `config.mobile.json`，移动端适配通过 Go 端 `ENCV_MOBILE` 环境变量实现。android-overlay 中的残留引用是幽灵改动。
**Migration**: 所有 `config.mobile.json` 引用替换为 `config.user.json`，与已部署版本保持一致。
