package com.encvgo.plugin.openlist

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.os.PowerManager
import android.util.Log
import androidx.core.app.NotificationCompat
import androidx.localbroadcastmanager.content.LocalBroadcastManager
import java.net.InetSocketAddress
import java.net.Socket
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean

class OpenListService : Service() {
    companion object {
        private const val TAG = "OpenList-Service"
        private const val CHANNEL_ID = "openlist_server"
        private const val FOREGROUND_ID = 5224
        private const val DEFAULT_PORT = 5244
        private const val PORT_CONFLICT_TIMEOUT_MS = 2_000
        private const val DB_SYNC_INTERVAL_MS = 5 * 60 * 1_000L

        const val ACTION_SHUTDOWN = "com.encvgo.plugin.openlist.ACTION_SHUTDOWN"
        const val BROADCAST_PORT_CONFLICT = "com.encvgo.plugin.openlist.BROADCAST_PORT_CONFLICT"
        const val BROADCAST_LOG = "com.encvgo.plugin.openlist.BROADCAST_LOG"
        const val EXTRA_CONFLICT_PORT = "conflict_port"

        @Volatile
        var isRunning: Boolean = false
            private set

        @Volatile
        var currentPort: Int = 0
            private set

        @Volatile
        private var instance: OpenListService? = null

        fun getInstance(): OpenListService? = instance

        /**
         * Phase 0 合规修复：供 OpenListPluginEntry.onUnload() 调用。
         * 通知当前运行的 Service 优雅停止（如未运行则 no-op）。
         */
        fun stopIfRunning() {
            val ctx = instance?.applicationContext ?: return
            val intent = Intent(ctx, OpenListService::class.java).apply {
                action = ACTION_SHUTDOWN
            }
            try {
                ctx.startService(intent)
            } catch (e: Exception) {
                Log.w(TAG, "stopIfRunning: startService failed (maybe Service not declared)", e)
            }
        }
    }

