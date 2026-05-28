# Tasks

- [ ] Task 1: 恢复 FilePreview.vue iframe 文本预览
  - [ ] 1.1 读取当前 FilePreview.vue 完整内容（确认上轮修改的具体位置）
  - [ ] 1.2 将 `<pre><code>{{ textContent }}</code></pre>` + loading/error 状态回退为 `<iframe :src="textPreviewUrl" class="preview-iframe">`
  - [ ] 1.3 恢复 `textPreviewUrl` ref 变量（移除 textContent/textLoading/textError）
  - [ ] 1.4 恢复 loadFile() 中 text 分支的 `textPreviewUrl.value = getFilePreviewUrl('text.html', path)` 赋值
  - [ ] 1.5 移除新增的 `loadTextContent()` 函数和 `.text-content` / `.text-preview` CSS 样式

- [ ] Task 2: 修复 TempFileReadCloser.Close 自动删除导致文件消失
  - [ ] 2.1 修改 `/workspace/internal/v2/reader/temp_file.go` 的 `Close()` 方法：移除 `os.Remove(t.path)` 调用，仅保留 `t.file.Close()`
  - [ ] 2.2 在 registry.go `EncryptFileWithPlugin()` 中，在 `defer dataReader.Close()` 之后添加显式清理：
      ```go
      defer func() {
          if tmpPath, ok := dataReader.(interface{ Name() string }); ok {
              os.Remove(tmpPath.Name())
          }
      }()
      ```
      但要确保在 PostEncryptProcessor + verifyContainer **之后**才执行。当前 defer 顺序是正确的（函数返回时执行），但需要确认 verifyContainer 不依赖 Close 后的文件。
  - [ ] 2.3 排查 "stsz box missing" 验证失败的真正原因 — 这可能是 v4 容器 quick check 对 remux 后的 MP4 文件不适用（remux 改变了 box 结构）。检查 content_verifier.go 的 SkipStructCheck 是否在加密后验证路径中生效。

- [ ] Task 3: 修复插件安装状态不更新
  - [ ] 3.1 读取 GoProcessPlugin.kt `installFromPath()` 完整代码，确认 ComboLite 反射调用 `installPlugin` 的方法签名匹配情况
  - [ ] 3.2 如果反射方法名/签名不匹配导致静默失败：添加详细日志（方法名、参数类型、返回值）到 installFromPath
  - [ ] 3.3 在 ExtensionsPage.vue 中确认 `loadExtensions()` 是否正确刷新已安装插件列表（checkInstalledPlugins → 更新 extensions 列表 → badge 显示）
  - [ ] 3.4 如果 pickAndInstallPlugin 返回 success 但 checkInstalledPlugins 仍返回未安装：说明 ComboLite 安装成功但查询方法有问题 → 增强 fallback 查询逻辑

# Task Dependencies
- 无跨任务依赖，可并行执行
