package com.encvgo.app

import android.app.Activity
import android.content.pm.ActivityInfo
import android.util.Log
import android.view.SurfaceHolder
import android.view.SurfaceView
import android.view.View
import android.view.ViewGroup
import android.view.WindowManager
import com.lynx.jsbridge.LynxMethod
import com.lynx.jsbridge.LynxModule
import com.lynx.react.bridge.Callback
import com.lynx.react.bridge.JavaOnlyArray
import com.lynx.react.bridge.JavaOnlyMap
import com.lynx.tasm.behavior.LynxContext
import `is`.xyz.mpv.MPVLib

class MpvPlayerModule(context: android.content.Context) : LynxModule(context) {
    companion object {
        private const val TAG = "MpvPlayerModule"
        const val EVENT_STATE_CHANGE = "mpv:state-change"
        const val EVENT_POSITION_UPDATE = "mpv:position-update"
        private const val MPV_EVENT_FILE_LOADED = 21
        private const val MPV_EVENT_END_FILE = 7
        private const val MPV_FORMAT_FLAG = 3

        @Volatile
        private var _instance: MpvPlayerModule? = null
        fun getInstance(): MpvPlayerModule? = _instance
    }

    private val lynxContext = context as LynxContext
    private val activity = lynxContext.context as Activity
    private var mpvSurfaceView: MpvSurfaceView? = null
    private var isFullscreen = false
    private var mpvInitialized = false
    private var pendingUrl: String? = null

    private val eventObserver = object : MPVLib.EventObserver {
        override fun eventProperty(property: String) {}
        override fun eventProperty(property: String, value: Long) {}
        override fun eventProperty(property: String, value: Boolean) {
            when (property) {
                "pause" -> dispatchStateChange(if (value) "paused" else "playing")
                "idle" -> { if (value) dispatchStateChange("end-file") }
            }
        }
        override fun eventProperty(property: String, value: Double) {}
        override fun eventProperty(property: String, value: String) {}
        override fun event(eventId: Int) {
            when (eventId) {
                MPV_EVENT_FILE_LOADED -> dispatchStateChange("playing")
                MPV_EVENT_END_FILE -> dispatchStateChange("end-file")
            }
        }
    }

    init {
        _instance = this
        Log.d(TAG, "init: MpvPlayerModule created")
    }

    private fun ensureMpvInitialized() {
        if (mpvInitialized) return
        Log.d(TAG, "ensureMpvInitialized: initializing MPVLib")
        MPVLib.create(activity.application)
        MPVLib.setOptionString("config", "yes")
        MPVLib.setOptionString("vo", "gpu")
        MPVLib.setOptionString("hwdec", "auto")
        MPVLib.setOptionString("force-window", "no")
        MPVLib.setOptionString("idle", "yes")
        MPVLib.init()
        MPVLib.addObserver(eventObserver)
        MPVLib.observeProperty("pause", MPV_FORMAT_FLAG)
        MPVLib.observeProperty("idle", MPV_FORMAT_FLAG)
        mpvInitialized = true
        Log.d(TAG, "ensureMpvInitialized: done")
    }

