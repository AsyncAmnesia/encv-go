package com.encvgo.app

import android.Manifest
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
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
import java.util.concurrent.ConcurrentHashMap

@CapacitorPlugin(
    name = "GoProcess"
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
        val filePath = call.getString("filePath", "")
        val name = call.getString("name", "")
        val mimeType = call.getString("mimeType", "")
        try {
            Log.d(TAG, "openPlayer: filePath=$filePath, name=$name, mimeType=$mimeType")
            val mainActivity = activity as? MainActivity
            if (mainActivity == null) {
                Log.w(TAG, "openPlayer: activity is not MainActivity, falling back to openInPlayer")
                if (filePath.isNullOrEmpty()) {
                    openPlayerHome(call)
                } else {
                    openInPlayer(call)
                }
                return
            }
            PlayerOverlayManager.getInstance().showOverlay(
                mainActivity, filePath ?: "", name ?: "", mimeType ?: "", false
            )
            call.resolve()
        } catch (e: Exception) {
            Log.e(TAG, "openPlayer failed", e)
            call.reject("Failed to open player: ${e.message}")
        }
    }

    @PluginMethod
    fun closePlayer(call: PluginCall) {
        try {
            Log.d(TAG, "closePlayer: closing player overlay")
            PlayerOverlayManager.getInstance().hideOverlay()
            call.resolve()
        } catch (e: Exception) {
            Log.e(TAG, "closePlayer failed", e)
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
}
