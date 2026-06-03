package com.encvgo.plugin.openlist

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.PowerManager
import android.util.Log
import androidx.core.app.NotificationCompat
import com.combo.core.component.service.BasePluginService
import java.net.InetSocketAddress
import java.net.Socket
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Phase 25 架构正确化（ComboLite §10.3）：OpenList plugin 改成 IPluginService 实现。
 *
 * 历史：
 *   - Phase 24 之前：`class OpenListService : Service()` + manifest `<service>` 声明
 *     状态：永远不会被系统实例化（plugin 没系统 install），但 startupSequence 逻辑
 *           是手写在 OpenListService.onStartCommand 里，**没有任何路径触发**。
 *     运行时实际启动靠 host 通过 classloader 反射 `OpenListBridge.start()`。
 *   - Phase 24：保留 `extends Service` 死代码，仅在反射 `OpenListBridge` 上做改进。
 *
 * 现在（Phase 25 方案 A）：
 *   - 改为 `class OpenListPluginService : BasePluginService()`（IPluginService 实现）
 *   - 删 manifest `<service>` 声明（plugin service 是 POJO，框架走 BaseHostService 代理）
 *   - Context 来源：`proxyService`（host manifest 注册的 BaseHostService 实例）
 *   - 启动入口（待 A2 落地）：
 *       host 端 `context.startPluginService(OpenListPluginService::class.java, "main")`
 *       → ProxyManager.acquireServiceProxy → Intent 指向 host service
 *       → BaseHostService.onStartCommand → initPluginService → newInstance() + onAttach + onCreate
 *       → onAttach(proxyService) 注入 host service 引用（plugin 拿 Context）
 *       → onCreate() (本类) → 走 startupSequence
 *   - A2 落地前运行时仍由 host 反射 `OpenListBridge.start()` 走 in-process 路径；
 *     OpenListPluginService 类只是**架构正确**的占位，没有被启动过。
 *
 * 关键适配（BasePluginService → Service API 转换）：
 *   - `getSystemService(...)` → `proxyService!!.getSystemService(...)`（必须 non-null）
 *   - `startForeground(id, notif)` → `proxyService!!.startForeground(id, notif)`
 *   - `stopForeground(flag)` → `proxyService!!.stopForeground(flag)`
 *   - `stopSelf()` → `proxyService!!.stopSelf()`
 *   - `packageManager` → `proxyService!!.packageManager`
 *   - `applicationContext` → `proxyService!!.applicationContext` (或 `proxyService!!`)
 */
class OpenListPluginService : BasePluginService() {

