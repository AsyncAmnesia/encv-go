package com.encvgo.app

import android.content.Intent
import android.os.Bundle
import android.util.Log
import com.combo.core.component.activity.BaseHostActivity

class EncvHostActivity : BaseHostActivity() {
    private const val TAG = "EncvHostActivity"
    private var proxyStarted = false

    override fun onCreate(savedInstanceState: Bundle?) {
        Log.i(TAG, "onCreate: intent=$intent")
        val pluginId = intent.getStringExtra("plugin_id") ?: intent.getStringExtra(PLAYER_ENTRY.EXTRA_MODE)
        val targetActivity = intent.getStringExtra("target_activity")
        Log.i(TAG, "onCreate: pluginId=$pluginId targetActivity=$targetActivity")

        try {
            super.onCreate(savedInstanceState)
            proxyStarted = true
            Log.i(TAG, "onCreate: super.onCreate completed, proxy should have started")
        } catch (e: Exception) {
            Log.e(TAG, "onCreate: super.onCreate failed: ${e.message}", e)
            finishWithResult(pluginId, false, "代理启动失败", e.message)
        }
    }

    override fun onResume() {
        super.onResume()
        if (!proxyStarted) {
            Log.w(TAG, "onResume: proxy not started, finishing with error")
            val pluginId = intent.getStringExtra("plugin_id") ?: ""
            finishWithResult(pluginId, false, "播放器未启动", "proxyStarted=false after onResume")
        }
    }

    override fun onDestroy() {
        super.onDestroy()
        Log.i(TAG, "onDestroy: proxyStarted=$proxyStarted")
    }

    private fun finishWithResult(
        pluginId: String?,
        success: Boolean,
        error: String,
        detail: String?
    ) {
        Log.e(TAG, "finishWithResult: success=$success error=$error detail=$detail")
        setResult(RESULT_OK, Intent().apply {
            putExtra("player_success", success)
            putExtra("player_error", error)
            putExtra("player_error_detail", detail ?: "")
            putExtra("player_plugin_id", pluginId ?: "")
        })
        finish()
    }

    companion object {
        const val RESULT_EXTRA_SUCCESS = "player_success"
        const val RESULT_EXTRA_ERROR = "player_error"
        const val RESULT_EXTRA_ERROR_DETAIL = "player_error_detail"
        const val RESULT_EXTRA_PLUGIN_ID = "player_plugin_id"
    }
}