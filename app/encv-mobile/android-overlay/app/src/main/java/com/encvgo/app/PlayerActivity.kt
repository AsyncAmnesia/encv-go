package com.encvgo.app

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.database.Cursor
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.provider.OpenableColumns
import android.util.Log
import androidx.core.content.ContextCompat
import com.getcapacitor.BridgeActivity
import org.json.JSONObject
import java.io.File
import java.io.FileOutputStream
import java.io.InputStream

class PlayerActivity : BridgeActivity() {
    companion object {
        private const val TAG = "ENCV-go"

        var intentFilePath: String = ""
        var intentFileName: String = ""
        var intentFileMimeType: String = ""
    }

    private var backendReceiverRegistered = false

    private val backendReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            when (intent?.action) {
                EncvGoService.BROADCAST_BACKEND_READY,
                EncvGoService.BROADCAST_BACKEND_STATUS -> {
                    val port = intent.getIntExtra(EncvGoService.EXTRA_PORT, 0)
                    val error = intent.getStringExtra(EncvGoService.EXTRA_ERROR)
                    val running = intent.getBooleanExtra(EncvGoService.EXTRA_RUNNING, false)
                    val source = intent.getStringExtra(EncvGoService.EXTRA_SOURCE)
                    val command = intent.getStringExtra(EncvGoService.EXTRA_COMMAND)
                    notifyFrontend(port, running, error, source, command)
                }
            }
        }
    }

    override fun load() {
        super.load()
        try {
            bridge?.webView?.loadUrl("https://localhost/player.html")
            Log.i(TAG, "PlayerActivity loading isolated player app")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to load player app", e)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        try {
            registerPlugin(GoProcessPlugin::class.java)
        } catch (e: Exception) {
            Log.e(TAG, "registerPlugin failed", e)
        }
        super.onCreate(savedInstanceState)
        registerBackendReceiver()
        resolveFileInfo(intent)
        if (EncvGoService.isRunning && EncvGoService.lastKnownPort > 0) {
            notifyFrontend(EncvGoService.lastKnownPort, true, null, "player", null)
        } else {
            startBackendService(EncvGoService.ACTION_START, "player", null)
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        resolveFileInfo(intent)
    }

    override fun onDestroy() {
        if (backendReceiverRegistered) {
            unregisterReceiver(backendReceiver)
            backendReceiverRegistered = false
        }
        super.onDestroy()
    }

    private fun registerBackendReceiver() {
        if (backendReceiverRegistered) return
        val filter = IntentFilter().apply {
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

    private fun resolveFileInfo(intent: Intent?) {
        if (intent == null) return

        val internalPath = intent.getStringExtra("file_path")
        if (!internalPath.isNullOrEmpty()) {
            intentFilePath = internalPath
            intentFileName = intent.getStringExtra("file_name") ?: File(internalPath).name
            intentFileMimeType = intent.getStringExtra("file_mime_type") ?: ""
            return
        }

        val uri: Uri? = intent.data ?: intent.getParcelableExtra(Intent.EXTRA_STREAM)
        if (uri == null) return

        intentFileMimeType = intent.type ?: ""

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
                } catch (_: Exception) {
                }
                if (filePath.isEmpty() || !File(filePath).exists()) {
                    filePath = copyContentToCache(uri)
                }
                intentFilePath = filePath
                intentFileName = fileName
                if (intentFileMimeType.isEmpty()) {
                    intentFileMimeType = contentResolver.getType(uri) ?: ""
                }
            }
            "file" -> {
                val path = uri.path ?: ""
                intentFilePath = path
                intentFileName = uri.lastPathSegment ?: File(path).name
                if (intentFileMimeType.isEmpty()) {
                    intentFileMimeType = contentResolver.getType(uri) ?: ""
                }
            }
        }
    }

    private fun copyContentToCache(uri: Uri): String {
        val fileName = try {
            contentResolver.query(uri, null, null, null, null)?.use { cursor ->
                if (cursor.moveToFirst()) {
                    val nameIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                    if (nameIndex >= 0) cursor.getString(nameIndex) else null
                } else null
            }
        } catch (_: Exception) {
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
        } catch (e: Exception) {
            Log.e(TAG, "Failed to copy content to cache", e)
            return ""
        }
        return destFile.absolutePath
    }

    private fun startBackendService(action: String, source: String, command: String?) {
        val serviceIntent = EncvGoService.createIntent(this, action, source).apply {
            if (!command.isNullOrEmpty()) {
                putExtra(EncvGoService.EXTRA_COMMAND, command)
            }
        }
        ContextCompat.startForegroundService(this, serviceIntent)
    }

    private fun notifyFrontend(port: Int, running: Boolean, error: String?, source: String?, command: String?) {
        runOnUiThread {
            try {
                val detail = JSONObject().apply {
                    put("port", port)
                    put("running", running)
                    if (error != null) put("error", error)
                    if (source != null) put("source", source)
                    if (command != null) put("command", command)
                }
                val readyEvent = "window.dispatchEvent(new CustomEvent('encv:backend-ready',{detail:${detail}}))"
                val statusEvent = "window.dispatchEvent(new CustomEvent('encv:backend-status',{detail:${detail}}))"
                bridge?.webView?.evaluateJavascript(readyEvent, null)
                bridge?.webView?.evaluateJavascript(statusEvent, null)
            } catch (e: Exception) {
                Log.w(TAG, "Failed to notify frontend", e)
            }
        }
    }
}
