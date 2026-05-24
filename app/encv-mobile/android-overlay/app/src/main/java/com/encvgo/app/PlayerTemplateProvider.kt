package com.encvgo.app

import android.content.Context
import android.util.Log
import com.lynx.tasm.provider.AbsTemplateProvider
import java.io.ByteArrayOutputStream
import java.io.IOException

class PlayerTemplateProvider(context: Context) : AbsTemplateProvider() {
    companion object {
        private const val TAG = "PlayerTemplateProvider"
    }

    private val mContext = context.applicationContext

    override fun loadTemplate(uri: String, callback: Callback) {
        Log.d(TAG, "loadTemplate: uri=$uri, context=$mContext")
        Thread {
            try {
                val assetPath = uri
                Log.d(TAG, "loadTemplate: attempting assets.open($assetPath)")

                val fileList = mContext.assets.list("") ?: emptyArray()
                val matchingFiles = fileList.filter { it.contains("lynx") || it.contains("player") }
                Log.d(TAG, "loadTemplate: assets root files matching 'lynx'/'player': $matchingFiles")

                if (!fileList.contains(assetPath)) {
                    Log.e(TAG, "loadTemplate: WARNING - '$assetPath' not found in assets root! Total files: ${fileList.size}")
                }

                val inputStream = mContext.assets.open(assetPath)
                inputStream.use { stream ->
                    ByteArrayOutputStream().use { byteArrayOutputStream ->
                        val buffer = ByteArray(4096)
                        var length: Int
                        while (stream.read(buffer).also { length = it } != -1) {
                            byteArrayOutputStream.write(buffer, 0, length)
                        }
                        val data = byteArrayOutputStream.toByteArray()
                        Log.d(TAG, "loadTemplate: loaded ${data.size} bytes for $uri")
                        callback.onSuccess(data)
                    }
                }
            } catch (e: Exception) {
                Log.e(TAG, "loadTemplate: failed to load $uri", e)
                try {
                    callback.onFailed(e.message ?: "Template not found: $uri")
                } catch (cbErr: Exception) {
                    Log.e(TAG, "loadTemplate: callback.onFailed also threw", cbErr)
                }
            }
        }.start()
    }
}
