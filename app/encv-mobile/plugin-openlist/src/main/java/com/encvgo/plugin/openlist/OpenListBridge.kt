package com.encvgo.plugin.openlist

import android.content.Context
import android.content.Intent
import android.os.Handler
import android.os.Looper
import android.util.Log
import androidx.localbroadcastmanager.content.LocalBroadcastManager
import openlistlib.Event
import openlistlib.LogCallback
import openlistlib.Openlistlib

object OpenListBridge : Event, LogCallback {

    private const val TAG = "OpenList"

    private val openlistlib = Openlistlib()
    private val handler = Handler(Looper.getMainLooper())

    @Volatile
    private var dataDir: String = ""
    @Volatile
    private var port: Int = 0
    @Volatile
    private var running: Boolean = false
    @Volatile
    private var appContext: Context? = null

    fun init(context: Context) {
        Log.e(TAG, "[SAT-DBG][OpenList] init() entry | dataDir=$dataDir port=$port | thread=${Thread.currentThread().name}")
        appContext = context.applicationContext
        openlistlib.setConfigData(dataDir)
        openlistlib.setPort(port.toLong())
        Log.e(TAG, "[SAT-DBG][OpenList] init() done | setConfigData + setPort applied")
    }

    fun start() {
        Log.e(TAG, "[SAT-DBG][OpenList] start() entry | running=$running | thread=${Thread.currentThread().name}")
        if (running) {
            Log.e(TAG, "[SAT-DBG][OpenList] start() skipped (already running)")
            return
        }
        Thread {
            Log.e(TAG, "[SAT-DBG][OpenList] start() background thread | thread=${Thread.currentThread().name}")
            openlistlib.start()
        }.start()
        running = true
        broadcastStatus(port, true)
        Log.e(TAG, "[SAT-DBG][OpenList] start() done | running=$running")
    }

    fun shutdown(timeoutMs: Long) {
        Log.e(TAG, "[SAT-DBG][OpenList] shutdown() entry | timeoutMs=$timeoutMs | running=$running | thread=${Thread.currentThread().name}")
        if (!running) {
            Log.e(TAG, "[SAT-DBG][OpenList] shutdown() skipped (not running)")
            return
        }
        openlistlib.shutdown(timeoutMs.toInt())
        Log.e(TAG, "[SAT-DBG][OpenList] shutdown() native call returned | starting 500ms grace timer")
        handler.postDelayed({
            if (running) {
                Log.e(TAG, "[SAT-DBG][OpenList] shutdown() grace timer expired | force-setting running=false")
                running = false
                broadcastStatus(0, false)
            }
        }, 500)
    }

    fun isRunning(): Boolean = running

    fun setAdminPassword(pwd: String) {
        Log.e(TAG, "[SAT-DBG][OpenList] setAdminPassword() entry | length=${pwd.length} | thread=${Thread.currentThread().name}")
        openlistlib.setAdminPassword(pwd)
        Log.e(TAG, "[SAT-DBG][OpenList] setAdminPassword() done")
    }

    fun forceDbSync() {
        Log.e(TAG, "[SAT-DBG][OpenList] forceDbSync() entry | thread=${Thread.currentThread().name}")
        try {
            openlistlib.forceDBSync()
            Log.e(TAG, "[SAT-DBG][OpenList] forceDbSync() done")
        } catch (e: Exception) {
            Log.e(TAG, "[SAT-DBG][OpenList] forceDbSync() failed", e)
        }
    }

    fun setDataDir(path: String) {
        dataDir = path
    }

    fun setPort(p: Int) {
        port = p
    }

    fun broadcastStatus(port: Int, running: Boolean) {
        Log.e(TAG, "[SAT-DBG][OpenList] broadcastStatus() | port=$port running=$running")
        val ctx = appContext ?: run {
            Log.e(TAG, "[SAT-DBG][OpenList] broadcastStatus() skipped (no appContext)")
            return
        }
        LocalBroadcastManager.getInstance(ctx).sendBroadcast(
            Intent(OpenListService.BROADCAST_STATUS_CHANGED)
                .putExtra(OpenListService.EXTRA_PORT, port)
                .putExtra(OpenListService.EXTRA_RUNNING, running)
        )
    }

    override fun onShutdown(type: String) {
        Log.e(TAG, "[SAT-DBG][OpenList] onShutdown() | type=$type | thread=${Thread.currentThread().name}")
        running = false
        broadcastStatus(0, false)
        val ctx = appContext ?: return
        LocalBroadcastManager.getInstance(ctx).sendBroadcast(
            Intent(OpenListService.BROADCAST_PROCESS_EXIT).putExtra("reason", type)
        )
    }

    override fun onStartError(type: String, msg: String) {
        Log.e(TAG, "[SAT-DBG][OpenList] onStartError() | type=$type msg=$msg | thread=${Thread.currentThread().name}")
        running = false
    }

    override fun onProcessExit(code: Long) {
        Log.e(TAG, "[SAT-DBG][OpenList] onProcessExit() | code=$code | thread=${Thread.currentThread().name}")
        running = false
        val ctx = appContext ?: return
        LocalBroadcastManager.getInstance(ctx).sendBroadcast(
            Intent(OpenListService.BROADCAST_PROCESS_EXIT).putExtra("code", code)
        )
    }

    override fun onLog(level: Short, time: Long, log: String) {
        Log.e(TAG, "[SAT-DBG][OpenList] onLog() | level=$level time=$time log=$log")
        val ctx = appContext ?: return
        LocalBroadcastManager.getInstance(ctx).sendBroadcast(
            Intent(OpenListService.BROADCAST_LOG)
                .putExtra("level", level.toInt())
                .putExtra("time", time)
                .putExtra("log", log)
        )
    }
}