package com.encvgo.app

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.os.Build
import android.util.Log
import com.lynx.tasm.LynxView
import com.lynx.tasm.behavior.LynxModule
import com.lynx.tasm.behavior.LynxModuleMethod
import com.lynx.tasm.behavior.LynxPromise
import org.json.JSONObject

class GoBackendModule(lynxView: LynxView) : LynxModule(lynxView) {
    companion object {
        private const val TAG = "GoBackendModule"
        const val EVENT_READY = "backend:ready"
        const val EVENT_ERROR = "backend:error"
    }

    private val context = lynxView.context
    private var receiverRegistered = false

    private val backendReceiver = object : BroadcastReceiver() {
        override fun onReceive(ctx: Context?, intent: Intent?) {
            Log.d(TAG, "received broadcast: ${intent?.action}")
            when (intent?.action) {
                EncvGoService.BROADCAST_BACKEND_READY -> {
                    val port = intent.getIntExtra(EncvGoService.EXTRA_PORT, 0)
                    Log.d(TAG, "broadcast: backend ready, port=$port")
                    dispatchReady(port)
                }
                EncvGoService.BROADCAST_BACKEND_STATUS -> {
                    val port = intent.getIntExtra(EncvGoService.EXTRA_PORT, 0)
                    val running = intent.getBooleanExtra(EncvGoService.EXTRA_RUNNING, false)
                    val error = intent.getStringExtra(EncvGoService.EXTRA_ERROR)
                    Log.d(TAG, "broadcast: status, port=$port, running=$running, error=$error")
                    if (error != null) {
                        dispatchError(error)
                    }
                }
            }
        }
    }

    init {
        registerReceiver()
        Log.d(TAG, "init: backend module created")
    }

    private fun registerReceiver() {
        if (receiverRegistered) return
        Log.d(TAG, "registerReceiver: registering backend receiver")
        val filter = IntentFilter().apply {
            addAction(EncvGoService.BROADCAST_BACKEND_READY)
            addAction(EncvGoService.BROADCAST_BACKEND_STATUS)
        }
        if (Build.VERSION.SDK_INT >= 33) {
            context.registerReceiver(backendReceiver, filter, Context.RECEIVER_NOT_EXPORTED)
        } else {
            @Suppress("DEPRECATION")
            context.registerReceiver(backendReceiver, filter)
        }
        receiverRegistered = true
        Log.d(TAG, "registerReceiver: registered")
    }

    fun unregisterReceiver() {
        if (!receiverRegistered) return
        Log.d(TAG, "unregisterReceiver: unregistering backend receiver")
        try {
            context.unregisterReceiver(backendReceiver)
        } catch (e: Exception) {
            Log.e(TAG, "unregisterReceiver failed", e)
        }
        receiverRegistered = false
    }

    @LynxModuleMethod
    fun getBackendStatus(params: Map<String, Any>, promise: LynxPromise) {
        val isRunning = EncvGoService.isRunning
        val port = EncvGoService.lastKnownPort
        Log.d(TAG, "getBackendStatus: running=$isRunning, port=$port")
        val result = JSONObject().apply {
            put("running", isRunning)
            put("port", port)
        }
        promise.resolve(result)
    }

    @LynxModuleMethod
    fun startBackend(params: Map<String, Any>, promise: LynxPromise) {
        Log.d(TAG, "startBackend: starting EncvGoService")
        try {
            val serviceIntent = EncvGoService.createIntent(context, EncvGoService.ACTION_START, "player")
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(serviceIntent)
            } else {
                context.startService(serviceIntent)
            }
            promise.resolve(true)
        } catch (e: Exception) {
            Log.e(TAG, "startBackend failed", e)
            promise.reject(e.message)
        }
    }

    @LynxModuleMethod
    fun getStreamUrl(params: Map<String, Any>, promise: LynxPromise) {
        val path = params["path"] as? String ?: run {
            promise.reject("path is required")
            return
        }
        val isExternal = params["isExternal"] as? Boolean ?: false
        val port = EncvGoService.lastKnownPort
        if (port <= 0) {
            promise.reject("backend not ready, port invalid")
            return
        }
        val encodedPath = android.net.Uri.encode(path)
        val url = if (isExternal) {
            "http://127.0.0.1:$port/api/stream/external?path=$encodedPath"
        } else {
            "http://127.0.0.1:$port/stream?path=$encodedPath"
        }
        Log.d(TAG, "getStreamUrl: path=$path, isExternal=$isExternal → $url")
        promise.resolve(url)
    }

    private fun dispatchReady(port: Int) {
        val data = JSONObject().apply {
            put("port", port)
        }
        lynxView.dispatchEvent(EVENT_READY, data)
    }

    private fun dispatchError(message: String) {
        val data = JSONObject().apply {
            put("message", message)
        }
        lynxView.dispatchEvent(EVENT_ERROR, data)
    }
}
