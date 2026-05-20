package com.encvgo.app

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.provider.Settings
import android.util.Log
import com.getcapacitor.JSObject
import com.getcapacitor.Plugin
import com.getcapacitor.PluginCall
import com.getcapacitor.PluginMethod
import com.getcapacitor.annotation.CapacitorPlugin

@CapacitorPlugin(
    name = "GoProcess"
)
class GoProcessPlugin : Plugin() {

    companion object {
        private const val TAG = "ENCV-go"
    }

    @PluginMethod
    fun restart(call: PluginCall) {
        Log.d(TAG, "GoProcess.restart() called")
        val mainActivity = activity as? MainActivity
        if (mainActivity == null) {
            call.reject("Activity is not MainActivity")
            return
        }
        mainActivity.restartGoDaemon { port ->
            if (port > 0) {
                val result = JSObject()
                result.put("success", true)
                result.put("port", port)
                call.resolve(result)
            } else {
                call.reject("Backend failed to start")
            }
        }
    }

    @PluginMethod
    fun stop(call: PluginCall) {
        Log.d(TAG, "GoProcess.stop() called")
        val mainActivity = activity as? MainActivity
        if (mainActivity == null) {
            call.reject("Activity is not MainActivity")
            return
        }
        mainActivity.stopGoDaemon()
        val result = JSObject()
        result.put("success", true)
        call.resolve(result)
    }

    @PluginMethod
    fun getStatus(call: PluginCall) {
        Log.d(TAG, "GoProcess.getStatus() called")
        val mainActivity = activity as? MainActivity
        if (mainActivity == null) {
            call.reject("Activity is not MainActivity")
            return
        }
        val result = JSObject()
        result.put("running", mainActivity.isBackendRunning())
        result.put("port", mainActivity.getBackendPort())
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
        if (Build.VERSION.SDK_INT >= 30) {
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
        } else {
            val readGranted = activity.checkSelfPermission(Manifest.permission.READ_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED
            val writeGranted = activity.checkSelfPermission(Manifest.permission.WRITE_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED
            if (readGranted && writeGranted) {
                result.put("granted", true)
                call.resolve(result)
                return
            }
            activity.requestPermissions(arrayOf(Manifest.permission.READ_EXTERNAL_STORAGE, Manifest.permission.WRITE_EXTERNAL_STORAGE), 1002)
            result.put("granted", false)
            call.resolve(result)
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

        val storageGranted = if (Build.VERSION.SDK_INT >= 30) {
            Environment.isExternalStorageManager()
        } else {
            activity.checkSelfPermission(Manifest.permission.READ_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED
        }
        result.put("storage", storageGranted)

        call.resolve(result)
    }
}
