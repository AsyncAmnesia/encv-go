package com.encvgo.app

import android.util.Log
import com.getcapacitor.BridgeActivity
import java.io.File
import java.io.FileOutputStream

class MainActivity : BridgeActivity() {
    companion object {
        private const val TAG = "ENCV-go"
        private const val BINARY_NAME = "encv-go"
    }

    private var goProcess: Process? = null

    override fun onCreate(savedInstanceState: android.os.Bundle?) {
        super.onCreate(savedInstanceState)
        startGoDaemon()
    }

    private fun startGoDaemon() {
        try {
            ensureConfigExists()

            val binary = File(filesDir, BINARY_NAME)
            if (!binary.exists()) {
                copyBinaryFromAssets(binary)
            }
            binary.setExecutable(true)

            val configPath = File(filesDir, "config.user.json").absolutePath
            val pb = ProcessBuilder(binary.absolutePath, "start")
            pb.environment()["ENCV_CONFIG_PATH"] = configPath
            pb.environment()["ENCV_MOBILE"] = "1"
            pb.redirectErrorStream(true)
            goProcess = pb.start()

            Log.i(TAG, "ENCV-go daemon started")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start ENCV-go daemon", e)
        }
    }

    private fun ensureConfigExists() {
        val dest = File(filesDir, "config.user.json")
        if (!dest.exists()) {
            try {
                assets.open("config.mobile.json").use { input ->
                    FileOutputStream(dest).use { output ->
                        input.copyTo(output)
                    }
                }
                Log.i(TAG, "Copied default mobile config to ${dest.absolutePath}")
            } catch (e: Exception) {
                Log.e(TAG, "Failed to copy default config", e)
            }
        }
    }

    private fun copyBinaryFromAssets(dest: File) {
        assets.open(BINARY_NAME).use { input ->
            FileOutputStream(dest).use { output ->
                val buffer = ByteArray(8192)
                var len: Int
                while (input.read(buffer).also { len = it } != -1) {
                    output.write(buffer, 0, len)
                }
            }
        }
        Log.i(TAG, "Copied Go binary to ${dest.absolutePath}")
    }

    override fun onDestroy() {
        super.onDestroy()
        goProcess?.let {
            if (it.isAlive) {
                it.destroy()
                Log.i(TAG, "ENCV-go daemon stopped")
            }
        }
    }
}
