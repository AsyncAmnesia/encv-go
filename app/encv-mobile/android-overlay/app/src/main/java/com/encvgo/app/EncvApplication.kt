package com.encvgo.app

import android.app.Application
import android.util.Log
import com.facebook.drawee.backends.pipeline.Fresco
import com.facebook.imagepipeline.core.ImagePipelineConfig
import com.facebook.imagepipeline.memory.PoolConfig
import com.facebook.imagepipeline.memory.PoolFactory
import com.lynx.service.devtool.LynxDevToolService
import com.lynx.service.http.LynxHttpService
import com.lynx.service.image.LynxImageService
import com.lynx.service.log.LynxLogService
import com.lynx.tasm.LynxEnv
import com.lynx.tasm.service.LynxServiceCenter

class EncvApplication : Application() {
    companion object {
        private const val TAG = "EncvApplication"
        @Volatile
        private var lynxInitialized = false

        fun ensureLynxInitialized(app: Application) {
            if (lynxInitialized) return
            synchronized(EncvApplication::class.java) {
                if (lynxInitialized) return
                Log.d(TAG, "ensureLynxInitialized: initializing Lynx SDK")
                initLynxService(app)
                initLynxEnv(app)
                lynxInitialized = true
                Log.d(TAG, "ensureLynxInitialized: done")
            }
        }

        private fun initLynxService(app: Application) {
            try {
                val factory = PoolFactory(PoolConfig.newBuilder().build())
                val builder = ImagePipelineConfig.newBuilder(app).setPoolFactory(factory)
                Fresco.initialize(app, builder.build())
                LynxServiceCenter.inst().registerService(LynxImageService.getInstance())
                LynxServiceCenter.inst().registerService(LynxLogService)
                LynxServiceCenter.inst().registerService(LynxHttpService)
                LynxServiceCenter.inst().registerService(LynxDevToolService.INSTANCE)
                Log.d(TAG, "initLynxService: services registered")
            } catch (e: Exception) {
                Log.e(TAG, "initLynxService failed", e)
            }
        }

        private fun initLynxEnv(app: Application) {
            try {
                LynxEnv.inst().init(app, null, null, null)
                LynxEnv.inst().enableLynxDebug(BuildConfig.DEBUG)
                LynxEnv.inst().enableDevtool(BuildConfig.DEBUG)
                LynxEnv.inst().enableLogBox(BuildConfig.DEBUG)
                Log.d(TAG, "initLynxEnv: LynxEnv initialized")
            } catch (e: Exception) {
                Log.e(TAG, "initLynxEnv failed", e)
            }
        }
    }

    override fun onCreate() {
        super.onCreate()
        ensureLynxInitialized(this)
    }
}
