# app/openlist — Hi-Sillot OpenList Fork 协作入口

> **读到本文件即了解 fork 协作全貌，无需再向用户问任何背景问题。**

---

## 1. 文档目的与适用读者

本目录是 encv-mobile（`app/encv-mobile/`）与 Hi-Sillot/OpenList fork 协作的**总入口**。它不存放可编译代码，只承担三个职责：

1. **fork 维护**：clone / push / 打 tag / 同步 frontend dist
2. **跨会话 context 持久化**：新 AI 会话或新工程师把本文档纳入上下文，即可接手工作
3. **故障速查**：常见 8 类问题的「症状 → 根因 → 修复」一表

适用读者：

- **AI 会话**：把本文档贴入 system prompt 或 context，新会话就能自助
- **新工程师**：clone 仓库后先读本 README，再读 [主 spec](../../.trae/specs/integrate-openlist-as-combolite-plugin/spec.md)
- **运维**：构建 AAR / 推送 fork / 排查 CI 失败

---

## 2. 项目背景

端到端架构（移动端）：

```
[Android 设备]
  ├── encv-mobile WebView (Capacitor + Ionic Vue)
  │     └── UI 调 /openlist/* API
  ├── encv-go 子进程 (端口 2025)
  │     └── 反代 /openlist/* → 127.0.0.1:5244（OpenList）
  └── plugin-openlist ComboLite 插件 (端口 5244)
        ├── OpenListService (前台保活)
        └── OpenListBridge → openlist.aar → libgojni.so (in-process)
              └── OpenList 内部 gin → 解密 ENCV 容器 → 流回
```

**关键路径**（以访问一个 `.sccgv` 加密视频为例）：

```
用户点击文件
  → WebView 发 GET /openlist/local-loopback/d/test.sccgv?sign=xxx
  → encv-go(2025) 反代字节透传
  → OpenList(5244) 收到 → handleEncvPreviewFromLink() 识别 .sccgv
  → 调用 encv.GenerateENCVSettingItems + LoadENCVPluginSettings
  → 透明解密 → 以 video/mp4 流回
  → encv-go 透传 → WebView 播放
```

**结论**：OpenList 必须本地化才有解密能力；fork 是**唯一**具备 ENCV 解密能力的 OpenList 变体。

---

## 3. 五层 Fork 关系图

```mermaid
graph TB
  subgraph 本仓库 [encv-mobile / 本仓库]
    A[app/encv-mobile]
    P[plugin-openlist]
    S[scripts/build-openlist-aar.sh]
  end

  subgraph HiSillot [Hi-Sillot 个人 fork]
    F1[Hi-Sillot/OpenList<br/>dev 分支 + openlistlib/]
    F2[Hi-Sillot/OpenList-Frontend<br/>i18n 同步源]
  end

  subgraph 上游 [OpenListTeam 上游]
    U1[OpenListTeam/OpenList<br/>main 分支]
    U2[OpenListTeam/OpenList-Frontend<br/>releases dist]
  end

  subgraph 参考 [K-Sillot 参考实现]
    R1[K-Sillot/OpenList-Mobile<br/>gomobile bind 路线]
    R2[K-Sillot/OpenList-Desktop<br/>Tauri 桌面端]
  end

  S -->|clone dev| F1
  S -->|download releases/tags| U2
  A --> P
  P -->|implementation files libs/openlist.aar| F1
  F1 -.->|定期 rebase| U1
  F2 -.->|参考 i18n| U2
  R1 -.->|参考架构| F1
  R2 -.->|参考集成| A
```

| 仓库 | 角色 | 与本项目关系 |
|------|------|-------------|
| **encv-mobile**（本仓库） | 主项目：WebView + encv-go + ComboLite | — |
| **Hi-Sillot/OpenList** | 个人 fork，集成 ENCV 解密 + `openlistlib/` 入口 | 唯一被 build script clone |
| **OpenListTeam/OpenList** | 上游 OpenList | 周期性 rebase 来源 |
| **OpenListTeam/OpenList-Frontend** | 前端 dist 发布源 | 精确 tag 下载 |
| **K-Sillot/OpenList-Mobile** | gomobile bind 参考实现 | 架构蓝本 |
| **K-Sillot/OpenList-Desktop** | 桌面端 Tauri 实现 | 集成思路参考 |

---

## 4. Hi-Sillot Fork 维护工作流

