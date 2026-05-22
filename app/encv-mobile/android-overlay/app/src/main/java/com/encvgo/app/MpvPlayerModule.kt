package com.encvgo.app

import android.app.Activity
import android.content.pm.ActivityInfo
import android.os.Handler
import android.os.Looper
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
import `is`.xyz.mpv.MPVLib.MpvEvent
import `is`.xyz.mpv.MPVLib.MpvFormat

class MpvPlayerModule(context: android.content.Context) : LynxModule(context) {
    companion object {
        private const val TAG = "MpvPlayerModule"
        const val EVENT_STATE_CHANGE = "mpv:state-change"
        const val EVENT_POSITION_UPDATE = "mpv:position-update"

        @Volatile
        private var _instance: MpvPlayerModule? = null
        fun getInstance(): MpvPlayerModule? = _instance
    }

    private val lynxContext = context as? LynxContext
    private val activity = lynxContext?.activity
    private val mainHandler = Handler(Looper.getMainLooper())
    private var mpvSurfaceView: MpvSurfaceView? = null
    private var isFullscreen = false
    private var mpvInitialized = false
    private var surfaceReady = false
    private var pendingUrl: String? = null

    private val eventObserver = object : MPVLib.EventObserver {
        override fun eventProperty(property: String) {}
        override fun eventProperty(property: String, value: Long) {}
        override fun eventProperty(property: String, value: Boolean) {
            when (property) {
                "pause" -> dispatchStateChange(if (value) "paused" else "playing")
                "idle" -> { if (value) dispatchStateChange("ended") }
            }
        }
        override fun eventProperty(property: String, value: Double) {}
        override fun eventProperty(property: String, value: String) {}
        override fun event(eventId: Int) {
            when (eventId) {
                MpvEvent.MPV_EVENT_FILE_LOADED -> dispatchStateChange("playing")
                MpvEvent.MPV_EVENT_END_FILE -> dispatchStateChange("ended")
                MpvEvent.MPV_EVENT_SHUTDOWN -> dispatchStateChange("ended")
            }
        }
    }

    init {
        _instance = this
        Log.d(TAG, "init: MpvPlayerModule created")
    }

    private fun ensureMpvInitialized() {
        if (mpvInitialized) return
        val act = activity ?: run {
            Log.e(TAG, "ensureMpvInitialized: activity is null")
            dispatchStateChange("error", "Activity not available")
            return
        }
        Log.d(TAG, "ensureMpvInitialized: initializing MPVLib")
        try {
            val configDir = act.filesDir.absolutePath + "/mpv"
            val cacheDir = act.cacheDir.absolutePath + "/mpv"

            java.io.File(configDir).mkdirs()
            java.io.File(cacheDir).mkdirs()

            MPVLib.create(act)
            MPVLib.setOptionString("config", "yes")
            MPVLib.setOptionString("config-dir", configDir)
            for (opt in arrayOf("gpu-shader-cache-dir", "icc-cache-dir")) {
                MPVLib.setOptionString(opt, cacheDir)
            }
            MPVLib.setOptionString("vo", "gpu")
            MPVLib.setOptionString("hwdec", "auto")
            MPVLib.init()

            MPVLib.setOptionString("force-window", "no")
            MPVLib.setOptionString("idle", "once")

            MPVLib.addObserver(eventObserver)
            MPVLib.observeProperty("pause", MpvFormat.MPV_FORMAT_FLAG)
            MPVLib.observeProperty("idle", MpvFormat.MPV_FORMAT_FLAG)
            mpvInitialized = true
            Log.d(TAG, "ensureMpvInitialized: done, configDir=$configDir, cacheDir=$cacheDir")
        } catch (e: Exception) {
            Log.e(TAG, "ensureMpvInitialized: failed", e)
            mpvInitialized = false
            dispatchStateChange("error", "MPV init failed: ${e.message}")
        }
    }

