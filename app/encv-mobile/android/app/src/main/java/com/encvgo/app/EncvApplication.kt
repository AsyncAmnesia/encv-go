package com.encvgo.app

import android.app.Application
import com.tencent.bugly.crashreport.CrashReport
import android.util.Log

class EncvApplication : Application() {
    override fun onCreate() {
        super.onCreate()
        initBugly()
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
