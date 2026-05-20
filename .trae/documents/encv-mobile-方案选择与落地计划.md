## Summary

- 推荐方案：采用 `1.md` 的“最小改动增强方案”作为主方案，即保留当前 `MainActivity` 直接拉起 Go 进程的架构，重点优化启动就绪检测、超时诊断、状态通知和失败可观测性。
- 吸收内容：从 `3.md` 中仅吸收前端状态反馈的细化思路，不引入新的前端控制架构，不重做 `useGoProcess.ts`。
- 明确不选：当前阶段不采用 `2.md` 的 `Foreground Service` 重构方案，也不采用 `3.md` 那种围绕 Service 重新设计前后端控制链路的完整重构方案。
- 选择原因：当前仓库已经具备 `MainActivity.kt` + `GoProcessPlugin.kt` + `src/plugins/GoProcess.ts` + `useServerStatus.ts` + `ServerDetail.vue` 的完整控制链，主要问题集中在 Android 侧“进程启动与就绪判定”不稳定，而不是缺少启停入口。

## Current State Analysis

### 现有实现

- Android 入口在 `app/encv-mobile/android-overlay/app/src/main/java/com/encvgo/app/MainActivity.kt`。
- `MainActivity.kt` 当前负责：
  - 注册 `GoProcessPlugin`
  - 从 assets 解压 `encv-go`
  - 在 `filesDir`/`cacheDir`/`externalFilesDir` 多目录尝试执行二进制
  - 用 `waitForBackendAndNotify()` 对 `/health` 做最多 30 秒、每 500ms 一次的轮询
  - 通过 `window.dispatchEvent('encv:backend-ready')` 通知前端
- Capacitor 原生桥接已存在于 `app/encv-mobile/android-overlay/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt`，且已支持：
  - `restart`
  - `stop`
  - `getStatus`
  - 通知权限 / 存储权限查询和请求
- 前端桥接与页面已经存在：
  - `app/encv-mobile/src/plugins/GoProcess.ts`
  - `app/encv-mobile/src/plugins/web.ts`
  - `app/encv-mobile/src/composables/useServerStatus.ts`
  - `app/encv-mobile/src/views/ServerDetail.vue`
- `post-cap-sync.mjs` 已经会把 `MainActivity.kt` 和 `GoProcessPlugin.kt` 覆盖到 Android 工程，说明当前定制点已固定在 overlay 模式。

### 已确认的问题

- 当前就绪判定完全依赖 HTTP 轮询，入口在 `MainActivity.kt` 的 `waitForBackendAndNotify()`。
- 轮询策略较慢：最多 30 秒，每次 sleep 500ms，坏情况下用户等待明显。
- 启动失败诊断弱：
  - 虽然采集了 `goProcessOutput`
  - 但超时只截取尾部 500 字
  - 缺少“日志判定 ready”的一级信号
- `findExecutableBinary()` 的多目录回退逻辑较重，但它仍是当前代码中唯一针对 `noexec` 的兼容手段；直接删除会增加回归风险。
- 仓库当前并没有自己的 AndroidManifest overlay，也没有 `EncvGoService.kt`，说明切换到 Service 不是“补几行代码”，而是一次新的 Android 生命周期设计。

### 三份方案与现状匹配度

- `1.md` 匹配度最高：
  - 与当前 `MainActivity` 架构一致
  - 可以直接落到已存在的方法与字段
  - 改动面可控
- `2.md` 匹配度中低：
  - 需要新增 `EncvGoService.kt`
  - 需要补 `AndroidManifest.xml` service 声明与前台服务权限
  - 需要处理 Android 12/13/14 前台服务限制和常驻通知
  - 会改变当前“Activity 持有进程”的控制边界
- `3.md` 匹配度中等但收益偏低：
  - 前端控制抽象更完整
  - 但当前仓库已经有 `GoProcess.ts`、`useServerStatus.ts`、`ServerDetail.vue`
  - 若底层启动可靠性不解决，前端再包装一层不会根治问题

## Proposed Changes

### 方案结论

- 选定方案：`方案一（保守增强版）`
- 不选方案：
  - 不做 `Foreground Service`
  - 不做 JNI / `.so` 重构
  - 不做新的前端控制抽象层大改

### 文件级改动计划

#### 1. `app/encv-mobile/android-overlay/app/src/main/java/com/encvgo/app/MainActivity.kt`

- 目标：把“启动是否成功”的判断从“纯 HTTP 轮询”升级为“日志事件优先 + HTTP 兜底”的混合机制。
- 具体修改：
  - 为 Go 进程输出监听增加 ready 关键字匹配，例如：
    - `listening on`
    - `ready`
    - `server ready`
  - 一旦命中 ready 关键词，立即触发端口探测和前端通知，避免继续傻等。
  - 重写 `waitForBackendAndNotify()`：
    - 超时从 30 秒缩到 10 秒
    - 轮询间隔从 500ms 缩到 200ms
    - 保留端口扫描逻辑
  - 强化错误诊断：
    - timeout 时打印更完整的 `goProcessOutput`
    - `lastStartError` 中保留更有用的退出态、日志尾部和失败类别
  - 修正并收敛状态流转：
    - 启动前清理旧的 `backendReady` / `lastStartError` / 输出缓冲
    - 避免既收到 ready 日志又重复通知前端
    - 进程退出时，如果尚未 ready，应明确上报失败
