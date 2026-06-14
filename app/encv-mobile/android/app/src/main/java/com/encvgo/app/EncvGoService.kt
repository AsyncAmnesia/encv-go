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
        // 🆕 2026-06-12 Phase 4：心跳文件 mtime 探活阈值。
        // 8s = 5s ffmpeg timeout + 3s 容差（worker 启动 + 调度 + cgo dlopen）。
        // 真机 cgo ffmpeg_run 二次 hang 时，Process.isAlive() 仍 true，
        // 只能靠 mtime 探活主动发现 hang → 强杀重启。
        private const val HEARTBEAT_STALE_MS = 8_000L

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

    // 🆕 2026-06-12 Phase 4：心跳文件路径（与 Go 端 ENCV_HEARTBEAT_PATH 同步）
    // Go 进程每次 ffmpeg 调用后 write 当前时间戳到这个文件；
    // Kotlin 1s poll lastModified()，>HEARTBEAT_STALE_MS 没更新即判 hang。
    //
    // 🆕 2026-06-14 关键修复：之前路径逻辑用 `System.getenv("ENCV_SERVING_DIR")`，
    // 但这个 env var 只在 Go 子进程里被设置（ProcessBuilder.environment()），
    // Kotlin 自身 env 是 null → 走 fallback 到 filesDir。
    // 结果：Kotlin 读 filesDir/.encv_heartbeat，Go 写 /storage/emulated/0/.encv_heartbeat
    //      → 两个不同文件 → Kotlin 启动 touch 一次后 8s 内看不到 Go 更新 → 误判 hang 杀 Go
    //
    // 修复：把 heartbeatFile 改为 lazy 委托 resolvedHeartbeatFile()，
    //      resolvedHeartbeatFile() 走和 Go 完全一样的逻辑（看 ENCV_HEARTBEAT_PATH env 或 servingDir 推导）。
    //      同时改用 filesDir 作为最终 fallback（永远可写、不需要存储权限），
    //      避免真机用户拒绝存储权限后 /storage/emulated/0/ 写不了。
    private val heartbeatFile: File by lazy { resolvedHeartbeatFile() }

    /**
     * 解析 heartbeat 文件路径 — 与 Go 端 `servingDir/.encv_heartbeat` 完全一致。
     * 关键：Kotlin 启动 Go 子进程时把 ENCV_HEARTBEAT_PATH 设进 ProcessBuilder.environment()，
     *      这个 env 只在 Go 子进程里。Kotlin 自己读不到，所以必须用同样的
     *      fallback 链：ENCV_CACHE_DIR → servingDir 推导 → filesDir。
     */
    private fun resolvedHeartbeatFile(): File {
        val dir = System.getenv("ENCV_CACHE_DIR")
            ?: System.getenv("ENCV_SERVING_DIR")
            ?: filesDir.absolutePath  // 永远可写，不需要任何权限
        val file = File(dir, ".encv_heartbeat")
        Log.i(TAG, "Heartbeat file path: ${file.absolutePath} (dir writable check next)")
        // 启动时探一次：路径必须可写，否则 mtime monitor 永远 0 → 60s 后才报警
        try {
            file.parentFile?.mkdirs()
            file.writeText(System.currentTimeMillis().toString())
            Log.i(TAG, "Heartbeat file touch OK at startup: ${file.absolutePath}")
        } catch (e: Throwable) {
            Log.w(TAG, "Heartbeat file touch FAILED at startup: ${file.absolutePath} (Kotlin-side monitor will see mtime=0; fall back to filesDir)", e)
        }
        return file
    }

    /**
     * 🆕 2026-06-14 防御：探测 servingDir 可写性，失败则自动降级。
     *
     * 优先级链：
     *   1) caller set `ENCV_SERVING_DIR` env (e.g. Capacitor preview / 沙箱定制)
     *   2) 探测 `/storage/emulated/0` (config.user.json mobile.server.dir 默认值)
     *   3) 探失败 → 降级到 `filesDir.absolutePath` 并持久化到 config
     *   4) 探成功 → 用原值
     *
     * 注意：返回值是 Kotlin 端传给 Go 子进程用的 `ENCV_SERVING_DIR` env value。
     *       Go 端 [internal/server/server.go:Start](file:///workspace/internal/server/server.go) 会
     *       用它**覆盖** mobile overlay 默认的 `/storage/emulated/0`（如果 env 已 set）
     *       或者**作为 fallback**（如果没 set）。两边行为要保持一致 → 见 server.go 注释。
     *
     * 副作用：若降级，会写 `config.user.json` 的 `mobile.server.dir` 字段，
     *        下次启动即使 servingDir 可写也走 filesDir（用户卸载重装才重置）。
     */
    private fun resolveServingDir(configPath: String): String {
        // 1) caller 显式 set
        val envServing = System.getenv("ENCV_SERVING_DIR")
        if (!envServing.isNullOrEmpty()) {
            Log.i(TAG, "resolveServingDir: using caller env ENCV_SERVING_DIR=$envServing (skip probe)")
            return envServing
        }

        // 2) 读 config.user.json 看 mobile.server.dir 是不是已经被降级过
        val configServing = readMobileServerDirFromConfig(configPath) ?: "/storage/emulated/0"
        Log.i(TAG, "resolveServingDir: probing $configServing (from config)")

        // 3) 探写一次
        if (probeDirWritable(configServing)) {
            Log.i(TAG, "resolveServingDir: $configServing writable, keep")
            return configServing
        }

        // 4) 探失败 → 降级到 filesDir（永远可写）
        val fallback = filesDir.absolutePath
        Log.w(TAG, "resolveServingDir: $configServing NOT writable (scoped storage / 权限拒绝), " +
            "降级到 filesDir=$fallback 并持久化到 config")

        try {
            updateMobileServerDirInConfig(configPath, fallback)
            Log.i(TAG, "resolveServingDir: config updated, mobile.server.dir=$fallback")
        } catch (e: Throwable) {
            Log.w(TAG, "resolveServingDir: 写 config 失败（降级仅本次生效）", e)
        }
        return fallback
    }

    private fun readMobileServerDirFromConfig(configPath: String): String? {
        return try {
            val f = File(configPath)
            if (!f.exists()) return null
            val obj = JSONObject(f.readText())
            obj.optJSONObject("mobile")?.optJSONObject("server")?.optString("dir", null)
        } catch (e: Throwable) {
            Log.w(TAG, "readMobileServerDirFromConfig failed", e)
            null
        }
    }

    private fun updateMobileServerDirInConfig(configPath: String, newDir: String) {
        val f = File(configPath)
        val obj = if (f.exists()) {
            try { JSONObject(f.readText()) } catch (e: Throwable) { JSONObject() }
        } else {
            JSONObject()
        }
        val mobile = obj.optJSONObject("mobile") ?: JSONObject().also { obj.put("mobile", it) }
        val server = mobile.optJSONObject("server") ?: JSONObject().also { mobile.put("server", it) }
        server.put("dir", newDir)
        f.writeText(obj.toString(2))
    }

    private fun probeDirWritable(dirPath: String): Boolean {
        return try {
            val dir = File(dirPath)
            if (!dir.exists()) {
                dir.mkdirs()
            }
            // 探写：创 + 删 一个唯一文件
            val probe = File(dir, ".encv_probe_${System.currentTimeMillis()}.tmp")
            probe.writeText("ok")
            val readBack = probe.readText()
            probe.delete()
            val ok = readBack == "ok"
            Log.i(TAG, "probeDirWritable: $dirPath → $ok")
            ok
        } catch (e: Throwable) {
            Log.w(TAG, "probeDirWritable: $dirPath failed", e)
            false
        }
    }

    // 🆕 2026-06-12 崩溃根因修复：goProcess 死时自动重启（带指数退避 + 最多 3 次）
    //   死因：真机 libffmpeg.so 没编 encoder → go_GenerateMP3 走 NativeRunner → cgo panic
    //         cgo panic 跨 cgo boundary 杀整个进程 → 之前前端只看到 "Failed to fetch"
    //   修复：后台线程 1s poll process.isAlive；死 → publishFailure(完整 stderr) → 重启
    private var restartAttempts = 0
    private val MAX_RESTART_ATTEMPTS = 3
    private val restartExecutor = java.util.concurrent.Executors.newSingleThreadScheduledExecutor { r ->
        Thread(r, "encv-go-restart").apply { isDaemon = true }
    }

    override fun onCreate() {
        super.onCreate()
        instance = this
        createNotificationChannel()
        startProcessAliveMonitor()  // 🆕 2026-06-12：进程死后自动重启
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        currentSource = intent?.getStringExtra(EXTRA_SOURCE) ?: "manual"
        intent?.let { it ->
            when (it.action) {
                ACTION_STOP -> worker.execute { stopGoProcess("stopped", stopService = true) }
                ACTION_RESTART, ACTION_EXTERNAL_RESTART -> worker.execute {
                    restartGoProcess(currentSource, it.getStringExtra(EXTRA_COMMAND))
                }
                ACTION_STATUS -> publishStatus(lastError)
                ACTION_START, ACTION_EXTERNAL_START -> worker.execute {
                    startGoProcess(currentSource, it.getStringExtra(EXTRA_COMMAND))
                }
            }
        } ?: run {
            // 无 intent 视为默认 start（向后兼容旧版无 action 调用）
            worker.execute { startGoProcess(currentSource, null) }
        }
        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        instance = null
        stopGoProcess("service_destroyed", stopService = false)
        worker.shutdownNow()
        restartExecutor.shutdownNow()  // 🆕 2026-06-12
        super.onDestroy()
    }

    // 🆕 2026-06-12 Phase 4：mtime 探活 + 自动重启
    //
    // 双判探活：
    //   1) Process.isAlive()=false → 进程真死（exit/crash）→ 老逻辑处理
    //   2) heartbeatFile.lastModified() 超过 HEARTBEAT_STALE_MS 没更新 →
    //      Go 进程 OS 层还活着（cgo ffmpeg_run 阻塞 OS thread 不响应 ctx cancel）
    //      但 Go runtime 调度被污染 → kill + restart
    //
    // 触发后 publishFailure reason="go_hang:..."（前端按 go_hang 分类）
    private fun startProcessAliveMonitor() {
        restartExecutor.scheduleWithFixedDelay({
            val proc = goProcess ?: return@scheduleWithFixedDelay
            val wasReady = processReady.get()
            if (!wasReady) return@scheduleWithFixedDelay  // 启动阶段失败由 startGoProcess 自己处理

            // ① 进程真死 → 老逻辑
            if (!proc.isAlive) {
                processReady.set(false)
                restartAttempts += 1
                val exitCode = try { proc.exitValue() } catch (e: Exception) { -1 }
                val tail = outputBuffer.toString().takeLast(2 * 1024)
                val reason = "go_exit:$exitCode|attempts=$restartAttempts|output:${if (tail.isEmpty()) "(empty)" else tail}"
                publishFailure(reason, "alive_monitor", null)
                lastExitCode = exitCode
                scheduleAutoRestart(reason)
                return@scheduleWithFixedDelay
            }

            // 进程还活着 → 检查 mtime 心跳
            val mtimeMs = try { heartbeatFile.lastModified() } catch (e: Throwable) { 0L }
            if (mtimeMs == 0L) {
                // 心跳文件还没建（go 进程刚启动还没跑过 ffmpeg）→ 不判 hang
                // 但要等首次 ffmpeg 调用 5s 内必须更新；如果等超过 60s 还没建文件
                // 说明 Go 端 ENCV_HEARTBEAT_PATH 配置错 — 这种情况判定为 hang
                if (restartAttempts > 0 && (restartAttempts * 1_000L) > 60_000L) {
                    handleHang("heartbeat_file_never_created_after_${restartAttempts}s")
                }
                return@scheduleWithFixedDelay
            }
            val ageMs = System.currentTimeMillis() - mtimeMs
            if (ageMs > HEARTBEAT_STALE_MS) {
                handleHang("heartbeat_stale_${ageMs}ms")
                return@scheduleWithFixedDelay
            }

            // 进程健康 — 重置重启计数
            if (restartAttempts > 0) {
                restartAttempts = 0
            }
        }, 1, 1, java.util.concurrent.TimeUnit.SECONDS)
    }

    // 🆕 2026-06-12 Phase 4：mtime 探活触发 hang 处理
    //   - 不依赖 isAlive()（cgo hang 时仍 true）
    //   - 强杀 + 1s/2s/4s 退避重启
    //   - 通过 publishFailure 推 CustomEvent('encv:backend-status') 给前端
    private fun handleHang(reason: String) {
        processReady.set(false)
        restartAttempts += 1
        val tail = outputBuffer.toString().takeLast(2 * 1024)
        val fullReason = "go_hang:$reason|attempts=$restartAttempts|output:${if (tail.isEmpty()) "(empty)" else tail}"
        Log.e(TAG, "⚠️  Backend hang detected: $reason — destroying and scheduling restart")
        publishFailure(fullReason, "alive_monitor", null)
        try {
            goProcess?.destroyForcibly()
        } catch (e: Throwable) {
            Log.w(TAG, "destroyForcibly failed", e)
        }
        scheduleAutoRestart(fullReason)
    }

    // 提取自 startProcessAliveMonitor，复用于 go_exit / go_hang 两条路径
    private fun scheduleAutoRestart(reason: String) {
        if (restartAttempts > MAX_RESTART_ATTEMPTS) {
            Log.e(TAG, "❌ Go process died/hung $restartAttempts times; giving up auto-restart")
            return
        }
        val delayMs = (1L shl (restartAttempts - 1)) * 1000L  // 1s / 2s / 4s
        Log.w(TAG, "Auto-restart in ${delayMs}ms (attempt ${restartAttempts}/${MAX_RESTART_ATTEMPTS}): $reason")
        restartExecutor.schedule({
            try {
                startGoProcess("auto_restart:$reason", null)
            } catch (e: Throwable) {
                Log.e(TAG, "auto-restart failed", e)
            }
        }, delayMs, java.util.concurrent.TimeUnit.MILLISECONDS)
    }

    // 🆕 2026-06-12 Phase 4：启动后立即 touch 心跳文件，避免 8s 内 mtime=0 误判 hang
    //   Go 进程接管后每次 ffmpeg 调用会自己更新 mtime。
    //   这里只是 initial bootstrap。
    private fun touchHeartbeat() {
        try {
            heartbeatFile.parentFile?.mkdirs()
            heartbeatFile.writeText(System.currentTimeMillis().toString())
        } catch (e: Throwable) {
            Log.w(TAG, "Failed to touch heartbeat file (探活降级为仅 isAlive)", e)
        }
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

            // 🆕 2026-06-14：先探测 servingDir 是否可写，不可写则降级到 filesDir。
            //
            // 背景：Android 11+ scoped storage 限制 + 真机用户拒绝存储权限 →
            //   /storage/emulated/0 写不了（即使是 app 自己的目录）→ Go 端
            //   servingDir/.encv_heartbeat 写失败 → 即使 mtime 路径一致 Go 也写不了。
            //
            // 防御：每次启动 Go 前探写一次，失败就改写 config.user.json 的
            //   mobile.server.dir 到 filesDir（永远可写、无需权限），并把同
            //   路径也写进 ENCV_HEARTBEAT_PATH。
            val servingDir = resolveServingDir(configPath)
            Log.i(TAG, "Resolved servingDir=$servingDir (heartbeat=${heartbeatFile.absolutePath})")

            goProcess = ProcessBuilder(binary.absolutePath, "start").apply {
                environment()["ENCV_CONFIG_PATH"] = configPath
                environment()["ENCV_MOBILE"] = "1"
                environment()["HOME"] = filesDir.absolutePath
                // 🆕 2026-06-11 Phase 2: 跟 ENCV_LIB_DIR 同源
                // - ENCV_LIB_DIR 给 cgo CallFFmpegNative 用（dlopen libffmpeg.so）
                // - ENCV_FFMPEG_WORKER 给 ffmpeg-worker 路径用（workerClient.locateWorker 优先选这个）
                //   改用 subprocess worker 调 ffmpeg 后，父进程 ctx cancel 时可以 SIGKILL worker
                //   解锁（之前 in-process cgo 阻塞 OS thread 没法 cancel，hang spinner forever）
                environment()["ENCV_LIB_DIR"] = applicationInfo.nativeLibraryDir
                environment()["ENCV_FFMPEG_WORKER"] =
                    File(applicationInfo.nativeLibraryDir, "libffmpeg-worker.so").absolutePath
                // 🆕 2026-06-14：把探测后的 servingDir 显式 set 给 Go（覆盖 mobile overlay 的默认值）
                environment()["ENCV_SERVING_DIR"] = servingDir
                // 🆕 2026-06-14 关键修复：显式 set ENCV_HEARTBEAT_PATH 给 Go 子进程，
                // 让 Go 和 Kotlin 读**同一个文件**。这之前是隐性 bug：
                //   - Go 端：`cfg.Server.Dir` 经 mobile overlay → `/storage/emulated/0`
                //   - Kotlin 端：`System.getenv("ENCV_SERVING_DIR")` 在自己 env 是 null
                //                  → fallback 到 filesDir
                //   → 两个进程读不同的文件，Kotlin 看到的 mtime 永远不更新 → 8s 误判 hang
                //
                // 修复策略：把 Kotlin 自己用的 heartbeatFile 路径**原样**塞给 Go。
                // 路径 = `filesDir.absolutePath/.encv_heartbeat`（永远可写、无需任何权限）。
                environment()["ENCV_HEARTBEAT_PATH"] = heartbeatFile.absolutePath
                Log.i(TAG, "Go heartbeat file: ${heartbeatFile.absolutePath}")
                redirectErrorStream(true)
                directory(filesDir)
            }.start()

            // 🆕 2026-06-12 Phase 4：bootstrap 心跳文件 — Kotlin 端 1s poll mtime 前必须有文件
            touchHeartbeat()

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
                if (!serverObj.has("dir")) {
                    serverObj.put("dir", defaultServer.optString("dir", ""))
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
                if (!targetMobile.has("server")) {
                    targetMobile.put("server", defaultMobile.optJSONObject("server") ?: JSONObject())
                    changed = true
                }
                if (!targetMobile.has("output")) {
                    targetMobile.put("output", defaultMobile.optJSONObject("output") ?: JSONObject())
                    changed = true
                }
                if (!targetMobile.has("webdav")) {
                    targetMobile.put("webdav", defaultMobile.optJSONObject("webdav") ?: JSONObject())
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
                put("server", JSONObject().put("dir", "/storage/emulated/0"))
                put("output", JSONObject().put("path", "/storage/emulated/0/encv-output"))
                put("webdav", JSONObject().put("dir", ""))
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
