# ComboLite 集成规范（来自官方 README + 源码审计 + 实战踩坑）

> **核心原则：0 Hook、0 反射。** ComboLite 是完全基于 Android 官方公开 API 构建的框架。
> **任何对 ComboLite API 使用反射的代码都是错误的，说明没有理解框架设计。**
> **ComboLite 内部使用 kotlin-reflect（`::function.javaMethod`）做权限检查，因此宿主构建必须保证字节码与 @Metadata 注解的一致性。**

---

## 一、铁律（违反 = 严重错误）

### 1.1 禁止反射调用 ComboLite API

**❌ 错误（幻觉代码）**：
```kotlin
// PluginManager 是 Kotlin object 单例，没有 getInstance(Context) 方法！
val pm = Class.forName("com.combo.core.runtime.PluginManager")
    .getMethod("getInstance", Context::class.java)
    .invoke(null, context)

// installPlugin 定义在 InstallerManager 上，不在 PluginManager 上！
val method = pm.javaClass.methods.find { it.name == "installPlugin" && it.parameterCount == 2 }
method.invoke(pm, apkFile, true)

// getAllInstallPlugins 是直接方法，不需要反射猜测方法名
val pluginsMethod = pm.javaClass.methods.find {
    it.name == "getInstalledPlugins" || it.name == "getLoadedPlugins" || ...
}
```

**✅ 正确（直接引用）**：
```kotlin
import com.combo.core.runtime.PluginManager

// object 单例，直接使用
if (PluginManager.isInitialized) {
    // 通过属性访问子管理器
    val result = PluginManager.installerManager.installPlugin(apkFile, true)
    val plugins = PluginManager.getAllInstallPlugins()
    val info = PluginManager.getPluginInfo("mpv-player")
}
```

### 1.2 禁止 Hook 任何系统服务

ComboLite 的核心价值就是 **0 Hook**。如果发现代码中有：
- `XposedHelpers.findAndHookMethod`
- `Instrumentation` 替换
- AMS/PMS 代理
- ClassLoader `pathList` 修改

 **立即删除**，这不是 ComboLite 的用法。

### 1.3 禁止在 release 构建中启用 R8/ProGuard（⚠️ 新增！）

> **ComboLite 框架内部使用 `::function.javaMethod`（kotlin-reflect API）做 @RequiresPermission 权限检查。**
> **R8 会破坏 ComboLite 类的字节码与 `@Metadata` 注解之间的一致性，导致 kotlin-reflect 无法解析函数签名。**
> **ComboLite 官方 demo (`app/build.gradle.kts`) 明确设置 `isMinifyEnabled = false`。**

#### ⚠️ isMinifyEnabled vs isShrinkResources — 两者机制完全不同！

| 选项 | 作用对象 | 对 ComboLite 的风险 | 约束 |
|------|---------|-------------------|------|
| `isMinifyEnabled` | **DEX 字节码**（重命名方法、删除未用代码、内联） | **致命**：破坏 `@Metadata ↔ 字节码` 一致性 | **必须 `false`** |
| `isShrinkResources` | **resources.arsc + 资源文件**（删除未引用的 drawable/layout/string 等） | **安全**：只读 DEX 不改 DEX，不触碰 `@Metadata` | **推荐 `true`**（减少 APK 体积 5%~20%） |

**为什么 `isShrinkResources=true` 安全**：
- ResourceShrinker 只扫描 DEX 中的 `R.xxx.yyy` 静态引用来判断资源是否被使用
- 它**只读不改** DEX — 不重命名、不删除方法、不修改 `@Metadata`
- 当 `isMinifyEnabled=false` 时，ResourceShrinker 工作在保守模式，功能正常
- 唯一注意点：如果有代码通过 `getIdentifier()` 动态按名称查找资源，需在 `res/raw/keep.xml` 中用 `tools:keep` 保护

**受影响的类（4个，全部使用 `::function.javaMethod`）：**

| 类 | 使用 javaMethod 的方法 |
|---|------|
| `PluginManager` | `setValidationStrategy`, `loadEnabledPlugins`, `launchPlugin`, `unloadPlugin`, `setPluginEnabled` |
| `InstallerManager` | `installPlugin`, `uninstallPlugin` |
| `PluginCrashHandler` | `setGlobalClashCallback`, `setClashCallback` |
| `AuthorizationManager` | `setAuthorizationHandler` |

