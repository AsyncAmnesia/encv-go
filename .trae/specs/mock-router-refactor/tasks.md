# Tasks

- [x] Task 1: 重写 `mock/handlers.ts` 为通用路由分发器
  - [x] 删除全部 30+ 个逐端点 handler 函数
  - [x] 实现 `dispatchRequest(req, res)` 主入口：URL path 前缀匹配 + HTTP method 分发
  - [x] 实现 `fileSystemHandler()` 统一处理 `/api/files*` 全部子路径（list/stream/plugin-stream/mkdir/delete/rename/copy/move/search/exists/encrypt-output-exists/tags）
  - [x] 实现 `fileContentHandler()` 处理 `/api/file*`（GET 读内容/PATCH 重命名/POST rename/DELETE）
  - [x] 实现 `staticJsonHandler()` 处理固定 JSON 端点（config/plugins/permissions/health/versions/ffmpeg/build-info/schema/remote/webdav/alist-encrypt/container-extensions/text-preview-exts）
  - [x] 实现 `taskMockHandler()` 处理 `/api/tasks*`（CRUD + predict-plugin）
  - [x] 实现 `staticFileHandler()` 处理 `/stream*` 和 `/preview*`（二进制文件直出，含 MIME 映射）
  - [x] 实现兜底 handler：未匹配路径返回 `{ error, path }` JSON + 501，绝不 next()

- [x] Task 2: 清理 `mock/file-system.ts`
  - [x] 删除硬编码的 DEFAULT_FILES / MOVIES_FILES / DOCUMENTS_FILES / MUSIC_FILES 数组
  - [x] 删除 FILE_MAP 静态映射
  - [x] 保留 MOCK_PLUGINS、MOCK_SUFFIX、setMockSuffix/getMockSuffix
  - [x] 保留 customFiles 运行时缓存 + addMockFile/removeMockFile/resetMockFiles/__mock_control 接口

- [x] Task 3: 更新 `mock/index.ts` 中间件逻辑
  - [x] 将 Object.entries(handlers) 循环替换为单一 `dispatchRequest()` 调用
  - [x] 确保 mock 中间件在 proxy 之前执行且不穿透（try-catch 包裹防止 Vite 内部错误冒泡导致"Cannot set headers after they are sent"崩溃）

- [x] Task 4: 修复 `vite.config.ts` proxy 配置
  - [x] proxy target 端口修正为 2025（与 Go 后端一致）

- [ ] Task 5: 端到端验证
  - [x] 启动 vite dev server（先删 __mock_data__ 触发自动生成）
  - [x] 验证文件列表：`/api/files?path=/` → 4 个目录（SSE stream 也验证）
  - [x] 验证 txt 预览：`/api/file?path=...notes.txt` → UTF-8 内容
  - [x] 验证图片流：`/stream?path=...photo.jpg` → JPEG 二进制 + 正确 MIME
  - [x] 验证 config/plugins/health/tasks 等静态端点
  - [ ] 验证前端 Files 页面显示真实文件列表（非空目录）
  - [ ] 验证前端 txt 文件可点击预览内容

# Task Dependencies
- [Task 2] 无依赖，可与 Task 1 并行 ✅
- [Task 3] 依赖 [Task 1] ✅
- [Task 4] 可与 [Task 1] 并行 ✅
- [Task 5] 依赖 [Task 1][2][3][4] — API 层验证全部通过，前端 UI 验证待确认
