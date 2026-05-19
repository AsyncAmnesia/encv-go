# ENCV-Mobile 前端问题修复计划

## 问题总览

5 个需要修复的问题，涉及 6 个文件：

| # | 问题 | 涉及文件 | 优先级 |
|---|------|---------|--------|
| 1 | 后端连不上，无辅助信息 | `useServerStatus.ts`, `Settings.vue`, `useWebSocket.ts` | 高 |
| 2 | 日志级别 sheet 暗黑模式 + 外观/语言统一组件 | `Settings.vue` | 高 |
| 3 | GitHub 链接不正确 | `Settings.vue` | 中 |
| 4 | 配置保存/WebDAV 测试失败时日志输出详细信息 | `useConfig.ts`, `Settings.vue`, `WebDAV.vue`, `encv.ts` | 高 |
| 5 | 设置项必填/可选区分 | `Settings.vue`, `useI18n.ts` | 中 |

---

## 问题 1：后端连不上，无辅助信息

### 根因分析

当前连接状态展示存在两个问题：

1. **Settings 页面只显示 "在线/离线" badge**，没有任何诊断信息（连接地址、失败原因、重试建议）
2. **WebSocket 连接失败时静默重连**，用户完全不知道发生了什么
3. **App.vue 的 `connect()` 在 `onMounted` 中调用**，但 `useServerStatus` 也在 `onMounted` 中调用 `connect()`，存在重复初始化问题
4. **Files.vue 独立检查 `checkServerStatus()`**，与全局状态不同步

### 修复方案

**Settings.vue 连接区域增强**：
- 在 "离线" badge 旁显示当前连接地址
- 添加诊断信息：上次检查时间、失败原因（网络错误/服务未启动/地址错误）
- 添加 "查看日志" 按钮（跳转到 DevLogs 页面）

**useServerStatus.ts 增强**：
- 新增 `lastError` ref 记录最近一次连接失败的错误信息
- 新增 `lastCheckTime` ref 记录上次检查时间
- `checkStatus()` 捕获具体错误信息（不是简单 true/false）

**useWebSocket.ts 增强**：
- 在 `ws.onerror` 和 `ws.onclose` 中记录更详细的错误信息到 `lastError`
- 通过 eventBus 发送 `server:connection-error` 事件

**具体改动**：

```
useServerStatus.ts:
  + lastError: ref<string>('')
  + lastCheckTime: ref<Date|null>(null)
  + checkStatus() 改为捕获错误详情
  + 监听 server:connection-error 事件

useWebSocket.ts:
  + ws.onerror 中记录错误到 eventBus
  + ws.onclose 中记录关闭原因到 eventBus

Settings.vue:
  + 连接区域显示 serverUrl 值
  + 离线时显示错误原因
  + 添加 "查看日志" 链接按钮

encv.ts:
  + checkServerStatus() 返回更详细的错误信息（不只是 boolean）
```

---

## 问题 2：日志级别 sheet 暗黑模式 + 外观/语言统一组件

### 根因分析

当前 Settings.vue 中：
- **日志级别**（第152-166行）：使用 `ion-select` + `interface="action-sheet"` + `mode="ios"`，暗黑模式下 action-sheet 可能不适配
- **外观/语言**（第20-30行）：使用 `ion-select` + `interface="action-sheet"`，但**没有** `mode="ios"`

两者使用了不同的配置，不统一。

### 修复方案

**统一所有 select 使用 `interface="action-sheet"` + `mode="ios"`**：

Ionic 的 `action-sheet` 在暗黑模式下自动适配（因为 Ionic 组件本身支持暗黑模式）。问题可能出在 `mode="ios"` 上——这强制使用 iOS 风格，可能在 Android 暗黑模式下样式异常。

**改为统一使用 `interface="popover"`**：
- popover 在暗黑模式下完美适配（跟随系统主题）
- 视觉上更紧凑，适合设置页面
- 或者统一使用 `interface="action-sheet"` 但**不设置 `mode="ios"`**，让 Ionic 自动根据平台选择

**最终决定**：统一使用 `interface="action-sheet"`，移除 `mode="ios"`，让 Ionic 根据平台自动适配暗黑模式。

**具体改动**：

```
Settings.vue:
  - 语言 ion-select: 确保使用 interface="action-sheet"
  - 日志级别 ion-select: 移除 mode="ios"，确保 interface="action-sheet"
  两者统一风格
```

---

## 问题 3：GitHub 链接不正确

### 根因分析

当前代码（第336行）：
```js
window.open('https://github.com/encv-go', '_blank')
```

正确的仓库地址应该从 schema.json 的 `$id` 字段可以看出是 `Soltus/encv-go`，所以正确链接应为：
```
https://github.com/Soltus/encv-go
```

