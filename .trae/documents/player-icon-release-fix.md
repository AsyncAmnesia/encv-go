# 播放器图标修复 + Release 构建修复 + 播放器重构规划

## 任务 1：全屏图标修复（立即执行）

### 问题
当前全屏/退出全屏使用 `\u2912`（⤒ 上箭头到横线）和 `\u2913`（⤓ 下箭头到横线），不符合全屏图标的认知。

### 修复
替换为更直观的图标：
- 进入全屏：`⛶`（U+26F6，方框四角标记，标准全屏图标）
- 退出全屏：`⛶`（同上，上下文已明确）

修改文件：
- `lynx-player/src/components/PlayerControls.tsx` — `\u2912`/`\u2913` → `⛶`
- `lynx-player/preview.html` — 同步更新

---

## 任务 2：Release 构建修复（立即执行）

### 问题
`npx cap build android --keystorepath keystore/release.jks` 失败，因为 keystore 路径是相对于 `android/` 目录解析的，但 keystore 实际在 `app/encv-mobile/keystore/`，不在 `app/encv-mobile/android/keystore/`。

错误信息：`/home/runner/.../android/keystore/release.jks (No such file or directory)`

### 修复
在 CI 中使用绝对路径传递 keystore：

```yaml
--keystorepath ${{ github.workspace }}/app/encv-mobile/keystore/release.jks
```

---

## 任务 3：播放器重构规划（长期，分阶段）

### 当前架构
- 主应用（Ionic Vue）：文件浏览 → 点击视频 → 跳转 `/player` → StandalonePlayer.vue（Artplayer）
- Lynx 播放器：独立 Lynx 页面，通过 NativeModules 调用 mpv
- 两者独立，没有统一入口

### 目标架构
主应用新增首页 Tab，集成播放器入口，播放器具备完整软件能力。

### Phase 1：首页 + 播放器入口
1. 新增 `Home.vue` 首页，替换当前 `/` 路由
2. 首页展示：最近播放、收藏、媒体库入口
3. Tab 栏调整：首页 | 文件 | 任务 | 远端 | 设置（DevLogs 移入设置子页面）
4. 播放器入口：从首页/文件页点击媒体文件 → 启动 Lynx 播放器

### Phase 2：播放列表 + 媒体探测
1. 播放列表管理：当前目录/自定义列表/收藏列表
2. 媒体探测：扫描指定目录，自动识别音视频文件
3. 播放器内上/下一曲切换

### Phase 3：视频边下边播
1. HTTP Range 请求支持（Go 后端已有流式传输）
2. 缓冲管理：预读策略、缓冲进度显示
3. 离线缓存：已缓冲部分持久化

### Phase 4：音频增强
1. 歌词显示：LRC 格式解析 + 同步滚动
2. 频谱动效：Web Audio API / Canvas 绘制频谱
3. 音频后台播放：Android Service 保活

### Phase 5：设置界面
1. 播放器设置：默认画质、倍速、字幕偏好
2. 缓存管理：播放缓存大小限制、清理
3. 解码器选择：硬解/软解切换

---

## 本次执行范围

仅执行任务 1（图标修复）和任务 2（Release 构建修复）。任务 3 是长期规划，需要后续单独排期。
