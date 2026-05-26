package com.encvgo.app

import android.content.Context
import android.content.Intent
import android.os.Bundle
import android.util.Log
import androidx.core.content.FileProvider
import io.github.combolite.core.PluginManager
import com.encvgo.plugin.mpv.MpvPlayerActivity
import java.io.File

object PlayerEntry {
    private const val TAG = "PlayerEntry"
    private const val PLUGIN_ID = "mpv-player"
    private const val PREFS_NAME = "encv_player_prefs"
    private const val PREF_KEY_VIDEO_PLAYER = "video_player"
    private const val EXTRA_FILE_PATH = "file_path"
    private const val EXTRA_FILE_NAME = "file_name"
    private const val EXTRA_MIME_TYPE = "mime_type"
    private const val EXTRA_IS_EXTERNAL = "is_external"
    private const val EXTRA_BACKEND_URL = "backend_url"

    fun play(
        context: Context,
        filePath: String,
        fileName: String,
        mimeType: String = "",
        isExternal: Boolean = false
    ) {
        val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        val rawMode = prefs.getString(PREF_KEY_VIDEO_PLAYER, "artplayer") ?: "artplayer"
        val mode = if (rawMode == "mpv") "mpv-plugin" else rawMode

        Log.i(TAG, "play() mode=$mode filePath=$filePath")

        when (mode) {
            "mpv-plugin" -> {
                val pm = try { PluginManager.getInstance(context) } catch (e: Exception) {
                    Log.w(TAG, "PluginManager not available", e)
                    null
                }
                val mpvPlugin = pm?.getInstalledPlugin(PLUGIN_ID)

                if (mpvPlugin != null && mpvPlugin.enabled) {
                    Log.i(TAG, "MPV plugin available, routing to MpvPlayerActivity")
                    startMpvPlayer(context, filePath, fileName, mimeType, isExternal, pm, mpvPlugin)
                } else {
                    Log.w(TAG, "MPV plugin not available or disabled, showing toast + fallback")
                    android.widget.Toast.makeText(
                        context,
                        "MPV plugin not available",
                        android.widget.Toast.LENGTH_SHORT
                    ).show()
                    startArtPlayer(context, filePath, fileName)
                }
            }
            "external" -> openExternal(context, filePath)
            else -> startArtPlayer(context, filePath, fileName)
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
                    putString(EXTRA_BACKEND_URL, getBackendBaseUrl(context))
                }
            )

            if (context !is android.app.Activity) {
                intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }

            context.startActivity(intent)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start MPV player, falling back to ArtPlayer", e)
            startArtPlayer(context, filePath, fileName)
        }
    }

    private fun startArtPlayer(
        context: Context,
        filePath: String,
        fileName: String
    ) {
        try {
            val intent = Intent(context, PlayerActivityCapacitor::class.java).apply {
                putExtra(EXTRA_FILE_PATH, filePath)
                putExtra(EXTRA_FILE_NAME, fileName)

                if (context !is android.app.Activity) {
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                }
            }
            context.startActivity(intent)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start ArtPlayer", e)
        }
    }

    private fun openExternal(context: Context, filePath: String) {
        val file = File(filePath)
        if (file.exists()) {
            val uri = FileProvider.getUriForFile(
                context,
                "${context.packageName}.fileprovider",
                file
            )
            val intent = Intent(Intent.ACTION_VIEW).apply {
                setDataAndType(uri, "*/*")
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            if (intent.resolveActivity(context.packageManager) != null) {
                context.startActivity(intent)
            } else {
                android.widget.Toast.makeText(
                    context,
                    "No app can open this file",
                    android.widget.Toast.LENGTH_SHORT
                ).show()
            }
        } else {
            Log.w(TAG, "openExternal: file does not exist: $filePath")
        }
    }

    private fun getBackendBaseUrl(context: Context): String {
        return "http://127.0.0.1:8899"
    }
}
