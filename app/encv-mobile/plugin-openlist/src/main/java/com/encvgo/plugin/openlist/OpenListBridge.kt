package com.encvgo.plugin.openlist

import android.content.Context
import android.content.Intent
import android.os.Handler
import android.os.Looper
import android.os.Process
import android.util.Log
import androidx.localbroadcastmanager.content.LocalBroadcastManager
import openlistlib.Event
import openlistlib.LogCallback
import openlistlib.Openlistlib

/**
 * Kotlin bridge around the `openlistlib` gomobile-bind AAR.
 *
 * gomobile bind shape (https://go.dev/talks/2015/gophercon-go-on-mobile.slide §16):
 *   public abstract class openlistlib.Openlistlib {
 *     public static void SetConfigData(String path);
 *     public static void SetConfigAdminPassword(String pwd);
 *     public static void Init(Event e, LogCallback cb) throws Exception;
 *     public static void Start();
 *     public static void Shutdown(long timeoutMs) throws Exception;
 *     public static boolean IsRunning(String protocol);  // "" == any
 *     public static void ForceDBSync() throws Exception;
 *   }
 *
 * Rules enforced here:
 *   1. Openlistlib is abstract + private ctor → we only call its STATIC methods.
 *   2. Method names preserve Go case (capital S/A/P/I/R/F/D/B).
 *   3. Init/Shutdown/ForceDBSync may throw — wrap in try/catch.
 *   4. Port is NOT an openlistlib API: it lives in on-disk conf.Conf.Scheme.HttpPort
 *      and is read by `Start()` from there. We don't try to set it via the lib.
 *   5. SetConfigData(dataDir) tells openlistlib where to look for the config dir.
 */
object OpenListBridge : Event, LogCallback {

    private const val TAG = "OpenList"

    private val lock = Any()

    private val handler = Handler(Looper.getMainLooper())

    @Volatile
    private var dataDir: String = ""
    @Volatile
    private var port: Int = 0
    @Volatile
    private var running: Boolean = false
    @Volatile
    private var appContext: Context? = null
    @Volatile
    private var initialized: Boolean = false

    // === Snapshot fields (read by OpenListStatusProvider via snapshot()) ===
    @Volatile
    private var pid: Int = 0
    @Volatile
    private var dataSizeBytes: Long = 0L
    @Volatile
    private var lastError: String? = null
    @Volatile
    private var lastUpdateTs: Long = 0L