**R8 破坏机制（2026-05-28 实测确认）：**

```
@Metadata 存在且描述原始名称:
  mv=[2,2,0], k=1

但 R8 将方法重命名:
  setValidationStrategy → a()
  参数类型 ValidationStrategy → d6.j0
  awaitInitialization([e])     ← 参数类型被混淆为单字母

→ kotlin-reflect 读 @Metadata 找 'setValidationStrategy' → 找不到
→ KotlinReflectionInternalError: Function 'xxx' not resolved in class ...
→ 所有带 @RequiresPermission 的 suspend 函数全部不可调用
→ validationStrategy 停留在 Strict（setValidationStrategy 从未成功执行）
```

**✅ 正确配置**：
```kotlin
// app/build.gradle.kts — release 构建
buildTypes {
    release {
        isMinifyEnabled = false          // ← 必须 false！R8 破坏 @Metadata
        isShrinkResources = true         // ← 推荐 true！安全且减少 APK 体积
        signingConfig = signingConfigs.getByName("release")
        proguardFiles(...)
    }
}
```

**❌ 错误配置**：
```kotlin
// 即使加了 keep 规则也不够！R8 对 suspend 函数的合成方法、
// continuation 参数类型、lambda 类的处理会破坏 @Metadata 一致性
isMinifyEnabled = true   // ← 会导致所有 ::function.javaMethod 失败
```

### 1.4 必须显式依赖 kotlin-reflect 且版本匹配项目 Kotlin 版本

ComboLite AAR 的 POM 声明 `kotlin-reflect:2.2.0` (runtime scope)，但 Kotlin Gradle plugin 会将其 align 到项目 Kotlin 版本（如 2.3.21）。如果宿主没有显式声明 `kotlin-reflect` 依赖：

1. Maven 可能无法从镜像解析该版本（502 Bad Gateway）
2. 运行时可能缺少 `kotlin-reflect` JAR 或版本 ABI 不兼容

**✅ 正确配置**：
```toml
# libs.versions.toml
kotlin = "2.3.21"
kotlin-reflect = { group = "org.jetbrains.kotlin", name = "kotlin-reflect", version.ref = "kotlin" }

# app/build.gradle.kts
dependencies {
    implementation(libs.kotlin.stdlib)
    implementation(libs.kotlin.reflect)      // ← 必须显式声明！
    implementation(libs.combolite.core)
}
```

---

## 二、架构认知（必须理解）

### 2.1 五大管理器职责与获取方式

| Manager | 类型 | 获取方式 | 职责 |
|---------|------|----------|------|
| **PluginManager** | `object` 单例 | 直接引用 `PluginManager` | 中心协调器：初始化、加载、卸载、查询 |
| **InstallerManager** | `class` | `PluginManager.installerManager` | 安装 APK、更新、签名校验、事务性卸载 |
| **ResourceManager** | `class` | `PluginManager.resourcesManager` | 插件资源加载/合并（lifecycleManager 自动调用） |
| **ProxyManager** | `class` | `PluginManager.proxyManager` | 四大组件代理（Activity/Service/Receiver/Provider） |
| **DependencyManager** | `internal class` | 仅通过 PluginManager 间接访问 | 类索引 O(1) 查找、依赖图维护 |

### 2.2 初始化时序

```
Application.onCreate()
  → BaseHostApplication.onCreate()        // 宿主继承此类
    → PluginCrashHandler.initialize()
    → PluginManager.initialize(context, onFrameworkSetup)  // ← onFrameworkSetup 在此回调内执行
      → 内部创建所有 Manager（installerManager, proxyManager, resourcesManager...）
      → 执行 onFrameworkSetup() 回调     // ← 此时所有 Manager 已可用
      → loadEnabledPlugins()             // 自动加载已启用的插件
```

**关键**：`onFrameworkSetup()` 是在 `initialize()` **内部**同步执行的回调，此时所有子管理器已经创建完成，可以安全调用 `proxyManager.setHostActivity()` 等。

### 2.3 安装流程正确路径

