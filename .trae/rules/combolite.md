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

#### ⚠️ isMinifyEnabled 与 isShrinkResources — AGP 硬耦合！

**重要：AGP（Android Gradle Plugin）强制要求两者必须同时开启或同时关闭！**

| 选项 | 作用对象 | 对 ComboLite 的风险 | 约束 |
|------|---------|-------------------|------|
| `isMinifyEnabled` | **DEX 字节码**（重命名方法、删除未用代码、内联） | **致命**：破坏 `@Metadata ↔ 字节码` 一致性 | **必须 `false`** |
| `isShrinkResources` | **resources.arsc + 资源文件**（删除未引用资源） | 理论上安全（只读 DEX 不改字节码），但... | **必须 `false`**（见下文） |

**为什么 `isShrinkResources=true` 不能单独使用（CI 实测确认）：**

AGP 源码级硬约束（`AndroidResourcesCreationConfigImpl.kt:91`）：
```kotlin
// AGP internal check — 无法绕过
if (!buildType.isMinifyEnabled && androidResources.shrink) {
    issueReporter.reportError(
        "Removing unused resources requires unused code shrinking to be turned on."
    )
}
```

ResourceShrinker 的依赖图分析需要 R8/ProGuard 先生成完整的类→资源映射文件才能工作。当 `isMinifyEnabled=false` 时，AGP 在配置阶段直接抛出 `EvalIssueException`，构建无法继续。

**结论：由于 ComboLite 要求 `isMinifyEnabled=false`，`isShrinkResources` 也必须为 `false`。两者是 AGP 强耦合的。**

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
        isShrinkResources = false         // ← 必须 false！AGP 硬耦合要求
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

// 同样错误！AGP 强制要求 shrinkResources 必须配合 minifyEnabled
isMinifyEnabled = false
isShrinkResources = true  // ← EvalIssueException: requires code shrinking!
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

### 1.6 宿主禁止编译期依赖任何扩展模块（⚠️ CI 踩坑！）

> **核心原则：宿主（`:app`、`combolite-host`）不得声明 `implementation(project(":plugin-*"))`。**
> **扩展是运行时通过 ComboLite `installPlugin()` 动态加载的独立进程 APK，宿主构建时根本不知道它是否存在。**

#### 为什么不能依赖

| 维度 | 说明 |
|------|------|
| **构建隔离** | CI 主应用构建不带 `-PincludePlugins=true`，settings.gradle.kts 不 include plugin project → `UnknownProjectException` 崩溃 |
| **架构正确性** | MPV 扩展零宿主依赖，OpenList 也应同样。宿主与扩展之间只通过 ContentProvider / Intent / 文件系统通信 |
| **aar2apk 级联** | 根 build.gradle.kts 的 aar2apk 模块注册和 combolite-host 的 dependencies 两处都必须无 plugin 引用，否则任一处都会在配置阶段崩溃 |

#### 正确的跨进程通信方式

```
宿主进程 (:app / combolite-host)          扩展进程 (:plugin-xxx, aar2apk 产物)
┌──────────────────────┐              ┌──────────────────────┐
│ OpenListStatusBridge  │──ContentResolver.query()──▶│ OpenListStatusProvider│
│ (内联 URI/列名常量)   │◀──MatrixCursor snapshot ──│ (exported provider)  │
│                      │──ContentResolver.insert()─▶│                     │
│ LocalBroadcastManager │◀──sendBroadcast() ───────│ OpenListBridge       │
└──────────────────────┘              └──────────────────────┘
```

#### 常量同步规则

如果宿主需要引用扩展定义的常量（如 ContentProvider authority、URI、列名）：

| 方式 | 正确性 | 示例 |
|------|--------|------|
| ❌ `implementation(project(":plugin-openlist"))` | **编译期耦合，CI 必崩** | import OpenListStatusProvider.STATUS_URI |
| ✅ **常量内联到宿主 bridge** | **零编译期依赖** | bridge 内部 `const val AUTHORITY = "..."` |
| ✅ **接口定义在 combolite-core** | **双方依赖同一抽象** | 如果是通用协议 |

#### 已踩坑案例

**combolite-host/build.gradle.kts:31** — `implementation(project(":plugin-openlist"))`：
- 只为了用 `OpenListStatusProvider` 的 6 个字符串常量（AUTHORITY、URI、列名）
- MPV 没有这种依赖 → OpenList 也不应该有
- CI 不传 `-PincludePlugins=true` 时 → `UnknownProjectException: Project with path ':plugin-openlist' could not be found`
- **修复**：将常量内联到 `OpenListStatusBridge.kt`，彻底移除 dependency

### 1.7 EncvHostActivity 透明主题陷阱（⚠️ 实战踩坑！）

> **使用透明主题的 HostActivity 如果 ProxyManager 代理失败，
> 用户会看到一个不可见的 Activity 覆盖在 WebView 上，表现为"卡住"。**

**症状**：`startActivity` 成功、无崩溃日志、无错误回调、触摸无响应

**根因链**：
```
EncvHostActivity(Theme.Translucent.NoTitleBar)
  → BaseHostActivity.onCreate() 执行完成（proxyStarted=true）✅
  → ProxyManager 代理启动目标插件 Activity 失败
  → BaseHostActivity 显示空白布局（全透明 → 用户看不到任何东西）
  → Activity 仍在栈顶拦截触摸事件 → "卡住"
  → startActivityForResult 的 pending call 永远不 resolve → 前端 await 永久挂起
```

