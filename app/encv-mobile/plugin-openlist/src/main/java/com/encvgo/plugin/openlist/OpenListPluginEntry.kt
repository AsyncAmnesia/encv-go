package com.encvgo.plugin.openlist

import android.content.Context
import android.util.Log
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Stop
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalLifecycleOwner
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.repeatOnLifecycle
import com.combo.core.api.IPluginEntryClass
import com.combo.core.model.PluginContext
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import org.koin.core.module.dsl.singleOf
import org.koin.dsl.module

class OpenListPluginEntry : IPluginEntryClass {
    companion object {
        private const val TAG = "OpenList-PluginEntry"
        private const val POLL_INTERVAL_MS = 2000L
    }

    override val pluginModule = listOf(module {
        singleOf(OpenListBridge)
    })

    override fun onLoad(context: PluginContext) {
        Log.e(TAG, "[OpenList] onLoad() | init deferred to OpenListService.startupSequence()")
    }

    override fun onUnload() {
        Log.e(TAG, "[OpenList] onUnload()")
    }

    @OptIn(ExperimentalMaterial3Api::class)
    @Composable
    override fun Content() {
        MaterialTheme {
            Scaffold(
                topBar = {
                    TopAppBar(
                        title = { Text("OpenList") },
                        colors = TopAppBarDefaults.topAppBarColors(
                            containerColor = MaterialTheme.colorScheme.primaryContainer,
                            titleContentColor = MaterialTheme.colorScheme.onPrimaryContainer,
                        ),
                    )
                },
            ) { padding ->
                LazyColumn(
                    modifier = Modifier
                        .fillMaxSize()
                        .padding(padding)
                        .padding(horizontal = 16.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    item { StatusCard() }
                    item { ControlCard() }
                    item { ConfigCard() }
                    item { Spacer(modifier = Modifier.height(32.dp)) }
                }
            }
        }
    }
}

// ── Status Card ──────────────────────────────────────────────

@Composable
private fun StatusCard() {
    var snapshot by remember { mutableStateOf(OpenListBridge.snapshot()) }
    var isInstalled by remember { mutableStateOf(true) }

    // Poll bridge snapshot every 2s
    LaunchedEffect(Unit) {
        while (isActive) {
            try {
                snapshot = OpenListBridge.snapshot()
                isInstalled = true
            } catch (_: Throwable) {
                isInstalled = false
            }
            delay(POLL_INTERVAL_MS)
        }
    }

    val running = (snapshot["running"] as? Boolean) ?: false
    val port = (snapshot["port"] as? Number)?.toInt() ?: 0
    val pid = (snapshot["pid"] as? Number)?.toInt() ?: 0
    val dataSizeBytes = (snapshot["data_size_bytes"] as? Number)?.toLong() ?: 0L
    val lastError = (snapshot["last_error"] as? String) ?: ""
    val lastUpdateTs = (snapshot["last_update_ts"] as? Number)?.toLong() ?: 0L

    val statusLabel = when {
        !isInstalled -> "未安装"
        running -> "运行中"
        lastError.isNotEmpty() -> "错误"
        else -> "已停止"
    }

    val statusColor = when {
        !isInstalled -> Color.Gray
        running -> Color(0xFF2DD36F)
        lastError.isNotEmpty() -> Color(0xFFEB445A)
        else -> Color(0xFF92949C)
    }

    val cardColor = when {
        running -> Color(0xFFE8F5E9)
        lastError.isNotEmpty() -> Color(0xFFFFEBEE)
        else -> Color.Transparent
    }

    Card(
        colors = CardDefaults.cardColors(containerColor = cardColor),
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(
                    text = "OpenList 状态",
                    style = MaterialTheme.typography.titleMedium,
                )
                Text(
                    text = statusLabel,
                    color = statusColor,
                    style = MaterialTheme.typography.labelMedium,
                )
            }

            if (!isInstalled) {
                Spacer(Modifier.height(8.dp))
                Text(
                    text = "OpenList 扩展未安装或 AAR 缺失",
                    color = Color.Gray,
                    style = MaterialTheme.typography.bodyMedium,
                )
            } else if (running) {
                Spacer(Modifier.height(12.dp))
                InfoGrid(
                    listOf(
                        "PID" to pid.toString(),
                        "端口" to port.toString(),
                        "数据大小" to formatFileSize(dataSizeBytes),
                        "心跳" to if (System.currentTimeMillis() - lastUpdateTs < 5000) "正常" else "超时",
                    ),
                )
                if (lastError.isNotEmpty()) {
                    Spacer(Modifier.height(8.dp))
                    Text(
                        text = "最近错误: $lastError",
                        color = Color(0xFFEB445A),
                        style = MaterialTheme.typography.bodySmall,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            } else if (lastError.isNotEmpty()) {
                Spacer(Modifier.height(8.dp))
                Text(
                    text = lastError,
                    color = Color(0xFFEB445A),
                    style = MaterialTheme.typography.bodyMedium,
                    maxLines = 3,
                    overflow = TextOverflow.Ellipsis,
                )
            } else {
                Spacer(Modifier.height(8.dp))
                Text(
                    text = "OpenList 已停止，点击下方按钮启动",
                    color = Color.Gray,
                    style = MaterialTheme.typography.bodyMedium,
                )
            }
        }
    }
}

// ── Control Card (Start / Stop) ────────────────────────────

@Composable
private fun ControlCard() {
    var isStarting by remember { mutableStateOf(false) }
    var isStopping by remember { mutableStateOf(false) }

    var snapshot by remember { mutableStateOf(OpenListBridge.snapshot()) }
    val running = (snapshot["running"] as? Boolean) ?: false

    // Refresh snapshot on action
    LaunchedEffect(isStarting, isStopping) {
        if (!isStarting && !isStopping) {
            delay(500)
            snapshot = OpenListBridge.snapshot()
        }
    }

    Card(modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(14.dp),
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Button(
                onClick = {
                    if (running) {
                        isStopping = true
                        OpenListBridge.shutdown(5000L)
                        isStopping = false
                    } else {
                        isStarting = true
                        OpenListBridge.start()
                        isStarting = false
                    }
                },
                enabled = !isStarting && !isStopping,
            ) {
                if (isStarting || isStopping) {
                    CircularProgressIndicator(
                        modifier = Modifier
                            .width(18.dp)
                            .height(18.dp),
                        strokeWidth = 2.dp,
                        color = MaterialTheme.colorScheme.onPrimary,
                    )
                } else {
                    Icon(
                        imageVector = if (running) Icons.Default.Stop else Icons.Default.PlayArrow,
                        contentDescription = null,
                    )
                }
                Spacer(Modifier.width(6.dp))
                Text(text = if (running) "停止" else "启动")
            }

            Text(
                text = if (running) "服务运行中，停止将断开所有连接" else "点击启动 OpenList 服务",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

// ── Configuration Card (Port / Password) ──────────────────

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ConfigCard() {
    var portStr by remember { mutableStateOf(OpenListConfig.DEFAULT_PORT.toString()) }
    var password by remember { mutableStateOf("") }
    var saved by remember { mutableStateOf(false) }

    // Load config from SharedPreferences
    LaunchedEffect(Unit) {
        // We need a Context to load config; use the bridge's appContext.
        // If not yet initialized, use defaults.
        try {
            // This will be called after init(), so appContext should be available.
            // For safety, we read from the bridge's current values.
            val snap = OpenListBridge.snapshot()
            // Port is cached in bridge after applyToBridge()
            portStr = (snap["port"] as? Number)?.toInt()?.toString()
                ?: OpenListConfig.DEFAULT_PORT.toString()
        } catch (_: Throwable) {
            // Not initialized yet, keep default
        }
    }

    Card(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.padding(14.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text(
                text = "配置",
                style = MaterialTheme.typography.titleMedium,
            )

            OutlinedTextField(
                value = portStr,
                onValueChange = { newValue ->
                    if (newValue.isEmpty() || (newValue.all { it.isDigit() } && newValue.length <= 5)) {
                        portStr = newValue
                    }
                },
                label = { Text("HTTP 端口") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )

            OutlinedTextField(
                value = password,
                onValueChange = { password = it },
                label = { Text("管理员密码（留空不变）") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )

            Button(
                onClick = {
                    val p = portStr.toIntOrNull() ?: OpenListConfig.DEFAULT_PORT
                    // Update bridge-side cache immediately
                    OpenListBridge.setPort(p)
                    if (password.isNotEmpty()) {
                        OpenListBridge.setAdminPassword(password)
                        password = ""
                    }
                    saved = true
                },
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(if (saved) "已保存" else "保存配置")
            }
        }
    }
}

// ── Helpers ─────────────────────────────────────────────────

@Composable
private fun InfoGrid(items: List<Pair<String, String>>) {
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        items.chunked(2).forEach { row ->
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceEvenly,
            ) {
                row.forEach { (label, value) ->
                    Column(
                        modifier = Modifier.weight(1f),
                        horizontalAlignment = Alignment.Start,
                    ) {
                        Text(
                            text = label,
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                        Text(
                            text = value,
                            style = MaterialTheme.typography.bodyMedium,
                            fontFamily = FontFamily.Monospace,
                        )
                    }
                }
                // Pad with empty columns if odd number of items
                if (row.size == 1) {
                    Column(
                        modifier = Modifier.weight(1f),
                    ) {}
                }
            }
        }
    }
}

private fun formatFileSize(bytes: Long): String {
    if (bytes < 1024) return "${bytes}B"
    if (bytes < 1024 * 1024) return "${bytes / 1024}KB"
    if (bytes < 1024 * 1024 * 1024) return String.format("%.1fMB", bytes / (1024.0 * 1024))
    return String.format("%.2fGB", bytes / (1024.0 * 1024 * 1024))
}
