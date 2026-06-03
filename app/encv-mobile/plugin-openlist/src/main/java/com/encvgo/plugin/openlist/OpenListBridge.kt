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
 *     public static void setConfigData(String path);          // SetConfigData
 *     public static void setAdminPassword(String pwd);        // SetAdminPassword
 *     public static void init(Event e, LogCallback cb);       // Init
 *     public static void start();                             // Start
 *     public static void shutdown(long timeoutMs);            // Shutdown
 *     public static boolean isRunning(String protocol);      // IsRunning
 *     public static void forceDbSync();                       // ForceDBSync
 *   }
 *
 * IMPORTANT: gomobile generates **camelCase** Java method names from Go's
 * PascalCase exported functions.  Always use `openlistlib.start()` never
 * `openlistlib.Start()`.  Same for interface callbacks: `onProcessExit`
 * not `OnProcessExit`.
 *
 * Rules enforced here:
 *   1. Openlistlib is abstract + private ctor → we only call its STATIC methods.
 *   2. Method names use gomobile's camelCase convention (Go Start → Java start).
 *   3. init/shutdown/forceDbSync may throw — wrap in try/catch.
 *   4. Port is NOT an openlistlib API: it lives in on-disk conf.Conf.Scheme.HttpPort
 *      and is read by start() from there. We don't try to set it via the lib.
 *   5. setConfigData(dataDir) tells openlistlib where to look for the config dir.
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

    // C5: assets extraction state
    private val ASSETS_PREFS = "openlist_assets"
    private val ASSETS_KEY_VERSION = "extracted_version"

    /**
     * One-time init: register this object (which implements Event + LogCallback)
     * with the openlistlib static Init() entry. Idempotent. Safe to call from
     * multiple call sites (Provider.onCreate, PluginEntry.onLoad, Service.onStart).
     */
    fun init(context: Context) {
        Log.e(TAG, "[SAT-DBG][OpenList] init() entry | dataDir=$dataDir port=$port | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        appContext = context.applicationContext
        synchronized(lock) {
            // Step 1: Extract frontend dist from APK assets/ to filesDir (C5)
            // This makes the production runtime use the on-disk dist instead of
            // the //go:embed copy inside libgojni.so. Also writes config.json
            // with dist_dir pointing to the extracted path, so OpenList reads
            // conf.Conf.DistDir → os.DirFS, not the embed.
            try {
                ensureAssetsExtracted(appContext!!)
                Log.e(TAG, "[SAT-DBG][OpenList] init() ensureAssetsExtracted OK")
            } catch (e: Throwable) {
                lastError = "ensureAssetsExtracted failed: ${e.message}"
                lastUpdateTs = System.currentTimeMillis()
                Log.e(TAG, "[SAT-DBG][OpenList] init() ensureAssetsExtracted FAILED", e)
            }

            // Step 2: Apply data dir to openlistlib (tells it where to look for config.json)
            try {
                if (dataDir.isNotEmpty()) {
                    Openlistlib.setConfigData(dataDir)
                    Log.e(TAG, "[SAT-DBG][OpenList] init() Openlistlib.SetConfigData($dataDir) OK")
                }
            } catch (e: Throwable) {
                lastError = "SetConfigData failed: ${e.message}"
                lastUpdateTs = System.currentTimeMillis()
                Log.e(TAG, "[SAT-DBG][OpenList] init() Openlistlib.SetConfigData FAILED", e)
            }
            if (!initialized) {
                try {
                    Openlistlib.init(this, this)
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
     * Extract OpenList frontend dist from APK assets/dist/ to filesDir/openlist/dist/.
     * On first launch (or when the bundled VERSION changes), we:
     *   1. Recursively copy assets/dist/* to filesDir/openlist/dist/
     *   2. Write/update config.json in dataDir with dist_dir = filesDir/openlist/dist
     *      so OpenList reads from disk at runtime (os.DirFS), not from embed.FS.
     *
     * On subsequent launches (same VERSION), this is a fast no-op (just rewrites config.json).
     *
     * Why: spec §2.2 C5 — lets us ship frontend updates as small APK patches without
     * rebuilding the ~150MB gomobile AAR.
     */
    private fun ensureAssetsExtracted(context: Context) {
        val filesDir = context.filesDir
        val targetDist = java.io.File(filesDir, "openlist/dist")
        val targetDataDir = java.io.File(filesDir, "openlist/data")
        targetDataDir.mkdirs()

        val prefs = context.getSharedPreferences(ASSETS_PREFS, Context.MODE_PRIVATE)
        val bundledVersion = try {
            context.assets.open("dist/VERSION").bufferedReader().use { it.readText().trim() }
        } catch (e: Throwable) {
            Log.e(TAG, "[OpenList] no dist/VERSION in APK assets (fallback to embed); err=${e.message}")
            ""
        }
        val extractedVersion = prefs.getString(ASSETS_KEY_VERSION, "") ?: ""

        val needsExtract = !targetDist.exists() ||
            targetDist.list()?.isEmpty() == true ||
            (bundledVersion.isNotEmpty() && bundledVersion != extractedVersion)

        if (needsExtract && bundledVersion.isNotEmpty()) {
            Log.e(TAG, "[OpenList] extracting dist from APK assets | bundled=$bundledVersion extracted=$extractedVersion | target=$targetDist")
            copyAssetDir(context, java.io.File("dist"), targetDist)
            prefs.edit().putString(ASSETS_KEY_VERSION, bundledVersion).apply()
        } else {
            Log.e(TAG, "[OpenList] dist extraction skipped | bundled=${bundledVersion.ifEmpty { "<none>" }} extracted=$extractedVersion")
        }

        // Write config.json with dist_dir pointing to the extracted dist
        writeRuntimeConfig(targetDataDir, targetDist, prefs)
    }

    /**
     * Recursively copy an asset directory from APK to local file system.
     * Handles nested subdirectories (assets/dist/assets/index-*.js etc).
     */
    private fun copyAssetDir(context: Context, source: java.io.File, target: java.io.File) {
        target.mkdirs()
        val assetPath = source.path  // "dist" or "dist/assets" etc
        val entries = try { context.assets.list(assetPath) ?: emptyArray() } catch (e: Throwable) { emptyArray() }
        for (name in entries) {
            val childAsset = "$assetPath/$name"
            val childTarget = java.io.File(target, name)
            // If list(name) returns non-empty, it's a directory; else it's a file
            val children = try { context.assets.list(childAsset) ?: emptyArray() } catch (e: Throwable) { emptyArray() }
            if (children.isNotEmpty()) {
                copyAssetDir(context, java.io.File(childAsset), childTarget)
            } else {
                context.assets.open(childAsset).use { input ->
                    childTarget.outputStream().use { output -> input.copyTo(output) }
                }
            }
        }
    }

    /**
     * Write config.json for the OpenList runtime. Preserves user-customized fields
     * if config.json already exists, only injects/updates the dist_dir field.
     */
    private fun writeRuntimeConfig(dataDir: java.io.File, distDir: java.io.File, prefs: android.content.SharedPreferences) {
        val configFile = java.io.File(dataDir, "config.json")
        val existing = if (configFile.exists()) {
            try { org.json.JSONObject(configFile.readText()) } catch (e: Throwable) { org.json.JSONObject() }
        } else {
            org.json.JSONObject()
        }
        existing.put("dist_dir", distDir.absolutePath)
        // Don't override scheme: user might have custom port/HTTPS settings.
        // If absent, the OpenList defaults take over.
        configFile.writeText(existing.toString(2))
        Log.e(TAG, "[OpenList] wrote config.json | dist_dir=${distDir.absolutePath} | dataDir=$dataDir")
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
                Openlistlib.start()
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
                Openlistlib.shutdown(timeoutMs)
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
        Openlistlib.isRunning(protocol)
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
                Openlistlib.forceDbSync()
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
            Openlistlib.setAdminPassword(pwd)
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

    // === openlistlib.Event interface (gomobile generates camelCase) ===
    override fun onStartError(t: String, err: String) {
        Log.e(TAG, "[SAT-DBG][OpenList] OnStartError() | t=$t err=$err | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        val combined = "$t: $err"
        synchronized(lock) {
            lastError = combined
            lastUpdateTs = System.currentTimeMillis()
            running = false
        }
        broadcastStatus(0, false)
    }

    override fun onShutdown(t: String) {
        Log.e(TAG, "[SAT-DBG][OpenList] OnShutdown() | t=$t | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        synchronized(lock) {
            running = false
            lastUpdateTs = System.currentTimeMillis()
        }
        broadcastStatus(0, false)
    }

    override fun onProcessExit(code: Long) {
        Log.e(TAG, "[SAT-DBG][OpenList] OnProcessExit() | code=$code | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        val msg = "process exited with code $code"
        synchronized(lock) {
            lastError = msg
            lastUpdateTs = System.currentTimeMillis()
            running = false
        }
        broadcastStatus(0, false)
    }

    // === openlistlib.LogCallback interface (gomobile generates camelCase) ===
    override fun onLog(level: Short, time: Long, log: String) {
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
