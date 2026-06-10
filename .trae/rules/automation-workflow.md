# 自动化测试工作流铁律（动态构建 / 状态同步 / 本地持久化）

> **核心原则：测试工作流不能硬编码 cipherMode / compressionMode / sourcePath —— 必须从 plugin.taskOptions.extraFields + plugin.supportedExtensions 派生。**
> **WS 状态变化必须全链路同步（task:update / task:progress / task:created / task:completed 4 件套全监）。**
> **测试结果必须持久化到 localStorage（刷新页面 / 关 App 不丢失）。**

---

## 一、5 个历史 bug（2026-06-10 同日连修）

| # | 症状 | 根因 | 修复 |
|---|------|------|------|
| **#1** | 测试报告状态不刷新（running 期间 progress=0%） | [useWorkflowEngine.ts](file:///workspace/app/encv-mobile/src/composables/useWorkflowEngine.ts) `startListening()` **只监听 `task:completed`** | 加 `task:update` / `task:progress` / `task:created` 监听，引入 `findStepByTaskId()` 状态机升级 |
| **#2** | 测试结果刷新页面丢失 | [useAutomationTests.ts](file:///workspace/app/encv-mobile/src/composables/useAutomationTests.ts) results 只在内存 | 新增 `persistCurrentRun()` + `getPersistedRuns()` + `clearPersistedRuns()`，localStorage key `encv_automation_results_v1`，最多 50 次 run |
| **#3** | 任务组 group card 简陋、跟普通 task 区分度低 | [Tasks.vue](file:///workspace/app/encv-mobile/src/views/Tasks.vue) group card 是简单 ion-item | 加 tone (`automation` / `ai_agent`)、icon-bubble、左侧 4px 渐变 border、group-progress-track、% 文本 + checkmark icon |
| **#4** | 所有 plugin 的加解密任务都走 sample.mp4 | `buildDynamicWorkflow()` **写死** `sourcePath: DEFAULT_AUTOMATION_SOURCE` | 加 `categoryForExt()` ext→目录分类映射，按 `plugin.supportedExtensions[0]` 选源 |
| **#5** | 遍历加密选项承诺没生效 | `buildDynamicWorkflow()` 硬编码 `cipherMode=[0,1] / compressionMode=['none','zstd']`（v4 only） | 改为遍历 `plugin.taskOptions.extraFields`：type=select 笛卡尔积、type=bool 2^N，**删 v4 硬编码** |

## 二、test 报告状态同步 — 4 件套监听铁律

```ts
// 缺一不可（修复前只监 task:completed）
function startListening() {
  eventBus.on('task:completed', onTaskCompleted)
  eventBus.on('task:update', onTaskUpdate)        // 🆕 状态机升级 pending→queued→running
  eventBus.on('task:progress', onTaskProgress)    // 🆕 进度% / phase / speed / eta
  eventBus.on('task:created', onTaskCreated)      // 🆕 确认后端已收
}
```

