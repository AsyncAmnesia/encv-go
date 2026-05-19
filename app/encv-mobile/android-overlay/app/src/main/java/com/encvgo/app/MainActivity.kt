package com.encvgo.app

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.provider.Settings
import android.util.Log
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import com.getcapacitor.BridgeActivity
import java.io.BufferedReader
import java.io.File
import java.io.FileOutputStream
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URL
import org.json.JSONObject

class MainActivity : BridgeActivity() {
    companion object {
        private const val TAG = "ENCV-go"
        private const val BINARY_NAME = "encv-go"
        private const val DEFAULT_PORT = 2025
        private const val MAX_PORT_SCAN = 10
        private const val REQ_NOTIFICATIONS = 1001
        private const val REQ_MANAGE_STORAGE = 1002
    }

    private var goProcess: Process? = null
    private var backendPort = DEFAULT_PORT
    private var configPort = DEFAULT_PORT
    private var backendReady = false
    private var intentionallyStopped = false
    private var readyCallback: ((Int) -> Unit)? = null

    override fun onCreate(savedInstanceState: android.os.Bundle?) {
        registerPlugin(GoProcessPlugin::class.java)
        super.onCreate(savedInstanceState)
        requestRequiredPermissions()
        startGoDaemon()
    }

    private fun requestRequiredPermissions() {
        if (Build.VERSION.SDK_INT >= 33) {
            if (ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS)
                != PackageManager.PERMISSION_GRANTED
            ) {
                ActivityCompat.requestPermissions(
                    this,
                    arrayOf(Manifest.permission.POST_NOTIFICATIONS),
                    REQ_NOTIFICATIONS,
                )
            }
        }

