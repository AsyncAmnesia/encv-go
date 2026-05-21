package com.encvgo.app

import android.app.Activity
import android.content.pm.ActivityInfo
import android.util.Log
import android.view.View
import android.view.ViewGroup
import android.view.WindowManager
import com.lynx.jsbridge.LynxMethod
import com.lynx.jsbridge.LynxModule
import com.lynx.react.bridge.Callback
import com.lynx.tasm.behavior.LynxContext
import io.github.abdallahmehiz.mpvlib.MPVLib
import io.github.abdallahmehiz.mpvlib.MPVView
import org.json.JSONObject

class MpvPlayerModule(context: android.content.Context) : LynxModule(context) {
    companion object {
        private const val TAG = "MpvPlayerModule"
        const val EVENT_STATE_CHANGE = "mpv:state-change"
        const val EVENT_POSITION_UPDATE = "mpv:position-update"
    }

    private val lynxContext = context as LynxContext
    private val activity = lynxContext.context as Activity
    private var mpvView: MPVView? = null
    private var isFullscreen = false

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

    @LynxMethod
    fun play(url: String, callback: Callback) {
        Log.d(TAG, "play: url=$url")
        try {
            mpvView?.loadUrl(url)
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
            mpvView?.setPause(true)
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
            mpvView?.setPause(false)
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
            mpvView?.seekTo(positionMs)
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
        val durationMs = mpvView?.duration ?: 0
        Log.d(TAG, "getDuration: $durationMs ms")
        callback.invoke(durationMs)
    }

    @LynxMethod
    fun getCurrentPosition(callback: Callback) {
        val positionMs = mpvView?.currentPosition ?: 0
        Log.d(TAG, "getCurrentPosition: $positionMs ms")
        callback.invoke(positionMs)
    }

    @LynxMethod
    fun isPlaying(callback: Callback) {
        val playing = mpvView?.isPlaying ?: false
        Log.d(TAG, "isPlaying: $playing")
        callback.invoke(playing)
    }

    @LynxMethod
    fun setProperty(key: String, value: String, callback: Callback) {
        Log.d(TAG, "setProperty: key=$key, value=$value")
        try {
            mpvView?.setProperty(key, value)
            callback.invoke(true)
        } catch (e: Exception) {
            Log.e(TAG, "setProperty failed", e)
            callback.invoke(e.message)
        }
    }

    fun dispatchPositionUpdate() {
        val position = mpvView?.currentPosition ?: 0
        val duration = mpvView?.duration ?: 0
        val data = JSONObject().apply {
            put("position", position)
            put("duration", duration)
        }
        lynxContext.dispatchEvent(EVENT_POSITION_UPDATE, data)
    }

    private fun dispatchStateChange(state: String, error: String? = null) {
        val data = JSONObject().apply {
            put("state", state)
            if (error != null) put("error", error)
        }
        lynxContext.dispatchEvent(EVENT_STATE_CHANGE, data)
    }
}
