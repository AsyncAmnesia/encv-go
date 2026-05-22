package com.encvgo.app

import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.provider.OpenableColumns
import android.view.ViewGroup
import android.widget.FrameLayout
import androidx.appcompat.app.AppCompatActivity
import com.lynx.tasm.LynxView
import com.lynx.tasm.LynxViewBuilder
import com.lynx.tasm.LynxViewClient
import org.json.JSONObject
import java.io.File
import java.io.FileOutputStream

class PlayerActivityLynx : AppCompatActivity() {
    companion object {
        private const val TAG = "PlayerActivityLynx"
    }

    private var intentFilePath = ""
    private var intentFileName = ""
    private var intentFileMimeType = ""
    private var isExternalFile = false

    private var rootLayout: FrameLayout? = null
    private var lynxView: LynxView? = null

    private var backendReceiverRegistered = false
    private val backendReceiver = object : android.content.BroadcastReceiver() {
        override fun onReceive(ctx: android.content.Context?, intent: Intent?) {
            when (intent?.action) {
                EncvGoService.BROADCAST_BACKEND_READY,
                EncvGoService.BROADCAST_BACKEND_STATUS -> {
                    LogRelay.get().relay(TAG, "info", "backend broadcast: ${intent.action}")
                }
            }
        }
    }

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

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        LogRelay.get().relay(TAG, "info", "onCreate: starting")

        EncvApplication.ensureLynxInitialized(application)

        setContentView(R.layout.lynx_player_activity)
        rootLayout = findViewById(R.id.lynx_player_root)
        LogRelay.get().relay(TAG, "info", "onCreate: root layout found: $rootLayout")

        MpvPlayerModule.preInit(this)
        LogRelay.get().relay(TAG, "info", "onCreate: MPV preInit done")

        resolveFileInfo(intent)
        LogRelay.get().relay(TAG, "info", "onCreate: file info resolved, path=$intentFilePath, name=$intentFileName, mimeType=$intentFileMimeType, external=$isExternalFile")

        createLynxView()
        handleBackend()

