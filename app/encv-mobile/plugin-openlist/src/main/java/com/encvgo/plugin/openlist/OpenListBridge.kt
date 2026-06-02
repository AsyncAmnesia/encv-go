package com.encvgo.plugin.openlist

import android.content.Context
import android.content.Intent
import android.util.Log
import androidx.localbroadcastmanager.content.LocalBroadcastManager

// TODO: replace with real interfaces from openlist.aar (gomobile bind output) once available.
//   AAR ships as a 0-byte placeholder; run scripts/build-openlist-aar.sh to produce the real artifact.
//   These stubs mirror openlistlib.Event / openlistlib.LogCallback shape so call sites stay stable.
interface OpenListEvent {
    fun onShutdown(type: String)
    fun onStartError(type: String, msg: String)
    fun onProcessExit(code: Long)
}

interface OpenListLogCallback {
    fun onLog(level: Short, time: Long, log: String)
}

// TODO: when real openlist.aar lands, switch imports to openlistlib.Openlistlib / Event / LogCallback
//   and delete the local OpenListEvent / OpenListLogCallback stubs above.
object OpenListBridge : OpenListEvent, OpenListLogCallback {

    private const val TAG = "OpenList"

    @Volatile
    private var dataDir: String = ""
    @Volatile
    private var port: Int = 0
    @Volatile
    private var running: Boolean = false
    @Volatile
    private var appContext: Context? = null

    fun init(context: Context) {
        appContext = context.applicationContext
    }

    fun start() {
        if (running) return
        // TODO: call Openlistlib.setConfigData(dataDir) then Openlistlib.start()
        running = true
        Log.i(TAG, "OpenList started (placeholder), dataDir=$dataDir port=$port")
    }

    fun shutdown(timeoutMs: Long) {
        if (!running) return
        // TODO: call Openlistlib.shutdown(timeoutMs.toInt())
        running = false
        Log.i(TAG, "OpenList shutdown requested (placeholder), timeout=${timeoutMs}ms")
    }

    fun isRunning(): Boolean = running

    fun setAdminPassword(pwd: String) {
        // TODO: Openlistlib.setAdminPassword(pwd)
        Log.i(TAG, "setAdminPassword (placeholder), length=${pwd.length}")
    }

    fun forceDbSync() {
        // TODO: Openlistlib.forceDBSync()
        Log.d(TAG, "forceDbSync (placeholder)")
    }

    fun setDataDir(path: String) {
        dataDir = path
    }

    fun setPort(p: Int) {
        port = p
    }

    fun broadcastStatus(port: Int, running: Boolean) {
        val ctx = appContext ?: return
        LocalBroadcastManager.getInstance(ctx).sendBroadcast(
            Intent(OpenListService.BROADCAST_STATUS_CHANGED)
                .putExtra(OpenListService.EXTRA_PORT, port)
                .putExtra(OpenListService.EXTRA_RUNNING, running)
        )
    }

    override fun onShutdown(type: String) {
        Log.d(TAG, "onShutdown: $type")
        running = false
        broadcastStatus(0, false)
        val ctx = appContext ?: return
        LocalBroadcastManager.getInstance(ctx).sendBroadcast(
            Intent(OpenListService.BROADCAST_PROCESS_EXIT).putExtra("reason", type)
        )
    }

    override fun onStartError(type: String, msg: String) {
        Log.e(TAG, "onStartError: $type, $msg")
        running = false
    }

    override fun onProcessExit(code: Long) {
        Log.w(TAG, "onProcessExit: $code")
        running = false
        val ctx = appContext ?: return
        LocalBroadcastManager.getInstance(ctx).sendBroadcast(
            Intent(OpenListService.BROADCAST_PROCESS_EXIT).putExtra("code", code)
        )
    }

    override fun onLog(level: Short, time: Long, log: String) {
        Log.i(TAG, "[$level @ $time] $log")
        val ctx = appContext ?: return
        LocalBroadcastManager.getInstance(ctx).sendBroadcast(
            Intent(OpenListService.BROADCAST_LOG)
                .putExtra("level", level.toInt())
                .putExtra("time", time)
                .putExtra("log", log)
        )
    }
}
