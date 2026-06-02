# OpenList ComboLite 插件化集成 Spec

## Why

`/workspace/app/openlist/` 当前只是构建脚本与外部仓库链接，**没有任何在本仓库内可编译的 OpenList 集成代码**。"桌面端集成"实际上指 K-Sillot/OpenList-Desktop（Tauri）项目里把 OpenList 二进制作为 sidecar + 走 HTTP 协议进行联动。

encv-mobile 现有架构是：

```
WebView (Ionic Vue) → http://127.0.0.1:2025 → EncvGoService fork 的 encv-go 进程
                                                    └── internal/openlist/ 反代到「远端」OpenList
```

`internal/openlist/` 是**远端代理**，需要用户自备 OpenList 服务器。这意味着：

- 移动端用户必须有另一台机器跑 OpenList（家用 NAS、个人云等）
- 完全离线场景下，ENCV 容器解密需要往返局域网/互联网，延迟与稳定性差
- 桌面 OpenList-Desktop 把 OpenList 嵌进 App 的体验，在移动端缺失

同时已知：

- 用户在 `github.com/Hi-Sillot/OpenList` 上有 fork，已把 `github.com/Soltus/encv-go/pkg/encv/{plugins,openlist,reader}` 三个包 import 进 OpenList v4 内核（[internal/encv/init.go](file:///tmp/openlist-hisillot/internal/encv/init.go)），在 `server/handles/down.go` 的 `Proxy()` 路径上拦截 ENCV 容器，由 `handleEncvPreviewFromLink` 用 encv-go 的 reader 透明解密，再以正确的 Content-Type 返回
- 该 fork 的 `build.sh` 已有 `BuildReleaseAndroid` 函数，使用 NDK r26b 交叉编译 `android-arm64`，**二进制跨 Android 编译这条路径已经存在**
- encv-mobile 已稳定运行 ComboLite 2.0.2 框架（[plugin-mpv-player/](file:///workspace/app/encv-mobile/plugin-mpv-player/) 是参考实现）
- 用户决策：**本地 sidecar 二进制 + ComboLite 插件分发 + 仅 Android**

## What Changes

1. **新增 ComboLite 插件模块** `app/encv-mobile/plugin-openlist/`，沿用 `plugin-mpv-player` 的 Gradle 模式
   - `OpenListPluginEntry.kt`（`IPluginEntry` 实现）
   - `OpenListService.kt`（前台服务，仿 `EncvGoService` 管理 sidecar 进程生命周期）
   - `OpenListConfigActivity.kt`（管理 OpenList 端口、数据目录、密码）
   - 二进制资产路径：`src/main/jniLibs/arm64-v8a/libopenlist-arm64.so` → 在打包时由 `build-openlist-android.sh` 替换为真实 ELF

2. **新增 OpenList 编译脚本** `scripts/build-openlist-android.sh`
   - 克隆 `Hi-Sillot/OpenList` 到 `/tmp/openlist-hisillot`（若不存在）
   - 调用 `bash build.sh release android`，产出 `build/openlist-android-arm64`
   - 拷贝到 `plugin-openlist/src/main/jniLibs/arm64-v8a/libopenlist-arm64.so`
   - 文档化 NDK 路径依赖与 `BuildReleaseAndroid` 所需的 CGO 工具链

3. **OpenList 与 encv-go 的对接**（最小改动）
   - `internal/openlist/multi_openlist.go`：增加内置的 loopback site 常量 `LOCAL_OPENLIST_SITE_ID = "local"` 指向 `http://127.0.0.1:5244`
   - `internal/server/openlist_handlers.go`：增加 `/openlist/local/status`、`/openlist/local/sites/{id}/...` 的内部转发 helper，让 `Remote.vue` 现有的 Openlist tab 在检测到本地 OpenList 在线时**自动切换**到本地 site
   - **不**改 encv-go 的 ENCV 解密逻辑 —— 解密责任在 OpenList fork 一侧（避免双层解密与路径冲突）

4. **前端 UI 微调** `src/views/Remote.vue`
   - 新增"本地 OpenList 状态"卡片，显示 sidecar PID / 端口 / 数据目录
   - "安装 OpenList 插件"按钮调用宿主 ComboLite API 触发插件安装
   - 状态轮询通过现有 `task:refresh` 风格的 WS 消息通道（复用 `useWebSocket`）

5. **CGO 兼容性**（前置阻塞，需在 Phase 0 处理）
   - OpenList fork 引入的 `winfsp/cgofuse`、`wopan-sdk` 等含 C 依赖的 driver 在 Android 上大概率不可用
   - 需要在 build 时通过 `-tags=jsoniter` + 删除/隔离部分 driver，确认能在 `CGO_ENABLED=1` 下产出可在 Android 运行的精简二进制
   - 若失败，回退方案：让 `command.BuildReleaseAndroid` 单独把 fork 中 `cmd/server.go` 抽出来构建一个 "encv-openlist-android" 入口子命令，剔除所有 CGO driver

## Impact

- **Affected specs**：
  - `eval-combolite-mkv-ffmpeg-plugins`（同 ComboLite 模式，可复用插件分发/打包机制）
  - `implement-mobile-backend-api`（新增的 `/openlist/local/*` 是其扩展）
- **Affected code**：
  - 新增：`app/encv-mobile/plugin-openlist/`（整个插件模块）
  - 新增：`scripts/build-openlist-android.sh`（交叉编译入口）
  - 修改：`app/encv-mobile/android/settings.gradle.kts`（注册 `:plugin-openlist`）
  - 修改：`internal/openlist/multi_openlist.go`（加 local site）
  - 修改：`internal/server/openlist_handlers.go`（加 loopback 代理）
  - 修改：`src/views/Remote.vue`（本地 OpenList 状态卡）
- **不影响**：
  - `internal/openlist/` 的远端代理逻辑（保留，向后兼容）
  - `EncvGoService.kt`（不解 OpenList 进程化责任，保持现状）
  - iOS 端（用户明确排除）

## ADDED Requirements

### Requirement: ComboLite 插件模块 `plugin-openlist`

系统 SHALL 提供一个 ComboLite 插件模块 `app/encv-mobile/plugin-openlist/`，遵循 `plugin-mpv-player` 的目录结构与 Gradle 配置（`com.android.library` + `combolite-aar2apk` + `compileOnly(libs.combolite.core)`）。

#### Scenario: 插件模块被 Gradle 识别
- **WHEN** 在 `android/settings.gradle.kts` 添加 `include(":plugin-openlist")` 并 `project(":plugin-openlist").projectDir = file("../plugin-openlist")`
- **THEN** `./gradlew :plugin-openlist:assembleDebug` 成功产出插件 AAR（随后 aar2apk 打成 APK）

#### Scenario: 插件 APK 独立安装
- **WHEN** 用户从设置页点击"安装 OpenList 插件"或从 sdcard 选择 `plugin-openlist-release.apk`
- **THEN** ComboLite `PluginManager.install()` 返回成功，插件可在 `getInstalledPlugins()` 列表中查到

### Requirement: Sidecar 二进制管理

`OpenListPluginEntry` SHALL 在 `onLoad()` 中把 `src/main/jniLibs/arm64-v8a/libopenlist-arm64.so` 提取到 `context.filesDir/openlist/` 并通过 `Runtime.exec` 启动，监听 `127.0.0.1:5244`；在 `onUnload()` 中先 SIGTERM 等待 2s 再 SIGKILL。

#### Scenario: 插件加载时启动 sidecar
- **WHEN** ComboLite 触发 `IPluginEntry.onLoad()`
- **THEN** 在 5s 内 `http://127.0.0.1:5244/api/site/list`（OpenList 默认健康检查）返回 200；进程被托管在 `OpenListService` 下，系统设置 → 应用 → 运行中服务可见

#### Scenario: 插件卸载时清理进程
- **WHEN** 用户从 ComboLite 管理页卸载插件
- **THEN** `IPluginEntry.onUnload()` 触发 `OpenListService.stopOpenListProcess()`，端口 5244 在 3s 内被释放

### Requirement: 端口冲突与降级

`OpenListService` SHALL 在启动 sidecar 前检测 `127.0.0.1:5244` 是否已被占用；若已被占用，向宿主返回 `OPENLIST_PORT_CONFLICT` 状态码并通过 `BroadcastReceiver` 通知前端显示"5244 端口被占用，请在 OpenList 设置中修改"。

#### Scenario: 端口空闲
- **WHEN** 启动前 `socket.connect(new InetSocketAddress("127.0.0.1", 5244))` 抛 `ConnectException`
- **THEN** 继续正常启动

#### Scenario: 端口被占用
- **WHEN** `socket.connect(...)` 成功（即已有进程占着 5244）
- **THEN** 不启动新进程，发布 status=PORT_CONFLICT；前端可读出 `pid` 与 `occupiedBy`（本地 OpenList 或远端配置）

### Requirement: encv-go 自动发现本地 OpenList

`internal/openlist/multi_openlist.go` SHALL 在启动时执行 `http.Get("http://127.0.0.1:5244/api/site/list")`；若成功则自动注册内置 site `local-loopback`（host=`http://127.0.0.1:5244`，enable=true），名称为"本地 OpenList（Plugin）"。

#### Scenario: 插件已装，sidecar 在线
- **WHEN** encv-go 启动时本地 OpenList 可达
- **THEN** `GET /openlist/sites` 返回包含 `local-loopback` 的列表；用户不需手动配置

#### Scenario: 插件未装或 sidecar 离线
- **WHEN** 启动时 `http.Get` 失败或超时（>2s）
- **THEN** encv-go 不创建 `local-loopback` site；现有远端 site 配置不变

### Requirement: 解密责任单一化

OpenList fork SHALL 负责 ENCV 容器的解密与 Content-Type 识别（`.sccgv`/`sccgt`/`.sccgpdf`/`.sccgi`）；encv-go SHALL 不再对 `/openlist/*` 反代路径下的 ENCV 容器做二次解密。

#### Scenario: 透明解密 .sccgv
- **WHEN** 客户端请求 `GET /openlist/local-loopback/d/encrypt-test.sccgv?sign=xxx`
- **THEN** 链路是 encv-go(2025) → OpenList(5244) → 在 OpenList 内 `handleEncvPreviewFromLink` 中解密 → 以 `video/mp4` 流回；encv-go 仅做反向代理字节透传

### Requirement: 前端"本地 OpenList 状态"卡

`src/views/Remote.vue` SHALL 在 Openlist tab 顶部显示一张状态卡，字段：状态（运行中/未安装/端口冲突/未运行）、PID、端口、数据目录大小、最近一次心跳时间。

#### Scenario: 插件已装，sidecar 在线
- **WHEN** 页面 onIonViewWillEnter
- **THEN** 状态卡显示绿色"运行中"+ PID + 心跳时间（< 5s 内）

#### Scenario: 插件未装
- **WHEN** ComboLite `getInstalledPlugin("openlist") == null`
- **THEN** 状态卡显示"未安装"+ 跳转 ComboLite 商店的按钮

## MODIFIED Requirements

### Requirement: internal/openlist/multi_openlist.go 增加 LoopbackAutoRegister

`internal/openlist/multi_openlist.go` SHALL 在 `LoadSites()` 后增加 `tryRegisterLocalLoopback()` 调用；若 `127.0.0.1:5244` 健康检查通过则把内置 site `local-loopback` 插入到 `sites` 列表头部（用户可手动禁用）。

### Requirement: Remote.vue 站点列表排序

`src/views/Remote.vue` 的站点列表 SHALL 把 `local-loopback` 排在远端 site 之前，并在名称前显示"📱 本地"角标。

## REMOVED Requirements

无（保留所有现有远端 OpenList 代理能力）

---

## 风险与决策点（必须 Phase 0 解决）

| 编号 | 风险 | 决策依据 |
|------|------|---------|
| R1 | OpenList fork 的 CGO driver 在 Android NDK 下不能编译 | 在 Phase 0 Task 0.1 验证：去掉 `winfsp/cgofuse`、`wopan-sdk` 等 driver 后能否 `go build -o /dev/null -tags=jsoniter` 通过 |
| R2 | OpenList fork 强依赖 `OpenListTeam/OpenList/v4` 上游版本，encv-go plugin 包路径耦合 | 在 Phase 0 Task 0.2 确认 `go.mod` 中的 `replace` 指令与 fork 实际分支一致 |
| R3 | 二进制体积过大（参考桌面端 ~50-100MB），APK 加载慢 | 在 Phase 0 Task 0.3 用 `upx --best` + `-ldflags="-w -s"` 测一遍，< 40MB 才继续 |
| R4 | 端口 5244 与其他常用 App 冲突（alist 等） | 插件配置页允许用户改端口（`OpenListConfigActivity`），env var `OPENLIST_PORT` 透传 |
| R5 | OpenList 数据库（`data/`）写在 `filesDir/openlist/data`，占用用户存储 | spec 已固化路径，前端状态卡显示大小（见 ADDED Requirements 第 5 条） |

## 不在本次范围

- OpenList 的 Web 管理 UI（`5244/#/...`）从 WebView 打开
  - 由 OpenList fork 自带的 `public/dist` 静态资源提供；插件二进制已包含，**直接 Capacitor Browser 打开即可**（不写新的 Capacitor 插件）
- 多用户、远端 OpenList 集群管理
- OpenList 的存储驱动扩展
- iOS 端（用户排除）
