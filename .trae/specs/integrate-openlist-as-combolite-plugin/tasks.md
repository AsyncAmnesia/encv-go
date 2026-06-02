# Tasks

## Phase 0: 前置验证（必须全绿才能进 Phase 1）

- [ ] **Task 0.1**: 验证 OpenList fork 在 Android NDK r26b 下可编译
  - [ ] 0.1.1 准备干净的 `/tmp/openlist-hisillot`（已有 `git clone --depth 1 https://github.com/Hi-Sillot/OpenList.git`）
  - [ ] 0.1.2 在 fork 根目录运行 `bash build.sh release android`，记录错误信息
  - [ ] 0.1.3 若失败，定位 CGO 阻塞 driver（cgofuse / wopan / aliyundrive 等），逐个用 `// +build !android` 或 Go build tag 排除
  - [ ] 0.1.4 重新构建至产出 `build/openlist-android-arm64`（必须存在，> 30MB）
  - [ ] 0.1.5 `file build/openlist-android-arm64` 验证 ELF aarch64

- [ ] **Task 0.2**: 确认 encv-go 三个 import 包在 Go module 代理可达
  - [ ] 0.2.1 `go list -m github.com/Soltus/encv-go` 在 fork 的 go.mod 上下文中能解析
  - [ ] 0.2.2 `go mod download github.com/Soltus/encv-go/pkg/encv/plugins` 成功
  - [ ] 0.2.3 若私有仓库：在 fork 的 `go.mod` 添加 `replace github.com/Soltus/encv-go => ../encv-go`（相对路径指向 `/workspace`）

- [ ] **Task 0.3**: 体积与启动时间基线测量
  - [ ] 0.3.1 `ls -lh build/openlist-android-arm64` 记录大小
  - [ ] 0.3.2 `upx --best build/openlist-android-arm64` 后再记录
  - [ ] 0.3.3 `qemu-aarch64 -L /usr/aarch64-linux-gnu/ ./build/openlist-android-arm64 server --help` 验证可启动（基础 sanity check）
  - [ ] 0.3.4 若 > 40MB：在 fork 根用 `go build -gcflags="-l=4"` 试一遍；或拆成 server 子命令而非 main

## Phase 1: 插件模块骨架（1-2 天）

- [ ] **Task 1.1**: 创建 `:plugin-openlist` Gradle module
  - [ ] 1.1.1 新建 `app/encv-mobile/plugin-openlist/` 目录，仿 `plugin-mpv-player` 结构
  - [ ] 1.1.2 写 `build.gradle.kts`（library + combolite.aar2apk + compileOnly(libs.combolite.core) + namespace=`com.encvgo.plugin.openlist`）
  - [ ] 1.1.3 在 `android/settings.gradle.kts` 注册：`include(":plugin-openlist")` + `project(":plugin-openlist").projectDir = file("../plugin-openlist")`
  - [ ] 1.1.4 验证 `./gradlew :plugin-openlist:assembleDebug` 通过

- [ ] **Task 1.2**: 实现 `OpenListPluginEntry`
  - [ ] 1.2.1 `OpenListPluginEntry.kt` 实现 `IPluginEntry` 接口
  - [ ] 1.2.2 `onLoad(context)` 中：startForegroundService(OpenListService) → 异步等 sidecar 就绪
  - [ ] 1.2.3 `onUnload(context)` 中：stopService + kill process
  - [ ] 1.2.4 `onConfigurationChanged()` 中：端口配置变更时优雅重启 sidecar

- [ ] **Task 1.3**: `OpenListService` 进程管理
  - [ ] 1.3.1 仿 `EncvGoService.kt` 写前台服务（`NOTIFICATION_ID`、`NotificationChannel`、WakeLock）
  - [ ] 1.3.2 `findOpenListBinary()`：从 `applicationInfo.nativeLibraryDir` 找 `libopenlist-arm64.so`，`chmod 755` 后复制到 `filesDir/openlist/openlist`
  - [ ] 1.3.3 `startOpenListProcess()`：`ProcessBuilder(filesDir/openlist/openlist, "server", "--port", "$port", "--data", "$dataDir")`，`redirectErrorStream(true)`
  - [ ] 1.3.4 `stopOpenListProcess()`：先 SIGTERM 2s，超时则 SIGKILL
  - [ ] 1.3.5 `checkPortConflict()`：5s 超时 socket connect 127.0.0.1:5244

## Phase 2: encv-go 端的自动发现（0.5-1 天）

