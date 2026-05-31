# 修复：移动端预览时 `server.dir` 返回桌面端根路径

## 问题分析

### 当前数据流

```
前端 GET /api/config (经 Vite :5173)
  → mock handlers.ts 拦截
    → http.proxy 到后端 :2025
      → [server_config_api.go:28](internal/server/server_config_api.go#L28) handleGetConfigGin
        → os.ReadFile(config.user.json)  ← ⚠️ 读的是原始文件！不是 Load() 合并后的内存配置
        → 返回 { server: { dir: "/" }, mobile: { server_dir: "/storage/emulated/0" }, ... }
      ← 回到 mock handler
        → 只重写 mobile.server_dir → __mock_data__
        → 返回给前端: { server: { dir: "/" }, mobile: { server_dir: "__mock_data__" } }
```

### 根因

两个问题叠加：

1. **后端 `/api/config` 读取原始文件而非内存配置**
   - [handleGetConfigGin()](internal/server/server_config_api.go#L28) 用 `os.ReadFile(s.configPath)` 读取 `config.user.json`
   - 不经过 `Load()` 的合并逻辑，也不包含 `finalize()` 后处理
   - 所以 `server.dir` 是原始值 `"\""`（或 `"/workspace"` 经过 `finalize` 规整后）

2. **mock handler 只重写了 `mobile.server_dir`**
   - [handlers.ts:60](app/encv-mobile/mock/handlers.ts#L60) 只检查并重写 `mobile.server_dir`
   - `server.dir` 未被重写，前端拿到桌面端根路径

### 影响

前端 `useConfig` 拿到的 `server.dir = "/"`，导致：
- Settings 页面显示错误的根目录
- 文件路径构建可能基于错误的基础路径
- 与实际后端服务路径不一致

---

## 修复方案

### 核心原则（来自之前的约束）

1. ✅ **不 mock API 数据** — 所有数据来自后端真实响应
2. ✅ **不修改后端桌面端配置** — Go 后端 `server.dir` 保持桌面端路径
3. ✅ **config.dev.json 保持极简** — 仅 `mobile.server_dir`
4. ✅ **只做路径适配层** — 在 proxy 返回时重写字段值

### 修改点：mock/handlers.ts

在 `proxyAndRewriteMobileServerDir` 中，**同时重写 `server.dir`**：

```typescript
// 当前（只重写 mobile）
if (cfg.mobile && typeof cfg.mobile.server_dir === 'string') {
  cfg.mobile.server_dir = targetDir
}

// 修复后（同时重写 server.dir 和 mobile.server_dir）
if (cfg.mobile && typeof cfg.mobile.server_dir === 'string') {
  cfg.mobile.server_dir = targetDir
}
if (cfg.server && typeof cfg.server.dir === 'string') {
  cfg.server.dir = targetDir
}
```

### 修复后的完整数据流

```
前端 GET /api/config (经 Vite :5173)
  → mock handlers.ts 拦截（路径适配层，非数据 mock）
    → http.proxy 到后端 :2025
      → handleGetConfigGin 读取 config.user.json 原始文件
        → 返回 { server: { dir: "/", port: 2025, ... },
                  mobile: { server_dir: "/storage/emulated/0", ... },
                  ... }
      ← 回到 mock handler
        → 重写 server.dir → "__mock_data__/..."   ← 新增
        → 重写 mobile.server_dir → "__mock_data__/..."
        → 返回给前端: { server: { dir: "__mock_data__/...", port: 2025, ... },
                         mobile: { server_dir: "__mock_data__/...", ... } }
```

### 字段对照表

| 字段 | 后端原始值 | 前端收到值 | 说明 |
|------|-----------|-----------|------|
| `server.dir` | `"/"` (桌面端) | `"__mock_data__/..."` | **新增重写** |
| `server.port` | `2025` | `2025` | 不变 |
| `mobile.server_dir` | `"/storage/emulated/0"` | `"__mock_data__/..."` | 已有重写 |
| 其余所有字段 | 来自 config.user.json | 来自 config.user.json | 不变 |

---

## 实施步骤

### Step 1: 修改 [handlers.ts](app/encv-mobile/mock/handlers.ts)

在 `proxyAndRewriteMobileServerDir` 函数中，`JSON.parse(body)` 之后增加 `server.dir` 重写：

```typescript
const cfg = JSON.parse(body)
if (cfg.server && typeof cfg.server.dir === 'string') {
  cfg.server.dir = MOCK_DATA_DIR
}
if (cfg.mobile && typeof cfg.mobile.server_dir === 'string') {
  cfg.mobile.server_dir = targetDir  // 已有逻辑
}
```

注意：`targetDir` 变量从 `config.dev.json` 的 `mobile.server_dir` 读取，fallback 到 `MOCK_DATA_DIR`。
对于 `server.dir`，直接使用 `MOCK_DATA_DIR` 即可（不需要从 dev 配置读取）。

### Step 2: 验证

```bash
# 通过 Vite 端口验证（应看到两个路径都是 __mock_data__）
curl -s http://localhost:5173/api/config | python3 -c "
import sys,json; d=json.load(sys.stdin)
print('server.dir:    ', d.get('server',{}).get('dir'))
print('mobile.srv_dir:', d.get('mobile',{}).get('server_dir'))
"

# 通过后端端口验证（应保持原样，不受影响）
curl -s http://localhost:2025/api/config | python3 -c "
import sys,json; d=json.load(sys.stdin)
print('server.dir:    ', d.get('server',{}).get('dir'))
print('mobile.srv_dir:', d.get('mobile',{}).get('server_dir'))
"
```

预期结果：
- Vite 端口：`server.dir = __mock_data__/...`, `mobile.server_dir = __mock_data__/...`
- 后端端口：`server.dir = /` (或 `/workspace`), `mobile.server_dir = /storage/emulated/0`

---

## 变更清单

| 文件 | 操作 | 改动量 |
|------|------|--------|
| [mock/handlers.ts](app/encv-mobile/mock/handlers.ts) | 修改 `proxyAndRewriteMobileServerDir` | +3 行 |
