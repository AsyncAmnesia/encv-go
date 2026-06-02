package com.encvgo.combolite

import android.content.ContentResolver
import android.content.ContentValues
import android.content.Context
import android.database.Cursor
import android.net.Uri
import android.util.Log
import com.encvgo.plugin.openlist.OpenListStatusProvider

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
 */
object OpenListStatusBridge {

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
            resolver.query(OpenListStatusProvider.STATUS_URI, null, null, null, null)
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
            val running = cursor.getInt(cursor.getColumnIndexOrThrow(OpenListStatusProvider.COL_RUNNING)) == 1
            val port = cursor.getInt(cursor.getColumnIndexOrThrow(OpenListStatusProvider.COL_PORT))
            val pid = cursor.getInt(cursor.getColumnIndexOrThrow(OpenListStatusProvider.COL_PID))
            val dataSize = cursor.getLong(cursor.getColumnIndexOrThrow(OpenListStatusProvider.COL_DATA_SIZE))
            val lastErr = cursor.getString(cursor.getColumnIndexOrThrow(OpenListStatusProvider.COL_LAST_ERROR)) ?: ""
            val lastTs = cursor.getLong(cursor.getColumnIndexOrThrow(OpenListStatusProvider.COL_LAST_UPDATE))
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
            val result = context.contentResolver.insert(OpenListStatusProvider.CONTROL_URI, values)
            Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] control() action=$action → result=$result")
            result != null
        } catch (e: Throwable) {
            Log.e(TAG, "[SAT-DBG][OpenList][HostBridge] control() FAILED", e)
            false
        }
    }
}
