package com.encvgo.app

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.os.PowerManager
import android.util.Log
import androidx.core.app.NotificationCompat
import java.io.BufferedReader
import java.io.File
import java.io.FileOutputStream
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URL
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean
import org.json.JSONObject

class EncvGoService : Service() {
    companion object {
        private const val TAG = "ENCV-Service"
        private const val CHANNEL_ID = "encv_go_service"
        private const val NOTIFICATION_ID = 1001
        private const val BINARY_NAME = "encv-go"
        private const val DEFAULT_PORT = 2025
        private const val MAX_PORT_SCAN = 10
        private const val START_TIMEOUT_MS = 10_000L
        private const val POLL_INTERVAL_MS = 200L

        const val ACTION_START = "com.encvgo.action.START"
        const val ACTION_STOP = "com.encvgo.action.STOP"
        const val ACTION_RESTART = "com.encvgo.action.RESTART"
        const val ACTION_STATUS = "com.encvgo.action.STATUS"
        const val ACTION_EXTERNAL_START = "com.encvgo.action.EXTERNAL_START"
        const val ACTION_EXTERNAL_RESTART = "com.encvgo.action.EXTERNAL_RESTART"

        const val BROADCAST_BACKEND_READY = "com.encvgo.broadcast.BACKEND_READY"
        const val BROADCAST_BACKEND_STATUS = "com.encvgo.broadcast.BACKEND_STATUS"
        const val BROADCAST_EXTERNAL_RESULT = "com.encvgo.broadcast.EXTERNAL_RESULT"

        const val EXTRA_PORT = "port"
        const val EXTRA_ERROR = "error"
        const val EXTRA_RUNNING = "running"
        const val EXTRA_SOURCE = "source"
        const val EXTRA_COMMAND = "command"

        @Volatile
        var lastKnownPort: Int = 0

        @Volatile
        var isRunning: Boolean = false

        @Volatile
        var lastError: String? = null

        fun createIntent(context: Context, action: String, source: String = "manual"): Intent {
            return Intent(context, EncvGoService::class.java).apply {
                this.action = action
                putExtra(EXTRA_SOURCE, source)
            }
        }

        @Volatile
        private var instance: EncvGoService? = null

        fun getOutputSnapshot(): String {
            val svc = instance ?: return ""
            return synchronized(svc.outputBuffer) {
                svc.outputBuffer.toString()
            }
        }

        fun clearOutputSnapshot() {
            val svc = instance ?: return
            synchronized(svc.outputBuffer) {
                svc.outputBuffer.clear()
            }
        }
    }

    private val worker = Executors.newSingleThreadExecutor()
    private val startupGeneration = java.util.concurrent.atomic.AtomicInteger(0)
    private val readyKeywords = listOf("listening on", "server ready", "ready", "started")

    private var goProcess: Process? = null
    private var currentPort = DEFAULT_PORT
    private var configPort = DEFAULT_PORT
    private var currentSource = "manual"
    private var outputBuffer = StringBuilder()
    private var lastExitCode: Int? = null
    private val processReady = AtomicBoolean(false)
    private var wakeLock: PowerManager.WakeLock? = null

    override fun onCreate() {
        super.onCreate()
        instance = this
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        currentSource = intent?.getStringExtra(EXTRA_SOURCE) ?: "manual"
        when (intent?.action) {
            ACTION_STOP -> worker.execute { stopGoProcess("stopped", stopService = true) }
            ACTION_RESTART, ACTION_EXTERNAL_RESTART -> worker.execute {
                restartGoProcess(currentSource, intent?.getStringExtra(EXTRA_COMMAND))
            }
            ACTION_STATUS -> publishStatus(lastError)
            ACTION_START, ACTION_EXTERNAL_START, null -> worker.execute {
                startGoProcess(currentSource, intent?.getStringExtra(EXTRA_COMMAND))
            }
        }
        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        instance = null
        stopGoProcess("service_destroyed", stopService = false)
        worker.shutdownNow()
        super.onDestroy()
    }

