# Checklist

## Phase 0: 前置验证

- [ ] OpenList fork 在 NDK r26b 下 `bash build.sh release android` 成功
- [ ] `build/openlist-android-arm64` 存在且 `file` 输出 `ELF 64-bit LSB executable, ARM aarch64`
- [ ] `go list -m github.com/Soltus/encv-go` 在 fork 上下文可解析
- [ ] 二进制体积 ≤ 40MB（upx 后）
- [ ] `qemu-aarch64` 启动 sanity check 通过

## Phase 1: 插件模块骨架

- [ ] `app/encv-mobile/plugin-openlist/` 目录结构与 `plugin-mpv-player/` 一致
- [ ] `plugin-openlist/build.gradle.kts` 配置正确（library + aar2apk + compileOnly combolite-core）
- [ ] `android/settings.gradle.kts` 注册 `:plugin-openlist`
- [ ] `./gradlew :plugin-openlist:assembleDebug` 成功
- [ ] `OpenListPluginEntry` 实现 `IPluginEntry`，`onLoad`/`onUnload` 走 `OpenListService`
- [ ] `OpenListService` 仿 `EncvGoService` 提供前台服务 + WakeLock + NotificationChannel
- [ ] sidecar 进程 5s 内 `http://127.0.0.1:5244/api/site/list` 返回 200
- [ ] `onUnload` 释放端口 5244（在 3s 内）
- [ ] 端口冲突检测：socket connect 检测正确返回 CONFLICT 状态

## Phase 2: encv-go 自动发现

- [ ] `internal/openlist/multi_openlist.go` 新增 `tryRegisterLocalLoopback()`
- [ ] 启动时 2s 超时 GET `127.0.0.1:5244/api/site/list`，200 时插入 `local-loopback` site
- [ ] `SaveSites()` 跳过 `BuiltIn: true` site
- [ ] `internal/server/openlist_handlers.go` 新增 `GET /openlist/local/status`
- [ ] `/openlist/local/status` 返回 `running/pid/port/dataDirSize/lastHeartbeat`
- [ ] heartbeat 每次反代请求时刷新

## Phase 3: 前端 UI

- [ ] `<LocalOpenListStatusCard>` 组件就位
- [ ] 三态（未安装/端口冲突/运行中）正确展示
- [ ] 5s 轮询 status 端点
- [ ] 站点列表把 `local-loopback` 排第一
- [ ] "本地"角标用 IonChip `color="primary"`
- [ ] "打开 Web UI"按钮调起浏览器/Capacitor Browser 打开 5244
- [ ] ENCV 容器文件（.sccgv 等）经 `/openlist/local-loopback/...` 正常预览，Content-Type 正确

## Phase 4: 构建 & CI

- [ ] `scripts/build-openlist-android.sh` 入参/输出符合预期
- [ ] 脚本幂等：重复执行不破坏既有 fork 副本
- [ ] 输出 SHA256 打印到 stdout
- [ ] （可选）Gradle `preBuild` 钩子调用脚本成功
- [ ] 插件 AAR/APK 打包成功
- [ ] 真机端到端：插件装上 → encv-go 自动注册 local-loopback → 列表显示本地 site → ENCV 容器可预览

## 兼容性

- [ ] 现有远端 OpenList site 配置不变
- [ ] 现有 `internal/openlist/` 远端代理能力保留
- [ ] `EncvGoService` 不受影响
- [ ] 远端 OpenList 站点行为与本地完全一致
- [ ] iOS 端明确不在范围（CI 不应受本次改动影响）

## 文档

- [ ] README/README.md（plugin-openlist 目录）记录：依赖 fork 版本、NDK 版本、端口冲突解决
- [ ] `internal/openlist/multi_openlist.go` 顶部注释说明 `BuiltIn: true` 的含义

## 验证最终检查

- [ ] 在全新 emulator 上：
  - 1) 安装 host APK → 2) 安装 plugin-openlist APK → 3) 启动 App → 4) Openlist tab 显示"运行中" → 5) 点击站点访问远端文件 + 访问 ENCV 容器 → 6) 卸载插件 → 7) 端口释放，site 消失
- [ ] 重复以上步骤 3 次，确保幂等
- [ ] 关闭 App 后台杀进程 → 重新打开 → sidecar 自动重启（STICKY_SERVICE 验证）
