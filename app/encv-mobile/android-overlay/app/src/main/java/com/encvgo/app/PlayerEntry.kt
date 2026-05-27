package com.encvgo.app

import android.content.Context
import android.content.Intent
import android.os.Bundle
import android.util.Log
import io.github.combolite.core.PluginManager
import com.encvgo.plugin.mpv.MpvPlayerActivity

object PlayerEntry {
    private const val TAG = "PlayerEntry"
    private const val PLUGIN_ID = "mpv-player"
    private const val EXTRA_FILE_PATH = "file_path"
    private const val EXTRA_FILE_NAME = "file_name"
    private const val EXTRA_MIME_TYPE = "mime_type"
    private const val EXTRA_IS_EXTERNAL = "is_external"

    fun play(
        context: Context,
        filePath: String,
        fileName: String,
        mimeType: String = "",
        isExternal: Boolean = false
    ) {
        val pluginManager = PluginManager.getInstance(context)
        val mpvPlugin = pluginManager.getInstalledPlugin(PLUGIN_ID)

        if (mpvPlugin != null && mpvPlugin.enabled) {
            Log.i(TAG, "MPV plugin available, routing to MpvPlayerActivity")
            startMpvPlayer(context, filePath, fileName, mimeType, isExternal, pluginManager, mpvPlugin)
        } else {
            Log.i(TAG, "MPV plugin not available, falling back to ArtPlayer (Capacitor WebView)")
            startArtPlayer(context, filePath, fileName, mimeType, isExternal)
        }
    }

    fun isMpvAvailable(context: Context): Boolean {
        return try {
            val pluginManager = PluginManager.getInstance(context)
            pluginManager.getInstalledPlugin(PLUGIN_ID)?.enabled == true
        } catch (e: Exception) {
            Log.w(TAG, "isMpvAvailable check failed", e)
            false
        }
    }

    private fun startMpvPlayer(
        context: Context,
        filePath: String,
        fileName: String,
        mimeType: String,
        isExternal: Boolean,
        pluginManager: PluginManager,
        mpvPlugin: io.github.combolite.core.model.PluginInfo
    ) {
        try {
            val intent = pluginManager.createPluginIntent(
                mpvPlugin,
                MpvPlayerActivity::class.java,
                Bundle().apply {
                    putString(EXTRA_FILE_PATH, filePath)
                    putString(EXTRA_FILE_NAME, fileName)
                    putString(EXTRA_MIME_TYPE, mimeType)
                    putBoolean(EXTRA_IS_EXTERNAL, isExternal)
                }
            )

            if (context !is android.app.Activity) {
                intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }

            context.startActivity(intent)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start MPV player, falling back to ArtPlayer", e)
            startArtPlayer(context, filePath, fileName, mimeType, isExternal)
        }
    }

    private fun startArtPlayer(
        context: Context,
        filePath: String,
        fileName: String,
        mimeType: String,
        isExternal: Boolean
    ) {
        try {
            val intent = Intent(context, PlayerActivityCapacitor::class.java).apply {
                putExtra(EXTRA_FILE_PATH, filePath)
                putExtra(EXTRA_FILE_NAME, fileName)
                putExtra(EXTRA_MIME_TYPE, mimeType)
                putBoolean(EXTRA_IS_EXTERNAL, isExternal)

                if (context !is android.app.Activity) {
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                }
            }
            context.startActivity(intent)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start ArtPlayer", e)
        }
    }
}
