# 项目规则

## Trae Web 沙箱网络限制（重要！本地构建必读）

- **沙箱禁止 Java/JVM 进程出站 TCP 连接**（所有 JDK 版本均受影响）
- 详细诊断数据、进程级网络策略矩阵、绕过方案 → [trae_web_sandbox_network.md](.trae/rules/trae_web_sandbox_network.md)
- **CI 环境不受此限制**，Gradle 构建应在 CI 执行
- 沙箱内 `curl`/`npm`(走 MCP HTTP 代理) 可正常联网

## FFmpeg 版本备注

- 当前使用 FFmpeg 8.0，构建脚本: `app/encv-mobile/scripts/build-ffmpeg-android.sh`
- fftools（libffmpeg.so/libffprobe.so）采用静态链接方式：FFmpeg 各库的 `.a` 文件被链接进 fftools .so，运行时无需额外的 libavutil.so 等依赖
- 链接时使用 `--allow-multiple-definition`（解决 FFmpeg 多库重复符号如 ff_log2_tab）+ `--gc-sections`（死代码消除）
- 链接时必须包含 `-lz`（FFmpeg 默认启用 zlib，`libavformat` 使用 `uncompress()` 解压 MOV/MP4 容器数据；缺少 `-lz` 导致 `dlopen` 失败：`cannot locate symbol "uncompress"`）
- 编译和链接时使用 `-ffunction-sections -fdata-sections` + `-Wl,--gc-sections`（启用死代码消除，减小 .so 体积）
- FFmpeg configure 使用 `--enable-small`（优化代码大小）+ `--disable-asm`（见下文）
- 链接后使用 `llvm-strip --strip-all` 剥离调试符号
- CFLAGS 使用 `-std=c11 -include time.h`（解决 NDK Clang 的 struct tm 前向声明问题）
- CFLAGS **禁止**添加 `-I${FFMPEG_SRC}/libavutil` 等直接指向 FFmpeg 库子目录的 `-I` 标志（`libavutil/time.h` 会被 `-include time.h` 或 `#include <time.h>` 优先匹配到，导致系统 `<time.h>` 被遮蔽，`struct tm`/`gmtime`/`localtime`/`strftime`/`time` 未声明；fftools 源码使用 `#include "libavutil/xxx.h"` 形式，`-I${FFMPEG_SRC}` 已足够覆盖）
- CFLAGS 必须定义 `-DHAVE_SYS_RESOURCE_H=1 -DHAVE_UNISTD_H=1 -DHAVE_SYS_SELECT_H=1`（FFmpeg fftools 手动编译时不经过 configure 生成 config.h，这些宏控制条件包含 `<sys/time.h>`/`<unistd.h>`/`<sys/select.h>`；缺少 `HAVE_SYS_RESOURCE_H` 会导致 `ffmpeg_opt.c` 中 `struct tm`/`gmtime`/`localtime`/`strftime`/`time` 未声明，因为 NDK 的 `<time.h>` 在 `-std=c11 -D_POSIX_C_SOURCE` 下需要 `<sys/time.h>` 前置包含才能提供完整定义）
- CFLAGS 必须包含 `-I compat/stdbit`（FFmpeg 8.0 fftools 使用 C23 `<stdbit.h>`，NDK Clang 17 不支持，需使用 FFmpeg 自带的兼容头文件 `compat/stdbit/stdbit.h`，该文件使用 `__builtin_clz`/`__builtin_ctz`/`__builtin_popcount` 等 Clang 内建函数实现）
- FFmpeg 8.0 已移除 libpostproc，链接列表中不能有 `-lpostproc`
- FFmpeg configure 必须使用 `--disable-asm`（Android ARM64 上 NEON 汇编 `.S` 文件使用 `ADRP` 等 PC 相对重定位，与共享库链接不兼容，导致 `R_AARCH64_ADR_PREL_PG_HI21 cannot be used against symbol; recompile with -fPIC` 链接错误；vcpkg 的 FFmpeg 构建脚本同样在 Android 上禁用 asm）
- FFmpeg 8.0 fftools 新增 `textformat/` 子目录（ffprobe 输出格式化），需在 CFLAGS 中添加 `-I fftools/textformat`
- FFmpeg 8.0 fftools 新增 `graph/` 和 `resources/` 子目录，需在 CFLAGS 中添加 `-I fftools/graph` 和 `-I fftools/resources`
- h264 编码器在 8.0 中通过 `libx264` 包装器提供，configure 使用 `--enable-encoder=libx264`
- x264 的 `--enable-pic` 仍然必须（共享库需要位置无关代码）
- pkg-config wrapper 方案仍然适用

