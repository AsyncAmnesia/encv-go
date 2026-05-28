# ComboLite 集成规范（来自官方 README + 源码审计）

> **核心原则：0 Hook、0 反射。** ComboLite 是完全基于 Android 官方公开 API 构建的框架。
> **任何对 ComboLite API 使用反射的代码都是错误的，说明没有理解框架设计。**

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

→ **立即删除**，这不是 ComboLite 的用法。

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

---

## 三、宿主端集成 Checklist

### 3.1 Application 配置

- [ ] 继承 `BaseHostApplication`（非普通 `Application`）
- [ ] `onFrameworkSetup()` 中设置 `ValidationStrategy`（Insecure/UserGrant/Strict）
- [ ] `onFrameworkSetup()` 中配置 `PluginManager.proxyManager.setHostActivity(HostActivity::class.java)`
- [ ] 如果插件需要 Service：配置 `setServicePool()`
- [ ] 如果插件需要 ContentProvider：配置 `setHostProviderAuthority()`

### 3.2 AndroidManifest.xml

- [ ] 声明 HostActivity 子类（`exported="false"`）
- [ ] 声明 HostService 子类（如需）
- [ ] 声明 FileProvider（APK 安装用）

### 3.3 build.gradle.kts（宿主 :app）

```kotlin
plugins {
    alias(libs.plugins.combolite.aar2apk)    // aar2apk 打包插件
}

packagePlugins {                                // 开发时自动集成
    enabled.set(true)
    buildType.set(PackageBuildType.DEBUG)
    pluginsDir.set("debug_plugins")
}

dependencies {
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

### 错误 C：「忽略 suspend 函数**

> **症状**：直接在主线程调用 `installPlugin()` 导致 ANR
> **根因**：`installPlugin` 是 `suspend fun`（内部有文件 IO 和签名校验）
> **修复**：必须用协程 `GlobalScope.launch(Dispatchers.IO) { ... }` 调用

### 错误 D：「忘记处理返回值」

> **症状**：调用 `installPlugin()` 后不检查 `InstallResult`，默认成功
> **根因**：把 `suspend fun` 当 `void` 用
> **修复**：`when (result) { is Success -> ...; is Failure -> ... }`

---

## 五、源码参考（克隆至 `/tmp/ComboLite`）

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

---

## 六、调试技巧

1. **`PluginManager.isInitialized`** — 第一步检查这个，未初始化时所有操作无效
2. **Logcat 过滤**：`TAG="ENCV-go"` 或 `TAG="PluginLifecycleManager"`
3. **安装失败时**：检查 `InstallResult.Failure.reason` 和 `.exception?.stackTraceToString()`
4. **Activity 无法启动时**：检查 ProxyManager 是否配置了 setHostActivity + Manifest 是否声明了 HostActivity
5. **资源找不到时**：检查 ResourceManager 是否被 lifecycleManager 自动触发（loadPlugin L230）
