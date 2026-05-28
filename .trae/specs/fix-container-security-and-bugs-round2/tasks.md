# Tasks

## Phase 1: Bug 修复 — FilePickerModal 新建文件夹

- [ ] Task 1: 修复 FilePickerModal 新建文件夹 UI 和交互
  - [ ] SubTask 1.1: 分析当前 v-if/v-else 渲染冲突：`v-if="showNewFolder"` (div) 与 `v-else` (ion-list) 导致点击 + 后文件列表消失
  - [ ] SubTask 1.2: 重构模板：将 new-folder-input 改为 overlay 定位（position: absolute/fixed），不替换 ion-list
  - [ ] SubTask 1.3: 确认 createDirectory API 调用路径正确（检查 encv.ts 中 URL 拼接）
  - [ ] SubTask 1.4: 确认 navigateTo 成功后触发 loadFiles 刷新文件列表
  - [ ] SubTask 1.5: 添加错误处理：API 失败时显示 alert 并保持输入框打开

## Phase 2: Bug 修复 — v4 stsz box missing 误报

- [ ] Task 2: 增加 SkipStructCheck 非严格模式
  - [ ] SubTask 2.1: 在 VerifyOptions 中新增 `SkipStructCheck bool` 字段
  - [ ] SubTask 2.2: 修改 QuickStructCheck 方法签名，接收 opts 参数
  - [ ] SubTask 2.3: 当 SkipStructCheck=true 时，跳过 stsz/moov/parsing 检查，返回 nil（或仅 warning）
  - [ ] SubTask 2.4: 修改 verifyContainer()：重编码模式下同时传入 `SkipStructCheck: true`
  - [ ] SubTask 2.5: 新增测试：重编码 MP4（无标准 stsz）通过 SkipStructCheck 验证

## Phase 3: Bug 修复 — v3 临时目录创建失败

- [ ] Task 3: content_preprocessor MkdirAll 防御
  - [ ] SubTask 3.1: 在 remapMP4ForFastStart() 的 os.CreateTemp 前添加 `os.MkdirAll(p.outputDir, 0755)`
  - [ ] SubTask 3.2: 在 transcodeToFastStartMP4() 的 os.CreateTemp 前添加同样的防御
  - [ ] SubTask 3.3: 在 remapWithMKVMerge() 和 remapMKVWithFFmpeg() 中同样处理
  - [ ] SubTask 3.4: 创建辅助函数 `ensureOutputDir()` 统一处理，避免重复代码
  - [ ] SubTask 3.5: 新增测试：outputDir 不存在时 CreateTemp 能成功创建

## Phase 4: Bug 修复 — v4 容器信息乱码

- [ ] Task 4: 诊断并修复容器信息编码问题
  - [ ] SubTask 4.1: 在 mobile_service.go GetFileInfo() 中检查 ManifestV4() 返回的 Segments 数据是否含二进制/非 UTF-8 数据
  - [ ] SubTask 4.2: 检查 container_id 是否为有效字符串（UUID 格式）
  - [ ] SubTask 4.3: 如果 manifest 中 Segments 含二进制数据，在 JSON 序列化前做 base64 编码或过滤
  - [ ] SubTask 4.4: 在前端 FileInfo.vue / FilePreview.vue 中增加乱码检测和容错显示
  - [ ] SubTask 4.5: 新增测试：验证 /api/file/info 对 v4 容器返回可解析的 JSON

## Phase 5: 测试完善 — Mock 覆盖不足

- [ ] Task 5: 新增关键路径测试
  - [ ] SubTask 5.1: 创建 FilePickerModal 新建文件夹组件测试（mock createDirectory API）
    - 测试点击 + 显示输入框
    - 测试输入名称 + 确认 → 调用 API → navigateTo → 刷新
    - 测试取消 → 输入框隐藏
    - 测试空名称提交被拦截
    - 测试 API 失败 → 显示错误 alert
  - [ ] SubTask 5.2: 创建加密流程 E2E mock 测试（使用 fixture 文件）
    - 测试 v3 不重编码加密完整流程
    - 测试 v4 重编码加密完整流程（验证 SkipSizeCheck + SkipStructCheck）
    - 测试 outputDir 不存在时的自动创建行为
  - [ ] SubTask 5.3: 创建容器信息 API 测试
    - 测试 v3 容器 info 返回正确结构
    - 测试 v4 容器 info 返回正确 container_id 和 manifest（无乱码）

# Task Dependencies

- Task 2 depends on Task 1（无直接依赖，但都是高优先级 bug 修复）
- Task 3 is independent of Tasks 1, 2
- Task 4 is independent of Tasks 1-3
- Task 5 depends on Tasks 1-4（测试需在修复后编写才能验证）

# Parallelizable Work

- Task 1 + Task 2 + Task 3 + Task 4 可并行（互不依赖）
- Task 5 需在 Tasks 1-4 之后
