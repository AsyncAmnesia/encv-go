# plugin-openlist

ComboLite 插件：在 Android 设备上以本地服务形式运行 OpenList，端口 5244，数据目录位于 App 沙箱内。

> **本模块属于 OpenList fork 集成的客户端；fork 侧工作流（clone / push / frontend pin / i18n overlay / 沙箱 GITHUB_TOKEN 推送）见 [`app/openlist/README.md`](../../openlist/README.md)。**

## 模块结构

```
plugin-openlist/
├── build.gradle.kts
├── libs/
│   └── openlist.aar          # 占位文件，真实 AAR 由 scripts/build-openlist-aar.sh 生成
├── src/main/
│   ├── AndroidManifest.xml
│   └── java/com/encvgo/plugin/openlist/
│       ├── OpenListPluginEntry.kt   # IPluginEntryClass 实现，onLoad 启 Service
│       ├── OpenListService.kt       # 前台 Service + WakeLock + 5 分钟 db sync + 端口冲突检测
│       ├── OpenListBridge.kt        # object 单例，封装 openlistlib 调用 + 广播转发
│       └── OpenListConfig.kt        # SharedPreferences 持久化（port / dataDir / adminPassword）
└── README.md
```

## 依赖

- `compileOnly(libs.combolite.core)` — ComboLite 核心运行时（宿主提供）
- `implementation(files("libs/openlist.aar"))` — gomobile bind 产物，提供 `openlistlib.Openlistlib` 等 Go 绑定 API
- `implementation("androidx.localbroadcastmanager:localbroadcastmanager:1.1.0")` — 进程内事件广播
- `compileOnly("androidx.compose.runtime:runtime")` — 仅用于 IPluginEntryClass.Content() 的 `@Composable` 注解；本插件不写任何 Compose UI

## 关键设计

### 1. 端口冲突解决

`OpenListService.startupSequence()` 在启动前以 **2 秒超时** `connect(127.0.0.1:5244)`：

- 连接成功 → 端口被占用，停止前台服务并 `LocalBroadcastManager` 发送 `BROADCAST_PORT_CONFLICT`（`EXTRA_CONFLICT_PORT`）
- 连接失败/超时 → 端口空闲，正常启动

### 2. 数据目录

默认 `${filesDir}/openlist/data`，即 App 私有目录：

```
/data/data/com.encvgo.plugin.openlist/files/openlist/data/
├── config.json
├── data.db
├── data.db-wal
├── data.db-shm
├── log/
└── ...
```

可通过 `OpenListConfig.save(context, port, dataDir, password)` 修改。

### 3. 后台保活

- `START_STICKY` — 系统回收后自动重建
- `PARTIAL_WAKE_LOCK` 标签 `openlist::Service` — 防止 CPU 休眠
- `startForeground(FOREGROUND_ID, ...)` + 通知渠道 `openlist_server`，`IMPORTANCE_LOW`
- 5 分钟 Handler 定时 `OpenListBridge.forceDbSync()`，合并 SQLite WAL

### 4. 广播协议

| Action | 触发方 | 携带 extras | 用途 |
|--------|--------|------------|------|
| `BROADCAST_STATUS_CHANGED` | Service / Bridge | `port`, `running` | 运行状态变更 |
| `BROADCAST_PORT_CONFLICT` | Service | `conflict_port` | 端口被占 |
| `BROADCAST_LOG` | Bridge | `level`, `time`, `log` | OpenList 内部日志 |
| `BROADCAST_PROCESS_EXIT` | Bridge | `code` 或 `reason` | 进程异常退出 |

### 5. 权限

| 权限 | 用途 |
|------|------|
| `INTERNET` / `ACCESS_NETWORK_STATE` / `ACCESS_WIFI_STATE` | OpenList 网络访问 |
| `FOREGROUND_SERVICE` | 前台 Service |
| `FOREGROUND_SERVICE_DATA_SYNC` | Android 14+ 前台服务类型 |
| `POST_NOTIFICATIONS` | Android 13+ 通知权限 |
| `WAKE_LOCK` | PARTIAL_WAKE_LOCK |
| `RECEIVE_BOOT_COMPLETED` | 开机自启（暂未实现 BroadcastReceiver，后续任务扩展） |

## 编译

```bash
# 1) 生成真实 openlist.aar（需要 gomobile 工具链）
bash scripts/build-openlist-aar.sh

# 2) 通过 aar2apk 插件编译插件 APK
./gradlew :plugin-openlist:assembleRelease

# 3) 产物路径
plugin-openlist/build/outputs/apk/release/plugin-openlist-release.apk
```

## 当前状态

- ✅ 插件框架：完整骨架，IPluginEntryClass 实现 + 前台 Service + 配置持久化
- ✅ 端口冲突检测 + 广播转发
- ✅ WakeLock / 5 分钟 db sync / START_STICKY
- ⚠️  `openlist.aar` 为 0 字节占位，运行时 OpenList 实际未启动；`OpenListBridge` 中的 Go 调用均为 TODO 注释
- ⚠️  BootReceiver 未实现（`RECEIVE_BOOT_COMPLETED` 权限已声明，待后续任务）