## 移动端 ffmpeg 调用架构

- Go 后端通过 cgo + dlopen 直接加载 `libffmpeg.so`/`libffprobe.so`，调用 `ffmpeg_run()`/`ffprobe_run()` 函数
- 不经过 HTTP/Kotlin/JNI 中间层
- stdout/stderr 通过 dup2 重定向到临时文件捕获
- 环境变量 `ENCV_LIB_DIR` 指向 Android `nativeLibraryDir`
- `build-info.json` 通过 Android assets 分发：构建脚本复制到 `assets/`，Kotlin 端 `ensureBuildInfoExists()` 复制到 `filesDir`，Go 后端从 `HOME` 环境变量（即 `filesDir`）读取
- 相关文件：`internal/utils/ffmpeg_dlopen.go`（Android）、`internal/utils/ffmpeg_dlopen_stub.go`（桌面端）、`internal/utils/video.go`、`internal/utils/build_info.go`

## 前端构建验证

- `vue-tsc --noEmit` 对 `.vue` 文件 `<script setup>` 中未使用导入存在漏检（TS6133）
- **必须同时运行 `vite build`**，Rollup 会检测未使用导入并报错
- 完整验证流程：`vue-tsc --noEmit && vite build`

## 配置模板保护（重要！）

- **严禁擅自修改 `config.user.json`**：该文件是唯一用户配置模板（桌面端+移动端共用），任何端口/路径/密码等值的修改必须通过用户明确指令执行
- 如需临时改变开发端口等参数，应使用环境变量 `ENCV_CONFIG_PATH` 指向临时配置文件，或命令行 `--config` 标志
- **不得创建独立的 `config.mobile.json` 或其他平台特定配置模板**：移动端适配通过 `mobile` 配置段的 overlay 机制实现
- **严禁在合并/初始化阶段隐式替换配置值**：`mergeConfigDefaults()` 等函数只负责填补缺失字段，**绝不能**根据 mobile 段的值去修改 server.dir 等顶层字段。mobile→顶层的映射只能通过 `ApplyMobileOverlay()` 在运行时完成
- 违反此规则导致的配置模板破坏将被视为严重错误

## Mobile Overlay 机制（核心架构）

> **原则：`mobile` 段是运行时 overlay（覆盖层），不修改持久化的 config.user.json。**

### 字段命名规则

`mobile` 段的字段命名**必须镜像目标配置路径**，实现无歧义映射：

```json
{
  "mobile": {
    "server": { "dir": "/storage/emulated/0" },
    "output": { "path": "/storage/emulated/0/encv-output" },
    "webdav": { "dir": "" }
  }
}
```

| mobile 路径 | 映射到顶层 | Go 类型 |
|------------|-----------|---------|
| `mobile.server.dir` | `server.dir` | `MobileServerConfig.Dir` |
| `mobile.output.path` | `output_path` | `MobileOutputConfig.Path` |
| `mobile.webdav.dir` | `webdav.dir` | `MobileWebdavConfig.Dir` |

**禁止使用扁平命名**如 `server_dir`、`output_path`、`webdav_dir`——这类命名无法从 JSON 结构推断映射关系。

### 触发条件

`ApplyMobileOverlay(cfg)` 在 `config.Load()` 的 `finalize()` 阶段调用，触发条件：