### 4.1 分支策略

| 分支类型 | 命名 | 用途 | 生命周期 |
|---------|------|------|---------|
| `main` | 上游 main 同步 | 长期 | 永久 |
| `dev` | ENCV 补丁开发 | **默认 checkout 分支** | 永久 |
| `feature/*` | 单次功能开发 | 短期 | 合并后删除 |
| `encv-v*.*.*` | 稳定发布 tag | CI 切到固定版本 | 永久 |

### 4.2 沙箱推送命令模板

参见 §10，此处仅列最常用命令：

```bash
# 1. 克隆（URL 注入 token，详见 §10）
git clone --branch dev \
    https://x-access-token:${GITHUB_TOKEN}@github.com/Hi-Sillot/OpenList.git

# 2. 修改 + 提交
cd OpenList
echo "v4.0.0" > frontend-pinned.txt
git add frontend-pinned.txt
git commit -m "bump frontend pin to v4.0.0"

# 3. 推送（必须用 URL 注入，extraHeader 在 push 时被拒）
git push \
    https://x-access-token:${GITHUB_TOKEN}@github.com/Hi-Sillot/OpenList.git dev
```

### 4.3 Tag 流程（切到稳定版本）

```bash
# 在 fork 上打 tag
cd OpenList
git tag encv-v0.1.0
git push origin encv-v0.1.0

# 在 encv-mobile 仓库切到该 tag
# 编辑 scripts/openlist-fork.env:
#   OPENLIST_FORK_PINNED_TAG=encv-v0.1.0
# 重新跑 build-openlist-aar.sh 即可
```

---

## 5. gomobile bind AAR 架构

### 5.1 为什么 in-process

| 维度 | sidecar binary（错） | **gomobile AAR（对）** |
|------|---------------------|----------------------|
| 进程数 | 2（host + OpenList） | **1（共享 JVM）** |
| 启动 | 3-5s 冷启 | ~100ms in-process |
| Go runtime 数量 | 2 份（encv-go + OpenList） | **1 份（OpenList 嵌入）** |
| AAR/.so 体积 | — | ~30-50MB libgojni.so（stripped） |
| 端口冲突解决 | 外部进程协调 | 简单（共享 process group） |
| 调试 | 跨进程日志拼接 | 统一 Logcat tag=`OpenList` |

### 5.2 链路

```
plugin-openlist/libs/openlist.aar
  ├── classes.jar
  │     └── openlistlib.Openlistlib (Java stub，由 gobind 生成)
  └── jni/arm64-v8a/libgojni.so
        ├── Go runtime (goroutine scheduler, GC, net)
        ├── OpenList 全部业务代码
        └── JNI 导出（Init/Start/Shutdown/IsRunning/ForceDBSync/...）
```

加载链：`System.loadLibrary("gojni")` → JNI 注册 → `Openlistlib.init(event, logCallback)` → Go 侧 `cmd/server.go::Start()` 流程。

### 5.3 5244 端口

OpenList 内部 gin 仍绑 `127.0.0.1:5244`（与桌面版一致）。encv-go 通过 loopback 访问，**无需特殊代码**。

---

## 6. encv-Go 集成点

Hi-Sillot/OpenList fork 侧的 3 个 ENCV 集成点：

| 路径 | 符号 | 调用时机 |
|------|------|---------|
| `internal/encv/init.go` | `GenerateENCVSettingItems()` | `cmd/server.go::Start()` 开头 |
| `internal/encv/init.go` | `LoadENCVPluginSettings()` | 同上，紧随其后 |
| `server/handles/down_ext.go` | `handleEncvPreviewFromLink()` | 拦截 `/d/*` 下载 + `.sccgv` 后缀 |

调用顺序（在 `openlistlib/server.go::Start()` 中封装）：

```go
func Start() {
    // 1. 注册 ENCV 设置项
    encv.GenerateENCVSettingItems()
    encv.LoadENCVPluginSettings()
    encvPlugins.InitializeWithSettings()

    // 2. 上游 bootstrap
    bootstrap.InitOfflineDownloadTools()
    bootstrap.LoadStorages()
    bootstrap.InitTaskManager()

    // 3. 起 server
    r := gin.New()
    server.Init(r)
    r.Run("127.0.0.1:5244")
}
```

