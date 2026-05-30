# Plan: Alist-Encrypt 后缀冲突检测与插件扩展名唯一性保障

## 问题分析

### 当前状态

| 层级 | 现状 | 问题 |
|------|------|------|
| **后端 registry.go** | `initializeExtensions()` 用 `map[ext]=true` 收集所有插件容器扩展名 | **无冲突检测**，后注册的静默覆盖先注册的 |
| **后端 alistencrypt/plugin.go:75** | `reservedSuffixes = {".sccgv": ".encv"}` | 只硬编码了 2 个值，**未包含其他 5 个插件的 ext**（`.sccga/.sccgi/.sccgpdf/.sccgt/.sccgwps`） |
| **前端 Settings.vue（已删除）** | 原 `CONFLICT_SUFFIXES = ['.sccgv', '.encv']` | 同样不完整，且随重构被移除 |
| **前端 PluginSettings.vue** | 无任何校验逻辑 | 用户可随意输入冲突后缀并保存成功 |
| **config.user.json 实际数据** | `alist_encrypt.suffix = ".sccgv"` **与 video.ext 冲突！** | 已存在真实冲突 |

### 核心区分（用户明确要求）

```
源文件扩展名 → 允许冲突
  例：.ts 可能是文本文件（TypeScript代码）也可能是视频文件
  → ENC V 通过 MIME 检测 + 插件优先级链自动分辨
  → 这是插件系统的能力，不应阻断

插件容器扩展名 → 禁止冲突
  例：video 用 .sccgv 做加密容器后缀，alist_encrypt 也用 .sccgv
  → IsContainer() 无法区分到底该由哪个插件解密
  → 必须在配置层阻断
```

## 实现方案

### Step 1: 后端 — 新增扩展名冲突检测 API

**文件**: `internal/server/mobile_api.go`

新增接口：
```
GET /api/plugins/container-extensions
Response: {
  "extensions": {
    "video": ".sccgv",
    "audio": ".sccga",
    "image": ".sccgi",
    "wps": ".sccgwps",
    "pdf": ".sccgpdf",
    "text": ".sccgt",
    "alist_encrypt": ".bin"
  },
  "conflicts": []  // 如果有冲突，列出冲突的 plugin name 列表
}
```

实现方式：遍历 `Plugins` 列表，调用每个插件的 `GetContainerExtension()`，构建 map 时检测重复。

### Step 2: 前端 — 新增 usePluginExtensions composable

**文件**: `src/composables/usePluginExtensions.ts`（新建）

职责：
- 调用 `GET /api/plugins/container-extensions` 获取所有插件容器扩展名
- 提供 `getConflictingPlugins(suffix: string): string[]` 方法
- 缓存结果，避免重复请求

### Step 3: 前端 PluginSettings.vue — 后缀输入实时校验

**文件**: `src/views/PluginSettings.vue`

在渲染 `plugin_settings.alist_encrypt.suffix` 的 `<ion-input>` 旁添加：

1. **实时冲突检测**：`@ionInput` 触发时调用 `getConflictingPlugins(newValue)`
2. **冲突警告 UI**：如果返回非空数组，显示红色提示条：
   > ⚠️ 后缀 `.xxx` 与以下插件的容器扩展名冲突：video、audio
   > 加密容器扩展名必须唯一，请修改后缀。
3. **保存阻断**：如果存在冲突，禁用保存按钮或保存时 reject 并提示

### Step 4: 前端 schema.json — 标记 suffix 字段需要交叉校验

**文件**: `src/config/schema.json`

在 `alist_encrypt.suffix` 字段添加自定义属性（供前端读取）：
```json
"suffix": {
  "type": "string",
  "default": ".bin",
  "description": "...",
  "x-cross-validate": "container-extension-unique"
}
```

### Step 5: 清理 — 移除后端 alistencrypt 硬编码的 reservedSuffixes

**文件**: `internal/v2/plugins/alistencrypt/plugin.go`

删除 L75 的 `reservedSuffixes` 硬编码。冲突检测统一交给 Step 1 的 API + 前端校验。
保留 Initialize() 中的格式校验（以 `.` 开头、长度限制），但将冲突检测改为 warning 日志而非静默回退。

## 不做的事

- ❌ 不修改 `registry.go` 的 `initializeExtensions()` 核心逻辑（保持简单 O(1) 查找）
- ❌ 不在后端 `updateConfig` API 中加拦截（前端阻断足够，后端只负责暴露数据）
- ❌ 不影响源文件扩展名的匹配逻辑（MIME → 扩展名 → ShouldProcess 链路不变）

## 验证方式

1. 设置 `plugin_settings.alist_encrypt.suffix = .sccgv` → 应显示与 video 冲突
2. 设置 `plugin_settings.alist_encrypt.suffix = .sccga` → 应显示与 audio 冲突
3. 设置 `plugin_settings.alist_encrypt.suffix = .myenc` → 无冲突，正常保存
4. `vue-tsc --noEmit` 零错误
5. 浏览器预览页面正常显示冲突提示