- 原因：
  - 当前启动不稳的核心就在这里
  - 此处改完即可显著提升成功率、启动速度和可诊断性

#### 2. `app/encv-mobile/android-overlay/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt`

- 目标：保持 API 不变，只修正与 Android 状态同步相关的细节。
- 具体修改：
  - 继续保留 `restart` / `stop` / `getStatus`
  - 配合 `MainActivity` 的新状态流转，确保：
    - `restart()` 在 ready 或明确失败后再 resolve/reject
    - `getStatus()` 返回值和 `MainActivity` 的真实状态一致
- 原因：
  - 当前前端接口已经接通，不需要破坏性调整
  - 让原生实现稳定优先于扩展能力

#### 3. `app/encv-mobile/src/composables/useServerStatus.ts`

- 目标：最小代价提升前端对原生状态的消费，不重做组合式封装。
- 具体修改：
  - 保留当前 `encv:backend-ready` 监听方案
  - 在收到 `detail.error` 时更明确地更新 `isOnline` / `lastError`
  - 在重启和停止过程中加入更清晰的状态切换，避免 UI 短时间显示旧状态
  - 仅在必要时补一个“等待 native 回调”的兜底处理，不引入新的轮询框架
- 原因：
  - 当前问题在原生侧居多
  - 前端只需配合，不应扩大改动面

#### 4. `app/encv-mobile/src/views/ServerDetail.vue`

- 目标：对齐新的状态反馈，但不做 UI 架构重写。
- 具体修改：
  - 保留现有按钮与权限区块
  - 如原生错误信息变得更明确，则直接复用当前 `connectionError` 展示
  - 仅在必要时补充 loading/失败提示文案细节
- 原因：
  - 当前页面已经具备启停交互
  - 不需要按 `3.md` 再新增一整套调试展示逻辑

#### 5. `app/encv-mobile/scripts/post-cap-sync.mjs`

- 目标：只在需要时同步 Android overlay 逻辑，不引入 Service 文件复制。
- 具体修改：
  - 保持当前 overlay 文件复制范围为 `MainActivity.kt` 和 `GoProcessPlugin.kt`
  - 如新增了与日志/配置有关的小型辅助文件，再一并纳入复制
- 原因：
  - 既然不选 Service 方案，就不应把 `EncvGoService.kt`、Manifest patch 等复杂度带进来

## Assumptions & Decisions

- 决策 1：本轮目标是“为当前仓库指定最适合落地的方案”，不是同时推进短期修复和长期架构升级。
- 决策 2：主问题被定义为“Android 端进程拉起后的就绪检测与诊断不足”，不是“前端缺失启停能力”。
- 决策 3：短期内继续保留 `MainActivity` 持有 `Process` 的结构，因为这与现有工程最一致，改动最小。
- 决策 4：暂不引入 `Foreground Service`，原因是：
  - 当前仓库没有相应清单与 Service 基础设施
  - 会显著扩大 Android 适配面
  - 会引入通知常驻、权限和审核层面的额外负担
- 决策 5：暂不引入 `3.md` 中新的 `useGoProcess.ts` 方案，原因是当前已有 `src/plugins/GoProcess.ts` 与 `useServerStatus.ts`，重复抽象收益不高。
- 假设 1：Go 后端输出中存在或可以识别稳定的 ready 关键词；若实际日志不包含 ready 标志，则回退到 HTTP 轮询兜底。
- 假设 2：当前 `findExecutableBinary()` 的多目录执行策略虽然不优雅，但在未完成 `jniLibs/.so` 重构前仍需要保留。

## Verification Steps

- Android 编译验证：
  - 执行 Capacitor Android 同步流程，确认 overlay 后 Kotlin 文件可正常编译。
- 启动链路验证：
  - 首次冷启动 App，确认后端能在 10 秒内完成 ready 通知或明确报错。
  - 观察日志，确认 ready 检测优先于纯 HTTP 超时。
- 重启链路验证：
  - 在 `ServerDetail.vue` 点击重启，确认：
    - UI 先进入重启中
    - 后端 ready 后恢复在线
    - 失败时展示更明确错误
- 停止链路验证：
  - 点击停止，确认：
    - `getStatus()` 返回 `running=false`
    - 前端状态变离线
    - 不保留旧端口信息造成误判
- 异常验证：
  - 人为制造配置错误或二进制不可执行场景，确认：
    - `lastStartError` 有可读信息
    - 前端能拿到错误并展示

## 最终建议

- 现在应执行的，就是把 `1.md` 中“短期修复现在的问题”落到现有仓库。
- `2.md` 可以保留为后续“长期后台常驻能力”预研方案，但不应作为当前第一步。
- `3.md` 的前端重构不应先做；只有在 Android 启动链路稳定后，再考虑是否值得继续抽象前端状态层。
