# WebDAV 自动化测试 + 前端连接测试 + Android Mock 测试

## 问题

WebDAV 服务反复出问题，需要：
1. 在开发环境自动化测试 WebDAV 功能，确保基本功能正常
2. 前端添加 WebDAV 连接测试，让用户能快速判断 WebDAV 是否可用
3. Android 端 mock 测试，确保 GoProcessPlugin 和 EncvGoService 的 WebDAV 相关逻辑正确

## 方案

### 一、Go 后端自动化测试

编写 `internal/server/webdav_test.go`，启动完整服务器并测试 WebDAV 端点：

**测试用例**：
1. `TestWebDAV_PROPFIND_Root` — PROPFIND `/webdav/` 返回 207 Multi-Status
2. `TestWebDAV_PROPFIND_WithoutSlash` — PROPFIND `/webdav`（无尾部斜杠）返回 207
3. `TestWebDAV_AuthRequired` — 无凭据时返回 401
4. `TestWebDAV_AuthSuccess` — 正确凭据时返回 207
5. `TestWebDAV_AuthWrong` — 错误凭据时返回 401
6. `TestWebDAV_Options` — OPTIONS 请求返回正确头部
7. `TestWebDAV_GetFile` — GET 普通文件返回内容
8. `TestWebDAV_LogMiddleware` — WebDAV 请求被 slog 记录
9. `TestWebDAV_PROPFIND_SubDir` — PROPFIND 子目录返回正确内容

**实现方式**：
- 使用 `encv.NewServer()` + `server.Start()` 在随机端口启动完整服务器
- 创建临时目录作为 serving dir，放入测试文件
- 使用 `net/http` 客户端发送 WebDAV 请求（PROPFIND 用 XML body）
- 测试完成后 `server.Stop()` + 清理临时目录

**关键依赖**：
- 需要调用 `encv.Init()` 初始化加密模块
- 使用 `config.Config` 结构体配置 WebDAV（root、dir、username、password）

### 二、后端 API：测试本机 WebDAV

添加 `GET /api/webdav/test-local` 端点：

**返回信息**：
```json
{
  "available": true/false,
  "url": "http://127.0.0.1:2025/webdav/",
  "authRequired": true/false,
  "error": "错误信息",
  "details": {
    "propfindRoot": "ok/fail/skip",
    "authWorks": "ok/fail/skip",
    "dirReadable": "ok/fail/skip"
  }
}
```

**实现逻辑**：
1. 检查 `s.webdavFS != nil`（WebDAV 是否启用）
2. 构造本机 URL `http://127.0.0.1:{actualPort}/webdav/`
3. 发送 PROPFIND 请求到 `/webdav/`，检查返回 207
4. 如果配置了凭据，测试认证是否工作
5. 返回详细结果

### 三、前端 WebDAV 连接测试

在 Settings.vue 的 WebDAV 配置区域添加"测试连接"按钮：

**UI 设计**：
- 在 WebDAV section 的 list-header 右侧添加测试按钮
- 点击后调用 `/api/webdav/test-local`
- 显示测试结果（成功/失败 + 详细信息）
- 测试中显示 loading 状态

### 四、Android Mock 测试

编写 Android 单元测试，mock WebDAV 相关的 Android 组件：

**测试文件**：`android/app/src/test/java/com/encvgo/app/GoProcessPluginTest.kt`

**测试用例**：
1. `testRestart_PendingCallResolved` — restart 调用后，BroadcastReceiver 收到 BACKEND_READY 时正确 resolve
2. `testRestart_PendingCallRejected` — restart 调用后，收到错误广播时正确 reject
3. `testGetStatus_Running` — getStatus 返回正确的运行状态
4. `testGetStatus_Stopped` — getStatus 返回正确的停止状态
5. `testStop_ImmediateResolve` — stop 立即 resolve
6. `testResolvePendingCall_IgnoresWrongCommand` — 忽略非 restart 命令的广播

**测试文件**：`android/app/src/test/java/com/encvgo/app/EncvGoServiceTest.kt`

**测试用例**：
1. `testCheckHealth_Success` — mock HTTP 200 返回 true
2. `testCheckHealth_Failure` — mock 连接失败返回 false
3. `testBuildNotification` — 通知构建正确
4. `testResetStateForStart` — 状态重置正确
5. `testMaybeMarkReady_SetsRunning` — 标记就绪后 isRunning=true
6. `testMaybeMarkReady_IgnoresStaleGeneration` — 忽略过期的 generation

**依赖**：
- 添加 `testImplementation "org.mockito:mockito-core:5.8.0"`
- 添加 `testImplementation "org.mockito.kotlin:mockito-kotlin:5.2.1"`
- 添加 `testImplementation "org.robolectric:robolectric:4.11.1"`（用于 Android 框架类 mock）

**注意**：由于 EncvGoService 依赖 Android Service 框架，使用 Robolectric 或纯 Mockito mock。GoProcessPlugin 的测试可以纯 Mockito，因为核心逻辑（pendingCalls、BroadcastReceiver）不依赖 Android 框架。

## 实施步骤

### 步骤 1：编写 Go 后端自动化测试

1. 创建 `internal/server/webdav_test.go`
2. 实现 `setupTestServer` 和 `teardownTestServer` 辅助函数
3. 编写 9 个测试用例
4. 运行 `go test ./internal/server/ -run TestWebDAV -v -timeout 60s` 验证

### 步骤 2：添加后端 test-local API

1. `internal/server/mobile_api.go` — 添加 `handleTestLocalWebDAVGin` handler
2. `internal/server/server.go` — 注册 `GET /api/webdav/test-local` 路由

### 步骤 3：添加前端 WebDAV 测试按钮

1. `app/encv-mobile/src/api/encv.ts` — 添加 `testLocalWebDAV()` 函数和返回类型
2. `app/encv-mobile/src/views/Settings.vue` — 在 WebDAV section 添加测试按钮
3. `app/encv-mobile/src/composables/useI18n.ts` — 添加 i18n 键

### 步骤 4：编写 Android Mock 测试

1. `android/app/build.gradle` — 添加 Mockito 和 Robolectric 依赖
2. 创建 `android/app/src/test/java/com/encvgo/app/GoProcessPluginTest.kt`
3. 创建 `android/app/src/test/java/com/encvgo/app/EncvGoServiceTest.kt`
4. 运行 `./gradlew test` 验证

### 步骤 5：验证

1. `go test ./internal/server/ -run TestWebDAV -v` 通过
2. `go build ./internal/...` 通过
3. `vue-tsc --noEmit && vite build` 通过
4. Android 单元测试通过（如果 gradle 可用）
