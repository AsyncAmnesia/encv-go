package com.encvgo.combolite

import android.content.Context
import android.content.Intent
import com.encvgo.combolite.engine.PluginLifecycleEngine
import com.encvgo.combolite.model.OperationResult
import com.encvgo.combolite.model.PluginState

object EncvComboLiteHost {

    val isInitialized: Boolean get() = PluginLifecycleEngine.isInitialized()

    fun getInstalledPlugins(): List<PluginState> = PluginLifecycleEngine.getInstalledPlugins()

    fun getPluginInfo(pluginId: String): PluginState? = PluginLifecycleEngine.getPluginInfo(pluginId)

    fun isPluginAvailable(pluginId: String): Boolean =
        getInstalledPlugins().any { it.id == pluginId && it.enabled }

    suspend fun installPlugin(apkFile: java.io.File): OperationResult<PluginState> =
        PluginLifecycleEngine.installPlugin(apkFile)

    suspend fun uninstallPlugin(pluginId: String): OperationResult<Unit> =
        PluginLifecycleEngine.uninstallPlugin(pluginId)

    suspend fun setPluginEnabled(pluginId: String, enabled: Boolean): OperationResult<Unit> =
        PluginLifecycleEngine.setPluginEnabled(pluginId, enabled)

    suspend fun launchPlugin(pluginId: String): Boolean =
        PluginLifecycleEngine.launchPlugin(pluginId)

    fun ensurePluginLoaded(pluginId: String) = PluginLifecycleEngine.ensurePluginLoaded(pluginId)

    fun createProxyIntent(
        context: Context,
        pluginId: String,
        targetActivity: String,
        hostActivityClass: Class<*>,
        extras: Map<String, Any> = emptyMap()
    ): Intent = PluginLifecycleEngine.createProxyIntent(context, pluginId, targetActivity, hostActivityClass, extras)

    fun setupFramework(hostActivityClass: Class<*>) = PluginLifecycleEngine.setupFramework(hostActivityClass)
}