    companion object {
        private const val TAG = "OpenList-PluginService"
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

        /**
         * 当前活跃的 plugin service 实例（如果有）。
         * host 在 [com.encvgo.plugin.openlist.OpenListPluginEntry.onUnload] 关闭时
         * 通过 `service.shutdown()` 优雅停止（如未运行则 no-op）。
         */
        @Volatile
        private var instance: OpenListPluginService? = null

        /**
         * 拿到当前 service 实例（用于 host 在 onUnload 触发 graceful shutdown）。
         * 与历史 `OpenListService.getInstance()` 等价语义。
         */
        fun getInstance(): OpenListPluginService? = instance

        /**
         * 合规修复（Phase 0）：供 OpenListPluginEntry.onUnload() 调用。
         * 通知当前运行的 service 优雅停止（如未运行则 no-op）。
         * 与历史 OpenListService.stopIfRunning() 行为一致。
         */
        fun stopIfRunning() {
            val svc = instance
            if (svc == null) {
                Log.w(TAG, "stopIfRunning: no active instance (service not started via startPluginService)")
                return
            }
            svc.shutdownFromExternal()
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

    // === IPluginService lifecycle ===

    override fun onCreate() {
        Log.e(TAG, "[SAT-DBG][OpenList] OpenListPluginService.onCreate() | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        super.onCreate()
        val ctx = proxyService
        if (ctx == null) {
            Log.e(TAG, "[SAT-DBG][OpenList] onCreate() FAILED: proxyService is null (host didn't call onAttach)")
            return
        }
        instance = this
        try {
            createNotificationChannel(ctx)
        } catch (e: Throwable) {
            Log.e(TAG, "[SAT-DBG][OpenList] createNotificationChannel FAILED", e)
        }
        Log.e(TAG, "[SAT-DBG][OpenList] onCreate() done | notification channel created")
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val portOverride = intent?.getIntExtra("port", -1) ?: -1
        Log.e(TAG, "[SAT-DBG][OpenList] onStartCommand() | action=${intent?.action} portOverride=$portOverride flags=$flags startId=$startId | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        val ctx = proxyService
        if (ctx == null) {
            Log.e(TAG, "[SAT-DBG][OpenList] onStartCommand() FAILED: proxyService is null")
            return android.app.Service.START_NOT_STICKY
        }
        try {
            ctx.startForeground(FOREGROUND_ID, buildNotification(ctx, "OpenList 启动中"))
        } catch (e: Throwable) {
            Log.e(TAG, "[SAT-DBG][OpenList] startForeground FAILED", e)
        }
        acquireWakeLock()
        if (started.compareAndSet(false, true)) {
            worker.execute { startupSequence(portOverride) }
        }
        if (intent?.action == ACTION_SHUTDOWN) {
            worker.execute { shutdownSequence() }
        }
        return android.app.Service.START_STICKY
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

    /**
     * 外部触发优雅停止（[stopIfRunning] 调用）。等价于在 plugin service 上
     * 派发 ACTION_SHUTDOWN intent——通过 worker 走 shutdownSequence。
     */
    private fun shutdownFromExternal() {
        try {
            worker.execute { shutdownSequence() }
        } catch (e: Throwable) {
            Log.w(TAG, "shutdownFromExternal FAILED", e)
        }
    }

    // === 业务逻辑（从历史 OpenListService 搬过来，几乎原样） ===

    private fun startupSequence(portOverride: Int = -1) {
        Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() begin | portOverride=$portOverride | thread=${Thread.currentThread().name}")
        val ctx = proxyService ?: run {
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() FAILED: proxyService null")
            return
        }

        // Step 1: load config
        val t0 = System.currentTimeMillis()
        Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step1: loading config...")
        val cfg = OpenListConfig.load(ctx)
        val port = if (portOverride > 0) {
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step1: portOverride=$portOverride → override config port ${cfg.port}")
            portOverride
        } else {
            cfg.port
        }
        currentPort = port
        Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step1 done: config loaded | port=$port dataDir=${cfg.dataDir} | elapsed=${System.currentTimeMillis() - t0}ms")

        // Step 2: port check
        val t1 = System.currentTimeMillis()
        Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step2: checking port $port...")
        if (isPortOccupied(port)) {
            Log.w(TAG, "Port $port already in use, broadcasting PORT_CONFLICT")
            // PORT_CONFLICT 仍走 LocalBroadcastManager (host 监听, plugin 内 0 receiver 监听到——保留原行为)
            androidx.localbroadcastmanager.content.LocalBroadcastManager.getInstance(ctx).sendBroadcast(
                Intent(BROADCAST_PORT_CONFLICT).putExtra(EXTRA_CONFLICT_PORT, port)
            )
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step2 failed: PORT_CONFLICT at $port | elapsed=${System.currentTimeMillis() - t1}ms")
            try { ctx.stopForeground(android.app.Service.STOP_FOREGROUND_REMOVE) } catch (_: Throwable) {}
            try { ctx.stopSelf() } catch (_: Throwable) {}
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
            OpenListBridge.init(ctx)
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step4 done: bridge initialized | elapsed=${System.currentTimeMillis() - t3}ms")

            // Step 5: bridge start
            val t4 = System.currentTimeMillis()
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step5: starting bridge...")
            OpenListBridge.start()
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() step5 done: bridge started | elapsed=${System.currentTimeMillis() - t4}ms")

            isRunning = true
            OpenListBridge.broadcastStatus(port, true)
            try { updateNotification(ctx, "OpenList 运行中 :$port") } catch (_: Throwable) {}
            handler.postDelayed(dbSyncRunnable, DB_SYNC_INTERVAL_MS)
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() complete | total=${System.currentTimeMillis() - t0}ms")
        } catch (e: Exception) {
            Log.e(TAG, "[SAT-DBG][OpenList] startupSequence() FAILED | elapsed=${System.currentTimeMillis() - t0}ms", e)
            OpenListBridge.broadcastStatus(0, false)
            try { ctx.stopForeground(android.app.Service.STOP_FOREGROUND_REMOVE) } catch (_: Throwable) {}
            try { ctx.stopSelf() } catch (_: Throwable) {}
        }
    }

    private fun shutdownSequence() {
        Log.e(TAG, "[SAT-DBG][OpenList] shutdownSequence() begin | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        val ctx = proxyService
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
        OpenListBridge.broadcastStatus(0, false)
        if (ctx != null) {
            try { ctx.stopForeground(android.app.Service.STOP_FOREGROUND_REMOVE) } catch (_: Throwable) {}
            try { ctx.stopSelf() } catch (_: Throwable) {}
        }
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

    // === Notification / WakeLock 工具方法（接受 Context 参数） ===

    private fun createNotificationChannel(ctx: Context) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val manager = ctx.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
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

    private fun buildNotification(ctx: Context, text: String): Notification {
        val flags = PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        val openIntent = ctx.packageManager.getLaunchIntentForPackage(ctx.packageName)
        val pendingIntent = if (openIntent != null) {
            PendingIntent.getActivity(ctx, 0, openIntent, flags)
        } else {
            // plugin 端 fallback：调 proxyService 自身（host service 实例）
            val proxy = proxyService
            val pi = if (proxy != null) {
                PendingIntent.getService(ctx, 0, Intent(ctx, proxy.javaClass), flags)
            } else {
                PendingIntent.getService(ctx, 0, Intent(), flags)
            }
            pi
        }
        return NotificationCompat.Builder(ctx, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setContentTitle("OpenList")
            .setContentText(text)
            .setContentIntent(pendingIntent)
            .setOngoing(true)
            .build()
    }

    private fun updateNotification(ctx: Context, text: String) {
        val manager = ctx.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        manager.notify(FOREGROUND_ID, buildNotification(ctx, text))
    }

    private fun acquireWakeLock() {
        if (wakeLock?.isHeld == true) return
        val ctx = proxyService ?: return
        try {
            val pm = ctx.getSystemService(Context.POWER_SERVICE) as PowerManager
            wakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "openlist::PluginService")
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
