# Checklist

## Phase 1: 瘦身 Plugin APK（移除 Compose UI）

- [ ] `OpenListPluginEntry.kt` 不包含任何 `@Composable` UI 组件（StatusCard/ControlCard/ConfigCard 已删除）
- [ ] `OpenListPluginEntry.Content()` 返回最小占位 Composable
- [ ] `build.gradle.kts` 不包含 compose plugin / buildFeatures / compose dependencies
- [ ] `./gradlew :plugin-openlist:compileDebugKotlin` 编译通过
- [ ] plugin-openlist 产物 AAR 不包含 `androidx.compose.*` 类

## Phase 2: Host 侧 Capacitor UI

- [ ] `useOpenListManager.ts` 存在且 export 完整 API（runtime/start/stop/setAdminPassword/isControlling/error/refresh）
- [ ] `useOpenListManager.ts` 自动轮询 getOpenListRuntime() 每 3 秒
- [ ] start() 调用 controlOpenList('start') 并刷新状态
- [ ] stop() 调用 controlOpenList('stop') 并刷新状态
- [ ] setAdminPassword() 通过 ContentProvider IPC 写入密码
- [ ] isControlling 锁防止重复提交
- [ ] `LocalOpenListStatusCard.vue` 集成 useOpenListManager，包含：状态展示 + 启停控制 + 配置编辑 + WebUI 入口按钮
- [ ] "打开管理界面"按钮在 running=true 时可用，调用 Capacitor Browser 打开 5244
- [ ] ExtensionsPage.vue 已安装 OpenList 卡片显示运行状态 + "管理"入口
- [ ] `vue-tsc --noEmit` 0 errors

## Phase 3: 验证与清理

- [ ] Kotlin 全模块编译通过（plugin-openlist + combolite-host + app）
- [ ] TypeScript 编译通过（vue-tsc --noEmit 0 error）
- [ ] 无残留的 Compose import 在 plugin-openlist 的任何 .kt 文件中
- [ ] useOpenListBridge.ts 要么被 useOpenListManager 替换要么标记 deprecated
- [ ] 沙箱预览启动正常（npm run dev / scripts/start-preview.sh）
