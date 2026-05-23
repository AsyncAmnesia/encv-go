# 修复计划：Lynx 播放器从 Vue 切换到 React + Sparkling 导航

## 背景

Vue Lynx 版播放器 UI 完全不工作，根因是 Lynx 默认 `display: linear` 导致所有 flex 布局失效。与其逐个修复 CSS，不如切换到 ReactLynx（Lynx 官方框架，文档完善、生态成熟），并使用 Sparkling（TikTok 官方 Lynx 容器框架）处理多页面导航。

## 架构变化

| 维度 | 当前（Vue Lynx） | 目标（React Lynx + Sparkling） |
|------|-----------------|-------------------------------|
| 框架 | vue-lynx 0.3.1 | @lynx-js/react 3.7 |
| 路由 | vue-router (createMemoryHistory) | Sparkling navigate/close（原生容器导航） |
| 多页面 | 单 bundle + vue-router | 多 bundle（每个页面独立 bundle） |
| CSS | 默认 linear 布局需显式 flex | 同样需显式 flex，但 ReactLynx 文档有完整示例 |
| 导航 | JS 端路由切换 | 原生容器级导航（Sparkling scheme URL） |
| Android 集成 | 手动 LynxViewBuilder | Sparkling SDK + HybridKit |

## 实施步骤

### 第一阶段：JS 项目重构

#### 步骤1：清理 Vue Lynx 项目，初始化 React Lynx 项目

- 删除 `lynx-player/src/` 下所有 Vue 文件
- 删除 `vue-lynx`、`vue-router` 依赖
- 添加 `@lynx-js/react`、`sparkling-navigation` 依赖
- 修改 `lynx.config.ts`：替换 `pluginVueLynx()` 为 `@lynx-js/react-rsbuild-plugin`，设置 `defaultDisplayLinear: false`
- 配置多入口（每个页面一个 bundle）：
  - `player` → `./src/pages/player/index.tsx`（播放器页面）
  - `home` → `./src/pages/home/index.tsx`（首页）

#### 步骤2：实现播放器页面（player bundle）

文件：`src/pages/player/index.tsx`

核心逻辑从 Vue 版 `PlayerView.vue` 移植，改为 React hooks：
- `useState` 替代 `ref`
- `useEffect` 替代 `onMounted`/`onUnmounted`
- `useCallback` 替代普通函数
- `lynx.__globalProps` 获取 initData（Sparkling 传递的 queryItems）
- `GlobalEventEmitter` 监听 mpv 事件
- `NativeModules` 调用 MpvPlayerModule/GoBackendModule

文件：`src/pages/player/PlayerControls.tsx`

从 Vue 版 `PlayerControls.vue` 移植，改为 React 函数组件：
- props 替代 defineProps/defineEmits
- 所有 CSS 添加 `display: flex`（因为 `defaultDisplayLinear: false` 全局设置后默认就是 flex，但显式声明更安全）

文件：`src/pages/player/ProgressBar.tsx`

从 Vue 版 `ProgressBar.vue` 移植。

文件：`src/pages/player/player.css`

播放器样式，从 Vue 版 CSS 移植。

#### 步骤3：实现首页（home bundle）

文件：`src/pages/home/index.tsx`

从 Vue 版 `HomeView.vue` 移植，改为 React：
- 使用 `sparkling-navigation` 的 `navigate()` 导航到播放器页面
- 不再使用 vue-router

文件：`src/pages/home/home.css`

#### 步骤4：实现设置页面（settings bundle，可选）

如果需要，可以后续添加。当前先聚焦播放器和首页。

### 第二阶段：Android 端集成 Sparkling

#### 步骤5：添加 Sparkling SDK 依赖

修改 `android/app/build.gradle`：
```groovy
implementation("com.tiktok.sparkling:sparkling:2.0.0")
```

#### 步骤6：初始化 HybridKit