    fun attachToLayout(rootLayout: ViewGroup) {
        Log.d(TAG, "attachToLayout: adding MPV surface view to root layout (index 0)")
        ensureMpvInitialized()
        mpvSurfaceView = MpvSurfaceView(activity).apply {
            id = ViewGroup.generateViewId()
            keepScreenOn = true
        }
        val params = ViewGroup.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT,
            ViewGroup.LayoutParams.MATCH_PARENT
        )
        rootLayout.addView(mpvSurfaceView, 0, params)
        Log.d(TAG, "attachToLayout: MPV surface view attached")
    }

    fun detachFromLayout(rootLayout: ViewGroup) {
        Log.d(TAG, "detachFromLayout: removing MPV surface view")
        mpvSurfaceView?.let { rootLayout.removeView(it) }
    }

    fun release() {
        Log.d(TAG, "release: stopping MPV")
        if (mpvInitialized) {
            try {
                MPVLib.removeObserver(eventObserver)
                MPVLib.destroy()
            } catch (e: Exception) {
                Log.e(TAG, "release: MPVLib.destroy failed", e)
            }
            mpvInitialized = false
        }
        mpvSurfaceView = null
        _instance = null
    }

    @LynxMethod
    fun play(url: String, callback: Callback) {
        Log.d(TAG, "play: url=$url")
        try {
            ensureMpvInitialized()
            MPVLib.command(arrayOf("loadfile", url))
            callback.invoke(true)
        } catch (e: Exception) {
            Log.e(TAG, "play failed", e)
            callback.invoke(e.message)
        }
    }

    @LynxMethod
    fun pause(callback: Callback) {
        Log.d(TAG, "pause")
        try {
            MPVLib.setPropertyBoolean("pause", true)
            callback.invoke(true)
        } catch (e: Exception) {
            Log.e(TAG, "pause failed", e)
            callback.invoke(e.message)
        }
    }

    @LynxMethod
    fun resume(callback: Callback) {
        Log.d(TAG, "resume")
        try {
            MPVLib.setPropertyBoolean("pause", false)
            callback.invoke(true)
        } catch (e: Exception) {
            Log.e(TAG, "resume failed", e)
            callback.invoke(e.message)
        }
    }

    @LynxMethod
    fun seekTo(positionMs: Int, callback: Callback) {
        Log.d(TAG, "seekTo: $positionMs ms")
        try {
            val positionSec = positionMs / 1000.0
            MPVLib.command(arrayOf("seek", positionSec.toString(), "absolute"))
            callback.invoke(true)
        } catch (e: Exception) {
            Log.e(TAG, "seekTo failed", e)
            callback.invoke(e.message)
        }
    }

    @LynxMethod
    fun setFullscreen(enabled: Boolean, callback: Callback) {
        Log.d(TAG, "setFullscreen: $enabled")
        try {
            if (enabled) {
                activity.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE
                activity.window.addFlags(WindowManager.LayoutParams.FLAG_FULLSCREEN)
                @Suppress("DEPRECATION")
                activity.window.decorView.systemUiVisibility = (
                    View.SYSTEM_UI_FLAG_LAYOUT_STABLE
                        or View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION
                        or View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
                        or View.SYSTEM_UI_FLAG_HIDE_NAVIGATION
                        or View.SYSTEM_UI_FLAG_FULLSCREEN
                        or View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY
                )
                isFullscreen = true
            } else {
                activity.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_SENSOR_PORTRAIT
                activity.window.clearFlags(WindowManager.LayoutParams.FLAG_FULLSCREEN)
                @Suppress("DEPRECATION")
                activity.window.decorView.systemUiVisibility = View.SYSTEM_UI_FLAG_VISIBLE
                isFullscreen = false
            }
            callback.invoke(true)
        } catch (e: Exception) {
            Log.e(TAG, "setFullscreen failed", e)
            callback.invoke(e.message)
        }
    }

    @LynxMethod
    fun setOrientation(orientation: String, callback: Callback) {
        Log.d(TAG, "setOrientation: $orientation")
        try {
            activity.requestedOrientation = when (orientation) {
                "landscape" -> ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE
                "portrait" -> ActivityInfo.SCREEN_ORIENTATION_SENSOR_PORTRAIT
                else -> ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED
            }
            callback.invoke(true)
        } catch (e: Exception) {
            Log.e(TAG, "setOrientation failed", e)
            callback.invoke(e.message)
        }
    }

    @LynxMethod
    fun getDuration(callback: Callback) {
        val durationSec = MPVLib.getPropertyDouble("duration") ?: 0.0
        val durationMs = (durationSec * 1000).toInt()
        Log.d(TAG, "getDuration: $durationMs ms")
        callback.invoke(durationMs)
    }

    @LynxMethod
    fun getCurrentPosition(callback: Callback) {
        val positionSec = MPVLib.getPropertyDouble("time-pos") ?: 0.0
        val positionMs = (positionSec * 1000).toInt()
        Log.d(TAG, "getCurrentPosition: $positionMs ms")
        callback.invoke(positionMs)
    }

    @LynxMethod
    fun isPlaying(callback: Callback) {
        val paused = MPVLib.getPropertyBoolean("pause") ?: true
        val playing = !paused
        Log.d(TAG, "isPlaying: $playing")
        callback.invoke(playing)
    }

    @LynxMethod
    fun setProperty(key: String, value: String, callback: Callback) {
        Log.d(TAG, "setProperty: key=$key, value=$value")
        try {
            MPVLib.setPropertyString(key, value)
            callback.invoke(true)
        } catch (e: Exception) {
            Log.e(TAG, "setProperty failed", e)
            callback.invoke(e.message)
        }
    }

    fun dispatchPositionUpdate() {
        val positionSec = MPVLib.getPropertyDouble("time-pos") ?: 0.0
        val durationSec = MPVLib.getPropertyDouble("duration") ?: 0.0
        val position = (positionSec * 1000).toInt()
        val duration = (durationSec * 1000).toInt()
        val data = JavaOnlyMap().apply {
            put("position", position)
            put("duration", duration)
        }
        val params = JavaOnlyArray()
        params.pushMap(data)
        lynxContext.sendGlobalEvent(EVENT_POSITION_UPDATE, params)
    }

    private fun dispatchStateChange(state: String, error: String? = null) {
        val data = JavaOnlyMap().apply {
            put("state", state)
            if (error != null) put("error", error)
        }
        val params = JavaOnlyArray()
        params.pushMap(data)
        lynxContext.sendGlobalEvent(EVENT_STATE_CHANGE, params)
    }

    private inner class MpvSurfaceView(context: android.content.Context) :
        SurfaceView(context), SurfaceHolder.Callback {

        init {
            holder.addCallback(this)
        }

        override fun surfaceCreated(holder: SurfaceHolder) {
            Log.d(TAG, "MpvSurfaceView: surfaceCreated, attaching surface")
            MPVLib.attachSurface(holder.surface)
            MPVLib.setOptionString("force-window", "yes")
            MPVLib.setPropertyString("vo", "gpu")
            pendingUrl?.let { url ->
                Log.d(TAG, "MpvSurfaceView: playing pending url=$url")
                MPVLib.command(arrayOf("loadfile", url))
                pendingUrl = null
            }
        }

        override fun surfaceChanged(holder: SurfaceHolder, format: Int, width: Int, height: Int) {
            MPVLib.setPropertyString("android-surface-size", "${width}x$height")
        }

        override fun surfaceDestroyed(holder: SurfaceHolder) {
            Log.d(TAG, "MpvSurfaceView: surfaceDestroyed, detaching surface")
            MPVLib.setPropertyString("vo", "null")
            MPVLib.detachSurface()
        }
    }
}
