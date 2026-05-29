package com.encvgo.app

import android.content.Context
import android.content.Intent
import android.util.Log
import android.widget.Toast
import androidx.core.content.FileProvider
import java.io.File

object PlayerEntry {
    private const val TAG = "PlayerEntry"
    private const val PLUGIN_ID = "com.encvgo.plugin.mpv"
    private const val PREFS_NAME = "encv_player_prefs"
    private const val PREF_KEY_VIDEO_PLAYER = "video_player"
    const val EXTRA_FILE_PATH = "file_path"
    const val EXTRA_FILE_NAME = "file_name"
    const val EXTRA_MIME_TYPE = "mime_type"
    const val EXTRA_IS_EXTERNAL = "is_external"
    const val EXTRA_BACKEND_URL = "backend_url"
    const val EXTRA_MODE = "player_mode"

    fun play(
        context: Context,
        filePath: String,
        fileName: String,
        mimeType: String = "",
        isExternal: Boolean = false,
        mode: String = ""
    ) {
        val effectiveMode = if (mode.isNotEmpty()) {
            if (mode == "mpv") "mpv-plugin" else mode
        } else {
            val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            val rawMode = prefs.getString(PREF_KEY_VIDEO_PLAYER, "artplayer") ?: "artplayer"
            if (rawMode == "mpv") "mpv-plugin" else rawMode
        }

        Log.i(TAG, "play() mode=$effectiveMode (param=$mode) filePath=$filePath")

        when (effectiveMode) {
            "mpv-plugin" -> {
                startMpvPlayer(context, filePath, fileName, mimeType, isExternal)
            }
            "external" -> openExternal(context, filePath)
            else -> startArtPlayer(context, filePath, fileName)
        }
    }

    fun isMpvAvailable(context: Context): Boolean {
        return try {
            val pm = com.combo.core.runtime.PluginManager
            if (!pm.isInitialized) return false
            pm.getPluginInfo(PLUGIN_ID) != null
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
        isExternal: Boolean
    ) {
        try {
            val intent = Intent(context, EncvHostActivity::class.java).apply {
                putExtra(EXTRA_FILE_PATH, filePath)
                putExtra(EXTRA_FILE_NAME, fileName)
                putExtra(EXTRA_MIME_TYPE, mimeType)
                putExtra(EXTRA_IS_EXTERNAL, isExternal)
                putExtra(EXTRA_BACKEND_URL, getBackendBaseUrl(context))
                putExtra(EXTRA_MODE, "mpv-plugin")
                putExtra("_combo_plugin_id", PLUGIN_ID)
                putExtra("_combo_target_activity", "com.encvgo.plugin.mpv.MpvPlayerActivity")
                if (context !is android.app.Activity) {
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                }
            }
            context.startActivity(intent)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start MPV player plugin", e)
            Toast.makeText(context, "MPV 插件启动失败: ${e.message}", Toast.LENGTH_LONG).show()
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
                Toast.makeText(
                    context,
                    "No app can open this file",
                    Toast.LENGTH_SHORT
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
