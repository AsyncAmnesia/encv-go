package com.encvgo.plugin.openlist

import android.util.Log
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
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

    /**
     * IPluginEntryClass requires a Composable Content() entry. The OpenList plugin
     * intentionally has NO UI of its own — all user-facing status is rendered by the
     * host app's `LocalOpenListStatusCard.vue` (Capacitor/Ionic Vue layer), which
     * polls the cross-process ContentProvider exposed by OpenListStatusProvider.
     *
     * We still need to provide an implementation so the class is not abstract.
     * Returning a minimal centered placeholder keeps the composable tree happy if
     * a host app ever chooses to render the plugin entry directly.
     */
    @Composable
    override fun Content() {
        Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            Text("OpenList plugin — no embedded UI; use host LocalOpenListStatusCard.")
        }
    }
}
