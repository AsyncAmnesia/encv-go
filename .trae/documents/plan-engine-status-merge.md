# 引擎状态并入联机设置组 + 二级页面

## 目标

1. 将 Settings.vue 中独立的"引擎状态"分组移除，其入口并入"联机"设置组
2. 新建 `EngineDetail.vue` 二级页面，提供引擎运行时状态和构建配置细节
3. AboutDetail.vue 保留第三方库展示（库名 + 版本 + 许可证），但移除构建配置细节（NDK/ABI/CFLAGS/编解码器列表等），这些迁移到 EngineDetail

## 当前结构

- Settings.vue L83-100："联机"组 → 单项跳转 ServerDetail
- Settings.vue L102-129："引擎状态"组（仅 `isNative()`）→ 内联显示 FFmpeg/FFprobe badge + 错误文本
- ServerDetail.vue：服务器连接/服务地址/权限
- AboutDetail.vue：应用版本 + GitHub + **第三方库（FFmpeg 版本+license+构建配置+组件列表 / x264 版本+license+配置 / Go 版本）** + 危险区

## 信息分层

| 信息 | 关于页面 | 引擎详情 |
|------|---------|---------|
| FFmpeg 版本 + codename + license | ✅ | ✅ |
| FFmpeg NDK/API Level/ABI/链接方式/构建日期/CFLAGS | ❌ | ✅ |
| FFmpeg 解码器/编码器/封装器/解封装器/解析器/协议/滤镜/静态库列表 | ❌ | ✅ |
| x264 版本 + license | ✅ | ✅ |
| x264 配置选项 | ❌ | ✅ |
| Go 版本 + license | ✅ | ❌（Go 不是引擎组件） |
| FFmpeg/FFprobe 运行时可用性 + 错误详情 | ❌ | ✅ |

## 实施步骤

### 步骤 1：Settings.vue — 合并引擎状态到联机组

1. 在"联机"组的服务器入口下方，新增一个"引擎状态"入口项：
   - 图标：`filmOutline`（复用现有 import）
   - 标题：`t('settings.engineStatus')`
   - 副标题：简要状态摘要
     - Native 平台：显示 FFmpeg/FFprobe 各自的 ✅/❌ badge
     - 非 Native 平台：不显示此项（`v-if="isNative()"`）
   - 点击跳转 `/tabs/settings/engine`
   - 带 `detail` 箭头
2. 删除原来独立的"引擎状态"分组（L102-129 的整个 `<ion-list>`）
3. 删除不再需要的 CSS 类：`.engine-error-inline`、`.engine-detail-text`
4. `engineStatus` ref 和 `fetchFFmpegStatus` 调用保留（仍需在 Settings 概览中显示摘要 badge）

### 步骤 2：新建 EngineDetail.vue

路径：`/workspace/app/encv-mobile/src/views/EngineDetail.vue`

页面结构：

```
<ion-page>
  <ion-header>
    <ion-back-button default-href="/tabs/settings">
    <ion-title>引擎详情</ion-title>
  </ion-header>

  <ion-content>
    <!-- 运行时状态 -->
    <ion-list header="运行时状态">
      <ion-item> FFmpeg 可用性 badge + 错误详情 </ion-item>
      <ion-item> FFprobe 可用性 badge + 错误详情 </ion-item>
      <ion-item> 刷新按钮 </ion-item>
    </ion-list>

    <!-- 构建信息 -->
    <ion-list header="构建信息" v-if="buildInfo">
      <ion-item> FFmpeg 版本 + codename + license badge </ion-item>
      <ion-item> x264 版本 + license badge </ion-item>
      <ion-item> NDK 版本 </ion-item>
      <ion-item> API Level </ion-item>
      <ion-item> ABI </ion-item>
      <ion-item> 链接方式 </ion-item>
      <ion-item> 构建日期 </ion-item>
      <ion-item> CFLAGS（折叠展示） </ion-item>
    </ion-list>

    <!-- 组件列表（手风琴） -->
    <ion-list header="组件" v-if="buildInfo">
      <ion-accordion-group>
        <ion-accordion> 解码器列表 </ion-accordion>
        <ion-accordion> 编码器列表 </ion-accordion>
        <ion-accordion> 封装器列表 </ion-accordion>
        <ion-accordion> 解封装器列表 </ion-accordion>
        <ion-accordion> 解析器列表 </ion-accordion>
        <ion-accordion> 协议列表 </ion-accordion>
        <ion-accordion> 滤镜列表 </ion-accordion>
        <ion-accordion> 静态库列表 </ion-accordion>
      </ion-accordion-group>
    </ion-list>
  </ion-content>
</ion-page>
```

数据来源：
- 运行时状态：`fetchFFmpegStatus()` → `/api/ffmpeg-status`
- 构建信息：`fetchBuildInfo()` → `/api/build-info`
- 页面 `onMounted` 时并行请求两个 API
- 刷新按钮重新调用 `fetchFFmpegStatus()`

