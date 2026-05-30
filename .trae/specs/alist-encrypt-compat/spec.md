# Alist-Encrypt 兼容层 Spec（ENCV Plugin 架构）

## Why

encv-go 已有完善的 **ENCV Plugin 体系**（`internal/v2/plugins/`），包含 video/image/audio/text/pdf/wps 等插件，每个插件实现统一的 `Plugin` 接口并通过 `registry.go` 统一管理。现在需要新增一个 **alist-encrypt 插件**，使系统能够：
- 解密 alist-encrypt 格式（AES-128-CTR）的加密文件
- 加密文件为 alist-encrypt 格式
- 流式预览加密视频（支持 seek）
- 在线解码显示真实文件名（MixBase64）

**架构决策：三层分离**
1. **算法层** (`internal/alistencrypt/`) — 业务无关的基础设施（AES-128-CTR + MixBase64 + V2头检测），纯 Go 包 ✅ 已完成
2. **插件层** (`internal/v2/plugins/alistencrypt/`) — 实现 ENCV `Plugin` 接口，编排加解密流程，注册到 Plugins 列表 ✅ 已完成
3. **UI 层**（已有通道）— 通过 `mobile_api.go` + 前端复用现有任务/预览机制 ⚠️ 需完善

---

## What Changes

### 新增模块结构

```
internal/alistencrypt/                          ← Layer 1: 算法基础设施包（业务无关）✅ 已完成
├── cipher.go / aesctr.go / registry.go / errors.go / filename.go / content_header.go / reader.go
└── alistencrypt_test.go                       ← ✅ 52 个测试通过

internal/v2/plugins/alistencrypt/                ← Layer 2: ENCV Plugin 实现（业务编排）✅ 已完成
├── plugin.go / types.go / encryptor.go / decryptor.go / streamer.go
└── (待补充 plugin_test.go)

app/encv-mobile/src/                            ← Layer 3: 移动端 UI ⚠️ 骨架已有，需完善
├── api/encv.ts                                ← ✅ streamAlistFile / decodeAlistFilename
├── views/Files.vue                             ← ⚠️ 基本识别+菜单，需完善密码输入/状态反馈
├── views/Tasks.vue                             ← ⚠️ 需确认任务类型展示
├── composables/useAlistEncrypt.ts              ← 🆕 抽取 composable（密码管理/状态）
├── views/Files.vue                             ← 🆕 完善交互流程
└── __tests__/                                 ← 🆕 Vitest mock 测试
    ├── encv.alistencrypt.spec.ts               ← API 函数 mock 测试
    ├── Files.alistencrypt.spec.ts              ← 文件检测逻辑测试
    └── useAlistEncrypt.spec.ts                ← composable 逻辑测试
```

### 不在 MVP 范围内（TODO）

- **OpenList 代理集成**：接入 internal/openlist/ 代理链
- **桌面端 UI**：openlist 桌面客户端适配
- **RC4MD5 / ChaCha20 扩展**：必须通过 Cipher 接口 + Register() 引入，禁止进入主包

---

## ADDED Requirements

### Requirement: 算法隔离架构（铁律）✅ 已实现

> 详见 spec.md 原始内容，Phase 1 已完成。

### Requirement: AES-128-CTR 核心算法 ✅ 已实现

### Requirement: 文件名 MixBase64 加解密 ✅ 已实现

### Requirement: V2 内容头自动检测 ✅ 已实现

### Requirement: ENCV Plugin 接口实现 ✅ 已实现

### Requirement: 后缀安全校验 ✅ 已实现

### Requirement: 流式预览 ✅ 已实现

### Requirement: 文件名解码 API ✅ 已实现

### Requirement: 注册与发现 ✅ 已实现

---

## ADDED Requirements（本次新增 — 移动端 UI 完善）

### Requirement: 密码输入与管理流程

系统 SHALL 提供完整的密码输入和管理机制，而非使用硬编码的空字符串。

#### 场景：用户点击解密时弹出密码输入
- **WHEN** 用户在 Files.vue 中长按 `.bin` 文件选择「解密」
- **THEN** 弹出 IonAlert 或自定义密码输入弹窗（非 Toast）
- **AND** 弹窗包含：密码输入框 + 「记住本次会话」开关 + 取消/确认按钮
- **AND** 用户确认后密码被传递给解密 API；如果勾选「记住」则缓存在内存中（session 级别，不持久化）