    fun attachToLayout(rootLayout: ViewGroup) {
        Log.d(TAG, "attachToLayout: adding MPV surface view to root layout (index 0)")
        try {
            ensureMpvInitialized()
            mpvSurfaceView = MpvSurfaceView(rootLayout.context).apply {
                id = ViewGroup.generateViewId()
                keepScreenOn = true
            }
            val params = ViewGroup.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.MATCH_PARENT
            )
            rootLayout.addView(mpvSurfaceView, 0, params)
            Log.d(TAG, "attachToLayout: MPV surface view attached")
        } catch (e: Exception) {
            Log.e(TAG, "attachToLayout: failed", e)
            dispatchStateChange("error", "Surface attach failed: ${e.message}")
        }
    }

    fun detachFromLayout(rootLayout: ViewGroup) {
        Log.d(TAG, "detachFromLayout: removing MPV surface view")
        try {
            mpvSurfaceView?.let { rootLayout.removeView(it) }
        } catch (e: Exception) {
            Log.e(TAG, "detachFromLayout: failed", e)
        }
    }

    fun release() {
        Log.d(TAG, "release: stopping MPV")
        if (mpvInitialized) {
            try {
                MPVLib.removeObserver(eventObserver)
            } catch (e: Exception) {
                Log.e(TAG, "release: removeObserver failed", e)
            }
            try {
                MPVLib.destroy()
            } catch (e: Exception) {
                Log.e(TAG, "release: MPVLib.destroy failed", e)
            }
            mpvInitialized = false
        }
        surfaceReady = false
        pendingUrl = null
        mpvSurfaceView = null
        _instance = null
    }

    @LynxMethod
    fun play(url: String, callback: Callback) {
        Log.d(TAG, "play: url=$url, surfaceReady=$surfaceReady, mpvInitialized=$mpvInitialized")
        try {
            ensureMpvInitialized()
            if (!mpvInitialized) {
                callback.invoke("MPV not initialized")
                return
            }
            if (surfaceReady) {
                MPVLib.command(arrayOf("loadfile", url))
            } else {
                Log.d(TAG, "play: surface not ready, queuing url as pending")
                pendingUrl = url
            }
            callback.invoke(true)
        } catch (e: Exception) {
            Log.e(TAG, "play failed", e)
            dispatchStateChange("error", "Play failed: ${e.message}")
            callback.invoke(e.message)
        }
    }

    @LynxMethod
    fun pause(callback: Callback) {
        Log.d(TAG, "pause")
        try {
            if (!mpvInitialized) { callback.invoke("MPV not initialized"); return }
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
            if (!mpvInitialized) { callback.invoke("MPV not initialized"); return }
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
            if (!mpvInitialized) { callback.invoke("MPV not initialized"); return }
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
            val act = activity ?: run { callback.invoke("Activity not available"); return }
            if (enabled) {
                act.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE
                act.window.addFlags(WindowManager.LayoutParams.FLAG_FULLSCREEN)
                @Suppress("DEPRECATION")
                act.window.decorView.systemUiVisibility = (
                    View.SYSTEM_UI_FLAG_LAYOUT_STABLE
                        or View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION
                        or View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
                        or View.SYSTEM_UI_FLAG_HIDE_NAVIGATION
                        or View.SYSTEM_UI_FLAG_FULLSCREEN
                        or View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY
                )
                isFullscreen = true
            } else {
                act.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_SENSOR_PORTRAIT
                act.window.clearFlags(WindowManager.LayoutParams.FLAG_FULLSCREEN)
                @Suppress("DEPRECATION")
                act.window.decorView.systemUiVisibility = View.SYSTEM_UI_FLAG_VISIBLE
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
            val act = activity ?: run { callback.invoke("Activity not available"); return }
            act.requestedOrientation = when (orientation) {
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
        try {
            if (!mpvInitialized) { callback.invoke(0); return }
            val durationSec = MPVLib.getPropertyDouble("duration") ?: 0.0
            callback.invoke((durationSec * 1000).toInt())
        } catch (e: Exception) {
            Log.e(TAG, "getDuration failed", e)
            callback.invoke(0)
        }
    }

    @LynxMethod
    fun getCurrentPosition(callback: Callback) {
        try {
            if (!mpvInitialized) { callback.invoke(0); return }
            val positionSec = MPVLib.getPropertyDouble("time-pos") ?: 0.0
            callback.invoke((positionSec * 1000).toInt())
        } catch (e: Exception) {
            Log.e(TAG, "getCurrentPosition failed", e)
            callback.invoke(0)
        }
    }

    @LynxMethod
    fun isPlaying(callback: Callback) {
        try {
            if (!mpvInitialized) { callback.invoke(false); return }
            val paused = MPVLib.getPropertyBoolean("pause") ?: true
            callback.invoke(!paused)
        } catch (e: Exception) {
            Log.e(TAG, "isPlaying failed", e)
            callback.invoke(false)
        }
    }

    @LynxMethod
    fun setProperty(key: String, value: String, callback: Callback) {
        Log.d(TAG, "setProperty: key=$key, value=$value")
        try {
            if (!mpvInitialized) { callback.invoke("MPV not initialized"); return }
            MPVLib.setPropertyString(key, value)
            callback.invoke(true)
        } catch (e: Exception) {
            Log.e(TAG, "setProperty failed", e)
            callback.invoke(e.message)
        }
    }

    fun dispatchPositionUpdate() {
        try {
            if (!mpvInitialized || !surfaceReady) return
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
            mainHandler.post {
                try {
                    lynxContext?.sendGlobalEvent(EVENT_POSITION_UPDATE, params)
                } catch (e: Exception) {
                    Log.e(TAG, "dispatchPositionUpdate sendGlobalEvent failed", e)
                }
            }
        } catch (e: Exception) {
            Log.e(TAG, "dispatchPositionUpdate failed", e)
        }
    }

    private fun dispatchStateChange(state: String, error: String? = null) {
        try {
            val data = JavaOnlyMap().apply {
                put("state", state)
                if (error != null) put("error", error)
            }
            val params = JavaOnlyArray()
            params.pushMap(data)
            mainHandler.post {
                try {
                    lynxContext?.sendGlobalEvent(EVENT_STATE_CHANGE, params)
                } catch (e: Exception) {
                    Log.e(TAG, "dispatchStateChange sendGlobalEvent failed", e)
                }
            }
        } catch (e: Exception) {
            Log.e(TAG, "dispatchStateChange failed", e)
        }
    }

    private inner class MpvSurfaceView(context: android.content.Context) :
        SurfaceView(context), SurfaceHolder.Callback {

        init {
            holder.addCallback(this)
        }

        override fun surfaceCreated(holder: SurfaceHolder) {
            Log.d(TAG, "MpvSurfaceView: surfaceCreated, attaching surface")
            try {
                MPVLib.attachSurface(holder.surface)
                MPVLib.setOptionString("force-window", "yes")
                MPVLib.setPropertyString("vo", "gpu")
                surfaceReady = true
                pendingUrl?.let { url ->
                    Log.d(TAG, "MpvSurfaceView: playing pending url=$url")
                    MPVLib.command(arrayOf("loadfile", url))
                    pendingUrl = null
                }
            } catch (e: Exception) {
                Log.e(TAG, "MpvSurfaceView: surfaceCreated failed", e)
                surfaceReady = false
                dispatchStateChange("error", "Surface create failed: ${e.message}")
            }
        }

        override fun surfaceChanged(holder: SurfaceHolder, format: Int, width: Int, height: Int) {
            try {
                MPVLib.setPropertyString("android-surface-size", "${width}x$height")
            } catch (e: Exception) {
                Log.e(TAG, "MpvSurfaceView: surfaceChanged failed", e)
            }
        }

        override fun surfaceDestroyed(holder: SurfaceHolder) {
            Log.d(TAG, "MpvSurfaceView: surfaceDestroyed, detaching surface")
            try {
                MPVLib.setPropertyString("vo", "null")
                MPVLib.setPropertyString("force-window", "no")
                MPVLib.detachSurface()
            } catch (e: Exception) {
                Log.e(TAG, "MpvSurfaceView: surfaceDestroyed failed", e)
            }
            surfaceReady = false
        }
    }
}