    private fun startGoProcess(source: String, command: String?) {
        if (goProcess?.isAlive == true && processReady.get()) {
            publishStatus(null, source, command)
            return
        }

        startForeground(NOTIFICATION_ID, buildNotification("后端启动中"))
        acquireWakeLock()
        resetStateForStart(source)

        try {
            ensureConfigExists()
            ensureBuildInfoExists()
            configPort = readConfigPort()
            val binary = findExecutableBinary() ?: run {
                publishFailure("no_binary", source, command)
                return
            }

            val configPath = File(filesDir, "config.user.json").absolutePath
            Log.i(TAG, "Starting backend: ${binary.absolutePath} start")

            goProcess = ProcessBuilder(binary.absolutePath, "start").apply {
                environment()["ENCV_CONFIG_PATH"] = configPath
                environment()["ENCV_MOBILE"] = "1"
                environment()["HOME"] = filesDir.absolutePath
                environment()["ENCV_LIB_DIR"] = applicationInfo.nativeLibraryDir
                redirectErrorStream(true)
                directory(filesDir)
            }.start()

            monitorProcessOutput(startupGeneration.incrementAndGet(), source, command)
            waitForBackendReady(startupGeneration.get(), source, command)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start backend", e)
            publishFailure("start_failed:${e.message ?: "unknown"}", source, command)
        }
    }

    private fun restartGoProcess(source: String, command: String?) {
        stopGoProcess("restarting", stopService = false)
        startGoProcess(source, command)
    }

    private fun stopGoProcess(errorMessage: String?, stopService: Boolean) {
        processReady.set(false)
        isRunning = false
        lastKnownPort = 0
        currentPort = DEFAULT_PORT
        lastError = errorMessage
        releaseWakeLock()

        goProcess?.let {
            try {
                if (it.isAlive) {
                    it.destroyForcibly()
                    Log.i(TAG, "Backend process stopped")
                }
            } catch (e: Exception) {
                Log.w(TAG, "Failed to stop backend process", e)
            }
        }
        goProcess = null
        outputBuffer = StringBuilder()
        updateNotification(if (stopService) "后端已停止" else "后端重启中")
        publishStatus(errorMessage, currentSource, if (stopService) ACTION_STOP else null)

        if (stopService) {
            sendExternalResult(true, null, currentSource, "stop")
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
        }
    }

    private fun monitorProcessOutput(generation: Int, source: String, command: String?) {
        Thread {
            try {
                val reader = BufferedReader(InputStreamReader(goProcess?.inputStream))
                var line: String?
                while (reader.readLine().also { line = it } != null) {
                    val content = line ?: continue
                    Log.i(TAG, "[go] $content")
                    synchronized(outputBuffer) {
                        outputBuffer.append(content).append('\n')
                    }
                    if (!processReady.get() && readyKeywords.any { content.contains(it, ignoreCase = true) }) {
                        maybeMarkReady(generation, source, command)
                    }
                    if (content.contains("error", ignoreCase = true) ||
                        content.contains("fatal", ignoreCase = true) ||
                        content.contains("panic", ignoreCase = true) ||
                        content.contains("permission denied", ignoreCase = true)) {
                        lastError = "go_error:$content"
                        publishStatus(lastError, source, command)
                    }
                }

                lastExitCode = goProcess?.waitFor() ?: -1
                Log.w(TAG, "Backend exited with code: $lastExitCode")
                if (!processReady.get()) {
                    publishFailure("go_exit:${lastExitCode ?: -1}", source, command)
                } else {
                    isRunning = false
                    lastKnownPort = 0
                    publishStatus("go_exit:${lastExitCode ?: -1}", source, command)
                    updateNotification("后端已退出")
                }
            } catch (e: Exception) {
                Log.w(TAG, "Error monitoring backend output", e)
                if (!processReady.get()) {
                    publishFailure("monitor_error:${e.message ?: "unknown"}", source, command)
                }
            }
        }.start()
    }

    private fun waitForBackendReady(generation: Int, source: String, command: String?) {
        Thread {
            val startAt = System.currentTimeMillis()
            while (System.currentTimeMillis() - startAt < START_TIMEOUT_MS) {
                if (generation != startupGeneration.get()) return@Thread
                if (processReady.get()) return@Thread
                for (offset in 0..MAX_PORT_SCAN) {
                    val port = configPort + offset
                    if (checkHealth(port)) {
                        currentPort = port
                        maybeMarkReady(generation, source, command)
                        return@Thread
                    }
                }
                Thread.sleep(POLL_INTERVAL_MS)
            }

            if (!processReady.get()) {
                val tail = synchronized(outputBuffer) {
                    outputBuffer.takeLast(1000).toString().trim()
                }
                val exitInfo = lastExitCode?.let { "exit=$it" } ?: if (goProcess?.isAlive == true) "alive=true" else "alive=false"
                publishFailure("timeout:$exitInfo|output:${if (tail.isEmpty()) "(empty)" else tail}", source, command)
            }
        }.start()
    }