样式：从 AboutDetail.vue 迁移手风琴 + tag-list 相关 CSS

### 步骤 3：AboutDetail.vue — 精简第三方库展示

保留第三方库区块，但精简为"库名 + 版本 + 许可证"概览：

```
<ion-list header="第三方库">
  <ion-item> FFmpeg 图标 + "FFmpeg" + 版本 badge + license badge </ion-item>
  <ion-item> x264 图标 + "x264" + 版本 badge + license badge </ion-item>
  <ion-item> Go 图标 + "Go Runtime" + 版本 badge + "BSD" badge </ion-item>
</ion-list>
```

删除内容：
1. FFmpeg 手风琴展开内容（构建配置 + 组件列表）→ 迁移到 EngineDetail
2. x264 手风琴展开内容（配置选项）→ 迁移到 EngineDetail
3. Go 手风琴展开内容（版本详情）→ 直接在 item 中显示版本号即可
4. `ion-accordion-group` / `ion-accordion` 组件 → 改为普通 `ion-item` 列表
5. 所有手风琴/tag-list 相关 CSS
6. `formatDate` 函数（不再需要）

保留内容：
1. `buildInfo` ref + `fetchBuildInfo` 调用（仍需版本号和 license 信息）
2. `buildInfoLoading` / `buildInfoError` 状态
3. 应用版本、GitHub、危险区

### 步骤 4：路由注册

在 `/workspace/app/encv-mobile/src/router/index.ts` 的 tabs children 中添加：

```typescript
{
  path: 'settings/engine',
  component: () => import('@/views/EngineDetail.vue'),
},
```

### 步骤 5：i18n 新增 key

在 `useI18n.ts` 中添加：

| key | 中文 | English |
|-----|------|---------|
| `settings.engineDetail` | 引擎详情 | Engine Detail |
| `engine.runtimeStatus` | 运行时状态 | Runtime Status |
| `engine.buildInfo` | 构建信息 | Build Info |
| `engine.components` | 组件 | Components |
| `engine.available` | 可用 | Available |
| `engine.unavailable` | 不可用 | Unavailable |
| `engine.ffmpegVersion` | FFmpeg 版本 | FFmpeg Version |
| `engine.x264Version` | x264 版本 | x264 Version |
| `engine.ndkVersion` | NDK 版本 | NDK Version |
| `engine.apiLevel` | API Level | API Level |
| `engine.abi` | 架构 | Architecture |
| `engine.linking` | 链接方式 | Linking |
| `engine.buildDate` | 构建日期 | Build Date |
| `engine.cflags` | 编译标志 | CFLAGS |
| `engine.staticLinking` | 静态链接 | Static Linking |
| `engine.decoders` | 解码器 | Decoders |
| `engine.encoders` | 编码器 | Encoders |
| `engine.muxers` | 封装器 | Muxers |
| `engine.demuxers` | 解封装器 | Demuxers |
| `engine.parsers` | 解析器 | Parsers |
| `engine.protocols` | 协议 | Protocols |
| `engine.filters` | 滤镜 | Filters |
| `engine.staticLibs` | 静态库 | Static Libraries |
| `engine.refresh` | 刷新 | Refresh |
| `engine.loadFailed` | 加载失败 | Failed to load |
| `engine.configureOpts` | 配置选项 | Configure Options |

### 步骤 6：构建验证

```bash
cd /workspace && go vet ./internal/...
cd /workspace/app/encv-mobile && npx vue-tsc --noEmit && npx vite build
```

## 文件变更清单

| 文件 | 操作 |
|------|------|
| `app/encv-mobile/src/views/Settings.vue` | 编辑：引擎状态并入联机组，删除独立分组 |
| `app/encv-mobile/src/views/EngineDetail.vue` | 新建：引擎详情二级页面（运行时状态 + 构建信息 + 组件列表） |
| `app/encv-mobile/src/views/AboutDetail.vue` | 编辑：精简第三方库为版本+许可证概览，移除构建配置细节 |
| `app/encv-mobile/src/router/index.ts` | 编辑：添加 settings/engine 路由 |
| `app/encv-mobile/src/composables/useI18n.ts` | 编辑：添加 i18n key |

## 设计考量

1. **关于页面保留第三方库**：展示库名 + 版本 + 许可证，满足"关于"页面的开源合规需求
2. **构建配置细节迁移到引擎详情**：NDK/ABI/CFLAGS/编解码器列表等开发者/调试信息属于引擎详情
3. **运行时状态只在引擎详情**：FFmpeg/FFprobe 可用性是动态信息，放在引擎详情语义正确
4. **消除重复**：版本号在两处都显示是合理的（关于页面是概览，引擎详情是技术详情），但构建配置和组件列表只在引擎详情
5. **联机组概览只显示摘要**：Settings 联机组中的引擎入口只显示 ✅/❌ badge
