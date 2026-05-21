package com.encvgo.app

import android.content.Intent
import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity

class PlayerActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val targetIntent = if (BuildConfig.USE_LYNX_PLAYER) {
            Intent(this, PlayerActivityLynx::class.java)
        } else {
            Intent(this, PlayerActivityCapacitor::class.java)
        }

        targetIntent.putExtras(intent)
        intent.data?.let { targetIntent.data = it }
        startActivity(targetIntent)
        finish()
    }
}
