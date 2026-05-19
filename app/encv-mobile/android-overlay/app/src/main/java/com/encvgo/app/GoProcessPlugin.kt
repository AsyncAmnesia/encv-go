package com.encvgo.app

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.provider.Settings
import android.util.Log
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import com.getcapacitor.JSObject
import com.getcapacitor.Plugin
import com.getcapacitor.PluginCall
import com.getcapacitor.PluginMethod
import com.getcapacitor.annotation.CapacitorPlugin
import com.getcapacitor.annotation.Permission
import com.getcapacitor.annotation.PermissionCallback

@CapacitorPlugin(
    name = "GoProcess",
    permissions = [
        Permission(
            alias = "notifications",
            strings = ["android.permission.POST_NOTIFICATIONS"]
        ),
        Permission(
            alias = "storage",
            strings = [
                "android.permission.READ_EXTERNAL_STORAGE",
                "android.permission.WRITE_EXTERNAL_STORAGE"
            ]
        )
    ]
)
class GoProcessPlugin : Plugin() {

    companion object {
        private const val TAG = "ENCV-go"
        private const val REQ_MANAGE_STORAGE = 2001
    }

    @PluginMethod
    fun restart(call: PluginCall) {
        val activity = activity as? MainActivity
        if (activity == null) {
            call.reject("Activity is not MainActivity")
            return
        }
        activity.restartGoDaemon { port ->
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
        val activity = activity as? MainActivity
        if (activity == null) {
            call.reject("Activity is not MainActivity")
            return
        }
        activity.stopGoDaemon()
        val result = JSObject()
        result.put("success", true)
        call.resolve(result)
    }

    @PluginMethod
    fun getStatus(call: PluginCall) {
        val activity = activity as? MainActivity
        if (activity == null) {
            call.reject("Activity is not MainActivity")
            return
        }
        val result = JSObject()
        result.put("running", activity.isBackendRunning())
        result.put("port", activity.getBackendPort())
        call.resolve(result)
    }

    @PluginMethod
    fun requestNotificationPermission(call: PluginCall) {
        if (Build.VERSION.SDK_INT < 33) {
            val result = JSObject()
            result.put("granted", true)
            call.resolve(result)
            return
        }
        if (ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS)
            == PackageManager.PERMISSION_GRANTED
        ) {
            val result = JSObject()
            result.put("granted", true)
            call.resolve(result)
            return
        }
        requestPermissionForAlias("notifications", call, "notificationPermissionCallback")
    }

    @PermissionCallback
    private fun notificationPermissionCallback(call: PluginCall) {
        val granted = getPermissionState("notifications") == com.getcapacitor.PermissionState.GRANTED
        val result = JSObject()
        result.put("granted", granted)
        call.resolve(result)
    }

    @PluginMethod
    fun requestStoragePermission(call: PluginCall) {
        if (Build.VERSION.SDK_INT >= 30) {
            val alreadyGranted = Environment.isExternalStorageManager()
            if (alreadyGranted) {
                val result = JSObject()
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
            val result = JSObject()
            result.put("granted", false)
            result.put("requiresSettings", true)
            call.resolve(result)
        } else {
            val readGranted = ContextCompat.checkSelfPermission(context, Manifest.permission.READ_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED
            val writeGranted = ContextCompat.checkSelfPermission(context, Manifest.permission.WRITE_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED
            if (readGranted && writeGranted) {
                val result = JSObject()
                result.put("granted", true)
                call.resolve(result)
                return
            }
            requestPermissionForAlias("storage", call, "storagePermissionCallback")
        }
    }

    @PermissionCallback
    private fun storagePermissionCallback(call: PluginCall) {
        val readState = getPermissionState("storage")
        val granted = readState == com.getcapacitor.PermissionState.GRANTED
        val result = JSObject()
        result.put("granted", granted)
        call.resolve(result)
    }

    @PluginMethod
    fun checkPermissions(call: PluginCall) {
        val result = JSObject()

        val notificationGranted = if (Build.VERSION.SDK_INT >= 33) {
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
        } else {
            true
        }
        result.put("notifications", notificationGranted)

        val storageGranted = if (Build.VERSION.SDK_INT >= 30) {
            Environment.isExternalStorageManager()
        } else {
            ContextCompat.checkSelfPermission(context, Manifest.permission.READ_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED
        }
        result.put("storage", storageGranted)

        call.resolve(result)
    }
}
