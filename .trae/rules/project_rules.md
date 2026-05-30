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
- **不得创建独立的 `config.mobile.json` 或其他平台特定配置模板**：移动端适配通过 Go 端 `Load()` 中的 `ENCV_MOBILE` 路径自动修正实现
- 违反此规则导致的配置模板破坏将被视为严重错误

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
