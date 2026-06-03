# Checklist

## Phase 1: Capacitor 插件（Host App）

- [ ] `OpenListEmbedPlugin.kt` 已创建（`@CapacitorPlugin(name="OpenListEmbed")`）
- [ ] `OpenListEmbedService.kt` 已创建（`ConcurrentHashMap<containerId, Instance>`）
- [ ] `OpenListEmbedInstance.kt` 已创建（data class）
- [ ] `open/close/setBounds/isLoaded/navigate/getOpenListRuntime/controlOpenList` 方法实现
- [ ] WebView 错误自愈（启动 OpenList + 重试 3 次）
- [ ] 按需启动 OpenList（第一个 open() 触发 control('start')）
- [ ] 延迟停止（所有 WebView 关闭后 30s 计时器）
- [ ] GoProcessPlugin 移除 `getOpenListRuntime` / `controlOpenList` 方法
- [ ] `./gradlew :app:compileDebugKotlin` 通过

## Phase 2: TypeScript 插件

- [ ] `src/plugins/OpenListEmbed.ts` 已创建（registerPlugin）
- [ ] `src/plugins/openlist-embed/web.ts` 已创建（WebPlugin stub）
- [ ] GoProcess.ts 移除 OpenList 相关导出
- [ ] web.ts 移除 OpenList 相关接口
- [ ] 修复所有引用 `GoProcess.getOpenListRuntime` 改用 `OpenListEmbed.getOpenListRuntime`
- [ ] `npx vue-tsc --noEmit` 通过 (0 errors)

## Phase 3: Vue 容器组件

- [ ] `OpenListEmbedContainer.vue` 已创建（仅渲染 div + 调 open/close）
- [ ] `OpenListPage.vue` 已创建（含 OpenListEmbedContainer）
- [ ] router 添加 `/openlist` 路由
- [ ] `vue-tsc --noEmit` 通过

## Phase 4: 现有页面修改

- [ ] LocalOpenListStatusCard.vue 改用 `OpenListEmbed.getOpenListRuntime()`
- [ ] Remote.vue 添加"打开 OpenList"入口按钮
- [ ] ExtensionsPage.vue 已安装 OpenList 卡片增加"打开管理"按钮

## Phase 5: Plugin APK 瘦身

- [ ] OpenListPluginEntry.kt 删除 Compose UI（StatusCard/ControlCard/ConfigCard/InfoGrid/formatFileSize）
- [ ] 无 compose material3/icons/foundation/lifecycle import
- [ ] `pluginModule = emptyList()`
- [ ] `Content()` 返回 `Box {}` 最小占位
- [ ] build.gradle.kts 移除 compose plugin / buildFeatures / compose dependencies
- [ ] OpenListService 改为按需启动（移除 onCreate 自动启动）
- [ ] plugin-openlist 编译通过
- [ ] combolite-host 编译通过
- [ ] 产物 AAR 不含 `androidx.compose.*` 类

## Phase 6: 端到端验证

- [ ] 全模块编译通过
- [ ] `vue-tsc --noEmit` 0 errors
- [ ] 沙箱预览启动正常
- [ ] `/openlist` 路径可访问
- [ ] Native 侧 WebView 加载 OpenList Web UI
- [ ] 多例验证：多次 open 同一 containerId 复用；不同 containerId 独立实例
- [ ] close 单实例不影响其他实例
- [ ] 全部关闭后 30s 延迟停止 OpenList 后端
