package com.encvgo.combolite

import android.content.ContentResolver
import android.content.ContentValues
import android.content.Context
import android.database.Cursor
import android.net.Uri
import android.util.Log

/**
 * Host-side bridge to the OpenList extension's ContentProvider.
 *
 * The OpenList APK is an independent process (aar2apk sets applicationId
 * → separate process). To read its runtime state, we use ContentResolver
 * to query the plugin's exported provider.
 *
 * Authority: com.encvgo.plugin.openlist.provider
 *   - /status → query returns MatrixCursor snapshot
 *   - /control → insert dispatches action
 *
 * NOTE: All URI/column constants are defined here (host side) and MUST match
 * OpenListStatusProvider (plugin side).  This avoids a compile-time dependency
 * on :plugin-openlist — the host builds and ships independently of any plugin.
 */
object OpenListStatusBridge {

    // ── ContentProvider contract (mirrors OpenListStatusProvider) ──
    private const val AUTHORITY = "com.encvgo.plugin.openlist.provider"
    private const val PATH_STATUS = "status"
    private const val PATH_CONTROL = "control"

    /** content://com.encvgo.plugin.openlist.provider/status */
    internal val STATUS_URI: Uri = Uri.parse("content://$AUTHORITY/$PATH_STATUS")
    /** content://com.encvgo.plugin.openlist.provider/control */
    internal val CONTROL_URI: Uri = Uri.parse("content://$AUTHORITY/$PATH_CONTROL")

    // MatrixCursor column names — must match STATUS_COLUMNS in OpenListStatusProvider.
    private const val COL_RUNNING = "running"
    private const val COL_PORT = "port"
    private const val COL_PID = "pid"
    private const val COL_DATA_SIZE = "data_size_bytes"
    private const val COL_LAST_ERROR = "last_error"
    private const val COL_LAST_UPDATE = "last_update_ts"

    // ── End of contract constants ──

    private const val TAG = "OpenList-HostBridge"

    data class OpenListRuntime(
        val isInstalled: Boolean,
        val running: Boolean,
        val port: Int,
        val pid: Int,
        val dataSizeBytes: Long,
        val lastError: String,
        val lastUpdateTs: Long,
    ) {
        companion object {
            val NotInstalled = OpenListRuntime(
                isInstalled = false,
                running = false,
                port = 0,
                pid = 0,
                dataSizeBytes = 0L,
                lastError = "openlist extension not installed",
                lastUpdateTs = 0L,
            )
        }
    }

    fun read(context: Context): OpenListRuntime {
        Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] read() begin | ts=${System.currentTimeMillis()}")
        val resolver: ContentResolver = context.contentResolver
        val cursor: Cursor? = try {
            resolver.query(STATUS_URI, null, null, null, null)
        } catch (e: IllegalArgumentException) {
            Log.w(TAG, "[SAT-DBG][OpenList][HostBridge] provider authority not found, OpenList not installed", e)
            null
        } catch (e: SecurityException) {
            Log.w(TAG, "[SAT-DBG][OpenList][HostBridge] provider permission denied", e)
            null
        } catch (e: Throwable) {
            Log.w(TAG, "[SAT-DBG][OpenList][HostBridge] query() unexpected error", e)
            null
        }
        if (cursor == null) {
            Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] read() → NotInstalled (cursor=null)")
            return OpenListRuntime.NotInstalled
        }
        return try {
            if (!cursor.moveToFirst()) {
                Log.w(TAG, "[SAT-DBG][OpenList][HostBridge] cursor empty → NotInstalled")
                return OpenListRuntime.NotInstalled
            }
            val running = cursor.getInt(cursor.getColumnIndexOrThrow(COL_RUNNING)) == 1
            val port = cursor.getInt(cursor.getColumnIndexOrThrow(COL_PORT))
            val pid = cursor.getInt(cursor.getColumnIndexOrThrow(COL_PID))
            val dataSize = cursor.getLong(cursor.getColumnIndexOrThrow(COL_DATA_SIZE))
            val lastErr = cursor.getString(cursor.getColumnIndexOrThrow(COL_LAST_ERROR)) ?: ""
            val lastTs = cursor.getLong(cursor.getColumnIndexOrThrow(COL_LAST_UPDATE))
            Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] read() OK | running=$running port=$port pid=$pid dataSize=$dataSize lastErr='$lastErr'")
            OpenListRuntime(
                isInstalled = true,
                running = running,
                port = port,
                pid = pid,
                dataSizeBytes = dataSize,
                lastError = lastErr,
                lastUpdateTs = lastTs,
            )
        } catch (e: Throwable) {
            Log.w(TAG, "[SAT-DBG][OpenList][HostBridge] cursor read error", e)
            OpenListRuntime.NotInstalled
        } finally {
            cursor.close()
        }
    }

    fun control(context: Context, action: String, args: Map<String, Any> = emptyMap()): Boolean {
        Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] control() action=$action args=$args")
        val values = ContentValues().apply {
            put("action", action)
            for ((k, v) in args) {
                when (v) {
                    is String -> put(k, v)
                    is Int -> put(k, v)
                    is Long -> put(k, v)
                    is Boolean -> put(k, v)
                    else -> put(k, v.toString())
                }
            }
        }
        return try {
            val result = context.contentResolver.insert(CONTROL_URI, values)
            Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] control() action=$action → result=$result")
            result != null
        } catch (e: Throwable) {
            Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] control() FAILED", e)
            false
        }
    }
}
