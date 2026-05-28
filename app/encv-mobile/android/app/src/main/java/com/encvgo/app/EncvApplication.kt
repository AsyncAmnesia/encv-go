package com.encvgo.app

import android.app.Application
import com.tencent.bugly.crashreport.CrashReport
import com.combo.core.runtime.PluginManager
import com.combo.core.runtime.ValidationStrategy
import com.combo.core.runtime.app.BaseHostApplication
import com.combo.core.security.crash.PluginCrashHandler
import android.util.Log

class EncvApplication : BaseHostApplication() {
    companion object {
        private const val TAG = "ENCV-App"
    }

    override fun onCreate() {
        super.onCreate()
        initBugly()
    }

    override fun onFrameworkSetup(): suspend () -> Unit {
        return {
            try {
                PluginManager.setValidationStrategy(ValidationStrategy.Insecure)
                Log.i(TAG, "onFrameworkSetup: setValidationStrategy(Insecure) OK")
            } catch (e: Error) {
                Log.w(TAG, "onFrameworkSetup: setValidationStrategy failed: ${e.javaClass.simpleName}: ${e.message}")
            } catch (e: Exception) {
                Log.w(TAG, "onFrameworkSetup: setValidationStrategy FAILED", e)
            }
            try {
                PluginCrashHandler.setGlobalClashCallback(null)
            } catch (e: Error) {
                Log.w(TAG, "onFrameworkSetup: setGlobalClashCallback Error: ${e.javaClass.simpleName}")
            } catch (e: Exception) {
                Log.w(TAG, "onFrameworkSetup: setGlobalClashCallback FAILED", e)
            }
            try {
                PluginManager.proxyManager.setHostActivity(com.encvgo.app.EncvHostActivity::class.java)
                Log.i(TAG, "onFrameworkSetup: setHostActivity(EncvHostActivity) OK")
            } catch (e: Exception) {
                Log.e(TAG, "onFrameworkSetup: setHostActivity FAILED", e)
            }
            Log.i(TAG, "onFrameworkSetup: complete, PluginManager.isInitialized=${PluginManager.isInitialized}")
        }
    }

    private fun initBugly() {
        try {
            val appId = BuildConfig.BUGLY_APP_ID
            if (appId.isEmpty()) {
                Log.w("ENCV-Bugly", "BUGLY_APP_ID not configured, skipping")
                return
            }
            CrashReport.initCrashReport(applicationContext, appId, false)
            Log.i("ENCV-Bugly", "Bugly initialized: appId=$appId")
        } catch (e: Exception) {
            Log.e("ENCV-Bugly", "Failed to initialize Bugly", e)
        }
    }
}