#### 场景：流式预览也需要密码
- **WHEN** 用户选择「流式预览」且未缓存密码
- **THEN** 同样弹出密码输入弹窗
- **AND** 输入完成后构造带 password 参数的 stream URL

#### 场景：默认密码自动填充
- **WHEN** 后端配置了 `default_password` 且非空
- **THEN** 密码输入弹窗预填该值，用户可直接确认或修改

### Requirement: 操作状态反馈与错误展示

系统 SHALL 为所有 alist-encrypt 操作提供清晰的状态反馈。

#### 解密操作状态机：
```
idle → confirming_password → submitting → in_progress → completed/failed
```

| 状态 | UI 表现 |
|------|--------|
| `confirming_password` | 密码输入弹窗 |
| `submitting` | Loading spinner + toast「正在提交任务...」 |
| `in_progress` | 自动跳转 Tasks.vue 页面（复用现有任务进度展示） |
| `completed` | Tasks.vue 中显示绿色成功标记 |
| `failed` | Tasks.vue 中显示红色错误 + 可展开详情 |

#### 错误分类展示（遵循 frontend-design.md 规范）：

| 错误类型 | 展示方式 |
|---------|---------|
| 密码错误 | 特殊样式提示（红色背景 + lock 图标），与数据损坏区分 |
| 文件格式无效 | `task-error` 样式 |
| ErrExtensionRequired | `task-warning` 样式 + 说明「该算法需要扩展包」 |
| 网络超时 | 重试按钮 |

#### 流式预览状态：
- 流 URL 构建中 → 播放器区域显示 loading indicator
- 流加载失败 → 显示错误信息 + 「重试」按钮
- 播放中 → 正常播放器界面

### Requirement: useAlistEncrypt Composable

抽取 `useAlistEncrypt()` Vue composable，集中管理 alist-encrypt 相关状态和逻辑：

```typescript
// composables/useAlistEncrypt.ts
export function useAlistEncrypt() {
  const sessionPasswords = ref<Record<string, string>>({}) // path → password
  const decodedNames = ref<Record<string, string>>()       // path → plainName
  
  async function promptPassword(file: FileItem): Promise<string | null>
  async function decodeFilename(file: FileItem): Promise<string>
  function getStreamUrl(file: FileItem): string
  function isAlistEncrypted(file: FileItem): boolean
  function cachedPassword(file: FileItem): string | undefined
  
  return { sessionPasswords, decodedNames, promptPassword, decodeFilename, getStreamUrl, isAlistEncrypted, cachedPassword }
}
```

**设计原则**：
- 密码仅内存缓存（session 级别），不写入 localStorage/secureStorage（安全性考虑）
- 文件名解码结果缓存避免重复请求
- 所有 API 调用通过此 composable 统一管理，方便 mock 测试

### Requirement: 设置页集成

在现有设置页面中增加 alist-encrypt 配置区域（如果后端 `/api/config/schema` 返回了 `alist_encrypt` 字段）：

| 配置项 | 控件类型 | 说明 |
|--------|---------|------|
| enabled | ion-toggle | 启用/禁用 alist-encrypt 功能 |
| suffix | ion-input | 自定义后缀（含冲突校验提示） |
| default_password | ion-input（type=password） | 默认密码 |
| enc_type | ion-select | 加密算法选择（MVP 仅 aesctr 可选） |

**校验规则同步后端**：
- 输入 `.sccgv` 或 `.encv` 时显示红色警告 + 提示文本
- suffix 不以 `.` 开头时自动补全

### Requirement: ExtensionsPage.vue 集成

ExtensionsPage.vue 中显示 alist-encrypt 扩展卡片：

```typescript
{
  id: 'alist-decrypt',
  name: 'Alist-Encrypt 解密',
  description: '支持 AES-128-CTR 加密文件的解密、加密和流式预览',
  installed: boolean,     // 由后端 Plugins 列表判断
  enabled: boolean,
  sizeDisplay: '~150 KB', // 纯 Go 实现，无 native 库
}
```

