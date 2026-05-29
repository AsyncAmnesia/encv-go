package com.encvgo.app

import android.Manifest
import android.app.Activity
import android.content.Intent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.IntentFilter
import android.content.pm.ActivityInfo
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.os.PowerManager
import android.provider.Settings
import android.util.Log
import androidx.core.content.ContextCompat
import com.getcapacitor.JSObject
import com.getcapacitor.Plugin
import com.getcapacitor.PluginCall
import com.getcapacitor.PluginMethod
import com.getcapacitor.annotation.CapacitorPlugin
import java.io.File
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.ConcurrentLinkedQueue
import com.combo.core.runtime.PluginManager
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext
import kotlin.reflect.jvm.javaMethod

private const val REQUEST_CODE_PLUGIN_PICK = 9001
private const val REQUEST_CODE_INSTALL_CONFIRM = 9002

@CapacitorPlugin(
    name = "GoProcess",
    requestCodes = [REQUEST_CODE_PLUGIN_PICK, REQUEST_CODE_INSTALL_CONFIRM]
)
class GoProcessPlugin : Plugin() {

    companion object {
        private const val TAG = "ENCV-go"

        private val appLogBuffer = ConcurrentLinkedQueue<String>()
        private const val APP_LOG_MAX = 3000

        fun appLog(level: String, tag: String, msg: String) {
            val entry = "${System.currentTimeMillis()} $level/$tag: $msg"
            appLogBuffer.add(entry)
            while (appLogBuffer.size > APP_LOG_MAX) {
                appLogBuffer.poll()
            }
        }

        fun getAppLogs(): String = appLogBuffer.joinToString("\n")

        fun clearAppLogs() = appLogBuffer.clear()
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
        appLog("I", TAG, "GoProcessPlugin.load: requestCodes declared in @CapacitorPlugin, handleOnActivityResult will work")
    }

    override fun handleOnDestroy() {
        if (receiverRegistered) {
            context.unregisterReceiver(statusReceiver)
            receiverRegistered = false
        }
        pendingCalls.clear()
        super.handleOnDestroy()
    }

    @PluginMethod
    fun restart(call: PluginCall) {
        Log.d(TAG, "GoProcess.restart() called")
        pendingCalls["restart"] = call
        startService(EncvGoService.ACTION_RESTART, "manual", "restart")
    }

    @PluginMethod
    fun stop(call: PluginCall) {
        Log.d(TAG, "GoProcess.stop() called")
        startService(EncvGoService.ACTION_STOP, "manual", "stop")
        val result = JSObject()
        result.put("success", true)
        result.put("port", 0)
        call.resolve(result)
    }

    @PluginMethod
    fun getStatus(call: PluginCall) {
        Log.d(TAG, "GoProcess.getStatus() called")
        val result = JSObject()
        result.put("running", EncvGoService.isRunning)
        result.put("port", EncvGoService.lastKnownPort)
        if (!EncvGoService.lastError.isNullOrEmpty()) {
            result.put("lastError", EncvGoService.lastError)
        }
        call.resolve(result)
    }