**关键不变量**：ENCV 容器解密**只在** OpenList fork 侧发生；encv-go **不**对 `/openlist/*` 路径下的 ENCV 容器做二次解密。

---

## 7. Frontend-pinned.txt 同步机制

### 7.1 为什么不能用 `releases/latest`

Hi-Sillot fork 在 `internal/conf/const.go` 中新增了 ENCV 设置项（`EncvDecryptPassword` / `EncvTextExt` / `EncvAudioExt` / `EncvVideoExt` / `EncvImageExt`）。若用 `releases/latest` 拉前端 dist：

- 上游最新版 frontend **没有**这些 key → Web UI 显示空白 / 跳 404
- i18n 翻译条目版本不匹配 → 切语言后部分页面回退英文
- 上游新增路由 → fork backend 未实现 → 跳 404

### 7.2 4 级 Pin 优先级（高 → 低）

1. `${SRC_DIR}/frontend-pinned.txt`（fork 提交时 pin，**不漂移**）
2. `--frontend-version` CLI / PowerShell 入参
3. `OPENLIST_FRONTEND_VERSION` 环境变量
4. fallback `releases/latest` + stderr warning（CI lint 应 fail）

`build-openlist-aar.sh` 内部：

```bash
WEB_VERSION="$(grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?' \
  "${SRC_DIR}/frontend-pinned.txt" 2>/dev/null | head -n 1)"
# ↓ 若空则走 CLI / env / latest fallback
curl "https://api.github.com/repos/OpenListTeam/OpenList-Frontend/releases/tags/${WEB_VERSION}"
```

### 7.3 VERSION 文件

下载 + i18n overlay 合并后写 `public/dist/VERSION`（内容 `${WEB_VERSION}-encv`，如 `v4.0.0-encv`）。OpenList 后端 `Bootstrap()` 读取并存为 `conf.FrontendVersion`。

ldflags 同步注入：

```
-X 'github.com/OpenListTeam/OpenList/v4/internal/conf.WebVersion=${WEB_VERSION}'
```

---

## 8. i18n Overlay 机制

### 8.1 目录约定

若 fork 在 `public/dist/i18n-overlay/<lang>/translation.json` 放置 ENCV 专用翻译补丁，构建脚本会用 `jq` 合并：

```
public/dist/i18n-overlay/
├── zh-CN/
│   └── translation.json   # ENCV 设置项的中文翻译补丁
└── en/
    └── translation.json
```

### 8.2 合并命令

```bash
jq -s '.[0] * .[1]' \
  public/dist/assets/zh-CN.json \
  public/dist/i18n-overlay/zh-CN/translation.json \
  > public/dist/assets/zh-CN.json.tmp && \
  mv public/dist/assets/zh-CN.json.tmp public/dist/assets/zh-CN.json
```

overlay key **覆盖**原 key（jq `.[0] * .[1]` 语义）。

### 8.3 示例

```json
// public/dist/i18n-overlay/zh-CN/translation.json
{
  "encv": {
    "decryptPassword": "解密密码",
    "textExt": "文本扩展名",
    "audioExt": "音频扩展名",
    "videoExt": "视频扩展名",
    "imageExt": "图片扩展名"
  }
}
```

合并后 OpenList Web UI 切到 zh-CN 显示中文；不切则保持英文 fallback。

---

## 9. 构建脚本索引

| 路径 | 用途 | 读者 |
|------|------|------|
| [scripts/build-openlist-aar.sh](../../scripts/build-openlist-aar.sh) | 主构建脚本（gomobile bind） | 开发者 + CI |
| [scripts/build-openlist-aar.ps1](../../scripts/build-openlist-aar.ps1) | Windows CI 镜像 | CI Windows leg |
| [scripts/openlist-fork.env](../../scripts/openlist-fork.env) | fork URL / branch / pin 配置 | 所有人 |
| [scripts/README.md](../../scripts/README.md) | 构建脚本详细文档 | 开发者 |
| [app/encv-mobile/plugin-openlist/README.md](../encv-mobile/plugin-openlist/README.md) | 插件模块说明 | 移动端开发者 |
| [app/openlist/build-encv-desktop.ps1](./build-encv-desktop.ps1) | 桌面端 Tauri 构建（**已废弃**，仅留作历史） | 维护参考 |

### 9.1 快速开始

