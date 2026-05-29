package com.encvgo.app

import android.app.Application
import com.tencent.bugly.crashreport.CrashReport
import com.combo.core.runtime.app.BaseHostApplication
import com.encvgo.combolite.EncvComboLiteHost
import android.util.Log

class EncvApplication : BaseHostApplication() {
    companion object {
        private const val TAG = "ENCV-App"
    }

    override fun onCreate() {
        super.onCreate()
        initBugly()
    }

    override fun onFrameworkSetup(): suspend () -> Unit = {
        EncvComboLiteHost.setupFramework(EncvHostActivity::class.java)
        Log.i(TAG, "onFrameworkSetup: complete via EncvComboLiteHost, PluginManager.isInitialized=${com.combo.core.runtime.PluginManager.isInitialized}")
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
