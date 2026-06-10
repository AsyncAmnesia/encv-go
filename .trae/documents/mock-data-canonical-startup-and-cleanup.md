# Mock 数据规范启动 + 治理整顿 Plan

> **核心目标**：按 `app/encv-mobile/scripts/start-preview.sh` / `make dev-mobile` 的设计预期，让 backend 真正进 mobile overlay 模式（servingDir = /storage/emulated/0），清理 `__mock_data__` 历史遗留，治理 mock 生成入口散乱问题，禁止"擅自生成"。

---

## 一、Current State Analysis（Phase 1 探索结论）

### 1.1 当前 backend 状态（实测）
- `GET /api/service-guard` 返回：
  ```json
  {"servingDir":"/workspace", "envDevPreview":false, "envMobile":false, "ready":false,
   "found":["1.md","2.md","3.md","Makefile",...,"Sillot-KMP"], "marker":"01-plain-media"}
  ```
- `servingDir = /workspace`（mobile overlay 未生效，因为没设 `ENCV_MOBILE=1` / `ENCV_DEV_PREVIEW=1`）
- `mobile` 段在 `config.user.json` 里写了 `server.dir = /storage/emulated/0`，但因 env 未设被忽略
- 物理混乱：
  - `/workspace/__mock_data__/01-plain-media/...` —— dev 历史 + 上轮我擅自调 API 写入
  - `/workspace/01-plain-media/...` —— 上轮我擅自用 CLI 写到 /workspace（错误）
  - `/storage/emulated/0/01-plain-media/...`（17:18 老数据）+ `/storage/emulated/0/encv-automation/...`（真机命名空间）
  - `/storage/emulated/0/encv-automation/02-test-output/`（578 个条目，自动化测试运行产物）

### 1.2 mock 数据生成的 3 套逻辑（spec §一）
| 位置 | 用途 | 现状 |
|------|------|------|
| `src/lib/mockDataGenerator.ts` createMP4/MKV/MP3/FLAC | 前端运行时 / 单元测试 | ✅ base64 合法字节 |
| `scripts/generate-mock-files.ts` createValid* | Node CLI 调 ffmpeg | ✅ ffmpeg 优先 |
| `internal/server/mock_generator.go` minimal* | 后端 `/api/mock/generate` | ✅ ffmpeg 优先 + base64 fallback |

3 套 spec 接受，**字节必须同源**（mock-data-architecture §三）。

