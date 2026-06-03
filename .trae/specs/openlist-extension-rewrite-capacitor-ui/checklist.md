# Checklist

## Phase 1: 瘦身 Plugin APK（移除 Compose UI）

- [ ] `OpenListPluginEntry.kt` 不包含任何 `@Composable` UI 组件（StatusCard/ControlCard/ConfigCard 已删除）
- [ ] `OpenListPluginEntry.Content()` 返回空/最小占位 Composable
- [ ] `build.gradle.kts` 不包含 compose plugin / buildFeatures / compose dependencies
- [ ] `./gradlew :plugin-openlist:compileDebugKotlin` 编译通过
- [ ] plugin-openlist 产物 AAR 不包含 `androidx.compose.*` 类
- [ ] `./gradlew :combolite-host:compileDebugKotlin` 编译通过

## Phase 2: Host App 侧——"打开 OpenList"入口

- [ ] `LocalOpenListStatusCard.vue` 包含"打开 OpenList"按钮（调 Browser.open）
- [ ] 按钮仅在 running=true 时可用
- [ ] `@capacitor/browser` 在 package.json 中已声明依赖
- [ ] Remote.vue 未被改造为管理面板
- [ ] ExtensionsPage.vue 未增加启停控制或配置编辑功能

## Phase 3: 清理验证

- [ ] TypeScript 编译通过（vue-tsc --noEmit 0 error）
- [ ] plugin-openlist 目录无 Compose UI 残留引用
- [ ] 全项目无外部文件引用被删除的 Compose 组件名