**必须的防御措施（4 层）**：

| 层级 | 措施 | 文件 | 效果 |
|------|------|------|------|
| L1 可见性 | 半透明主题 `#CC000000` 而非全透明 | `styles.xml` | 用户能看到 Activity 存在 |
| L2 超时检测 | `onPostCreate` + Handler.postDelayed(5s) | `EncvHostActivity.kt` | proxy 未启动自动 finish+setResult |
| L3 onResume 诊断 | 时间戳差值 + proxyStarted 检查 | `EncvHostActivity.kt` | 日志暴露卡在哪个阶段 |
| L4 Promise 兜底 | GoProcessPlugin 端 15s 超时 resolve | `GoProcessPlugin.kt` | 前端不会永久挂起 |

**正确配置**：
```xml
<!-- styles.xml -->
<style name="Theme.EncvHostTranslucent" parent="@android:style/Theme.Translucent.NoTitleBar">
    <item name="android:windowBackground">#CC000000</item>
    <item name="android:windowIsTranslucent">true</item>
    <item name="android:windowContentOverlay">@null</item>
</style>
```

```kotlin
// EncvHostActivity.kt — 必须包含的超时机制
companion object {
    const val PROXY_TIMEOUT_MS = 5000L
}

override fun onPostCreate(savedInstanceState: Bundle?) {
    super.onPostCreate(savedInstanceState)
    if (!proxyStarted) {
        Handler(Looper.getMainLooper()).postDelayed({
            if (!proxyStarted && !isFinishing && !resultSet) {
                finishWithResult(pluginId, false, "播放器启动超时", "...")
            }
        }, PROXY_TIMEOUT_MS)
    }
}
```

```kotlin
// GoProcessPlugin.kt — Promise 超时兜底
Handler(Looper.getMainLooper()).postDelayed({
    if (pendingCalls.containsKey("mpvPlayer")) {
        pendingCalls.remove("mpvPlayer")?.resolve(timeoutErrorJSObject)
    }
}, 15000)
```

**反模式（禁止）**：
- ❌ 全透明主题 + 无超时检测 = 用户以为 app 死了
- ❌ 依赖 onActivityResult 回调而不做超时兜底 = Promise 永远 pending
- ❌ 不记录 onCreate→onResume 时间戳 = 无法诊断卡在哪一步

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
            isShrinkResources = false            // ⚠️ 必须 false！AGP 硬耦合
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
> **修复**：**禁用 R8**（`isMinifyEnabled = false`），与 ComboLite 官方 demo 保持一致。注意 `isShrinkResources` 也必须为 `false`（AGP 硬耦合，见 §1.3）

### 错误 H：「遗漏 kotlin-reflect 依赖」（⚠️ 新增！）

> **症状**：CI 构建报错 `Could not resolve org.jetbrains.kotlin:kotlin-reflect:2.3.21`（aliyun mirror 502）
> **或运行时**：`NoClassDefFoundError: kotlin/reflect/jvm/KFunction` 或 `KotlinReflectionInternalError`
> **根因**：ComboLite POM 声明 `kotlin-reflect:2.2.0` (runtime)，Gradle align 到项目版本 2.3.21，但未显式声明依赖
> **修复**：`implementation(libs.kotlin.reflect)` + 确保 mavenCentral() 在仓库列表首位

### 错误 I：「插件 implementation 共享依赖导致 PluginDependencyException」（⚠️ 新增！实战踩坑）

> **症状**：`com.combo.core.exception.PluginDependencyException: 插件 [xxx] 依赖的类 [androidx.compose.material.icons.filled.PauseKt] 未找到`
> **根因**：ComboLite 的 `PluginClassLoader` 继承 `DexClassLoader`，对插件自行 `implementation` 打包的某些库类解析不稳定。当宿主和插件都包含同一库的不同版本或解析路径时，插件的 ClassLoader 无法正确找到该类。
> **修复原则**：**宿主提供运行时（implementation）+ 插件仅编译时引用（compileOnly，不指定版本）**

#### 正确配置模式

```kotlin
// ===== 插件模块 build.gradle.kts =====
dependencies {
    compileOnly(libs.combolite.core)                    // ComboLite 核心：compileOnly
    compileOnly("androidx.core:core-ktx")               // 不指定版本！让 Gradle 从已有 implementation 传递解析
    compileOnly("androidx.activity:activity-ktx")       // 同上
    compileOnly("org.jetbrains.kotlinx:kotlinx-coroutines-android")  // 同上
    compileOnly("androidx.compose.material:material-icons-extended") // 同上
    // ... 其他由宿主提供的共享依赖同理
}

// ===== 宿主 :app build.gradle.kts =====
dependencies {
    implementation("androidx.core:core-ktx:1.17.0")           // 宿主提供运行时
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")
    implementation("androidx.activity:activity-ktx:1.11.0")
    implementation("androidx.compose.material:material-icons-extended")
    // ...
}
```

#### 关键规则

