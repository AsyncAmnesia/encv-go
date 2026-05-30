# Alist-Encrypt 兼容层 Spec（ENCV Plugin 架构）

## Why

encv-go 已有完善的 **ENCV Plugin 体系**（`internal/v2/plugins/`），每个插件实现统一 `Plugin` 接口并通过 `registry.go` 统一管理。现在需要新增一个 **alist-encrypt 插件**，使系统能够解密/加密/流式预览/文件名解码 AES-128-CTR 格式的加密文件。

**架构决策：三层分离 + 前端 FileFeature 注册机制**

```
Layer 1: 算法层 (Go)          internal/alistencrypt/         ✅ 已完成
Layer 2: 插件层 (Go)          internal/v2/plugins/alistencrypt/ ✅ 已完成
Layer 3: UI 层 (Vue)          features/ + Files.vue(渲染器)    ⚠️ 需重构
```

**UI 层核心设计范式：FileFeature Registry（前端镜像 Go Plugin 机制）**

> **设计原则：Files.vue 永远不直接导入任何业务特性代码。它是一个纯渲染器，通过 FileFeature Registry 获取所有视觉装饰和操作入口。每个业务特性（alist-encrypt、ENCV 容器、OpenList 等）以独立 Feature Module 形式注册自己。**

---

## What Changes

### 新增/重构的模块结构

```
app/encv-mobile/src/
├── types/
│   └── file-feature.ts                    🆕 FileFeature 接口定义（前端扩展点契约）
│
├── composables/
│   ├── useFileFeatures.ts                 🆕 FileFeature 注册表 + 查询 API
│   └── useAlistEncrypt.ts                 🆕 alist-encrypt 业务 composable
│
├── features/                              🆕 特性模块目录（每个目录 = 一个 FileFeature）
│   ├── alist-encrypt/                     ← alist-encrypt 特性模块（自包含）
│   │   ├── index.ts                       ← 导出 createAlistEncryptFeature()
│   │   ├── useAlistEncrypt.ts             ← 密码管理 / 文件名解码 / API 调用
│   │   ├── actions.ts                     ← action 定义（解密/预览）
│   │   ├── badge.ts                       ← 徽章组件逻辑
│   │   ├── subtitle.ts                    ← 副标题（真实文件名）逻辑
│   │   └── password-dialog.ts            ← 密码输入弹窗逻辑
│   │
│   ├── encv-container/                     ← 未来：ENCV 容器特性模块
│   └── openlist/                          ← 未来：OpenList 特性模块
│
├── views/
│   └── Files.vue                           🔧 重构：移除所有内联 alist 代码，改为 useFileFeatures
│
└── __tests__/                             🆕 测试
    ├── file-feature.registry.spec.ts       ← useFileFeatures 测试
    ├── features.alist-encrypt.spec.ts      ← alist-encrypt feature 模块测试
    ├── encv.alistencrypt.spec.ts           ← API 函数 mock 测试
    └── useAlistEncrypt.spec.ts            ← composable 测试
```

### FileFeature 接口定义（前端扩展点契约）

