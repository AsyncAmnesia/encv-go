# 播放器修复计划：ArtPlayer 控件崩溃 + 暗黑模式缺失

## 问题分析

### 问题 1：ArtPlayer 黑屏无法播放
**错误信息**：`ArtPlayerError: option.controls.0.html require 'string' or 'Element' type`

**根因**：StandalonePlayer.vue 中 ArtPlayer 的 `controls` 配置写法错误：
```typescript
controls: [
  { name: 'play' },
  { name: 'time' },
  { name: 'progress' },
  { name: 'volume' },
  { name: 'settings' },
  { name: 'fullscreen-web' },
],
```
ArtPlayer 5.x 的 `controls` 选项是用于**自定义控件**的，每个控件必须有 `html` 属性（string 或 Element）。内置控件（播放、进度条、音量、全屏等）默认就会显示，不需要在 `controls` 数组中声明。

**对比**：主应用的 Player.vue 没有写 `controls` 数组，ArtPlayer 正常工作。

### 问题 2：播放器未继承暗黑模式
**根因**：`player-main.ts` 没有调用 `useTheme().initTheme()`，所以 `document.body` 上永远不会添加 `dark` class。主应用的 `main.ts` 通过 App.vue 间接调用了 `initTheme()`，但播放器入口完全跳过了这一步。

**补充问题**：`player-main.ts` 缺少主应用导入的多个 Ionic CSS 文件（structure、typography、padding、flex-utils、display），可能导致布局异常。

---

## 修复步骤

### 步骤 1：修复 ArtPlayer 控件配置
**文件**：`src/views/StandalonePlayer.vue`

- 删除 `initArtPlayer()` 中 ArtPlayer 构造参数的 `controls` 数组
- 与主应用 Player.vue 保持一致，让 ArtPlayer 使用默认内置控件

### 步骤 2：播放器入口添加暗黑模式初始化
**文件**：`src/player-main.ts`

- 导入 `useTheme`
- 在应用挂载前调用 `initTheme()`，读取 localStorage 中的 `encv-theme-preference` 并应用 `dark` class

### 步骤 3：补全 Ionic CSS 导入
**文件**：`src/player-main.ts`

- 补齐主应用 main.ts 中有但 player-main.ts 缺失的 Ionic CSS 导入：
  - `@ionic/vue/css/structure.css`
  - `@ionic/vue/css/typography.css`
  - `@ionic/vue/css/padding.css`
  - `@ionic/vue/css/flex-utils.css`
  - `@ionic/vue/css/display.css`

### 步骤 4：PlayerSettings 添加暗黑模式开关
**文件**：`src/views/PlayerSettings.vue`

- 导入 `useTheme`
- 在"播放"分组中添加暗黑模式 Toggle，与主应用 Settings.vue 一致
- 调用 `toggleDark()` 切换

### 步骤 5：本地构建验证
- 执行 `npm run build` 确认零错误
