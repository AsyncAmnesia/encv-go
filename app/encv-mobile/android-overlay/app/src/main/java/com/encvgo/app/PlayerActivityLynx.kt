package com.encvgo.app

import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.provider.OpenableColumns
import android.util.Log
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
                    Log.d(TAG, "backend broadcast: ${intent.action}")
                }
            }
        }
    }

    private val positionUpdateRunnable = object : Runnable {
        override fun run() {
            try {
                MpvPlayerModule.getInstance()?.dispatchPositionUpdate()
            } catch (e: Exception) {
                Log.e(TAG, "positionUpdateRunnable failed", e)
            }
            lynxView?.postDelayed(this, 500)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        Log.d(TAG, "onCreate: starting")

        EncvApplication.ensureLynxInitialized(application)

        setContentView(R.layout.lynx_player_activity)
        rootLayout = findViewById(R.id.lynx_player_root)
        Log.d(TAG, "onCreate: root layout found: $rootLayout")

        resolveFileInfo(intent)
        Log.d(TAG, "onCreate: file info resolved, path=$intentFilePath, name=$intentFileName, mimeType=$intentFileMimeType, external=$isExternalFile")

        createLynxView()
        handleBackend()

        Log.d(TAG, "onCreate: setup complete")
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        Log.d(TAG, "onNewIntent: new intent received")
        setIntent(intent)
        resolveFileInfo(intent)
        val initData = buildInitDataJson()
        lynxView?.renderTemplateUrl("player.lynx.bundle", initData)
    }

    override fun onDestroy() {
        Log.d(TAG, "onDestroy: cleaning up")
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
            Log.e(TAG, "onDestroy: cleanup error", e)
        }
        super.onDestroy()
    }

    private fun resolveFileInfo(intent: Intent?) {
        if (intent == null) {
            Log.w(TAG, "resolveFileInfo: intent is null")
            return
        }

        val internalPath = intent.getStringExtra("file_path")
        if (!internalPath.isNullOrEmpty()) {
            intentFilePath = internalPath
            intentFileName = intent.getStringExtra("file_name") ?: File(internalPath).name
            intentFileMimeType = intent.getStringExtra("file_mime_type") ?: ""
            isExternalFile = false
            Log.d(TAG, "resolveFileInfo: internal file, path=$intentFilePath")
            return
        }

        val uri = intent.data ?: if (Build.VERSION.SDK_INT >= 33) {
            intent.getParcelableExtra(Intent.EXTRA_STREAM, Uri::class.java)
        } else {
            @Suppress("DEPRECATION")
            intent.getParcelableExtra<Uri>(Intent.EXTRA_STREAM)
        }

        if (uri == null) {
            Log.w(TAG, "resolveFileInfo: no URI found")
            return
        }

        intentFileMimeType = intent.type ?: ""
        isExternalFile = true
        Log.d(TAG, "resolveFileInfo: URI, scheme=${uri.scheme}, uri=$uri")

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
                    Log.w(TAG, "resolveFileInfo: query data failed", e)
                }
                if (filePath.isEmpty() || !File(filePath).exists()) {
                    Log.d(TAG, "resolveFileInfo: copying content to cache")
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
        Log.d(TAG, "resolveFileInfo: external file resolved, path=$intentFilePath, name=$intentFileName")
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
            Log.d(TAG, "copyContentToCache: copied to ${destFile.absolutePath}")
        } catch (e: Exception) {
            Log.e(TAG, "copyContentToCache failed", e)
            return ""
        }
        return destFile.absolutePath
    }

    private fun createLynxView() {
        Log.d(TAG, "createLynxView: START")
        try {
            val viewBuilder = LynxViewBuilder()
            Log.d(TAG, "createLynxView: LynxViewBuilder created")

            viewBuilder.setTemplateProvider(PlayerTemplateProvider(this))
            Log.d(TAG, "createLynxView: TemplateProvider set")

            viewBuilder.registerModule("MpvPlayerModule", MpvPlayerModule::class.java)
            viewBuilder.registerModule("GoBackendModule", GoBackendModule::class.java)
            Log.d(TAG, "createLynxView: Modules registered (MpvPlayerModule, GoBackendModule)")

            lynxView = viewBuilder.build(this)
            Log.d(TAG, "createLynxView: LynxView built, instance=$lynxView")

            lynxView?.addLynxViewClient(object : LynxViewClient() {
                private val CLIENT_TAG = "LynxPlayerClient"

                override fun onPageStart(url: String?) {
                    Log.d(CLIENT_TAG, "onPageStart: url=$url")
                }

                override fun onRuntimeReady() {
                    Log.d(CLIENT_TAG, "onRuntimeReady: JS environment ready")
                }

                override fun onLoadSuccess() {
                    Log.d(CLIENT_TAG, "onLoadSuccess: template loaded and rendered successfully")
                }

                override fun onLoadFailed(message: String) {
                    Log.e(CLIENT_TAG, "onLoadFailed: message=$message")
                    runOnUiThread {
                        android.widget.Toast.makeText(
                            this@PlayerActivityLynx,
                            "Player load failed: $message",
                            android.widget.Toast.LENGTH_LONG
                        ).show()
                    }
                }

                override fun onFirstScreen() {
                    Log.d(CLIENT_TAG, "onFirstScreen: first screen rendered")
                }

                override fun onReceivedError(error: com.lynx.tasm.LynxError) {
                    Log.e(CLIENT_TAG, "onReceivedError: code=${error.errorCode} msg=${error.summaryMessage} rootCause=${error.rootCause}")
                    runOnUiThread {
                        android.widget.Toast.makeText(
                            this@PlayerActivityLynx,
                            "Player error: ${error.summaryMessage}",
                            android.widget.Toast.LENGTH_LONG
                        ).show()
                    }
                }

                override fun onReceivedJSError(jsError: com.lynx.tasm.LynxError) {
                    Log.e(CLIENT_TAG, "onReceivedJSError: msg=${jsError.getMsg()} rootCause=${jsError.rootCause}")
                    runOnUiThread {
                        android.widget.Toast.makeText(
                            this@PlayerActivityLynx,
                            "JS error: ${jsError.summaryMessage}",
                            android.widget.Toast.LENGTH_LONG
                        ).show()
                    }
                }

                override fun onReceivedJavaError(javaError: com.lynx.tasm.LynxError) {
                    Log.e(CLIENT_TAG, "onReceivedJavaError: msg=${javaError.getMsg()}")
                }

                override fun onReceivedNativeError(nativeError: com.lynx.tasm.LynxError) {
                    Log.e(CLIENT_TAG, "onReceivedNativeError: msg=${nativeError.getMsg()}")
                }
            })
            Log.d(TAG, "createLynxView: LynxViewClient registered")

            val lynxParams = ViewGroup.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.MATCH_PARENT
            )
            rootLayout?.addView(lynxView, lynxParams)
            Log.d(TAG, "createLynxView: LynxView added to rootLayout")

            val initData = buildInitDataJson()
            Log.d(TAG, "createLynxView: initData=$initData")

            lynxView?.renderTemplateUrl("player.lynx.bundle", initData)
            Log.d(TAG, "createLynxView: renderTemplateUrl called with player.lynx.bundle")

            lynxView?.post {
                try {
                    val mpvModule = MpvPlayerModule.getInstance()
                    if (mpvModule != null && rootLayout != null) {
                        mpvModule.attachToLayout(rootLayout!!)
                    }
                } catch (e: Exception) {
                    Log.e(TAG, "createLynxView: attachToLayout failed", e)
                }
                lynxView?.post(positionUpdateRunnable)
            }
        } catch (e: Exception) {
            Log.e(TAG, "createLynxView: failed", e)
            android.widget.Toast.makeText(this, "Player init failed: ${e.message}", android.widget.Toast.LENGTH_LONG).show()
            finish()
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
        Log.d(TAG, "handleBackend: checking backend state")
        when {
            EncvGoService.isRunning && EncvGoService.lastKnownPort > 0 -> {
                Log.d(TAG, "handleBackend: backend already running, port=${EncvGoService.lastKnownPort}")
            }
            else -> {
                Log.d(TAG, "handleBackend: starting backend service")
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
