package com.encvgo.app

import android.Manifest
import android.app.Activity
import android.content.Context
import android.content.Intent
import android.content.BroadcastReceiver
import android.content.IntentFilter
import android.net.Uri
import android.util.Log
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.content.pm.ActivityInfo
import androidx.core.content.ContextCompat
import com.getcapacitor.JSObject
import com.getcapacitor.Plugin
import com.getcapacitor.PluginCall
import com.getcapacitor.PluginMethod
import com.getcapacitor.annotation.CapacitorPlugin
import com.encvgo.combolite.EncvComboLiteHost
import com.encvgo.combolite.OpenListStatusBridge
import com.encvgo.combolite.diagnostic.DiagnosticKit
import com.encvgo.combolite.model.OperationResult
import com.encvgo.combolite.model.PluginFullState
import java.io.File
import java.util.concurrent.ConcurrentHashMap
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

private const val REQUEST_CODE_PLUGIN_PICK = 9001
private const val REQUEST_CODE_INSTALL_CONFIRM = 9002
private const val REQUEST_CODE_MPV_PLAYER = 9003
private const val REQUEST_CODE_PICK_FOLDER = 9010

@CapacitorPlugin(
    name = "GoProcess",
    requestCodes = [REQUEST_CODE_PLUGIN_PICK, REQUEST_CODE_INSTALL_CONFIRM, REQUEST_CODE_MPV_PLAYER, REQUEST_CODE_PICK_FOLDER]
)
class GoProcessPlugin : Plugin() {

    companion object {
        private const val TAG = "ENCV-go"
    }

    private val pendingCalls = ConcurrentHashMap<String, PluginCall>()
    private var receiverRegistered = false

