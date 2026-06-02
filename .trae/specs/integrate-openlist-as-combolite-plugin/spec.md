# OpenList ComboLite 插件化集成 Spec

## Why

`/workspace/app/openlist/` 当前是构建脚本与外部仓库链接，**本仓库内没有可编译的 OpenList 集成代码**。"桌面端集成"指 K-Sillot/OpenList-Desktop（Tauri）项目；移动端目前通过 `internal/openlist/` **反代远端** OpenList 服务器。

### K-Sillot/OpenList-Mobile 的做法（参考）

K-Sillot/OpenList-Mobile 用的是 **gomobile bind → Android AAR** 路线：

1. `openlist-lib/openlistlib/{server,settings,common}.go` 是 OpenList 的 Go **库入口**（不是 main）
2. CI 用 `gomobile bind -ldflags "..." -v -androidapi 19` 把这个库编译为 `openlist.aar`
3. AAR 内部是 `lib/arm64-v8a/libgojni.so`（Go runtime + OpenList 二进制代码 + JNI stubs）
4. 宿主 APK 把 AAR 放进 `app/libs/`，`Openlistlib.java` 即可被 Kotlin import
5. OpenList 在**主进程**内运行（不走 child process），但内部 gin 仍绑定 `127.0.0.1:5244`
6. `OpenListService.kt`（前台服务）调用 `Openlistlib.init()` + `Openlistlib.start()` 拉起 server
7. log 回调走 `LogCallback` 接口，事件回调走 `Event` 接口（都是 gobind 自动生成的 Java 接口）

### 当前 encv-mobile 现状

