package com.encvgo.plugin.mpv

import android.os.Bundle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.appcompat.app.AppCompatActivity
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.ui.Modifier
import com.encvgo.plugin.mpv.theme.EncvMpVPlayerTheme

class MpvPlayerActivity : AppCompatActivity() {

    private lateinit var engine: MpvEngine

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        val filePath = intent.getStringExtra("file_path") ?: ""
        val fileName = intent.getStringExtra("file_name") ?: ""
        val mimeType = intent.getStringExtra("mime_type") ?: ""
        val isExternal = intent.getBooleanExtra("is_external", false)

        engine = createMpvEngine()

        setContent {
            EncvMpVPlayerTheme {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = MaterialTheme.colorScheme.background
                ) {
                    MpvPlayerScreen(
                        filePath = filePath,
                        fileName = fileName,
                        mimeType = mimeType,
                        isExternal = isExternal,
                        engine = engine,
                        onBack = { finish() }
                    )
                }
            }
        }
    }

    override fun onDestroy() {
        super.onDestroy()
        try {
                engine.destroy()
            } catch (_: Exception) {}
    }

    private fun createMpvEngine(): MpvEngine {
        TODO("Phase 3.1: Create and return concrete MpvEngine implementation wrapping MPVLib")
    }
}