    @Synchronized
    private fun maybeMarkReady(generation: Int, source: String, command: String?) {
        if (generation != startupGeneration.get() || processReady.get()) return

        if (currentPort == DEFAULT_PORT) {
            for (offset in 0..MAX_PORT_SCAN) {
                val port = configPort + offset
                if (checkHealth(port)) {
                    currentPort = port
                    break
                }
            }
        }

        processReady.set(true)
        isRunning = true
        lastKnownPort = currentPort
        lastError = null
        updateNotification("后端已就绪 :$currentPort")
        publishReady(source, command)
    }

    private fun publishReady(source: String, command: String?) {
        val readyIntent = Intent(BROADCAST_BACKEND_READY).apply {
            putExtra(EXTRA_PORT, currentPort)
            putExtra(EXTRA_RUNNING, true)
            putExtra(EXTRA_SOURCE, source)
            putExtra(EXTRA_COMMAND, command)
        }
        sendBroadcast(readyIntent)

        publishStatus(null, source, command)
        sendExternalResult(true, null, source, command)
    }

    private fun publishFailure(error: String, source: String, command: String?) {
        lastError = error
        isRunning = false
        processReady.set(false)
        lastKnownPort = 0
        updateNotification("后端启动失败")
        publishStatus(error, source, command)
        sendExternalResult(false, error, source, command)
    }

    private fun publishStatus(error: String?, source: String = currentSource, command: String? = null) {
        val statusIntent = Intent(BROADCAST_BACKEND_STATUS).apply {
            putExtra(EXTRA_PORT, if (isRunning) currentPort else 0)
            putExtra(EXTRA_RUNNING, isRunning)
            putExtra(EXTRA_SOURCE, source)
            putExtra(EXTRA_COMMAND, command)
            if (!error.isNullOrEmpty()) {
                putExtra(EXTRA_ERROR, error)
            }
        }
        sendBroadcast(statusIntent)
    }

    private fun sendExternalResult(success: Boolean, error: String?, source: String, command: String?) {
        val intent = Intent(BROADCAST_EXTERNAL_RESULT).apply {
            putExtra("success", success)
            putExtra(EXTRA_PORT, if (success) currentPort else 0)
            putExtra(EXTRA_RUNNING, success)
            putExtra(EXTRA_SOURCE, source)
            putExtra(EXTRA_COMMAND, command)
            if (!error.isNullOrEmpty()) {
                putExtra(EXTRA_ERROR, error)
            }
        }
        sendBroadcast(intent)
    }

    private fun resetStateForStart(source: String) {
        currentSource = source
        currentPort = DEFAULT_PORT
        lastKnownPort = 0
        isRunning = false
        lastError = null
        lastExitCode = null
        processReady.set(false)
        outputBuffer = StringBuilder()
    }

    private fun buildNotification(text: String): Notification {
        val openIntent = Intent(this, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
        }
        val pendingIntent = PendingIntent.getActivity(
            this,
            0,
            openIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setContentTitle("ENCV-go")
            .setContentText(text)
            .setContentIntent(pendingIntent)
            .setOngoing(true)
            .build()
    }

    private fun updateNotification(text: String) {
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        manager.notify(NOTIFICATION_ID, buildNotification(text))
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val manager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
            val channel = NotificationChannel(
                CHANNEL_ID,
                "ENCV 后端服务",
                NotificationManager.IMPORTANCE_LOW
            )
            manager.createNotificationChannel(channel)
        }
    }

    private fun readConfigPort(): Int {
        return try {
            val configFile = File(filesDir, "config.user.json")
            if (!configFile.exists()) return DEFAULT_PORT
            val jsonObj = JSONObject(configFile.readText())
            jsonObj.optJSONObject("server")?.optInt("port", DEFAULT_PORT) ?: DEFAULT_PORT
        } catch (e: Exception) {
            Log.w(TAG, "Failed to read config port", e)
            DEFAULT_PORT
        }
    }

    private fun ensureConfigExists() {
        val dest = File(filesDir, "config.user.json")
        if (dest.exists()) {
            mergeConfigDefaults(dest)
            return
        }
        copyDefaultConfig(dest)
    }

