package com.encvgo.app

import android.app.Application
import com.tencent.bugly.crashreport.CrashReport
import com.combo.core.runtime.PluginManager
import com.combo.core.runtime.ValidationStrategy
import com.combo.core.runtime.app.BaseHostApplication
import com.combo.core.security.crash.PluginCrashHandler
import android.util.Log

class EncvApplication : BaseHostApplication() {
    override fun onCreate() {
        super.onCreate()
        initBugly()
    }

    override fun onFrameworkSetup(): suspend () -> Unit {
        return {
            PluginManager.proxyManager.apply {
                setHostActivity(com.encvgo.app.MainActivity::class.java)
            }
            PluginManager.setValidationStrategy(ValidationStrategy.Insecure)
            PluginCrashHandler.setGlobalClashCallback(null)
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
