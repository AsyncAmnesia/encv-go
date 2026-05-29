package com.encvgo.app

import android.content.Context
import android.content.Intent
import android.util.Log
import androidx.core.content.FileProvider
import com.combo.core.model.LoadedPluginInfo
import com.encvgo.combolite.EncvComboLiteHost
import java.io.File

data class PlayResult(
    val success: Boolean,
    val error: String = "",
    val errorDetail: String = ""
)

object PlayerEntry {
    private const val TAG = "PlayerEntry"
    private const val PLUGIN_ID = "com.encvgo.plugin.mpv"
    private const val TARGET_ACTIVITY = "com.encvgo.plugin.mpv.MpvPlayerActivity"
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
    ): PlayResult {
        val effectiveMode = if (mode.isNotEmpty()) {
            if (mode == "mpv") "mpv-plugin" else mode
        } else {
            val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            val rawMode = prefs.getString(PREF_KEY_VIDEO_PLAYER, "artplayer") ?: "artplayer"
            if (rawMode == "mpv") "mpv-plugin" else rawMode
        }

        Log.i(TAG, "play() mode=$effectiveMode (param=$mode) filePath=$filePath fileName=$fileName")

        return when (effectiveMode) {
            "mpv-plugin" -> startMpvPlayer(context, filePath, fileName, mimeType, isExternal)
            "external" -> openExternal(context, filePath)
            else -> startArtPlayer(context, filePath, fileName)
        }
    }

    fun isMpvAvailable(context: Context): Boolean = EncvComboLiteHost.isPluginAvailable(PLUGIN_ID)

    private fun startMpvPlayer(
        context: Context,
        filePath: String,
        fileName: String,
        mimeType: String,
        isExternal: Boolean
    ): PlayResult {
        Log.i(TAG, "startMpvPlayer: filePath=$filePath fileName=$fileName mimeType=$mimeType")

        // 1. 检查框架初始化
        if (!EncvComboLiteHost.isInitialized) {
            Log.w(TAG, "startMpvPlayer: ComboLite not initialized")
            return PlayResult(false, "播放器框架未初始化", "PluginManager.isInitialized=false")
        }
        Log.i(TAG, "startMpvPlayer: ComboLite initialized ✓")

        // 2. 检查插件完整状态
        val state = EncvComboLiteHost.getPluginFullState(PLUGIN_ID)
        Log.i(TAG, "startMpvPlayer: plugin state=$state.status name=${state.name} version=${state.version}")

        when (state.status) {
            "not_installed" -> {
                Log.w(TAG, "startMpvPlayer: MPV plugin not installed")
                return PlayResult(false, "MPV 播放器未安装", "请前往扩展管理安装")
            }
            "disabled" -> {
                Log.w(TAG, "startMpvPlayer: MPV plugin disabled")
                return PlayResult(false, "MPV 播放器已禁用", "请前往扩展管理启用")
            }
            "framework_not_ready" -> {
                Log.w(TAG, "startMpvPlayer: framework not ready")
                return PlayResult(false, "播放器框架未就绪", "请重启应用")
            }
            "not_loaded", "load_failed" -> {
                Log.w(TAG, "startMpvPlayer: plugin state=${state.status}, attempting load...")
                val loaded = EncvComboLiteHost.ensurePluginLoaded(PLUGIN_ID)
                Log.i(TAG, "startMpvPlayer: ensurePluginLoaded result=$loaded")
                if (!loaded) {
                    return PlayResult(false, "MPV 加载失败", "请重启应用或重新启用扩展")
                }
            }
        }

        // 3. 关键：检查 LoadedPluginInfo 是否包含目标 Activity
        // startActivity() 成功 ≠ 播放成功！EncvHostActivity 启动后可能白屏
        val loadedInfo = EncvComboLiteHost.getLoadedPluginInfo(PLUGIN_ID)
        if (loadedInfo == null) {
            Log.e(TAG, "startMpvPlayer: getLoadedPluginInfo returned null after successful load!")
            return PlayResult(false, "MPV 插件信息异常", "已加载但 getLoadedPluginInfo 返回 null")
        }
        Log.i(TAG, "startMpvPlayer: loadedInfo id=${loadedInfo.id} name=${loadedInfo.pluginName}")

        // 4. 检查插件是否注册了 Activity（ProxyManager 能否找到）
        val activities = loadedInfo.activities
        Log.i(TAG, "startMpvPlayer: plugin activities=$activities target=$TARGET_ACTIVITY")
        if (activities.isEmpty()) {
            Log.e(TAG, "startMpvPlayer: plugin has NO registered activities!")
            return PlayResult(false, "MPV 插件无 Activity", "插件未声明任何 Activity，APK 可能损坏")
        }
        if (!activities.any { it.contains("MpvPlayerActivity", ignoreCase = true) }) {
            Log.w(TAG, "startMpvPlayer: target $TARGET_ACTIVITY not found in plugin activities: $activities")
            return PlayResult(false, "MPV Activity 未注册", "目标 Activity 不在插件清单中: $TARGET_ACTIVITY")
        }

        // 5. 启动播放 — 此时才真正 startActivity
        return try {
            val extras = mapOf<String, Any>(
                EXTRA_FILE_PATH to filePath,
                EXTRA_FILE_NAME to fileName,
                EXTRA_MIME_TYPE to mimeType,
                EXTRA_IS_EXTERNAL to isExternal,
                EXTRA_BACKEND_URL to getBackendBaseUrl(context),
                EXTRA_MODE to "mpv-plugin",
            )
            val intent = EncvComboLiteHost.createProxyIntent(
                context = context,
                pluginId = PLUGIN_ID,
                targetActivity = TARGET_ACTIVITY,
                hostActivityClass = EncvHostActivity::class.java,
                extras = extras
            )
            context.startActivity(intent)
            Log.i(TAG, "startMpvPlayer: startActivity dispatched ✓ (result=pending, verify in EncvHostActivity)")
            PlayResult(true)
        } catch (e: Exception) {
            Log.e(TAG, "startMpvPlayer: startActivity failed: ${e.message}", e)
            PlayResult(false, "播放器启动失败", e.message ?: "Unknown error")
        }
    }

    private fun startArtPlayer(context: Context, filePath: String, fileName: String): PlayResult {
        Log.i(TAG, "startArtPlayer: filePath=$filePath fileName=$fileName")
        return try {
            val intent = Intent(context, PlayerActivityCapacitor::class.java).apply {
                putExtra(EXTRA_FILE_PATH, filePath)
                putExtra(EXTRA_FILE_NAME, fileName)
                if (context !is android.app.Activity) addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            context.startActivity(intent)
            Log.i(TAG, "startArtPlayer: success ✓")
            PlayResult(true)
        } catch (e: Exception) {
            Log.e(TAG, "startArtPlayer: failed: ${e.message}", e)
            PlayResult(false, "内置播放器启动失败", e.message ?: "Unknown error")
        }
    }

    private fun openExternal(context: Context, filePath: String): PlayResult {
        Log.i(TAG, "openExternal: filePath=$filePath")
        val file = File(filePath)
        if (!file.exists()) {
            Log.w(TAG, "openExternal: file does not exist: $filePath")
            return PlayResult(false, "文件不存在", "path=$filePath")
        }
        return try {
            val uri = FileProvider.getUriForFile(context, "${context.packageName}.fileprovider", file)
            val intent = Intent(Intent.ACTION_VIEW).apply {
                setDataAndType(uri, "*/*")
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            if (intent.resolveActivity(context.packageManager) != null) {
                context.startActivity(intent)
                Log.i(TAG, "openExternal: success ✓")
                PlayResult(true)
            } else {
                Log.w(TAG, "openExternal: no app can open this file")
                PlayResult(false, "没有应用可以打开此文件", "resolveActivity=null")
            }
        } catch (e: Exception) {
            Log.e(TAG, "openExternal: failed: ${e.message}", e)
            PlayResult(false, "外部打开失败", e.message ?: "Unknown error")
        }
    }

    private fun getBackendBaseUrl(context: Context): String = "http://127.0.0.1:8899"
}