    private val statusReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            if (intent == null) return
            when (intent.action) {
                EncvGoService.BROADCAST_BACKEND_READY,
                EncvGoService.BROADCAST_BACKEND_STATUS -> resolvePendingCall(intent)
            }
        }
    }

    override fun load() {
        super.load()
        registerStatusReceiver()
    }

    override fun handleOnDestroy() {
        if (receiverRegistered) { context.unregisterReceiver(statusReceiver); receiverRegistered = false }
        pendingCalls.clear()
        super.handleOnDestroy()
    }

    @PluginMethod
    fun restart(call: PluginCall) {
        pendingCalls["restart"] = call
        startService(EncvGoService.ACTION_RESTART, "manual", "restart")
    }

    @PluginMethod
    fun stop(call: PluginCall) {
        startService(EncvGoService.ACTION_STOP, "manual", "stop")
        call.resolve(JSObject().apply { put("success", true); put("port", 0) })
    }

    @PluginMethod
    fun getStatus(call: PluginCall) {
        call.resolve(JSObject().apply {
            put("running", EncvGoService.isRunning)
            put("port", EncvGoService.lastKnownPort)
            if (!EncvGoService.lastError.isNullOrEmpty()) put("lastError", EncvGoService.lastError)
        })
    }

    @PluginMethod
    fun requestNotificationPermission(call: PluginCall) {
        PermissionHelper.requestNotificationPermission(activity, 1001)
        call.resolve(JSObject().put("granted", PermissionHelper.isNotificationGranted(context)))
    }

    @PluginMethod
    fun requestStoragePermission(call: PluginCall) {
        PermissionHelper.requestStoragePermission(activity)
        call.resolve(JSObject().apply { put("granted", false); put("requiresSettings", true) })
    }

    @PluginMethod
    fun requestBatteryOptimization(call: PluginCall) {
        PermissionHelper.requestBatteryOptimization(activity)
        call.resolve(JSObject().apply {
            put("granted", PermissionHelper.isBatteryOptimizationIgnored(context))
            if (!PermissionHelper.isBatteryOptimizationIgnored(context)) put("requiresSettings", true)
        })
    }

    @PluginMethod
    fun isStandaloneMode(call: PluginCall) {
        call.resolve(JSObject().put("standalone", activity is PlayerActivityCapacitor))
    }

    @PluginMethod
    fun getIntentFileInfo(call: PluginCall) {
        call.resolve(if (activity is PlayerActivityCapacitor) JSObject().apply {
            put("path", PlayerActivityCapacitor.intentFilePath)
            put("name", PlayerActivityCapacitor.intentFileName)
            put("mimeType", PlayerActivityCapacitor.intentFileMimeType)
        } else JSObject().apply { put("path", ""); put("name", ""); put("mimeType", "") })
    }

    @PluginMethod
    fun openPlayer(call: PluginCall) {
        try {
            val mode = call.getString("mode") ?: ""
            val effectiveMode = if (mode.isEmpty() || mode == "mpv" || mode == "mpv-plugin") "mpv-activity" else mode

            if (effectiveMode.startsWith("mpv-")) {
                LogBridge.i(TAG, "openPlayer: using startActivityForResult for mode=$effectiveMode")
                val (intent, result) = PlayerEntry.buildMpvIntent(context ?: activity!!,
                    call.getString("filePath") ?: "",
                    call.getString("name") ?: "",
                    call.getString("mimeType") ?: "",
                    isExternal = false)
                if (intent == null || !result.success) {
                    call.resolve(JSObject().apply {
                        put("success", false)
                        put("error", result.error)
                        put("errorDetail", result.errorDetail)
                    })
                } else {
                    pendingCalls["mpvPlayer"] = call
                    call.save()
                    activity.startActivityForResult(intent, REQUEST_CODE_MPV_PLAYER)
                    LogBridge.i(TAG, "openPlayer: startActivityForResult dispatched for $effectiveMode")
                    Handler(Looper.getMainLooper()).postDelayed({
                        if (pendingCalls.containsKey("mpvPlayer")) {
                            LogBridge.w(TAG, "openPlayer: mpv result timeout (15s), resolving with warning")
                            val staleCall = pendingCalls.remove("mpvPlayer")
                            try { staleCall?.resolve(JSObject().apply {
                                put("success", false)
                                put("error", "播放器响应超时")
                                put("errorDetail", "startActivityForResult dispatched but no result within 15s")
                            }) } catch (_: Exception) {}
                        }
                    }, 15000)
                }
            } else {
                val result = PlayerEntry.play(context ?: activity!!,
                    call.getString("filePath") ?: "",
                    call.getString("name") ?: "",
                    call.getString("mimeType") ?: "",
                    isExternal = false,
                    mode = mode)
                if (result.success) {
                    call.resolve(JSObject().apply { put("success", true) })
                } else {
                    call.resolve(JSObject().apply {
                        put("success", false)
                        put("error", result.error)
                        put("errorDetail", result.errorDetail)
                    })
                }
            }
        } catch (e: Exception) { call.reject(e.message) }
    }

    @PluginMethod
    fun closePlayer(call: PluginCall) = call.resolve()

    @PluginMethod
    fun openExternal(call: PluginCall) {
        try {
            context.startActivity(Intent.createChooser(Intent(Intent.ACTION_VIEW).apply {
                setDataAndType(Uri.parse(call.getString("url") ?: return@openExternal), call.getString("mimeType", "video/*"))
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }, null))
            call.resolve()
        } catch (e: Exception) { call.reject(e.message) }
    }

    @PluginMethod
    fun openInPlayer(call: PluginCall) {
        try {
            activity.startActivity(Intent(activity, PlayerActivity::class.java).apply {
                addFlags(Intent.FLAG_ACTIVITY_NEW_DOCUMENT or Intent.FLAG_ACTIVITY_MULTIPLE_TASK or Intent.FLAG_ACTIVITY_RETAIN_IN_RECENTS)
                data = Uri.parse("encvgo://player/${System.currentTimeMillis()}")
                putExtra("file_path", call.getString("path") ?: return@openInPlayer)
                putExtra("file_name", call.getString("name") ?: "")
                putExtra("file_mime_type", call.getString("mimeType") ?: "")
                putExtra(PlayerEntry.EXTRA_MODE, call.getString("mode") ?: "")
            })
            call.resolve()
        } catch (e: Exception) { call.reject(e.message) }
    }

    @PluginMethod
    fun openPlayerHome(call: PluginCall) {
        try {
            activity.startActivity(Intent(activity, PlayerActivity::class.java).apply {
                addFlags(Intent.FLAG_ACTIVITY_NEW_DOCUMENT or Intent.FLAG_ACTIVITY_MULTIPLE_TASK or Intent.FLAG_ACTIVITY_RETAIN_IN_RECENTS)
                data = Uri.parse("encvgo://player/home/${System.currentTimeMillis()}")
            })
            call.resolve()
        } catch (e: Exception) { call.reject(e.message) }
    }

    @PluginMethod
    override fun checkPermissions(call: PluginCall) {
        val s = PermissionHelper.checkAll(context)
        call.resolve(JSObject().apply { put("notifications", s.notifications); put("storage", s.storage); put("batteryOptimization", s.batteryOptimization) })
    }

    @PluginMethod
    fun setScreenOrientation(call: PluginCall) {
        try {
            when (call.getString("orientation", "unlocked")) {
                "portrait" -> activity.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_SENSOR_PORTRAIT
                "landscape" -> activity.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE
                "unlocked" -> activity.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED
            }
            call.resolve()
        } catch (e: Exception) { call.reject(e.message) }
    }

    @PluginMethod
    fun installPlugin(call: PluginCall) {
        val apkPath = call.getString("apkPath") ?: run { call.reject("apkPath is required"); return }
        startInstallConfirm(call, apkPath, File(apkPath).name)
    }

    @PluginMethod
    fun pickAndInstallPlugin(call: PluginCall) {
        AppLogger.log("I", TAG, "pickAndInstallPlugin: starting file picker")
        pendingCalls["pickPlugin"] = call
        try {
            activity.startActivityForResult(Intent(Intent.ACTION_GET_CONTENT).apply {
                type = "application/vnd.android.package-archive"
                addCategory(Intent.CATEGORY_OPENABLE)
                putExtra(Intent.EXTRA_MIME_TYPES, arrayOf("application/vnd.android.package-archive"))
            }, REQUEST_CODE_PLUGIN_PICK)
        } catch (e: Exception) {
            pendingCalls.remove("pickPlugin")?.reject(e.message)
        }
    }

    @PluginMethod
    fun pickFolder(call: PluginCall) {
        pendingCalls["pickFolder"] = call
        try {
            val intent = Intent(Intent.ACTION_OPEN_DOCUMENT_TREE).apply {
                flags = Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION
            }
            activity.startActivityForResult(intent, REQUEST_CODE_PICK_FOLDER)
        } catch (e: Exception) {
            pendingCalls.remove("pickFolder")?.reject(e.message)
        }
    }

    @PluginMethod
    fun checkInstalledPlugins(call: PluginCall) {
        val result = JSObject()
        for (plugin in EncvComboLiteHost.getInstalledPlugins()) {
            result.put(plugin.id, JSObject().apply { put("installed", true); put("enabled", plugin.enabled); put("versionName", plugin.versionName) })
        }
        call.resolve(result)
    }

    @PluginMethod
    fun getPluginFullState(call: PluginCall) {
        val pluginId = call.getString("pluginId") ?: run { call.reject("pluginId required"); return }
        val state = EncvComboLiteHost.getPluginFullState(pluginId)
        call.resolve(JSObject().apply { put("id", state.id); put("status", state.status); put("name", state.name ?: ""); put("version", state.version ?: "") })
    }

    @PluginMethod
    fun getOpenListRuntime(call: PluginCall) {
        Log.e(TAG, "[SAT-DBG][OpenList][Capacitor] getOpenListRuntime() called")
        try {
            val runtime = OpenListStatusBridge.read(context.applicationContext)
            val ret = JSObject().apply {
                put("isInstalled", runtime.isInstalled)
                put("running", runtime.running)
                put("port", runtime.port)
                put("pid", runtime.pid)
                put("dataSizeBytes", runtime.dataSizeBytes)
                put("lastError", runtime.lastError ?: "")
                put("lastUpdateTs", runtime.lastUpdateTs)
            }
            Log.e(TAG, "[SAT-DBG][OpenList][Capacitor] getOpenListRuntime() → $ret")
            call.resolve(ret)
        } catch (e: Throwable) {
            Log.e(TAG, "[SAT-DBG][OpenList][Capacitor] getOpenListRuntime() FAILED", e)
            call.reject("getOpenListRuntime failed: ${e.message}")
        }
    }

    @PluginMethod
    fun controlOpenList(call: PluginCall) {
        val action = call.getString("action", "start") ?: "start"
        Log.e(TAG, "[SAT-DBG][OpenList][Capacitor] controlOpenList() action=$action")
        val ok = OpenListStatusBridge.control(context.applicationContext, action)
        val ret = JSObject().apply { put("success", ok) }
        call.resolve(ret)
    }

    @PluginMethod
    fun ensurePluginLoaded(call: PluginCall) {
        val pluginId = call.getString("pluginId") ?: run { call.reject("pluginId required"); return }
        val success = EncvComboLiteHost.ensurePluginLoaded(pluginId)
        call.resolve(JSObject().apply { put("success", success) })
    }

    @PluginMethod
    fun togglePluginEnabled(call: PluginCall) {
        val pluginId = call.getString("pluginId") ?: run { call.reject("pluginId required"); return }
        val enabled = call.getBoolean("enabled", true) ?: true
        GlobalScope.launch(Dispatchers.IO) {
            when (val r = EncvComboLiteHost.setPluginEnabled(pluginId, enabled)) {
                is OperationResult.Success -> withContext(Dispatchers.Main) {
                    AppLogger.log("I", TAG, "togglePluginEnabled SUCCESS: $pluginId -> ${if (enabled) "ENABLED" else "DISABLED"}")
                    call.resolve(JSObject().apply { put("success", true); put("pluginId", pluginId); put("enabled", enabled) })
                }
                is OperationResult.Failure -> withContext(Dispatchers.Main) {
                    AppLogger.log("E", TAG, "togglePluginEnabled FAILED: ${r.reason}")
                    call.reject(r.reason)
                }
            }
        }
    }

    @PluginMethod
    fun uninstallPlugin(call: PluginCall) {
        val pluginId = call.getString("pluginId") ?: run { call.reject("pluginId required"); return }
        GlobalScope.launch(Dispatchers.IO) {
            when (val r = EncvComboLiteHost.uninstallPlugin(pluginId)) {
                is OperationResult.Success -> withContext(Dispatchers.Main) {
                    AppLogger.log("I", TAG, "uninstallPlugin SUCCESS: $pluginId")
                    call.resolve(JSObject().apply { put("success", true); put("pluginId", pluginId) })
                }
                is OperationResult.Failure -> withContext(Dispatchers.Main) {
                    AppLogger.log("E", TAG, "uninstallPlugin FAILED: ${r.reason}")
                    call.reject(r.reason)
                }
            }
        }
    }

    @PluginMethod
    fun debugLifecycleFlow(call: PluginCall) {
        call.resolve(JSObject().put("debugLog", DiagnosticKit.lifecycleDiagnostic(
            call.getString("pluginId", "com.encvgo.plugin.mpv") ?: "com.encvgo.plugin.mpv", context).joinToString("\n")))
    }

    override fun handleOnActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.handleOnActivityResult(requestCode, resultCode, data)
        when (requestCode) {
            REQUEST_CODE_PLUGIN_PICK -> handlePickResult(resultCode, data)
            REQUEST_CODE_INSTALL_CONFIRM -> handleInstallConfirmResult(resultCode, data)
            REQUEST_CODE_MPV_PLAYER -> handleMpvPlayerResult(resultCode, data)
            REQUEST_CODE_PICK_FOLDER -> handlePickFolderResult(resultCode, data)
        }
    }

    private fun handleMpvPlayerResult(resultCode: Int, data: Intent?) {
        val call = pendingCalls.remove("mpvPlayer") ?: return
        LogBridge.i(TAG, "handleMpvPlayerResult: resultCode=$resultCode data=$data")
        if (data != null) {
            val success = data.getBooleanExtra("player_success", true)
            val error = data.getStringExtra("player_error") ?: ""
            val errorDetail = data.getStringExtra("player_error_detail") ?: ""
            LogBridge.i(TAG, "handleMpvPlayerResult: success=$success error=$error detail=$errorDetail")
            call.resolve(JSObject().apply {
                put("success", success)
                put("error", error)
                put("errorDetail", errorDetail)
            })
        } else {
            LogBridge.w(TAG, "handleMpvPlayerResult: no intent data, assuming user back-pressed (success)")
            call.resolve(JSObject().apply { put("success", true) })
        }
    }

    private fun handlePickResult(resultCode: Int, data: Intent?) {
        val call = pendingCalls.remove("pickPlugin") ?: return
        if (resultCode != Activity.RESULT_OK || data?.data == null) { call.reject("File picker cancelled"); return }
        val tempFile = UriUtils.copyUriToFile(context, data.data!!, File(context.cacheDir, "plugin_install"))
            ?: run { call.reject("Cannot read selected file"); return }
        startInstallConfirm(call, tempFile.absolutePath, tempFile.name)
    }

    private fun handlePickFolderResult(resultCode: Int, data: Intent?) {
        val call = pendingCalls.remove("pickFolder") ?: return
        if (resultCode != Activity.RESULT_OK || data?.data == null) { call.reject("Folder picker cancelled"); return }
        val uri = data.data!!
        val path = resolveTreeUriToPath(uri)
        LogBridge.i(TAG, "pickFolder: uri=$uri → path=$path")
        call.resolve(JSObject().apply { put("path", path) })
    }

    private fun resolveTreeUriToPath(uri: Uri): String {
        if (uri.scheme == "file") return uri.path ?: "/"
        if (uri.authority == "com.android.externalstorage.documents") {
            val docId = uri.path?.removePrefix("/tree/") ?: return "/"
            val parts = docId.split(":", limit = 2)
            val storagePath = when (parts[0]) {
                "primary" -> "/storage/emulated/0"
                else -> "/storage/${parts[0]}"
            }
            return if (parts.size > 1 && parts[1].isNotEmpty()) "$storagePath/${parts[1]}" else storagePath
        }
        try {
            val docFile = android.provider.DocumentsContract.buildDocumentUriUsingTree(uri, android.provider.DocumentsContract.getTreeDocumentId(uri))
            val cursor = context.contentResolver.query(docFile, arrayOf(android.provider.OpenableColumns.DISPLAY_NAME), null, null, null)
            cursor?.use {
                if (it.moveToFirst()) {
                    val name = it.getString(0)
                    val treeId = android.provider.DocumentsContract.getTreeDocumentId(uri)
                    val treeParts = treeId.split(":", limit = 2)
                    val basePath = when (treeParts[0]) {
                        "primary" -> "/storage/emulated/0"
                        else -> "/storage/${treeParts[0]}"
                    }
                    return if (treeParts.size > 1 && treeParts[1].isNotEmpty()) "$basePath/${treeParts[1]}" else basePath
                }
            }
        } catch (_: Exception) {}
        return uri.toString()
    }

    private fun handleInstallConfirmResult(resultCode: Int, data: Intent?) {
        val call = pendingCalls.remove("installConfirm") ?: return
        val apkPath = data?.getStringExtra(InstallConfirmActivity.EXTRA_APK_PATH) ?: call.getString("apkPath") ?: ""
        if (resultCode == Activity.RESULT_OK) executeComboLiteInstall(call, File(apkPath)) else call.reject("\u7528\u6237\u53d6\u6d88\u5b89\u88c5")
    }

    private fun startInstallConfirm(call: PluginCall, apkPath: String, name: String) {
        pendingCalls["installConfirm"] = call; call.getData().put("apkPath", apkPath); call.save()
        try {
            activity.startActivityForResult(Intent(activity, InstallConfirmActivity::class.java).apply {
                putExtra(InstallConfirmActivity.EXTRA_APK_PATH, apkPath)
                putExtra(InstallConfirmActivity.EXTRA_FILE_NAME, name)
            }, REQUEST_CODE_INSTALL_CONFIRM)
        } catch (e: Exception) { pendingCalls.remove("installConfirm"); call.reject(e.message) }
    }

    @PluginMethod
    fun debugInstallFlow(call: PluginCall) {
        val testApk = File(context.cacheDir, "plugin_install").listFiles()?.filter { it.extension == "apk" }?.firstOrNull()
        call.resolve(JSObject().put("debugLog",
            if (testApk != null) DiagnosticKit.installTest(testApk, context).joinToString("\n")
            else "=== Actual installPlugin Test ===\nSKIPPED: no APK file found in plugin_install"))
    }

    @PluginMethod
    fun debugKotlinReflect(call: PluginCall) {
        call.resolve(JSObject().put("debugLog", DiagnosticKit.kotlinReflectHealthCheck().joinToString("\n")))
    }

    @PluginMethod
    fun debugApkValidation(call: PluginCall) {
        val testApk = File(context.cacheDir, "plugin_install").listFiles()?.filter { it.extension == "apk" }?.firstOrNull()
        call.resolve(JSObject().put("debugLog",
            if (testApk != null) DiagnosticKit.apkValidation(testApk, context).joinToString("\n")
            else "No APK file found in plugin_install"))
    }

    @PluginMethod
    fun debugValidationStrategy(call: PluginCall) {
        call.resolve(JSObject().put("debugLog", DiagnosticKit.validationStrategyStatus(context).joinToString("\n")))
    }

    @PluginMethod
    fun getLocalFilePath(call: PluginCall) {
        val path = call.getString("path", "") ?: ""
        call.resolve(JSObject().apply {
            put("path", when {
                path.isEmpty() -> ""
                File(path).exists() && File(path).isFile && File(path).canRead() -> File(path).absolutePath
                File(context.filesDir, path.removePrefix("/")).exists() && File(context.filesDir, path.removePrefix("/")).isFile -> File(context.filesDir, path.removePrefix("/")).absolutePath
                else -> ""
            })
        })
    }

    @PluginMethod
    fun startMpvInPlace(call: PluginCall) {
        try {
            val result = MpvEmbedService.startEmbed(
                activity = activity,
                containerId = call.getString("containerId") ?: "mpv-container",
                filePath = call.getString("filePath") ?: "",
                fileName = call.getString("name") ?: "",
                mimeType = call.getString("mimeType") ?: "",
                isExternal = false
            )
            call.resolve(JSObject().apply {
                put("success", result.success)
                put("error", result.error)
                put("errorDetail", result.errorDetail)
            })
        } catch (e: Exception) { call.reject(e.message) }
    }

    @PluginMethod
    fun stopMpvInPlace(call: PluginCall) {
        val success = MpvEmbedService.stopEmbed()
        call.resolve(JSObject().apply { put("success", success); put("embedded", MpvEmbedService.isEmbedded()) })
    }

    private fun executeComboLiteInstall(call: PluginCall, apkFile: File) {
        AppLogger.log("I", TAG, "executeComboLiteInstall: ${apkFile.name} (${apkFile.length()}B)")
        GlobalScope.launch(Dispatchers.IO) {
            when (val r = EncvComboLiteHost.installPlugin(apkFile)) {
                is OperationResult.Success -> withContext(Dispatchers.Main) {
                    AppLogger.log("I", TAG, "install SUCCESS: ${r.data.id}")
                    call.resolve(JSObject().apply { put("success", true); put("method", "combolite"); put("pluginId", r.data.id) })
                }
                is OperationResult.Failure -> withContext(Dispatchers.Main) {
                    AppLogger.log("E", TAG, "install FAILED: ${r.reason}")
                    call.reject(r.reason)
                }
            }
        }
    }

    @PluginMethod
    fun exportLogs(call: PluginCall) {
        try {
            val r = LogExporter.export(context)
            if (r.success) call.resolve(JSObject().apply { put("success", true); put("path", r.path) }) else call.reject("Failed to export logs")
        } catch (e: Exception) { call.reject(e.message) }
    }

    @PluginMethod
    fun clearLogs(call: PluginCall) {
        call.resolve(JSObject().put("success", LogExporter.clear(context)))
    }

    @PluginMethod
    fun openLogViewer(call: PluginCall) {
        call.resolve(JSObject().put("success", LogExporter.openViewer(context)))
    }

    @PluginMethod
    fun saveDevLogs(call: PluginCall) {
        val p = LogExporter.saveDevLogs(context, call.getString("logs") ?: run { call.reject("logs required"); return })
        if (p != null) call.resolve(JSObject().apply { put("success", true); put("path", p) }) else call.reject("Failed to save dev logs")
    }

    private fun registerStatusReceiver() {
        if (receiverRegistered) return
        val filter = IntentFilter().apply { addAction(EncvGoService.BROADCAST_BACKEND_READY); addAction(EncvGoService.BROADCAST_BACKEND_STATUS) }
        if (Build.VERSION.SDK_INT >= 33) context.registerReceiver(statusReceiver, filter, Context.RECEIVER_NOT_EXPORTED)
        else @Suppress("DEPRECATION") context.registerReceiver(statusReceiver, filter)
        receiverRegistered = true
    }

    private fun startService(action: String, source: String, command: String) =
        ContextCompat.startForegroundService(context, EncvGoService.createIntent(context, action, source).apply { putExtra(EncvGoService.EXTRA_COMMAND, command) })

    private fun resolvePendingCall(intent: Intent) {
        val cmd = intent.getStringExtra(EncvGoService.EXTRA_COMMAND) ?: return
        if (cmd != "restart") return
        val call = pendingCalls.remove("restart") ?: return
        if (intent.getBooleanExtra(EncvGoService.EXTRA_RUNNING, false) && intent.getIntExtra(EncvGoService.EXTRA_PORT, 0) > 0)
            call.resolve(JSObject().apply { put("success", true); put("port", intent.getIntExtra(EncvGoService.EXTRA_PORT, 0)) })
        else intent.getStringExtra(EncvGoService.EXTRA_ERROR)?.let { call.reject(it) } ?: call.reject("Unknown error")
    }
}