    @PluginMethod
    fun requestNotificationPermission(call: PluginCall) {
        Log.d(TAG, "GoProcess.requestNotificationPermission() called")
        val result = JSObject()
        if (Build.VERSION.SDK_INT < 33) {
            result.put("granted", true)
            call.resolve(result)
            return
        }
        if (activity.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED) {
            result.put("granted", true)
            call.resolve(result)
            return
        }
        activity.requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), 1001)
        result.put("granted", false)
        call.resolve(result)
    }

    @PluginMethod
    fun requestStoragePermission(call: PluginCall) {
        Log.d(TAG, "GoProcess.requestStoragePermission() called")
        val result = JSObject()
        if (Environment.isExternalStorageManager()) {
            result.put("granted", true)
            call.resolve(result)
            return
        }
        try {
            val intent = Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION)
            intent.data = Uri.parse("package:${context.packageName}")
            activity.startActivity(intent)
        } catch (e: Exception) {
            val intent = Intent(Settings.ACTION_MANAGE_ALL_FILES_ACCESS_PERMISSION)
            activity.startActivity(intent)
        }
        result.put("granted", false)
        result.put("requiresSettings", true)
        call.resolve(result)
    }

    @PluginMethod
    fun requestBatteryOptimization(call: PluginCall) {
        Log.d(TAG, "GoProcess.requestBatteryOptimization() called")
        val result = JSObject()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            val pm = context.getSystemService(Context.POWER_SERVICE) as PowerManager
            if (pm.isIgnoringBatteryOptimizations(context.packageName)) {
                result.put("granted", true)
            } else {
                try {
                    val intent = Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS)
                    intent.data = Uri.parse("package:${context.packageName}")
                    activity.startActivity(intent)
                } catch (e: Exception) {
                    Log.w(TAG, "requestBatteryOptimization failed", e)
                }
                result.put("granted", false)
                result.put("requiresSettings", true)
            }
        } else {
            result.put("granted", true)
        }
        call.resolve(result)
    }

    @PluginMethod
    fun isStandaloneMode(call: PluginCall) {
        val result = JSObject()
        result.put("standalone", activity is PlayerActivityCapacitor)
        call.resolve(result)
    }

    @PluginMethod
    fun getIntentFileInfo(call: PluginCall) {
        val result = JSObject()
        if (activity is PlayerActivityCapacitor) {
            result.put("path", PlayerActivityCapacitor.intentFilePath)
            result.put("name", PlayerActivityCapacitor.intentFileName)
            result.put("mimeType", PlayerActivityCapacitor.intentFileMimeType)
        } else {
            result.put("path", "")
            result.put("name", "")
            result.put("mimeType", "")
        }
        call.resolve(result)
    }

    @PluginMethod
    fun openPlayer(call: PluginCall) {
        val filePath = call.getString("filePath", "") ?: ""
        val name = call.getString("name", "") ?: ""
        val mimeType = call.getString("mimeType", "") ?: ""
        val mode = call.getString("mode", "") ?: ""
        try {
            Log.d(TAG, "openPlayer: filePath=$filePath, name=$name, mimeType=$mimeType, mode=$mode")
            PlayerEntry.play(context ?: activity!!, filePath, name, mimeType, isExternal = false, mode = mode)
            call.resolve()
        } catch (e: Exception) {
            Log.e(TAG, "openPlayer failed", e)
            call.reject("Failed to open player: ${e.message}")
        }
    }

    @PluginMethod
    fun closePlayer(call: PluginCall) {
        try {
            Log.d(TAG, "closePlayer: MPV player is standalone Activity, no-op")
            call.resolve()
        } catch (e: Exception) {
            call.reject("Failed to close player: ${e.message}")
        }
    }

    @PluginMethod
    fun openExternal(call: PluginCall) {
        val url = call.getString("url", "")
        val mimeType = call.getString("mimeType", "video/*")
        if (url.isNullOrEmpty()) {
            call.reject("url is required")
            return
        }
        try {
            Log.d(TAG, "openExternal: url=$url, mimeType=$mimeType")
            val intent = Intent(Intent.ACTION_VIEW).apply {
                setDataAndType(Uri.parse(url), mimeType)
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            val chooser = Intent.createChooser(intent, null)
            activity.startActivity(chooser)
            call.resolve()
        } catch (e: Exception) {
            Log.e(TAG, "openExternal failed", e)
            call.reject("Failed to open externally: ${e.message}")
        }
    }

    @PluginMethod
    fun openInPlayer(call: PluginCall) {
        val path = call.getString("path", "")
        val name = call.getString("name", "")
        val mimeType = call.getString("mimeType", "")
        val mode = call.getString("mode", "")
        if (path.isNullOrEmpty()) {
            Log.w(TAG, "openInPlayer rejected: path is empty")
            call.reject("path is required")
            return
        }
        try {
            Log.d(TAG, "openInPlayer: path=$path, name=$name, mimeType=$mimeType, mode=$mode")
            val uniqueId = System.currentTimeMillis().toString()
            val intent = Intent(activity, PlayerActivity::class.java).apply {
                addFlags(
                    Intent.FLAG_ACTIVITY_NEW_DOCUMENT
                        or Intent.FLAG_ACTIVITY_MULTIPLE_TASK
                        or Intent.FLAG_ACTIVITY_RETAIN_IN_RECENTS
                )
                data = Uri.parse("encvgo://player/$uniqueId")
                putExtra("file_path", path)
                putExtra("file_name", name)
                putExtra("file_mime_type", mimeType)
                putExtra(PlayerEntry.EXTRA_MODE, mode)
            }
            Log.d(TAG, "openInPlayer: launching with NEW_DOCUMENT+MULTIPLE_TASK+RETAIN_IN_RECENTS, data=${intent.data}")
            activity.startActivity(intent)
            call.resolve()
        } catch (e: Exception) {
            Log.e(TAG, "openInPlayer failed to start PlayerActivity", e)
            call.reject("Failed to open player: ${e.message}")
        }
    }

    @PluginMethod
    fun openPlayerHome(call: PluginCall) {
        try {
            Log.d(TAG, "openPlayerHome: launching PlayerActivity without file")
            val intent = Intent(activity, PlayerActivity::class.java).apply {
                addFlags(
                    Intent.FLAG_ACTIVITY_NEW_DOCUMENT
                        or Intent.FLAG_ACTIVITY_MULTIPLE_TASK
                        or Intent.FLAG_ACTIVITY_RETAIN_IN_RECENTS
                )
                data = Uri.parse("encvgo://player/home/${System.currentTimeMillis()}")
            }
            activity.startActivity(intent)
            call.resolve()
        } catch (e: Exception) {
            Log.e(TAG, "openPlayerHome failed", e)
            call.reject("Failed to open player: ${e.message}")
        }
    }

    @PluginMethod
    override fun checkPermissions(call: PluginCall) {
        Log.d(TAG, "GoProcess.checkPermissions() called")
        val result = JSObject()

        val notificationGranted = if (Build.VERSION.SDK_INT >= 33) {
            activity.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
        } else {
            true
        }
        result.put("notifications", notificationGranted)
        result.put("storage", Environment.isExternalStorageManager())
        val batteryOptGranted = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            val pm = context.getSystemService(Context.POWER_SERVICE) as PowerManager
            pm.isIgnoringBatteryOptimizations(context.packageName)
        } else {
            true
        }
        result.put("batteryOptimization", batteryOptGranted)
        call.resolve(result)
    }

    private fun registerStatusReceiver() {
        if (receiverRegistered) return
        val filter = IntentFilter().apply {
            addAction(EncvGoService.BROADCAST_BACKEND_READY)
            addAction(EncvGoService.BROADCAST_BACKEND_STATUS)
        }
        if (Build.VERSION.SDK_INT >= 33) {
            context.registerReceiver(statusReceiver, filter, Context.RECEIVER_NOT_EXPORTED)
        } else {
            @Suppress("DEPRECATION")
            context.registerReceiver(statusReceiver, filter)
        }
        receiverRegistered = true
    }

    private fun startService(action: String, source: String, command: String) {
        val serviceIntent = EncvGoService.createIntent(context, action, source).apply {
            putExtra(EncvGoService.EXTRA_COMMAND, command)
        }
        ContextCompat.startForegroundService(context, serviceIntent)
    }

    private fun resolvePendingCall(intent: Intent) {
        val command = intent.getStringExtra(EncvGoService.EXTRA_COMMAND) ?: return
        if (command != "restart") return

        val call = pendingCalls.remove("restart") ?: return
        val running = intent.getBooleanExtra(EncvGoService.EXTRA_RUNNING, false)
        val port = intent.getIntExtra(EncvGoService.EXTRA_PORT, 0)
        val error = intent.getStringExtra(EncvGoService.EXTRA_ERROR)

        if (running && port > 0) {
            val result = JSObject()
            result.put("success", true)
            result.put("port", port)
            call.resolve(result)
        } else if (!error.isNullOrEmpty()) {
            call.reject(error)
        }
    }

    @PluginMethod
    fun setScreenOrientation(call: PluginCall) {
        val orientation = call.getString("orientation", "unlocked")
        try {
            when (orientation) {
                "portrait" -> activity.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_SENSOR_PORTRAIT
                "landscape" -> activity.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE
                "unlocked" -> activity.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED
                else -> Log.w(TAG, "setScreenOrientation: unknown orientation=$orientation")
            }
            Log.d(TAG, "setScreenOrientation: $orientation")
            call.resolve()
        } catch (e: Exception) {
            Log.e(TAG, "setScreenOrientation failed", e)
            call.reject("Failed to set orientation: ${e.message}")
        }
    }

    @PluginMethod
    fun installPlugin(call: PluginCall) {
        val apkPath = call.getString("apkPath") ?: run {
            call.reject("apkPath is required")
            return
        }
        try {
            val apkFile = File(apkPath)
            if (!apkFile.exists()) {
                call.reject("APK file not found: $apkPath")
                return
            }
            if (!PluginManager.isInitialized) {
                call.reject("PluginManager not initialized, cannot install plugin")
                return
            }
            startInstallConfirm(call, apkPath, apkFile.name)
        } catch (e: Exception) {
            Log.e(TAG, "installPlugin failed", e)
            call.reject("Failed to install plugin: ${e.message}")
        }
    }

    @PluginMethod
    fun pickAndInstallPlugin(call: PluginCall) {
        Log.d(TAG, "pickAndInstallPlugin() called")
        appLog("I", TAG, "pickAndInstallPlugin: starting file picker")
        pendingCalls["pickPlugin"] = call
        try {
            val intent = Intent(Intent.ACTION_GET_CONTENT).apply {
                type = "application/vnd.android.package-archive"
                addCategory(Intent.CATEGORY_OPENABLE)
                putExtra(Intent.EXTRA_MIME_TYPES, arrayOf("application/vnd.android.package-archive"))
            }
            activity.startActivityForResult(intent, REQUEST_CODE_PLUGIN_PICK)
            appLog("I", TAG, "pickAndInstallPlugin: startActivityForResult launched with requestCode=$REQUEST_CODE_PLUGIN_PICK")
        } catch (e: Exception) {
            Log.e(TAG, "pickAndInstallPlugin failed", e)
            appLog("E", TAG, "pickAndInstallPlugin: FAILED ${e.message}")
            pendingCalls.remove("pickPlugin")?.reject("Failed to open file picker: ${e.message}")
        }
    }

    @PluginMethod
    fun checkInstalledPlugins(call: PluginCall) {
        Log.d(TAG, "checkInstalledPlugins() called")
        val result = JSObject()
        try {
            if (PluginManager.isInitialized) {
                val plugins = PluginManager.getAllInstallPlugins()
                for (plugin in plugins) {
                    result.put(plugin.id, true)
                }
                Log.i(TAG, "checkInstalledPlugins via ComboLite: $result")
                call.resolve(result)
                return
            }
            Log.w(TAG, "checkInstalledPlugins: PluginManager not initialized, returning empty")
            call.resolve(result)
        } catch (e: Exception) {
            Log.e(TAG, "checkInstalledPlugins failed", e)
            call.reject("Failed to check installed plugins: ${e.message}")
        }
    }

    @PluginMethod
    fun togglePluginEnabled(call: PluginCall) {
        val pluginId = call.getString("pluginId") ?: run {
            call.reject("pluginId is required")
            return
        }
        val enabled = call.getBoolean("enabled", true) ?: true
        Log.d(TAG, "togglePluginEnabled() called: pluginId=$pluginId, enabled=$enabled")
        appLog("I", TAG, "togglePluginEnabled: pluginId=$pluginId, enabled=$enabled")

        if (!PluginManager.isInitialized) {
            call.reject("PluginManager not initialized")
            return
        }

        GlobalScope.launch(Dispatchers.IO) {
            try {
                PluginManager.setPluginEnabled(pluginId, enabled)
                val state = if (enabled) "ENABLED" else "DISABLED"
                Log.i(TAG, "togglePluginEnabled SUCCESS: $pluginId -> $state")
                appLog("I", TAG, "togglePluginEnabled SUCCESS: $pluginId -> $state")

                val result = JSObject().apply {
                    put("success", true)
                    put("pluginId", pluginId)
                    put("enabled", enabled)
                }
                withContext(Dispatchers.Main) { call.resolve(result) }
            } catch (e: Error) {
                Log.e(TAG, "togglePluginEnabled Error: ${e.javaClass.simpleName}: ${e.message}", e)
                appLog("E", TAG, "togglePluginEnabled ERROR: ${e.javaClass.simpleName}: ${e.message}")
                withContext(Dispatchers.Main) { call.reject("togglePluginEnabled error: ${e.javaClass.simpleName}: ${e.message}") }
            } catch (e: Exception) {
                Log.e(TAG, "togglePluginEnabled FAILED", e)
                appLog("E", TAG, "togglePluginEnabled FAILED: ${e.message}\n${e.stackTraceToString().take(500)}")
                withContext(Dispatchers.Main) { call.reject("togglePluginEnabled failed: ${e.message}") }
            }
        }
    }

    @PluginMethod
    fun uninstallPlugin(call: PluginCall) {
        val pluginId = call.getString("pluginId") ?: run {
            call.reject("pluginId is required")
            return
        }
        Log.d(TAG, "uninstallPlugin() called: pluginId=$pluginId")
        appLog("I", TAG, "uninstallPlugin: pluginId=$pluginId")

        if (!PluginManager.isInitialized) {
            call.reject("PluginManager not initialized")
            return
        }

        GlobalScope.launch(Dispatchers.IO) {
            try {
                PluginManager.installerManager.uninstallPlugin(pluginId)
                Log.i(TAG, "uninstallPlugin SUCCESS: $pluginId")
                appLog("I", TAG, "uninstallPlugin SUCCESS: $pluginId")
                val res = JSObject().apply {
                    put("success", true)
                    put("pluginId", pluginId)
                }
                withContext(Dispatchers.Main) { call.resolve(res) }
            } catch (e: Error) {
                val msg = "${e.javaClass.simpleName}: ${e.message}"
                Log.e(TAG, "uninstallPlugin Error: $msg", e)
                appLog("E", TAG, "uninstallPlugin ERROR: $msg")
                withContext(Dispatchers.Main) { call.reject("Uninstall error: $msg") }
            } catch (e: Exception) {
                val msg = e.message ?: "unknown error"
                Log.e(TAG, "uninstallPlugin FAILED: $msg", e)
                appLog("E", TAG, "uninstallPlugin FAILED: $msg\n${e.stackTraceToString().take(500)}")
                withContext(Dispatchers.Main) { call.reject("Uninstall failed: $msg") }
            }
        }
    }

    @PluginMethod
    fun debugLifecycleFlow(call: PluginCall) {
        val steps = mutableListOf<String>()
        val pluginId = call.getString("pluginId", "com.encvgo.plugin.mpv") ?: "com.encvgo.plugin.mpv"

        steps.add("=== Plugin Lifecycle Diagnostic ===")

        steps.add("1. PluginManager State:")
        steps.add("   isInitialized = ${PluginManager.isInitialized}")

        if (PluginManager.isInitialized) {
            try {
                val plugins = PluginManager.getAllInstallPlugins()
                steps.add("   installedPlugins(${plugins.size}) = ${plugins.joinToString { "${it.id}(v${it.versionName},enabled=${it.enabled})" }}")
                val target = plugins.find { it.id == pluginId }
                if (target != null) {
                    steps.add("   ✅ target '$pluginId' FOUND: v${target.versionName}, enabled=${target.enabled}")
                } else {
                    steps.add("   ❌ target '$pluginId' NOT found in installed list")
                }
            } catch (e: Exception) {
                steps.add("   getAllInstallPlugins FAILED: ${e.message}")
            }

            steps.add("")
            steps.add("2. ProxyManager State:")
            try {
                val pm = PluginManager.proxyManager
                steps.add("   proxyManager = $pm")
                steps.add("   hostActivity configured = ${pm != null}")
            } catch (e: Exception) {
                steps.add("   proxyManager check FAILED: ${e.message}")
            }

            steps.add("")
            steps.add("3. Activity Resolution Test:")
            try {
                val hostActivityClass = com.encvgo.app.EncvHostActivity::class.java
                steps.add("   EncvHostActivity.resolved = ✅ ($hostActivityClass)")
            } catch (e: Exception) {
                steps.add("   EncvHostActivity.resolved = ❌ (${e.message})")
            }
            try {
                val ctx = context ?: activity!!
                val pm = ctx.packageManager
                val hostInfo = pm.getActivityInfo(
                    android.content.ComponentName(ctx, com.encvgo.app.EncvHostActivity::class.java), 0
                )
                steps.add("   EncvHostActivity in Manifest = ✅ (name=${hostInfo.name})")
            } catch (e: Exception) {
                steps.add("   EncvHostActivity in Manifest = ❌ (${e.message})")
            }

            steps.add("")
            steps.add("4. setPluginEnabled test (dry run):")
            try {
                val info = PluginManager.getPluginInfo(pluginId)
                if (info != null) {
                    val infoStr = buildString {
                        append("id=${info.id}, versionName=")
                        append(info.versionName ?: "null")
                        try { append(", enabled=${info.enabled}") } catch (_: Exception) {}
                    }
                    steps.add("   getPluginInfo('$pluginId') = ✅ ($infoStr)")
                } else {
                    steps.add("   getPluginInfo('$pluginId') = ❌ null (not installed?)")
                }
            } catch (e: Exception) {
                steps.add("   getPluginInfo FAILED: ${e.javaClass.simpleName}: ${e.message}")
            } catch (e: Error) {
                steps.add("   getPluginInfo ERROR: ${e.javaClass.simpleName}: ${e.message}")
            }

            steps.add("")
            steps.add("5. uninstallPlugin test (dry run — will NOT execute):")
            steps.add("   installerManager.available = ${try { PluginManager.installerManager != null } catch (e: Exception) { "ERROR: ${e.message}" }}")
        }

        val result = JSObject()
        result.put("debugLog", steps.joinToString("\n"))
        call.resolve(result)
    }

    override fun handleOnActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.handleOnActivityResult(requestCode, resultCode, data)
        appLog("I", TAG, "handleOnActivityResult: requestCode=$requestCode, resultCode=$resultCode, hasData=${data != null}")

        when (requestCode) {
            REQUEST_CODE_PLUGIN_PICK -> handlePickResult(resultCode, data)
            REQUEST_CODE_INSTALL_CONFIRM -> handleInstallConfirmResult(resultCode, data)
        }
    }

    private fun handlePickResult(resultCode: Int, data: Intent?) {
        val call = pendingCalls.remove("pickPlugin") ?: run {
            appLog("W", TAG, "handlePickResult: no pending call for pickPlugin")
            return
        }
        if (resultCode != Activity.RESULT_OK || data?.data == null) {
            appLog("I", TAG, "handlePickResult: picker cancelled or no file selected")
            call.reject("File picker cancelled or no file selected")
            return
        }
        val uri = data.data!!
        appLog("I", TAG, "handlePickResult: picked URI=$uri")
        Log.d(TAG, "handleOnActivityResult: picked URI=$uri")
        try {
            val contentResolver = context.contentResolver
            val cursor = contentResolver.query(uri, null, null, null, null)
            var displayName = ""
            if (cursor != null && cursor.moveToFirst()) {
                val nameIndex = cursor.getColumnIndex("_display_name")
                if (nameIndex >= 0) displayName = cursor.getString(nameIndex)
                cursor.close()
            }
            if (displayName.isEmpty()) displayName = uri.lastPathSegment ?: "plugin.apk"
            val inputStream = contentResolver.openInputStream(uri)
            if (inputStream == null) {
                call.reject("Cannot read selected file")
                return
            }
            val tempDir = File(context.cacheDir, "plugin_install")
            tempDir.mkdirs()
            val tempApk = File(tempDir, displayName)
            tempApk.outputStream().use { output -> inputStream.copyTo(output) }
            inputStream.close()
            appLog("I", TAG, "handlePickResult: APK copied to ${tempApk.absolutePath} (${tempApk.length()} bytes)")
            startInstallConfirm(call, tempApk.absolutePath, displayName)
        } catch (e: Exception) {
            Log.e(TAG, "handlePickResult failed", e)
            appLog("E", TAG, "handlePickResult: FAILED ${e.message}")
            call.reject("Failed to process selected file: ${e.message}")
        }
    }

    private fun handleInstallConfirmResult(resultCode: Int, data: Intent?) {
        val call = pendingCalls.remove("installConfirm") ?: run {
            appLog("W", TAG, "handleInstallConfirmResult: no pending call for installConfirm")
            return
        }
        val apkPath = data?.getStringExtra(com.encvgo.app.InstallConfirmActivity.EXTRA_APK_PATH)
            ?: call.getString("apkPath") ?: ""
        val apkFile = File(apkPath)

        if (resultCode == Activity.RESULT_OK) {
            appLog("I", TAG, "handleInstallConfirmResult: user confirmed, installing ${apkFile.name}")
            Log.i(TAG, "Install confirmed via onActivityResult for: ${apkFile.name}")
            executeComboLiteInstall(call, apkFile)
        } else {
            appLog("I", TAG, "handleInstallConfirmResult: user cancelled")
            Log.i(TAG, "Install cancelled via onActivityResult")
            call.reject("用户取消安装")
        }
    }

    private fun startInstallConfirm(call: PluginCall, apkPath: String, name: String) {
        pendingCalls["installConfirm"] = call
        call.getData().put("apkPath", apkPath)
        call.save()
        appLog("I", TAG, "startInstallConfirm: apkPath=$apkPath, name=$name")
        try {
            val intent = Intent(activity, com.encvgo.app.InstallConfirmActivity::class.java).apply {
                putExtra(com.encvgo.app.InstallConfirmActivity.EXTRA_APK_PATH, apkPath)
                putExtra(com.encvgo.app.InstallConfirmActivity.EXTRA_FILE_NAME, name)
            }
            activity.startActivityForResult(intent, REQUEST_CODE_INSTALL_CONFIRM)
            appLog("I", TAG, "startInstallConfirm: startActivityForResult SUCCESS with requestCode=$REQUEST_CODE_INSTALL_CONFIRM")
        } catch (e: Exception) {
            appLog("E", TAG, "startInstallConfirm: startActivityForResult FAILED: ${e.javaClass.name}: ${e.message}")
            Log.e(TAG, "SATURATION-DEBUG startInstallConfirm: FAILED", e)
            pendingCalls.remove("installConfirm")
            call.reject("Failed to show install confirmation: ${e.message}")
        }
    }

    @PluginMethod
    fun debugInstallFlow(call: PluginCall) {
        val steps = mutableListOf<String>()

        steps.add("=== PluginManager State ===")
        val pmInit = PluginManager.isInitialized
        steps.add("1. PluginManager.isInitialized = $pmInit")
        if (pmInit) {
            try {
                val allPlugins = PluginManager.getAllInstallPlugins()
                steps.add("2. installedPlugins = ${allPlugins.map { "${it.id}(v${it.versionName},enabled=${it.enabled})" }}")
            } catch (e: Exception) {
                steps.add("2. getAllInstallPlugins FAILED: ${e.message}")
            }
            try {
                steps.add("3. validationStrategy = ${PluginManager.validationStrategy}")
            } catch (e: Exception) {
                steps.add("3. validationStrategy FAILED: ${e.javaClass.simpleName}: ${e.message}")
            }
        }

        steps.add("=== APK Files ===")
        val pluginInstallDir = File(context.cacheDir, "plugin_install")
        if (pluginInstallDir.exists()) {
            val apkFiles = pluginInstallDir.listFiles()?.filter { it.extension == "apk" }
            steps.add("4. APK files in cache = ${apkFiles?.map { "${it.name}(${it.length()}B)" }}")
        } else {
            steps.add("4. plugin_install dir not found")
        }

        steps.add("=== Actual installPlugin Test ===")
        if (!pmInit) {
            steps.add("5. SKIPPED: PluginManager not initialized")
        } else {
            val apkFiles = pluginInstallDir.listFiles()?.filter { it.extension == "apk" }
            val testApk = apkFiles?.firstOrNull()
            if (testApk == null) {
                steps.add("5. SKIPPED: no APK file found in plugin_install")
            } else {
                steps.add("5. testApk = ${testApk.name} (${testApk.length()}B)")
                steps.add("6. calling installPlugin(testApk, forceOverwrite=true)...")
                try {
                    val installResult = runBlocking(Dispatchers.IO) {
                        PluginManager.installerManager.installPlugin(testApk, true)
                    }
                    when (installResult) {
                        is com.combo.core.runtime.installer.InstallerManager.InstallResult.Success -> {
                            steps.add("7. installPlugin result = SUCCESS")
                            steps.add("   pluginId = ${installResult.pluginInfo.id}")
                            steps.add("   versionName = ${installResult.pluginInfo.versionName}")
                            steps.add("   entryClass = ${installResult.pluginInfo.entryClass}")
                        }
                        is com.combo.core.runtime.installer.InstallerManager.InstallResult.Failure -> {
                            steps.add("7. installPlugin result = FAILURE")
                            steps.add("   reason = ${installResult.reason}")
                            val excStack = installResult.exception?.stackTraceToString()?.take(800) ?: "(no exception)"
                            steps.add("   exception = $excStack")
                        }
                    }
                } catch (e: Error) {
                    steps.add("7. installPlugin threw Error: ${e.javaClass.simpleName}: ${e.message}")
                    steps.add("   stack = ${e.stackTraceToString().take(800)}")
                } catch (e: Exception) {
                    steps.add("7. installPlugin threw Exception: ${e.javaClass.simpleName}: ${e.message}")
                    steps.add("   stack = ${e.stackTraceToString().take(800)}")
                }
            }
        }

        steps.add("=== Post-Install State ===")
        if (pmInit) {
            try {
                val allPlugins = PluginManager.getAllInstallPlugins()
                steps.add("8. installedPlugins after = ${allPlugins.map { "${it.id}(enabled=${it.enabled})" }}")
            } catch (e: Exception) {
                steps.add("8. getAllInstallPlugins FAILED: ${e.message}")
            }
            val pluginsDir = File(context.filesDir, "plugins")
            steps.add("9. plugins dir exists = ${pluginsDir.exists()}")
            if (pluginsDir.exists()) {
                steps.add("   contents = ${pluginsDir.listFiles()?.map { it.name }}")
            }
        }

        Log.i(TAG, "SATURATION-DEBUG debugInstallFlow:\n${steps.joinToString("\n")}")
        val results = JSObject()
        results.put("debugLog", steps.joinToString("\n"))
        call.resolve(results)
    }

    @PluginMethod
    fun debugKotlinReflect(call: PluginCall) {
        val steps = mutableListOf<String>()

        steps.add("=== kotlin-reflect Health Check ===")

        steps.add("1. Testing ::function.javaMethod on PluginManager...")
        try {
            val method = PluginManager::setValidationStrategy.javaMethod
            steps.add("   setValidationStrategy.javaMethod = $method")
            steps.add("   declaringClass = ${method?.declaringClass?.name}")
            steps.add("   annotations = ${method?.annotations?.map { it.annotationClass.simpleName }}")
        } catch (e: Error) {
            steps.add("   FAILED with Error: ${e.javaClass.simpleName}: ${e.message}")
            steps.add("   stack = ${e.stackTraceToString().take(500)}")
        } catch (e: Exception) {
            steps.add("   FAILED with Exception: ${e.javaClass.simpleName}: ${e.message}")
        }

        try {
            val method = PluginManager::loadEnabledPlugins.javaMethod
            steps.add("   loadEnabledPlugins.javaMethod = $method")
        } catch (e: Error) {
            steps.add("   loadEnabledPlugins FAILED: ${e.javaClass.simpleName}: ${e.message}")
        } catch (e: Exception) {
            steps.add("   loadEnabledPlugins FAILED: ${e.javaClass.simpleName}: ${e.message}")
        }

        try {
            val method = PluginManager::launchPlugin.javaMethod
            steps.add("   launchPlugin.javaMethod = $method")
        } catch (e: Error) {
            steps.add("   launchPlugin FAILED: ${e.javaClass.simpleName}: ${e.message}")
        } catch (e: Exception) {
            steps.add("   launchPlugin FAILED: ${e.javaClass.simpleName}: ${e.message}")
        }

        steps.add("2. Testing ::function.javaMethod on InstallerManager...")
        try {
            if (PluginManager.isInitialized) {
                val im = PluginManager.installerManager
                val method = im::installPlugin.javaMethod
                steps.add("   installPlugin.javaMethod = $method")
                steps.add("   declaringClass = ${method?.declaringClass?.name}")
            } else {
                steps.add("   SKIPPED: PluginManager not initialized")
            }
        } catch (e: Error) {
            steps.add("   FAILED with Error: ${e.javaClass.simpleName}: ${e.message}")
            steps.add("   stack = ${e.stackTraceToString().take(500)}")
        } catch (e: Exception) {
            steps.add("   FAILED with Exception: ${e.javaClass.simpleName}: ${e.message}")
        }

        steps.add("3. Testing ::function.javaMethod on PluginCrashHandler...")
        try {
            val method = com.combo.core.security.crash.PluginCrashHandler::setGlobalClashCallback.javaMethod
            steps.add("   setGlobalClashCallback.javaMethod = $method")
        } catch (e: Error) {
            steps.add("   FAILED with Error: ${e.javaClass.simpleName}: ${e.message}")
            steps.add("   stack = ${e.stackTraceToString().take(500)}")
        } catch (e: Exception) {
            steps.add("   FAILED with Exception: ${e.javaClass.simpleName}: ${e.message}")
        }

        steps.add("4. Testing ::function.javaMethod on AuthorizationManager...")
        try {
            if (PluginManager.isInitialized) {
                val am = PluginManager.authorizationManager
                val method = am::setAuthorizationHandler.javaMethod
                steps.add("   setAuthorizationHandler.javaMethod = $method")
            } else {
                steps.add("   SKIPPED: PluginManager not initialized")
            }
        } catch (e: Error) {
            steps.add("   FAILED with Error: ${e.javaClass.simpleName}: ${e.message}")
            steps.add("   stack = ${e.stackTraceToString().take(500)}")
        } catch (e: Exception) {
            steps.add("   FAILED with Exception: ${e.javaClass.simpleName}: ${e.message}")
        }

        steps.add("5. Checking @Metadata on ComboLite classes...")
        try {
            val pmClass = PluginManager::class.java
            val meta = pmClass.getAnnotation(kotlin.Metadata::class.java)
            steps.add("   PluginManager @Metadata = ${meta != null}")
            if (meta != null) {
                steps.add("   mv = ${meta.metadataVersion.toList()}")
                steps.add("   k = ${meta.kind}")
            }
        } catch (e: Exception) {
            steps.add("   @Metadata check FAILED: ${e.message}")
        }
        try {
            val imClass = Class.forName("com.combo.core.runtime.installer.InstallerManager")
            val meta = imClass.getAnnotation(kotlin.Metadata::class.java)
            steps.add("   InstallerManager @Metadata = ${meta != null}")
            if (meta != null) {
                steps.add("   mv = ${meta.metadataVersion.toList()}")
                steps.add("   k = ${meta.kind}")
            }
        } catch (e: Exception) {
            steps.add("   InstallerManager @Metadata check FAILED: ${e.message}")
        }

        steps.add("6. Checking R8 mapping on ComboLite classes...")
        try {
            val pmMethods = PluginManager::class.java.declaredMethods.map { "${it.name}(${it.parameterTypes.map { t -> t.simpleName }})" }
            steps.add("   PluginManager methods (first 10) = ${pmMethods.take(10)}")
        } catch (e: Exception) {
            steps.add("   PluginManager method list FAILED: ${e.message}")
        }
        try {
            val imClass = Class.forName("com.combo.core.runtime.installer.InstallerManager")
            val imMethods = imClass.declaredMethods.map { "${it.name}(${it.parameterTypes.map { t -> t.simpleName }})" }
            steps.add("   InstallerManager methods (first 10) = ${imMethods.take(10)}")
        } catch (e: Exception) {
            steps.add("   InstallerManager method list FAILED: ${e.message}")
        }

        Log.i(TAG, "SATURATION-DEBUG debugKotlinReflect:\n${steps.joinToString("\n")}")
        val results = JSObject()
        results.put("debugLog", steps.joinToString("\n"))
        call.resolve(results)
    }

    @PluginMethod
    fun debugApkValidation(call: PluginCall) {
        val steps = mutableListOf<String>()

        steps.add("=== APK Validation ===")
        val pluginInstallDir = File(context.cacheDir, "plugin_install")
        val apkFiles = pluginInstallDir.listFiles()?.filter { it.extension == "apk" }
        val testApk = apkFiles?.firstOrNull()

        if (testApk == null) {
            steps.add("1. No APK file found in plugin_install")
            val results = JSObject()
            results.put("debugLog", steps.joinToString("\n"))
            call.resolve(results)
            return
        }

        steps.add("1. APK file = ${testApk.name} (${testApk.length()}B)")
        steps.add("   exists = ${testApk.exists()}")
        steps.add("   canRead = ${testApk.canRead()}")

        steps.add("2. PackageManager.getPackageArchiveInfo...")
        try {
            val pkgInfo = context.packageManager.getPackageArchiveInfo(
                testApk.absolutePath,
                PackageManager.GET_META_DATA or PackageManager.GET_SIGNATURES or PackageManager.GET_ACTIVITIES
            )
            if (pkgInfo == null) {
                steps.add("   FAILED: getPackageArchiveInfo returned null (invalid APK?)")
            } else {
                steps.add("   packageName = ${pkgInfo.packageName}")
                steps.add("   versionCode = ${if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) pkgInfo.longVersionCode else pkgInfo.versionCode}")
                steps.add("   versionName = ${pkgInfo.versionName}")

                val appInfo = pkgInfo.applicationInfo
                if (appInfo != null) {
                    appInfo.publicSourceDir = testApk.absolutePath
                    val label = context.packageManager.getApplicationLabel(appInfo)
                    steps.add("   appLabel = $label")
                    steps.add("   iconResId = ${appInfo.icon}")

                    val metaData = appInfo.metaData
                    if (metaData != null) {
                        steps.add("   metaData keys = ${metaData.keySet().toList()}")
                        val entryClass = metaData.getString("plugin.entryClass")
                        steps.add("   plugin.entryClass = $entryClass")
                        val desc = metaData.getString("plugin.description")
                        steps.add("   plugin.description = $desc")
                    } else {
                        steps.add("   metaData = NULL ← CRITICAL: plugin must have meta-data")
                    }
                }

                val activities = pkgInfo.activities
                steps.add("   activities = ${activities?.map { it.name } ?: "none"}")

                val signatures = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                    val pkgInfoSign = context.packageManager.getPackageArchiveInfo(
                        testApk.absolutePath,
                        PackageManager.GET_SIGNING_CERTIFICATES
                    )
                    pkgInfoSign?.signingInfo?.apkContentsSigners?.map { it.toCharsString().take(16) + "..." } ?: listOf("(none)")
                } else {
                    @Suppress("DEPRECATION")
                    pkgInfo.signatures?.map { it.toCharsString().take(16) + "..." } ?: listOf("(none)")
                }
                steps.add("   signatures = $signatures")
            }
        } catch (e: Exception) {
            steps.add("   FAILED: ${e.javaClass.simpleName}: ${e.message}")
        }

        steps.add("3. Host app signatures...")
        try {
            val hostPkgInfo = context.packageManager.getPackageInfo(
                context.packageName,
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) PackageManager.GET_SIGNING_CERTIFICATES else PackageManager.GET_SIGNATURES
            )
            val hostSigs = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                @Suppress("DEPRECATION")
                hostPkgInfo.signingInfo?.apkContentsSigners?.map { it.toCharsString().take(16) + "..." }
                    ?: @Suppress("DEPRECATION") hostPkgInfo.signatures?.map { it.toCharsString().take(16) + "..." }
                    ?: listOf("(none)")
            } else {
                @Suppress("DEPRECATION")
                hostPkgInfo.signatures?.map { it.toCharsString().take(16) + "..." } ?: listOf("(none)")
            }
            steps.add("   host signatures = $hostSigs")
        } catch (e: Exception) {
            steps.add("   FAILED: ${e.message}")
        }

        steps.add("4. APK ZIP integrity...")
        try {
            val zipFile = java.util.zip.ZipFile(testApk)
            val entries = zipFile.entries().toList().map { it.name }
            steps.add("   entry count = ${entries.size}")
            steps.add("   has AndroidManifest.xml = ${entries.contains("AndroidManifest.xml")}")
            steps.add("   has classes.dex = ${entries.any { it.startsWith("classes") && it.endsWith(".dex") }}")
            steps.add("   has resources.arsc = ${entries.contains("resources.arsc")}")
            val soFiles = entries.filter { it.endsWith(".so") }
            steps.add("   .so files = $soFiles")
            zipFile.close()
        } catch (e: Exception) {
            steps.add("   FAILED: ${e.message}")
        }

        Log.i(TAG, "SATURATION-DEBUG debugApkValidation:\n${steps.joinToString("\n")}")
        val results = JSObject()
        results.put("debugLog", steps.joinToString("\n"))
        call.resolve(results)
    }

    @PluginMethod
    fun debugValidationStrategy(call: PluginCall) {
        val steps = mutableListOf<String>()

        steps.add("=== ValidationStrategy State ===")

        steps.add("1. PluginManager.isInitialized = ${PluginManager.isInitialized}")
        if (!PluginManager.isInitialized) {
            steps.add("2. SKIPPED: PluginManager not initialized")
        } else {
            try {
                val currentStrategy = PluginManager.validationStrategy
                steps.add("2. current validationStrategy = $currentStrategy")
            } catch (e: Exception) {
                steps.add("2. reading validationStrategy FAILED: ${e.javaClass.simpleName}: ${e.message}")
            }

            steps.add("3. Testing setValidationStrategy(Insecure)...")
            try {
                runBlocking(Dispatchers.IO) {
                    PluginManager.setValidationStrategy(com.combo.core.runtime.ValidationStrategy.Insecure)
                }
                steps.add("   SUCCESS: no error thrown")
                val afterStrategy = PluginManager.validationStrategy
                steps.add("   validationStrategy after = $afterStrategy")
                if (afterStrategy != com.combo.core.runtime.ValidationStrategy.Insecure) {
                    steps.add("   ← PROBLEM: strategy not actually changed! setValidationStrategy silently failed")
                }
            } catch (e: Error) {
                steps.add("   FAILED with Error: ${e.javaClass.simpleName}: ${e.message}")
                steps.add("   stack = ${e.stackTraceToString().take(500)}")
            } catch (e: Exception) {
                steps.add("   FAILED with Exception: ${e.javaClass.simpleName}: ${e.message}")
            }

            steps.add("4. Testing loadEnabledPlugins.javaMethod...")
            try {
                val method = PluginManager::loadEnabledPlugins.javaMethod
                steps.add("   loadEnabledPlugins.javaMethod = $method")
                if (method != null) {
                    val hasAnnotation = method.isAnnotationPresent(com.combo.core.security.permission.RequiresPermission::class.java)
                    steps.add("   has @RequiresPermission = $hasAnnotation")
                    if (hasAnnotation) {
                        val ann = method.getAnnotation(com.combo.core.security.permission.RequiresPermission::class.java)
                        steps.add("   permissionLevel = ${ann?.level}")
                    }
                }
            } catch (e: Error) {
                steps.add("   FAILED with Error: ${e.javaClass.simpleName}: ${e.message}")
            } catch (e: Exception) {
                steps.add("   FAILED with Exception: ${e.javaClass.simpleName}: ${e.message}")
            }

            steps.add("5. Testing loadEnabledPlugins() call...")
            try {
                val count = runBlocking(Dispatchers.IO) {
                    PluginManager.loadEnabledPlugins()
                }
                steps.add("   loadEnabledPlugins() returned $count")
            } catch (e: Error) {
                steps.add("   FAILED with Error: ${e.javaClass.simpleName}: ${e.message}")
                steps.add("   stack = ${e.stackTraceToString().take(500)}")
            } catch (e: Exception) {
                steps.add("   FAILED with Exception: ${e.javaClass.simpleName}: ${e.message}")
            }
        }

        steps.add("6. EncvApplication onFrameworkSetup log...")
        try {
            val logFile = File(context.filesDir, "encv.log")
            if (logFile.exists()) {
                val lines = logFile.readLines().filter { it.contains("onFrameworkSetup") || it.contains("setValidationStrategy") || it.contains("ValidationStrategy") }
                steps.add("   relevant log lines = ${lines.take(10)}")
            } else {
                steps.add("   encv.log not found")
            }
        } catch (e: Exception) {
            steps.add("   FAILED: ${e.message}")
        }

        Log.i(TAG, "SATURATION-DEBUG debugValidationStrategy:\n${steps.joinToString("\n")}")
        val results = JSObject()
        results.put("debugLog", steps.joinToString("\n"))
        call.resolve(results)
    }

    @PluginMethod
    fun getLocalFilePath(call: PluginCall) {
        val path = call.getString("path", "")
        val result = JSObject()
        try {
            if (path.isNullOrEmpty()) {
                result.put("path", "")
                call.resolve(result)
                return
            }
            val file = File(path)
            if (file.exists() && file.isFile && file.canRead()) {
                result.put("path", file.absolutePath)
            } else {
                val resolved = File(context.filesDir, path!!.removePrefix("/"))
                if (resolved.exists() && resolved.isFile && resolved.canRead()) {
                    result.put("path", resolved.absolutePath)
                } else {
                    result.put("path", "")
                }
            }
            call.resolve(result)
        } catch (e: Exception) {
            Log.e(TAG, "getLocalFilePath failed", e)
            result.put("path", "")
            call.resolve(result)
        }
    }

    private fun executeComboLiteInstall(call: PluginCall, apkFile: File) {
        val apkPath = apkFile.absolutePath
        appLog("I", TAG, "executeComboLiteInstall: starting for ${apkFile.name} (${apkFile.length()} bytes)")
        GlobalScope.launch(Dispatchers.IO) {
            try {
                val result = PluginManager.installerManager.installPlugin(apkFile, true)
                when (result) {
                    is com.combo.core.runtime.installer.InstallerManager.InstallResult.Success -> {
                        Log.i(TAG, "Plugin installed via ComboLite: $apkPath -> ${result.pluginInfo.id}")
                        appLog("I", TAG, "ComboLite install SUCCESS: ${result.pluginInfo.id}")
                        call.resolve(JSObject().apply {
                            put("success", true)
                            put("method", "combolite")
                            put("pluginId", result.pluginInfo.id)
                        })
                    }
                    is com.combo.core.runtime.installer.InstallerManager.InstallResult.Failure -> {
                        val reason = result.reason
                        val excDetail = result.exception?.stackTraceToString()?.take(500) ?: ""
                        Log.e(TAG, "ComboLite install failed: $reason", result.exception)
                        appLog("E", TAG, "ComboLite install FAILED: $reason\n$excDetail")
                        call.reject("ComboLite install failed: $reason")
                    }
                }
            } catch (e: Error) {
                Log.e(TAG, "ComboLite installPlugin Error: ${e.javaClass.simpleName}: ${e.message}", e)
                appLog("E", TAG, "ComboLite install ERROR: ${e.javaClass.simpleName}: ${e.message}\n${e.stackTraceToString().take(500)}")
                call.reject("ComboLite install error: ${e.javaClass.simpleName}: ${e.message}")
            } catch (e: Exception) {
                Log.e(TAG, "ComboLite installPlugin exception", e)
                appLog("E", TAG, "ComboLite install EXCEPTION: ${e.message}\n${e.stackTraceToString().take(500)}")
                call.reject("ComboLite install error: ${e.message}")
            }
        }
    }

    @PluginMethod
    fun exportLogs(call: PluginCall) {
        try {
            val logDir = File(context.cacheDir, "encv_logs_export")
            logDir.mkdirs()
            val timestamp = java.text.SimpleDateFormat("yyyyMMdd_HHmmss", java.util.Locale.US).format(java.util.Date())

            val appLogFile = File(logDir, "app_log_${timestamp}.txt")
            val appLogs = getAppLogs()
            appLogFile.writeText(if (appLogs.isNotEmpty()) appLogs else "(no app log entries)")

            val logcatFile = File(logDir, "logcat_${timestamp}.txt")
            try {
                val pid = android.os.Process.myPid()
                val process = Runtime.getRuntime().exec(arrayOf("logcat", "-d", "--pid=$pid", "-t", "5000", "-v", "threadtime"))
                process.inputStream.bufferedReader().use { reader ->
                    logcatFile.outputStream().bufferedWriter().use { writer ->
                        var line: String?
                        while (reader.readLine().also { line = it } != null) {
                            writer.write(line)
                            writer.newLine()
                        }
                    }
                }
            } catch (e: Exception) {
                Log.w(TAG, "exportLogs: logcat exec failed", e)
            }
            if (!logcatFile.exists() || logcatFile.length() == 0L) {
                logcatFile.writeText("(logcat empty — app may lack READ_LOGS permission on Android 14+, use app_log instead)\n")
            }

            val goBackendLogFile = File(logDir, "go_backend_${timestamp}.txt")
            val goOutput = EncvGoService.getOutputSnapshot()
            goBackendLogFile.writeText(if (goOutput.isNotEmpty()) goOutput else "(Go backend not running or no output)")

            val zipFile = File(context.cacheDir, "encv_logs_${timestamp}.zip")
            java.util.zip.ZipOutputStream(zipFile.outputStream()).use { zos ->
                fun addToZip(file: File, entryName: String) {
                    if (!file.exists()) return
                    try {
                        zos.putNextEntry(java.util.zip.ZipEntry(entryName))
                        file.inputStream().use { it.copyTo(zos) }
                        zos.closeEntry()
                    } catch (e: Exception) {
                        Log.w(TAG, "exportLogs: failed to add $entryName", e)
                    }
                }
                addToZip(appLogFile, "app_log_${timestamp}.txt")
                addToZip(logcatFile, "logcat_${timestamp}.txt")
                addToZip(goBackendLogFile, "go_backend/go_backend_${timestamp}.txt")

                val devLogsJson = File(context.cacheDir, "devlogs_export.json")
                if (devLogsJson.exists()) addToZip(devLogsJson, "frontend/devlogs.json")

                val infoFile = File(logDir, "device_info_${timestamp}.txt")
                infoFile.writeText(buildString {
                    appendLine("Device: ${android.os.Build.MANUFACTURER} ${android.os.Build.MODEL}")
                    appendLine("Android: ${android.os.Build.VERSION.RELEASE} (API ${android.os.Build.VERSION.SDK_INT})")
                    appendLine("App: ${context.packageName}")
                    appendLine("PluginManager.isInitialized: ${PluginManager.isInitialized}")
                    appendLine("GoBackend running: ${EncvGoService.isRunning}")
                    appendLine("GoBackend port: ${EncvGoService.lastKnownPort}")
                    appendLine("Timestamp: $timestamp")
                    appendLine("AppLog size: ${appLogFile.length()} bytes")
                    appendLine("Logcat size: ${logcatFile.length()} bytes")
                    appendLine("GoBackendLog size: ${goBackendLogFile.length()} bytes")
                })
                addToZip(infoFile, "device_info_${timestamp}.txt")
            }

            appLogFile.delete()
            logcatFile.delete()
            goBackendLogFile.delete()

            val uri = androidx.core.content.FileProvider.getUriForFile(
                context, "${context.packageName}.fileprovider", zipFile)
            val shareIntent = Intent(Intent.ACTION_SEND).apply {
                type = "application/zip"
                putExtra(Intent.EXTRA_STREAM, uri)
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
                putExtra(Intent.EXTRA_SUBJECT, "ENCV Logs ${timestamp}")
            }
            val chooser = Intent.createChooser(shareIntent, "导出日志")
            if (context !is Activity) {
                chooser.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            context.startActivity(chooser)

            call.resolve(JSObject().put("success", true).put("path", zipFile.absolutePath))
        } catch (e: Exception) {
            Log.e(TAG, "exportLogs failed", e)
            call.reject("Failed to export logs: ${e.message}")
        }
    }

    @PluginMethod
    fun clearLogs(call: PluginCall) {
        try {
            clearAppLogs()
            try {
                Runtime.getRuntime().exec(arrayOf("logcat", "-c"))
            } catch (_: Exception) {}
            EncvGoService.clearOutputSnapshot()

            val exportDir = File(context.cacheDir, "encv_logs_export")
            if (exportDir.exists()) {
                exportDir.listFiles()?.forEach { it.delete() }
            }

            call.resolve(JSObject().put("success", true))
        } catch (e: Exception) {
            Log.e(TAG, "clearLogs failed", e)
            call.reject("Failed to clear logs: ${e.message}")
        }
    }

    @PluginMethod
    fun openLogViewer(call: PluginCall) {
        try {
            val logFile = File(context.cacheDir, "encv_logs_export/app_log_latest.txt")
            logFile.parentFile?.mkdirs()
            val appLogs = getAppLogs()
            logFile.writeText(if (appLogs.isNotEmpty()) appLogs else "(no app log entries)")

            val uri = androidx.core.content.FileProvider.getUriForFile(
                context, "${context.packageName}.fileprovider", logFile)
            val viewIntent = Intent(Intent.ACTION_VIEW).apply {
                setDataAndType(uri, "text/plain")
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
                if (context !is Activity) {
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                }
            }
            context.startActivity(viewIntent)
            call.resolve(JSObject().put("success", true))
        } catch (e: Exception) {
            Log.e(TAG, "openLogViewer failed", e)
            call.reject("Failed to open log viewer: ${e.message}")
        }
    }

    @PluginMethod
    fun saveDevLogs(call: PluginCall) {
        try {
            val logsJson = call.getString("logs") ?: run {
                call.reject("logs parameter required")
                return
            }
            val devLogsFile = File(context.cacheDir, "devlogs_export.json")
            devLogsFile.writeText(logsJson)
            call.resolve(JSObject().put("success", true).put("path", devLogsFile.absolutePath))
        } catch (e: Exception) {
            Log.e(TAG, "saveDevLogs failed", e)
            call.reject("Failed to save dev logs: ${e.message}")
        }
    }
}