**后端推送的 task 事件**（[internal/service/task_manager.go](file:///workspace/internal/service/task_manager.go)）：

| 事件 | payload | 触发时机 |
|------|---------|---------|
| `task:created` | `{id, type, sourcePath}` | submit 后立即 |
| `task:update` | `{id, status, type, progress}` | status 变化（queued/running/cancelling/cancelled） |
| `task:progress` | `{id, progress, phase, speed, eta}` | 进度推（每 N% / 每 Ns） |
| `task:completed` | `{id, error?}` | 终态 |

**前端的 useWebSocket 透传**：message.type → eventBus.emit(type, data)。所以**任意前端的 useTaskEventBridge / useWorkflowEngine / useAutomationTests 都必须全订阅 4 件套**。

## 三、动态工作流构建 — 消除硬编码

### 旧实现（❌ 硬编码 v4 cipher + 写死 sample.mp4）

```ts
for (const plugin of plugins.value) {
  for (const taskType of ['encrypt', 'decrypt'] as const) {
    for (const version of versions) {
      const isV4 = version === 4
      const cipherModes = isV4 && taskType === 'encrypt' ? [0, 1] : [undefined]  // ❌ 硬编码
      const compressionModes = isV4 && taskType === 'encrypt' ? ['none', 'zstd'] : [undefined]  // ❌ 硬编码
      steps.push({
        action: { params: { sourcePath: DEFAULT_AUTOMATION_SOURCE, ... } }  // ❌ 一刀切
      })
    }
  }
}
```

### 新实现（✅ 动态遍历）

```ts
// 1. 按 plugin.supportedExtensions 选源
const sourceExt = plugin.supportedExtensions[0]
const sourcePath = `${mockRoot.value}01-plain-media/${categoryForExt(sourceExt)}/sample.${sourceExt}`

// 2. 按 plugin.taskOptions.extraFields 派生笛卡尔积
const selectFields: { field: any; values: string[] }[] = []
const boolFields: { field: any }[] = []
for (const f of opts.extraFields ?? []) {
  if (f.type === 'select' && Array.isArray(f.options) && f.options.length > 1) {
    selectFields.push({ field: f, values: f.options })
  } else if (f.type === 'bool') {
    boolFields.push({ field: f })
  }
}

// 3. 笛卡尔积展开
for (const taskType of ['encrypt', 'decrypt'] as const) {
  for (const version of versions) {
    // 按 taskType 过滤 ExtraFields（Condition='encrypt' 字段只 encrypt 用）
    const taskSelectFields = selectFields.filter((sf) => !sf.field.condition || sf.field.condition === taskType)
    const taskBoolFields = boolFields.filter((bf) => !bf.field.condition || bf.field.condition === taskType)
    const selectCombos = cartesianExpand(taskSelectFields.map((sf) => sf.values))
    const boolCombos: boolean[][] = (taskBoolFields.length === 0)
      ? [[]]
      : Array.from({ length: 1 << taskBoolFields.length }, (_, mask) =>
          Array.from({ length: taskBoolFields.length }, (_, i) => Boolean(mask & (1 << i))))

    for (const selectCombo of selectCombos) {
      for (const boolCombo of boolCombos) {
        const extraFields: Record<string, string> = {}
        taskSelectFields.forEach((sf, i) => { extraFields[sf.field.key] = selectCombo[i] })
        taskBoolFields.forEach((bf, i) => { extraFields[bf.field.key] = boolCombo[i] ? 'true' : 'false' })

        steps.push({
          action: {
            type: 'encv_task',
            taskType,
            pluginName: plugin.name,
            params: { sourcePath, password, version, extraFields },
          },
        })
      }
    }
  }
}
```

## 四、ext → 目录分类映射（避免一刀切 sample.mp4）

| ext | category | sample 文件 |
|-----|----------|------------|
| mp4 / mkv / avi / mov / webm / flv / wmv | `video` | `sample.mp4` / `comedy.mkv` |
| mp3 / flac / ogg / m4a / wav / aac / opus | `audio` | `sample.mp3` / `sample.flac` |
| png / jpg / jpeg / gif / webp / bmp / tiff | `image` | `sample.png` / `sample.jpg` |
| pdf | `pdf` | `sample.pdf` |
| doc / docx / xls / xlsx / ppt / pptx | `wps` | `sample.docx` |
| txt / md / rtf / log | `text` | `sample.txt` |
| encv / ae | `alist-encrypted` | `sample.encv` |
| 其他 | `misc` | （需要时再补） |

**策略**：每个 plugin 取 `supportedExtensions[0]`（避免笛卡尔积爆炸）。如果未来要遍历所有 ext，把 `const sourceExt = supportedExts[0]` 改为 `for (const sourceExt of supportedExts)` 即可。

## 五、StepRun 字段扩展

`src/lib/workflow/types.ts` StepRun 加：

```ts
export interface StepRun {
  // ...原有
  progress?: number   // 🆕 由 task:update / task:progress 驱动
  phase?: string      // 🆕 task phase label
  speed?: string      // 🆕 速率（"12.5 MB/s"）
  eta?: string        // 🆕 剩余时间
}
```

`StepStatus` 加 `'cancelling'`（区分取消中 vs 已取消），`VALID_TRANSITIONS` 同步更新：
```ts
running: new Set(['cancelling', 'success', 'failure', 'cancelled', 'timed_out']),
cancelling: new Set(['cancelled', 'failure', 'success']),
```

`EncvTaskActionParams` 加 `extraFields?: Record<string, string>`，并把 `useWorkflowEngine.submitAction` 第 7 个参数从 `{}` 改为 `spec.params.extraFields ?? {}`。

## 六、本地持久化规范

**localStorage key**：`encv_automation_results_v1`（带版本号，方便未来 schema 迁移）

**数据格式**：
```ts
interface PersistedRun {
  id: string
  startedAt: string
  completedAt?: string
  totalCases: number
  passed: number
  failed: number
  skipped: number
  results: TestCaseResult[]
}
```

**裁剪策略**：按 `startedAt` 倒序，最多保留 50 次。防止 localStorage 撑爆（每个 run 几十 KB，50 个 ≈ 几 MB）。

**触发时机**：`runTests()` 所有 case 提交完后立即 `persistCurrentRun()`（不等 WS 回调，保证提交阶段的结果不丢）。

**清空**：`clearPersistedRuns()` 仅清测试历史，**不动** workflow definition / runs / triggeredBy 标记。

## 七、任务组 group card UI 规范

| 元素 | 样式 | 用意 |
|------|------|------|
| 左侧 border | 4px 渐变（automation 蓝 / ai_agent 紫） | 一眼区分触发器 |
| icon-bubble | 40×40 圆形填充（automation primary / ai_agent secondary） | 比纯 ion-icon 大气 |
| title | `自动化测试 · 12 个任务` / `AI agent · 8 个任务` | 中文 trigger 名 + N |
| 徽章 | ✓ N / ✗ N / ▶ N（spinner）/ N | running 用 spinner 图标（动态感） |
| 进度条 | 6px 高度，渐变 fill | 跟 task 卡片对齐 |
| % 文本 | 右上角 monospace 数字 | 一眼读完成度 |
| chevron | 折叠=chevronForward，展开=chevronBack | 直观 |
| **tone='ai_agent'** | 紫色 secondary 色 | 区分 automation（蓝） |

**type 字段**（`DisplayItem` union）：
```ts
{ kind: 'group'; key: string; groupKey: string; tone: 'automation' | 'ai_agent'; tasks: EncvTask[]; summary: { passed; failed; running; pending; percent; latestCreatedAt } }
{ kind: 'task'; key: string; task: EncvTask }
```

## 八、测试用例数估算（防爆）

| plugin 数 | supportedExts 策略 | taskType | version | select 笛卡尔积 | bool 2^N | 总数 |
|----------|-------------------|---------|---------|---------------|---------|------|
| 7 | 1 ext each | encrypt+decrypt | v2+v3+v4 | 0~3 fields × 2~4 options | 0~3 bool | **典型 200-500** |
| 7 | 全部 ext 展开 | encrypt+decrypt | v2+v3+v4 | ... | ... | **典型 2000+** |

**当前策略**（1 ext per plugin）= 200-500 case，并行 `max: 5`，后端可承载。

## 九、跨层参考

| 主题 | 文档位置 |
|------|---------|
| 工作流引擎 | [useWorkflowEngine.ts](file:///workspace/app/encv-mobile/src/composables/useWorkflowEngine.ts) |
| 工作流存储 | [useWorkflowStore.ts](file:///workspace/app/encv-mobile/src/composables/useWorkflowStore.ts) |
| 自动化测试入口 | [useAutomationTests.ts](file:///workspace/app/encv-mobile/src/composables/useAutomationTests.ts) |
| 自动化测试 UI | [AutomationTestsDetail.vue](file:///workspace/app/encv-mobile/src/views/AutomationTestsDetail.vue) |
| 类型定义 | [workflow/types.ts](file:///workspace/app/encv-mobile/src/lib/workflow/types.ts) |
| 状态机 | [workflow/stateMachine.ts](file:///workspace/app/encv-mobile/src/lib/workflow/stateMachine.ts) |
| 任务组折叠 | [task-group-collapse.md](file:///workspace/.trae/rules/task-group-collapse.md) |
| Mock 数据生成 | [mock-data-architecture.md](file:///workspace/.trae/rules/mock-data-architecture.md) |

## 十、扩展铁律

> **任何"测试报告 / 工作流运行"类 UI 必须监全 4 件套 task 事件**（task:created / task:update / task:progress / task:completed）。
> 漏一个 → running 期间 step 状态永远 stuck，用户看到"假死"。
>
> **任何"批量执行"类代码必须显式持久化结果**到 localStorage / 后端 DB。
> 不持久化 → 刷新页面 / 关 App / 切 Tab → 用户投诉"我之前的测试跑哪去了"。
>
> **任何动态生成测试用例的代码必须从 plugin 元数据派生**（extraFields + supportedExtensions），禁止硬编码 cipherMode / version / 源文件路径。
> 硬编码 → 插件升级加新选项时，测试报告永远测不到。
