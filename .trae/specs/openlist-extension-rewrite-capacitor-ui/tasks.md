# Tasks

## Phase 1: 瘦身 Plugin APK（移除 Compose UI）

### 1.1 重写 `OpenListPluginEntry.kt`

- [ ] 1.1.1 删除所有 `@Composable` 私有函数（`StatusCard`, `ControlCard`, `ConfigCard`, `InfoGrid`, `formatFileSize`）
- [ ] 1.1.2 `Content()` 改为返回最小 Composable（`Box + Text("OpenList Plugin — see host app for management")`）
- [ ] 1.1.3 删除所有 `androidx.compose.*` 的 import（保留 `IPluginEntryClass`, `PluginContext`, `Composable`, `@Composable` 最小集合）
- [ ] 1.1.4 保留 `onLoad()` / `onUnload()` / `pluginModule` 不变
- [ ] 1.1.5 验证 `./gradlew :plugin-openlist:compileDebugKotlin` 通过（移除 compose 后需同步改 build.gradle.kts，见 1.2）

### 1.2 瘦身 `build.gradle.kts`

- [ ] 1.2.1 删除 `id("org.jetbrains.kotlin.plugin.compose")` plugin
- [ ] 1.2.2 删除 `buildFeatures { compose = true }`
- [ ] 1.2.3 删除所有 `implementation(platform(libs.compose.bom))` 及其下的 compose 依赖：
  - `libs.compose.ui`
  - `libs.compose.runtime`
  - `libs.compose.material3`
  - `implementation("androidx.compose.material:material-icons-extended")`
  - `implementation("androidx.lifecycle:lifecycle-runtime-compose")`
- [ ] 1.2.4 保留 `compileOnly(libs.combolite.core)`, `implementation(files("libs/openlist-classes.jar"))`, `implementation("androidx.core:core-ktx")`, `implementation("androidx.localbroadcastmanager")`, `compileOnly("io.insert-koin:koin-core")`
- [ ] 1.2.5 验证 `./gradlew :plugin-openlist:compileDebugKotlin` 通过

### 1.3 清理无用的 import 和辅助函数

- [ ] 1.3.1 检查并清理其他 .kt 文件中因 Compose 移除而变得无用的 import（`OpenListBridge.kt`, `OpenListService.kt`, `OpenListConfig.kt`, `OpenListStatusProvider.kt` 应不受影响——它们不依赖 Compose）
- [ ] 1.3.2 验证 plugin-openlist 模块编译产物不包含任何 `androidx.compose.*` 类

## Phase 2: Host 侧 Capacitor UI（替代 Compose UI 功能）

### 2.1 新建 `useOpenListManager.ts` composable

- [ ] 2.1.1 新建 `src/composables/useOpenListManager.ts`
- [ ] 2.1.2 封装 `getOpenListRuntime()` 轮询（3s interval，来自现有 `useOpenListBridge.ts` 逻辑）
- [ ] 2.1.3 封装 `start()` → `controlOpenList('start')` + 自动 refresh
- [ ] 2.1.4 封装 `stop()` → `controlOpenList('stop')` + 自动 refresh
- [ ] 2.1.5 封装 `setAdminPassword(pwd)` → `controlOpenList('set_admin_password', { password: pwd })`
- [ ] 2.1.6 `isControlling` 防重复提交锁（start/stop 操作期间禁用按钮）
- [ ] 2.1.7 error 状态管理（最近错误展示）
- [ ] 2.1.8 onMounted 启动轮询，onUnmounted 清理定时器
- [ ] 2.1.9 导出 `{ runtime, start, stop, setAdminPassword, isControlling, error, refresh }`
- [ ] 2.1.10 `vue-tsc --noEmit` 通过

### 2.2 增强 `LocalOpenListStatusCard.vue` 为可交互管理面板

- [ ] 2.2.1 引入 `useOpenListManager` 替代或增强现有的 `useOpenListBridge`
- [ ] 2.2.2 新增控制区：Start 按钮（running=false 时显示）+ Stop 按钮（running=true 时显示），调用 `start()`/`stop()`
- [ ] 2.2.3 新增配置区：端口输入框（数字校验）+ 管理员密码输入框 + "保存"按钮
- [ ] 2.2.4 新增"打开管理界面"按钮：调 Capacitor Browser 插件打开 `http://127.0.0.1:5244/#/login`（仅在 running=true 时启用）
- [ ] 2.2.5 保留原有 4 态卡片变体（not_installed / running / port_conflict / stopped / crash_loop）
- [ ] 2.2.6 操作中按钮显示 spinner 或 disabled 状态（`isControlling` 锁）
- [ ] 2.2.7 操作失败时 toast/error 展示（error ref）
- [ ] 2.2.8 `vue-tsc --noEmit` 通过

### 2.3 增强 `Remote.vue` OpenList 区域

- [ ] 2.3.1 确认 `LocalOpenListStatusCard` 在 Remote.vue 中正确渲染（已有基础集成）
- [ ] 2.3.2 如果当前是独立组件形式，确认升级后的 StatusCard 包含全部功能（状态+控制+配置+WebUI 入口）
- [ ] 2.3.3 确保 5s 轮询 `/openlist/local/status` 或 `getOpenListRuntime()` 数据流正确

### 2.4 增强 `ExtensionsPage.vue` 已安装状态操作

- [ ] 2.4.1 已安装的 OpenList 卡片增加运行状态指示器（小圆点：绿色=运行中，灰色=已停止）
- [ ] 2.4.2 已安装且已启用的 OpenList 卡片将 enable/disable 按钮替换为"管理"按钮
- [ ] 2.4.3 "管理"按钮点击后 router.push 到 Remote tab（或 scroll 到 OpenList 面板区域）
- [ ] 2.4.4 未安装状态保持不变（install from local 按钮）

## Phase 3: 验证与清理

### 3.1 编译验证

- [ ] 3.1.1 `cd /workspace/app/encv-mobile && npx vue-tsc --noEmit` 通过（0 errors）
- [ ] 3.1.2 `cd /workspace/app/encv-mobile/android && ./gradlew :plugin-openlist:compileDebugKotlin` 通过
- [ ] 3.1.3 `./gradlew :combolite-host:compileDebugKotlin` 通过（确保 host 侧不受影响）
- [ ] 3.1.4 确认 plugin-openlist 产物 AAR 不包含 compose 类：`unzip -l plugin-openlist.aar | grep -c "compose"` 返回 0

### 3.2 功能验证（沙箱预览）

- [ ] 3.2.1 `npm run dev` 启动后 ExtensionsPage 正常加载
- [ ] 3.2.2 OpenList 扩展卡片正确显示已安装/未安装状态
- [ ] 3.2.3 Remote 页面 OpenList 管理面板渲染完整（状态+控制+配置）
- [ ] 3.2.4 Start/Stop 按钮点击后有正确的 loading 状态和错误处理

### 3.3 清理旧代码

- [ ] 3.3.1 确认 `useOpenListBridge.ts` 要么被 `useOpenListManager.ts` 完全替换，要么标记为 deprecated
- [ ] 3.3.1 检查是否有其他文件引用了被删除的 Compose 组件（全局搜索 `StatusCard` / `ControlCard` / `ConfigCard` 在 Kotlin 文件中的引用）

## Task Dependencies

- Phase 1（1.1 → 1.2 → 1.3）必须顺序执行
- Phase 2 可在 Phase 1 完成后开始
  - 2.1（useOpenListManager）先于 2.2（StatusCard 增强）
  - 2.3 / 2.4 可与 2.2 并行
- Phase 3 在 Phase 1 + 2 全部完成后执行
