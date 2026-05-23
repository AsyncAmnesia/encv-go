package com.encvgo.app

import android.os.Handler
import android.os.Looper
import android.util.Log
import android.view.ViewGroup
import android.widget.FrameLayout
import com.lynx.tasm.LynxView
import com.lynx.tasm.LynxViewBuilder
import com.lynx.tasm.LynxViewClient
import org.json.JSONObject

class PlayerOverlayManager private constructor() {
    companion object {
        private const val TAG = "PlayerOverlay"

        @Volatile
        private var _instance: PlayerOverlayManager? = null
        fun getInstance(): PlayerOverlayManager =
            _instance ?: synchronized(this) {
                _instance ?: PlayerOverlayManager().also { _instance = it }
            }
    }

    private var overlayLayout: FrameLayout? = null
    private var lynxView: LynxView? = null
    private var activity: MainActivity? = null
    private val mainHandler = Handler(Looper.getMainLooper())
    private var isShowing = false

    private val positionUpdateRunnable = object : Runnable {
        override fun run() {
            try {
                MpvPlayerModule.getInstance()?.dispatchPositionUpdate()
            } catch (e: Exception) {
                LogRelay.get().relay(TAG, "error", "positionUpdateRunnable failed: ${e.message}")
            }
            lynxView?.postDelayed(this, 500)
        }
    }

    fun showOverlay(
        activity: MainActivity,
        streamUrl: String,
        fileName: String,
        mimeType: String,
        isExternal: Boolean
    ) {
        if (isShowing) {
            LogRelay.get().relay(TAG, "warn", "showOverlay: already showing, ignoring")
            return
        }

        this.activity = activity
        LogRelay.get().relay(TAG, "info", "showOverlay: streamUrl=$streamUrl, name=$fileName, mimeType=$mimeType, external=$isExternal")

        mainHandler.post {
            try {
                EncvApplication.ensureLynxInitialized(activity.application)
                MpvPlayerModule.preInit(activity)

                val decorView = activity.window.decorView as? ViewGroup
                if (decorView == null) {
                    LogRelay.get().relay(TAG, "error", "showOverlay: decorView is null")
                    return@post
                }

                overlayLayout = FrameLayout(activity).apply {
                    id = R.id.player_overlay_root
                    layoutParams = ViewGroup.LayoutParams(
                        ViewGroup.LayoutParams.MATCH_PARENT,
                        ViewGroup.LayoutParams.MATCH_PARENT
                    )
                    setBackgroundColor(0xFF000000.toInt())
                }
                decorView.addView(overlayLayout)
                LogRelay.get().relay(TAG, "info", "showOverlay: overlay FrameLayout added to decorView")

                createLynxView(streamUrl, fileName, mimeType, isExternal)

                isShowing = true
                LogRelay.get().relay(TAG, "info", "showOverlay: complete")
            } catch (e: Exception) {
                LogRelay.get().relay(TAG, "error", "showOverlay: failed: ${e.message}")
                cleanupOverlay()
            }
        }
    }

    fun hideOverlay() {
        if (!isShowing) {
            LogRelay.get().relay(TAG, "info", "hideOverlay: not showing, ignoring")
            return
        }
        LogRelay.get().relay(TAG, "info", "hideOverlay: removing overlay")
        mainHandler.post {
            cleanupOverlay()
        }
    }

    fun isOverlayShowing(): Boolean = isShowing

