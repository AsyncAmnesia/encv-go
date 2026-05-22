# 远端页面重构方案：WebDAV + Openlist 站点

## 现状分析

### 当前架构
- **Tab 栏**：文件 / 任务 / WebDAV / 设置 / DevLogs
- **WebDAV 页面**：纯前端 localStorage 存储的 WebDAV 配置列表，支持增删改查 + 测试连接
- **后端 WebDAV 服务器**：`config.webdav` 配置（port/root/dir/username/password），通过 `/api/config` 可获取
- **后端 Openlist 代理**：`config.proxy.sites` map，key 是 siteId，value 是 `ProxySiteConfig{host, description}`
- **后端 API**：`/api/config` GET 返回完整配置（含 webdav 和 proxy），`/api/webdav/test` 测试 WebDAV 连接
- **前端 API**：`fetchConfig()` 已能获取后端配置，`WebDAVConfig` 类型 + localStorage 存储

### 核心需求
1. WebDAV 页面改为"远端"页面，顶部 Tab 切换 WebDAV / Openlist 站点
2. encv 自身提供的 WebDAV 应当自动识别并添加到 WebDAV 列表

---

## 实现步骤

### Step 1：后端 — 新增 `/api/remote/info` 端点

**文件**：`internal/server/server.go` + `internal/server/mobile_api.go`

新增 API 端点，返回远端服务信息：

```json
GET /api/remote/info
{
  "webdav": {
    "enabled": true,
    "url": "http://127.0.0.1:8080/webdav/",
    "username": "admin",
    "root": "/webdav/"
  },
  "openlistSites": {
    "myalist": {
      "host": "http://192.168.1.100:5244",
      "description": "我的Alist",
      "proxyUrl": "http://127.0.0.1:2025/openlist/sites/myalist/"
    }
  }
}
```

关键逻辑：
- `webdav.enabled`：`config.Webdav.Port > 0`
- `webdav.url`：根据 `config.Server.Port`（本机 IP）+ `config.Webdav.Root` 拼接
- `openlistSites`：遍历 `config.Proxy.Sites`，为每个站点生成代理 URL
- 本机 IP 获取：使用 `net.InterfaceAddrs()` 或直接用 `127.0.0.1`（移动端场景）

### Step 2：前端 — API 层扩展

**文件**：`src/api/encv.ts`

1. 新增 `RemoteInfo` 接口：
```ts
export interface RemoteWebDAVInfo {
  enabled: boolean
  url: string
  username: string
  root: string
}

export interface OpenlistSiteInfo {
  host: string
  description: string
  proxyUrl: string
}

export interface RemoteInfo {
  webdav: RemoteWebDAVInfo
  openlistSites: Record<string, OpenlistSiteInfo>
}
```

2. 新增 `fetchRemoteInfo()` 函数

3. 扩展 `WebDAVConfig` 添加 `isBuiltIn?: boolean` 标记

### Step 3：前端 — 重构 WebDAV.vue 为 Remote.vue

**文件**：`src/views/WebDAV.vue` → 重命名为 `src/views/Remote.vue`

页面结构：
```
┌─────────────────────────────────────┐
│ 远端                                 │ ← ion-title
├─────────────────────────────────────┤
│ [WebDAV]  [Openlist]                │ ← ion-segment 顶部 Tab
├─────────────────────────────────────┤
│                                     │
│  WebDAV Tab:                        │
│  ┌─────────────────────────────────┐│
│  │ 🏠 本机 WebDAV    ● 已启用      ││ ← 自动识别，不可删除
│  │ http://192.168.1.x:8080/webdav/ ││
│  └─────────────────────────────────┘│
│  ┌─────────────────────────────────┐│
│  │ ☁️ 我的NAS       ○ 已保存       ││ ← 用户手动添加
│  │ https://nas.local/webdav        ││
│  └─────────────────────────────────┘│
│                                     │
│  Openlist Tab:                      │
│  ┌─────────────────────────────────┐│
│  │ 📂 我的Alist                    ││ ← 从后端配置自动读取
│  │ http://192.168.1.100:5244       ││
│  │ 代理: /openlist/sites/myalist/  ││
│  └─────────────────────────────────┘│
│                                     │
└─────────────────────────────────────┘
```

关键逻辑：
1. **顶部 Segment**：`ion-segment` 切换 WebDAV / Openlist
2. **WebDAV Tab**：
   - 调用 `fetchRemoteInfo()` 获取本机 WebDAV 信息
   - 如果 `webdav.enabled`，自动在列表顶部添加"本机 WebDAV"项（标记 `isBuiltIn=true`）
   - 本机 WebDAV 项：显示 URL + 用户名，不可删除/编辑，可测试连接
   - 用户手动添加的 WebDAV：保持现有逻辑（增删改查 + 测试）
3. **Openlist Tab**：
   - 从 `fetchRemoteInfo()` 获取站点列表
   - 每个站点显示：站点 ID + 描述 + 原始 Host + 代理 URL
   - 只读展示（站点配置在设置页面管理），可点击复制代理 URL
   - 点击站点可浏览文件（后续功能，当前只展示信息）

### Step 4：前端 — 路由和 Tab 更新

**文件**：`src/router/index.ts`

- 路由路径从 `webdav` 改为 `remote`

**文件**：`src/views/Tabs.vue`

- Tab 从 `webdav` 改为 `remote`
- 图标从 `cloud` 改为 `globe`（更符合"远端"含义）
- 标签从 `WebDAV` 改为 `远端` / `Remote`

### Step 5：前端 — i18n 翻译

**文件**：`src/composables/useI18n.ts`

新增翻译键：
```
tabs.remote: 远端 / Remote
remote.title: 远端 / Remote
remote.webdav: WebDAV
remote.openlist: Openlist
remote.builtInWebdav: 本机 WebDAV / Built-in WebDAV
remote.enabled: 已启用 / Enabled
remote.disabled: 未启用 / Disabled
remote.noOpenlistSites: 暂无 Openlist 站点 / No Openlist sites
remote.noOpenlistSitesDesc: 在设置中配置 Openlist 代理站点 / Configure Openlist proxy sites in Settings
remote.proxyUrl: 代理地址 / Proxy URL
remote.host: 原始地址 / Host
remote.copied: 已复制 / Copied
```

---

## 修改文件清单

| 文件 | 修改内容 |
|------|---------|
| `internal/server/server.go` | 注册 `/api/remote/info` 路由 |
| `internal/server/mobile_api.go` | 新增 `handleRemoteInfo` 处理函数 |
| `src/api/encv.ts` | 新增 RemoteInfo 类型 + fetchRemoteInfo + WebDAVConfig.isBuiltIn |
| `src/views/WebDAV.vue` → `src/views/Remote.vue` | 重构为远端页面（Segment Tab + 本机 WebDAV + Openlist） |
| `src/router/index.ts` | 路由从 webdav 改为 remote |
| `src/views/Tabs.vue` | Tab 从 webdav 改为 remote，图标改 globe |
| `src/composables/useI18n.ts` | 新增远端相关翻译 |

## 注意事项

- 本机 WebDAV 的 IP 地址：移动端场景下，后端和前端在同一设备上，使用 `127.0.0.1` 即可。但 WebDAV 服务的端口可能不同于 HTTP API 端口（`config.Webdav.Port` vs `config.Server.Port`）
- Openlist 站点配置是只读的：站点增删改在设置页面完成，远端页面只展示和操作
- WebDAV 配置仍然用 localStorage 存储（用户手动添加的），本机 WebDAV 从后端 API 动态获取
- 保留 `/api/webdav/test` 端点用于测试连接
