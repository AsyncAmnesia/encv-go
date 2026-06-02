package com.encvgo.plugin.openlist

import android.content.Intent
import android.os.Build
import androidx.compose.runtime.Composable
import androidx.core.content.ContextCompat
import com.combo.core.api.IPluginEntryClass
import com.combo.core.model.PluginContext

class OpenListPluginEntry : IPluginEntryClass {
    override val pluginModule = emptyList<org.koin.core.module.Module>()

    override fun onLoad(context: PluginContext) {
        val app = context.application
        val intent = Intent(app, OpenListService::class.java)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            ContextCompat.startForegroundService(app, intent)
        } else {
            app.startService(intent)
        }
    }

    override fun onUnload() {
    }

    @Composable
    override fun Content() {
    }
}
