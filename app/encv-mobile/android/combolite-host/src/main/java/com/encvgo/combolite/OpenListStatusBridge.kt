package com.encvgo.combolite

import android.content.Context
import android.util.Log

/**
 * Host-side bridge to the OpenList extension.
 *
 * Phase 23 重写（in-process 替代 ContentProvider）：
 * 历史：原本用 ContentResolver.query("content://com.encvgo.plugin.openlist.provider/status")
 *       来读 OpenListStatusProvider 的 snapshot。
 *       失败原因：ComboLite 是文件系统 + PluginClassLoader 架构（[ComboLite source]:
 *       [com/combo/core/runtime/loader/PluginClassLoader.kt]），plugin APK 没系统级
 *       install → 系统 ContentProvider authority 找不到 → IllegalArgumentException。
 *       即使 host 加 BaseHostProvider + proxyManager.setHostProviderAuthority() 转发，
 *       也需要 plugin manifest 的 <provider> 在 proxy 里有 entry——但这层 setup 也没做。
 *
 * 新方案：直接通过 PluginClassLoader 反射调 [com.encvgo.plugin.openlist.OpenListBridge]
 *       的 static `snapshot()` 方法（plugin 在 host 进程里跑，classloader 可达）。
 *       Host ↔ plugin 通过 [com.encvgo.plugin.openlist.OpenListBridge.statusListener]
 *       in-process lambda 推送状态变更（替代 Phase 22 的跨进程 broadcast）。
 *
 * 参考 ComboLite:
 *   - [com/combo/core/runtime/PluginManager.kt:225] PluginManager.getInterface()
 *   - [com/combo/core/runtime/loader/PluginClassLoader.kt:100] PluginClassLoader.getInterface()
 *   - 关键限制：getInterface() 调 `getDeclaredConstructor().newInstance()` 创建新实例——
 *     对 Kotlin `object`（单例）不安全。我们直接拿 `classLoader.loadClass()` +
 *     `getDeclaredField("INSTANCE").get(null)`，避开 newInstance。
 */
object OpenListStatusBridge {

    /** 与 plugin-openlist/build.gradle.kts 的 applicationId 对齐 */
    private const val PLUGIN_ID = "com.encvgo.plugin.openlist"

    /** plugin 内部 OpenListBridge 的 FQ class name。classloader 反射用。 */
    private const val BRIDGE_CLASS_NAME = "com.encvgo.plugin.openlist.OpenListBridge"

    private const val TAG = "OpenList-HostBridge"

    data class OpenListRuntime(
        val isInstalled: Boolean,
        val running: Boolean,
        val port: Int,
        val pid: Int,
        val dataSizeBytes: Long,
        val lastError: String,
        val lastUpdateTs: Long,
    ) {
        companion object {
            val NotInstalled = OpenListRuntime(
                isInstalled = false,
                running = false,
                port = 0,
                pid = 0,
                dataSizeBytes = 0L,
                lastError = "openlist extension not installed",
                lastUpdateTs = 0L,
            )
        }
    }

    /**
     * 一次性读快照（替代前端 3s 轮询）。
     * 失败语义：未安装 → NotInstalled；已安装未加载 → NotLoaded；
     * 已加载但 bridge 没初始化 → InstalledNotInitialized。
     */
    fun read(context: Context): OpenListRuntime {
        Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] read() begin | ts=${System.currentTimeMillis()}")

