package com.encvgo.app

import android.app.Activity
import android.content.pm.ActivityInfo
import android.os.Handler
import android.os.Looper
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

        @Volatile
        private var preInitialized = false

        fun preInit(context: android.content.Context) {
            if (preInitialized) return
            if (Looper.myLooper() != Looper.getMainLooper()) {
                LogRelay.get().relay(TAG, "error", "preInit: must be on main thread! current=${Thread.currentThread().name}")
                return
            }
            try {
                val configDir = context.filesDir.absolutePath + "/mpv"
                val cacheDir = context.cacheDir.absolutePath + "/mpv"
                java.io.File(configDir).mkdirs()
                java.io.File(cacheDir).mkdirs()
                MPVLib.create(context)
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
                preInitialized = true
                LogRelay.get().relay(TAG, "info", "preInit: MPV engine initialized on main thread")
            } catch (e: Exception) {
                LogRelay.get().relay(TAG, "error", "preInit: failed: ${e.message}")
            }
        }
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
                MpvEvent.MPV_EVENT_FILE_LOADED -> {
                    val videoWidth = try { MPVLib.getPropertyInt("width") ?: 0 } catch (_: Exception) { 0 }
                    val videoHeight = try { MPVLib.getPropertyInt("height") ?: 0 } catch (_: Exception) { 0 }
                    val isAudioOnly = videoWidth == 0 || videoHeight == 0
                    if (isAudioOnly) {
                        mainHandler.post { mpvSurfaceView?.visibility = View.GONE }
                        dispatchStateChange("audio_only")
                    } else {
                        mainHandler.post { mpvSurfaceView?.visibility = View.VISIBLE }
                        dispatchStateChange("playing")
                    }
                }
                MpvEvent.MPV_EVENT_END_FILE -> dispatchStateChange("ended")
                MpvEvent.MPV_EVENT_SHUTDOWN -> dispatchStateChange("ended")
            }
        }
    }

    init {
        _instance = this
        LogRelay.get().relay(TAG, "info", "init: MpvPlayerModule created on thread: ${Thread.currentThread().name}")
        mainHandler.post {
            try {
                if (isAttached()) {
                    LogRelay.get().relay(TAG, "info", "init auto-attach: already attached, skipping")
                    return@post
                }
                val act = activity
                if (act == null) {
                    LogRelay.get().relay(TAG, "error", "init auto-attach: activity is null")
                    return@post
                }
                val root = act.findViewById<android.widget.FrameLayout>(R.id.lynx_player_root)
                if (root == null) {
                    LogRelay.get().relay(TAG, "error", "init auto-attach: lynx_player_root not found")
                    return@post
                }
                attachToLayout(root)
                LogRelay.get().relay(TAG, "info", "init auto-attach: surface attached successfully")
            } catch (e: Exception) {
                LogRelay.get().relay(TAG, "error", "init auto-attach failed: ${e.message}")
            }
        }
    }

    private fun ensureMpvInitialized() {
        if (mpvInitialized) return
        if (preInitialized) {
            mpvInitialized = true
            MPVLib.addObserver(eventObserver)
            MPVLib.observeProperty("pause", MpvFormat.MPV_FORMAT_FLAG)
            MPVLib.observeProperty("idle", MpvFormat.MPV_FORMAT_FLAG)
            dispatchStateChange("mpv_ready")
            LogRelay.get().relay(TAG, "info", "ensureMpvInitialized: using pre-initialized MPV engine")
            return
        }
        if (Looper.myLooper() != Looper.getMainLooper()) {
            LogRelay.get().relay(TAG, "error", "ensureMpvInitialized: must be on main thread! current=${Thread.currentThread().name}")
            return
        }
        val act = activity ?: run {
            LogRelay.get().relay(TAG, "error", "ensureMpvInitialized: activity is null")
            dispatchStateChange("error", "Activity not available")
            return
        }
        LogRelay.get().relay(TAG, "info", "ensureMpvInitialized: initializing MPVLib")
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
            dispatchStateChange("mpv_ready")
            LogRelay.get().relay(TAG, "info", "ensureMpvInitialized: done, configDir=$configDir, cacheDir=$cacheDir")
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "ensureMpvInitialized: failed: ${e.message}")
            mpvInitialized = false
            dispatchStateChange("error", "MPV init failed: ${e.message}")
        }
    }

    fun attachToLayout(rootLayout: ViewGroup) {
        if (mpvSurfaceView != null && mpvSurfaceView?.parent != null) {
            LogRelay.get().relay(TAG, "info", "attachToLayout: already attached, skipping")
            return
        }
        LogRelay.get().relay(TAG, "info", "attachToLayout: adding MPV surface view to root layout (index 0)")
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
            mpvSurfaceView?.visibility = android.view.View.VISIBLE
            LogRelay.get().relay(TAG, "info", "attachToLayout: MPV surface view attached (VISIBLE)")
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "attachToLayout: failed: ${e.message}")
            dispatchStateChange("error", "Surface attach failed: ${e.message}")
        }
    }

    fun isAttached(): Boolean = mpvSurfaceView != null && mpvSurfaceView?.parent != null

    fun detachFromLayout(rootLayout: ViewGroup) {
        LogRelay.get().relay(TAG, "info", "detachFromLayout: removing MPV surface view")
        try {
            mpvSurfaceView?.let { rootLayout.removeView(it) }
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "detachFromLayout: failed: ${e.message}")
        }
    }

    fun release() {
        LogRelay.get().relay(TAG, "info", "release: stopping MPV")
        if (mpvInitialized) {
            try {
                MPVLib.removeObserver(eventObserver)
            } catch (e: Exception) {
                LogRelay.get().relay(TAG, "error", "release: removeObserver failed: ${e.message}")
            }
            try {
                MPVLib.destroy()
            } catch (e: Exception) {
                LogRelay.get().relay(TAG, "error", "release: MPVLib.destroy failed: ${e.message}")
            }
            mpvInitialized = false
        }
        preInitialized = false
        surfaceReady = false
        pendingUrl = null
        mpvSurfaceView = null
        _instance = null
    }

    @LynxMethod
    fun play(url: String, callback: Callback) {
        LogRelay.get().relay(TAG, "info", "play: url=$url, surfaceReady=$surfaceReady, mpvInitialized=$mpvInitialized, preInitialized=$preInitialized")
        try {
            ensureMpvInitialized()
            if (!mpvInitialized) {
                if (!preInitialized) {
                    LogRelay.get().relay(TAG, "info", "play: MPV not pre-initialized, scheduling delayed init on main thread")
                    mainHandler.post {
                        try {
                            ensureMpvInitialized()
                            if (mpvInitialized && surfaceReady) {
                                MPVLib.command(arrayOf("loadfile", url))
                            } else if (mpvInitialized) {
                                pendingUrl = url
                                dispatchStateChange("waiting_surface")
                            }
                        } catch (e: Exception) {
                            LogRelay.get().relay(TAG, "error", "play delayed init failed: ${e.message}")
                        }
                    }
                    callback.invoke("MPV initializing, will play when ready")
                    return
                }
                callback.invoke("MPV not initialized")
                return
            }
            if (surfaceReady) {
                LogRelay.get().relay(TAG, "info", "play: calling loadfile with url=$url")
                MPVLib.command(arrayOf("loadfile", url))
            } else {
                LogRelay.get().relay(TAG, "info", "play: surface not ready, queuing url as pending")
                pendingUrl = url
                dispatchStateChange("waiting_surface")
            }
            callback.invoke(true)
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "play failed: ${e.message}")
            dispatchStateChange("error", "Play failed: ${e.message}")
            callback.invoke(e.message)
        }
    }

    @LynxMethod
    fun pause(callback: Callback) {
        LogRelay.get().relay(TAG, "info", "pause")
        try {
            if (!mpvInitialized) { callback.invoke("MPV not initialized"); return }
            MPVLib.setPropertyBoolean("pause", true)
            callback.invoke(true)
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "pause failed: ${e.message}")
            callback.invoke(e.message)
        }
    }

    @LynxMethod
    fun resume(callback: Callback) {
        LogRelay.get().relay(TAG, "info", "resume")
        try {
            if (!mpvInitialized) { callback.invoke("MPV not initialized"); return }
            MPVLib.setPropertyBoolean("pause", false)
            callback.invoke(true)
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "resume failed: ${e.message}")
            callback.invoke(e.message)
        }
    }

    @LynxMethod
    fun seekTo(positionMs: Int, callback: Callback) {
        LogRelay.get().relay(TAG, "info", "seekTo: $positionMs ms")
        try {
            if (!mpvInitialized) { callback.invoke("MPV not initialized"); return }
            val positionSec = positionMs / 1000.0
            MPVLib.command(arrayOf("seek", positionSec.toString(), "absolute"))
            callback.invoke(true)
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "seekTo failed: ${e.message}")
            callback.invoke(e.message)
        }
    }

    @LynxMethod
    fun setFullscreen(enabled: Boolean, callback: Callback) {
        LogRelay.get().relay(TAG, "info", "setFullscreen: $enabled")
        val act = activity ?: run { callback.invoke("Activity not available"); return }
        mainHandler.post {
            try {
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
                LogRelay.get().relay(TAG, "error", "setFullscreen failed: ${e.message}")
                callback.invoke(e.message)
            }
        }
    }

    @LynxMethod
    fun setOrientation(orientation: String, callback: Callback) {
        LogRelay.get().relay(TAG, "info", "setOrientation: $orientation")
        val act = activity ?: run { callback.invoke("Activity not available"); return }
        mainHandler.post {
            try {
                act.requestedOrientation = when (orientation) {
                    "landscape" -> ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE
                    "portrait" -> ActivityInfo.SCREEN_ORIENTATION_SENSOR_PORTRAIT
                    else -> ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED
                }
                callback.invoke(true)
            } catch (e: Exception) {
                LogRelay.get().relay(TAG, "error", "setOrientation failed: ${e.message}")
                callback.invoke(e.message)
            }
        }
    }

    @LynxMethod
    fun finish(callback: Callback) {
        LogRelay.get().relay(TAG, "info", "finish: closing player activity")
        val act = activity ?: run { callback.invoke("Activity not available"); return }
        mainHandler.post {
            try {
                act.finish()
                callback.invoke(true)
            } catch (e: Exception) {
                LogRelay.get().relay(TAG, "error", "finish failed: ${e.message}")
                callback.invoke(e.message)
            }
        }
    }

    @LynxMethod
    fun getDuration(callback: Callback) {
        try {
            if (!mpvInitialized) { callback.invoke(0); return }
            val durationSec = MPVLib.getPropertyDouble("duration") ?: 0.0
            callback.invoke((durationSec * 1000).toInt())
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "getDuration failed: ${e.message}")
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
            LogRelay.get().relay(TAG, "error", "getCurrentPosition failed: ${e.message}")
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
            LogRelay.get().relay(TAG, "error", "isPlaying failed: ${e.message}")
            callback.invoke(false)
        }
    }

    @LynxMethod
    fun setProperty(key: String, value: String, callback: Callback) {
        LogRelay.get().relay(TAG, "info", "setProperty: key=$key, value=$value")
        try {
            if (!mpvInitialized) { callback.invoke("MPV not initialized"); return }
            MPVLib.setPropertyString(key, value)
            callback.invoke(true)
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "setProperty failed: ${e.message}")
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
                    LogRelay.get().relay(TAG, "error", "dispatchPositionUpdate sendGlobalEvent failed: ${e.message}")
                }
            }
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "dispatchPositionUpdate failed: ${e.message}")
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
            LogRelay.get().relay(TAG, "info", "dispatchStateChange: state=$state${if (error != null) " error=$error" else ""}")
            mainHandler.post {
                try {
                    lynxContext?.sendGlobalEvent(EVENT_STATE_CHANGE, params)
                } catch (e: Exception) {
                    LogRelay.get().relay(TAG, "error", "dispatchStateChange sendGlobalEvent failed: ${e.message}")
                }
            }
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "dispatchStateChange failed: ${e.message}")
        }
    }

    private inner class MpvSurfaceView(context: android.content.Context) :
        SurfaceView(context), SurfaceHolder.Callback {

        init {
            holder.addCallback(this)
        }

        override fun surfaceCreated(holder: SurfaceHolder) {
            LogRelay.get().relay(TAG, "info", "MpvSurfaceView: surfaceCreated, attaching surface")
            try {
                MPVLib.attachSurface(holder.surface)
                MPVLib.setOptionString("force-window", "yes")
                MPVLib.setPropertyString("vo", "gpu")
                surfaceReady = true
                dispatchStateChange("surface_ready")
                pendingUrl?.let { url ->
                    LogRelay.get().relay(TAG, "info", "MpvSurfaceView: playing pending url=$url")
                    MPVLib.command(arrayOf("loadfile", url))
                    pendingUrl = null
                }
            } catch (e: Exception) {
                LogRelay.get().relay(TAG, "error", "MpvSurfaceView: surfaceCreated failed: ${e.message}")
                surfaceReady = false
                dispatchStateChange("error", "Surface create failed: ${e.message}")
            }
        }

        override fun surfaceChanged(holder: SurfaceHolder, format: Int, width: Int, height: Int) {
            try {
                MPVLib.setPropertyString("android-surface-size", "${width}x$height")
            } catch (e: Exception) {
                LogRelay.get().relay(TAG, "error", "MpvSurfaceView: surfaceChanged failed: ${e.message}")
            }
        }

        override fun surfaceDestroyed(holder: SurfaceHolder) {
            LogRelay.get().relay(TAG, "info", "MpvSurfaceView: surfaceDestroyed, detaching surface")
            try {
                MPVLib.setPropertyString("vo", "null")
                MPVLib.setPropertyString("force-window", "no")
                MPVLib.detachSurface()
            } catch (e: Exception) {
                LogRelay.get().relay(TAG, "error", "MpvSurfaceView: surfaceDestroyed failed: ${e.message}")
            }
            surfaceReady = false
        }
    }
}
