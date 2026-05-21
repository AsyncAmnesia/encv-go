package com.encvgo.app

import android.app.Activity
import android.content.pm.ActivityInfo
import android.util.Log
import android.view.ViewGroup
import android.view.WindowManager
import com.lynx.tasm.LynxView
import com.lynx.tasm.behavior.LynxModule
import com.lynx.tasm.behavior.LynxModuleMethod
import com.lynx.tasm.behavior.LynxPromise
import io.github.abdallahmehiz.mpvlib.MPVLib
import io.github.abdallahmehiz.mpvlib.MPVView
import org.json.JSONObject

class MpvPlayerModule(lynxView: LynxView) : LynxModule(lynxView) {
    companion object {
        private const val TAG = "MpvPlayerModule"
        const val EVENT_STATE_CHANGE = "mpv:state-change"
        const val EVENT_POSITION_UPDATE = "mpv:position-update"
    }

    private val activity = lynxView.context as Activity
    private var mpvView: MPVView? = null
    private var isFullscreen = false
    private var lastOrientationLock = ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED

    init {
        Log.d(TAG, "init: creating MPVView")
        MPVLib.init(activity.application)
        mpvView = MPVView(activity).apply {
            id = ViewGroup.generateViewId()
            keepScreenOn = true
            setListener(object : MPVLib.EventListener {
                override fun onEvent(event: MPVLib.Event) {
                    Log.d(TAG, "mpv event: $event")
                    when (event) {
                        is MPVLib.Event.FileLoaded -> {
                            dispatchStateChange("playing")
                        }
                        is MPVLib.Event.EndFile -> {
                            dispatchStateChange("end-file")
                        }
                        is MPVLib.Event.Pause -> {
                            dispatchStateChange(if (event.pause) "paused" else "playing")
                        }
                        is MPVLib.Event.Error -> {
                            Log.e(TAG, "mpv error: ${event.message}")
                            dispatchStateChange("error", event.message)
                        }
                        else -> {
                        }
                    }
                }
            })
        }
        Log.d(TAG, "init: MPVView created")
    }