| 规则 | 说明 |
|------|------|
| **插件用 `compileOnly` 不带版本号** | 让 Gradle 从模块内已有的 `implementation` 依赖传递链自动解析版本，避免 `{strictly ...}` 版本冲突 |
| **宿主用 `implementation` 带具体版本** | 宿主 PathClassLoader 加载这些类，AndroidX 向后兼容 |
| **禁止插件对同一库同时使用 `implementation` 和 `compileOnly`** | `implementation` 已传递拉入该库，再 `compileOnly` 声明不同版本必冲突 |
| **`compileOnly` 不代表"不参与版本解析"** | Gradle 仍需在 compileClasspath 解析，必须与传递依赖图一致 |

#### 已确认需要此模式的依赖清单

| 依赖 | 触发错误的类示例 |
|------|-----------------|
| `material-icons-extended` | `PauseKt`, `PlayArrowKt`, `VolumeUpKt` 等 Icon 对象单例 |
| `core-ktx` | `WindowCompat`, `WindowInsetsCompat`, `WindowInsetsControllerCompat` |
| `activity-ktx` | Activity 扩展函数 |
| `coroutines-android` | `delay`, `launch`, `flow.*` 协程 API |

#### 反模式（禁止）

```kotlin
// ❌ 插件用 implementation 打包共享依赖 → PluginDependencyException
implementation("androidx.compose.material:material-icons-extended")

// ❌ compileOnly 指定版本 → 与 implementation 传递链冲突
//    错误: Cannot find a version satisfying {strictly 1.13.0} vs declared 1.17.0
compileOnly("androidx.core:core-ktx:1.17.0")

// ✅ 正确：compileOnly 不指定版本
compileOnly("androidx.core:core-ktx")
```

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

---

## 九、IPluginEntryClass 实际接口契约（combolite-core 2.0.2 源码审计）

> 本节为插件开发者直接提供。源码来自 Maven Central `io.github.lnzz123:combolite-core:2.0.2`。
> **不要凭印象猜**——任何「我以为 `Content()` 可以是非 Composable」「IPluginService 必须实现」等假设，先读 `/tmp/combolite-src/com/combo/core/api/` 实际文件再下结论。

### 9.1 IPluginEntryClass（**唯一强契约入口**）

```kotlin
// combolite-core 2.0.2 /com/combo/core/api/IPluginEntryClass.kt
package com.combo.core.api

import androidx.compose.runtime.Composable
import com.combo.core.model.PluginContext
import org.koin.core.module.Module

interface IPluginEntryClass {
    val pluginModule: List<Module>      // 必含: org.koin.core.module.Module 类型
    fun onLoad(context: PluginContext)   // 必含: 加载时初始化
    fun onUnload()                       // 必含: 卸载时清理
    @Composable
    fun Content()                        // 必含 + 必为 @Composable, 无替代入口
}
```

**PluginContext**（`/com/combo/core/model/PluginContext.kt`）：
```kotlin
data class PluginContext(
    val application: Application,
    val pluginInfo: PluginInfo
)
```

### 9.2 4 个 plugin 入口契约点

| 入口点 | 类型 | 必含? | 实现复杂度 |
|--------|------|-------|----------|
| `pluginModule` | `List<org.koin.core.module.Module>` | ✅ | 可为 `emptyList()` |
| `onLoad(context)` | `fun` | ✅ | 自由实现 |
| `onUnload()` | `fun` | ✅ | 自由实现 |
| `Content()` | `@Composable fun` | ✅ | **必为 @Composable**（不可省注解） |

### 9.3 可选入口（meta-data 声明）

| 接口 | Meta-data 写法 | 何时需要 |
|------|--------------|---------|
| `IPluginActivity` | `plugin.activities` (XmlManager) | 插件要提供 Activity 容器 |
| `IPluginService` | `plugin.services` (XmlManager) | 插件要提供 Service **且需要 host 代理** |
| `IPluginReceiver` | `plugin.staticReceivers` (XmlManager) | 插件要接收广播且需 host 代理 |
| `IPluginProvider` | `plugin.providers` (XmlManager) | 插件要提供 ContentProvider 且需 host 代理 |

> **OPENLIST 例外**（phase 13 经验）：用**普通 Android `Service` / `ContentProvider`**（manifest 里直接 `<service>` / `<provider>`），插件自己管生命周期（`onLoad`/`onUnload` 启动/停止 Service），**不需要** `IPluginService` / `IPluginProvider` proxy 路径。这避开了 host proxy 的复杂度，适合单一进程插件。

### 9.4 ClassLoader 拓扑（决定 deps 用 `compileOnly` 还是 `implementation`）

combolite-core `PluginLifecycleManager.kt:224`：
```kotlin
val classLoader = PluginClassLoader(
    pluginId = plugin.id,
    pluginFile = pluginApkFile,
    parent = context.application.classLoader,    // ← 父 classloader = host 的
    ...
)
```

**结论**：插件运行时**通过 parent classloader 委托给 host**——host 已 `implementation` 的任何依赖，插件**可用 `compileOnly`**，运行时由 parent 解析。

| 依赖类型 | 何时 `compileOnly` | 何时 `implementation` |
|---------|-------------------|----------------------|
| combolite-core (api jar) | ✅ host 已 implementation | — |
| androidx.core:core-ktx | ✅ host 已 implementation | — |
| compose-ui | ✅ host 已 implementation | — |
| koin-core (类型引用) | ✅ host 启动 Koin | — |
| **localbroadcastmanager** | ❌ host **没有** | ✅ 必须 implementation |
| **gomobile classes.jar** | ❌ host **没有** | ✅ 必须 implementation |
| **OpenList 自定义 jar/AAR** | ❌ host **没有** | ✅ 必须 implementation |