    private val handler = Handler(Looper.getMainLooper())
    private val worker = Executors.newSingleThreadExecutor()
    private val started = AtomicBoolean(false)
    private var wakeLock: PowerManager.WakeLock? = null
    private val dbSyncRunnable = object : Runnable {
        override fun run() {
            try {
                OpenListBridge.forceDbSync()
            } catch (e: Exception) {
                Log.w(TAG, "forceDbSync failed", e)
            }
            handler.postDelayed(this, DB_SYNC_INTERVAL_MS)
        }
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        Log.e(TAG, "[SAT-DBG][OpenList] onCreate() | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        super.onCreate()
        instance = this
        createNotificationChannel()
        Log.e(TAG, "[SAT-DBG][OpenList] onCreate() done | notification channel created")
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        Log.e(TAG, "[SAT-DBG][OpenList] onStartCommand() | action=${intent?.action} flags=$flags startId=$startId | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        startForeground(FOREGROUND_ID, buildNotification("OpenList 启动中"))
        acquireWakeLock()
        if (started.compareAndSet(false, true)) {
            worker.execute { startupSequence() }
        }
        if (intent?.action == ACTION_SHUTDOWN) {
            worker.execute { shutdownSequence() }
        }
        return START_STICKY
    }

    override fun onDestroy() {
        Log.e(TAG, "[SAT-DBG][OpenList] onDestroy() | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        instance = null
        handler.removeCallbacks(dbSyncRunnable)
        releaseWakeLock()
        worker.shutdownNow()
        super.onDestroy()
        Log.e(TAG, "[SAT-DBG][OpenList] onDestroy() done")
    }

    private fun startupSequence() {
        Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() begin | thread=${Thread.currentThread().name}")

        // Step 1: load config
        val t0 = System.currentTimeMillis()
        Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step1: loading config...")
        val cfg = OpenListConfig.load(this)
        val port = cfg.port
        currentPort = port
        Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step1 done: config loaded | port=$port dataDir=${cfg.dataDir} | elapsed=${System.currentTimeMillis() - t0}ms")

        // Step 2: port check
        val t1 = System.currentTimeMillis()
        Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step2: checking port $port...")
        if (isPortOccupied(port)) {
            Log.w(TAG, "Port $port already in use, broadcasting PORT_CONFLICT")
            LocalBroadcastManager.getInstance(this).sendBroadcast(
                Intent(BROADCAST_PORT_CONFLICT).putExtra(EXTRA_CONFLICT_PORT, port)
            )
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step2 failed: PORT_CONFLICT at $port | elapsed=${System.currentTimeMillis() - t1}ms")
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
            return
        }
        Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step2 done: port $port is free | elapsed=${System.currentTimeMillis() - t1}ms")

        // Step 3: apply config to bridge
        val t2 = System.currentTimeMillis()
        Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step3: applying config to bridge...")
        try {
            cfg.applyToBridge(OpenListBridge)
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step3 done: config applied | elapsed=${System.currentTimeMillis() - t2}ms")

            // Step 4: bridge init
            val t3 = System.currentTimeMillis()
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step4: initializing bridge...")
            OpenListBridge.init(this)
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step4 done: bridge initialized | elapsed=${System.currentTimeMillis() - t3}ms")

            // Step 5: bridge start
            val t4 = System.currentTimeMillis()
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step5: starting bridge...")
            OpenListBridge.start()
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step5 done: bridge started | elapsed=${System.currentTimeMillis() - t4}ms")

            isRunning = true
            publishStatus(port, true)
            updateNotification("OpenList 运行中 :$port")
            handler.postDelayed(dbSyncRunnable, DB_SYNC_INTERVAL_MS)
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() complete | total=${System.currentTimeMillis() - t0}ms")
        } catch (e: Exception) {
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() FAILED | elapsed=${System.currentTimeMillis() - t0}ms", e)
            publishStatus(0, false)
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
        }
    }

    private fun shutdownSequence() {
        Log.e(TAG, "[SAT-DBG][OpenList] shutdownSequence() begin | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        handler.removeCallbacks(dbSyncRunnable)
        try {
            Log.e(TAG, "[SAT-DBG][OpenList] shutdownSequence() calling bridge shutdown...")
            OpenListBridge.shutdown(5_000L)
            Log.e(TAG, "[SAT-DBG][OpenList] shutdownSequence() bridge shutdown returned")
        } catch (e: Exception) {
            Log.w(TAG, "OpenList shutdown error", e)
            Log.e(TAG, "[SAT-DBG][OpenList] shutdownSequence() bridge shutdown error", e)
        }
        isRunning = false
        currentPort = 0
        publishStatus(0, false)
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
        Log.e(TAG, "[SAT-DBG][OpenList] shutdownSequence() done")
    }

    private fun isPortOccupied(port: Int): Boolean {
        val t0 = System.currentTimeMillis()
        return try {
            Socket().use { socket ->
                socket.connect(InetSocketAddress("127.0.0.1", port), PORT_CONFLICT_TIMEOUT_MS)
                val elapsed = System.currentTimeMillis() - t0
                Log.e(TAG, "[SAT-DBG][OpenList] isPortOccupied() true | port=$port | connectElapsed=${elapsed}ms")
                true
            }
        } catch (_: Exception) {
            false
        }
    }

    /**
     * Phase 23: 状态变更推送（in-process 替代轮询）。
     * 历史：Phase 22 误用 LocalBroadcastManager + 跨进程系统广播，host 收不到。
     * 现在：直接调 [OpenListBridge.broadcastStatus] 触发已注册的 [OpenListBridge.statusListener]
     *       （host 启动时通过 PluginClassLoader 反射注册）。
     */
    private fun publishStatus(port: Int, running: Boolean) {
        OpenListBridge.broadcastStatus(port, running)
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val manager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
            val channel = NotificationChannel(
                CHANNEL_ID,
                "OpenList Server",
                NotificationManager.IMPORTANCE_LOW
            )
            channel.setShowBadge(false)
            channel.lockscreenVisibility = Notification.VISIBILITY_PRIVATE
            manager.createNotificationChannel(channel)
        }
    }

    private fun buildNotification(text: String): Notification {
        val flags = PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        val openIntent = packageManager.getLaunchIntentForPackage(packageName)
        val pendingIntent = if (openIntent != null) {
            PendingIntent.getActivity(this, 0, openIntent, flags)
        } else {
            PendingIntent.getService(this, 0, Intent(this, OpenListService::class.java), flags)
        }
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setContentTitle("OpenList")
            .setContentText(text)
            .setContentIntent(pendingIntent)
            .setOngoing(true)
            .build()
    }

    private fun updateNotification(text: String) {
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        manager.notify(FOREGROUND_ID, buildNotification(text))
    }

    private fun acquireWakeLock() {
        if (wakeLock?.isHeld == true) return
        try {
            val pm = getSystemService(Context.POWER_SERVICE) as PowerManager
            wakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "openlist::Service")
            wakeLock?.acquire()
        } catch (e: Exception) {
            Log.w(TAG, "Failed to acquire WakeLock", e)
        }
    }

    private fun releaseWakeLock() {
        wakeLock?.let {
            if (it.isHeld) {
                try {
                    it.release()
                } catch (e: Exception) {
                    Log.w(TAG, "Failed to release WakeLock", e)
                }
            }
        }
        wakeLock = null
    }
}