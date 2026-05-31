# 移除 __mock_data__ 幽灵代码，统一使用 /storage/emulated/0

## 问题

i18n 服务守卫消息已正确写明 `--dir /storage/emulated/0`，但以下代码仍引用 `__mock_data__`（幽灵代码）：

1. `scripts/generate-mock-files.ts:8` — `MOCK_ROOT` 默认值为 `__mock_data__`
2. `mock/index.ts:7` — `MOCK_DATA_ROOT` 硬编码 `__mock_data__`
3. `mock/handlers.ts:5` — `MOCK_DATA_DIR` 硬编码 `__mock_data__`
4. `__mock_data__/` 目录 — 整个目录是旧数据，应删除

## 修复步骤

### Step 1: 修改 `scripts/generate-mock-files.ts`

将第 8 行：
```ts
const MOCK_ROOT = path.resolve(process.cwd(), '__mock_data__')
```
改为：
```ts
const MOCK_ROOT = '/storage/emulated/0'
```

这样默认输出路径就是 `/storage/emulated/0`，`--dir` 参数仍可覆盖。

### Step 2: 修改 `mock/index.ts`

将第 7 行：
```ts
const MOCK_DATA_ROOT = path.resolve(__dirname, '../__mock_data__')
```
改为：
```ts
const MOCK_DATA_ROOT = '/storage/emulated/0'
```

### Step 3: 修改 `mock/handlers.ts`

将第 5 行：
```ts
const MOCK_DATA_DIR = path.resolve(__dirname, '../__mock_data__')
```
改为：
```ts
const MOCK_DATA_DIR = '/storage/emulated/0'
```

### Step 4: 删除 `__mock_data__/` 目录

```bash
rm -rf /workspace/app/encv-mobile/__mock_data__
```

### Step 5: 创建 `/storage/emulated/0` 目录并生成 mock 数据

```bash
sudo mkdir -p /storage/emulated/0
sudo chown $(whoami) /storage/emulated/0
cd /workspace/app/encv-mobile && npx tsx scripts/generate-mock-files.ts
```

### Step 6: 验证

- 确认 `/storage/emulated/0/01-plain-media/` 存在
- 确认 `__mock_data__` 目录已不存在
- 确认 Go 后端 `ENCV_DEV_PREVIEW=1` 启动后 `server.dir` 指向 `/storage/emulated/0`（由 mobile overlay 自动生效）
- 确认前端服务守卫不再拦截

## 影响范围

- `generate-mock-files.ts` — 默认输出路径变更
- `mock/index.ts` — Vite mock 插件数据源路径变更
- `mock/handlers.ts` — Mock API handler 数据源路径变更
- `__mock_data__/` — 删除旧数据目录
- 不修改 `config.user.json`（严禁修改）
- 不修改 Go 后端代码（mobile overlay 机制已正确）
