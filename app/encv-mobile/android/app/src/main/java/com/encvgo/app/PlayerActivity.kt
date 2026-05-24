package com.encvgo.app

import android.content.Intent
import android.os.Bundle
import android.util.Log
import androidx.appcompat.app.AppCompatActivity

class PlayerActivity : AppCompatActivity() {
    companion object {
        private const val TAG = "PlayerActivity"
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val targetIntent = if (BuildConfig.USE_LYNX_PLAYER) {
            Intent(this, PlayerActivityLynx::class.java)
        } else {
            Intent(this, PlayerActivityCapacitor::class.java)
        }

        targetIntent.putExtras(intent)
        intent.data?.let { targetIntent.data = it }

        try {
            startActivity(targetIntent)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start player activity", e)
        }
        finish()
    }
}
