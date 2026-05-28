package com.encvgo.plugin.mpv

import androidx.compose.runtime.Composable
import com.combo.core.api.IPluginEntryClass
import com.combo.core.model.PluginContext

class MpvPluginEntry : IPluginEntryClass {
    override val pluginModule = emptyList<org.koin.core.module.Module>()

    override fun onLoad(context: PluginContext) {
    }

    override fun onUnload() {
    }

    @Composable
    override fun Content() {
        MpvPlayerScreen(
            filePath = "",
            fileName = "",
            mimeType = "",
            isExternal = false,
            engine = null,
            onBack = {}
        )
    }
}
