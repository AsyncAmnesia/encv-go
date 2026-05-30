# Plan：后端服务设置从一级界面移动到三级界面

## 一、现状分析

### 当前层级结构（2 级）

```
Level 1 (/tabs/settings — Settings.vue)
├── 外观（暗色模式、语言）
├── 播放器（视频/音频播放器、屏幕方向）
├── 连接 ← ⚠️ 后端服务设置在一级直接展示
│   ├── 🔴 后端服务  [在线/离线 badge + 端口]  ← goServer() → Level 2
│   └── 引擎状态    [FFmpeg/FFprobe badge]     ← goEngine() → Level 2 [native]
├── 缓存
├── 插件设置
├── 预览
├── DevTools
├── 关于
└── 编辑原始配置

Level 2 (/tabs/settings/server — ServerDetail.vue)
├── 连接
│   ├── 服务器地址（只读 + 复制）
│   └── 状态（在线/离线 + 刷新/停止/重启按钮）
├── 服务地址
│   ├── HTTP Server
│   ├── Admin Server
│   └── WebDAV Server
└── 权限 [native only]
    ├── 通知权限
    ├── 存储权限
    └── 电池优化
```

### 问题

Settings.vue 的「连接」区域（L89-L120）直接展示了：
- 服务器的在线/离线状态 badge
- 后端端口号
- 连接错误信息
- 引擎 FFmpeg/FFprobe 状态

这些属于**运维细节**，对普通用户是噪音。应该下沉到更深层级。

---

## 二、目标层级结构（3 级）

```
Level 1 (/tabs/settings — Settings.vue)  ← 精简后的主页
├── 外观（不变）
├── 播放器（不变）
├── 连接  ← 改为纯入口，无状态详情
│   └── 📡 连接设置  [一行入口，无 badge]  → goConnection() → Level 2
├── 缓存（不变）
├── 插件设置（不变）
├── 预览（不变）
├── DevTools（不变）
├── 关于（不变）
└── 编辑原始配置（不变）

Level 2 (/tabs/settings/connection — ConnectionDetail.vue)  ← 新建
├── 后端服务     [在线/离线 badge + 端口]  → goServer() → Level 3
├── 引擎状态     [FFmpeg/FFprobe badge]     → goEngine() → Level 3 [native]
└── （未来可扩展：代理设置、WebSocket 状态等）

Level 3 (/tabs/settings/server — ServerDetail.vue)  ← 不变
├── 连接（服务器地址、状态控制）
├── 服务地址（HTTP/Admin/WebDAV）
└── 权限（通知/存储/电池）[native]
```

---

## 三、实施步骤

### Step 1：新建 ConnectionDetail.vue（Level 2 连接设置页）

**文件**：`src/views/ConnectionDetail.vue`

**内容**：
- 从 Settings.vue L89-L120 提取「连接」区域的完整逻辑
- 包含 `useServerStatus` + `useEngineStatus`（或对应 composable）
- 两个入口项：
  1. **后端服务** → `router.push('/tabs/settings/server')`
  2. **引擎状态** → `router.push('/tabs/settings/engine')` [native only]
- 每项显示：名称 + 状态 badge（在线/离线 / FFmpeg 可用/不可用）

**模板结构**：
```vue
<ion-page>
  <ion-header>
    <ion-toolbar>
      <ion-buttons slot="start">
        <ion-back-button default-href="/tabs/settings"></ion-back-button>
      </ion-buttons>
      <ion-title>{{ t('settings.connection') }}</ion-title>
    </ion-toolbar>
  </ion-header>
  <ion-content>
    <ion-list>
      <!-- 后端服务入口 -->
      <ion-item button @click="goServer" detail>
        <ion-icon :icon="serverIcon" slot="start"></ion-icon>
        <ion-label>
          <h3>{{ t('settings.serverTitle') }}</h3>
          <p>
            <ion-badge :color="serverOnline ? 'success' : 'danger'">
              {{ serverOnline ? t('settings.online') : t('settings.offline') }}
            </ion-badge>
            <span v-if="serverOnline && backendPort">:{{ backendPort }}</span>
            <span v-if="!serverOnline && connectionError"> - {{ connectionError }}</span>
          </p>
        </ion-label>
      </ion-item>

      <!-- 引擎状态入口 [native only] -->
      <ion-item v-if="isNative()" button @click="goEngine" detail>
        <ion-icon :icon="filmOutline" slot="start"></ion-icon>
        <ion-label>
          <h3>{{ t('settings.engineStatus') }}</h3>
          <p>
            <ion-badge :color="engineStatus?.ffmpeg_available ? 'success' : 'danger'">
              {{ t('settings.ffmpegAvail') }}
            </ion-badge>
            <ion-badge :color="engineStatus?.ffprobe_available ? 'success' : 'danger'">
              {{ t('settings.ffprobeAvail') }}
            </ion-badge>
          </p>
        </ion-label>
      </ion-item>
    </ion-list>
  </ion-content>
</ion-page>
```

### Step 2：注册路由

**文件**：`src/router/index.ts`

在 `settings/server` 路由之前添加：

```typescript
{
  path: 'settings/connection',
  component: () => import('@/views/ConnectionDetail.vue'),
},
```