        if (Build.VERSION.SDK_INT >= 30) {
            if (!Environment.isExternalStorageManager()) {
                try {
                    val intent = Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION)
                    intent.data = Uri.parse("package:$packageName")
                    startActivity(intent)
                } catch (e: Exception) {
                    val intent = Intent(Settings.ACTION_MANAGE_ALL_FILES_ACCESS_PERMISSION)
                    startActivity(intent)
                }
            }
        } else {
            val perms = mutableListOf<String>()
            if (ContextCompat.checkSelfPermission(this, Manifest.permission.READ_EXTERNAL_STORAGE)
                != PackageManager.PERMISSION_GRANTED
            ) {
                perms.add(Manifest.permission.READ_EXTERNAL_STORAGE)
            }
            if (ContextCompat.checkSelfPermission(this, Manifest.permission.WRITE_EXTERNAL_STORAGE)
                != PackageManager.PERMISSION_GRANTED
            ) {
                perms.add(Manifest.permission.WRITE_EXTERNAL_STORAGE)
            }
            if (perms.isNotEmpty()) {
                ActivityCompat.requestPermissions(this, perms.toTypedArray(), REQ_MANAGE_STORAGE)
            }
        }
    }

    fun isBackendRunning(): Boolean = backendReady && goProcess?.isAlive == true

    fun getBackendPort(): Int = if (backendReady) backendPort else 0

    fun stopGoDaemon() {
        intentionallyStopped = true
        backendReady = false
        goProcess?.let {
            if (it.isAlive) {
                it.destroyForcibly()
                Log.i(TAG, "Go daemon stopped by user")
            }
        }
        goProcess = null
        notifyFrontend(0, "stopped")
    }

    fun restartGoDaemon(callback: (Int) -> Unit) {
        stopGoDaemon()
        intentionallyStopped = false
        readyCallback = callback
        startGoDaemon()
    }

    private fun startGoDaemon() {
        try {
            ensureConfigExists()
            configPort = readConfigPort()

            val binary = findExecutableBinary()
                ?: run {
                    Log.e(TAG, "Failed to extract Go binary to any executable location")
                    notifyFrontend(0, "no_binary")
                    readyCallback?.invoke(-1)
                    readyCallback = null
                    return
                }

            Log.i(TAG, "Using binary at: ${binary.absolutePath}")
            Log.i(TAG, "Binary exists: ${binary.exists()}, size: ${binary.length()}, canExecute: ${binary.canExecute()}")

            val configPath = File(filesDir, "config.user.json").absolutePath
            val pb = ProcessBuilder(binary.absolutePath, "start")
            pb.environment()["ENCV_CONFIG_PATH"] = configPath
            pb.environment()["ENCV_MOBILE"] = "1"
            pb.environment()["HOME"] = filesDir.absolutePath
            pb.redirectErrorStream(true)
            goProcess = pb.start()

            Thread {
                try {
                    val reader = BufferedReader(InputStreamReader(goProcess?.inputStream))
                    var line: String?
                    while (reader.readLine().also { line = it } != null) {
                        Log.i(TAG, "[go] $line")
                    }
                } catch (e: Exception) {
                    Log.w(TAG, "Error reading process output", e)
                }
            }.start()

            Thread {
                waitForBackendAndNotify()
            }.start()

            Log.i(TAG, "ENCV-go daemon started (pid: unknown, async)")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start ENCV-go daemon", e)
            e.message?.split("\n")?.forEach { Log.e(TAG, it) }
            notifyFrontend(0, "start_failed")
            readyCallback?.invoke(-1)
            readyCallback = null
        }
    }

    private fun waitForBackendAndNotify() {
        for (attempt in 1..60) {
            if (intentionallyStopped) return
            for (offset in 0..MAX_PORT_SCAN) {
                val port = configPort + offset
                if (checkHealth(port)) {
                    backendPort = port
                    backendReady = true
                    Log.i(TAG, "Backend is ready on port $port (attempt $attempt, offset $offset)")
                    notifyFrontend(port, null)
                    readyCallback?.invoke(port)
                    readyCallback = null
                    return
                }
            }
            try {
                Thread.sleep(500)
            } catch (e: InterruptedException) {
                return
            }
        }
        Log.w(TAG, "Backend failed to start within 30 seconds")
        notifyFrontend(0, "timeout")
        readyCallback?.invoke(-1)
        readyCallback = null
    }

    private fun checkHealth(port: Int): Boolean {
        return try {
            val url = URL("http://127.0.0.1:$port/health")
            val conn = url.openConnection() as HttpURLConnection
            conn.connectTimeout = 300
            conn.readTimeout = 300
            val code = conn.responseCode
            conn.disconnect()
            code == 200
        } catch (e: Exception) {
            false
        }
    }

    private fun notifyFrontend(port: Int, error: String?) {
        runOnUiThread {
            try {
                val detail = JSONObject()
                detail.put("port", port)
                if (error != null) detail.put("error", error)
                val js = "window.dispatchEvent(new CustomEvent('encv:backend-ready',{detail:${detail.toString()}}))"
                bridge.webView.evaluateJavascript(js, null)
                Log.i(TAG, "Notified frontend: port=$port, error=$error")
            } catch (e: Exception) {
                Log.w(TAG, "Failed to notify frontend", e)
            }
        }
    }

    private fun readConfigPort(): Int {
        try {
            val configFile = File(filesDir, "config.user.json")
            if (configFile.exists()) {
                val json = configFile.readText()
                val jsonObj = JSONObject(json)
                val serverObj = jsonObj.optJSONObject("server")
                return serverObj?.optInt("port", DEFAULT_PORT) ?: DEFAULT_PORT
            }
        } catch (e: Exception) {
            Log.w(TAG, "Failed to read config port, using default $DEFAULT_PORT", e)
        }
        return DEFAULT_PORT
    }

    private fun findExecutableBinary(): File? {
        val candidateDirs = listOf(
            filesDir to "filesDir",
            cacheDir to "cacheDir",
            getExternalFilesDir(null) to "externalFilesDir",
        )

        for ((dir, name) in candidateDirs) {
            if (dir == null) continue
            Log.d(TAG, "Trying location: $name -> ${dir.absolutePath}")
            val binary = File(dir, BINARY_NAME)

            if (!binary.exists()) {
                copyBinaryFromAssets(binary)
            }

            binary.setReadable(true)
            binary.setExecutable(true)
            binary.setWritable(true)

            if (binary.canExecute()) {
                Log.i(TAG, "Binary is executable at: $name (${binary.absolutePath})")
                return binary
            }

            Log.w(TAG, "Binary NOT executable at: $name (Android ${Build.VERSION.RELEASE} API ${Build.VERSION.SDK_INT})")
        }

        return null
    }

    private fun ensureConfigExists() {
        val dest = File(filesDir, "config.user.json")
        if (!dest.exists()) {
            try {
                assets.open("config.mobile.json").use { input ->
                    FileOutputStream(dest).use { output ->
                        input.copyTo(output)
                    }
                }
                Log.i(TAG, "Copied default mobile config to ${dest.absolutePath}")
            } catch (e: Exception) {
                Log.e(TAG, "Failed to copy default config", e)
            }
        } else {
            Log.i(TAG, "Config already exists at ${dest.absolutePath}")
        }
    }

    private fun copyBinaryFromAssets(dest: File) {
        dest.parentFile?.mkdirs()
        assets.open(BINARY_NAME).use { input ->
            FileOutputStream(dest).use { output ->
                val buffer = ByteArray(8192)
                var len: Int
                while (input.read(buffer).also { len = it } != -1) {
                    output.write(buffer, 0, len)
                }
            }
        }
        Log.i(TAG, "Copied Go binary to ${dest.absolutePath} (${dest.length()} bytes)")
    }

    override fun onDestroy() {
        super.onDestroy()
        goProcess?.let {
            if (it.isAlive) {
                it.destroyForcibly()
                Log.i(TAG, "ENCV-go daemon force-stopped")
            }
        }
    }
}
