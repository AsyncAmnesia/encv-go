package com.encvgo.plugin.openlist

import android.content.Context
import android.content.SharedPreferences
import java.io.File

data class OpenListConfig(
    val port: Int = DEFAULT_PORT,
    val dataDir: String = "",
    val adminPassword: String = "",
) {
    companion object {
        const val DEFAULT_PORT = 5244
        private const val PREFS_NAME = "openlist_config"
        private const val KEY_PORT = "port"
        private const val KEY_DATA_DIR = "data_dir"
        private const val KEY_ADMIN_PASSWORD = "admin_password"

        fun defaultDataDir(context: Context): String =
            File(context.filesDir, "openlist/data").absolutePath

        fun load(context: Context): OpenListConfig {
            val prefs: SharedPreferences = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            return OpenListConfig(
                port = prefs.getInt(KEY_PORT, DEFAULT_PORT),
                dataDir = prefs.getString(KEY_DATA_DIR, null) ?: defaultDataDir(context),
                adminPassword = prefs.getString(KEY_ADMIN_PASSWORD, "") ?: ""
            )
        }

        fun save(context: Context, port: Int, dataDir: String, adminPassword: String) {
            val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            prefs.edit()
                .putInt(KEY_PORT, port)
                .putString(KEY_DATA_DIR, dataDir)
                .putString(KEY_ADMIN_PASSWORD, adminPassword)
                .apply()
        }
    }

    fun applyToBridge(bridge: OpenListBridge) {
        bridge.setPort(port)
        bridge.setDataDir(dataDir)
        if (adminPassword.isNotEmpty()) {
            bridge.setAdminPassword(adminPassword)
        }
    }
}