        LogRelay.get().relay(TAG, "info", "onCreate: setup complete")
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        LogRelay.get().relay(TAG, "info", "onNewIntent: new intent received")
        setIntent(intent)
        resolveFileInfo(intent)
        val initData = buildInitDataJson()
        lynxView?.renderTemplateUrl("player.lynx.bundle", initData)
    }

    override fun onDestroy() {
        LogRelay.get().relay(TAG, "info", "onDestroy: cleaning up")
        try {
            lynxView?.removeCallbacks(positionUpdateRunnable)
            MpvPlayerModule.getInstance()?.let { mpvModule ->
                mpvModule.detachFromLayout(rootLayout ?: FrameLayout(this))
                mpvModule.release()
            }
            GoBackendModule.getInstance()?.unregisterReceiver()
            lynxView?.destroy()
            if (backendReceiverRegistered) {
                unregisterReceiver(backendReceiver)
            }
            finishAndRemoveTask()
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "onDestroy: cleanup error: ${e.message}")
        }
        super.onDestroy()
    }

    private fun resolveFileInfo(intent: Intent?) {
        if (intent == null) {
            LogRelay.get().relay(TAG, "warn", "resolveFileInfo: intent is null")
            return
        }

        val internalPath = intent.getStringExtra("file_path")
        if (!internalPath.isNullOrEmpty()) {
            intentFilePath = internalPath
            intentFileName = intent.getStringExtra("file_name") ?: File(internalPath).name
            intentFileMimeType = intent.getStringExtra("file_mime_type") ?: ""
            isExternalFile = false
            LogRelay.get().relay(TAG, "info", "resolveFileInfo: internal file, path=$intentFilePath")
            return
        }

        val uri = intent.data ?: if (Build.VERSION.SDK_INT >= 33) {
            intent.getParcelableExtra(Intent.EXTRA_STREAM, Uri::class.java)
        } else {
            @Suppress("DEPRECATION")
            intent.getParcelableExtra<Uri>(Intent.EXTRA_STREAM)
        }

        if (uri == null) {
            LogRelay.get().relay(TAG, "warn", "resolveFileInfo: no URI found")
            return
        }

        intentFileMimeType = intent.type ?: ""
        isExternalFile = true
        LogRelay.get().relay(TAG, "info", "resolveFileInfo: URI, scheme=${uri.scheme}, uri=$uri")

        when (uri.scheme) {
            "content" -> {
                var fileName = ""
                var filePath = ""
                contentResolver.query(uri, null, null, null, null)?.use { cursor ->
                    if (cursor.moveToFirst()) {
                        val nameIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                        if (nameIndex >= 0) {
                            fileName = cursor.getString(nameIndex)
                        }
                    }
                }
                if (fileName.isEmpty()) {
                    fileName = uri.lastPathSegment ?: "unknown_file"
                }
                val projection = arrayOf(android.provider.MediaStore.MediaColumns.DATA)
                try {
                    contentResolver.query(uri, projection, null, null, null)?.use { cursor ->
                        if (cursor.moveToFirst()) {
                            val dataIndex = cursor.getColumnIndexOrThrow(android.provider.MediaStore.MediaColumns.DATA)
                            filePath = cursor.getString(dataIndex)
                        }
                    }
                } catch (e: Exception) {
                    LogRelay.get().relay(TAG, "warn", "resolveFileInfo: query data failed: ${e.message}")
                }
                if (filePath.isEmpty() || !File(filePath).exists()) {
                    LogRelay.get().relay(TAG, "info", "resolveFileInfo: copying content to cache")
                    filePath = copyContentToCache(uri)
                }
                intentFilePath = filePath
                intentFileName = fileName
                if (intentFileMimeType.isEmpty()) {
                    intentFileMimeType = contentResolver.getType(uri) ?: ""
                }
            }
            "file" -> {
                intentFilePath = uri.path ?: ""
                intentFileName = uri.lastPathSegment ?: File(intentFilePath).name
                if (intentFileMimeType.isEmpty()) {
                    intentFileMimeType = contentResolver.getType(uri) ?: ""
                }
            }
        }
        LogRelay.get().relay(TAG, "info", "resolveFileInfo: external file resolved, path=$intentFilePath, name=$intentFileName")
    }

    private fun copyContentToCache(uri: Uri): String {
        val fileName = try {
            contentResolver.query(uri, null, null, null, null)?.use { cursor ->
                if (cursor.moveToFirst()) {
                    val nameIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                    if (nameIndex >= 0) cursor.getString(nameIndex) else null
                } else null
            }
        } catch (e: Exception) {
            null
        } ?: uri.lastPathSegment ?: "cached_file"

        val cacheDir = File(cacheDir, "player_cache")
        cacheDir.mkdirs()
        val destFile = File(cacheDir, fileName)
        if (destFile.exists()) {
            destFile.delete()
        }
        try {
            contentResolver.openInputStream(uri)?.use { input ->
                FileOutputStream(destFile).use { output ->
                    val buffer = ByteArray(8192)
                    var len: Int
                    while (input.read(buffer).also { len = it } != -1) {
                        output.write(buffer, 0, len)
                    }
                }
            }
            LogRelay.get().relay(TAG, "info", "copyContentToCache: copied to ${destFile.absolutePath}")
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "copyContentToCache failed: ${e.message}")
            return ""
        }
        return destFile.absolutePath
    }

    private fun createLynxView() {
        LogRelay.get().relay(TAG, "info", "createLynxView: START")
        try {
            val viewBuilder = LynxViewBuilder()
            LogRelay.get().relay(TAG, "info", "createLynxView: LynxViewBuilder created")

            viewBuilder.setTemplateProvider(PlayerTemplateProvider(this))
            LogRelay.get().relay(TAG, "info", "createLynxView: TemplateProvider set")

            viewBuilder.registerModule("MpvPlayerModule", MpvPlayerModule::class.java)
            viewBuilder.registerModule("GoBackendModule", GoBackendModule::class.java)
            viewBuilder.registerModule("LogBridge", LogBridgeModule::class.java)
            LogRelay.get().relay(TAG, "info", "createLynxView: Modules registered (MpvPlayerModule, GoBackendModule, LogBridge)")

            val displayMetrics = resources.displayMetrics
            val screenWidth = displayMetrics.widthPixels
            val screenHeight = displayMetrics.heightPixels
            LogRelay.get().relay(TAG, "info", "createLynxView: screen size ${screenWidth}x${screenHeight}")
            viewBuilder.setScreenSize(screenWidth, screenHeight)
            LogRelay.get().relay(TAG, "info", "createLynxView: screenSize set")

            lynxView = viewBuilder.build(this)
            LogRelay.get().relay(TAG, "info", "createLynxView: LynxView built, instance=$lynxView")
            lynxView?.setBackgroundColor(0)

            lynxView?.addLynxViewClient(object : LynxViewClient() {
                private val CLIENT_TAG = "LynxPlayerClient"

                override fun onPageStart(url: String?) {
                    LogRelay.get().relay(CLIENT_TAG, "info", "onPageStart: url=$url")
                }

                override fun onRuntimeReady() {
                    LogRelay.get().relay(CLIENT_TAG, "info", "onRuntimeReady: JS environment ready")
                    tryAttachMpvModule()
                }

                override fun onLoadSuccess() {
                    LogRelay.get().relay(CLIENT_TAG, "info", "onLoadSuccess: template loaded and rendered successfully")
                    tryAttachMpvModule()
                }

                override fun onLoadFailed(message: String) {
                    LogRelay.get().relay(CLIENT_TAG, "error", "onLoadFailed: message=$message")
                    runOnUiThread {
                        android.widget.Toast.makeText(
                            this@PlayerActivityLynx,
                            "Player load failed: $message",
                            android.widget.Toast.LENGTH_LONG
                        ).show()
                    }
                }

                override fun onFirstScreen() {
                    LogRelay.get().relay(CLIENT_TAG, "info", "onFirstScreen: first screen rendered")
                }

                override fun onReceivedError(error: com.lynx.tasm.LynxError) {
                    val fullError = buildString {
                        append("code=").append(error.errorCode)
                        append(" summary=").append(error.summaryMessage)
                        append(" rootCause=").append(error.rootCause)
                        append(" msg=").append(error.getMsg())
                    }
                    LogRelay.get().relay(CLIENT_TAG, "error", "onReceivedError: $fullError")
                    runOnUiThread {
                        android.widget.Toast.makeText(
                            this@PlayerActivityLynx,
                            fullError,
                            android.widget.Toast.LENGTH_LONG
                        ).show()
                    }
                }

                override fun onReceivedJSError(jsError: com.lynx.tasm.LynxError) {
                    val fullError = buildString {
                        append("summary=").append(jsError.summaryMessage)
                        append(" rootCause=").append(jsError.rootCause)
                        append(" msg=").append(jsError.getMsg())
                    }
                    LogRelay.get().relay(CLIENT_TAG, "error", "onReceivedJSError: $fullError")
                    runOnUiThread {
                        android.widget.Toast.makeText(
                            this@PlayerActivityLynx,
                            fullError,
                            android.widget.Toast.LENGTH_LONG
                        ).show()
                    }
                }

                override fun onReceivedJavaError(javaError: com.lynx.tasm.LynxError) {
                    LogRelay.get().relay(CLIENT_TAG, "error", "onReceivedJavaError: msg=${javaError.getMsg()}")
                }

                override fun onReceivedNativeError(nativeError: com.lynx.tasm.LynxError) {
                    LogRelay.get().relay(CLIENT_TAG, "error", "onReceivedNativeError: msg=${nativeError.getMsg()}")
                }
            })
            LogRelay.get().relay(TAG, "info", "createLynxView: LynxViewClient registered")

            val lynxParams = ViewGroup.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.MATCH_PARENT
            )
            rootLayout?.addView(lynxView, lynxParams)
            LogRelay.get().relay(TAG, "info", "createLynxView: LynxView added to rootLayout")

            val initData = buildInitDataJson()
            LogRelay.get().relay(TAG, "info", "createLynxView: initData=$initData")

            lynxView?.renderTemplateUrl("player.lynx.bundle", initData)
            LogRelay.get().relay(TAG, "info", "createLynxView: renderTemplateUrl called with player.lynx.bundle")

            lynxView?.post(positionUpdateRunnable)
        } catch (e: Exception) {
            LogRelay.get().relay(TAG, "error", "createLynxView: failed: ${e.message}")
            android.widget.Toast.makeText(this, "Player init failed: ${e.message}", android.widget.Toast.LENGTH_LONG).show()
            finish()
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
        val root = findViewById<android.widget.FrameLayout>(R.id.lynx_player_root)
        if (root == null) {
            LogRelay.get().relay(TAG, "warn", "tryAttachMpvModule: lynx_player_root not found")
            return
        }
        runOnUiThread {
            if (!mpvModule.isAttached()) {
                mpvModule.attachToLayout(root)
            }
        }
    }

    private fun buildInitDataJson(): String {
        val data = JSONObject().apply {
            put("filePath", intentFilePath)
            put("fileName", intentFileName)
            put("mimeType", intentFileMimeType)
            put("isExternal", isExternalFile)
        }
        return data.toString()
    }

    private fun handleBackend() {
        LogRelay.get().relay(TAG, "info", "handleBackend: checking backend state")
        when {
            EncvGoService.isRunning && EncvGoService.lastKnownPort > 0 -> {
                LogRelay.get().relay(TAG, "info", "handleBackend: backend already running, port=${EncvGoService.lastKnownPort}")
            }
            else -> {
                LogRelay.get().relay(TAG, "info", "handleBackend: starting backend service")
                startBackendService()
            }
        }
        registerBackendReceiver()
    }

    private fun startBackendService() {
        val intent = EncvGoService.createIntent(this, EncvGoService.ACTION_START, "player")
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(intent)
        } else {
            startService(intent)
        }
    }

    private fun registerBackendReceiver() {
        if (backendReceiverRegistered) {
            return
        }
        val filter = android.content.IntentFilter().apply {
            addAction(EncvGoService.BROADCAST_BACKEND_READY)
            addAction(EncvGoService.BROADCAST_BACKEND_STATUS)
        }
        if (Build.VERSION.SDK_INT >= 33) {
            registerReceiver(backendReceiver, filter, RECEIVER_NOT_EXPORTED)
        } else {
            @Suppress("DEPRECATION")
            registerReceiver(backendReceiver, filter)
        }
        backendReceiverRegistered = true
    }
}
