package com.encvgo.plugin.openlist

import android.util.Log
import com.combo.core.api.IPluginEntryClass
import com.combo.core.model.PluginContext
import org.koin.dsl.module

class OpenListPluginEntry : IPluginEntryClass {
    companion object {
        private const val TAG = "OpenList-PluginEntry"
    }

    override val pluginModule = listOf(module {
        single { OpenListBridge }
    })

    override fun onLoad(context: PluginContext) {
        Log.e(TAG, "[SAT-DBG][OpenList] onLoad() begin | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        val app = context.application
        try {
            Log.e(TAG, "[SAT-DBG][OpenList] onLoad() OpenListBridge.init() ...")
            OpenListBridge.init(app)
            Log.e(TAG, "[SAT-DBG][OpenList] onLoad() OpenListBridge.init() OK")
        } catch (e: Throwable) {
            Log.e(TAG, "[SAT-DBG][OpenList] onLoad() init FAILED", e)
        }
        Log.e(TAG, "[SAT-DBG][OpenList] onLoad() done | OpenListStatusProvider auto-registered via AndroidManifest")
    }

    override fun onUnload() {
        Log.e(TAG, "[SAT-DBG][OpenList] onUnload() | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
    }
}