```typescript
// types/file-feature.ts

export interface FileAction {
  id: string                              // 唯一标识，如 'alist-decrypt'
  text: () => string                      // i18n 文本函数（延迟求值支持语言切换）
  icon: any                               // Ionicons 图标
  color?: 'primary' | 'danger' | 'warning'
  visible: (file: FileItem) => boolean     // 是否对当前文件显示
  handler: (file: FileItem) => Promise<void>
}

export interface FileBadge {
  text: string
  color: string                            // CSS 变量或颜色值
  icon?: any
}

export interface FileSubtitle {
  text: string
  color?: string
}

/**
 * FileFeature — 前端文件特性扩展接口
 *
 * 设计原则（镜像 Go Plugin 接口）：
 * - 每个 FileFeature 是自包含的模块，包含自己的状态、逻辑、i18n、样式
 * - Files.vue 通过 useFileFeatures() 获取所有已注册的 Feature，不做任何业务判断
 * - 新增特性只需：1) 创建 features/xxx/ 目录 2) 实现 FileFeature 接口 3) 在 useFileFeatures 中注册
 * - Files.vue 零修改
 */
export interface FileFeature {
  id: string                              // 全局唯一 ID，如 'alist-encrypt'

  // === 检测 ===
  /** 此特性是否应对该文件激活 */
  isActive(file: FileItem): boolean

  // === 视觉装饰（Files.vue 列表项中渲染）===
  /** 文件列表项徽章（如 AE 加密标记）*/
  getBadge?(file: FileItem): FileBadge | null | Promise<FileBadge | null>

  /** 文件列表项副标题（如解码后的真实文件名）*/
  getSubtitle?(file: FileItem): FileSubtitle | null | Promise<FileSubtitle | null>

  // === 操作入口（长按菜单中的按钮）===
  /** 返回此特性为该文件提供的所有操作 */
  getFileActions?(file: FileItem): FileAction[] | Promise<FileAction[]>

  // === 生命周期 ===
  onActivate?(): void                      // 特性被启用时调用
  onDeactivate?(): void                   // 特性被禁用时调用
}
```

### useFileFeatures 注册表

```typescript
// composables/useFileFeatures.ts

const registry = new Map<string, FileFeature>()

export function registerFileFeature(feature: FileFeature): void {
  if (registry.has(feature.id)) {
    console.warn(`[FileFeature] Duplicate registration: ${feature.id}`)
    return
  }
  registry.set(feature.id, feature)
  feature.onActivate?.()
}

export function unregisterFileFeature(id: string): void {
  const f = registry.get(id)
  f?.onDeactivate?.()
  registry.delete(id)
}

export function useFileFeatures() {
  const allFeatures = computed(() => Array.from(registry.values()))

  // 查询 API：给 Files.vue 使用
  function getBadges(file: FileItem): Promise<FileBadge[]> { ... }
  function getSubtitles(file: FileItem): Promise<FileSubtitle[]> { ... }
  function getAllActions(file: FileItem): Promise<FileAction[]> { ... }

  return { allFeatures, registerFileFeature, unregisterFileFeature, getBadges, getSubtitles, getAllActions }
}
```

### Files.vue 重构后的形态

```vue
<!-- Files.vue 关键变化 -->

<script setup>
// 之前：直接 import alistencrypt 逻辑
import { isAlistEncrypted, handleAlistDecrypt, handleAlistStreamPreview } from './useAlistEncrypt'

// 之后：仅使用通用注册表
import { useFileFeatures } from '@/composables/useFileFeatures'
const { getBadges, getSubtitles, getAllActions } = useFileFeatures()

// handleLongPress 中：
// 之前：if (isAlistEncrypted(file)) { buttons.push({text:'解密', handler:...}) }
// 之后：
const featureActions = await getAllActions(file)
buttons.push(...featureActions.map(a => ({ text: a.text(), icon: a.icon, handler: () => a.handler(file) })))
</script>

<template>
  <!-- 之前：<span v-if="isAlistEncrypted(file)" class="ae-badge">AE</span> -->
  <!-- 之后： -->
  <ion-badge v-for="badge in fileBadges[file.path]" :key="badge.text"
             :color="badge.color" class="file-feature-badge">
    {{ badge.text }}
  </ion-badge>

  <!-- 之前：<p v-if="decodedNames[file.path]" class="real-name">...</p> -->
  <!-- 之后： -->
  <p v-for="sub in fileSubtitles[file.path]" :key="sub.text"
     class="file-feature-subtitle" :style="{color: sub.color}">
    {{ sub.text }}
  </p>
</template>
```

---

## ADDED Requirements

### Requirement: FileFeature 接口与注册机制（UI 隔离骨架）

系统 SHALL 提供 `FileFeature` 接口和 `useFileFeatures()` 注册表，作为前端文件操作的可扩展机制。

#### 核心契约

1. **Files.vue 不导入任何业务特性代码** — 仅通过 `useFileFeatures()` 查询
2. **每个业务特性是一个独立的 `features/xxx/` 目录** — 自包含状态/i18n/样式/API
3. **注册时机**：应用启动时在 main.ts 或 App.vue 中调用 `registerFileFeature()`
4. **条件注册**：可根据后端配置/插件安装状态决定是否注册某个 Feature