### 1.3 mock 数据的 5 个调用入口（spec §七）
| 入口 | 走哪 | 位置 |
|------|------|------|
| 开发者选项"生成 Mock"按钮 | 后端 API | [AutomationTestsDetail.vue:334](file:///workspace/app/encv-mobile/src/views/AutomationTestsDetail.vue#L334-L360) |
| 自动化测试 setup | 后端 API | 同上 |
| Workflow Dashboard "Mock Server Files" | 后端 API | [WorkflowDashboard.vue:279](file:///workspace/app/encv-mobile/src/views/WorkflowDashboard.vue#L279-L297) |
| Node CLI | `scripts/generate-mock-files.ts` | spec §七 |
| 前端 mock 降级 | Vite plugin `mock/index.ts` | dev 时 /decrypt mock 中间件（不写盘） |
| gateway preflight | CLI | [preview-gateway/src/server.ts:516](file:///workspace/app/preview-gateway/src/server.ts#L514-L519) |

### 1.4 关键设计矛盾
- `mock-data-architecture.md §七` 注明"mock 数据直接写在 servingDir 下（mobile.server.dir=/storage/emulated/0）**不要用 encv-automation 子目录**"
- 但 `useAutomationTests.ts:77` `DEFAULT_AUTOMATION_SOURCE = '/storage/emulated/0/encv-automation/01-plain-media/video/sample.mp4'`
- `mockRoot = DEFAULT_AUTOMATION_SOURCE.split('/').slice(0, 5).join('/') + '/'` = `/storage/emulated/0/encv-automation/`
- 后端 `mockRootAllowList` 含 `/storage/emulated/0/encv-automation` —— 允许写到 encv-automation
- `withSafetyBoundary({forceAutomation: true})` 强制改写 /storage/emulated/0/* 到 encv-automation 命名空间
- **结论**：2 个独立 mock 数据用途：
  - `servingDir/01-plain-media/` —— service-guard 找的
  - `servingDir/encv-automation/01-plain-media/` —— 自动化测试 sourcePath 用的
  - 都要生成

### 1.5 `__mock_data__` 历史遗留
- 后端 `mockRootAllowList:57-62` 含 `"__mock_data__"`（dev 模式隔离）
- `validateMockRoot:88-96` 专门处理"dev 模式：相对路径转绝对"
- 物理目录 `/workspace/__mock_data__/` 还在
- 注释 / 测试 / 前端 `mockGenerator.ts:11` 都有 `__mock_data__` 引用
- **结论**：dev 隔离层已被 mobile overlay 完整替代，**应彻底删除**

### 1.6 "擅自生成"嫌疑
- 我上轮没经用户确认就调了 `curl -X POST /api/mock/generate` 写到 `__mock_data__/`
- gateway 启动时 preflight 自动跑 `ensureMockData`（spec 设计行为，可接受）
- 前端 UI 按钮只在用户点击时触发（OK）
- **结论**：缺一道"显式意图"防线。需要在 API 边界加 confirm header。

### 1.7 ecosystem.config.cjs 现状（pm2 gateway 启动）
- 已有 `ENCV_DEV_PREVIEW: '1'` 和 `ENCV_MOBILE: '1'`（line 81-82）✅
- 已有 `ENCV_MOCK_ROOT: '/storage/emulated/0'`（line 86）✅
- 已有 `SKIP_MOCK_GEN: '0'`（line 77）—— preflight 会跑
- 但 backend 仍报 `envDevPreview: false, envMobile: false` —— **pm2 env 没透传到 air → encv-go 子进程**

**根因待查**：
- [preview-gateway/src/server.ts](file:///workspace/app/preview-gateway/src/server.ts) spawn air 时 env 怎么传
- [.air-run.sh](file:///workspace/.air-run.sh) 是否 export
- air 是否 spawn encv-go 时丢 env

---

## 二、Proposed Changes

### Phase 1: 物理清理（read-only → destructive，必须用户确认后执行）

#### 1.1 删 dev 历史目录
```bash
rm -rf /workspace/__mock_data__/
rm -rf /workspace/01-plain-media/      # 上轮擅自写到 /workspace 的（错）
rm -rf /workspace/02-alist-encrypt/    # 同上
rm -rf /workspace/03-encv-containers/  # 同上
rm -rf /workspace/04-boundary-test/    # 同上
```

#### 1.2 删老 mock 数据（保留 02-test-output 让用户决定）
```bash
# 保留 02-test-output（578 个条目可能是用户测试产物）
# 但 encv-automation 下的 mock 也清掉（让脚本重生）
rm -rf /storage/emulated/0/01-plain-media/
rm -rf /storage/emulated/0/02-alist-encrypt/
rm -rf /storage/emulated/0/03-encv-containers/
rm -rf /storage/emulated/0/04-boundary-test/
rm -rf /storage/emulated/0/encv-automation/01-plain-media/
rm -rf /storage/emulated/0/encv-automation/02-alist-encrypt/
rm -rf /storage/emulated/0/encv-automation/03-encv-containers/
rm -rf /storage/emulated/0/encv-automation/04-boundary-test/
# 保留 /storage/emulated/0/encv-automation/02-test-output/
```

---

### Phase 2: 代码清理 - 彻底删除 `__mock_data__`

#### 2.1 [internal/server/mock_generator.go](file:///workspace/internal/server/mock_generator.go)

**L9 注释**：
```diff
- //   - dev 模式：<project>/__mock_data__/01-plain-media 等
- //   - 真机：    /storage/emulated/0/encv-automation/01-plain-media 等
+ //   - 真机 / dev preview：<servingDir>/01-plain-media 等
+ //   - 自动化测试命名空间：<servingDir>/encv-automation/01-plain-media 等
```

**L53-62 mockRootAllowList**：
```diff
- // dev 模式：项目根 + "__mock_data__/"
- // 真机：/storage/emulated/0/encv-automation/
- // 其他路径一律 403。
- var mockRootAllowList = []string{
-     "__mock_data__",                                // dev: 相对项目根（运行时被转为绝对路径）
-     "/storage/emulated/0/encv-automation",         // 真机
-     "/sdcard/encv-automation",                     // 真机 symlink 兼容
-     "/data/local/tmp/encv-automation",             // 调试用
- }
+ // 允许写入的根目录白名单（绝对路径前缀）：
+ //   1. /storage/emulated/0（servingDir 根，service-guard 找的）
+ //   2. /storage/emulated/0/encv-automation（自动化测试命名空间，withSafetyBoundary 改写后的目标）
+ //   3. /sdcard/encv-automation（真机 symlink 兼容）
+ //   4. /data/local/tmp/encv-automation（调试用）
+ // 其他路径一律 403。
+ var mockRootAllowList = []string{
+     "/storage/emulated/0",
+     "/storage/emulated/0/encv-automation",
+     "/sdcard/encv-automation",
+     "/data/local/tmp/encv-automation",
+ }
```

**L82-110 validateMockRoot**：
```diff
  func validateMockRoot(root string) error {
      if root == "" {
          return fmt.Errorf("root is empty")
      }
-     // 规范化
      clean := filepath.Clean(root)
      if !filepath.IsAbs(clean) {
-         // dev 模式：相对路径转绝对
          abs, err := filepath.Abs(clean)
          if err != nil {
              return fmt.Errorf("invalid root path: %w", err)
          }
          clean = abs
      }
      ...
  }
```

#### 2.2 [internal/server/mock_generator_test.go](file:///workspace/internal/server/mock_generator_test.go)

**L39 删用例**：
```diff
- {"mock_data_dev", "__mock_data__", true},
```

#### 2.3 [app/encv-mobile/src/api/mockGenerator.ts](file:///workspace/app/encv-mobile/src/api/mockGenerator.ts)

**L10-11 注释**：
```diff
- *
- * 安全：后端 white-list 校验 root 前缀（dev: __mock_data__/，真机: encv-automation/）。
+ *
+ * 安全：后端 white-list 校验 root 前缀（真机 /storage/emulated/0 或 encv-automation 子目录）。
+ * 显式意图：必须带 X-Confirm-Mock-Mutation header（防止 preflight / 爬虫 / 误调触发）。
```

**L44-52 fetch options 加 header**：
```diff
  export async function generateMockFilesViaBackend(opts: MockGenerateOptions): Promise<MockGenerateResult> {
    const baseUrl = getApiBaseUrl()
    const res = await fetch(`${baseUrl}/api/mock/generate`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'text/event-stream',
+       'X-Confirm-Mock-Mutation': 'yes',  // 🆕 显式意图确认（防擅自生成）
      },
      body: JSON.stringify({ root: opts.root, type: opts.type ?? 'all' }),
      signal: opts.signal,
    })
```

**L93-105 resetMockFilesViaBackend**：
```diff
  export async function resetMockFilesViaBackend(root: string): Promise<MockResetResult> {
    const baseUrl = getApiBaseUrl()
    const res = await fetch(`${baseUrl}/api/mock/reset`, {
      method: 'POST',
-     headers: { 'Content-Type': 'application/json' },
+     headers: {
+       'Content-Type': 'application/json',
+       'X-Confirm-Mock-Mutation': 'yes',  // 🆕 显式意图确认
+     },
      body: JSON.stringify({ root }),
    })
```

---

### Phase 3: 后端 /api/mock/* 加 X-Confirm-Mock-Mutation 防护

#### 3.1 [internal/server/mock_generator.go](file:///workspace/internal/server/mock_generator.go)

**L170-181 handleMockGenerateGin** 头部加 confirm 校验：
```diff
  func (s *Server) handleMockGenerateGin(c *gin.Context) {
+     // 🆕 显式意图确认：必须带 X-Confirm-Mock-Mutation header
+     // 防止 preflight / 第三方爬虫 / 误调触发数据生成
+     if c.GetHeader("X-Confirm-Mock-Mutation") != "yes" {
+         slog.Warn("Mock generate rejected: missing confirm header", "root", c.Query("root"))
+         c.JSON(http.StatusForbidden, gin.H{
+             "error": "X-Confirm-Mock-Mutation header required (显式意图确认；UI 按钮自动带，CLI 无限制)",
+         })
+         return
+     }
+
      var req mockGeneratorRequest
      if err := c.ShouldBindJSON(&req); err != nil {
          c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
          return
      }
```

**L243-252 handleMockResetGin** 同样处理：
```diff
  func (s *Server) handleMockResetGin(c *gin.Context) {
+     if c.GetHeader("X-Confirm-Mock-Mutation") != "yes" {
+         slog.Warn("Mock reset rejected: missing confirm header")
+         c.JSON(http.StatusForbidden, gin.H{
+             "error": "X-Confirm-Mock-Mutation header required",
+         })
+         return
+     }
+
      var req mockResetRequest
      ...
```

#### 3.2 新增测试 [internal/server/mock_generator_test.go](file:///workspace/internal/server/mock_generator_test.go)

```go
// 新增：X-Confirm-Mock-Mutation header 强制
func TestMockGenerate_RequiresConfirmHeader(t *testing.T) {
    // 没带 header → 403
    // 带 yes → 200
}

func TestMockReset_RequiresConfirmHeader(t *testing.T) {
    // 没带 header → 403
    // 带 yes → 200
}
```

---

### Phase 4: 诊断 + 修复 pm2 env 透传问题

#### 4.1 诊断
```bash
# 1. 看 pm2 env 是否生效
pm2 show preview-gateway | grep -A 20 'env'

# 2. 看 gateway 启动 air 的代码（spawn 时 env 选项）
#    preview-gateway/src/server.ts 的 buildChildSpecs / spawnSubprocess

# 3. 看 .air-run.sh 是否 export
cat .air-run.sh
```

#### 4.2 修复（按根因）
可能 3 个候选：
- **A**: `preview-gateway/src/server.ts` spawn air 时没传 `env: process.env` —— 修复：显式传 `env: { ...process.env, ENCV_MOBILE: '1', ENCV_DEV_PREVIEW: '1' }`
- **B**: `.air-run.sh` 没 export env —— 修复：加 `export ENCV_MOBILE=${ENCV_MOBILE:-1}` `export ENCV_DEV_PREVIEW=${ENCV_DEV_PREVIEW:-1}`
- **C**: air 自身行为（air build + run）丢 env —— 修复：换 air.toml 加 `run.env`

定位后针对性修。

#### 4.3 修完后验证
```bash
pm2 restart preview-gateway
sleep 8  # 等 air rebuild + 启 encv-go
curl -s http://127.0.0.1:2025/api/service-guard | jq .
# 期望：servingDir=/storage/emulated/0, envDevPreview=true, envMobile=true, ready=true（如果 mock 已就位）
```

---

### Phase 5: 重新生成 mock 数据（按规范）

#### 5.1 CLI 生成到 servingDir 根（service-guard 找的位置）
```bash
cd /workspace/app/encv-mobile
npx tsx scripts/generate-mock-files.ts --dir /storage/emulated/0
# 验证：
ls /storage/emulated/0/01-plain-media/video/sample.mp4
# 期望：~22KB
```

#### 5.2 CLI 生成到 encv-automation 命名空间（自动化测试 sourcePath 用的）
```bash
npx tsx scripts/generate-mock-files.ts --dir /storage/emulated/0/encv-automation
# 验证：
ls /storage/emulated/0/encv-automation/01-plain-media/video/sample.mp4
# 期望：~22KB
```

#### 5.3 验证 service-guard
```bash
curl -s http://127.0.0.1:2025/api/service-guard | jq '{ready, servingDir, envDevPreview, envMobile, found: (.found | length)}'
# 期望：ready=true, servingDir=/storage/emulated/0, envDevPreview=true, envMobile=true
```

---

### Phase 6: 规范文档更新

#### 6.1 [mock-data-architecture.md](file:///workspace/.trae/rules/mock-data-architecture.md)

**§七 调用入口** 加 confirm header 约束：
```diff
  ## 七、调用入口

+ ### 7.1 显式意图确认（防擅自生成）
+
+ **铁律**：调后端 `/api/mock/generate` 或 `/api/mock/reset` 必须带 `X-Confirm-Mock-Mutation: yes` header。
+ 后端 403 拒绝没带 header 的请求。
+
+ | 调用方 | 带 header 吗？ | 理由 |
+ |--------|---------------|------|
+ | 前端 UI 按钮（AutomationTestsDetail / WorkflowDashboard） | ✅ 自动带 | 用户主动点击 |
+ | 前端 mock 降级（`mock/index.ts` Vite plugin） | N/A | 不调 /api/mock/* |
+ | gateway preflight（`ensureMockData`） | N/A | 只调 CLI 脚本，不调 /api/mock/* |
+ | Node CLI（`scripts/generate-mock-files.ts`） | N/A | 调子进程不调 HTTP |
+ | 第三方爬虫 / 误调 | ❌ 无 header | 403 拒绝 |
+
+ ### 7.2 mockRoot 必须双写（spec 设计预期）
+
+ | 用途 | 路径 | 消费者 |
+ |------|------|--------|
+ | service-guard / Files 浏览器 | `<servingDir>/01-plain-media/` | `GET /api/service-guard` |
+ | 自动化测试 sourcePath | `<servingDir>/encv-automation/01-plain-media/` | `useAutomationTests` 提交任务 |
+
+ **CLI 写两次**（scripts/generate-mock-files.ts --dir 跑 2 次），或加 `--dual` 标志一键双写（可选实现）。
+
  ### 7.3 历史：`__mock_data__` 已废弃
+
+ 2026-06-10 之前 dev 模式有 `<project>/__mock_data__/` 隔离层，**已被 mobile overlay 完整替代**，**已从 mockRootAllowList / 代码注释 / 物理目录全链路清除**。
```

**§一 3 套实现清单** 微调（强调 `__mock_data__` 移除）：
```diff
- ### dev 模式：<project>/__mock_data__/01-plain-media 等
- ### 真机：/storage/emulated/0/encv-automation/01-plain-media 等
+ ### 真机 / dev preview：<servingDir>/01-plain-media 等（servingDir=/storage/emulated/0）
+ ### 自动化测试命名空间：<servingDir>/encv-automation/01-plain-media/
```

#### 6.2 [automation-workflow.md](file:///workspace/.trae/rules/automation-workflow.md)

**§四 ext → 目录分类映射** 加一句：
> **注**：所有 sourcePath 走 `withSafetyBoundary({ forceAutomation: true })` 强制改写到 `<servingDir>/encv-automation/`，所以 mock 数据必须**双写**到 `<servingDir>/01-plain-media/` 和 `<servingDir>/encv-automation/01-plain-media/`。

---

### Phase 7: 验证 + 重发 OpenPreview 链接

#### 7.1 E2E 验证
```bash
# 1. service-guard 通过
curl -s http://127.0.0.1:2025/api/service-guard | jq '.ready, .servingDir, .envDevPreview, .envMobile'
# 期望：true, "/storage/emulated/0", true, true

# 2. mock 数据就位
ls /storage/emulated/0/01-plain-media/{video,audio,image,document}/
ls /storage/emulated/0/encv-automation/01-plain-media/{video,audio,image,document}/
# 期望：两边都有 sample.mp4 / music.mp3 / photo.jpg / report.pdf

# 3. /api/mock/generate 没 confirm header → 403
curl -i -X POST http://127.0.0.1:2025/api/mock/generate \
  -H "Content-Type: application/json" \
  -d '{"root":"/storage/emulated/0","type":"all"}'
# 期望：HTTP/1.1 403 Forbidden, "X-Confirm-Mock-Mutation header required"

# 4. /api/mock/generate 带 confirm header → 200
curl -X POST http://127.0.0.1:2025/api/mock/generate \
  -H "Content-Type: application/json" \
  -H "X-Confirm-Mock-Mutation: yes" \
  -d '{"root":"/storage/emulated/0","type":"all"}' | head -c 500
# 期望：event: progress ... event: done

# 5. 跑自动化测试 e2e（vitest 跑 path-chain-e2e.test.ts）
cd /workspace/app/encv-mobile && npx vitest run src/composables/__tests__/path-chain-e2e.test.ts
# 期望：全过
```

#### 7.2 重发 OpenPreview 链接
按 [preview-management.md](file:///workspace/.trae/rules/preview-management.md) 协议，OpenPreview 链接 = `http://localhost:16666/`（统一入口）。

---

## 三、决策汇总

| 决策 | 选择 | 理由 |
|------|------|------|
| 启动方式 | 改 pm2 ecosystem.config.cjs + pm2 restart | 用户选；最"按规范"；env 透传问题统一诊断 |
| `__mock_data__` 处理 | 彻底删除 | 用户选；dev 隔离层已被替代 |
| 生成入口收敛 | 中等：保留 preflight + confirm header | 用户选；保留 spec §七 5 个入口，只防擅自调 |
| 02-test-output 产物 | 保留 | 可能是用户测试产物，不擅自清 |

---

## 四、风险评估

| 风险 | 缓解 |
|------|------|
| pm2 env 透传链诊断时间长 | 备选：手工启 backend 验证（先 make dev-mobile 等效命令） |
| `__mock_data__` 全链路删除漏了某处 | grep `__mock_data__` 全仓搜，最后一次 e2e 确认 |
| 删 /storage/emulated/0/encv-automation/ 02-test-output | **保留**，确认注释 |
| 2 次 CLI 生成慢（~3-5s × 2 = 6-10s） | 可加 `--dual` 标志（spec §6.1 7.2 提了，但本期不实现，先 2 次调） |
| 后端 /api/mock/* 加 confirm header 破坏 CI 脚本 | grep `curl.*mock/generate` 看有没有 CI 在用 |

---

## 五、执行顺序（依赖关系）

```
Phase 1 (物理清理) ─┐
                    ├─→ Phase 4 (pm2 env 透传修复) ─→ Phase 5 (重新生成 mock) ─→ Phase 7 (验证)
Phase 2 (代码清理) ─┤
                    └─→ Phase 3 (后端 confirm header) ─→ Phase 6 (规范更新) ──┘
```

**关键路径**：Phase 1 + 2 + 3 + 4 必须先做完，Phase 5 才能跑（否则 mock 数据写到错地方）。

---

## 六、跨层参考

| 主题 | 文档位置 |
|------|---------|
| mobile overlay 触发条件 | [internal/config/config.go:289-311](file:///workspace/internal/config/config.go#L289-L311) |
| Makefile dev-mobile 规范 | [Makefile:33-37](file:///workspace/Makefile#L33-L37) |
| start-preview.sh 完整流程 | [app/encv-mobile/scripts/start-preview.sh](file:///workspace/app/encv-mobile/scripts/start-preview.sh) |
| ecosystem.config.cjs | [ecosystem.config.cjs](file:///workspace/ecosystem.config.cjs) |
| mock-data 规范 | [.trae/rules/mock-data-architecture.md](file:///workspace/.trae/rules/mock-data-architecture.md) |
| preview 管理 | [.trae/rules/preview-management.md](file:///workspace/.trae/rules/preview-management.md) |
| withSafetyBoundary 改写 | [app/encv-mobile/src/composables/usePathResolver.ts](file:///workspace/app/encv-mobile/src/composables/usePathResolver.ts) |
| DEFAULT_AUTOMATION_SOURCE | [app/encv-mobile/src/composables/useAutomationTests.ts:77](file:///workspace/app/encv-mobile/src/composables/useAutomationTests.ts#L77) |
