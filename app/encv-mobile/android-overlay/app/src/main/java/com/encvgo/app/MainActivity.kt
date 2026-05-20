package com.encvgo.app

import android.os.Build
import android.util.Log
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
    }

    private var goProcess: Process? = null
    private var backendPort = DEFAULT_PORT
    private var configPort = DEFAULT_PORT
    private var backendReady = false
    private var intentionallyStopped = false
    private var readyCallback: ((Int) -> Unit)? = null

    override fun onCreate(savedInstanceState: android.os.Bundle?) {
        Log.d(TAG, "=== onCreate: start ===")
        try {
            Log.d(TAG, "Registering GoProcessPlugin...")
            registerPlugin(GoProcessPlugin::class.java)
            Log.d(TAG, "GoProcessPlugin registered successfully via registerPlugin()")
        } catch (e: Exception) {
            Log.e(TAG, "registerPlugin FAILED", e)
        }
        super.onCreate(savedInstanceState)
        Log.d(TAG, "=== onCreate: end, starting daemon ===")
        startGoDaemon()
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
            val cmd = "${binary.absolutePath} start"
            Log.i(TAG, "Starting via sh -c: $cmd")
            goProcess = ProcessBuilder("/system/bin/sh", "-c", cmd).apply {
                environment()["ENCV_CONFIG_PATH"] = configPath
                environment()["ENCV_MOBILE"] = "1"
                environment()["HOME"] = filesDir.absolutePath
                redirectErrorStream(true)
                directory(filesDir)
            }.start()

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
            Log.e(TAG, "=== Start daemon FAILED ===", e)
            Log.e(TAG, "Exception: ${e.javaClass.name}: ${e.message}")
            e.stackTraceToString().lines().forEach { Log.e(TAG, it) }
            notifyFrontend(0, "start_failed:${e.message?.take(200) ?: 'unknown'}")
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

    Log.e(TAG, "=== Binary diagnosis: ALL locations failed ===")
    for ((dir, name) in candidateDirs) {
        if (dir == null) continue
        val binary = File(dir, BINARY_NAME)
        Log.e(TAG, "$name: exists=${binary.exists()}, len=${binary.length()}, canExec=${binary.canExecute()}, path=${binary.absolutePath}")
    }
    Log.e(TAG, "SDK_INT=${Build.VERSION.SDK_INT}, RELEASE=${Build.VERSION.RELEASE}")
    notifyFrontend(0, "no_binary")
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