```
用户选择 APK 文件
  → 复制到临时目录
  → PluginManager.installerManager.installPlugin(apkFile, forceOverwrite=true)
    → 内部流程：签名验证 → 版本检查 → APK 复制 → so 解压 → 类索引创建 → 组件解析 → XML 持久化
    → 返回 InstallResult.Success(PluginInfo) 或 InstallResult.Failure(reason)
  → 如果 ValidationStrategy != Insecure 且签名不匹配
    → 启动 AuthorizationActivity（InstallPermissionScreen）
    → 用户确认后继续安装
```

### 2.4 权限检查机制（⚠️ 新增！必须理解）

ComboLite 在以下敏感 API 入口处自动进行权限检查：

```
调用者代码 → installPlugin(apkFile, true)
  → InstallerManager 内部第一行:
    if (::installPlugin.javaMethod?.checkApiCaller() == false) return
  → checkApiCaller() 流程:
    1. 获取 ::installPlugin.javaMethod 的 @RequiresPermission 注解
    2. 分析当前线程堆栈，跳过 com.combo.core.* 帧
    3. 在 PluginManager.getClassIndex() 中查找调用者类名
    4. 如果找到 → 是插件调用，走 AuthorizationManager 授权流程
       如果没找到 → 是宿主代码（caller=null），直接放行返回 true
```

**关键点**：
- `::function.javaMethod` 使用 **kotlin-reflect**（非 Java 反射），要求字节码与 `@Metadata` 一致
- 宿主代码（不在类索引中）始终通过权限检查 → `checkApiCaller()` 返回 `true`
- 如果 `::function.javaMethod` 本身抛异常（如 R8 破坏了一致性），整个 API 调用直接崩溃

---

## 三、宿主端集成 Checklist

### 3.1 Application 配置

- [ ] 继承 `BaseHostApplication`（非普通 `Application`）
- [ ] `onFrameworkSetup()` 中设置 `ValidationStrategy`（Insecure/UserGrant/Strict），**必须用 try-catch 包裹 Error 和 Exception**
- [ ] `onFrameworkSetup()` 中配置 `PluginManager.proxyManager.setHostActivity(HostActivity::class.java)`
- [ ] 如果插件需要 Service：配置 `setServicePool()`
- [ ] 如果插件需要 ContentProvider：配置 `setHostProviderAuthority()`

**⚠️ onFrameworkSetup 必须防御 kotlin-reflect 失败：**

```kotlin
override fun onFrameworkSetup(): suspend () -> Unit {
    return {
        // setValidationStrategy 使用 ::function.javaMethod，可能因 R8/kotlin-reflect 问题失败
        try {
            PluginManager.setValidationStrategy(ValidationStrategy.Insecure)
            Log.i(TAG, "onFrameworkSetup: setValidationStrategy(Insecure) OK")
        } catch (e: Error) {   // ← 必须捕获 Error（不是 Exception）！
            Log.w(TAG, "onFrameworkSetup: setValidationStrategy failed: ${e.javaClass.simpleName}: ${e.message}")
            // 不要 rethrow，让其他初始化继续
        } catch (e: Exception) {
            Log.w(TAG, "onFrameworkSetup: setValidationStrategy FAILED", e)
        }
        // ... 其他初始化
    }
}
```

### 3.2 AndroidManifest.xml

- [ ] 声明 HostActivity 子类（`exported="false"`）
- [ ] 声明 HostService 子类（如需）
- [ ] 声明 FileProvider（APK 安装用）

### 3.3 build.gradle.kts（宿主 :app）— ⚠️ 关键约束！

```kotlin
plugins {
    alias(libs.plugins.combolite.aar2apk)    // aar2apk 打包插件
}

android {
    buildTypes {
        release {
            isMinifyEnabled = false             // ⚠️ 必须 false！R8 破坏 @Metadata
            isShrinkResources = true            // ✅ 推荐 true！安全且减少 APK 体积
            // ... signing config
        }
    }
}

packagePlugins {                                // CI 构建时应禁用（插件由单独步骤构建）
    enabled.set(false)                         // 开发调试时可设为 true
    buildType.set(PackageBuildType.DEBUG)
    pluginsDir.set("debug_plugins")
}

dependencies {
    implementation(libs.kotlin.stdlib)
    implementation(libs.kotlin.reflect)         // ⚠️ 必须显式声明，版本匹配项目 Kotlin
    implementation(libs.combolite.core)         // 核心 runtime
}
```

