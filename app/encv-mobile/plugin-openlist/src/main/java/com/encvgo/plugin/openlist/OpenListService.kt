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
        const val BROADCAST_STATUS_CHANGED = "com.encvgo.plugin.openlist.BROADCAST_STATUS_CHANGED"
        const val BROADCAST_PORT_CONFLICT = "com.encvgo.plugin.openlist.BROADCAST_PORT_CONFLICT"
        const val BROADCAST_LOG = "com.encvgo.plugin.openlist.BROADCAST_LOG"
        const val BROADCAST_PROCESS_EXIT = "com.encvgo.plugin.openlist.BROADCAST_PROCESS_EXIT"

        const val EXTRA_PORT = "port"
        const val EXTRA_RUNNING = "running"
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
        super.onCreate()
        instance = this
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
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
        instance = null
        handler.removeCallbacks(dbSyncRunnable)
        releaseWakeLock()
        worker.shutdownNow()
        super.onDestroy()
    }

    private fun startupSequence() {
        val cfg = OpenListConfig.load(this)
        val port = cfg.port
        currentPort = port

        if (isPortOccupied(port)) {
            Log.w(TAG, "Port $port already in use, broadcasting PORT_CONFLICT")
            LocalBroadcastManager.getInstance(this).sendBroadcast(
                Intent(BROADCAST_PORT_CONFLICT).putExtra(EXTRA_CONFLICT_PORT, port)
            )
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
            return
        }

        try {
            cfg.applyToBridge(OpenListBridge)
            OpenListBridge.init(this)
            OpenListBridge.start()
            isRunning = true
            publishStatus(port, true)
            updateNotification("OpenList 运行中 :$port")
            handler.postDelayed(dbSyncRunnable, DB_SYNC_INTERVAL_MS)
        } catch (e: Exception) {
            Log.e(TAG, "OpenList startup failed", e)
            publishStatus(0, false)
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
        }
    }

    private fun shutdownSequence() {
        handler.removeCallbacks(dbSyncRunnable)
        try {
            OpenListBridge.shutdown(5_000L)
        } catch (e: Exception) {
            Log.w(TAG, "OpenList shutdown error", e)
        }
        isRunning = false
        currentPort = 0
        publishStatus(0, false)
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private fun isPortOccupied(port: Int): Boolean {
        return try {
            Socket().use { socket ->
                socket.connect(InetSocketAddress("127.0.0.1", port), PORT_CONFLICT_TIMEOUT_MS)
                true
            }
        } catch (_: Exception) {
            false
        }
    }

    private fun publishStatus(port: Int, running: Boolean) {
        LocalBroadcastManager.getInstance(this).sendBroadcast(
            Intent(BROADCAST_STATUS_CHANGED)
                .putExtra(EXTRA_PORT, port)
                .putExtra(EXTRA_RUNNING, running)
        )
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