- 主进程（WebView + Capacitor）+ `EncvGoService`（fork 出的 encv-go 子进程，端口 2025）
- encv-go 通过 `internal/openlist/` 反代**远端** OpenList
- ComboLite 框架已就位（[plugin-mpv-player](file:///workspace/app/encv-mobile/plugin-mpv-player/) 是参考实现）
- 用户 fork [Hi-Sillot/OpenList](file:///tmp/openlist-hisillot) 已把 encv-go 的 `pkg/encv/{plugins,openlist,reader}` import 进 OpenList 内核，在 `server/handles/down.go::Proxy()` 上拦截 ENCV 容器，由 `handleEncvPreviewFromLink` 透明解密

### 缺口

- 移动端用户没有 OpenList 服务器 → 必须本地化
- 桌面 OpenList-Desktop 把 OpenList 嵌进 App 的体验在移动端缺失
- 用户决策：**本地 OpenList（gomobile AAR）+ ComboLite 插件分发 + 仅 Android**

## What Changes

### 一、Hi-Sillot/OpenList fork 需补全 `openlistlib/` 入口

参考 K-Sillot/OpenList-Mobile，**Hi-Sillot fork 缺少 `openlistlib/` Go 库入口**。在 fork 仓库里新增：

| 路径 | 作用 |
|------|------|
| `openlistlib/server.go` | 暴露 `Init(Event, LogCallback)` / `Start()` / `Shutdown(timeoutMs)` / `IsRunning(type)` / `ForceDBSync()` |
| `openlistlib/settings.go` | 暴露 `SetConfigData(path)` / `SetConfigLogStd(b)` / `SetConfigDebug(b)` / `SetConfigNoPrefix(b)` / `SetAdminPassword(pwd)` |
| `openlistlib/common.go` | 暴露 `GetOutboundIPString()` |
| `openlistlib/internal/log.go` | `MyFormatter{OnLog func(*log.Entry)}`，把 logrus 转发到 Java 回调 |
| `openlistlib/event.go` | 暴露 `Event` 接口（`OnStartError`/`OnShutdown`/`OnProcessExit`） |

`server.go` 的 `Start()` 直接调用上游 `cmd/server.go` 已有的 `bootstrap.InitOfflineDownloadTools / LoadStorages / InitTaskManager` + `server.Init(r)` + `gin` ListenAndServe。Fork 已有的 `internal/encv/init.go` 提供的 `GenerateENCVSettingItems` / `LoadENCVPluginSettings` 由 `cmd/server.go` 在 `Start()` 开头调用（与 Hi-Sillot 当前实现一致）。

**保留** fork 的 `internal/encv/` + `server/handles/down_ext.go`（这是 ENCV 解密能力的核心），**新增** `openlistlib/` 仅作为 gomobile 入口。

### 二、encv-mobile 新增 `plugin-openlist` ComboLite 模块

`app/encv-mobile/plugin-openlist/`，仿 `plugin-mpv-player/` 骨架，但**不分 Compose UI**（OpenList 自带 Web 管理 UI，WebView 直接打开 `http://127.0.0.1:5244/#/`）：

```
plugin-openlist/
├── libs/
│   └── openlist.aar              # 由 build-openlist-aar.sh 生成（gomobile bind 产物）
├── src/main/
│   ├── AndroidManifest.xml
│   ├── java/com/encvgo/plugin/openlist/
│   │   ├── OpenListPluginEntry.kt        # IPluginEntry 实现
│   │   ├── OpenListService.kt            # 前台服务（仿 EncvGoService）
│   │   ├── OpenListBridge.kt             # 单例 wrapper，调 Openlistlib.*（仿 K-Sillot 的 model.openlist.OpenList）
│   │   ├── OpenListConfig.kt             # 端口 / 数据目录 / 管理员密码
│   │   └── OpenListEvent.kt              # Event + LogCallback 回调实现
│   └── res/...
├── build.gradle.kts
```

`OpenListBridge` 是核心：

```kotlin
object OpenListBridge : openlistlib.Event, openlistlib.LogCallback {
  fun init(dataDir: String) {
    Openlistlib.setConfigData(dataDir)
    Openlistlib.setConfigLogStd(true)
    Openlistlib.init(this, this)  // Init(Event, LogCallback)
  }
  fun start() = Openlistlib.start()
  fun shutdown(timeoutMs: Long) = Openlistlib.shutdown(timeoutMs)
  fun isRunning() = Openlistlib.isRunning("")
  fun setAdminPassword(pwd: String) = Openlistlib.setAdminPassword(pwd)
  fun forceDbSync() = Openlistlib.forceDBSync()
  // override onLog / onStartError / onShutdown / onProcessExit
}
```

`OpenListService` 仿 [K-Sillot 的 OpenListService.kt](file:///tmp/openlist-mobile-ksillot/android/app/src/main/kotlin/com/openlist/mobile/OpenListService.kt)：
- `FOREGROUND_ID` 通知 + NotificationChannel + WakeLock
- `onCreate`: `OpenList.addListener(this)` + acquire WakeLock
- `onStartCommand`: `OpenListBridge.init(filesDir/openlist/data)` + `OpenListBridge.start()`
- `onDestroy`: `OpenListBridge.shutdown(5000)` + release WakeLock
- 5 分钟一次的 `forceDbSync`（防止 SQLite WAL 丢失）

`OpenListPluginEntry`：

```kotlin
class OpenListPluginEntry : IPluginEntry {
  override fun onLoad(context: Context) {
    context.startForegroundService(Intent(context, OpenListService::class.java))
  }
  override fun onUnload(context: Context) {
    context.stopService(Intent(context, OpenListService::class.java))
  }
}
```

### 三、构建脚本 `scripts/build-openlist-aar.sh`

仿 K-Sillot 的 `openlist-lib/scripts/{init_openlist,init_web,init_gomobile,gobind}.sh`：

1. 接受 `--output <aar-path>` `--fork <git-url>` `--branch <branch>` `--ndk <path>` 入参
2. 临时克隆 Hi-Sillot/OpenList 到 `$WORK_DIR/openlist`
3. 准备 `public/dist/` 静态资源（OpenList-Frontend dist tar）
4. `go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init`
5. 修复 fork 的 `replace github.com/Soltus/encv-go => ../../../` 指向 `encv-go` 真实路径（用 sed 改成 `replace github.com/Soltus/encv-go => /workspace` 或绝对路径参数化）
6. `gomobile bind -ldflags "$ldflags" -v -androidapi 19 -target="android/arm64"`
7. 把产出的 `openlist.aar` 拷贝到 `--output` 指定路径

环境要求：
- Go 1.25.x（与 fork go.mod 一致）
- NDK r25c 或 r26b
- Java 17

### 四、encv-go 端最小适配

- `internal/openlist/multi_openlist.go`：增加内置 `local-loopback` site（host=`http://127.0.0.1:5244`），启动时 2s 超时健康检查
- `internal/server/openlist_handlers.go`：增加 `GET /openlist/local/status` 端点（running/pid/port/dataDirSize/lastHeartbeat）
- `cmd/encv-mobile/main.go`：**无改动**（encv-go 不感知 OpenList 是远端还是本地的 local-loopback）

> **关键洞察**：由于 OpenList 跑在**主进程内**（gomobile AAR + JNI），它**仍绑定** `127.0.0.1:5244`。encv-go 子进程像访问远端一样访问 loopback，**完全无需特殊代码**。

### 五、前端 Remote.vue 状态卡

`src/views/Remote.vue` 顶部新增 `<LocalOpenListStatusCard>`：
- 三态：未安装（引导）/ 端口冲突 / 运行中
- 5s 轮询 `/openlist/local/status`
- "打开 Web UI"按钮：`window.open('http://127.0.0.1:5244/#/login', '_system')`（Capacitor Browser 插件调起）
- 站点列表把 `local-loopback` 排第一 + "📱 本地"角标

## Impact

### Affected specs
- `eval-combolite-mkv-ffmpeg-plugins`（同 ComboLite 模式，可参考插件分发/打包）
- `implement-mobile-backend-api`（新增 `/openlist/local/status` 端点是其扩展）
- `fix-runtime-triple-defects` 等最近修复涉及的 `EncvGoService` 不受影响

### Affected code
**新增**：
- `app/encv-mobile/plugin-openlist/`（整个 ComboLite 插件模块）
- `scripts/build-openlist-aar.sh`（gomobile 入口）
- `scripts/build-openlist-aar.ps1`（Windows CI 镜像）

**修改**：
- `app/encv-mobile/android/settings.gradle.kts`（注册 `:plugin-openlist`）
- `app/encv-mobile/android/gradle/libs.versions.toml`（可选，OpenList 版本常量）
- `internal/openlist/multi_openlist.go`（loopback auto-register）
- `internal/server/openlist_handlers.go`（`/openlist/local/status`）
- `src/views/Remote.vue`（本地 OpenList 状态卡）
- `app/openlist/build-encv-desktop.ps1`（可选：标注已废弃，指向新文档）

**外部依赖**（在 fork 仓库）：
- Hi-Sillot/OpenList：`openlistlib/{server,settings,common,event}.go` + `openlistlib/internal/log.go`（新增）
- Hi-Sillot/OpenList：`go.mod` 的 `replace` 路径参数化

**不影响**：
- `internal/openlist/` 远端代理能力（保留）
- `EncvGoService` 与 encv-go 子进程
- iOS 端（用户明确排除）
- Capacitor WebView 本身

## ADDED Requirements

### Requirement: Hi-Sillot fork 提供 `openlistlib/` Go 库入口

fork SHALL 在 `openlistlib/` 提供 gobind 入口，暴露至少 8 个方法：`Init / Start / Shutdown / IsRunning / ForceDBSync / SetConfigData / SetConfigLogStd / SetAdminPassword` + 2 个接口 `Event / LogCallback`。

#### Scenario: gomobile bind 成功产出 .aar
- **WHEN** 在 fork 根目录运行 `gomobile bind -ldflags "$ldflags" -v -androidapi 19 -target="android/arm64"`
- **THEN** 生成 `openlist.aar`，包含 `lib/arm64-v8a/libgojni.so` + `classes.jar` 中的 `openlistlib.Openlistlib` Java stub

#### Scenario: AAR 在真机加载并启动
- **WHEN** APK 启动后 `System.loadLibrary("gojni")` 成功 + `OpenListBridge.init(dataDir)` + `OpenListBridge.start()`
- **THEN** 5s 内 `http://127.0.0.1:5244/api/site/list` 返回 200；JNI log 通过 `LogCallback.onLog` 流向 Kotlin 日志系统

### Requirement: ComboLite 插件 `plugin-openlist` 完整骨架

`app/encv-mobile/plugin-openlist/` SHALL 仿 `plugin-mpv-player/` 的 Gradle 结构（library + combolite-aar2apk + compileOnly(libs.combolite.core)），但**不**用 Compose；包含 `OpenListPluginEntry` / `OpenListService` / `OpenListBridge` / `OpenListConfig`。

#### Scenario: 插件模块编译成功
- **WHEN** `./gradlew :plugin-openlist:assembleDebug`
- **THEN** 产出 `plugin-openlist-debug.aar`（library 形态），再由 aar2apk 打成 `plugin-openlist-debug.apk`

#### Scenario: 插件 APK 独立安装
- **WHEN** 用户从设置页点击"安装 OpenList 插件"或选择 `plugin-openlist-release.apk`
- **THEN** ComboLite `PluginManager.install()` 返回成功；`getInstalledPlugin("openlist")` 命中

### Requirement: OpenList 主进程内运行（in-process）

`OpenListBridge.start()` SHALL 在主进程（JVM）内拉起 OpenList，**不**通过 `ProcessBuilder` 或 `Runtime.exec`；OpenList 内部 gin 服务器绑定 `127.0.0.1:5244`。

#### Scenario: 主进程承载 OpenList
- **WHEN** `OpenListBridge.start()` 调用后
- **THEN** 在 `lsof -i :5244` 中进程 PID 等于 host APK PID；OpenList 进程数 = 1（无 sidecar）

#### Scenario: encv-go 子进程可达
- **WHEN** encv-go 子进程 `http.Get("http://127.0.0.1:5244/api/site/list")`
- **THEN** 返回 200（说明 OpenList 在主进程内监听 loopback 成功）

### Requirement: OpenListService 前台保活

`OpenListService` SHALL 维持与 `EncvGoService` 同等水平的前台保活（FOREGROUND_ID 通知、NotificationChannel、PARTIAL_WAKE_LOCK、START_STICKY）。

#### Scenario: App 后台后 OpenList 仍存活
- **WHEN** 用户按 Home 键让 App 退到后台
- **THEN** 系统设置 → 应用 → 运行中服务列表中"OpenList"仍在

#### Scenario: App 被杀后重启自启
- **WHEN** App 被系统杀死但 BootReceiver 触发或用户手动打开
- **THEN** OpenListService 在 5s 内重新拉起 OpenList server

### Requirement: 端口冲突检测

`OpenListService` SHALL 在 `start()` 前用 `socket.connect(127.0.0.1:5244)` + 2s 超时检测端口占用；若占用则发布 `PORT_CONFLICT` 状态并跳过启动。

#### Scenario: 端口空闲
- **WHEN** socket connect 抛 `ConnectException`
- **THEN** 继续正常 start OpenList

#### Scenario: 端口被外部进程占用
- **WHEN** socket connect 成功
- **THEN** OpenList **不**启动；通过 `LocalBroadcastManager` 发出 `PORT_CONFLICT`；前端状态卡显示"5244 端口被占用，请在 OpenList 设置中修改端口"

### Requirement: encv-go 自动注册 local-loopback site

`internal/openlist/multi_openlist.go` SHALL 在 `LoadSites()` 后调用 `tryRegisterLocalLoopback()`：2s 超时 `http.Get("http://127.0.0.1:5244/api/site/list")`，成功则插入 site `{ID: "local-loopback", Name: "本地 OpenList（Plugin）", Host: "http://127.0.0.1:5244", Enable: true, BuiltIn: true}`。

#### Scenario: 插件在线
- **WHEN** encv-go 启动时 local OpenList 健康
- **THEN** `GET /openlist/sites` 列表中 `local-loopback` 在第一位

#### Scenario: 插件离线
- **WHEN** GET 超时或非 200
- **THEN** encv-go 不创建 `local-loopback`；远端 site 配置不变

#### Scenario: BuiltIn site 持久化隔离
- **WHEN** `SaveSites()` 被调用
- **THEN** 跳过 `BuiltIn: true` 的 site，避免污染用户配置文件

### Requirement: ENCV 解密责任单一化

ENCV 容器（`.sccgv`/`.sccgt`/`.sccgpdf`/`.sccgi`）的解密 SHALL **只在** OpenList fork 的 `handleEncvPreviewFromLink`（[server/handles/down_ext.go](file:///tmp/openlist-hisillot/server/handles/down_ext.go)）中发生；encv-go SHALL **不**对 `/openlist/*` 路径下的 ENCV 容器做二次解密。

#### Scenario: 透明解密 .sccgv
- **WHEN** 客户端请求 `GET /openlist/local-loopback/d/encrypt-test.sccgv?sign=xxx`
- **THEN** 链路：encv-go(2025) → loopback(5244) → OpenList `handleEncvPreviewFromLink` 解密 → 以 `video/mp4` 流回；encv-go 仅做反向代理字节透传

### Requirement: 前端 Remote.vue 状态卡

`src/views/Remote.vue` SHALL 在 Openlist tab 顶部显示状态卡，字段：状态 / PID / 端口 / 数据目录大小 / 最近心跳。

#### Scenario: 三态正确展示
- **WHEN** 页面 `onIonViewWillEnter` 触发
- **THEN** 状态卡根据 `GET /openlist/local/status` 返回值显示：未安装（引导安装） / 端口冲突（提示改端口） / 运行中（绿色）

#### Scenario: Web UI 跳转
- **WHEN** 用户点击"打开 OpenList Web UI"
- **THEN** Capacitor Browser 插件调起 `http://127.0.0.1:5244/#/login`

## MODIFIED Requirements

### Requirement: `internal/openlist/multi_openlist.go` 增加 LoopbackAutoRegister

`multi_openlist.go` 的 `LoadSites()` SHALL 在末尾调用 `tryRegisterLocalLoopback()`；返回的 `local-loopback` 站点的 `BuiltIn: true` 字段 SHALL 区分系统内置 vs 用户配置。

### Requirement: Remote.vue 站点列表排序

`Remote.vue` 站点列表 SHALL 把 `local-loopback` 排在远端 site 之前，名称前显示 IonChip "📱 本地"。

## REMOVED Requirements

无。

---

## 与 v1 spec 的关键差异（用户反馈后修正）

| 维度 | v1 spec（错） | v2 spec（本版） | 来源 |
|------|---------------|---------------|------|
| OpenList 部署形态 | sidecar 二进制 + Runtime.exec | **gomobile bind AAR + 进程内加载** | K-Sillot/OpenList-Mobile |
| Go runtime 数量 | encv-go + OpenList = 2 | **仅 encv-go 一个独立 runtime**（OpenList 共享 JVM 进程内的 Go runtime） | K-Sillot/OpenList-Mobile |
| 二进制体积 | ~50-100MB（.so + rclone + drivers） | ~30-50MB（一个 libgojni.so，AAR 内嵌） | 经验值 |
| 启动时间 | sidecar 冷启动 ~3-5s | in-process 调用 ~100ms | 经验值 |
| CGO 编译 | 需 NDK r26b 独立编译 .so | gomobile bind 自动处理 CGO + NDK | K-Sillot gobind.sh |
| fork 仓库改动 | 无（直接用 Hi-Sillot 当前形态） | **需新增 openlistlib/ 入口包** | K-Sillot OpenList-Mobile |
| 构建脚本 | 独立 `build-encv-local.sh` 已有 | 仿 K-Sillot `openlist-lib/scripts/{init_openlist,init_web,init_gomobile,gobind}.sh` 4 件套 | K-Sillot 仓库 |
| 端口冲突 | 通过 `socket.connect` 检测 | 同样方法 | 通用 |

## 风险与决策点（Phase 0 必答）

| 编号 | 风险 | 决策依据 |
|------|------|---------|
| R1 | Hi-Sillot fork 没有 `openlistlib/`，需提交 PR/手动添加 | 由用户在 fork 仓库添加（参考 K-Sillot 的 `openlistlib/` 包结构） |
| R2 | gomobile bind CGO 是否能与 encv-go 模块（含 CGO 子依赖）共存 | K-Sillot 已证明可行（OpenList 同样含 CGO） |
| R3 | `replace github.com/Soltus/encv-go => ../../../` 路径在 encv-mobile 仓库下不成立 | build-openlist-aar.sh 用 sed 改成 `replace github.com/Soltus/encv-go => /workspace` |
| R4 | AAR 体积（libgojni.so 包含完整 Go runtime） | K-Sillot 实测 ~40MB，`-ldflags="-s -w"` 后可接受 |
| R5 | iOS 端 gomobile bind 需独立 `-tags="ios,mobile"` 排包 | 用户明确仅 Android，**不处理** |
| R6 | 5244 端口冲突（alist 等） | 插件配置页允许改端口（`OpenListConfig`），通过 `setConfigData` + `OpenListConfigManager` 持久化 |

## 不在本次范围

- OpenList Web UI 的内嵌化（用 InAppBrowser / Capacitor Browser 直接调起）
- OpenList 多用户、集群、存储驱动扩展
- ENCV 解密逻辑改写（保留在 OpenList fork 侧）
- iOS 端（用户排除）