| 环境变量 | 场景 | 效果 |
|---------|------|------|
| `ENCV_MOBILE=1` | Android 真机（EncvGoService.kt 设置） | ✅ overlay 生效 |
| `ENCV_DEV_PREVIEW=1` | 桌面端移动预览（Makefile dev-mobile） | ✅ overlay 生效 |
| 均未设置 | 桌面端正常启动 | ❌ mobile 段被忽略 |

### 数据流

```
config.user.json (持久化, 不被修改)
  ├── server: { dir: "/", port: 2025 }     ← 桌面端值
  └── mobile:
      └── server: { dir: "/storage/emulated/0" }
              ↓
   Load() → finalize() → ApplyMobileOverlay()  ← 仅内存中生效
              ↓
   运行时 Config: server.dir = "/storage/emulated/0"
   (config.user.json 文件内容不变)
```

### 禁止的反模式

1. ❌ **在 mergeConfigDefaults() 中用 mobile 值替换 server.dir** — 这是隐式覆盖，违反配置透明性
2. ❌ **扁平命名 `server_dir`** — 无法从 JSON 结构自描述映射关系
3. ❌ **只在 ENCV_DEV_PREVIEW 下生效** — 导致 Android 真机的 mobile 配置永远不被应用
4. ❌ **在 finalize() 中对 "/" 做 os.Getwd() fallback 后再 overlay** — 顺序错误，overlay 应在最后

## Go Build Tag 平台约束规则（重要！）

- **凡是有平台特定 stub 实现的函数，其对应的主实现文件必须添加互斥 build tag**
- 移动端存根文件使用 `//go:build android`，对应桌面端实现必须使用 `//go:build !android`
- Windows 平台同理：`//go:build windows` ↔ `//go:build !windows`
- **禁止**只给 stub 文件加 tag 而主文件留空（会导致 GOOS 交叉编译时重复声明错误）
- 正确示例参考：`internal/utils/ffmpeg_dlopen.go` (android) ↔ `internal/utils/ffmpeg_dlopen_stub.go` (!android)
- 每次新增平台 stub 文件对时，**必须**同时验证 `GOOS=android` 和默认平台的编译通过

## GitHub 项目搜索规范（重要！）

