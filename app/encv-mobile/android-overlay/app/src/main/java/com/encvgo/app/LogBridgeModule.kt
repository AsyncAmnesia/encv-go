package com.encvgo.app

import com.lynx.jsbridge.LynxMethod
import com.lynx.jsbridge.LynxModule
import com.lynx.react.bridge.Callback

class LogBridgeModule(context: android.content.Context) : LynxModule(context) {

    @LynxMethod
    fun log(level: String, msg: String, callback: Callback) {
        try {
            LogRelay.get().relay("LynxPlayer", level, msg)
        } catch (_e: Exception) {
        }
        callback.onSuccess(null)
    }
}
