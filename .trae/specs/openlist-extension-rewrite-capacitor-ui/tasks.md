# Tasks

## Phase 1: 瘦身 Plugin APK（移除 Compose UI）—— 唯一核心改动

### 1.1 重写 `OpenListPluginEntry.kt`

- [ ] 1.1.1 删除所有 `@Composable` 私有函数：`StatusCard`, `ControlCard`, `ConfigCard`, `InfoGrid`, `formatFileSize`
- [ ] 1.1.2 删除所有 Compose 相关 import（`androidx.compose.*`, `androidx.material3.*`, `androidx.compose.material.icons.*`）
- [ ] 1.1.3 `Content()` 改为返回空 `Box {}` 或最小占位（不包含任何 UI 组件）
- [ ] 1.1.4 保留 `onLoad()`, `onUnload()`, `pluginModule` 不变
- [ ] 1.1.5 保留 `IPluginEntryClass`, `PluginContext`, `@Composable` 的 import（Content() 签名需要）

### 1.2 瘦身 `build.gradle.kts`

- [ ] 1.2.1 删除 `id("org.jetbrains.kotlin.plugin.compose")` plugin
- [ ] 1.2.2 删除 `buildFeatures { compose = true }`
- [ ] 1.2.3 删除 compose BOM 和所有 compose 依赖：
  - `implementation(platform(libs.compose.bom))`
  - `libs.compose.ui`
  - `libs.compose.runtime`
  - `libs.compose.material3`
  - `implementation("androidx.compose.material:material-icons-extended")`
  - `implementation("androidx.lifecycle:lifecycle-runtime-compose")`
- [ ] 1.2.4 保留的非 compose 依赖不变：
  - `compileOnly(libs.combolite.core)`
  - `implementation(files("libs/openlist-classes.jar"))`
  - `implementation("androidx.core:core-ktx")`
  - `implementation("androidx.localbroadcastmanager:localbroadcastmanager:1.1.0")`
  - `compileOnly("io.insert-koin:koin-core:4.1.0")`

### 1.3 编译验证

- [ ] 1.3.1 `./gradlew :plugin-openlist:compileDebugKotlin` 通过
- [ ] 1.3.2 确认产物不含 compose 类：`unzip -l build/outputs/aar/plugin-openlist-debug.aar | grep "compose"` 返回空
- [ ] 1.3.3 `./gradlew :combolite-host:compileDebugKotlin` 通过（确保 host 侧不受影响）

## Phase 2: Host App 侧——确保"打开 OpenList"入口

### 2.1 检查并确认 LocalOpenListStatusCard.vue

- [ ] 2.1.1 读取现有 `LocalOpenListStatusCard.vue`，确认当前是否已有"打开 OpenList"/"打开管理界面"按钮
- [ ] 2.1.2 如果没有 → 新增一个 `ion-button`，点击调用 `Browser.open({ url: 'http://127.0.0.1:5244/#/login' })`
- [ ] 2.1.3 按钮仅在 `runtime.running === true` 时 enabled
- [ ] 2.1.4 确认 `@capacitor/browser` 已在 package.json dependencies 中（检查是否有）

### 2.2 不改动 Remote.vue / ExtensionsPage.vue

- [ ] 2.2.1 确认 Remote.vue 不需要改造为管理面板（只读状态摘要 + 打开按钮已足够）
- [ ] 2.2.2 确认 ExtensionsPage.vue 不增加启停控制或配置编辑功能

## Phase 3: 清理验证

### 3.1 TypeScript 编译

- [ ] 3.1.1 `npx vue-tsc --noEmit` 通过（0 errors）

### 3.2 全局搜索残留引用

- [ ] 3.2.1 在 plugin-openlist 目录下 grep `StatusCard\|ControlCard\|ConfigCard\|@Composable` 确认无残留
- [ ] 3.2.2 在整个项目中 grep 引用被删除的 Compose 组件名称（确保无外部依赖）

## Task Dependencies

- Phase 1（1.1 → 1.2 → 1.3）顺序执行
- Phase 2 可与 Phase 1 并行（只读取/微调 Vue 文件）
- Phase 3 在 Phase 1 + 2 完成后执行
