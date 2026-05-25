# 修复 FFmpeg 符号缺失 + simplifyErrorMessage bug

## 问题 1：`ff_graph_css_data` 符号缺失

```
dlopen failed: cannot locate symbol "ff_graph_css_data" referenced by libffmpeg.so
```

**根因**：`--gc-sections` 删除了 libavfilter 中仅被间接引用的 `ff_graph_css_data`。

**修复**：[build-ffmpeg-android.sh](file:///workspace/app/encv-mobile/scripts/build-ffmpeg-android.sh) 两处链接命令添加 `-Wl,--undefined=ff_graph_css_data`

## 问题 2（关键 bug）：Tasks 错误详情/复制按钮永远不显示

**根因**：[task_manager.go:672](file:///workspace/internal/service/task_manager.go#L672) `simplifyErrorMessage()` 对不匹配的错误原样返回：

```go
task.Error = simplifyErrorMessage(errMsg)  // = "ffprobe failed (exit 1): ..."
task.ErrorDetail = errMsg                  // = "ffprobe failed (exit 1): ..." ← 完全相同！
```

前端 [Tasks.vue:63](file:///workspace/app/encv-mobile/src/views/Tasks.vue#L63)：`v-if="task.errorDetail && task.errorDetail !== task.error"` → **永远 false**

**修复**：新增 ffprobe/ffmpeg/encryption 模式的简化规则 + 超长消息截断，确保 Error ≠ ErrorDetail。

## 问题 3：WebDAV 确认

WebDAV.vue（内联结果区）、mobile_service.go（PROPFIND）、mobile_api.go（结构化返回）、encv.ts（WebDAVTestResult 接口）均已确认包含新代码。Go 后端需重编方可生效。

## 实施步骤

### 步骤 1：修复 FFmpeg 链接符号
**文件**：`app/encv-mobile/scripts/build-ffmpeg-android.sh`
- ffmpeg 链接命令（~272行）：`-Wl,--gc-sections \` 后加 `-Wl,--undefined=ff_graph_css_data \`
- ffprobe 链接命令（~303行）：同上

### 步骤 2：修复 simplifyErrorMessage
**文件**：`internal/service/task_manager.go`

在 `return errMsg` 前新增：
```go
if strings.Contains(errMsg, "ffprobe failed") {
    return "failed to read video metadata"
}
if strings.Contains(errMsg, "ffmpeg failed") {
    return "video encoding failed"
}
if strings.Contains(errMsg, "encryption failed") || strings.Contains(errMsg, "plugin failed") {
    return "encryption processing failed"
}
if len(errMsg) > 120 {
    return errMsg[:120] + "..."
}
```

### 步骤 3：验证
- `go vet ./internal/service/... ./internal/server/... ./internal/utils/...`
- `vue-tsc --noEmit && vite build`

## 文件变更清单

| 文件 | 操作 |
|------|------|
| `app/encv-mobile/scripts/build-ffmpeg-android.sh` | 修改 |
| `internal/service/task_manager.go` | 修改 |