    private fun ensureBuildInfoExists() {
        val dest = File(filesDir, "build-info.json")
        if (dest.exists()) return
        try {
            assets.open("build-info.json").use { input ->
                FileOutputStream(dest).use { output ->
                    input.copyTo(output)
                }
            }
            Log.i(TAG, "Copied build-info.json to filesDir")
        } catch (e: Exception) {
            Log.w(TAG, "Failed to copy build-info.json", e)
        }
    }

    private fun copyDefaultConfig(dest: File) {
        try {
            assets.open("config.user.json").use { input ->
                FileOutputStream(dest).use { output ->
                    input.copyTo(output)
                }
            }
            Log.i(TAG, "Default config copied to ${dest.absolutePath}")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to copy default config", e)
            writeFallbackConfig(dest)
        }
    }

    private fun mergeConfigDefaults(dest: File) {
        try {
            val existing = JSONObject(dest.readText())
            var changed = false

            val defaults = try {
                JSONObject(assets.open("config.user.json").bufferedReader().use { it.readText() })
            } catch (e: Exception) {
                Log.w(TAG, "Cannot read default config for merge", e)
                return
            }

            val serverObj = existing.optJSONObject("server")
            val defaultServer = defaults.optJSONObject("server")
            if (serverObj != null && defaultServer != null) {
                if (!serverObj.has("port")) {
                    serverObj.put("port", defaultServer.optInt("port", DEFAULT_PORT))
                    changed = true
                }
                val currentDir = serverObj.optString("dir", "")
                if (currentDir.isEmpty() || currentDir == "/") {
                    val mobileDir = existing.optJSONObject("mobile")?.optString("server_dir", "")
                        ?: defaults.optJSONObject("mobile")?.optString("server_dir", "")
                        ?: "/storage/emulated/0"
                    serverObj.put("dir", mobileDir)
                    changed = true
                }
            }

            if (!existing.has("password")) {
                existing.put("password", defaults.optString("password", ""))
                changed = true
            }
            if (!existing.has("output_path")) {
                existing.put("output_path", defaults.optString("output_path", "/storage/emulated/0/encv-output"))
                changed = true
            }
            if (!existing.has("plugin_settings")) {
                existing.put("plugin_settings", defaults.optJSONObject("plugin_settings") ?: JSONObject())
                changed = true
            }
            if (!existing.has("log")) {
                existing.put("log", defaults.optJSONObject("log") ?: JSONObject().put("level", "info").put("console", true))
                changed = true
            }

            val existingMobile = existing.optJSONObject("mobile")
            val defaultMobile = defaults.optJSONObject("mobile")
            if (defaultMobile != null) {
                val targetMobile = existingMobile ?: JSONObject().also {
                    existing.put("mobile", it)
                    changed = true
                }
                if (!targetMobile.has("server_dir")) {
                    targetMobile.put("server_dir", defaultMobile.optString("server_dir", ""))
                    changed = true
                }
                if (!targetMobile.has("output_path")) {
                    targetMobile.put("output_path", defaultMobile.optString("output_path", ""))
                    changed = true
                }
                if (!targetMobile.has("webdav_dir")) {
                    targetMobile.put("webdav_dir", defaultMobile.optString("webdav_dir", ""))
                    changed = true
                }
            }

            if (!existing.has("recover")) {
                existing.put("recover", defaults.optBoolean("recover", false))
                changed = true
            }
            if (!existing.has("default_container_version")) {
                existing.put("default_container_version", defaults.optInt("default_container_version", 4))
                changed = true
            }
            if (!existing.has("admin")) {
                existing.put("admin", defaults.optJSONObject("admin") ?: JSONObject().put("password", ""))
                changed = true
            }
            if (!existing.has("webdav")) {
                val defaultWebdav = defaults.optJSONObject("webdav")
                existing.put("webdav", defaultWebdav ?: JSONObject().put("root", "").put("dir", "").put("username", "").put("password", ""))
                changed = true
            }
            if (!existing.has("proxy")) {
                val defaultProxy = defaults.optJSONObject("proxy")
                existing.put("proxy", defaultProxy ?: JSONObject().put("sites", JSONObject()).put("disable_signature_verification", true))
                changed = true
            }

            if (changed) {
                dest.writeText(existing.toString(2))
                Log.i(TAG, "Config merged with new defaults")
            }
        } catch (e: Exception) {
            Log.w(TAG, "Failed to merge config defaults", e)
        }
    }

