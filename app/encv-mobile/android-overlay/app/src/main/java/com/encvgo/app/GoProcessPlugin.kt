package com.encvgo.app

import android.Manifest
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Environment
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
    fun isStandaloneMode(call: PluginCall) {
        val result = JSObject()
        result.put("standalone", activity is PlayerActivity)
        call.resolve(result)
    }

    @PluginMethod
    fun getIntentFileInfo(call: PluginCall) {
        val result = JSObject()
        if (activity is PlayerActivity) {
            result.put("path", PlayerActivity.intentFilePath)
            result.put("name", PlayerActivity.intentFileName)
            result.put("mimeType", PlayerActivity.intentFileMimeType)
        } else {
            result.put("path", "")
            result.put("name", "")
            result.put("mimeType", "")
        }
        call.resolve(result)
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
}
