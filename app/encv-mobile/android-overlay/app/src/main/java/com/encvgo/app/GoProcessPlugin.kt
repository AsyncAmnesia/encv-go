package com.encvgo.app

import com.getcapacitor.JSObject
import com.getcapacitor.Plugin
import com.getcapacitor.PluginCall
import com.getcapacitor.PluginMethod
import com.getcapacitor.annotation.CapacitorPlugin

@CapacitorPlugin(name = "GoProcess")
class GoProcessPlugin : Plugin() {

    @PluginMethod
    fun restart(call: PluginCall) {
        val activity = activity as? MainActivity
        if (activity == null) {
            call.reject("Activity is not MainActivity")
            return
        }
        activity.restartGoDaemon { port ->
            if (port > 0) {
                val result = JSObject()
                result.put("success", true)
                result.put("port", port)
                call.resolve(result)
            } else {
                call.reject("Backend failed to start")
            }
        }
    }

    @PluginMethod
    fun stop(call: PluginCall) {
        val activity = activity as? MainActivity
        if (activity == null) {
            call.reject("Activity is not MainActivity")
            return
        }
        activity.stopGoDaemon()
        val result = JSObject()
        result.put("success", true)
        call.resolve(result)
    }

    @PluginMethod
    fun getStatus(call: PluginCall) {
        val activity = activity as? MainActivity
        if (activity == null) {
            call.reject("Activity is not MainActivity")
            return
        }
        val result = JSObject()
        result.put("running", activity.isBackendRunning())
        result.put("port", activity.getBackendPort())
        call.resolve(result)
    }
}