- [ ] **Task 2.1**: `internal/openlist/multi_openlist.go` 加 `tryRegisterLocalLoopback`
  - [ ] 2.1.1 在 `LoadSites()` 末尾调用 `tryRegisterLocalLoopback()`
  - [ ] 2.1.2 实现函数：2s 超时 GET `http://127.0.0.1:5244/api/site/list`，200 则插入 site `{ID: "local-loopback", Name: "本地 OpenList（Plugin）", Host: "http://127.0.0.1:5244", Enable: true, BuiltIn: true}`
  - [ ] 2.1.3 `SaveSites()` 跳过 `BuiltIn: true` 的 site，避免污染用户配置

- [ ] **Task 2.2**: `internal/server/openlist_handlers.go` 增加 health proxy
  - [ ] 2.2.1 新增 `GET /openlist/local/status` → 返回 `{running: bool, pid: int, port: int, dataDirSize: int64, lastHeartbeat: time}`
  - [ ] 2.2.2 `lastHeartbeat` 来源：encv-go 每次反代 OpenList 请求时刷新 `atomic.Int64` 时间戳
  - [ ] 2.2.3 `pid` 来自插件 manifest 还是进程探测？**决策**：通过 `/proc/net/tcp` + 端口反查 PID（Android 22+ 限制 → 备选用 `lsof` 或插件提供 GET 接口）

## Phase 3: 前端 UI（0.5-1 天）

- [ ] **Task 3.1**: `src/views/Remote.vue` 顶部状态卡
  - [ ] 3.1.1 写 `<LocalOpenListStatusCard>` 组件（Vue 3 SFC，setup script）
  - [ ] 3.1.2 字段：状态/PID/端口/数据目录/心跳；5s 轮询 `/openlist/local/status`
  - [ ] 3.1.3 三态：未安装（引导安装） / 端口冲突（提示修改） / 运行中（绿色 + 一键打开 Web UI）
  - [ ] 3.1.4 "打开 OpenList Web UI"按钮：`window.open('http://127.0.0.1:5244/#/login', '_blank', 'width=1024,height=768')`（Capacitor 走 InAppBrowser 或直接新 Activity 跳转）

- [ ] **Task 3.2**: 站点列表排序
  - [ ] 3.2.1 远端 sites 通过 `GET /openlist/sites` 拉；`local-loopback` 排第一
  - [ ] 3.2.2 卡片角标："📱 本地"用 IonChip `color="primary"` 渲染
  - [ ] 3.2.3 "已禁用"开关只对 `BuiltIn: false` 的 site 可见

## Phase 4: 构建与 CI（1 天）

- [ ] **Task 4.1**: `scripts/build-openlist-android.sh` 交叉编译脚本
  - [ ] 4.1.1 入参：`--ndk <path>` `--output <path>`，默认 NDK 来自 `$ANDROID_HOME/ndk/26.3.11579264`
  - [ ] 4.1.2 步骤：clone fork → `cd` → `bash build.sh release android` → 拷贝 `build/openlist-android-arm64` 到目标路径
  - [ ] 4.1.3 chmod 755、生成 SHA256、打印到 stdout
  - [ ] 4.1.4 若 fork 不存在 / 拉取失败：exit 1 并提示手动 `git clone`

- [ ] **Task 4.2**: Gradle `preBuild` 钩子（可选）
  - [ ] 4.2.1 在 `plugin-openlist/build.gradle.kts` 加 `tasks.named("preBuild") { dependsOn("buildOpenListBinary") }`
  - [ ] 4.2.2 自定义 task `buildOpenListBinary` 调用 `exec { commandLine("bash", "../../scripts/build-openlist-android.sh", "--output", "${project.projectDir}/src/main/jniLibs/arm64-v8a/libopenlist-arm64.so") }`
  - [ ] 4.2.3 CI 上预先构建产物，省去每次 `preBuild` 重编

- [ ] **Task 4.3**: 验证流水线
  - [ ] 4.3.1 本地：`./gradlew :plugin-openlist:assembleDebug` 产出 AAR
  - [ ] 4.3.2 `aar2apk` 任务产出 `plugin-openlist-debug.apk`
  - [ ] 4.3.3 在真机/模拟器上手动 `adb install` 后验证 `PluginManager.getInstalledPlugin("openlist")` 返回非 null
  - [ ] 4.3.4 端到端：插件装上 → encv-go 自动注册 local-loopback → 列表显示本地 site → 打开一个 ENCV 容器能正常预览

## Task Dependencies

- Phase 1 全部依赖 Phase 0 全绿
- Task 2.1 依赖 Task 1.3 至少有最小可用的 sidecar 启动逻辑（用于本地测试）
- Task 3.1 依赖 Task 2.2 提供 `/openlist/local/status` 端点
- Phase 4 的 Gradle preBuild 钩子依赖 Task 4.1 脚本可用

## 估时

- Phase 0：1-2 天（最大阻塞在 CGO 编译）
- Phase 1：1-2 天
- Phase 2：0.5-1 天
- Phase 3：0.5-1 天
- Phase 4：1 天
- **合计 4-7 天**