- **搜索 GitHub 项目时**：优先使用 `WebFetch` 访问 `https://github.com/search?q=关键词&type=repositories`（GitHub 官方搜索 API），而非通用搜索引擎
- **搜索技巧**：用 `site:github.com` 限定 + 精确引号包裹项目名，如 `"Sillot-KMP" site:github.com`
- **已知优质参考项目**：
  - [Hi-Sillot/Sillot-KMP](https://github.com/Hi-Sillot/Sillot-KMP) — Kotlin Multiplatform + Compose 模板，含完整的阿里云/腾讯 Maven 镜像配置
  - [K-Sillot](https://github.com/K-Sillot)（汐洛套件）— 上游依赖镜像集合
  - [Tencent-TDS/KuiklyUI-AI](https://github.com/Tencent-TDS/KuiklyUI-AI) — Kuikly Compose DSL 编码规范（rules/kuiklyComposeDSL.mdc）
- **拉取源码参考**：优先 `raw.githubusercontent.com/{owner}/{repo}/{branch}/{path}` 获取原始文件内容
- **Gradle/Maven 镜像（国内网络）**：当沙箱无法访问 `maven.google.com` / `repo.maven.org` 时，使用以下镜像（来自 Sillot-KMP settings.gradle.kts）：

### pluginManagement.repositories 镜像顺序
```kotlin
maven { url = uri("https://maven.aliyun.com/repository/google") }
maven { url = uri("https://maven.aliyun.com/repository/central") }
maven { url = uri("https://maven.aliyun.com/repository/gradle-plugin") }
maven { url = uri("https://maven.aliyun.com/repository/public") }
maven { url = uri("https://mirrors.tencent.com/nexus/repository/maven-tencent/") }
google()
gradlePluginPortal()
mavenCentral()
```

### dependencyResolutionManagement.repositories 镜像顺序
```kotlin
maven { url = uri("https://maven.aliyun.com/repository/google") }
if (System.getenv("CI") == null) {
    maven { url = uri("https://mirrors.tencent.com/nexus/repository/maven-public/") }
}
maven { url = uri("https://mirrors.tencent.com/repository/maven-tencent/") }
maven { url = uri("https://maven.aliyun.com/repository/public") }
google()
mavenCentral()
```

## UI 交互铁律（重要！）

- **严禁自动 fallback**：用户选择的功能（如 MPV 播放器）不可用时，**禁止**静默切换到其他方案（如 Artplayer）。必须在用户选择时就明确告知不可用（禁用选项、状态标签），让用户主动选择替代方案。
- **严禁 Toast 提示**：Toast 是临时性、易被忽略的提示，不符合饱和调试原则。状态信息必须通过持久性 UI 元素显示（如选项旁的状态标签、设置页面的状态指示器）。
- **正确做法**：在设置页面播放器选项旁显示插件状态（未安装/已禁用/已加载），不可用时禁用该选项或显示警告标签，让用户明确知道当前状态并主动选择其他播放器。

## Jetpack Compose 编码规范

- **权威参照文件**：[compose-reference.md](.trae/rules/compose-reference.md)（Android 官方文档摘录 + 本项目已验证代码）
- **State `by` 委托必须同时 import**：`androidx.compose.runtime.getValue` + `androidx.compose.runtime.setValue`（缺一不可）
- **Material Icons Extended 包路径**：`Icons.Outlined.XXX`（**大写 O**），不是小写 `outlined`
- **本项目"金标准"文件**（已在 CI 编译通过，写新代码前必须参照其 import 风格和 API 用法）：
  - `plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvPlayerScreen.kt`
  - `plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvProgressBar.kt`
  - **写完任何 .kt Compose 文件后**：对照 compose-reference.md 逐条检查 import 完整性和 API 正确性

## 防御性编程铁律（重要！违反 = 严重错误）

> 来自实战踩坑：硬编码 fallback 导致 API 失效时放行危险操作。

### 一、禁止硬编码动态数据

**任何由后端 API 返回的运行时数据，前端不得维护本地副本或 fallback 默认值。**

| 数据类型 | 示例 | 正确做法 |
|---------|------|---------|
| 容器扩展名映射 | `.sccgv → video` | 始终从 `/api/plugins/container-extensions` 获取 |
| 插件配置 schema | 字段名/类型/默认值 | 从 `/api/config/schema` 动态加载 |
| 文件类型 → 插件路由 | `.ts → text/video` | 由后端 `SupportedExtensions()` + MIME 检测决定 |
| 加密算法列表 | AES/ChaCha20 等 | 后端注册表驱动 |

**错误模式（禁止）**：
```typescript
// ❌ 硬编码 fallback — 新增插件后此处过时
const FALLBACK: Record<string, string> = { '.sccgv': 'video', '.sccga': 'audio' }
return data?.extensions ?? FALLBACK
```

**正确做法**：API 未就绪时返回特殊标记值，触发阻断行为（见下文）。

### 二、不确定时阻断，不猜测（Fail-Safe 原则）

当验证逻辑依赖的数据源不可用时（API 404/超时/未初始化），**必须阻断危险操作而非放行**。

```
数据源状态        验证结果          操作允许？
─────────       ────────         ───────
✅ API 正常      返回 ["video"]    按 conflict 判断
⚠️ API 未加载   返回 [UNAVAILABLE]  🚫 禁用保存
❌ API 失败     返回 [UNAVAILABLE]  🚫 禁用保存
```

**实现模式**：

```typescript
const UNAVAILABLE = '__unavailable__'

function getConflictingPlugins(suffix): string[] {
  if (!data.value) return [UNAVAILABLE]  // data=null 时返回标记值
  // ... 正常检测逻辑
}

// PluginSettings.vue 保存按钮
:disabled="configLoading || suffixConflict.length > 0"
// UNAVAILABLE.length === 1 > 0 → 自动禁用 ✅
```

**UI 区分两种阻断原因**：
- **已知冲突**（橙色）：`.sccgv 与 video 冲突`
- **验证不可用**（红色）：无法验证唯一性（API 不可用），为防止冲突暂不允许保存

### 三、三层防御架构

本项目的输入校验遵循「前端实时提示 → API 层拦截(400) → 启动时检测(Error日志)」三层防御。每一层都必须独立完整，不得假设上层已拦截：

| 层级 | 触发时机 | 行为 |
|------|---------|------|
| L1 前端 | 用户输入即时 | disabled 保存按钮 + 警告文案 |
| L2 API | PUT /api/config | `validateContainerExtensionsInConfig()` 返回 400 + 错误信息 |
| L3 启动 | `InitializePlugins()` | slog.Error 日志 + 继续启动不 abort |

**关键约束**：L2 和 L3 不得依赖 L1 的存在。用户可能通过第三方编辑器直接修改 config JSON 绕过前端。

## Trae Web 沙箱前端访问规则（重要！）

> **铁律：云端沙箱只能通过 agent-tool-host 代理访问前端，严禁混淆端口身份**

### 端口身份（不可混淆）

| 端口 | 进程 | 身份 |
|------|------|------|
| **5173** (或动态分配) | `agent-tool-host` (`/app/bin/agent-tool-host`) | **前端 HTTP 代理** — 用户浏览器实际访问的入口，等价于 vite dev server |
| **5174/5175/...** (vite 动态分配) | `node .../vite` | Vite dev server 原始端口 — agent-tool-host 反向代理到此 |

### 关键认知

1. **agent-tool-host 就是前端服务**：用户通过 OpenPreview 或浏览器访问的 URL 指向的端口就是 agent-tool-host，它代理了 Vite HMR
2. **Vite 端口可能漂移**：当 5173 被占用时 vite 会尝试 5174、5175… 但用户始终通过 agent-tool-host 访问
3. **`lsof -i :5173` 看到 agent-tool-host 是正常的**，不代表"vite 没在运行"
4. **禁止将 agent-tool-host 进程误判为非前端服务**：这会导致错误结论如"你访问的不是新代码"

### 验证代码是否生效的正确方法

```bash
# 1. 确认 vite 在运行（可能在 5174+）
lsof -i :5173 -i :5174 -i :5175 -t | xargs ps -p -o command= 2>/dev/null

# 2. 通过 agent-tool-host 端口验证源码内容
curl -s http://localhost:5173/src/views/Tasks.vue | grep "predictPlugin"

# 3. 如果 agent-tool-host 不在 5173，用 OpenPreview 获取的实际 URL
```

### 禁止行为

- ❌ 看到 `agent-tool-host` 就断言"这不是 vite / 这不是前端"
- ❌ 杀掉 agent-tool-host 进程（这是沙箱基础设施）
- ❌ 让用户访问 vite 原始端口而非 agent-tool-host 代理端口

## 测试覆盖铁律（重要！违反 = 严重失职）

> **任何涉及 UI 状态变更的逻辑必须有对应测试覆盖，不允许依赖手动浏览器验证**

### 必须测试的场景

| 场景类型 | 示例 | 测试方式 |
|---------|------|---------|
| 路由跳转 + Modal 打开 | Files → Tasks 新建任务 | 单元测试 mock router + 断言 modal state |
| API 调用触发时机 | processQueryAction 中 predictPlugin 是否被调用 | spy/predictPlugin mock + 断言调用次数 |
| computed 派生状态 | candidates 变化 → predictedPlugin 自动更新 | 设置 candidates → 断言 predictedPlugin 值 |
| 表单重置逻辑 | validateSourcePath 错误路径清空状态 | 触发错误条件 → 断言所有相关 ref 已重置 |
| 条件渲染 | passwordStrategy=independent 时字段显隐 | 设置不同 strategy → 断言 DOM 元素存在性 |

### 测试优先级

1. **路由/导航逻辑** — 最高优先级（modal 不打开 = 功能完全不可用）
2. **API 调用链** — 高优先级（数据不加载 = UI 为空）
3. **computed 派生** — 中优先级（显示错误但可排查）
4. **样式/CSS** — 低优先级（视觉问题不影响功能）
