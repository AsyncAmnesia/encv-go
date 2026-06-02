package com.encvgo.plugin.openlist

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.os.Build
import android.util.Log
import androidx.compose.runtime.Composable
import androidx.core.content.ContextCompat
import androidx.localbroadcastmanager.content.LocalBroadcastManager
import com.combo.core.api.IPluginEntryClass
import com.combo.core.model.PluginContext
import org.koin.dsl.module

class OpenListPluginEntry : IPluginEntryClass {
    companion object {
        private const val TAG = "OpenList-PluginEntry"
    }

    override val pluginModule = listOf(module {
        single { OpenListBridge }
    })

    override fun onLoad(context: PluginContext) {
        Log.e(TAG, "[SAT-DBG][OpenList] onLoad() begin | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
        Log.e(TAG, "[SAT-DBG][OpenList] Koin DI registered OpenListBridge singleton")

        val app = context.application

        Log.e(TAG, "[SAT-DBG][OpenList] onLoad() registering BroadcastReceiver...")
        val receiver = OpenListBroadcastReceiver()
        LocalBroadcastManager.getInstance(app).registerReceiver(
            receiver,
            IntentFilter().apply {
                addAction(OpenListService.BROADCAST_STATUS_CHANGED)
                addAction(OpenListService.BROADCAST_PORT_CONFLICT)
                addAction(OpenListService.BROADCAST_PROCESS_EXIT)
            }
        )
        Log.e(TAG, "[SAT-DBG][OpenList] onLoad() BroadcastReceiver registered for 3 actions")

        Log.e(TAG, "[SAT-DBG][OpenList] onLoad() starting foreground service...")
        val intent = Intent(app, OpenListService::class.java)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            ContextCompat.startForegroundService(app, intent)
        } else {
            app.startService(intent)
        }
        Log.e(TAG, "[SAT-DBG][OpenList] onLoad() done | service start requested")
    }

    override fun onUnload() {
        Log.e(TAG, "[SAT-DBG][OpenList] onUnload() | thread=${Thread.currentThread().name} | ts=${System.currentTimeMillis()}")
    }

    @Composable
    override fun Content() {
    }

    private class OpenListBroadcastReceiver : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            if (context == null || intent == null) return
            Log.e(TAG, "[SAT-DBG][OpenList] BroadcastReceiver.onReceive() | action=${intent.action} | extras=${intent.extras?.keySet()}")
            // Broadcasts are forwarded to through LocalBroadcastManager;
            // Capacitor plugin bridge registration is handled in a separate step.
        }
    }
}