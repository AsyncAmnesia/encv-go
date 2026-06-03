package com.encvgo.plugin.openlist

import android.content.Context
import android.content.SharedPreferences
import android.util.Log
import java.io.File

data class OpenListConfig(
    val port: Int = DEFAULT_PORT,
    val dataDir: String = "",
    val adminPassword: String = "",
) {
    companion object {
        private const val TAG = "OpenList-Config"
        const val DEFAULT_PORT = 5244
        private const val PREFS_NAME = "openlist_config"
        private const val KEY_PORT = "port"
        private const val KEY_DATA_DIR = "data_dir"
        private const val KEY_ADMIN_PASSWORD = "admin_password"

        fun defaultDataDir(context: Context): String =
            File(context.filesDir, "openlist/data").absolutePath

        fun load(context: Context): OpenListConfig {
            val prefs: SharedPreferences = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            val port = prefs.getInt(KEY_PORT, DEFAULT_PORT)
            val dataDir = prefs.getString(KEY_DATA_DIR, null) ?: defaultDataDir(context)
            val adminPassword = prefs.getString(KEY_ADMIN_PASSWORD, "") ?: ""
            val cfg = OpenListConfig(
                port = port,
                dataDir = dataDir,
                adminPassword = adminPassword
            )
            Log.e(TAG, "[SAT-DBG][OpenList] load() | port=$port dataDir=$dataDir adminPasswordLen=${adminPassword.length}")
            return cfg
        }

        fun save(context: Context, port: Int, dataDir: String, adminPassword: String) {
            Log.e(TAG, "[SAT-DBG][OpenList] save() | port=$port dataDir=$dataDir adminPasswordLen=${adminPassword.length}")
            val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            prefs.edit()
                .putInt(KEY_PORT, port)
                .putString(KEY_DATA_DIR, dataDir)
                .putString(KEY_ADMIN_PASSWORD, adminPassword)
                .apply()
        }
    }

    /**
     * Push the local config to OpenListBridge so it can be applied during init().
     *
     * IMPORTANT: do NOT call bridge.setAdminPassword() here — that delegates to
     * Openlistlib.SetAdminPassword which queries the user DB; the DB isn't
     * initialized until Openlistlib.Init() (and even then, not until Start()
     * triggers bootstrap). Admin password reset must be deferred to AFTER
     * the server has fully started (see OpenListService.startupSequence step6).
     *
     * The actual port that openlistlib binds to is read from the on-disk
     * conf.Conf.Scheme.HttpPort at Start() time; this Kotlin-side port is
     * only the snapshot the UI/StatusProvider report.
     */
    fun applyToBridge(bridge: OpenListBridge) {
        Log.e(TAG, "[SAT-DBG][OpenList] applyToBridge() | port=$port dataDir=$dataDir (adminPassword deferred to post-Start)")
        bridge.setPort(port)
        bridge.setDataDir(dataDir)
        // Intentionally NOT calling bridge.setAdminPassword(adminPassword) here.
    }
}
