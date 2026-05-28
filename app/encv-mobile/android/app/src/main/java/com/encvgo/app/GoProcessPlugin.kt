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
import com.combo.core.runtime.PluginManager
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.launch

@CapacitorPlugin(
    name = "GoProcess"
)
class GoProcessPlugin : Plugin() {

    companion object {
        private const val TAG = "ENCV-go"
        const val REQUEST_CODE_PLUGIN_PICK = 9001
        const val REQUEST_CODE_INSTALL_CONFIRM = 9002
    }

    private val pendingCalls = ConcurrentHashMap<String, PluginCall>()
    private var receiverRegistered = false

    private val installConfirmReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            Log.i(TAG, "SATURATION-DEBUG installConfirmReceiver.onReceive: action=${intent?.action}, extras=${intent?.extras}")
            if (intent == null) return
            val requestId = intent.getStringExtra("request_id") ?: ""
            val resultCode = intent.getIntExtra("result_code", Activity.RESULT_CANCELED)
            Log.i(TAG, "SATURATION-DEBUG installConfirmReceiver: requestId=$requestId, resultCode=$resultCode")

            val callKey = when {
                requestId == "installConfirm" -> "installConfirm"
                else -> return
            }

            val call = pendingCalls.remove(callKey) ?: run {
                Log.w(TAG, "No pending call for broadcast result: $callKey")
                return
            }

            val apkPath = call.getString("apkPath") ?: ""
            val apkFile = java.io.File(apkPath)