**路由顺序**（保持现有顺序，插入到 server 之前）：
```
settings              → Settings.vue (Level 1)
settings/connection   → ConnectionDetail.vue (Level 2)  ← NEW
settings/server       → ServerDetail.vue (Level 3)
settings/engine       → EngineDetail.vue (Level 3)
...
```

### Step 3：精简 Settings.vue 的「连接」区域（Level 1）

**文件**：`src/views/Settings.vue`

**改动 L89-L120**：将整个 `<ion-list>`（连接区域）替换为一行入口：

```vue
<!-- Before (Level 1 直接展示状态) -->
<ion-list>
  <ion-list-header><ion-label>{{ t('settings.connection') }}</ion-label></ion-list-header>
  <ion-item button @click="goServer" detail>  ...  </ion-item>   <!-- 含 badge + 端口 + 错误信息 -->
  <ion-item v-if="isNative()" button @click="goEngine" detail>  ...  </ion-item>  <!-- 含 FFmpeg badge -->
</ion-list>

<!-- After (Level 1 纯入口) -->
<ion-list>
  <ion-list-header><ion-label>{{ t('settings.connection') }}</ion-label></ion-list-header>
  <ion-item button @click="goConnection" detail>
    <ion-icon :icon="serverIcon" slot="start"></ion-icon>
    <ion-label>
      <h3>{{ t('settings.connectionSettings') }}</h3>
      <p>{{ t('settings.connectionSettingsDesc') }}</p>
    </ion-label>
  </ion-item>
</ion-list>
```

**script 变更**：
- 新增 `goConnection()` 函数：`router.push('/tabs/settings/connection')`
- 可以移除 `goServer()` 和 `goEngine()` 从 Settings.vue（它们移到了 ConnectionDetail.vue），或者保留作为 fallback
- `serverOnline`、`connectionError`、`backendPort`、`engineStatus` 等 **响应式变量可以移除**（不再在 Level 1 展示），减少 Settings.vue 的数据依赖

### Step 4：添加 i18n 键

**文件**：`src/composables/useI18n.ts`

新增两个键（中英文）：

```typescript
// 中文
'settings.connectionSettings': '连接设置',
'settings.connectionSettingsDesc': '后端服务、引擎状态、网络配置',

// English
'settings.connectionSettings': 'Connection',
'settings.connectionSettingsDesc': 'Backend service, engine status, network config',
```

### Step 5：调整 Settings.vue 的 script（保留必要逻辑）

**文件**：`src/views/Settings.vue`

**关键发现**：以下变量**不能移除**，因为被 `onMounted` 和 `watch` 使用：

| 变量 | 使用位置 | 作用 |
|------|---------|------|
| `serverOnline` | L756(onMounted), L807(watch) | 控制配置加载时机 |
| `engineStatus` | L761(onMounted), L814(watch) | 引擎状态缓存 |
| `connectionError` | useServerStatus 返回值 | 间接使用 |
| `backendPort` | useServerStatus 返回值 | 仅模板使用 |

**实际变更**：
1. ✅ **新增** `goConnection()` 函数 → `router.push('/tabs/settings/connection')`
2. ✅ **可移除** `goServer()` 函数（移至 ConnectionDetail.vue）
3. ✅ **可移除** `goEngine()` 函数（移至 ConnectionDetail.vue）
4. ❌ **保留** `useServerStatus` import 及其返回值（`serverOnline`, `connectionError`, `backendPort`）
5. ❌ **保留** `engineStatus` ref 和 `fetchFFmpegStatus` 调用
6. ⚠️ **模板中不再引用** `serverOnline`/`connectionError`/`backendPort`/`engineStatus`（它们仍存在于 script 中供 onMounted/watch 使用，只是不在 Level 1 展示）

### Step 6：验证

1. **vue-tsc** 零错误
2. **vitest** 全部通过（208/208）
3. **vite build** 成功
4. **手动验证路径**：
   - Settings → 点击「连接设置」→ 进入 ConnectionDetail（Level 2）
   - ConnectionDetail → 点击「后端服务」→ 进入 ServerDetail（Level 3）
   - ConnectionDetail → 点击「引擎状态」→ 进入 EngineDetail（Level 3）
   - 所有返回按钮正确回退

---

## 四、影响范围

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| `src/views/ConnectionDetail.vue` | **新建** | Level 2 连接设置聚合页 |
| `src/router/index.ts` | **修改** | 新增 `settings/connection` 路由 |
| `src/views/Settings.vue` | **修改** | 精简连接区域为单行入口 |
| `src/composables/useI18n.ts` | **修改** | 新增 2 个 i18n 键 |
| `src/views/ServerDetail.vue` | **不变** | 保持 Level 3 |
| `src/views/EngineDetail.vue` | **不变** | 保持 Level 3 |

---

## 五、不涉及的内容

- ❌ 不修改 ServerDetail.vue 内部逻辑
- ❌ 不修改 EngineDetail.vue 内部逻辑
- ❌ 不改变路由的嵌套结构（仍在 Tabs children 下）
- ❌ 不影响其他设置项（外观/播放器/缓存等保持原位）