```bash
# 1. 编译 openlist.aar
bash scripts/build-openlist-aar.sh \
    --output /workspace/app/encv-mobile/plugin-openlist/libs

# 2. 编译插件 APK
cd app/encv-mobile/android
./gradlew :plugin-openlist:assembleDebug

# 3. 产物
#   app/encv-mobile/plugin-openlist/build/outputs/apk/debug/plugin-openlist-debug.apk
```

---

## 10. 沙箱 GITHUB_TOKEN 推送工作流

> **这是本节最重要的内容，新会话最容易踩坑。**

### 10.1 根因

`GITHUB_TOKEN` 已 export 在 shell env 中，但 **`git` 不会自动把环境变量当凭证使用**。沙箱既无 `~/.netrc` 也无 `~/.gitconfig`：

```bash
$ env | grep GITHUB
GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx  # ← 你的 PAT（示例，非真实值）

$ git push origin dev
fatal: could not read Username for 'https://github.com': terminal prompts disabled
```

### 10.2 解决方案对比

| 方案 | 命令 | 优点 | 缺点 |
|------|------|------|------|
| **1. URL 注入（推荐）** | `git push https://x-access-token:${GITHUB_TOKEN}@github.com/Hi-Sillot/OpenList.git dev` | git 命令统一；clone / push / fetch 都通过 | token 会进 shell history（用 `unset HISTFILE` 或 `set +o history` 缓解） |
| **2. extraHeader** | `git -c http.extraHeader="Authorization: Bearer ${GITHUB_TOKEN}" clone <url>` | clone 可用 | **push 时报 `invalid credentials`**（GitHub 不接受 Bearer 头做写操作），**不可靠** |
| **3. 临时 netrc** | `cat > ~/.netrc <<EOF<br>machine github.com login x password ${GITHUB_TOKEN}<br>EOF`<br>`chmod 600 ~/.netrc` | git 命令无需修饰 | 沙箱每次重启失效；token 写盘 |
| **4. SSH 协议** | `git@github.com:Hi-Sillot/OpenList.git` 配 SSH key | 一劳永逸 | 沙箱无 ssh-agent，**不可行** |

> **macOS Keychain 不适用沙箱**：`git credential-osxkeychain` 仅在交互式 macOS 桌面有效。
>
> **URL 注入的 token 隔离**：`https://x-access-token:TOKEN@github.com/...` 是 GitHub 官方推荐的 PAT 传递方式（触发 HTTP Basic Auth，GitHub PAT 接受）。`https://user:TOKEN@github.com/...` 也可但**user 部分会被 GitHub 忽略**。`x-access-token` 是占位 user name。

### 10.3 失败 / 成功对比

```bash
# ❌ 失败 1（裸命令，触发 username 提示）
git push origin dev
# fatal: could not read Username for 'https://github.com': terminal prompts disabled

# ❌ 失败 2（extraHeader 在 push 时被拒，GitHub 不接受 Bearer 头做写）
git -c http.extraHeader="Authorization: Bearer ${GITHUB_TOKEN}" push origin dev
# remote: invalid credentials
# fatal: Authentication failed for 'https://github.com/Hi-Sillot/OpenList.git/'

# ✅ 成功（URL 注入 — GitHub 接受 x-access-token user + PAT password 走 Basic Auth）
git push https://x-access-token:${GITHUB_TOKEN}@github.com/Hi-Sillot/OpenList.git dev
# To https://github.com/Hi-Sillot/OpenList.git
#    abc1234..def5678  dev -> dev
```

> **shell history 防护**：URL 注入会把 token 写进 `~/.bash_history`，**先**跑 `set +o history` 再 push；或者用 `GITHUB_TOKEN="$(cat ~/.encv-token)"` 内联避免 env 暴露。

### 10.4 build-openlist-aar.sh 已自动处理

`scripts/build-openlist-aar.sh` 检测到 `GITHUB_TOKEN` 时**自动**把 fork URL 改写为 `https://x-access-token:${GITHUB_TOKEN}@github.com/...` 形式，无需手工操作（脚本内日志 `[INFO] using GITHUB_TOKEN for fork auth (ghp_****)` 仅前 4 字符 + 4 星号）。

---

## 11. 故障排查 Checklist