### 3.4 build.gradle.kts（插件模块）

```kotlin
plugins {
    alias(libs.plugins.combolite.aar2apk)    // 同样需要 aar2apk
}

dependencies {
    compileOnly(libs.combolite.core)         // ⚠️ 必须是 compileOnly，运行时由宿主提供
}
```

### 3.5 插件 AndroidManifest.xml

```xml
<application>
    <!-- 入口类声明 -->
    <meta-data android:name="plugin.entryClass"
               android:value="com.example.MyPluginEntry" />
    
    <!-- 插件 Activity（exported=false，通过 ProxyManager 代理启动）-->
    <activity android:name=".MyActivity"
              android:exported="false"
              android:launchMode="singleTop" />
</application>
```

### 3.6 插件入口类实现

```kotlin
class MyPluginEntry : IPluginEntryClass {
    override fun onLoad(context: PluginContext) { /* 初始化 */ }
    override fun onUnload() { /* 清理 */ }
    override val pluginModule: List<Module> = emptyList() // Koin 模块（可选）
}
```

---

## 四、常见错误模式（警惕！）

### 错误 A：「反射万能」思维

> **症状**：遇到不熟悉的 API 就用反射绕过
> **根因**：没有阅读源码或文档，不理解 Kotlin object / 属性暴露 / sealed class
> **后果**：代码看似能编译但永远走 fallback 分支，浪费数天调试时间

### 错误 B：「在错误对象上找方法」

> **症状**：在 `PluginManager` 上搜 `installPlugin`，但它定义在 `InstallerManager` 上
> **根因**：没有看源码的类结构，凭直觉猜 API 归属
> **修复**：先读源码的 `object PluginManager` 声明的公开属性

### 错误 C：「忽略 suspend 函数」

> **症状**：直接在主线程调用 `installPlugin()` 导致 ANR
> **根因**：`installPlugin` 是 `suspend fun`（内部有文件 IO 和签名校验）
> **修复**：必须用协程 `GlobalScope.launch(Dispatchers.IO) { ... }` 调用

### 错误 D：「忘记处理返回值」

> **症状**：调用 `installPlugin()` 后不检查 `InstallResult`，默认成功
> **根因**：把 `suspend fun` 当 `void` 用
> **修复**：`when (result) { is Success -> ...; is Failure -> ... }`

### 错误 E：「用系统安装器安装插件 APK」

> **症状**：`PluginManager.isInitialized == false` 时走 `Intent.ACTION_INSTALL_PACKAGE` 兜底
> **根因**：把 ComboLite 插件 APK 当作普通 Android 应用
> **后果**：系统安装器要么失败（插件无 launcher Activity），要么装出一个无法运行的独立应用
> **修复**：`PluginManager.isInitialized == false` 时直接 reject，绝不走系统安装兜底

### 错误 F：「文件扫描代替 PluginManager 查询」

> **症状**：`PluginManager.isInitialized == false` 时扫描文件系统查找 APK 文件
> **根因**：认为"文件存在 = 插件已安装"
> **后果**：假阳性——文件存在不代表插件已通过 ComboLite 正确安装（需要签名校验、类索引创建、组件解析、XML 持久化）
> **修复**：`PluginManager.isInitialized == false` 时返回空结果，不使用文件扫描兜底

### 错误 G：「启用 R8/ProGuard 并期望 -keep 规则足够」（⚠️ 新增！实战踩坑）

> **症状**：release 构建 `isMinifyEnabled=true`，添加 `-keep class com.combo.core.** { *; }` 后仍崩溃
> **错误信息**：`KotlinReflectionInternalError: Function 'xxx' not resolved in class ...`
> **根因**：R8 即使 keep 了类名和方法名，仍可能修改 suspend 函数的合成方法名（`$default`）、continuation 参数类型（`g6/e` → `d6.j0`）、lambda 类名，破坏 `@Metadata` 与实际字节码的一致性
> **证据**：饱和调试日志显示部分方法保留原名（`getInterfaceFromHost`）、部分被混淆（`a()`、`e()`），`@Metadata` 仍描述原始名称
> **修复**：**禁用 R8**（`isMinifyEnabled = false`），与 ComboLite 官方 demo 保持一致。注意：`isShrinkResources=true` 不受此限制，可安全开启以减小 APK 体积

