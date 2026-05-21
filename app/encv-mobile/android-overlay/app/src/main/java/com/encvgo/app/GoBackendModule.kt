package com.encvgo.app

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.os.Build
import android.util.Log
import com.lynx.jsbridge.LynxMethod
import com.lynx.jsbridge.LynxModule
import com.lynx.react.bridge.Callback
import com.lynx.react.bridge.JavaOnlyArray
import com.lynx.react.bridge.JavaOnlyMap
import com.lynx.tasm.behavior.LynxContext

class GoBackendModule(context: android.content.Context) : LynxModule(context) {
    companion object {
        private const val TAG = "GoBackendModule"
        const val EVENT_READY = "backend:ready"
        const val EVENT_ERROR = "backend:error"

        @Volatile
        private var _instance: GoBackendModule? = null
        fun getInstance(): GoBackendModule? = _instance
    }

    private val lynxContext = context as LynxContext
    private val appContext = context.applicationContext
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
        _instance = this
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
            appContext.registerReceiver(backendReceiver, filter, Context.RECEIVER_NOT_EXPORTED)
        } else {
            @Suppress("DEPRECATION")
            appContext.registerReceiver(backendReceiver, filter)
        }
        receiverRegistered = true
        Log.d(TAG, "registerReceiver: registered")
    }

    fun unregisterReceiver() {
        if (!receiverRegistered) return
        Log.d(TAG, "unregisterReceiver: unregistering backend receiver")
        try {
            appContext.unregisterReceiver(backendReceiver)
        } catch (e: Exception) {
            Log.e(TAG, "unregisterReceiver failed", e)
        }
        receiverRegistered = false
    }

    @LynxMethod
    fun getBackendStatus(callback: Callback) {
        val isRunning = EncvGoService.isRunning
        val port = EncvGoService.lastKnownPort
        Log.d(TAG, "getBackendStatus: running=$isRunning, port=$port")
        val result = JavaOnlyMap().apply {
            put("running", isRunning)
            put("port", port)
        }
        callback.invoke(result)
    }

    @LynxMethod
    fun startBackend(callback: Callback) {
        Log.d(TAG, "startBackend: starting EncvGoService")
        try {
            val serviceIntent = EncvGoService.createIntent(appContext, EncvGoService.ACTION_START, "player")
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                appContext.startForegroundService(serviceIntent)
            } else {
                appContext.startService(serviceIntent)
            }
            callback.invoke(true)
        } catch (e: Exception) {
            Log.e(TAG, "startBackend failed", e)
            callback.invoke(e.message)
        }
    }

    @LynxMethod
    fun getStreamUrl(path: String, isExternal: Boolean, callback: Callback) {
        val port = EncvGoService.lastKnownPort
        if (port <= 0) {
            callback.invoke("backend not ready, port invalid")
            return
        }
        val encodedPath = android.net.Uri.encode(path)
        val url = if (isExternal) {
            "http://127.0.0.1:$port/api/stream/external?path=$encodedPath"
        } else {
            "http://127.0.0.1:$port/stream?path=$encodedPath"
        }
        Log.d(TAG, "getStreamUrl: path=$path, isExternal=$isExternal → $url")
        callback.invoke(url)
    }

    private fun dispatchReady(port: Int) {
        val data = JavaOnlyMap().apply { put("port", port) }
        val params = JavaOnlyArray()
        params.pushMap(data)
        lynxContext.sendGlobalEvent(EVENT_READY, params)
    }

    private fun dispatchError(message: String) {
        val data = JavaOnlyMap().apply { put("message", message) }
        val params = JavaOnlyArray()
        params.pushMap(data)
        lynxContext.sendGlobalEvent(EVENT_ERROR, params)
    }
}
