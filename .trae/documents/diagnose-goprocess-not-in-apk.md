# 修复：GoProcess plugin "not implemented on android"

## 改动（2 个文件）

### 1. post-cap-sync.mjs — 两处新增

**改动 A：显式 sourceSets 声明**（第 135-143 行）

CI 日志关键证据：`checkKotlinGradlePluginConfigurationErrors SKIPPED` + `compileDebugKotlin` 无任何编译输出。说明 Kotlin 编译器可能未将 `app/src/main/java` 纳入源码集。

在 jvmTarget 配置后追加：
```groovy
android.sourceSets {
    main.java.srcDirs += 'src/main/java'
}
```
这确保 Kotlin 编译器明确知道去哪里找 .kt 文件。

**改动 B：包名一致性验证**（第 217-229 行）

每次 post-cap-sync 执行时校验 MainActivity.kt 和 GoProcessPlugin.kt 的 `package` 声明是否为 `com.encvgo.app`，不匹配则立即 exit 1 阻断构建。

### 2. android.yml — Verify 步骤替换

将 `unzip -l | grep "GoProcessPlugin"`（无法检测 dex 内类）替换为 `strings classes.dex | grep`（直接检测二进制内容）+ kotlin-classes 输出目录检查。

## 根因判断

| 指标 | 值 | 含义 |
|------|-----|------|
| checkKotlinGradlePluginConfigurationErrors | **SKIPPED** | Kotlin 插件配置不完整 |
| compileDebugKotlin 耗时 | **8 秒** | 有工作要做，但可能只编了依赖 |
| unzip -l \| grep GoProcessPlugin | **NOT found** | ⚠️ 此方法无效（dex 内的类不是独立文件）|
| capacitor.plugins.json | **[]** | 正常（本地插件不在此文件中）|

**最可能原因**：`apply plugin: 'kotlin-android'` 以 legacy 方式注入后，Gradle 未自动将 `app/src/main/java` 加入 Kotlin 源码集 → 我们的 .kt 文件存在但未被编译 → APK 中无 GoProcessPlugin.class → 运行时 "not implemented"。

## 下次 CI 构建后如何确认修复成功

1. 日志应出现 `[kotlin] added explicit sourceSets for kotlin sources`
2. 日志应出现 `pkg-ok: MainActivity.kt → com.encvgo.app` 和 `pkg-ok: GoProcessPlugin.kt → com.encvgo.app`
3. Verify 步骤应输出 `✅ GoProcess found in DEX (compiled+packaged)` 或 `✅ encvgo .class in kotlin-classes`
