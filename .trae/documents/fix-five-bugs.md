# 修复五个 Bug 计划

## Bug 1: 服务器地址端口叠加 `http://127.0.0.1:2025:2025`

### 根因
[ServerDetail.vue:152](file:///workspace/app/encv-mobile/src/views/ServerDetail.vue#L152) 提取 host 时只去除了协议和路径，没有去除端口：
```js
const host = serverUrl.value.replace(/^https?:\/\//, '').replace(/\/.*$/, '')
```
`serverUrl` = `http://127.0.0.1:2025` → `host` = `127.0.0.1:2025` → 拼接 `:${port}` → `http://127.0.0.1:2025:2025`

### 修复方案
提取 host 时同时去除端口部分：
```js
const host = serverUrl.value.replace(/^https?:\/\//, '').replace(/\/.*$/, '').replace(/:.*$/, '')
```

### 修改文件
- `app/encv-mobile/src/views/ServerDetail.vue` 第 152 行

---

## Bug 2: 内置 MPV 播放失败路径为空

### 根因分析
调用链路：`Files.vue:191` → `openPlayer(file.path, ...)` → `GoProcessPlugin:168` → `PlayerOverlayManager.showOverlay(path, ...)` → `buildInitDataJson()` → LynxView `initData.filePath` → `PlayerApp.tsx:38` `const filePath = (initData.filePath as string) || ''`

**问题1**：`file.path` 是文件系统路径如 `/storage/emulated/0/Movies/test.mp4`，传给 `GoBackendModule.getStreamUrl()` 后构建为 `http://127.0.0.1:$port/stream?path=/storage/...`，但后端的 `/stream` 端点期望的是相对路径（如 `Movies/test.mp4`），不是绝对文件系统路径。后端无法找到文件导致播放失败。

**问题2**：`PlayerApp.tsx:258` 的 `useEffect` 中 `if (filePath)` 判断，当 initData 传递正确时应该能进入 `startPlayback`。但如果 initData 传递失败（如 `lynx.__globalProps` 未正确设置），`filePath` 会是空字符串，直接跳过播放，不会显示任何错误提示。

### 修复方案
1. **MPV 模式下传递流 URL 而非文件系统路径**：在 `Files.vue` 的 `playMedia()` 中，MPV 模式应使用 `getFileStreamUrl(file.path)` 获取流 URL，然后传给 `openPlayer()`。`GoProcessPlugin.openPlayer()` 和 `PlayerOverlayManager.showOverlay()` 需要增加一个参数来区分是流 URL 还是本地路径。

2. **空路径时显示错误状态**：`PlayerApp.tsx` 中，当 `filePath` 为空时，应设置 `playerState` 为 `'error'` 并显示错误信息，而不是静默跳过。

### 修改文件
- `app/encv-mobile/src/views/Files.vue` — `playMedia()` MPV 分支改用流 URL
- `app/encv-mobile/src/plugins/GoProcess.ts` — `openPlayer()` 增加参数
- `app/encv-mobile/src/plugins/web.ts` — 接口同步
- `app/encv-mobile/android/app/src/main/java/com/encvgo/app/GoProcessPlugin.kt` — 接收新参数
- `app/encv-mobile/android/app/src/main/java/com/encvgo/app/PlayerOverlayManager.kt` — 传递流 URL
- `app/encv-mobile/lynx-player/src/player/PlayerApp.tsx` — 空路径时显示错误

---

## Bug 3: 内置 MPV 播放失败样式没有垂直居中

### 根因
[player.css:33-39](file:///workspace/app/encv-mobile/lynx-player/src/player/player.css#L33-L39) 中 `.ErrorContainer` 缺少 `flex: 1` 或 `height: 100%`：
```css
.ErrorContainer {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 24px;
  width: 100%;
  /* 缺少 flex: 1 或 height: 100% */
}
```
由于没有高度，容器只有内容高度，`align-items: center` 无效。

### 修复方案
添加 `flex: 1` 使 ErrorContainer 占满父容器剩余空间，实现垂直居中：
```css
.ErrorContainer {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  padding: 24px;
  width: 100%;
  flex: 1;
}
```

### 修改文件
- `app/encv-mobile/lynx-player/src/player/player.css` 第 33-39 行

---

## Bug 4: 弹窗确认按钮文字显示 `COMMON.CONFIRM`

### 根因
[useI18n.ts](file:///workspace/app/encv-mobile/src/composables/useI18n.ts) 中没有定义 `common.confirm` 和 `common.cancel` 键，但以下位置使用了：
- [Files.vue:592](file:///workspace/app/encv-mobile/src/views/Files.vue#L592) — `t('common.confirm')`
- [Files.vue:654](file:///workspace/app/encv-mobile/src/views/Files.vue#L654) — `t('common.confirm')`
- [DevLogs.vue:274](file:///workspace/app/encv-mobile/src/views/DevLogs.vue#L274) — `t('common.cancel')`
- [DevLogs.vue:276](file:///workspace/app/encv-mobile/src/views/DevLogs.vue#L276) — `t('common.confirm')`

当 i18n 键不存在时，`t()` 函数返回键名大写形式 `COMMON.CONFIRM`。

### 修复方案
在 `useI18n.ts` 的两个语言对象中添加 `common.confirm` 和 `common.cancel` 键：
- `zh-CN`: `'common.confirm': '确认'`, `'common.cancel': '取消'`
- `en`: `'common.confirm': 'Confirm'`, `'common.cancel': 'Cancel'`

### 修改文件
- `app/encv-mobile/src/composables/useI18n.ts` — 添加 common 命名空间键

---

## Bug 5: 深色模式下文件长按操作文字灰色可见度差

### 根因
[variables.css:146-150](file:///workspace/app/encv-mobile/src/theme/variables.css#L146-L150) 已定义了 ActionSheet 深色模式变量：
```css
--ion-action-sheet-background: #1e1e1e;
--ion-action-sheet-button-background: #2a2a2a;
--ion-action-sheet-button-color: #ffffff;
--ion-action-sheet-destructive-color: #ff4961;
--ion-action-sheet-title-color: #aaaaaa;
```
但 Ionic 的 ActionSheet 按钮文字可能没有完全使用 `--ion-action-sheet-button-color` 变量，或者某些子元素（如 `.action-sheet-button` 内的 span）使用了默认的灰色。需要检查 Ionic 的 CSS 变量是否完整覆盖了所有文字颜色。

### 修复方案
在 `variables.css` 的 `body.dark` 中增加更完整的 ActionSheet CSS 覆盖，确保所有按钮文字颜色在深色模式下清晰可见：
```css
body.dark {
  /* 现有变量保留 */
  --ion-action-sheet-button-color: #ffffff;
  --ion-action-sheet-button-color-activated: #cccccc;
  --ion-action-sheet-button-color-hover: #e0e0e0;
  --ion-action-sheet-button-color-focused: #e0e0e0;
}
```
如果 CSS 变量不够，则添加直接的 CSS 选择器覆盖：
```css
body.dark .action-sheet-button {
  color: #ffffff !important;
}
body.dark .action-sheet-button .action-sheet-button-inner {
  color: #ffffff !important;
}
```

### 修改文件
- `app/encv-mobile/src/theme/variables.css` — 深色模式 ActionSheet 样式覆盖

---

## Bug 6: WebDAV 测试连接结果总是连接成功

### 根因
[mobile_service.go:228-246](file:///workspace/internal/service/mobile_service.go#L228-L246) 中 `TestWebDAV` 方法不检查 HTTP 响应状态码：
```go
func (s *MobileService) TestWebDAV(url, username, password string) error {
    // ...
    resp, err := client.Do(httpReq)
    if err != nil {
        return err
    }
    resp.Body.Close()
    return nil  // BUG: 不检查 resp.StatusCode
}
```
401（认证失败）、403（禁止访问）、404（未找到）等响应都被视为成功。

### 修复方案
检查 HTTP 响应状态码，非 2xx 返回具体错误：
```go
func (s *MobileService) TestWebDAV(url, username, password string) error {
    client := &http.Client{Timeout: 5 * time.Second}
    httpReq, err := http.NewRequest(http.MethodGet, url, nil)
    if err != nil {
        return &BadRequestError{Err: err}
    }
    if username != "" || password != "" {
        httpReq.SetBasicAuth(username, password)
    }
    resp, err := client.Do(httpReq)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 200 && resp.StatusCode < 300 {
        return nil
    }
    return fmt.Errorf("连接失败: HTTP %d", resp.StatusCode)
}
```

### 修改文件
- `internal/service/mobile_service.go` 第 228-246 行

---

## 实施顺序

1. Bug 1（端口叠加）— 最简单，一行修复
2. Bug 4（COMMON.CONFIRM）— 简单，添加 i18n 键
3. Bug 3（MPV 错误样式）— 简单，CSS 修复
4. Bug 5（深色模式文字）— 中等，CSS 变量覆盖
5. Bug 6（WebDAV 测试连接）— 中等，后端逻辑修复
6. Bug 2（MPV 播放路径为空）— 最复杂，涉及前后端多文件修改

## 构建验证

修改完成后运行：
```bash
cd /workspace/app/encv-mobile && npx vue-tsc --noEmit && npx vite build
```
