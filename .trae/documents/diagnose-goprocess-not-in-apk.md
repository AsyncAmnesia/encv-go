# 诊断：GoProcessPlugin 未被 Capacitor 识别（基于 CI 日志权威分析）

## 一、CI 日志铁证

### 1. capacitor.plugins.json 为空数组 🔴
```
行 13100: === capacitor.plugins.json in APK ===
[]
```
**这是根因。** Capacitor 运行时通过此文件知道哪些原生插件可用。空数组 = 所有原生插件不可见 = JS 端调用报 "not implemented on android"。

### 2. post-cap-sync 执行成功但用了 legacy 格式
```
行 12665: [kotlin] applied kotlin-android via legacy apply
行 12836: 2:apply plugin: 'kotlin-android'    ← 在第2行！
```
post-cap-sync.mjs 的注入逻辑走了 `else if (c.includes("apply plugin: 'com.android.application'"))` 分支（legacy apply 格式），而非 `plugins {}` DSL 分支。

### 3. Kotlin 编译任务异常标记
```
行 12900: > Task :app:checkKotlinGradlePluginConfigurationErrors SKIPPED
行 12971: > Task :app:compileDebugJavaWithJavac NO-SOURCE
行 12970: > Task :app:compileDebugKotlin    (耗时8秒，无错误输出)
```
- `checkKotlinGradlePluginConfigurationErrors SKIPPED` → Kotlin 插件配置可能不完整
- `compileDebugJavaWithJavac NO-SOURCE` → Java 编译器看不到任何 .java 文件（正常）
- `compileDebugKotlin` 执行了8秒但 **无任何 e:/w: 输出**

### 4. 文件复制链路完整 ✅
```
行 12667: overlay: MainActivity.kt          ← 第1次 post-cap-sync (cap add 触发)
行 12668: overlay: GoProcessPlugin.kt       ← 第1次
行 12686: overlay: MainActivity.kt          ← 第2次 post-cap-sync (cap sync 触发)
行 12687: overlay: GoProcessPlugin.kt       ← 第2次
行 12844: ✅ GoProcessPlugin.kt exists      ← Verify 步骤确认
行 12846: ✅ MainActivity.kt exists         ← Verify 步骤确认
```

## 二、根因分析

### 因果链

```
Capacitor 8.x 生成的 app/build.gradle 使用 plugins{} DSL 格式
  ↓
post-cap-sync.mjs 检测到 "apply plugin: 'com.android.application'" 字符串
  ↓
走 legacy 分支：用 "apply plugin: 'kotlin-android'" 在第2行插入
  ↓
plugins{} DSL 块与 apply plugin: 语句混用 → Kotlin 源码集配置不完整
  ↓
Capacitor 注解处理器无法扫描 @CapacitorPlugin 注解的 Kotlin 类
  ↓
capacitor.plugins.json 生成为 []
  ↓
运行时 Plugins.GoProcess → "not implemented on android"
```

### 为什么 compileDebugKotlin 不报错却无产出？

`apply plugin: 'kotlin-android'` 以 legacy 方式添加后，Gradle 会创建 `compileDebugKotlin` 任务并执行（所以看到8秒耗时），但由于与 `plugins {}` DSL 的 Android 插件协调不完全，**编译产物可能未被正确关联到 Capacitor 的注解处理器管线**。

## 三、修复方案

### 核心修改：post-cap-sync.mjs 的 kotlin-android 注入逻辑

**问题代码**（当前走的是 legacy 分支）:
```javascript
// 当前：Capacitor 8.x 的 build.gradle 同时包含两种格式，
// "apply plugin:" 匹配优先命中，导致使用 legacy 方式注入
} else if (c.includes("apply plugin: 'com.android.application'")) {
    c = c.replace(
        "apply plugin: 'com.android.application'",
        "apply plugin: 'com.android.application'\napply plugin: 'kotlin-android'",
    )
}
```

**修复方案**: 调整匹配优先级 + 兼容两种格式：

```javascript
// 1. kotlin-android plugin — 优先尝试 plugins {} DSL（Capacitor 6+/8.x 标准）
if (!c.includes('kotlin-android') && !c.includes('org.jetbrains.kotlin.android')) {
  if (c.match(/plugins\s*\{/)) {
    // 新 DSL 格式: id 'kotlin-android'
    c = c.replace(
      /plugins\s*\{/,
      "plugins {\n    id 'kotlin-android'",
    )
    console.log('  [kotlin] injected id \'kotlin-android\' into plugins {} block')
  } else if (c.includes("apply plugin: 'com.android.application'")) {
    // 旧 apply 格式: apply plugin: 'kotlin-android'
    c = c.replace(
      "apply plugin: 'com.android.application'",
      "apply plugin: 'com.android.application'\napply plugin: 'kotlin-android'",
    )
    console.log('  [kotlin] applied kotlin-android via legacy apply')
  } else {
    // Capacitor 8.x 可能使用 plugins DSL 但关键字不同
    // 尝试匹配 id 'com.android.application' 格式
    const pluginsMatch = c.match(/plugins\s*\{[^}]*id\s+['"]com\.android\.application['"]/s)
    if (pluginsMatch) {
      c = c.replace(
        /plugins\s*\{/,
        "plugins {\n    id 'kotlin-android'",
      )
      console.log('  [kotlin] injected into plugins {} (matched com.android.application id)')
    } else {
      console.error('  [kotlin] WARNING: could not find plugins block or com.android.application!')
      console.error('  [kotlin] build.gradle first 500 chars:', c.substring(0, 500))
    }
  }
}
```

### 验证步骤（CI 日志中添加）

在 Verify 步骤中增加对 `capacitor.plugins.json` 内容的检查：

```bash
echo "=== capacitor.plugins.json content ==="
unzip -p "$APK_PATH" assets/capacitor.plugins.json | head -5
echo "=== Checking for GoProcess in plugins.json ==="
unzip -p "$APK_PATH" assets/capacitor.plugins.json | grep -q "GoProcess" && echo "✅ GoProcess registered" || echo "❌ GoProcess NOT in plugins.json"
```

## 四、执行步骤

1. [ ] **修改 post-cap-sync.mjs**：增强 kotlin-android 注入逻辑，正确处理 Capacitor 8.x 的 `plugins {}` DSL 格式
2. [ ] **本地验证**（如可能）：确认生成的 build.gradle 中 kotlin-android 位于 `plugins {}` 块内
3. [ ] **触发 CI 构建**：运行 android.yml workflow
4. [ ] **检查 CI 日志确认**：
   - `[kotlin] injected id 'kotlin-android' into plugins {} block` 日志出现
   - `capacitor.plugins.json` 包含 `GoProcess` 条目
   - APK 验证步骤显示 `✅ GoProcess registered`
