package com.encvgo.combolite

import android.content.Context
import android.content.Intent
import com.encvgo.combolite.engine.PluginLifecycleEngine
import com.encvgo.combolite.model.OperationResult
import com.encvgo.combolite.model.PluginFullState
import com.encvgo.combolite.model.PluginState

object EncvComboLiteHost {

    val isInitialized: Boolean get() = PluginLifecycleEngine.isInitialized()

    fun getInstalledPlugins(): List<PluginState> = PluginLifecycleEngine.getInstalledPlugins()

    fun getPluginInfo(pluginId: String): PluginState? = PluginLifecycleEngine.getPluginInfo(pluginId)

    fun getPluginFullState(pluginId: String): PluginFullState {
        if (!PluginLifecycleEngine.isInitialized()) {
            return PluginFullState(id = pluginId, status = "framework_not_ready")
        }
        val state = getPluginInfo(pluginId)
        if (state == null) {
            return PluginFullState(id = pluginId, status = "not_installed")
        }
        if (!state.enabled) {
            return PluginFullState(id = pluginId, status = "disabled", name = state.name)
        }
        val loaded = PluginLifecycleEngine.isPluginLoaded(pluginId)
        return PluginFullState(
            id = pluginId,
            status = if (loaded) "ready" else "not_loaded",
            name = state.name,
            version = state.versionName
        )
    }

    fun isPluginAvailable(pluginId: String): Boolean {
        if (!PluginLifecycleEngine.isInitialized()) return false
        val state = getPluginInfo(pluginId)
        return state != null && state.installed && state.enabled && PluginLifecycleEngine.isPluginLoaded(pluginId)
    }

    fun ensurePluginLoaded(pluginId: String): Boolean = PluginLifecycleEngine.ensurePluginLoaded(pluginId)

    suspend fun installPlugin(apkFile: java.io.File): OperationResult<PluginState> =
        PluginLifecycleEngine.installPlugin(apkFile)

    suspend fun uninstallPlugin(pluginId: String): OperationResult<Unit> =
        PluginLifecycleEngine.uninstallPlugin(pluginId)

    suspend fun setPluginEnabled(pluginId: String, enabled: Boolean): OperationResult<Unit> =
        PluginLifecycleEngine.setPluginEnabled(pluginId, enabled)

    suspend fun launchPlugin(pluginId: String): Boolean =
        PluginLifecycleEngine.launchPlugin(pluginId)

    fun createProxyIntent(
        context: Context,
        pluginId: String,
        targetActivity: String,
        hostActivityClass: Class<*>,
        extras: Map<String, Any> = emptyMap()
    ): Intent = PluginLifecycleEngine.createProxyIntent(context, pluginId, targetActivity, hostActivityClass, extras)

    fun setupFramework(hostActivityClass: Class<*>) = PluginLifecycleEngine.setupFramework(hostActivityClass)
}