### 修复方案

直接修改 URL 为 `https://github.com/Soltus/encv-go`

---

## 问题 4：配置保存/WebDAV 测试失败时日志输出详细信息

### 根因分析

当前所有错误处理都是 `catch {}` 静默吞掉错误或只显示通用 toast：

1. **useConfig.ts `saveConfig()`**：`catch (error) { throw error }` — 只是重新抛出，不记录详情
2. **useConfig.ts `loadConfig()`**：`catch {}` — 完全静默，用默认值替代
3. **Settings.vue `handleSaveConfig()`**：`catch {}` — 只显示 "保存配置失败"，无具体原因
4. **WebDAV.vue `testConfig()/testConnection()`**：`testWebDAVConnection` 只返回 boolean，无错误详情
5. **encv.ts `testWebDAVConnection()`**：`catch { return false }` — 丢失所有错误信息
6. **encv.ts `checkServerStatus()`**：`catch { return false }` — 同上

### 修复方案

**核心思路**：让 API 函数抛出带详细信息的错误，在 UI 层捕获并展示。

**encv.ts 改动**：
- `testWebDAVConnection()` 改为抛出错误而非返回 false，包含 HTTP 状态码和响应体
- `checkServerStatus()` 改为返回 `{ online: boolean, error?: string }` 或抛出错误
- `updateConfig()` 的错误信息包含 HTTP 状态码和响应体

**useConfig.ts 改动**：
- `loadConfig()` 在 catch 中 `console.error` 记录详细错误
- `saveConfig()` 在 catch 中 `console.error` 记录详细错误再 re-throw

**Settings.vue 改动**：
- `handleSaveConfig()` 的 catch 中显示具体错误信息（如 "HTTP 400: password is required"）

**WebDAV.vue 改动**：
- `testConfig()/testConnection()` 的 catch 中显示具体错误信息

**具体改动**：

```
encv.ts:
  testWebDAVConnection(): 失败时 throw Error(含响应体/状态码)
  checkServerStatus(): 返回 { online, error? } 或 try/catch 中记录详情
  updateConfig(): 失败时 throw Error(含响应体)

useConfig.ts:
  loadConfig(): catch 中 console.error 详细信息
  saveConfig(): catch 中 console.error 详细信息再 throw

Settings.vue:
  handleSaveConfig(): toast 显示具体错误消息

WebDAV.vue:
  testConnection()/testConfig(): catch 错误并显示详情
```

---

## 问题 5：设置项必填/可选区分

### 根因分析

`schemaParser.ts` 已经解析了 `required` 字段并存入 `FieldDef.required`，但 `Settings.vue` 渲染时完全没有使用这个属性。

### 修复方案

在设置项 label 旁添加必填标记：

**Settings.vue 改动**：
- 对于 `ion-input` 的 label，如果 `field.required` 为 true，在 label 后添加红色 `*` 标记
- 对于 `ion-toggle`，不需要标记（boolean 字段无所谓必填）
- 在 `ion-input` 的 placeholder 中，必填项显示 "(必填)" 或 "(required)"

**useI18n.ts 改动**：
- 添加 `settings.requiredMark` i18n key

**具体改动**：

```
Settings.vue:
  ion-input label 改为动态计算：
  :label="tField(key) + (field.required ? ' *' : '')"
  或使用 ion-note 显示必填标记

useI18n.ts:
  + 'settings.requiredMark': '必填' / 'Required'
```

---

## 实施步骤

### Step 1: 修复 GitHub 链接（最简单，先做）
- 文件：`Settings.vue`
- 改动：`https://github.com/encv-go` → `https://github.com/Soltus/encv-go`

### Step 2: 统一 select 组件风格（暗黑模式适配）
- 文件：`Settings.vue`
- 改动：移除日志级别 `mode="ios"`，统一所有 `ion-select` 使用 `interface="action-sheet"`

### Step 3: 设置项必填/可选区分
- 文件：`Settings.vue`, `useI18n.ts`
- 改动：在 input label 旁显示必填标记

### Step 4: API 层错误信息增强
- 文件：`encv.ts`
- 改动：`testWebDAVConnection()`, `checkServerStatus()`, `updateConfig()` 等函数返回/抛出详细错误

### Step 5: 配置保存/WebDAV 测试错误详情展示
- 文件：`useConfig.ts`, `Settings.vue`, `WebDAV.vue`
- 改动：catch 中记录并展示具体错误信息

### Step 6: 后端连接诊断信息增强
- 文件：`useServerStatus.ts`, `useWebSocket.ts`, `Settings.vue`
- 改动：添加 lastError、lastCheckTime，Settings 显示连接诊断信息