### 9.5 build.gradle.kts 最小骨架（基于实际契约）

```kotlin
plugins {
    id("com.android.library")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")   // ← 强契约: Content() 是 @Composable
    alias(libs.plugins.combolite.aar2apk)      // ← 强契约: 走 aar2apk 输出 plugin APK
}

android {
    namespace = "com.example.plugin"
    compileSdk = libs.versions.compileSdk.get().toInt()
    defaultConfig { minSdk = libs.versions.minSdk.get().toInt() }
    buildTypes {
        release {
            isMinifyEnabled = false        // ← 强契约: R8 破坏 kotlin-reflect @Metadata
            isShrinkResources = false      // ← 强契约: AGP 与 minify 硬耦合
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_21
        targetCompatibility = JavaVersion.VERSION_21
    }
    buildFeatures {
        compose = true                    // ← 强契约: @Composable 编译
    }
}

dependencies {
    compileOnly(libs.combolite.core)         // 由 host 提供
    // ↓ 只加本插件实际需要的 deps（按 §9.4 决定 compileOnly vs implementation）
}
```

### 9.6 锁镜警告（mpv ≠ openlist）

**禁止**直接把 plugin-mpv-player 的 deps 照搬到新插件。mpv 的 deps 反映 mpv 的功能：

| mpv 用的 | openlist 用的 | 结论 |
|---------|-------------|------|
| material3 | ❌（只用 AndroidView） | 不锁镜 |
| material-icons-extended | ❌ | 不锁镜 |
| activity-compose | ❌（无 Activity） | 不锁镜 |
| appcompat | ❌ | 不锁镜 |
| ❌ | localbroadcastmanager | openlist 独有 |
| ❌ | openlist-classes.jar (gomobile) | openlist 独有 |

**正确做法**：从 `IPluginEntryClass` 4 契约点出发 → 看本插件用哪些组件（Service/Receiver/Provider/Compose widget）→ 按 §9.4 决定 deps → 写 `build.gradle.kts`。

### 9.7 plugin.openlist 真实形态（spec 落地参照）

```kotlin
// OpenListPluginEntry.kt
class OpenListPluginEntry : IPluginEntryClass {
    override val pluginModule: List<Module> = emptyList()
    override fun onLoad(context: PluginContext) {
        // 1. 加载 OpenListConfig
        // 2. cfg.applyToBridge(OpenListBridge)
        // 3. OpenListBridge.init(context.applicationContext)
    }
    override fun onUnload() {
        OpenListService.stopIfRunning()
        OpenListBridge.shutdown(5_000L)
    }
    @Composable
    override fun Content() {
        OpenListEmbedWebView(
            containerId = "openlist-plugin-embed",
            initialUrl = "https://localhost/openlist/"   // dev 模式
        )
    }
}
```

`AndroidManifest.xml`：
- `<meta-data android:name="plugin.entryClass" android:value="...OpenListPluginEntry" />`
- `<service .OpenListService android:foregroundServiceType="dataSync" />`（普通 Service，非 IPluginService proxy）
- `<provider .OpenListStatusProvider android:exported="true" />`（普通 Provider，供外部 observer）
- **不需要** `plugin.activities` / `plugin.services` / `plugin.receivers` meta-data

---

## 十、Service / Receiver / Provider 三大模块架构铁律（⚠️ Phase 24 demo 研究，方案 A 决策）

> **核心原则：ComboLite 是文件系统 + PluginClassLoader 架构，插件 APK 不被系统 install。**
> **所有"Android 组件"在插件端都是纯类，由 host 端通过 ProxyManager 反射调度。**
> **插件 manifest 的 `<service>` / `<receiver>` / `<provider>` 声明只用于被框架解析，不被 Android 系统看到。**

### 10.1 三模块 host 端必备清单（demo 实测验证）

来源：lnzz123/ComboLite `app/src/main/AndroidManifest.xml` + `HostApp.kt`。

| 组件 | host manifest 注册数 | host class（demo） | proxyManager 配置 |
|------|-------------------|------------------|------------------|
| **Activity** | 1 | `HostActivity : BaseHostActivity` | `setHostActivity(HostActivity::class.java)` |
| **Service 池** | 10 | `HostService1..10 : BaseHostService` | `setServicePool(listOf(HostService1, ..., HostService10))` |
| **Provider** | 1 | `HostProvider : BaseHostProvider` | `setHostProviderAuthority("authority")` |
| **Receiver** | 1 | `HostReceiver : BaseHostReceiver` | **不需要** setReceiverXxx——框架按 intent-filter action 匹配 |

