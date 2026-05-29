# Kotlin 2.3.21 + Android 编译铁律（来自 CI 实战踩坑）

> **核心原则：Kotlin 编译器不会骗你，报错就是真的。不要猜 API，不要假设类型。**

---

## 一、变量声明（反复出错点）

### 1.1 `val` vs `var` — 编译器报 `'val' cannot be reassigned` 就是字面意思

**❌ 错误**：
```kotlin
val flags = PackageManager.GET_META_DATA or PackageManager.GET_ACTIVITIES
if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
    flags = flags or PackageManager.GET_SIGNING_CERTIFICATES  // ← 编译错误！
}
```

**✅ 正确**：
```kotlin
var flags = PackageManager.GET_META_DATA or PackageManager.GET_ACTIVITIES  // ← var！
if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
    flags = flags or PackageManager.GET_SIGNING_CERTIFICATES
}
```

**规则**：只要后续有重新赋值（`=`），必须用 `var`。没有例外。

### 1.2 类型推断与泛型

**当 `Cannot infer type for type parameter 'T'` 时**：
- 检查是否在 lambda/泛型上下文中，编译器无法推断返回类型
- **显式指定类型参数**或**给变量加类型注解**
```kotlin
// 错误：runBlocking 内部推断失败
val count = runBlocking { someGenericMethod() }

// 正确：显式指定
val count: Int = runBlocking { someGenericMethod() }
// 或
val count = runBlocking<Int> { someGenericMethod() }
```

---

## 二、Suspend 函数调用（反复出错点）

### 2.1 Suspend 函数只能在协程内调用

**错误信息模板**：`Suspend function 'suspend fun xxx()' can only be called from a coroutine or another suspend function.`

**根因**：ComboLite 的 `setValidationStrategy()`、`setGlobalClashCallback()`、`loadEnabledPlugins()` 等都是 `suspend fun`。

**✅ 在非 suspend 函数中调用 suspend 函数的唯一正确方式**：
```kotlin
import kotlinx.coroutines.runBlocking

fun setupFramework(hostActivityClass: Class<*>) {
    try {
        runBlocking { PluginManager.setValidationStrategy(ValidationStrategy.Insecure) }
    } catch (e: Error) {
        // 防御 kotlin-reflect 失败
    } catch (e: Exception) {
        // 防御其他异常
    }

    try {
        runBlocking { PluginCrashHandler.setGlobalClashCallback(null) }
    } catch (e: Error) {}
    catch (e: Exception) {}
}
```

**⚠️ 注意**：`runBlocking` 会阻塞当前线程。只在初始化等一次性场景使用，**禁止在 UI 线程/主线程调用**。

### 2.2 GlobalScope.launch vs runBlocking

| 场景 | 用法 |
|------|------|
| 从非协程代码启动异步任务（不阻塞） | `GlobalScope.launch(Dispatchers.IO) { ... }` |
| 从同步函数中调用 suspend 函数（阻塞等待结果） | `runBlocking { ... }` 或 `runBlocking(Dispatchers.IO) { ... }` |
| 已在协程内部 | 直接调用，无需包装 |

---

## 三、可见性修饰符（反复出错点）

### 3.1 `internal` 的含义

`internal object` / `internal class` / `internal fun` — **仅在同一个模块（Gradle module）内可见**。

**本项目中的关键实例**：

| 声明位置 | 可见性 | 谁能调用 |
|---------|--------|---------|
| `PluginLifecycleEngine` (`engine/` 包, `internal object`) | 仅 `:combolite-host` 模块内 | ✅ EncvComboLiteHost.kt（同模块）<br>❌ DiagnosticKit.kt（不同包但同模块）— 实际上同模块的不同包可以访问 internal<br>⚠️ 但如果 DiagnosticKit 在独立模块则不行 |
| `EncvComboLiteHost` (`public object`) | 所有依赖 `:combolite-host` 的模块 | ✅ GoProcessPlugin、DiagnosticKit |

**规则**：
- 跨包调用时，优先用 **public 门面** (`EncvComboLiteHost`)
- 同模块跨包调用 `internal` 对象 → **可以**（`internal` 是模块级不是包级）
- 但如果将来拆分模块会出问题 → **始终通过 public 门面调用**

### 3.2 本项目的可见性层次

```
:combolite-host 模块
├── public   EncvComboLiteHost          ← 外部唯一入口
├── internal PluginLifecycleEngine     ← 仅本模块内部
└── public   model/*                   ← 数据类需要外部可读

:app 模块
├── public   GoProcessPlugin           ← Capacitor 入口
├── public   AppLogger / LogExporter   ← 工具类
└── public   PlayerEntry / EncvApplication
```

