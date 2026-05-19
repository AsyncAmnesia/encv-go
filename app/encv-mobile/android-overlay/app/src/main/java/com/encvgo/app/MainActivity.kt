package com.encvgo.app

import android.os.Build
import android.util.Log
import com.getcapacitor.BridgeActivity
import java.io.BufferedReader
import java.io.File
import java.io.FileOutputStream
import java.io.InputStreamReader

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

            val binary = findExecutableBinary()
                ?: run {
                    Log.e(TAG, "Failed to extract Go binary to any executable location")
                    return
                }

            Log.i(TAG, "Using binary at: ${binary.absolutePath}")
            Log.i(TAG, "Binary exists: ${binary.exists()}, size: ${binary.length()}, canExecute: ${binary.canExecute()}")

            val configPath = File(filesDir, "config.user.json").absolutePath
            val pb = ProcessBuilder(binary.absolutePath, "start")
            pb.environment()["ENCV_CONFIG_PATH"] = configPath
            pb.environment()["ENCV_MOBILE"] = "1"
            pb.environment()["HOME"] = filesDir.absolutePath
            pb.redirectErrorStream(true)
            goProcess = pb.start()

            Thread {
                try {
                    val reader = BufferedReader(InputStreamReader(goProcess?.inputStream))
                    var line: String?
                    while (reader.readLine().also { line = it } != null) {
                        Log.i(TAG, "[go] $line")
                    }
                } catch (e: Exception) {
                    Log.w(TAG, "Error reading process output", e)
                }
            }.start()

            Log.i(TAG, "ENCV-go daemon started (pid: unknown, async)")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start ENCV-go daemon", e)
            e.message?.split("\n")?.forEach { Log.e(TAG, it) }
        }
    }

    private fun findExecutableBinary(): File? {
        val candidateDirs = listOf(
            filesDir to "filesDir",
            cacheDir to "cacheDir",
            getExternalFilesDir(null) to "externalFilesDir",
        )

        for ((dir, name) in candidateDirs) {
            if (dir == null) continue
            Log.d(TAG, "Trying location: $name -> ${dir.absolutePath}")
            val binary = File(dir, BINARY_NAME)

            if (!binary.exists()) {
                copyBinaryFromAssets(binary)
            }

            binary.setReadable(true)
            binary.setExecutable(true)
            binary.setWritable(true)

            if (binary.canExecute()) {
                Log.i(TAG, "Binary is executable at: $name (${binary.absolutePath})")
                return binary
            }

            Log.w(TAG, "Binary NOT executable at: $name (Android ${Build.VERSION.RELEASE} API ${Build.VERSION.SDK_INT})")
        }

        return null
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
        } else {
            Log.i(TAG, "Config already exists at ${dest.absolutePath}")
        }
    }

    private fun copyBinaryFromAssets(dest: File) {
        dest.parentFile?.mkdirs()
        assets.open(BINARY_NAME).use { input ->
            FileOutputStream(dest).use { output ->
                val buffer = ByteArray(8192)
                var len: Int
                while (input.read(buffer).also { len = it } != -1) {
                    output.write(buffer, 0, len)
                }
            }
        }
        Log.i(TAG, "Copied Go binary to ${dest.absolutePath} (${dest.length()} bytes)")
    }

    override fun onDestroy() {
        super.onDestroy()
        goProcess?.let {
            if (it.isAlive) {
                it.destroyForcibly()
                Log.i(TAG, "ENCV-go daemon force-stopped")
            }
        }
    }
}