            if (resultCode == Activity.RESULT_OK) {
                Log.i(TAG, "Install confirmed via broadcast for: ${apkFile.name}")
                executeComboLiteInstall(call, apkFile)
            } else {
                Log.i(TAG, "Install cancelled via broadcast")
                call.reject("用户取消安装")
            }
        }
    }

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
        registerInstallConfirmReceiver()
    }

    override fun handleOnDestroy() {
        if (receiverRegistered) {
            context.unregisterReceiver(statusReceiver)
            try { context.unregisterReceiver(installConfirmReceiver) } catch (_: Exception) {}
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
        val filePath = call.getString("filePath", "")
        val name = call.getString("name", "")
        val mimeType = call.getString("mimeType", "")
        try {
            Log.d(TAG, "openPlayer: filePath=$filePath, name=$name, mimeType=$mimeType")
            PlayerEntry.play(context ?: activity!!, filePath!!, name!!, mimeType!!)
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
        if (path.isNullOrEmpty()) {
            Log.w(TAG, "openInPlayer rejected: path is empty")
            call.reject("path is required")
            return
        }
        try {
            Log.d(TAG, "openInPlayer: path=$path, name=$name, mimeType=$mimeType")
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

    private fun registerInstallConfirmReceiver() {
        val installResultFilter = IntentFilter("com.encvgo.app.INSTALL_RESULT")
        try {
            if (Build.VERSION.SDK_INT >= 33) {
                context.registerReceiver(installConfirmReceiver, installResultFilter, Context.RECEIVER_NOT_EXPORTED)
            } else {
                @Suppress("DEPRECATION")
                context.registerReceiver(installConfirmReceiver, installResultFilter)
            }
            Log.i(TAG, "SATURATION-DEBUG registerInstallConfirmReceiver: SUCCESS, action=com.encvgo.app.INSTALL_RESULT")
        } catch (e: Exception) {
            Log.e(TAG, "SATURATION-DEBUG registerInstallConfirmReceiver: FAILED", e)
        }
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
            if (PluginManager.isInitialized) {
                pendingCalls["installConfirm"] = call
                Log.i(TAG, "SATURATION-DEBUG installPlugin: context type=${context.javaClass.simpleName}, isActivity=${context is Activity}, apkPath=$apkPath")
                try {
                    val intent = Intent(context, com.encvgo.app.InstallConfirmActivity::class.java).apply {
                        putExtra(com.encvgo.app.InstallConfirmActivity.EXTRA_APK_PATH, apkPath)
                        putExtra(com.encvgo.app.InstallConfirmActivity.EXTRA_FILE_NAME, apkFile.name)
                        putExtra("request_id", "installConfirm")
                        if (context !is Activity) {
                            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                        }
                    }
                    Log.i(TAG, "SATURATION-DEBUG installPlugin: starting InstallConfirmActivity, flags=${intent.flags}, extras=${intent.extras}")
                    context.startActivity(intent)
                    Log.i(TAG, "SATURATION-DEBUG installPlugin: startActivity succeeded")
                } catch (e: Exception) {
                    Log.e(TAG, "SATURATION-DEBUG installPlugin: startActivity FAILED", e)
                    pendingCalls.remove("installConfirm")
                    call.reject("Failed to show install confirmation: ${e.message}")
                }
                return
            }
            val uri = androidx.core.content.FileProvider.getUriForFile(
                context,
                "${context.packageName}.fileprovider",
                apkFile
            )
            val intent = android.content.Intent(android.content.Intent.ACTION_INSTALL_PACKAGE).apply {
                setDataAndType(uri, "application/vnd.android.package-archive")
                addFlags(android.content.Intent.FLAG_GRANT_READ_URI_PERMISSION)
                if (context !is android.app.Activity) {
                    addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK)
                }
            }
            context.startActivity(intent)
            Log.i(TAG, "Install intent fired for: $apkPath")
            call.resolve(JSObject().put("success", true).put("method", "intent"))
        } catch (e: Exception) {
            Log.e(TAG, "installPlugin failed", e)
            call.reject("Failed to install plugin: ${e.message}")
        }
    }

    @PluginMethod
    fun pickAndInstallPlugin(call: PluginCall) {
        Log.d(TAG, "pickAndInstallPlugin() called")
        pendingCalls["pickPlugin"] = call
        try {
            val intent = Intent(Intent.ACTION_GET_CONTENT).apply {
                type = "application/vnd.android.package-archive"
                addCategory(Intent.CATEGORY_OPENABLE)
                putExtra(Intent.EXTRA_MIME_TYPES, arrayOf("application/vnd.android.package-archive"))
            }
            activity.startActivityForResult(intent, REQUEST_CODE_PLUGIN_PICK)
        } catch (e: Exception) {
            Log.e(TAG, "pickAndInstallPlugin failed", e)
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
            fallbackCheckInstalled(result)
            call.resolve(result)
        } catch (e: Exception) {
            Log.e(TAG, "checkInstalledPlugins failed", e)
            call.reject("Failed to check installed plugins: ${e.message}")
        }
    }

    private fun fallbackCheckInstalled(result: JSObject) {
        val pluginDirs = listOf(
            File(context.applicationInfo.dataDir, "app_plugins"),
            File(context.filesDir, "assets/plugins"),
            File(context.cacheDir, "plugin_install")
        )
        for (dir in pluginDirs) {
            if (dir.exists() && dir.isDirectory) {
                dir.listFiles()
                    ?.filter { it.extension == "apk" && it.isFile }
                    ?.forEach { apk ->
                        result.put(apk.nameWithoutExtension.replace(Regex("[^a-z0-9_-]"), "-"), true)
                    }
            }
        }
        Log.i(TAG, "fallbackCheckInstalled: $result")
    }

    override fun handleOnActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.handleOnActivityResult(requestCode, resultCode, data)

        if (requestCode != REQUEST_CODE_PLUGIN_PICK) return
        val call = pendingCalls.remove("pickPlugin") ?: return
        if (resultCode != Activity.RESULT_OK || data?.data == null) {
            call.reject("File picker cancelled or no file selected")
            return
        }
        val uri = data.data!!
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
            installFromPath(call, tempApk.absolutePath, displayName)
        } catch (e: Exception) {
            Log.e(TAG, "handleOnActivityResult failed", e)
            call.reject("Failed to process selected file: ${e.message}")
        }
    }

    private fun installFromPath(call: PluginCall, apkPath: String, name: String) {
        try {
            val apkFile = File(apkPath)
            if (!apkFile.exists()) {
                call.reject("APK file not found after copy: $apkPath")
                return
            }
            if (PluginManager.isInitialized) {
                pendingCalls["installConfirm"] = call
                Log.i(TAG, "SATURATION-DEBUG installFromPath: context type=${context.javaClass.simpleName}, isActivity=${context is Activity}, apkPath=$apkPath")
                try {
                    val intent = Intent(context, com.encvgo.app.InstallConfirmActivity::class.java).apply {
                        putExtra(com.encvgo.app.InstallConfirmActivity.EXTRA_APK_PATH, apkPath)
                        putExtra(com.encvgo.app.InstallConfirmActivity.EXTRA_FILE_NAME, name)
                        putExtra("request_id", "installConfirm")
                        if (context !is Activity) {
                            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                        }
                    }
                    Log.i(TAG, "SATURATION-DEBUG installFromPath: starting InstallConfirmActivity, flags=${intent.flags}")
                    context.startActivity(intent)
                    Log.i(TAG, "SATURATION-DEBUG installFromPath: startActivity succeeded")
                } catch (e: Exception) {
                    Log.e(TAG, "SATURATION-DEBUG installFromPath: startActivity FAILED", e)
                    pendingCalls.remove("installConfirm")
                    call.reject("Failed to show install confirmation: ${e.message}")
                }
                return
            }
            val providerUri = androidx.core.content.FileProvider.getUriForFile(
                context,
                "${context.packageName}.fileprovider",
                apkFile
            )
            val intent = Intent(Intent.ACTION_INSTALL_PACKAGE).apply {
                setDataAndType(providerUri, "application/vnd.android.package-archive")
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
                if (context !is Activity) {
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                }
            }
            context.startActivity(intent)
            Log.i(TAG, "System installer launched (async), user must complete installation manually: $name")
            call.resolve(JSObject().put("success", true).put("method", "system").put("fileName", name).put("pending", true))
        } catch (e: Exception) {
            Log.e(TAG, "installFromPath failed", e)
            call.reject("Failed to install plugin: ${e.message}")
        }
    }

    @PluginMethod
    fun debugInstallFlow(call: PluginCall) {
        val results = JSObject()
        val steps = mutableListOf<String>()

        // === 1. PluginManager 状态 ===
        steps.add("=== PluginManager ===")
        val pmInit = PluginManager.isInitialized
        steps.add("1. PluginManager.isInitialized = $pmInit")
        results.put("pmInitialized", pmInit)

        if (pmInit) {
            try {
                val allPlugins = PluginManager.getAllInstallPlugins()
                val pluginIds = allPlugins.map { it.id }
                steps.add("2. installedPlugins = $pluginIds")
                results.put("installedPlugins", pluginIds.toString())
            } catch (e: Exception) {
                steps.add("2. getAllInstallPlugins FAILED: ${e.message}")
                results.put("installedPlugins", "ERROR: ${e.message}")
            }
            try {
                val proxy = PluginManager.proxyManager
                steps.add("3. proxyManager = ${proxy.javaClass.simpleName}")
                results.put("proxyManagerClass", proxy.javaClass.simpleName)
            } catch (e: Exception) {
                steps.add("3. proxyManager FAILED: ${e.message}")
                results.put("proxyManagerClass", "ERROR: ${e.message}")
            }
        } else {
            steps.add("2. installedPlugins = SKIPPED (PM not init)")
            steps.add("3. proxyManager = SKIPPED (PM not init)")
        }

        // === 4. Context 诊断 ===
        steps.add("=== Context ===")
        steps.add("4. context type = ${context.javaClass.name}")
        results.put("contextType", context.javaClass.name)
        steps.add("5. context is Activity = ${context is Activity}")
        results.put("contextIsActivity", context is Activity)
        steps.add("6. activity type = ${activity.javaClass.name}")
        results.put("activityType", activity.javaClass.name)
        steps.add("7. applicationContext = ${context.applicationContext.javaClass.name}")
        results.put("appContextType", context.applicationContext.javaClass.name)

        // === 8. BroadcastReceiver 状态 ===
        steps.add("=== BroadcastReceiver ===")
        steps.add("8. receiverRegistered = $receiverRegistered")
        results.put("receiverRegistered", receiverRegistered)
        val pendingKeys = pendingCalls.keys().toList()
        steps.add("9. pendingCalls = $pendingKeys")
        results.put("pendingCallsKeys", pendingKeys.toString())

        // === 10. InstallConfirmActivity 检查 ===
        steps.add("=== Activity Resolve ===")
        try {
            val resolveIntent = Intent(context, com.encvgo.app.InstallConfirmActivity::class.java)
            val resolveInfo = context.packageManager.resolveActivity(resolveIntent, 0)
            if (resolveInfo != null) {
                steps.add("10. InstallConfirmActivity resolved: ${resolveInfo.activityInfo?.name}")
                results.put("confirmActivityResolved", true)
                results.put("confirmActivityClass", resolveInfo.activityInfo?.name ?: "null")
            } else {
                steps.add("10. InstallConfirmActivity NOT RESOLVED (resolveActivity=null)")
                results.put("confirmActivityResolved", false)
            }
        } catch (e: Exception) {
            steps.add("10. resolveActivity FAILED: ${e.message}")
            results.put("confirmActivityResolved", false)
            results.put("confirmActivityError", e.message)
        }

        // === 11. EncvHostActivity 检查 ===
        try {
            val hostIntent = Intent(context, com.encvgo.app.EncvHostActivity::class.java)
            val hostResolve = context.packageManager.resolveActivity(hostIntent, 0)
            if (hostResolve != null) {
                steps.add("11. EncvHostActivity resolved: ${hostResolve.activityInfo?.name}")
                results.put("hostActivityResolved", true)
            } else {
                steps.add("11. EncvHostActivity NOT RESOLVED")
                results.put("hostActivityResolved", false)
            }
        } catch (e: Exception) {
            steps.add("11. EncvHostActivity check FAILED: ${e.message}")
            results.put("hostActivityResolved", false)
        }

        // === 12. EncvApplication 检查 ===
        steps.add("=== Application ===")
        val app = context.applicationContext
        steps.add("12. application class = ${app.javaClass.name}")
        results.put("appClass", app.javaClass.name)
        val isBaseHostApp = app is com.combo.core.runtime.app.BaseHostApplication
        steps.add("13. isBaseHostApplication = $isBaseHostApp")
        results.put("isBaseHostApplication", isBaseHostApp)

        // === 14. APK 文件检查 ===
        steps.add("=== APK Files ===")
        val pluginInstallDir = File(context.cacheDir, "plugin_install")
        steps.add("14. plugin_install dir exists = ${pluginInstallDir.exists()}")
        results.put("pluginInstallDirExists", pluginInstallDir.exists())
        if (pluginInstallDir.exists()) {
            val apkFiles = pluginInstallDir.listFiles()?.filter { it.extension == "apk" }?.map { "${it.name}(${it.length()}B)" }
            steps.add("15. APK files = $apkFiles")
            results.put("apkFiles", apkFiles.toString())
        }

        // === 16. 权限检查 ===
        steps.add("=== Permissions ===")
        val hasStorage = Environment.isExternalStorageManager()
        steps.add("16. MANAGE_EXTERNAL_STORAGE = $hasStorage")
        results.put("hasStoragePermission", hasStorage)

        // === 17. 实际启动测试 ===
        steps.add("=== StartActivity Test ===")
        try {
            val testIntent = Intent(context, com.encvgo.app.InstallConfirmActivity::class.java).apply {
                putExtra(com.encvgo.app.InstallConfirmActivity.EXTRA_APK_PATH, "/data/data/${context.packageName}/cache/plugin_install/test-debug.apk")
                putExtra(com.encvgo.app.InstallConfirmActivity.EXTRA_FILE_NAME, "test-debug.apk")
                putExtra("request_id", "debugTest")
                if (context !is Activity) {
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                }
            }
            steps.add("17. intent: component=${testIntent.component}, flags=${testIntent.flags}, action=${testIntent.action}")
            results.put("intentComponent", testIntent.component?.flattenToString() ?: "null")
            results.put("intentFlags", testIntent.flags)
            results.put("intentAction", testIntent.action ?: "null")

            context.startActivity(testIntent)
            steps.add("18. startActivity SUCCESS ← InstallConfirmActivity should be visible now")
            results.put("startActivityResult", "SUCCESS")
        } catch (e: Exception) {
            steps.add("18. startActivity FAILED: ${e.javaClass.name}: ${e.message}")
            results.put("startActivityResult", "FAILED")
            results.put("startActivityError", "${e.javaClass.name}: ${e.message}")

            // fallback: 尝试用 activity.startActivity
            try {
                val fallbackIntent = Intent(activity, com.encvgo.app.InstallConfirmActivity::class.java).apply {
                    putExtra(com.encvgo.app.InstallConfirmActivity.EXTRA_APK_PATH, "/data/data/${context.packageName}/cache/plugin_install/test-debug.apk")
                    putExtra(com.encvgo.app.InstallConfirmActivity.EXTRA_FILE_NAME, "test-debug.apk")
                    putExtra("request_id", "debugTest")
                }
                activity.startActivity(fallbackIntent)
                steps.add("19. FALLBACK activity.startActivity SUCCESS")
                results.put("fallbackResult", "SUCCESS")
            } catch (e2: Exception) {
                steps.add("19. FALLBACK activity.startActivity ALSO FAILED: ${e2.javaClass.name}: ${e2.message}")
                results.put("fallbackResult", "FAILED: ${e2.message}")
            }
        }

        // === 20. 系统安装 Intent 测试 ===
        steps.add("=== System Install Intent ===")
        try {
            val testApk = File(context.cacheDir, "plugin_install/test-debug.apk")
            if (testApk.exists()) {
                val uri = androidx.core.content.FileProvider.getUriForFile(
                    context, "${context.packageName}.fileprovider", testApk)
                steps.add("20. FileProvider URI = $uri")
                results.put("fileProviderUri", uri.toString())
            } else {
                steps.add("20. test APK not found, skipping FileProvider test")
                results.put("fileProviderUri", "SKIPPED: no test APK")
            }
        } catch (e: Exception) {
            steps.add("20. FileProvider FAILED: ${e.message}")
            results.put("fileProviderUri", "ERROR: ${e.message}")
        }

        Log.i(TAG, "SATURATION-DEBUG debugInstallFlow:\n${steps.joinToString("\n")}")
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
        GlobalScope.launch(Dispatchers.IO) {
            try {
                val result = PluginManager.installerManager.installPlugin(apkFile, true)
                when (result) {
                    is com.combo.core.runtime.installer.InstallerManager.InstallResult.Success -> {
                        Log.i(TAG, "Plugin installed via ComboLite: $apkPath -> ${result.pluginInfo.id}")
                        call.resolve(JSObject().apply {
                            put("success", true)
                            put("method", "combolite")
                            put("pluginId", result.pluginInfo.id)
                        })
                    }
                    is com.combo.core.runtime.installer.InstallerManager.InstallResult.Failure -> {
                        Log.e(TAG, "ComboLite install failed: ${result.reason}", result.exception)
                        call.reject("ComboLite install failed: ${result.reason}")
                    }
                }
            } catch (e: Exception) {
                Log.e(TAG, "ComboLite installPlugin exception", e)
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

            val logcatFile = File(logDir, "logcat_${timestamp}.txt")
            val process = Runtime.getRuntime().exec(arrayOf("logcat", "-d", "-t", "5000", "-v", "time"))
            process.inputStream.bufferedReader().use { reader ->
                logcatFile.outputStream().bufferedWriter().use { writer ->
                    var line: String?
                    while (reader.readLine().also { line = it } != null) {
                        writer.write(line)
                        writer.newLine()
                    }
                }
            }

            val goLogDir = File(context.filesDir, "logs")
            val goFiles = if (goLogDir.exists()) goLogDir.listFiles()?.toList() ?: emptyList() else emptyList()

            val zipFile = File(context.cacheDir, "encv_logs_${timestamp}.zip")
            java.util.zip.ZipOutputStream(zipFile.outputStream()).use { zos ->
                fun addToZip(file: File, entryName: String) {
                    if (!file.exists()) return
                    zos.putNextEntry(java.util.zip.ZipEntry(entryName))
                    file.inputStream().use { it.copyTo(zos) }
                    zos.closeEntry()
                }
                addToZip(logcatFile, "logcat_${timestamp}.txt")
                for (f in goFiles.filter { it.isFile }) {
                    addToZip(f, "go_backend/${f.name}")
                }
                val goStderrLog = File(context.filesDir, "encv-go-stderr.log")
                if (goStderrLog.exists()) addToZip(goStderrLog, "go_backend/encv-go-stderr.log")
                val goStdoutLog = File(context.filesDir, "encv-go-stdout.log")
                if (goStdoutLog.exists()) addToZip(goStdoutLog, "go_backend/encv-go-stdout.log")
            }

            logcatFile.delete()

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
            Runtime.getRuntime().exec(arrayOf("logcat", "-c"))

            val goLogDir = File(context.filesDir, "logs")
            if (goLogDir.exists()) {
                goLogDir.listFiles()?.forEach { it.delete() }
            }
            val goStderr = File(context.filesDir, "encv-go-stderr.log")
            if (goStderr.exists()) goStderr.delete()
            val goStdout = File(context.filesDir, "encv-go-stdout.log")
            if (goStdout.exists()) goStdout.delete()

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
            val logcatFile = File(context.cacheDir, "encv_logs_export/logcat_latest.txt")
            val process = Runtime.getRuntime().exec(arrayOf("logcat", "-d", "-t", "3000", "-v", "time"))
            logcatFile.parentFile?.mkdirs()
            process.inputStream.bufferedReader().use { reader ->
                logcatFile.outputStream().bufferedWriter().use { writer ->
                    var line: String?
                    while (reader.readLine().also { line = it } != null) {
                        writer.write(line)
                        writer.newLine()
                    }
                }
            }

            val uri = androidx.core.content.FileProvider.getUriForFile(
                context, "${context.packageName}.fileprovider", logcatFile)
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
}