#### Scenario: 新增一个文件特性无需改 Files.vue
- **WHEN** 开发者需要新增「OpenList 直链」功能
- **THEN** 创建 `features/openlist/` 目录 → 实现 `FileFeature` 接口 → 在 main.ts 中 `registerFileFeature(createOpenlistFeature())`
- **AND** Files.vue 自动获得新的徽章/副标题/菜单项，**零修改**

#### Scenario: 禁用某个特性
- **WHEN** 用户在设置页禁用了 alist-encrypt 功能
- **THEN** `unregisterFileFeature('alist-encrypt')` → 所有相关徽章/副标题/菜单项从 Files.vue 消失

### Requirement: AlistEncrypt FileFeature 实现

`features/alist-encrypt/` 模块 SHALL 实现完整的 FileFeature 接口：

| 方法 | 实现 |
|------|------|
| `id` | `'alist-encrypt'` |
| `isActive(file)` | `filepath.Ext(file.name).toLowerCase() === '.bin'` （或从配置读取 suffix）|
| `getBadge(file)` | `{ text: 'AE', color: 'var(--ion-color-danger)' }` |
| `getSubtitle(file)` | 异步调用 decode-filename API → `{ text: plainName, color: '...' }`；缓存结果避免重复请求 |
| `getFileActions(file)` | 返回 `[streamPreviewAction, decryptAction]` |

#### 密码输入流程（通过 Feature 内部管理）

```
用户点击「解密」action
  → action.handler(file)
    → useAlistEncrypt.promptPassword(file)
      → IonAlert 弹窗（password input + 记住会话 toggle）
        → 用户确认 → 密码缓存到 sessionPasswords map
        → 调用 POST /api/tasks 创建解密任务
          → 跳转 Tasks.vue 展示进度
```

密码仅在内存中缓存（session 级别），不持久化。

### Requirement: 密码输入与管理

详见 Phase 4 原始需求，但实现位置在 `features/alist-encrypt/password-dialog.ts` 和 `features/alist-encrypt/useAlistEncrypt.ts`。

### Requirement: 操作状态反馈与错误展示

详见 Phase 4 原始需求，错误分类展示遵循 `frontend-design.md` 规范。

### Requirement: 设置页集成

设置页通过后端 `/api/config/schema` 动态渲染 alist_encrypt 配置区域（enabled/suffix/default_password/enc_type）。
配置变更时：
- suffix 变更 → 通知 `features/alist-encrypt/` 更新 isActive 的匹配规则
- enabled=false → `unregisterFileFeature('alist-encrypt')`
- enabled=true → `registerFileFeature(createAlistEncryptFeature())`

### Requirement: ExtensionsPage.vue 集成

ExtensionsPage 显示 alist-decrypt 卡片，COMBO_LITE_ID_MAP 增加 mapping。

---

## ADDED Requirements（本次新增 — 性能约束与边界异常处理）

### Requirement: 文件列表渲染性能保证

FileFeature 查询 SHALL NOT 阻塞文件列表的首次渲染（Time to First Paint）。

#### 性能约束

| 指标 | 目标值 | 说明 |
|------|--------|------|
| `isActive(file)` 调用耗时 | **< 0.01ms** / 文件 | 纯字符串操作，禁止 I/O |
| `getBadge(file)` 调用耗时 | **< 0.05ms** / 文件 | 同步返回，禁止异步 |
| `getSubtitle(file)` 首次调用 | **异步，不阻塞渲染** | 返回 null 占位，后台加载后 reactive 更新 |
| 1000 个文件列表初始化 | **< 100ms** | 所有 Feature 的 isActive + getBadge 累计 |
| 文件名解码批量请求 | **合并为单次 API 调用** | 批量 decode-filename endpoint 或防抖 300ms |

#### 实现策略