| # | 症状 | 根因 | 修复 |
|---|------|------|------|
| 1 | **`bind: address already in use` 启动失败** | 5244 端口被占用（alist 旧版、nginx 转发、其他 OpenList） | `lsof -i :5244` 找进程；或在 plugin 设置改端口 |
| 2 | **`gomobile init` 失败：找不到 NDK** | NDK 路径错或版本低 | 传 `--ndk $ANDROID_HOME/ndk/26.3.11579264` |
| 3 | **AAR 缺 `openlistlib.Openlistlib` 类** | fork 缺 `openlistlib/` 包 | 参考主 spec §一补全 fork 入口 |
| 4 | **OpenList-Frontend tag 404** | `frontend-pinned.txt` 写了 `v9.9.9`（不存在） | 核对 tag 全名：`v4.0.0` 而非 `4.0.0` |
| 5 | **`replace github.com/Soltus/encv-go => ../../../` 解析失败** | sed 未把相对路径改绝对路径 | 检查 `--encv-go-root` 必为绝对路径 |
| 6 | **OpenListBridge.start() 后 5s 内 5244 不响应** | 端口冲突 / 防火墙 / AAR 缺类 | 跑 §11.3 端口检测；`adb logcat | grep OpenList` |
| 7 | **i18n overlay jq 合并失败** | 缺 `jq` 或 overlay JSON 语法错 | `apt install jq`；`jq . public/dist/i18n-overlay/zh-CN/translation.json` |
| 8 | **AAR 体积 > 50MB** | 编译时未加 `-ldflags="-s -w"` | build script 已默认加；若手动编译务必带 |
| 9 | **`git push` 报 `terminal prompts disabled`** | 见 §10 | 用 `git push https://x-access-token:${GITHUB_TOKEN}@github.com/...` 替换裸 `git push` |
| 10 | **`frontend-pinned.txt` 不被识别** | 文件编码非 UTF-8 / 多行注释不闭合 | `file frontend-pinned.txt` 应为 ASCII/UTF-8 |

### 11.1 调试命令速查

```bash
# 5244 端口冲突
lsof -i :5244
nc -z -w2 127.0.0.1 5244 && echo "OCCUPIED" || echo "FREE"

# OpenList 健康
curl -s http://127.0.0.1:5244/api/site/list | jq .

# fork 同步状态
cd /tmp/openlist-hisillot
git fetch origin && git log --oneline HEAD..origin/dev

# AAR 完整性
unzip -l plugin-openlist/libs/openlist.aar | grep -E "libgojni|Openlistlib"
```

---

## 12. 双向链接

### 12.1 本仓库 spec

- [主 spec: integrate-openlist-as-combolite-plugin](../../.trae/specs/integrate-openlist-as-combolite-plugin/spec.md)
- [本 spec: openlist-fork-onboarding-readme](../../.trae/specs/openlist-fork-onboarding-readme/spec.md)
- [相关: implement-mobile-backend-api](../../.trae/specs/implement-mobile-backend-api/spec.md)
- [相关: eval-combolite-mkv-ffmpeg-plugins](../../.trae/specs/eval-combolite-mkv-ffmpeg-plugins/spec.md)

### 12.2 本仓库 README

- [scripts/README.md](../../scripts/README.md)
- [app/encv-mobile/plugin-openlist/README.md](../encv-mobile/plugin-openlist/README.md)
- [app/openlist/build-encv-desktop.ps1](./build-encv-desktop.ps1)（已废弃）

### 12.3 外部仓库

- [Hi-Sillot/OpenList](https://github.com/Hi-Sillot/OpenList) — 个人 fork，本项目主依赖
- [Hi-Sillot/OpenList-Frontend](https://github.com/Hi-Sillot/OpenList-Frontend) — fork 前端
- [OpenListTeam/OpenList](https://github.com/OpenListTeam/OpenList) — 上游
- [OpenListTeam/OpenList-Frontend](https://github.com/OpenListTeam/OpenList-Frontend) — 上游前端
- [K-Sillot/OpenList-Mobile](https://github.com/K-Sillot/OpenList-Mobile) — gomobile bind 参考
- [K-Sillot/OpenList-Desktop](https://github.com/K-Sillot/OpenList-Desktop) — 桌面端参考

---

## 维护

- 工作流变更 → 同步更新本 README §4-§9；新 ENCV 设置项 → 同步 fork `internal/conf/const.go` + `frontend-pinned.txt` + `i18n-overlay/<lang>/translation.json`；新 CI 失败场景 → 追加到 §11 故障表末。