    private fun createLynxView(
        streamUrl: String,
        fileName: String,
        mimeType: String,
        isExternal: Boolean
    ) {
        val act = activity ?: return
        val root = overlayLayout ?: return

        LogRelay.get().relay(TAG, "info", "createLynxView: START")
        try {
            val viewBuilder = LynxViewBuilder()
            viewBuilder.setTemplateProvider(PlayerTemplateProvider(act))
            viewBuilder.registerModule("MpvPlayerModule", MpvPlayerModule::class.java)
            viewBuilder.registerModule("GoBackendModule", GoBackendModule::class.java)
            viewBuilder.registerModule("LogBridge", LogBridgeModule::class.java)

            val displayMetrics = act.resources.displayMetrics
            viewBuilder.setScreenSize(displayMetrics.widthPixels, displayMetrics.heightPixels)

            lynxView = viewBuilder.build(act)
            lynxView?.setBackgroundColor(0)

            lynxView?.addLynxViewClient(object : LynxViewClient() {
                private val CLIENT_TAG = "PlayerOverlayClient"

                override fun onRuntimeReady() {
                    LogRelay.get().relay(CLIENT_TAG, "info", "onRuntimeReady")
                    tryAttachMpvModule()
                }

                override fun onLoadSuccess() {
                    LogRelay.get().relay(CLIENT_TAG, "info", "onLoadSuccess")
                    tryAttachMpvModule()
                }

                override fun onLoadFailed(message: String) {
                    LogRelay.get().relay(CLIENT_TAG, "error", "onLoadFailed: $message")
                }

                override fun onFirstScreen() {
                    LogRelay.get().relay(CLIENT_TAG, "info", "onFirstScreen")
                }
            })

            val lynxParams = ViewGroup.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.MATCH_PARENT
            )
            root.addView(lynxView, lynxParams)
            LogRelay.get().relay(TAG, "info", "createLynxView: LynxView added to overlay (on top of MPV surface)")

            val initData = buildInitDataJson(streamUrl, fileName, mimeType, isExternal)
            LogRelay.get().relay(TAG, "info", "createLynxView: initData=$initData")
            lynxView?.renderTemplateUrl("player.lynx.bundle", initData)

            lynxView?.post(positionUpdateRunnable)
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "createLynxView: failed: ${e.message}")
        }
    }

    private fun tryAttachMpvModule() {
        val mpvModule = MpvPlayerModule.getInstance()
        if (mpvModule == null) {
            LogRelay.get().relay(TAG, "info", "tryAttachMpvModule: mpvModule not yet created")
            return
        }
        if (mpvModule.isAttached()) {
            LogRelay.get().relay(TAG, "info", "tryAttachMpvModule: already attached")
            return
        }
        val root = overlayLayout ?: return
        mainHandler.post {
            if (!mpvModule.isAttached()) {
                mpvModule.attachToLayout(root)
            }
        }
    }

    private fun buildInitDataJson(
        streamUrl: String,
        fileName: String,
        mimeType: String,
        isExternal: Boolean
    ): String {
        val mediaType = when {
            mimeType.startsWith("audio/") -> "audio"
            mimeType.startsWith("video/") -> "video"
            mimeType.isEmpty() -> {
                val ext = fileName.substringAfterLast('.', "").lowercase()
                when (ext) {
                    "mp3", "flac", "wav", "ogg", "aac", "m4a", "wma", "opus", "ape", "alac" -> "audio"
                    else -> "video"
                }
            }
            else -> "video"
        }
        return JSONObject().apply {
            put("streamUrl", streamUrl)
            put("fileName", fileName)
            put("mimeType", mimeType)
            put("isExternal", isExternal)
            put("mediaType", mediaType)
        }.toString()
    }

    private fun cleanupOverlay() {
        try {
            lynxView?.removeCallbacks(positionUpdateRunnable)

            MpvPlayerModule.getInstance()?.let { mpvModule ->
                mpvModule.detachFromLayout(overlayLayout ?: FrameLayout(activity ?: return@let))
                mpvModule.release()
            }

            GoBackendModule.getInstance()?.unregisterReceiver()

            lynxView?.destroy()
            lynxView = null

            overlayLayout?.let { layout ->
                (layout.parent as? ViewGroup)?.removeView(layout)
            }
            overlayLayout = null

            isShowing = false
            activity = null
            LogRelay.get().relay(TAG, "info", "cleanupOverlay: complete")
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "cleanupOverlay: error: ${e.message}")
            isShowing = false
        }
    }
}