1. **getBadge 必须同步返回** — 徽章是同步视觉元素，不允许 await
2. **getSubtitle 允许返回 Promise** — Files.vue 先渲染空副标题位置，数据到达后填充（Vue reactive 自动更新 DOM）
3. **批量解码** — `features/alist-encrypt/subtitle.ts` 实现请求合并：在 300ms 窗口内收集所有需要解码的文件路径 → 单次 POST 批量 API → 分发结果到各 file 的缓存
4. **LRU 缓存上限** — decodedNames 和 sessionPasswords Map 设置 **最大容量 500 条**，超出时淘汰最旧条目（防止内存泄漏）
5. **虚拟列表兼容** — 如果 Files.vue 使用 ion-virtual-scroll，getBadge/getSubtitle 仅对可见行调用

#### Scenario: 大目录性能
- **WHEN** 用户进入包含 2000 个 .bin 文件的目录
- **THEN** 文件列表在 < 200ms 内显示（带 AE 徽章），真实文件名在后续 1-2s 内逐步填充

### Requirement: 流式预览内存安全

Go 后端 stream endpoint SHALL 对超大文件有内存上界保护。

#### 内存约束

| 指标 | 约束 |
|------|------|
| 单次 Range 读取缓冲区 | **≤ 1 MB**（使用 io.LimitedReader 或 bufio.ReaderSize） |
| 并发 stream 连接数 | **≤ 3**（超过返回 429 Too Many Requests） |
| 总内存占用（stream） | **≤ 5 MB**（3 连接 × 1MB buffer） |
| 超大文件支持 | **≥ 4 GB**（纯流式，不全量加载到内存） |

#### 边界异常处理

| 异常场景 | 处理方式 |
|---------|---------|
| Range start > fileSize | 返回 **416 Range Not Satisfiable**，Content-Range: bytes */fileSize |
| Range end > fileSize | 截断到 fileSize-1，返回实际可用范围 |
| 负数 Range | 返回 **400 Bad Request** |
| 文件在读取过程中被删除 | 返回 **500 Internal Server Error**，日志记录 ENOENT |
| 密码错误（首次读取时才发现） | 中断流，返回 **400 Bad Request** + 错误 JSON `{error:"password_mismatch"}` |
| 客户端断开连接 | goroutine cleanup < 1s，不泄漏 |

### Requirement: 解密/加密任务异常边界

#### 文件系统异常

| 场景 | 行为 |
|------|------|
| 源文件不存在 | TaskManager 记录 failed + error=`"source file not found: ..."` |
| 目标目录不存在 | 自动创建（mkdir -p）；创建失败 → failed + error=`"cannot create output directory"` |
| 磁盘空间不足 | 在写入前检查可用空间；不足 → failed + error=`"insufficient disk space: need X MB, available Y MB"` |
| 源文件权限不足 | failed + error=`"permission denied reading source file"` |
| 目标路径已存在同名文件 | **覆盖确认**（TaskManager 层或前端二次确认）；MVP 阶段直接覆盖并记录 warn 日志 |

#### 数据完整性异常

| 场景 | 检测时机 | 行为 |
|------|---------|------|
| 加密文件大小 = 0 字节 | Decrypt() 入口 | 返回 ErrInvalidFormat `"empty encrypted file"` |
| 加密文件 < 32 bytes (V2) 或 < 16 bytes (V1) | CanDecrypt() 或 Decrypt() 入口 | 返回 ErrInvalidFormat `"file too small to be valid"` |
| 解密后数据与预期 PlainSize 不匹配 (V2) | Decrypt() 读取完成后 | 返回 DecryptionError `"decrypted size mismatch: expected X, got Y"` |
| 密码错误导致解密出大量非 UTF-8/非二进制垃圾数据 | Decrypt() 前 1KB 采样检查 | 返回 ErrInvalidPassword（启发式检测：如果解密后前 1KB 全部是不可打印字节且无已知 magic，高概率密码错误） |
| V2 头中 PlainSize = 0 | Decrypt() 入口 | 返回 ErrInvalidFormat `"V2 header declares zero plain size"` |

#### 并发安全