### 错误 H：「遗漏 kotlin-reflect 依赖」（⚠️ 新增！）

> **症状**：CI 构建报错 `Could not resolve org.jetbrains.kotlin:kotlin-reflect:2.3.21`（aliyun mirror 502）
> **或运行时**：`NoClassDefFoundError: kotlin/reflect/jvm/KFunction` 或 `KotlinReflectionInternalError`
> **根因**：ComboLite POM 声明 `kotlin-reflect:2.2.0` (runtime)，Gradle align 到项目版本 2.3.21，但未显式声明依赖
> **修复**：`implementation(libs.kotlin.reflect)` + 确保 mavenCentral() 在仓库列表首位

---

## 五、插件文件格式

### 5.1 扩展名：.apk（不可更改）

ComboLite 插件文件使用 `.apk` 扩展名，原因：

1. **文件本质就是 APK**：标准 ZIP 容器 + AndroidManifest.xml + DEX + resources.arsc + so 库，有合法的 Android 签名
2. **`InstallerManager.installPlugin(File)` 不检查扩展名**：内部通过 `PackageManager.getPackageArchiveInfo()` 和 `ZipFile` 解析文件内容
3. **安装后源文件被重命名为 `base.apk`**：`copyPluginApk()` 将源文件复制到 `plugins/{pluginId}/base.apk`，原始文件名无关
4. **aar2apk 构建工具硬编码输出 `.apk`**：`ConvertAarToApkTask.kt` 输出 `${pluginName}-${buildType}.apk`
5. **ComboLite 官方示例全部使用 `.apk`**：`updates/plugins/` 目录下所有插件文件

**禁止**将插件文件扩展名改为 `.plugin`、`.cpk` 等——这不会改变文件内容，只会破坏与 ComboLite 工具链的兼容性。

### 5.2 插件不是普通应用

虽然插件文件扩展名是 `.apk`，但它**不是普通 Android 应用**：
- 不能用 `Intent.ACTION_INSTALL_PACKAGE` 系统统安装器安装
- 不能通过 `adb install` 安装
- 只能通过 `PluginManager.installerManager.installPlugin()` 安装
- 安装后存储在 `filesDir/plugins/{pluginId}/base.apk`，不在系统应用目录

---

## 六、CI 构建中的插件分离原则

### 6.1 核心原则：插件 APK 不打包进主 APK

ComboLite 插件是运行时动态加载的，不应嵌入主 APK 的 assets 中。CI 构建必须将主应用和插件作为**独立的构建步骤**分离。

### 6.2 `packagePlugins` 必须禁用

```kotlin
// app/build.gradle.kts
packagePlugins {
    enabled.set(false)   // CI 构建时必须禁用！
    buildType.set(PackageBuildType.DEBUG)
    pluginsDir.set("debug_plugins")
}
```

**为什么必须禁用**：
- `enabled.set(true)` 会让 aar2apk 在 `assembleDebug`/`assembleRelease` 时自动构建插件并将 APK 放入 `debug_plugins/`（最终打包进主 APK 的 assets）
- 这导致：① 主 APK 体积膨胀 ② 插件被重复构建 ③ 插件 APK 不应随主 APK 分发
- 仅在本地开发调试时可以临时设为 `true`

### 6.3 CI 构建步骤分离

正确的 CI 构建顺序：

```
Step 1: 构建 JNI 库（如 libplayer.so）
Step 2: 构建主应用 APK（assembleDebug / assembleRelease）
         → 不触发插件构建（packagePlugins disabled）
Step 3: 编译插件 Kotlin 代码（:plugin-mpv-player:compileDebugKotlin）
Step 4: 转换插件 AAR→APK（convert_plugin-mpv-player_debug）
Step 5: 验证插件 APK 内容
```