修改 `EncvApplication.kt`：
```kotlin
import com.tiktok.sparkling.HybridKit
import com.tiktok.sparkling.SparklingHybridConfig
import com.tiktok.sparkling.SparklingLynxConfig
import com.tiktok.sparkling.baseinfo.BaseInfoConfig

// 在 onCreate 中：
HybridKit.init(this)
val baseInfoConfig = BaseInfoConfig(isDebug = BuildConfig.DEBUG)
val lynxConfig = SparklingLynxConfig.build(this) { }
val hybridConfig = SparklingHybridConfig.build(baseInfoConfig) {
    setLynxConfig(lynxConfig)
}
HybridKit.setHybridConfig(hybridConfig, this)
HybridKit.initLynxKit()
```

#### 步骤7：注册 Sparkling Method（路由）

```kotlin
import com.tiktok.sparkling.bridge.SparklingBridgeManager
import com.tiktok.sparkling.methods.router.RouterOpenMethod
import com.tiktok.sparkling.methods.router.RouterCloseMethod

SparklingBridgeManager.registerIDLMethod(RouterOpenMethod::class.java)
SparklingBridgeManager.registerIDLMethod(RouterCloseMethod::class.java)
```

需要添加 Sparkling Method 依赖：
```groovy
// JS 端安装后运行 npx sparkling autolink
```

#### 步骤8：重写 PlayerActivityLynx

使用 Sparkling 容器替代手动 LynxViewBuilder：

```kotlin
import com.tiktok.sparkling.Sparkling
import com.tiktok.sparkling.SparklingContext

// 在 onCreate 中：
val context = SparklingContext()
val scheme = "hybrid://lynxview_page?bundle=player.lynx.bundle&hide_nav_bar=1&hide_status_bar=1"
context.scheme = scheme
context.withInitData(buildInitDataJson())
Sparkling.build(this, context).navigate()
```

或者如果需要嵌入到现有 Activity 中（保留 MPV surface 控制），可以继续使用 LynxViewBuilder 但添加 Sparkling 的路由支持。

**关键决策**：由于播放器需要 MPV surface 嵌入到 LynxView 同级的 FrameLayout 中，Sparkling 的 `navigate()` 会打开新 Activity，可能不适合。更合理的方案是：
- 播放器页面继续使用 LynxViewBuilder（保持现有 MPV 集成方式）
- 首页到播放器的导航使用原生 Intent（现有方式）
- Sparkling 仅用于未来扩展（如设置页面等独立页面）

#### 步骤9：构建 bundle 并复制到 assets

```bash
cd lynx-player && npx rspeedy build
cp dist/player.lynx.bundle ../android/app/src/main/assets/
cp dist/home.lynx.bundle ../android/app/src/main/assets/
```

### 第三阶段：验证

#### 步骤10：构建验证

- JS 端：`npx rspeedy build` 成功
- Android 端：`./gradlew assembleDebug` 成功
- 运行时验证：播放器 UI 正确显示，播放功能正常

## 关键注意事项

1. **`defaultDisplayLinear: false`**：在 `lynx.config.ts` 中设置，使所有未声明 `display` 的元素默认使用 flex 布局
2. **ReactLynx 事件命名**：使用 `bindtap` 而非 `onClick`，`bindtouchstart` 而非 `onTouchStart`
3. **NativeModules 访问**：ReactLynx 中通过 `lynx.NativeModules` 或 `globalThis.NativeModules` 访问
4. **initData 获取**：Sparkling 通过 `lynx.__globalProps.queryItems` 传递；当前 LynxViewBuilder 通过 `lynx.__globalProps` 传递 initData
5. **MPV 集成**：播放器页面必须保持 LynxViewBuilder 方式（因为 MPV surface 需要嵌入到同层级），Sparkling navigate 不适合此场景
6. **CSS `display: flex`**：虽然 `defaultDisplayLinear: false` 使默认布局为 flex，但显式声明更安全