| 场景 | 保证 |
|------|------|
| 同一文件同时被解密和流式预览 | ✅ 允许（DecryptReader 无状态，每次创建新实例） |
| 同时注册/注销 FileFeature | ✅ RWMutex 保护 registry Map（useFileFeatures 内部） |
| sessionPasswords 并发读写 | ✅ 使用 Map 原子操作或 shallow ref 封装 |
| Go Cipher.SetPosition 并发调用 | ⚠️ **未定义行为** — CTR cipher 的 SetPosition 不是线程安全的。调用方应确保同一 Cipher 实例不被并发调用。Stream endpoint 每个 connection 创建独立 Cipher 实例（天然隔离）。 |

### Requirement: 前端网络异常恢复

| 场景 | 用户可见行为 | 技术实现 |
|------|------------|---------|
| decode-filename API 超时 (>5s) | 副标题显示「...」+ 重试按钮 | Promise catch → 显示重试 UI → 用户点击重新调用 |
| decode-filename API 返回 500 | 同上 | 同上 |
| stream URL 播放中断（网络波动） | MPV/ArtPlayer 自身重试机制 | HTTP handler 幂等性保证（Range 请求可重复） |
| 提交解密任务时网络断开 | toast 提示「网络错误，请检查连接」+ 保留输入内容 | Capacitor Http.request catch → showToast 不 clear 表单 |

### Requirement: Feature 注册状态一致性

#### 边界场景

| 场景 | 行为 |
|------|------|
| registerFileFeature 在 getBadges 执行期间被调用 | 当前批次查询不受影响，下一批次包含新 Feature |
| unregisterFileFeature 导致正在显示的 badge 消失 | Vue reactive 自动移除 DOM，无闪烁（badge v-for key 绑定 feature.id） |
| 同一 id 重复注册 | console.warn + 忽略（不替换已有实例） |
| 注销一个从未注册的 id | console.warn + 安全返回（不抛异常） |

---

## Mock 测试需求（按测试层级组织）

### Layer 1: FileFeature 架构测试

| 测试 | 验证内容 |
|------|---------|
| registerFileFeature | 注册后可在 allFeatures 中查到 |
| unregisterFileFeature | 注销后从 allFeatures 消失，onDeactivate 被调用 |
| duplicate registration | 重复注册不覆盖，console.warn |
| getBadges / getSubtitles / getAllActions | 正确聚合多个 Feature 的返回值 |
| empty registry | 无 Feature 注册时返回空数组 |

### Layer 2: AlistEncrypt Feature 模块测试

| 测试 | 验证内容 |
|------|---------|
| createAlistEncryptFeature.isActive | .bin→true, .mp4→false, .sccgv→false, 大小写不敏感 |
| createAlistEncryptFeature.getBadge | 返回 {text:'AE', color:danger} |
| createAlistEncryptFeature.getSubtitle | mock API → 返回解码名称；API 失败 → 返回 null |
| createAlistEncryptFeature.getFileActions | 返回 2 个 action（stream+decrypt）；非 .bin 文件返回空数组 |
| promptPassword | 成功→缓存密码；取消→null；二次调用→跳过弹窗用缓存 |
| getStreamUrl | 有缓存密码→构造正确 URL；无密码→URL 无 password 参数 |

### Layer 3: API 函数 mock 测试

| 测试 | 验证内容 |
|------|---------|
| getAlistEncryptStreamUrl | URL 编码正确；DEV base URL |
| decodeAlistFilename | 成功/失败/网络错误 |

### Layer 4: Go 后端补充测试

| 测试 | 验证内容 |
|------|---------|
| plugin_test.go | Initialize/CanDecrypt/加解密往返/V2头/StreamRange/后缀校验 |
| mobile_api handler test | decodeFilename/stream 正常/异常/边界 |

### 覆盖率目标

| 层 | 目标 |
|----|------|
| FileFeature 架构 (useFileFeatures) | ≥ 95% |
| AlistEncrypt Feature 模块 | ≥ 90% |
| encv.ts API 函数 | ≥ 90% |
| Go Plugin 层 | ≥ 80% |
| Go Handler 层 | ≥ 80% |

---

## MODIFIED Requirements

无（纯增量）。

## REMOVED Requirements

无。
