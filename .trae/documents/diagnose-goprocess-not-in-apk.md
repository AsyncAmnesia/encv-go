# 诊断与修复：GoProcess plugin "not implemented on android"

## 核心思路

**不做猜测，一次定位。** 在 Build 步骤前插入一个诊断步骤，同时修复验证方法。跑一次 CI 就能确定根因。

## 一、CI 日志关键事实

| 证据 | 日志行 | 值 |
|------|--------|-----|
| overlay 文件复制 | L12667-12668, L12686-12687 | ✅ 成功 |
| kotlin-android 注入 | L12665 | `apply plugin: 'kotlin-android'` (legacy) |
| compileDebugKotlin | L12970 | ✅ 执行了8秒 |
| BUILD SUCCESSFUL | L12931 | ✅ 94 tasks |
| capacitor.plugins.json | L13100 | `[]` (空) |
| unzip -l grep GoProcessPlugin | L13102 | NOT found ⚠️ |

## 二、重要发现

### `unzip -l` 无法检测 dex 内的类 🔴

APK 中 Kotlin/.class 文件被打包进 `classes.dex`（二进制），不是独立文件。**当前的 "GoProcessPlugin NOT found in APK" 是误报**，不能作为判断依据。

### capacitor.plugins.json = [] 是正常的

纯本地 .kt 插件不在 npm 包中，不会自动出现在此文件中。通过 `registerPlugin()` 手动注册即可（brain3-companion 同款项目证实）。

### 真正需要确认的事只有一件：

> **Gradle 的 compileDebugKotlin 是否真的编译了 app/src/main/java/com/encvgo/app/ 下的 .kt 文件？**

## 三、一次性修复方案（2 个文件改动）

### 改动 1：android.yml — 替换 Verify + 增加 Pre-build 诊断

将原来的 "Verify APK contents" 步骤替换为正确的方法，并在 Build 前增加诊断：

```yaml
# === 在 "Verify build.gradle kotlin setup" 之后、"Build APK" 之前插入 ===
- name: Pre-build diagnostic (source files & config)
  run: |
    echo "=== Source .kt files ==="
    find app/encv-mobile/android/app/src/main/java -name "*.kt" -type f
    echo "=== AndroidManifest <activity> ==="
    grep -n 'activity\|android:name' app/encv-mobile/android/app/src/main/AndroidManifest.xml | head -10
    echo "=== Package in MainActivity.kt ==="
    head -3 app/encv-mobile/android/app/src/main/java/com/encvgo/app/MainActivity.kt
    echo "=== Package in GoProcessPlugin.kt ==="
    head -3 app/encv-mobile/android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt
```

```yaml
# === 替换原有的 "Verify APK contents" 步骤 ===
- name: Verify APK (dex-level check)
  run: |
    APK_PATH=$(find app/encv-mobile/android/app/build/outputs/apk -name "*.apk" -type f | head -1)
    [ -z "$APK_PATH" ] && { echo "No APK found"; exit 1; }
    
    echo "=== APK size ==="
    ls -lh "$APK_PATH"
    
    echo "=== Assets ==="
    unzip -l "$APK_PATH" | grep -E "encv-go|config.mobile" || true
    
    echo "=== Compiled classes in build output (.class files) ==="
    find app/encv-mobile/android/app/build/tmp/kotlin-classes/debug -name "*.class" -path "*encvgo*" 2>/dev/null && \
      echo "✅ encvgo .class files found in kotlin-classes" || \
      echo "❌ No encvgo .class files in kotlin-classes"
    
    echo "=== DEX content check (strings) ==="
    TMPDIR=$(mktemp -d)
    unzip -q "$APK_PATH" -d "$TMPDIR"
    if [ -f "$TMPDIR/classes.dex" ]; then
      strings "$TMPDIR/classes.dex" | grep -i "encvgo\|GoProcess" && \
        echo "✅ GoProcess/encvgo found in DEX strings" || \
        echo "❌ Not found in DEX strings"
    else
      echo "❌ classes.dex not found in APK!"
      ls "$TMPDIR/"*.dex 2>/dev/null || echo "No .dex files at all"
    fi
    rm -rf "$TMPDIR"
```

### 改动 2：post-cap-sync.mjs — 末尾增加包名验证

在现有 `console.log('done')` 之前追加：

```javascript
// --- 最终验证: 包名一致性 ---
for (const f of ['MainActivity.kt', 'GoProcessPlugin.kt']) {
  const fp = join(JAVA_DIR, f)
  if (existsSync(fp)) {
    const src = readFileSync(fp, 'utf-8')
    const pkgMatch = src.match(/^package\s+(\S+)/m)
    const pkg = pkgMatch ? pkgMatch[1] : 'NONE'
    if (pkg !== 'com.encvgo.app') {
      console.error(`  ERROR: ${f} has wrong package: "${pkg}" (expected "com.encvgo.app")`)
      process.exit(1)
    }
    console.log(`  pkg-ok: ${f} → ${pkg}`)
  }
}
```

## 四、执行后如何判断根因

跑一次 CI 后看 Pre-build diagnostic 和 Verify APK 输出：

| 场景 | Pre-build 输出 | Verify 输出 | 结论 | 下一步 |
|------|---------------|-------------|------|--------|
| A | .kt 文件存在，包名正确 | ✅ encvgo .class found / ✅ DEX strings found | **一切正常** | 问题在别处（旧APK缓存？安装错误？）|
| B | .kt 文件存在，包名正确 | ❌ No encvgo .class | **Kotlin 编译未包含我们的源码** | 检查 sourceSets 或 kotlin 插件配置 |
| C | .kt 文件不存在或路径错 | N/A | **post-cap-sync 复制失败** | 修复路径逻辑 |
| D | 包名不匹配 | N/A | **package 声明与目录不一致** | 修正 .kt 文件的 package 声明 |
| E | activity 未指向 MainActivity | N/A | **AndroidManifest 使用默认 Activity** | 确保 activity name 正确 |

## 五、如果场景 B（Kotlin 编译未包含源码）

那才是真正需要修的地方。在 post-cap-sync.mjs 的 build.gradle patch 中追加 sourceSets 显式声明：

```javascript
// 在 jvmTarget 配置块之后追加
if (!c.includes('sourceSets')) {
  c += `
android.sourceSets {
    main.java.srcDirs += 'src/main/java'
}
`
  console.log('  [kotlin] added explicit sourceSets for java/kotlin')
}
```