**关键 Gradle 任务**：
- `:plugin-mpv-player:compile${BuildType}Kotlin` — 编译插件代码
- `convert_plugin-mpv-player_${buildType}` — AAR→APK 转换（aar2apk 提供）
- 插件 APK 输出路径：`build/` 目录下搜索 `plugin-mpv-player-${buildType}.apk`

### 6.4 插件分发方式

插件 APK 通过以下方式分发到用户设备：
1. **CI 产物**：插件 APK 作为独立构建产物上传
2. **运行时安装**：用户通过应用内文件选择器选择插件 APK → `PluginManager.installerManager.installPlugin()` 安装
3. **安装位置**：`filesDir/plugins/{pluginId}/base.apk`（应用私有目录，非系统应用目录）

### 6.5 错误 G：「主应用构建时自动打包插件」

> **症状**：`assembleDebug` 触发了 `:plugin-mpv-player:assembleDebug` 和 `:convert_plugin-mpv-player_debug`
> **根因**：`packagePlugins { enabled.set(true) }` 让 aar2apk 在主应用构建时自动执行插件打包
> **后果**：插件 APK 被嵌入主 APK 的 assets，体积膨胀 + 重复构建
> **修复**：`packagePlugins { enabled.set(false) }`，插件由 CI 单独步骤构建

---

## 七、源码参考（克隆至 `/tmp/ComboLite`）

关键文件速查：

| 文件 | 内容 |
|------|------|
| `comboLite-core/.../runtime/PluginManager.kt` | object 单例，所有属性入口 |
| `comboLite-core/.../runtime/installer/InstallerManager.kt` | installPlugin/uninstallPlugin |
| `comboLite-core/.../runtime/app/BaseHostApplication.kt` | 宿主 Application 基类 |
| `comboLite-core/.../component/activity/BaseHostActivity.kt` | 宿主 Activity 代理基类 |
| `comboLite-core/.../proxy/ProxyManager.kt` | 组件代理调度 |
| `comboLite-core/.../runtime/resource/PluginResourcesManager.kt` | 资源合并 |
| `comboLite-core/.../ui/InstallPermissionScreen.kt` | 安装确认界面（Compose） |
| `comboLite-core/.../security/auth/AuthorizationActivity.kt` | 授权 Activity 容器 |
| `comboLite-core/.../runtime/lifecycle/PluginLifecycleManager.kt` | 加载/卸载/重启全流程 |
| `comboLite-core/.../security/permission/PermissionExt.kt` | checkApiCaller 权限检查扩展 |
| `comboLite-core/consumer-rules.pro` | ComboLite 自带的 ProGuard 规则（⚠️ demo 未启用 R8，此规则未经完整测试） |
| `app/build.gradle.kts` | Demo 配置参考（`isMinifyEnabled = false`） |

---

## 八、调试技巧

1. **`PluginManager.isInitialized`** — 第一步检查这个，未初始化时所有操作无效
2. **Logcat 过滤**：`TAG="ENCV-go"` 或 `TAG="PluginLifecycleManager"`
3. **安装失败时**：检查 `InstallResult.Failure.reason` 和 `.exception?.stackTraceToString()`
4. **Activity 无法启动时**：检查 ProxyManager 是否配置了 setHostActivity + Manifest 是否声明了 HostActivity
5. **资源找不到时**：检查 ResourceManager 是否被 lifecycleManager 自动触发（loadPlugin L230）
6. **kotlin-reflect 崩溃时**：检查 `isMinifyEnabled` 是否为 false；检查 `kotlin-reflect` 依赖是否显式声明且版本匹配
7. **validationStrategy 异常时**：检查 EncvApplication.onFrameworkSetup 日志中是否有 setValidationStrategy 相关输出；如果日志为空说明 catch 吞掉了错误
8. **饱和调试按钮**（ExtensionsPage 页面）：4 个诊断按钮覆盖全部故障点
   - 🔧 installPlugin实际调用 — 测试完整安装流程
   - 🔧 kotlin-reflect健康检查 — 测试 4 个类的 ::function.javaMethod 是否可解析
   - 🔧 APK元数据+签名校验 — 验证 APK 结构和签名
   - 🔧 ValidationStrategy状态 — 验证策略是否真正生效
