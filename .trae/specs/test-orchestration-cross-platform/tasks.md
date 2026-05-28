# Tasks

- [ ] Task 1: 创建 GoProcessPlugin API 契约测试（T1）
  - [ ] 1.1 创建 `internal/service/goprocessplugin_contract_test.go`
  - [ ] 1.2 实现 @PluginMethod 方法签名枚举（通过读取 Kotlin 源码或硬编码预期签名表）
  - [ ] 1.3 实现 pendingCalls 键值一致性验证
  - [ ] 1.4 实现 BroadcastReceiver 注册/注销生命周期验证（mock context.registerReceiver）

- [ ] Task 2: 创建 ComboLite API 调用规范静态检查（T2）
  - [ ] 2.1 创建 `internal/service/combolite_static_check_test.go`
  - [ ] 2.2 实现「零反射」扫描：读取 GoProcessPlugin.kt 源码，搜索 Class.forName/getMethod/invoke 模式
  - [ ] 2.3 实现 InstallerManager 访问路径检查：确认 installPlugin 调用经过 installerManager
  - [ ] 2.4 实现 EncvApplication onFrameworkSetup 检查：确认 setHostActivity 调用存在

- [ ] Task 3: 创建加密 E2E 全链路测试（T3）
  - [ ] 3.1 创建 `internal/v2/plugins/video/encryption_roundtrip_e2e_test.go`
  - [ ] 3.2 准备小型测试 MP4 固件文件（或在测试中生成最小有效 MP4）
  - [ ] 3.3 实现 v3 容器 encrypt→decrypt→MD5 比对测试
  - [ ] 3.4 实现 v4 容器 encrypt→decrypt+verify（SkipStructCheck）测试
  - [ ] 3.5 实现加密后原始文件存在性验证（P0 防护回归）
  - [ ] 3.6 实现 ffprobe 异常格式容错验证（注入 frames 格式输出）

- [ ] Task 4: 创建前端↔Go API 对接测试（T4）
  - [ ] 4.1 创建 `app/encv-mobile/__tests__/api-contract.test.ts`
  - [ ] 4.2 实现 file list API 响应格式验证（mock server + 断言 response shape）
  - [ ] 4.3 实现 encrypt API 参数/响应格式验证
  - [ ] 4.4 实现 preview URL 生成逻辑验证（getFilePreviewUrl 参数组合）

- [ ] Task 5: 创建插件安装全链路前端测试（T6）
  - [ ] 5.1 创建 `app/encv-mobile/__tests__/extensions-install-flow.test.ts`
  - [ ] 5.2 mock GoProcessPlugin 的 {installPlugin, pickAndInstallPlugin, checkInstalledPlugins}
  - [ ] 5.3 实现安装状态机转换测试（idle→picking→confirming→installing→success）
  - [ ] 5.4 实现 120s 超时边界测试（BroadcastReceiver 模式应不再触发超时）
  - [ ] 5.5 实现 InstallConfirmActivity Intent 数据传递验证（mock bridge.startActivity）

- [ ] Task 6: 创建视频播放器启动链路测试（T7）
  - [ ] 6.1 创建 `app/encv-mobile/__tests__/player-entry.test.ts`
  - [ ] 6.2 mock PlayerEntry 的 startMpvPlayer / startArtPlayer
  - [ ] 6.3 实现 MPV 插件已加载时的 Intent 组件验证
  - [ ] 6.4 实现 MPV 未加载时的 ArtPlayer fallback 验证
  - [ ] 6.5 实现 ProxyManager 路由到 EncvHostActivity 的逻辑验证

- [ ] Task 7: 更新 CI 工作流测试矩阵（TC1）
  - [ ] 7.1 读取 `.github/workflows/android.yml`
  - [ ] 7.2 在现有 "Run Go unit tests" 步骤中扩展测试包路径（从 `./internal/service/` 到 `./internal/...`）
  - [ ] 7.3 添加 ComboLite 静态检查步骤（运行 Task 2 的测试）
  - [ ] 7.4 添加加密 E2E 测试步骤（可选：仅在 nightly 运行，标记 `[e2e]`）
  - [ ] 7.5 确保 Layer 1 总耗时 <5min（快速反馈）

# Task Dependencies
- [Task 1] 可独立并行
- [Task 2] 可独立并行
- [Task 3] 可独立并行（需要测试固件或生成逻辑）
- [Task 4] 可独立并行
- [Task 5] 可独立并行（前端测试）
- [Task 6] 可独立并行（前端测试）
- [Task 7] 依赖 [Task 1-6] 完成（CI 集成新测试）