        val installed = EncvComboLiteHost.getInstalledPlugins().any { it.id == PLUGIN_ID }
        if (!installed) {
            Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] read() → NotInstalled (not in PluginManager.installed list)")
            return OpenListRuntime.NotInstalled
        }
        Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] read() plugin IS installed (id=$PLUGIN_ID)")

        val loaded = EncvComboLiteHost.getLoadedPluginInfo(PLUGIN_ID)
        if (loaded == null) {
            // 已安装但没加载——尝试 load（可能在 onBoot 时未触发）
            val loadedNow = try {
                EncvComboLiteHost.ensurePluginLoaded(PLUGIN_ID)
            } catch (e: Throwable) {
                Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] ensurePluginLoaded FAILED", e)
                false
            }
            if (!loadedNow) {
                Log.w(TAG, "[SAT-DBG][OpenList][HostBridge] read() → InstalledNotLoaded")
                return OpenListRuntime(
                    isInstalled = true,
                    running = false,
                    port = 0, pid = 0, dataSizeBytes = 0L,
                    lastError = "installed but not loaded yet",
                    lastUpdateTs = 0L,
                )
            }
        }

        val loadedNow = loaded ?: EncvComboLiteHost.getLoadedPluginInfo(PLUGIN_ID)
        if (loadedNow == null) {
            Log.w(TAG, "[SAT-DBG][OpenList][HostBridge] read() → InstalledButCannotLoad")
            return OpenListRuntime(
                isInstalled = true, running = false, port = 0, pid = 0, dataSizeBytes = 0L,
                lastError = "installed but classloader unavailable", lastUpdateTs = 0L,
            )
        }

        // 通过 plugin classloader 反射调 OpenListBridge.snapshot()（static method）
        return try {
            val bridgeClass = loadedNow.classLoader.loadClass(BRIDGE_CLASS_NAME)
            val snapshot = bridgeClass.getMethod("snapshot").invoke(null) as? Map<*, *>
            if (snapshot == null) {
                Log.w(TAG, "[SAT-DBG][OpenList][HostBridge] snapshot() returned non-map")
                return OpenListRuntime(
                    isInstalled = true, running = false, port = 0, pid = 0, dataSizeBytes = 0L,
                    lastError = "snapshot returned null/non-map", lastUpdateTs = 0L,
                )
            }
            val runtime = OpenListRuntime(
                isInstalled = true,
                running = (snapshot["running"] as? Boolean) ?: false,
                port = (snapshot["port"] as? Int) ?: 0,
                pid = (snapshot["pid"] as? Int) ?: 0,
                dataSizeBytes = (snapshot["data_size_bytes"] as? Long) ?: 0L,
                lastError = (snapshot["last_error"] as? String) ?: "",
                lastUpdateTs = (snapshot["last_update_ts"] as? Long) ?: 0L,
            )
            Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] read() OK | running=${runtime.running} port=${runtime.port} dataSize=${runtime.dataSizeBytes} lastErr='${runtime.lastError}'")
            runtime
        } catch (e: Throwable) {
            Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] read() reflection FAILED", e)
            OpenListRuntime(
                isInstalled = true, running = false, port = 0, pid = 0, dataSizeBytes = 0L,
                lastError = "snapshot read failed: ${e.message}", lastUpdateTs = 0L,
            )
        }
    }

    /**
     * 控制 plugin（启动/停止/强制 DB sync/设置 admin 密码）。
     * 直接调 OpenListBridge 静态方法（gomobile 绑定）。
     * 启动后状态变更会自动通过 [com.encvgo.plugin.openlist.OpenListBridge.statusListener]
     * 推送到 host（已由 GoProcessPlugin 在 load() 时反射注册）。
     */
    fun control(context: Context, action: String, args: Map<String, Any> = emptyMap()): Boolean {
        Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] control() action=$action args=$args")
        val loaded = EncvComboLiteHost.getLoadedPluginInfo(PLUGIN_ID)
        if (loaded == null) {
            val ensure = try { EncvComboLiteHost.ensurePluginLoaded(PLUGIN_ID) } catch (e: Throwable) { false }
            if (!ensure) {
                Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] control() FAILED: plugin not loaded")
                return false
            }
        }
        val cl = (EncvComboLiteHost.getLoadedPluginInfo(PLUGIN_ID))?.classLoader
        if (cl == null) {
            Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] control() FAILED: no classloader")
            return false
        }
        return try {
            val bridgeClass = cl.loadClass(BRIDGE_CLASS_NAME)
            val result = when (action) {
                "start" -> {
                    val port = (args["port"] as? Int) ?: 0
                    bridgeClass.getMethod("start").invoke(null) as? Boolean ?: false
                }
                "stop" -> {
                    val timeout = (args["timeout_ms"] as? Long) ?: 5_000L
                    bridgeClass.getMethod("shutdown", java.lang.Long.TYPE).invoke(null, timeout)
                    true
                }
                "force_db_sync" -> {
                    bridgeClass.getMethod("forceDBSync").invoke(null)
                    true
                }
                "set_admin_password" -> {
                    val pwd = args["password"] as? String ?: ""
                    bridgeClass.getMethod("setAdminPassword", java.lang.String::class.java).invoke(null, pwd)
                    true
                }
                else -> {
                    Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] control() unknown action=$action")
                    false
                }
            }
            Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] control() action=$action → result=$result")
            true
        } catch (e: Throwable) {
            Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] control() FAILED", e)
            false
        }
    }
}
