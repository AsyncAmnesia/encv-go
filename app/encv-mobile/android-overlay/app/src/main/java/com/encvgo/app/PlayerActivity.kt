package com.encvgo.app

import android.content.Intent
import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity
import com.getcapacitor.BridgeActivity

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
        startActivity(targetIntent)
        finish()
    }
}