COMBO_LITE_ID_MAP 增加 `'alist-decrypt' → 'com.encvgo.plugin.alistdecrypt'`（预留未来 ComboLite 扩展点）。

---

## ADDED Requirements（本次新增 — Mock 测试）

### Requirement: Go 后端 Plugin 层单元测试

为 `internal/v2/plugins/alistencrypt/` 编写 `_test.go`，覆盖：

| 测试组 | 覆盖内容 |
|--------|---------|
| TestPluginInitialization | Initialize() 成功/失败场景、配置校验（冲突后缀/格式错误/enc_type 无效） |
| TestCanDecrypt | 扩展名匹配/不匹配、AECTR2 magic 存在/不存在、ENCV 容器碰撞检测 |
| TestEncryptDecryptRoundtrip | 内存数据加密→解密往返一致性（V1/V2 两种模式） |
| TestEncryptWithV2Header | V2 头正确写入（magic+nonce+size）、解密时正确跳过 |
| TestStreamRange | ServeStream() Range 请求处理（206/200、边界 offset、超出范围） |
| TestSuffixValidation | 黑名单回退、格式修正、日志输出验证 |

**Mock 策略**：
- 使用 `bytes.Buffer` 替代真实文件系统进行 IO 测试
- 使用固定 seed 的 test password 保证确定性
- 不依赖外部文件或网络

### Requirement: Go 后端 API Handler 测试

为 `mobile_api.go` 中的两个新 handler 编写测试：

| 测试 | 覆盖内容 |
|------|---------|
| handleAlistDecodeFilenameGin | 正常解码、空参数、错误密码、特殊字符编码 |
| handleAlistEncryptStreamGin | 正常流返回、Range 请求、文件不存在、密码错误 |

**Mock 策略**：
- 使用 `httptest.NewRecorder` + `gin.CreateTestContext`
- 使用临时目录创建测试用的加密文件（通过 alistencrypt 包直接生成）
- 测试完成后清理临时文件

### Requirement: 前端 Vitest Mock 测试

项目已配置 Vitest（`vitest@^4.1.7`），但尚无测试文件。新增以下测试：

#### encv.alistencrypt.spec.ts — API 函数测试

```typescript
describe('getAlistEncryptStreamUrl', () => {
  it('should construct correct URL with path and password')
  it('should use DEV base URL in development mode')
  it('should encode path parameter correctly')
})

describe('decodeAlistFilename', () => {
  it('should call correct endpoint and return plain_name on success')
  it('should return empty plain_name on failure')
  it('should handle network errors gracefully')
})
```

**Mock 方式**：mock `Capacitor.Http.request` 返回预设响应。

#### Files.alistencrypt.spec.ts — 文件检测逻辑测试

```typescript
describe('isAlistEncrypted', () => {
  it('should detect .bin files as encrypted')
  it('should not detect .sccgv files as alist-encrypted (ENCV container)')
  it('should not detect .mp4 files as encrypted')
  it('should be case-insensitive for extension matching')
})
```

**Mock 方式**：直接导入函数测试纯逻辑，无需组件渲染。

#### useAlistEncrypt.spec.ts — Composable 测试

```typescript
describe('useAlistEncrypt', () => {
  it('should cache password after promptPassword succeeds')
  it('should return cached password on subsequent calls')
  it('should decode filename and cache result')
  it('should construct stream URL with cached password')
  it('should clear password cache appropriately')
})
```

**Mock 方式**：mock `Capacitor.Http.request` + 在 `mount()` 的 Vue 组件上下文中测试。

### Requirement: 测试覆盖率目标

| 层 | 目标覆盖率 | 工具 |
|----|-----------|------|
| `internal/alistencrypt/` | ≥ 90% | `go test -cover` （当前已完成基础） |
| `internal/v2/plugins/alistencrypt/` | ≥ 80% | `go test -cover` |
| `mobile_api.go` handlers | ≥ 80% | `go test -cover` |
| `encv.ts` alistencrypt 函数 | ≥ 90% | `vitest --coverage` |
| `useAlistEncrypt` composable | ≥ 85% | `vitest --coverage` |
| `Files.vue` isAlistEncrypted | 100% | `vitest --coverage` |

---

## MODIFIED Requirements

无（纯增量需求）。

## REMOVED Requirements

无。
