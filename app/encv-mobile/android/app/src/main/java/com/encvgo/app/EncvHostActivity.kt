package com.encvgo.app

import android.content.Intent
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.util.Log
import com.combo.core.component.activity.BaseHostActivity

class EncvHostActivity : BaseHostActivity() {
    private var proxyStarted = false
    private var resultSet = false
    private var createTime = 0L

    override fun onCreate(savedInstanceState: Bundle?) {
        createTime = System.currentTimeMillis()
        Log.i(TAG, "onCreate: intent=$intent createTime=$createTime")
        val pluginId = intent.getStringExtra("plugin_id") ?: intent.getStringExtra(PlayerEntry.EXTRA_MODE)
        val targetActivity = intent.getStringExtra("target_activity")
        Log.i(TAG, "onCreate: pluginId=$pluginId targetActivity=$targetActivity")

        try {
            super.onCreate(savedInstanceState)
            proxyStarted = true
            val elapsed = System.currentTimeMillis() - createTime
            Log.i(TAG, "onCreate: super.onCreate completed, proxy should have started (elapsed=${elapsed}ms)")
        } catch (e: Exception) {
            Log.e(TAG, "onCreate: super.onCreate failed: ${e.message}", e)
            finishWithResult(pluginId, false, "代理启动失败", e.message)
        }
    }

    override fun onPostCreate(savedInstanceState: Bundle?) {
        super.onPostCreate(savedInstanceState)
        if (!proxyStarted) {
            Log.w(TAG, "onPostCreate: proxy still not started after ${System.currentTimeMillis() - createTime}ms, scheduling timeout check")
            Handler(Looper.getMainLooper()).postDelayed({
                if (!proxyStarted && !isFinishing && !resultSet) {
                    Log.e(TAG, "Timeout: proxy never started after onPostCreate+5s, finishing with error")
                    finishWithResult(intent?.getStringExtra("plugin_id"), false,
                        "播放器启动超时", "proxyStarted=false after ${System.currentTimeMillis() - createTime}ms")
                }
            }, PROXY_TIMEOUT_MS)
        }
    }

    override fun onResume() {
        super.onResume()
        val elapsed = System.currentTimeMillis() - createTime
        Log.i(TAG, "onResume: proxyStarted=$proxyStarted elapsed=${elapsed}ms resultSet=$resultSet")

        if (!proxyStarted) {
            Log.w(TAG, "onResume: proxy not started, finishing with error")
            val pluginId = intent?.getStringExtra("plugin_id")
            finishWithResult(pluginId, false, "播放器未启动", "proxyStarted=false after ${elapsed}ms")
            return
        }

        if (elapsed > PROXY_TIMEOUT_MS) {
            Log.w(TAG, "onResume: proxy started but took ${elapsed}ms (>${PROXY_TIMEOUT_MS}ms), may be stuck")
        }
    }

    override fun onDestroy() {
        if (!resultSet) {
            Log.w(TAG, "onDestroy: result never set (user pressed back or activity killed), setting success=true as default")
            setResult(RESULT_OK, Intent().apply {
                putExtra("player_success", true)
                putExtra("player_error", "")
                putExtra("player_error_detail", "")
                putExtra("player_plugin_id", intent?.getStringExtra("plugin_id") ?: "")
            })
            resultSet = true
        }
        Log.i(TAG, "onDestroy: proxyStarted=$proxyStarted resultSet=$resultSet elapsed=${System.currentTimeMillis() - createTime}ms")
        super.onDestroy()
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
        resultSet = true
        finish()
    }

    companion object {
        const val TAG = "EncvHostActivity"
        const val PROXY_TIMEOUT_MS = 5000L
    }
}
