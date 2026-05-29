package com.encvgo.app

import android.Manifest
import android.app.Activity
import android.content.Context
import android.content.Intent
import android.content.BroadcastReceiver
import android.content.IntentFilter
import android.net.Uri
import android.os.Build
import android.content.pm.ActivityInfo
import android.util.Log
import androidx.core.content.ContextCompat
import com.getcapacitor.JSObject
import com.getcapacitor.Plugin
import com.getcapacitor.PluginCall
import com.getcapacitor.PluginMethod
import com.getcapacitor.annotation.CapacitorPlugin
import com.encvgo.combolite.EncvComboLiteHost
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

@CapacitorPlugin(
    name = "GoProcess",
    requestCodes = [REQUEST_CODE_PLUGIN_PICK, REQUEST_CODE_INSTALL_CONFIRM]
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
            PlayerEntry.play(context ?: activity!!, call.getString("filePath") ?: "",
                call.getString("name") ?: "", call.getString("mimeType") ?: "",
                isExternal = false, mode = call.getString("mode") ?: "")
            call.resolve()
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
        }
    }

    private fun handlePickResult(resultCode: Int, data: Intent?) {
        val call = pendingCalls.remove("pickPlugin") ?: return
        if (resultCode != Activity.RESULT_OK || data?.data == null) { call.reject("File picker cancelled"); return }
        val tempFile = UriUtils.copyUriToFile(context, data.data!!, File(context.cacheDir, "plugin_install"))
            ?: run { call.reject("Cannot read selected file"); return }
        startInstallConfirm(call, tempFile.absolutePath, tempFile.name)
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