    private fun writeFallbackConfig(dest: File) {
        val fallback = JSONObject().apply {
            put("password", "")
            put("recover", false)
            put("default_container_version", 4)
            put("output_path", "/storage/emulated/0/encv-output")
            put("server", JSONObject().put("port", DEFAULT_PORT).put("dir", "/storage/emulated/0"))
            put("admin", JSONObject().put("password", ""))
            put("webdav", JSONObject().put("root", "").put("dir", "").put("username", "").put("password", ""))
            put("proxy", JSONObject().put("sites", JSONObject()).put("disable_signature_verification", true))
            put("plugin_settings", JSONObject())
            put("log", JSONObject().put("level", "info").put("file", "").put("console", true))
            put("mobile", JSONObject().apply {
                put("server_dir", "/storage/emulated/0")
                put("output_path", "/storage/emulated/0/encv-output")
                put("webdav_dir", "")
            })
        }
        dest.writeText(fallback.toString(2))
        Log.i(TAG, "Fallback config written to ${dest.absolutePath}")
    }

    private fun findExecutableBinary(): File? {
        val nativeLibDir = applicationInfo.nativeLibraryDir
        Log.i(TAG, "nativeLibraryDir: $nativeLibDir")

        val nativeBinary = File(nativeLibDir, "libencv-go.so")
        Log.i(TAG, "Checking native binary: exists=${nativeBinary.exists()}, canExecute=${nativeBinary.canExecute()}, path=${nativeBinary.absolutePath}")

        if (nativeBinary.exists() && nativeBinary.canExecute()) {
            Log.i(TAG, "Using binary from nativeLibraryDir: ${nativeBinary.absolutePath}")
            return nativeBinary
        }

        val libDir = File(nativeLibDir)
        if (libDir.exists()) {
            libDir.listFiles()?.forEach { f ->
                Log.i(TAG, "  lib dir entry: ${f.name} exe=${f.canExecute()}")
            }
        } else {
            Log.w(TAG, "nativeLibraryDir does not exist: $nativeLibDir")
        }

        Log.w(TAG, "nativeLibraryDir lookup failed, falling back to filesDir (may fail on Android 10+)")

        val candidateDirs = listOf(
            filesDir to "filesDir",
            cacheDir to "cacheDir",
            getExternalFilesDir(null) to "externalFilesDir",
        )

        for ((dir, name) in candidateDirs) {
            if (dir == null) continue
            val binary = File(dir, BINARY_NAME)
            if (!binary.exists()) {
                copyBinaryFromAssets(binary)
            }
            binary.setReadable(true)
            binary.setExecutable(true)
            binary.setWritable(true)
            if (binary.canExecute()) {
                Log.i(TAG, "Using binary from $name: ${binary.absolutePath}")
                return binary
            }
        }
        return null
    }

    private fun copyBinaryFromAssets(dest: File) {
        dest.parentFile?.mkdirs()
        try {
            assets.open(BINARY_NAME).use { input ->
                FileOutputStream(dest).use { output ->
                    val buffer = ByteArray(8192)
                    var len: Int
                    while (input.read(buffer).also { len = it } != -1) {
                        output.write(buffer, 0, len)
                    }
                }
            }
        } catch (e: Exception) {
            Log.w(TAG, "Binary not found in assets (expected on Android 10+ with jniLibs packaging)", e)
        }
    }

    private fun checkHealth(port: Int): Boolean {
        return try {
            val conn = URL("http://127.0.0.1:$port/health").openConnection() as HttpURLConnection
            conn.connectTimeout = 300
            conn.readTimeout = 300
            val code = conn.responseCode
            conn.disconnect()
            code == 200
        } catch (_: Exception) {
            false
        }
    }

    private fun acquireWakeLock() {
        if (wakeLock?.isHeld == true) return
        try {
            val pm = getSystemService(Context.POWER_SERVICE) as PowerManager
            wakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "encvgo::GoService")
            wakeLock?.acquire()
            Log.i(TAG, "WakeLock acquired")
        } catch (e: Exception) {
            Log.w(TAG, "Failed to acquire WakeLock", e)
        }
    }

    private fun releaseWakeLock() {
        wakeLock?.let {
            if (it.isHeld) {
                it.release()
                Log.i(TAG, "WakeLock released")
            }
        }
        wakeLock = null
    }
}