**HostApp.onFrameworkSetup() 标准模板**（[ComboLite demo HostApp.kt:54-72](https://raw.githubusercontent.com/lnzz123/ComboLite/master/app/src/main/java/com/combo/plugin/sample/HostApp.kt)）：

```kotlin
override fun onFrameworkSetup(): suspend () -> Unit {
    return {
        PluginManager.proxyManager.apply {
            setHostActivity(HostActivity::class.java)              // ① Activity 代理
            setServicePool(                                       // ② Service 池（demo = 10）
                listOf(HostService1::class.java, ..., HostService10::class.java)
            )
            setHostProviderAuthority("com.example.host.provider") // ③ Provider 代理 authority
        }
        setValidationStrategy(ValidationStrategy.UserGrant)
        PluginCrashHandler.setGlobalClashCallback(this@HostApp)
    }
}
```

**关键认知**：
- Service 必须**预注册 N 个**——每个 plugin service 启动时分配一个 host service 代理（用完归还，pool 模式）
- Provider **只需要 1 个**，但必须注册 authority——host ContentResolver 用 authority 路由到 plugin provider
- Receiver **只需要 1 个**，用 intent-filter action 匹配——无 authority 概念
- Activity 1 个即可——`startPluginActivity(cls)` 通过 host activity 的 intent extras 路由

### 10.2 插件端 manifest 真相

| 组件 | plugin manifest 需声明吗？ | manifest 声明的实际作用 |
|------|--------------------------|---------------------|
| **Service** | ❌ **不需要** `<service>` 标签 | N/A —— 框架 `PluginClassLoader.getInterface(IPluginService::class.java, className).newInstance()` 创建实例，host service 反射调用 lifecycle |
| **Receiver** | ✅ 需要 `<receiver>` 标签 + `<intent-filter>` | [InstallerManager.parseStaticReceivers](file:///tmp/combolite-src/com/combo/core/runtime/installer/InstallerManager.kt) 用 `AXmlResourceParser` 解析插件 APK 的 `AndroidManifest.xml`，存到 `plugins.xml` 的 `staticReceivers` 列表 |
| **Provider** | ✅ 需要 `<provider>` 标签 + `authorities` | 框架解析 authority + className 存到 `providerRegistry` |

**反直觉点**：插件 manifest 的 `<receiver>` / `<provider>` **Android 系统完全看不到**（plugin 没系统 install），所以 `<receiver>` 的 `android:exported`、`android:enabled` 等属性是给 **ComboLite 框架** 用的，不是给系统用的。

### 10.3 启动插件 Service 的标准模式

[ComboLite core Extensions.kt:102-119](file:///tmp/combolite-src/com/combo/core/utils/Extensions.kt#L102-L119)：

```kotlin
// host 端：启动 plugin service
context.startPluginService(MyPluginService::class.java, instanceId = "task1")
```

**内部链路**：
```
context.startPluginService(MyPluginService::class.java, "task1")
  ① PluginManager.proxyManager.acquireServiceProxy("MyPluginService:task1")
       → 从 availableServiceProxies 队列 poll 一个（demo = 10 个）
       → 放入 activeServiceProxies["MyPluginService:task1"] = HostService3::class
  ② Intent(this, HostService3::class.java).apply {
       putExtra("plugin_service_class_name", "MyPluginService")
       putExtra("plugin_service_instance_id", "MyPluginService:task1")
     }
  ③ context.startService(intent)        ← Intent 指向 HOST 的 service，不是 plugin!
  ④ Android 系统实例化 HostService3 (注册在 host manifest)
  ⑤ BaseHostService.onStartCommand(intent, ...)
       → initPluginService(intent) 解析 extras
       → PluginManager.getInterface(IPluginService::class.java, "MyPluginService")
            → PluginClassLoader.getInterface() → clazz.getDeclaredConstructor().newInstance()
       → pluginService.onAttach(this)        ← 注入 host service 引用（plugin 可调 Context）
       → pluginService.onCreate()            ← 标准 lifecycle
       → pluginService.onStartCommand(...)
```

**关键**：plugin service 不是真的 Android Service，**只是实现 `IPluginService` 接口的 POJO**。

```kotlin
// 插件端 MyPluginService.kt —— 不需要 <service> 声明
class MyPluginService : BasePluginService() {  // BasePluginService : IPluginService
    private var proxyService: Service? = null
    override fun onAttach(proxyService: Service) { this.proxyService = proxyService }
    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        // proxyService.startActivity(...) / getSystemService(...) / startForeground(...) 都可调
        return START_STICKY
    }
}
```

**禁止**：
- ❌ `class MyPluginService : Service()` + manifest `<service>` 声明——plugin 没系统 install，Android 永不实例化
- ❌ `context.startService(Intent(context, MyPluginService::class.java))`——系统找不到 service
- ❌ `extends Service` 而不是 `BasePluginService`——proxy 注入失败，无法调 `Context.startActivity` 等

### 10.4 跨进程 ContentProvider 访问标准模式

[ComboLite core Extensions.kt:210-251](file:///tmp/combolite-src/com/combo/core/utils/Extensions.kt#L210-L251)：

```kotlin
// 插件端声明（manifest）
<provider android:name=".MyProvider"
          android:authorities="com.example.myplugin.provider"
          android:exported="true" />

// 插件端实现 —— extends ContentProvider（不是 BasePluginProvider，普通 ContentProvider 即可）
class MyProvider : ContentProvider() {
    override fun onCreate() = true
    override fun query(uri: Uri, ...) = /* MatrixCursor */
    override fun insert(uri: Uri, values: ContentValues?) = null
}

// host 端访问
val cursor = contentResolver.queryPlugin(
    MyProvider.CONTENT_URI,           // 插件原始 URI
    arrayOf("col1", "col2"),
    null, null, null
)
```

**内部链路**：
```
contentResolver.queryPlugin(MyProvider.CONTENT_URI, ...)
  ① buildProxyUri(uri)
       → 查找 authorityToProviderMap[pluginAuthority] = "com.example.myplugin.MyProvider"
       → URLEncoder.encode(pluginAuthority) + path
       → 构造 proxyUri: content://<hostAuthority>/<encodedPluginAuthority>/<path>
  ② contentResolver.query(proxyUri, ...)
  ③ Android 系统路由到 host 的 HostProvider（authority = hostAuthority，注册在 host manifest）
  ④ BaseHostProvider.query(proxyUri, ...)
       → 解析 proxyUri 还原 pluginAuthority
       → PluginManager.proxyManager.findProviderInfoByAuthority(pluginAuthority)
       → PluginManager.proxyManager.getOrInstantiateProvider(className)
            → PluginManager.getInterface(ContentProvider::class.java, className)
            → clazz.getDeclaredConstructor().newInstance()
            → instance.attachInfo(context, null)  ← 关键：必须 attach 才能 query
       → provider.query(rewrittenUri, ...)         ← rewrittenUri 还原成 plugin 原生 URI
```

**关键点**：
- host **必须**注册 `<provider>` + 调 `setHostProviderAuthority(authority)`
- host provider authority = plugin provider authority 经过 URLEncoder 后的 path 段
- plugin provider 必须**用 `newInstance()` 创建**（不是 `getInstance()`），所以**不能用 Kotlin `object` 单例**——必须是 `class`
- `attachInfo(context, null)` 必须在 `newInstance()` 后立刻调，否则 `query()` 等方法会 NPE（context 为 null）

### 10.5 静态广播接收器标准模式

```kotlin
// 插件端声明（manifest）
<receiver android:name=".MyReceiver" android:exported="false">
    <intent-filter>
        <action android:name="com.example.MY_ACTION" />
    </intent-filter>
</receiver>

// 插件端实现 —— implements IPluginReceiver（不是 extends BroadcastReceiver）
class MyReceiver : IPluginReceiver {
    override fun onReceive(context: Context, intent: Intent) {
        // 处理广播
    }
}

// host 端触发
context.sendInternalBroadcast(Intent("com.example.MY_ACTION").apply {
    putExtra("key", "value")
})
```

**内部链路**：
```
context.sendInternalBroadcast(intent)  // intent.setPackage(hostPackageName)
  ① sendBroadcast(intent) → Android 系统投递到 host 的 HostReceiver
  ② BaseHostReceiver.onReceive(context, intent)
       → goAsync() + 协程
       → PluginManager.proxyManager.findReceiversForIntent(intent)
            → 遍历 staticReceiverRegistry
            → 匹配 action / category / scheme / exported 检查
       → 对每个匹配 plugin：PluginManager.getInterface(IPluginReceiver::class.java, className)
       → pluginReceiver.onReceive(context, intent)
```

**关键点**：
- host **必须**注册 `<receiver>`（exported=true + intent-filter）
- plugin 的 receiver 类是**实现 `IPluginReceiver` interface 的 POJO**，**不是** `extends BroadcastReceiver`
- host 端必须用 `sendInternalBroadcast(intent)`（自动 setPackage），不能用裸 `sendBroadcast`——否则 exported=false 的 plugin receiver 会被 ProxyManager 过滤掉

### 10.6 方案 A vs 方案 B 决策（OpenList plugin 适用，2026-06-03 用户拍板方案 A）

> **用户原话**："肯定选择方案A啊，谁说openlist扩展是无头的？"
> **方案 A = 标准 ComboLite 三模块架构（host 注册代理 + plugin 走 IPluginService / IPluginReceiver / ContentProvider）**

#### 决策背景

| 维度 | 方案 A（标准 ComboLite） | 方案 B（in-process 纯反射，Phase 24 已实现） |
|------|----------------------|--------------------------------------|
| 架构正确性 | ✅ 完全符合 ComboLite demo | ⚠️ openlist 特殊化，破坏通用模式 |
| 与未来 plugin 兼容 | ✅ 任何 IPluginService/IPluginReceiver 插件即插即用 | ❌ 每个插件都要重新写 classloader 反射 |
| 代码量 | 多（host 端多 13 个代理类 + plugin 端改 IPluginService 实现） | 少（删 OpenListService/StatusProvider，搬逻辑到 OpenListBridge） |
| Foreground Service | ✅ 可走 BaseHostService 代理（service pool 内有 host service 实例） | ❌ 必须用真 Service，但 plugin 没系统 install |
| 静态广播分发 | ✅ PluginManager.proxyManager.findReceiversForIntent 统一调度 | ❌ plugin 端收不到任何系统广播 |
| Provider 跨进程 | ✅ 走 host BaseHostProvider 代理 | ❌ IllegalArgumentException (authority 找不到) |
| 跨 ABI 稳定性 | ✅ BaseHostService 走 host 进程 | N/A |
| 调试复杂度 | 中（多 1 层代理） | 低（in-process 直接调） |

#### 方案 A 实施清单（OpenList plugin 改造路径）

1. **plugin 端**：
   - 删 [OpenListService.kt](file:///workspace/app/encv-mobile/plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListService.kt) 整文件
   - 删 [OpenListStatusProvider.kt](file:///workspace/app/encv-mobile/plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListStatusProvider.kt) 整文件
   - 新建 `OpenListPluginService : BasePluginService`（实现 `IPluginService`，把 startupSequence/shutdownSequence 搬过来）
   - 新建 `OpenListStatusProvider : ContentProvider`（保持 ContentProvider，因为 framework `attachInfo` 后能用）
   - manifest 删 `<service>` 声明，**保留** `<provider authority="com.encvgo.plugin.openlist.provider">`（框架解析需要）
   - [OpenListBridge.kt](file:///workspace/app/encv-mobile/plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListBridge.kt) 删除 `instance / isRunning / currentPort` 字段（搬到 OpenListPluginService）
   - [OpenListPluginEntry.kt](file:///workspace/app/encv-mobile/plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListPluginEntry.kt) `onLoad` 改用 `OpenListPluginService` proxy

2. **host 端**：
   - 新建 `HostService1..10 : BaseHostService`（10 个 service 池）
   - 新建 `HostStatusProvider : BaseHostProvider`
   - 新建 `HostStaticReceiver : BaseHostReceiver`
   - 新建 `HostPluginActivity : BaseHostActivity`（如需启动 plugin UI）
   - host manifest 注册上述 13 个组件
   - host `BaseHostApplication.onFrameworkSetup()` 调 `setServicePool(...)` + `setHostProviderAuthority("com.encvgo.app.provider")` + `setHostActivity(HostPluginActivity::class.java)`
   - [GoProcessPlugin.kt](file:///workspace/app/encv-mobile/android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt) `controlOpenList` 改用 `context.startPluginService(OpenListPluginService::class.java, instanceId="main")` 替代 classloader 反射
   - [OpenListStatusBridge.kt](file:///workspace/app/encv-mobile/android/combolite-host/src/main/java/com/encvgo/combolite/OpenListStatusBridge.kt) `read` 改用 `contentResolver.queryPlugin(OpenListStatusProvider.CONTENT_URI, ...)` 替代 classloader 反射

3. **验证**：
   - plugin 装上 → UI 显示 running/port
   - 按 start → host `startPluginService(OpenListPluginService)` 启动 → service 池分配 HostService3 → BaseHostService.onStartCommand → plugin service `onCreate` + `onStartCommand`
   - status 变更 → `OpenListPluginService.broadcastStatus` → host receiver → `PluginManager.proxyManager.findReceiversForIntent` 匹配 → 调 plugin IPluginReceiver
   - UI 查询 → host `contentResolver.queryPlugin(...)` → BaseHostProvider.query → plugin `OpenListStatusProvider.query`

### 10.7 PluginClassLoader.getInterface 的两个致命陷阱（⚠️ 实战踩坑！）

[PluginClassLoader.kt:100-119](file:///tmp/combolite-src/com/combo/core/runtime/loader/PluginClassLoader.kt#L100-L119)：

```kotlin
fun <T> getInterface(interfaceClass: Class<T>, className: String): T? = try {
    val clazz = loadClass(className)
    val instance = clazz.getDeclaredConstructor().newInstance()  // ← 陷阱 1
    if (interfaceClass.isInstance(instance)) instance as T else null
} catch (e: Exception) { null }
```

**陷阱 1：`getDeclaredConstructor().newInstance()` 对 Kotlin `object` 单例不安全**

Kotlin `object` 单例的 INSTANCE 字段是懒初始化的（首次访问才创建）。`getDeclaredConstructor().newInstance()` 会绕过 INSTANCE 直接 `new`，可能：
- 触发 `InstantiationException`（object 构造器是 private）
- 创建出"半初始化"实例（companion object 字段未就绪）
- 破坏单例语义（出现多个实例）

**修复**：
```kotlin
// ❌ 错误：用 PluginManager.getInterface(IPluginService::class.java, MyService::class.java.name)
//            → 内部 newInstance() → 崩
// ✅ 正确：plugin 端用 class extends BasePluginService（普通 class），不用 object
class MyService : BasePluginService() { ... }
```

**陷阱 2：`getInterface` 只查"插件自己 + 依赖链"——不查 host**

[PluginManager.kt:225-249](file:///tmp/combolite-src/com/combo/core/runtime/PluginManager.kt#L225-L249)：

```kotlin
fun <T : Any> getInterface(interfaceClass: Class<T>, className: String): T? {
    val targetPluginId = requireContext().classIndex[className]
    if (targetPluginId == null) {
        getInterfaceFromHost(interfaceClass, className)?.let { return it }  // ← host 兜底
        return null
    }
    val loadedPlugin = requireContext().loadedPlugins.value[targetPluginId]
    return loadedPlugin.classLoader.getInterface(interfaceClass, className)
}
```

如果 class 不在**任何插件的类索引**中，框架会调 `getInterfaceFromHost` 兜底（host 类加载器）。但这只对**确实在 host classpath 里**的类有效——如想反射调 `OpenListBridge`（在插件 classpath），必须先确保插件已 `loadEnabledPlugins()` 加载到 classloader 池。

### 10.8 plugin.openlist 当前形态对方案 A 的影响（决策参考）

| 文件 | 现状 | 方案 A 改造 |
|------|------|-----------|
| [AndroidManifest.xml](file:///workspace/app/encv-mobile/plugin-openlist/src/main/AndroidManifest.xml) | 声明 `<service>` + `<provider>` | 删 `<service>`，保留 `<provider>` |
| [OpenListService.kt](file:///workspace/app/encv-mobile/plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListService.kt) | `class OpenListService : Service()` | 改为 `class OpenListPluginService : BasePluginService()` |
| [OpenListStatusProvider.kt](file:///workspace/app/encv-mobile/plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListStatusProvider.kt) | `class OpenListStatusProvider : ContentProvider()` | 保持不变（ContentProvider 即可，框架 newInstance + attachInfo 正常） |
| [OpenListBridge.kt](file:///workspace/app/encv-mobile/plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListBridge.kt) | `object OpenListBridge : Event, LogCallback` | 删 `instance / isRunning / currentPort`，搬到 OpenListPluginService |
| [OpenListPluginEntry.kt](file:///workspace/app/encv-mobile/plugin-openlist/src/main/java/com/encvgo/plugin/openlist/OpenListPluginEntry.kt) | 普通 IPluginEntryClass | onLoad/onUnload 改用 `OpenListPluginService` proxy |
| [OpenListStatusBridge.kt](file:///workspace/app/encv-mobile/android/combolite-host/src/main/java/com/encvgo/combolite/OpenListStatusBridge.kt) | classloader 反射 `OpenListBridge.snapshot()` | 改用 `contentResolver.queryPlugin(OpenListStatusProvider.STATUS_URI, ...)` |
| [GoProcessPlugin.kt](file:///workspace/app/encv-mobile/android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt) | classloader 反射 `OpenListBridge.start()` + 反射注册 statusListener | 改用 `context.startPluginService(OpenListPluginService::class.java, "main")`；statusListener 反射注册保留（host → plugin 方向） |

### 10.9 Service pool size 选择（为什么 demo 用 10）

| 场景 | 池大小建议 | 原因 |
|------|----------|------|
| 普通 app（1-2 个 plugin service） | 4 | 占内存最小 |
| 通用 demo / 框架示例 | 10 | ComboLite demo 默认值，覆盖多 service 并发场景 |
| 复杂 app（多插件并发） | 16-20 | 预留 buffer |

**池耗尽语义**：[ProxyManager.kt:152-155](file:///tmp/combolite-src/com/combo/core/proxy/ProxyManager.kt#L152-L155) 返回 null，调用方 `startPluginService` 会 silently log error 不启动 service。用户感知不到任何反馈。

**修复**：
- 监控 `proxyManager.activeServiceProxies.size`，接近池大小时警告
- 不要为单个插件启 N 个 service（用 `instanceId` 区分，1 个 service class 可启多个实例）

### 10.10 关键参考源码

| 路径 | 内容 |
|------|------|
| [comboLite-core/.../api/IPluginService.kt](file:///tmp/combolite-src/com/combo/core/api/IPluginService.kt) | plugin service interface（onAttach + 完整 lifecycle） |
| [comboLite-core/.../component/service/BaseHostService.kt](file:///tmp/combolite-src/com/combo/core/component/service/BaseHostService.kt) | host service 代理基类（initPluginService + lifecycle 转发） |
| [comboLite-core/.../component/service/BasePluginService.kt](file:///tmp/combolite-src/com/combo/core/component/service/BasePluginService.kt) | plugin service 基类（proxyService 持有 + 默认空 lifecycle） |
| [comboLite-core/.../api/IPluginReceiver.kt](file:///tmp/combolite-src/com/combo/core/api/IPluginReceiver.kt) | plugin receiver interface |
| [comboLite-core/.../component/receiver/BaseHostReceiver.kt](file:///tmp/combolite-src/com/combo/core/component/receiver/BaseHostReceiver.kt) | host receiver 代理基类（goAsync + 协程 + 分发） |
| [comboLite-core/.../component/provider/BaseHostProvider.kt](file:///tmp/combolite-src/com/combo/core/component/provider/BaseHostProvider.kt) | host provider 代理基类（withForwardedRequest 通用转发） |
| [comboLite-core/.../utils/Extensions.kt](file:///tmp/combolite-src/com/combo/core/utils/Extensions.kt) | startPluginService / startPluginActivity / queryPlugin / sendInternalBroadcast 等扩展 |
| [comboLite-core/.../proxy/ProxyManager.kt](file:///tmp/combolite-src/com/combo/core/proxy/ProxyManager.kt) | 四大组件代理调度（acquireServiceProxy / setHostXxx / findReceiversForIntent） |
| [comboLite-core/.../runtime/loader/PluginClassLoader.kt](file:///tmp/combolite-src/com/combo/core/runtime/loader/PluginClassLoader.kt) | DexClassLoader + getInterface(newInstance 陷阱) |
| [lnzz123/ComboLite/app/src/main/java/com/combo/plugin/sample/HostApp.kt](https://raw.githubusercontent.com/lnzz123/ComboLite/master/app/src/main/java/com/combo/plugin/sample/HostApp.kt) | demo 标准 onFrameworkSetup 模板 |
| [lnzz123/ComboLite/app/src/main/AndroidManifest.xml](https://raw.githubusercontent.com/lnzz123/ComboLite/master/app/src/main/AndroidManifest.xml) | demo host manifest（10 service + 1 provider + 1 receiver + 1 activity） |
