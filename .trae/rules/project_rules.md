# 项目规则

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

## Lynx NativeModules 访问规则（重要！）

- **禁止使用 `globalThis.NativeModules`**，必须使用 `declare let NativeModules` 声明后直接用 `NativeModules.XXX`
- 原因：ReactLynx 双线程架构中，`globalThis.NativeModules` 在主线程/Lepus 线程中被设为 `undefined`
- NativeModules 只能在**后台线程**上下文中调用：
  - `useEffect` / `useLayoutEffect` hooks
  - 事件处理函数（bindtap 等）
  - `'background only'` 指令标记的函数
- **禁止在组件渲染阶段（函数体顶层）调用 NativeModules**
- 模块名必须与 Android 端 `registerModule()` 的第一个参数一致（如注册 "LogBridge" 则 JS 用 `NativeModules.LogBridge`，不是 `NativeModules.LogBridgeModule`）
- 类型声明在 `lynx-player/src/typing.d.ts`
- 构建脚本会检查 `globalThis.NativeModules` 和 `globalThis.lynx.getJSModule` 用法，发现则构建失败

## Lynx 全局事件监听规则（重要！）

- **禁止使用 `globalThis.lynx.getJSModule('GlobalEventEmitter')`**，必须使用 `useLynxGlobalEventListener` hook
- 原因：`globalThis.lynx.getJSModule` 在后台线程中不可用，返回 null
- `useLynxGlobalEventListener` 是 ReactLynx 官方 hook，内部使用 `lynx.getJSModule`（注意是 `lynx` 全局变量，不是 `globalThis.lynx`），并通过 `useMemo` 尽早注册监听器
- 用法：`useLynxGlobalEventListener('event-name', useCallback((event) => { ... }, [deps]))`
- 用法：`import { useLynxGlobalEventListener } from '@lynx-js/react'`

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