    fun attachToLayout(rootLayout: ViewGroup) {
        Log.d(TAG, "attachToLayout: adding MPVView to root layout (index 0)")
        val params = ViewGroup.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT,
            ViewGroup.LayoutParams.MATCH_PARENT
        )
        rootLayout.addView(mpvView, 0, params)
        Log.d(TAG, "attachToLayout: MPVView attached")
    }

    fun detachFromLayout(rootLayout: ViewGroup) {
        Log.d(TAG, "detachFromLayout: removing MPVView")
        rootLayout.removeView(mpvView)
    }

    fun release() {
        Log.d(TAG, "release: stopping MPVView")
        mpvView?.destroy()
        mpvView = null
    }

    @LynxModuleMethod
    fun play(params: Map<String, Any>, promise: LynxPromise) {
        val url = params["url"] as? String ?: run {
            promise.reject("url is required")
            return
        }
        Log.d(TAG, "play: url=$url")
        try {
            mpvView?.loadUrl(url)
            promise.resolve(true)
        } catch (e: Exception) {
            Log.e(TAG, "play failed", e)
            promise.reject(e.message)
        }
    }

    @LynxModuleMethod
    fun pause(params: Map<String, Any>, promise: LynxPromise) {
        Log.d(TAG, "pause")
        try {
            mpvView?.setPause(true)
            promise.resolve(true)
        } catch (e: Exception) {
            Log.e(TAG, "pause failed", e)
            promise.reject(e.message)
        }
    }

    @LynxModuleMethod
    fun resume(params: Map<String, Any>, promise: LynxPromise) {
        Log.d(TAG, "resume")
        try {
            mpvView?.setPause(false)
            promise.resolve(true)
        } catch (e: Exception) {
            Log.e(TAG, "resume failed", e)
            promise.reject(e.message)
        }
    }

    @LynxModuleMethod
    fun seekTo(params: Map<String, Any>, promise: LynxPromise) {
        val positionMs = params["positionMs"] as? Int ?: 0
        Log.d(TAG, "seekTo: $positionMs ms")
        try {
            mpvView?.seekTo(positionMs)
            promise.resolve(true)
        } catch (e: Exception) {
            Log.e(TAG, "seekTo failed", e)
            promise.reject(e.message)
        }
    }

    @LynxModuleMethod
    fun setFullscreen(params: Map<String, Any>, promise: LynxPromise) {
        val enabled = params["enabled"] as? Boolean ?: false
        Log.d(TAG, "setFullscreen: $enabled")
        try {
            if (enabled) {
                lastOrientationLock = activity.requestedOrientation
                val ratio = mpvView?.videoWidth?.toFloat() ?: 1f
                val targetOrientation = if (ratio > 1.3) {
                    ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE
                } else {
                    ActivityInfo.SCREEN_ORIENTATION_SENSOR_PORTRAIT
                }
                activity.requestedOrientation = targetOrientation
                activity.window.addFlags(WindowManager.LayoutParams.FLAG_FULLSCREEN)
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
                activity.window.decorView.systemUiVisibility = View.SYSTEM_UI_FLAG_VISIBLE
                isFullscreen = false
            }
            promise.resolve(true)
        } catch (e: Exception) {
            Log.e(TAG, "setFullscreen failed", e)
            promise.reject(e.message)
        }
    }

    @LynxModuleMethod
    fun setOrientation(params: Map<String, Any>, promise: LynxPromise) {
        val orientation = params["orientation"] as? String ?: "unlocked"
        Log.d(TAG, "setOrientation: $orientation")
        try {
            activity.requestedOrientation = when (orientation) {
                "landscape" -> ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE
                "portrait" -> ActivityInfo.SCREEN_ORIENTATION_SENSOR_PORTRAIT
                else -> ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED
            }
            promise.resolve(true)
        } catch (e: Exception) {
            Log.e(TAG, "setOrientation failed", e)
            promise.reject(e.message)
        }
    }

    @LynxModuleMethod
    fun getDuration(params: Map<String, Any>, promise: LynxPromise) {
        val durationMs = mpvView?.duration ?: 0
        Log.d(TAG, "getDuration: $durationMs ms")
        promise.resolve(durationMs)
    }

    @LynxModuleMethod
    fun getCurrentPosition(params: Map<String, Any>, promise: LynxPromise) {
        val positionMs = mpvView?.currentPosition ?: 0
        Log.d(TAG, "getCurrentPosition: $positionMs ms")
        promise.resolve(positionMs)
    }

    @LynxModuleMethod
    fun isPlaying(params: Map<String, Any>, promise: LynxPromise) {
        val playing = mpvView?.isPlaying ?: false
        Log.d(TAG, "isPlaying: $playing")
        promise.resolve(playing)
    }

    @LynxModuleMethod
    fun setProperty(params: Map<String, Any>, promise: LynxPromise) {
        val key = params["key"] as? String ?: run {
            promise.reject("key is required")
            return
        }
        val value = params["value"] as? String ?: ""
        Log.d(TAG, "setProperty: key=$key, value=$value")
        try {
            mpvView?.setProperty(key, value)
            promise.resolve(true)
        } catch (e: Exception) {
            Log.e(TAG, "setProperty failed", e)
            promise.reject(e.message)
        }
    }

    fun dispatchPositionUpdate() {
        val position = mpvView?.currentPosition ?: 0
        val duration = mpvView?.duration ?: 0
        val data = JSONObject().apply {
            put("position", position)
            put("duration", duration)
        }
        lynxView.dispatchEvent(EVENT_POSITION_UPDATE, data)
    }

    private fun dispatchStateChange(state: String, error: String? = null) {
        val data = JSONObject().apply {
            put("state", state)
            if (error != null) put("error", error)
        }
        lynxView.dispatchEvent(EVENT_STATE_CHANGE, data)
    }
}