---

## 四、类型系统（反复出错点）

### 4.1 Class<*> 类型转换

**错误信息**：`Argument type mismatch: actual type is 'Class<CapturedType(*)>', but 'Class<out BaseHostActivity>' was expected.`

**根因**：Java 泛型擦除后 `Class<?>` 与 `Class<SpecificType>` 不兼容。

**✅ 正确做法**：
```kotlin
fun setupFramework(hostActivityClass: Class<*>) {
    // ❌ PluginManager.proxyManager.setHostActivity(hostActivityClass)
    // ✅ 显式 cast
    PluginManager.proxyManager.setHostActivity(
        hostActivityClass as Class<com.combo.core.component.activity.BaseHostActivity>
    )
}
```

### 4.2 nullable vs non-null

**AAR 反编译确认的规则**（combolite-core 2.0.2）：

| 属性 | AAR 返回类型 | Kotlin 中使用 |
|------|------------|---------------|
| `PluginInfo.enabled` | `boolean` (primitive) | 非 null，直接用 |
| `PluginInfo.id` | `String` | 非 null |
| `getPluginInfo(String)` | `LoadedPluginInfo?` | 可能为 null |
| `uninstallPlugin(String)` | `Boolean` (boxed) | 可能是 null → 用 `== true` 判断 |
| `installPlugin(File, Boolean)` | `InstallResult` (sealed class) | 非 null |

---

## 五、import 规范

### 5.1 必须显式 import 的常见遗漏

| import | 使用场景 |
|-------|---------|
| `kotlinx.coroutines.runBlocking` | 在非 suspend 函数中调用 suspend 函数 |
| `kotlinx.coroutines.GlobalScope` | 启动后台协程 |
| `kotlinx.coroutines.Dispatchers.IO` | IO 线程池 |
| `kotlinx.coroutines.launch` | 协程构建器 |
| `kotlinx.coroutines.withContext` | 切换调度器并返回结果 |
| `android.content.Intent` | Intent 构造 |
| `android.net.Uri` | URI 处理 |
| `java.io.File` | 文件操作 |
| `java.util.concurrent.ConcurrentLinkedQueue` | 线程安全队列 |
| `java.util.concurrent.ConcurrentHashMap` | 线程安全 Map |

### 5.2 禁止使用的 import

| import | 原因 |
|-------|------|
| `com.combo.core.runtime.PluginManager` (在 :app 中) | 应通过 EncvComboLiteHost 访问 |
| `com.combo.core.runtime.installer.InstallerManager` (在 :app 中) | 同上 |
| `kotlin.reflect.jvm.javaMethod` (在 :app 中) | 已迁移到 DiagnosticKit |

---

## 六、CI 编译前自检清单

写完任何 .kt 文件后，**提交前**必须逐条检查：

- [ ] 所有 `val` 变量确实没有被重新赋值（否则改为 `var`）
- [ ] 所有 `suspend fun` 调用都在协程/runBlocking 内部
- [ ] 没有 `internal` 对象被跨模块引用（改用 public 门面）
- [ ] `Class<*>` 参数处做了必要的 `as Class<ConcreteType>` 转换
- [ ] 泛型方法调用处没有 `Cannot infer type` 错误（加显式类型参数）
- [ ] 没有 `Unresolved reference` 错误（检查 import 和可见性）
- [ ] ComboLite API 调用全部经过 AAR `javap -p` 验证

---

## 七、已验证通过的"金标准"代码模式

以下模式已在 CI 中编译通过，作为新代码的权威参照：

### suspend 函数在非协程中的调用
```kotlin
// EncvApplication.onFrameworkSetup (suspend 返回值)
override fun onFrameworkSetup(): suspend () -> Unit = {
    EncvComboLiteHost.setupFramework(EncvHostActivity::class.java)
}

// Engine.setupFramework (普通函数内调用 suspend)
fun setupFramework(hostActivityClass: Class<*>) {
    try { runBlocking { PluginManager.setValidationStrategy(...) } }
    catch (e: Error) {} catch (e: Exception) {}
}
```

### Capacitor @PluginMethod 中的协程模式
```kotlin
@PluginMethod
fun togglePluginEnabled(call: PluginCall) {
    val pluginId = call.getString("pluginId") ?: run { call.reject("..."); return }
    GlobalScope.launch(Dispatchers.IO) {
        val result = EncvComboLiteHost.setPluginEnabled(pluginId, enabled)
        when (result) {
            is OperationResult.Success -> withContext(Dispatchers.Main) { call.resolve(...) }
            is OperationResult.Failure -> withContext(Dispatchers.Main) { call.reject(...) }
        }
    }
}
```
