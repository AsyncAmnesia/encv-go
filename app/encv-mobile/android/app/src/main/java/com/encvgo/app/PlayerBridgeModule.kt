package com.encvgo.app

import android.content.Context
import com.lynx.jsbridge.LynxMethod
import com.lynx.jsbridge.LynxModule
import com.lynx.react.bridge.Callback

class PlayerBridgeModule(context: Context) : LynxModule(context) {

    @LynxMethod
    fun playFile(filePath: String, fileName: String, mimeType: String, callback: Callback) {
        try {
            PlayerEntry.play(context, filePath, fileName, mimeType)
            callback.invoke(true)
        } catch (e: Exception) {
            callback.invoke(e.message ?: "Failed to start player")
        }
    }

    @LynxMethod
    fun playFileExternal(filePath: String, fileName: String, mimeType: String, callback: Callback) {
        try {
            PlayerEntry.play(context, filePath, fileName, mimeType, isExternal = true)
            callback.invoke(true)
        } catch (e: Exception) {
            callback.invoke(e.message ?: "Failed to start player")
        }
    }

    @LynxMethod
    fun isMpvAvailable(callback: Callback) {
        try {
            val available = PlayerEntry.isMpvAvailable(context)
            callback.invoke(available)
        } catch (e: Exception) {
            callback.invoke(false)
        }
    }
}