    /**
     * One-time init: register this object (which implements Event + LogCallback)
     * with the openlistlib static Init() entry. Idempotent. Safe to call from
     * multiple call sites (Provider.onCreate, PluginEntry.onLoad, Service.onStart).
     */
    fun init(context: Context) {
        Log.e(TAG, "[SAT-DBG][OpenList] init() entry | dataDir=$dataDir port=$port | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        appContext = context.applicationContext
        synchronized(lock) {
            // Apply data dir every time (cheap, idempotent; gomobile sets a global flag).
            try {
                if (dataDir.isNotEmpty()) {
                    Openlistlib.SetConfigData(dataDir)
                    Log.e(TAG, "[SAT-DBG][OpenList] init() Openlistlib.SetConfigData($dataDir) OK")
                }
            } catch (e: Throwable) {
                lastError = "SetConfigData failed: ${e.message}"
                lastUpdateTs = System.currentTimeMillis()
                Log.e(TAG, "[SAT-DBG][OpenList] init() Openlistlib.SetConfigData FAILED", e)
            }
            if (!initialized) {
                try {
                    Openlistlib.Init(this, this)
                    initialized = true
                    lastUpdateTs = System.currentTimeMillis()
                    Log.e(TAG, "[SAT-DBG][OpenList] init() Openlistlib.Init() OK")
                } catch (e: Throwable) {
                    lastError = "Init failed: ${e.message}"
                    lastUpdateTs = System.currentTimeMillis()
                    Log.e(TAG, "[SAT-DBG][OpenList] init() Openlistlib.Init() FAILED", e)
                }
            } else {
                Log.e(TAG, "[SAT-DBG][OpenList] init() skipped (already initialized)")
            }
        }
    }

    /**
     * Start the OpenList server (HTTP/HTTPS/Unix + S3/FTP/SFTP per on-disk config).
     * Non-blocking: Start() returns quickly; listeners run on internal goroutines.
     * The actual port is read from on-disk conf.Conf.Scheme.HttpPort; we don't
     * pass it to openlistlib.
     */
    fun start() {
        Log.e(TAG, "[SAT-DBG][OpenList] start() entry | running=$running | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        synchronized(lock) {
            if (!initialized) {
                lastError = "start() called before init()"
                lastUpdateTs = System.currentTimeMillis()
                Log.e(TAG, "[SAT-DBG][OpenList] start() FAILED: not initialized")
                return
            }
            if (running) {
                Log.e(TAG, "[SAT-DBG][OpenList] start() skipped (already running)")
                return
            }
        }
        Thread {
            Log.e(TAG, "[SAT-DBG][OpenList] start() background thread | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
            try {
                Openlistlib.Start()
                val nativePid = Process.myPid()
                val ts = System.currentTimeMillis()
                synchronized(lock) {
                    pid = nativePid
                    running = true
                    lastUpdateTs = ts
                }
                Log.e(TAG, "[SAT-DBG][OpenList] start() Openlistlib.Start() returned | pid=$nativePid | ts=$ts")
                broadcastStatus(port, true)
            } catch (e: Throwable) {
                synchronized(lock) {
                    lastError = "start failed: ${e.message}"
                    lastUpdateTs = System.currentTimeMillis()
                }
                Log.e(TAG, "[SAT-DBG][OpenList] start() Openlistlib.Start() threw", e)
            }
        }.start()
    }

    /**
     * Graceful shutdown with a timeout (milliseconds). If the openlistlib goroutines
     * haven't finished by ~500ms after the call returns, we force running=false
     * locally (the actual onShutdown callback will fire when goroutines really
     * drain).
     */
    fun shutdown(timeoutMs: Long) {
        Log.e(TAG, "[SAT-DBG][OpenList] shutdown() entry | timeoutMs=$timeoutMs | running=$running | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        synchronized(lock) {
            if (!running) {
                Log.e(TAG, "[SAT-DBG][OpenList] shutdown() skipped (not running)")
                return
            }
        }
        Thread {
            try {
                Openlistlib.Shutdown(timeoutMs)
                Log.e(TAG, "[SAT-DBG][OpenList] shutdown() Openlistlib.Shutdown() returned")
            } catch (e: Throwable) {
                synchronized(lock) {
                    lastError = "shutdown error: ${e.message}"
                    lastUpdateTs = System.currentTimeMillis()
                }
                Log.e(TAG, "[SAT-DBG][OpenList] shutdown() Openlistlib.Shutdown() threw", e)
            }
        }.start()
        // Grace timer: if running is still true after 500ms, force it false.
        handler.postDelayed({
            synchronized(lock) {
                if (running) {
                    Log.e(TAG, "[SAT-DBG][OpenList] shutdown() grace timer expired | force-running=false")
                    running = false
                    lastUpdateTs = System.currentTimeMillis()
                }
            }
        }, 500)
    }

    fun isRunning(): Boolean = synchronized(lock) { running }

    fun isRunning(protocol: String): Boolean = try {
        Openlistlib.IsRunning(protocol)
    } catch (e: Throwable) {
        Log.e(TAG, "[SAT-DBG][OpenList] IsRunning($protocol) threw", e)
        false
    }

    /**
     * Force a SQLite WAL checkpoint. Invoked via the control URI by the host.
     */
    fun forceDbSync() {
        Log.e(TAG, "[SAT-DBG][OpenList] forceDbSync() entry | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        Thread {
            try {
                Openlistlib.ForceDBSync()
                val ts = System.currentTimeMillis()
                synchronized(lock) { lastUpdateTs = ts }
                Log.e(TAG, "[SAT-DBG][OpenList] forceDbSync() done | ts=$ts")
            } catch (e: Throwable) {
                synchronized(lock) {
                    lastError = "force_db_sync error: ${e.message}"
                    lastUpdateTs = System.currentTimeMillis()
                }
                Log.e(TAG, "[SAT-DBG][OpenList] forceDbSync() threw", e)
            }
        }.start()
    }

    /**
     * Reset the admin password and clear its login cache. Wraps
     * Openlistlib.SetAdminPassword; no return value to check.
     */
    fun setAdminPassword(pwd: String) {
        Log.e(TAG, "[SAT-DBG][OpenList] setAdminPassword() entry | length=${pwd.length} | thread=${Thread.currentThread().name}")
        try {
            Openlistlib.SetAdminPassword(pwd)
            Log.e(TAG, "[SAT-DBG][OpenList] setAdminPassword() Openlistlib.SetAdminPassword() OK")
        } catch (e: Throwable) {
            synchronized(lock) {
                lastError = "setAdminPassword error: ${e.message}"
                lastUpdateTs = System.currentTimeMillis()
            }
            Log.e(TAG, "[SAT-DBG][OpenList] setAdminPassword() threw", e)
        }
    }

    /**
     * Cache the data dir locally; will be pushed to openlistlib on the next init().
     * Pure local mutation, no native call here.
     */
    fun setDataDir(path: String) {
        Log.e(TAG, "[SAT-DBG][OpenList] setDataDir() | path=$path | thread=${Thread.currentThread().name}")
        dataDir = path
    }

    /**
     * Cache the port locally. NOTE: this is NOT pushed to openlistlib — the lib
     * reads the port from the on-disk conf.Conf.Scheme.HttpPort at Start() time.
     * This setter only updates the bridge-side snapshot (used by UI + StatusProvider).
     */
    fun setPort(p: Int) {
        Log.e(TAG, "[SAT-DBG][OpenList] setPort() | port=$p | thread=${Thread.currentThread().name}")
        port = p
    }

    /**
     * Atomic snapshot of all runtime state, read under a single synchronized(lock)
     * block. Consumed by OpenListStatusProvider.query() and (transitively) by the
     * host side OpenListStatusBridge via ContentResolver.
     */
    fun snapshot(): Map<String, Any?> = synchronized(lock) {
        mapOf(
            "running" to running,
            "port" to port,
            "pid" to pid,
            "data_size_bytes" to dataSizeBytes,
            "last_error" to (lastError ?: ""),
            "last_update_ts" to lastUpdateTs,
        )
    }

    fun broadcastStatus(port: Int, running: Boolean) {
        Log.e(TAG, "[SAT-DBG][OpenList] broadcastStatus() | port=$port running=$running | ts=${System.currentTimeMillis()}")
        val ctx = appContext ?: run {
            Log.e(TAG, "[SAT-DBG][OpenList] broadcastStatus() skipped (no appContext)")
            return
        }
        try {
            LocalBroadcastManager.getInstance(ctx).sendBroadcast(
                Intent(OpenListService.BROADCAST_STATUS_CHANGED)
                    .putExtra(OpenListService.EXTRA_PORT, port)
                    .putExtra(OpenListService.EXTRA_RUNNING, running)
            )
        } catch (e: Throwable) {
            Log.e(TAG, "[SAT-DBG][OpenList] broadcastStatus() FAILED", e)
        }
    }

    // === openlistlib.Event interface ===
    override fun OnStartError(t: String, err: String) {
        Log.e(TAG, "[SAT-DBG][OpenList] OnStartError() | t=$t err=$err | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        val combined = "$t: $err"
        synchronized(lock) {
            lastError = combined
            lastUpdateTs = System.currentTimeMillis()
            running = false
        }
        broadcastStatus(0, false)
    }

    override fun OnShutdown(t: String) {
        Log.e(TAG, "[SAT-DBG][OpenList] OnShutdown() | t=$t | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        synchronized(lock) {
            running = false
            lastUpdateTs = System.currentTimeMillis()
        }
        broadcastStatus(0, false)
    }

    override fun OnProcessExit(code: Int) {
        Log.e(TAG, "[SAT-DBG][OpenList] OnProcessExit() | code=$code | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        val msg = "process exited with code $code"
        synchronized(lock) {
            lastError = msg
            lastUpdateTs = System.currentTimeMillis()
            running = false
        }
        broadcastStatus(0, false)
    }

    // === openlistlib.LogCallback interface ===
    override fun OnLog(level: Short, time: Long, log: String) {
        // Mirror the logrus entry to local broadcast for in-process subscribers.
        val ctx = appContext
        if (ctx != null) {
            try {
                LocalBroadcastManager.getInstance(ctx).sendBroadcast(
                    Intent(OpenListService.BROADCAST_LOG)
                        .putExtra("level", level.toInt())
                        .putExtra("time", time)
                        .putExtra("log", log)
                )
            } catch (e: Throwable) {
                Log.e(TAG, "[SAT-DBG][OpenList] OnLog() broadcast FAILED", e)
            }
        }
        // Update lastUpdateTs on every log line for liveness.
        val ts = System.currentTimeMillis()
        if (log.startsWith("data_size=")) {
            val raw = log.removePrefix("data_size=").trim()
            val parsed = raw.toLongOrNull()
            if (parsed != null) {
                synchronized(lock) {
                    dataSizeBytes = parsed
                    lastUpdateTs = ts
                }
                Log.e(TAG, "[SAT-DBG][OpenList] OnLog() data_size update | dataSizeBytes=$parsed | ts=$ts")
                return
            }
        }
        synchronized(lock) { lastUpdateTs = ts }
    }
}
