# Go 移动端二进制体积优化计划

## 现状

- **构建命令**：`GOOS=android GOARCH=arm64 CGO_ENABLED=1 go build -ldflags="-s -w" -o encv-go-arm64 ./cmd/encv`
- **当前体积**：约 3.8MB（CI 构建输出，已使用 `-s -w` 剥离调试符号）
- **入口点**：`./cmd/encv` — 一个 cobra CLI 程序，移动端仅使用 `start` 子命令
- **移动端 API**：`pkg/encv` 包（Init、NewServer、FindServer、ParseServerFlags）
- **核心问题**：CLI 入口点将大量桌面端/CLI 专用依赖编译进移动端二进制

---

## 影响因素前十名

### 1. 🏆 gin 框架及其传递依赖链（预估贡献 ~1.5-2MB）

**依赖链**：
```
gin-gonic/gin
  ├── bytedance/sonic（高性能 JSON，含汇编优化）
  │     ├── bytedance/gopkg
  │     ├── bytedance/sonic/loader
  │     └── klauspost/cpuid/v2
  ├── json-iterator/go（另一 JSON 库，gin 双引擎）
  │     └── modern-go/reflect2 + concurrent
  ├── ugorji/go/codec（Codec 编码）
  ├── go-playground/validator/v10（请求校验）
  │     ├── go-playground/locales
  │     ├── go-playground/universal-translator
  │     └── leodido/go-urn
  ├── gabriel-vasile/mimetype（MIME 检测）
  ├── gin-contrib/sse（Server-Sent Events）
  └── goccy/go-json（又一个 JSON 库）
```

**问题**：gin 为高性能场景引入了 3 个 JSON 库（sonic + json-iterator + goccy/go-json）、汇编优化代码、完整的请求校验框架。移动端仅用 gin 做 localhost API 路由，不需要这些重量级特性。

**优化思路**：
- **方案 A**（推荐）：移动端替换为轻量路由器（如 `chi`、`mux`、或 `net/http` 标准库），gin 仅在桌面端使用
- **方案 B**：通过 build tag 将 gin_app.go 拆分为移动端/桌面端实现
- **方案 C**：gin 编译时禁用 sonic（`gin` 支持 `nosonic` build tag），但 json-iterator/ugorji 仍会被拉入

---

### 2. 🥈 pterm 终端 UI 库（预估贡献 ~800KB-1MB）

**依赖链**：
```
pterm/pterm
  ├── atomicgo.dev/cursor
  ├── atomicgo.dev/keyboard
  ├── atomicgo.dev/schedule
  ├── containerd/console
  ├── gookit/color
  ├── lithammer/fuzzysearch
  ├── mattn/go-runewidth
  ├── mattn/go-isatty
  ├── xo/terminfo
  ├── clipperhouse/uax29/v2（Unicode 分词）
  └── golang.org/x/term
```

**问题**：`pterm` 仅在 `internal/utils/terminal.go` 中使用，提供 `PrintSuccess`/`PrintError`/`PrintInfo`/`PrintSection`/`NewSpinner`/`PrintTable`/`PrintBox`/`PrintKV` 等终端美化输出函数。这些函数**仅在 CLI 模式下被调用**（`cmd/encv/main.go` 和 `cmd/encv/servers.go`），移动端完全不需要终端输出。

**优化思路**：
- **方案 A**（推荐）：为 `internal/utils/terminal.go` 添加 build tag 拆分
  - `terminal.go` → `//go:build !android`（完整 pterm 实现）
  - `terminal_mobile.go` → `//go:build android`（空实现/stub，用 `slog` 替代）
- **方案 B**：将 `terminal.go` 的调用方（`main.go`、`servers.go`）也用 build tag 隔离，因为整个 CLI 入口在移动端不使用

---

### 3. 🥉 cobra CLI 框架（预估贡献 ~300-500KB）

**依赖链**：
```
spf13/cobra
  ├── spf13/pflag
  └── inconshreveable/mousetrap（Windows）
```

**问题**：`cobra` 仅在 `cmd/encv/main.go` 及相关 cmd 文件中使用，提供 CLI 命令解析。移动端通过 `pkg/encv` API 直接调用 `NewServer()`，不需要 CLI 解析。

**优化思路**：
- **方案 A**（推荐）：创建独立的移动端入口点 `cmd/encv-mobile/main.go`，直接调用 `pkg/encv` API，不引入 cobra
- **方案 B**：在现有 `cmd/encv/main.go` 中用 build tag 区分移动端/桌面端入口

---

### 4. go-exif/v3 EXIF 元数据解析（预估贡献 ~500-800KB）

**依赖链**：
```
dsoprea/go-exif/v3
  ├── dsoprea/go-logging
  ├── dsoprea/go-utility/v2
  ├── golang/geo（地理坐标计算）
  └── go-errors/errors
```

**问题**：`go-exif` 仅在 `internal/v2/plugins/image/metadata_extractor.go` 和 `internal/v2/plugins/image/types.go` 中使用，用于提取图片 EXIF 元数据。`golang/geo` 是一个较大的地理计算库，移动端图片预览场景可能不需要完整的 EXIF 解析。

**优化思路**：
- **方案 A**：通过 build tag 将 EXIF 解析标记为桌面端功能
- **方案 B**：延迟加载 — 仅在用户请求 EXIF 信息时才导入，通过接口解耦
- **方案 C**：替换为更轻量的 EXIF 库（如 `drewolson/go-mediainfo` 或简化版解析）

---

### 5. go-mp4 MP4 结构解析（预估贡献 ~400-600KB）

**使用位置**：`internal/v2/plugins/video/content_verifier.go`

**问题**：`go-mp4` 用于解析 MP4 容器结构，验证视频内容完整性。移动端播放场景可能不需要内容验证。

**优化思路**：
- **方案 A**：通过 build tag 将视频内容验证标记为可选功能
- **方案 B**：移动端跳过 content verification，仅桌面端执行

---

### 6. 间接传递依赖：mongo-driver + golang-asm + quic-go（预估贡献 ~1-1.5MB）

**依赖链**（需 `go mod why` 确认具体路径）：
```
某直接依赖 → go.mongodb.org/mongo-driver/v2
                ├── twitchyliquid64/golang-asm（汇编优化 BSON 编码）
                └── quic-go/quic-go + qpack（QUIC 传输协议）
```

**问题**：这三个库在项目 Go 源码中**无任何直接 import**，完全是传递依赖。`mongo-driver` 是 MongoDB 客户端（含完整网络协议栈），`golang-asm` 是汇编代码生成器，`quic-go` 是完整的 QUIC 协议实现。移动端不使用 MongoDB。

**优化思路**：
- **第一步**：运行 `go mod why go.mongodb.org/mongo-driver/v2` 确认引入链
- **第二步**：在引入链的某个环节用接口/build tag 断开依赖
- **可能的引入者**：`goccy/go-yaml`（间接依赖，可能通过 `invopop/jsonschema` → 但 `jsonschema` 仅在 `cmd/encv-schema/` 中使用，不应进入 `cmd/encv` 构建路径）
- **关键排查**：如果这些依赖确实通过 `cmd/encv` 的代码路径引入，必须找到并断开

---

### 7. WebDAV 支持（golang.org/x/net/webdav）（预估贡献 ~300-500KB）

**使用位置**：`internal/webdav/fs_v2.go`、`internal/server/server.go`

**问题**：WebDAV 是完整的 Web 文件系统协议，移动端不需要远程文件挂载功能。`golang.org/x/net` 本身也较大。

**优化思路**：
- **方案 A**：移动端 build tag 排除 WebDAV 相关代码
- **方案 B**：将 WebDAV 作为插件/可选模块，通过接口解耦

---

### 8. cbor/v2 二进制编码（预估贡献 ~200-300KB）

**使用位置**：`internal/v2/types/header_v3.go`、`internal/v2/types/container_patch.go`、`internal/v2/container/detector/analyze.go`

**问题**：CBOR 是容器格式的核心编码，无法简单移除。但可以评估是否有更轻量的实现。

**优化思路**：
- **方案 A**：评估 `fxamacker/cbor/v2` 是否有更精简的 API 子集可用
- **方案 B**：如果容器 v3 格式在移动端不使用，可通过 build tag 排除

---

### 9. OpenList 代理 + embed.FS 嵌入资源（预估贡献 ~100-200KB）

**使用位置**：
- `internal/openlist/` — OpenList 远程站点代理
- `internal/openlist/web/preview.go` — `//go:embed static/preview/*`（text.html + pdf.html）

**问题**：OpenList 代理功能在移动端可能不需要。嵌入的 HTML 预览文件虽然体积小，但 `embed.FS` 基础设施本身有开销。

**优化思路**：
- **方案 A**：移动端 build tag 排除 OpenList 代理功能
- **方案 B**：预览 HTML 改为运行时从 assets 加载（Android 已有 assets 机制），不使用 `//go:embed`

---

### 10. CLI 专用入口点耦合（架构级问题，影响以上所有因素）

**问题**：当前移动端和桌面端共用 `cmd/encv/main.go` 入口点。这个入口点：
1. 导入 `cobra`（CLI 框架）
2. 定义所有 CLI 子命令（analyze/manifest/kvi/decrypt/encrypt/play/start）
3. `start` 命令中调用 `utils.PrintSection/PrintKV`（拉入 pterm）
4. 其他命令（analyze/manifest/kvi/decrypt/encrypt/play）在移动端完全无用

移动端实际调用路径：`Android JNI → libencv-go.so → NewServer() → Start()`，根本不经过 `main()`。

**优化思路**：
- **方案 A**（推荐）：创建 `cmd/encv-mobile/main.go` 作为移动端专用入口
  ```go
  //go:build android

  package main

  import (
      "context"
      "os"
      "github.com/Soltus/encv-go/internal/config"
      "github.com/Soltus/encv-go/pkg/encv"
  )

  var Version = "dev"

  // 移动端不需要 main()，由 JNI 直接调用导出函数
  func main() {
      // 仅作为 fallback，正常不执行
  }
  ```
- **方案 B**：将 `cmd/encv/` 下的 CLI 代码全部加 `//go:build !android` tag

---

## 综合优化方案

### 阶段一：快速见效（预估减少 30-40%）

1. **创建移动端专用入口** `cmd/encv-mobile/`
   - 不导入 cobra、pterm
   - 直接暴露 `NewServer()` / `Start()` 给 JNI
   - 修改 CI 构建命令为 `./cmd/encv-mobile`

2. **terminal.go build tag 拆分**
   - `terminal.go` → `//go:build !android`
   - `terminal_mobile.go` → `//go:build android`（空实现或 slog 替代）

3. **排查并断开 mongo-driver/quic-go/golang-asm 依赖链**
   - 运行 `go mod why` 确认引入路径
   - 在引入点用接口/build tag 断开

### 阶段二：深度优化（预估再减少 20-30%）

4. **gin 替换为轻量路由器**
   - 移动端使用 `net/http` 标准库 + 轻量中间件
   - 或使用 `chi`（~200KB vs gin ~1.5MB 传递依赖）
   - 通过 build tag 选择实现

5. **WebDAV + OpenList 移动端排除**
   - `webdav/fs_v2.go` → `//go:build !android`
   - `openlist/` 相关代码 → `//go:build !android`
   - server.go 中 WebDAV/OpenList 路由注册 → build tag 拆分

6. **go-exif/go-mp4 可选化**
   - 图片 EXIF 和视频内容验证标记为桌面端功能
   - 或通过插件机制延迟加载

### 阶段三：极致优化（预估再减少 10-15%）

7. **embed.FS 替换为 Android assets**
   - 预览 HTML 从 Android assets 加载，不嵌入 Go 二进制

8. **CBOR 编码评估**
   - 如果 v3 容器格式移动端不使用，build tag 排除

9. **UPX 压缩**
   - 对最终二进制使用 UPX 压缩（可减少 50-70% 体积）
   - 注意：Android 上 UPX 压缩的 .so 可能存在兼容性问题，需测试

---

## 实施步骤

### Step 1：创建移动端入口点
- 新建 `cmd/encv-mobile/main.go`
- 仅导入 `pkg/encv` 和 `internal/config`
- 修改 `.github/workflows/android.yml` 构建目标

### Step 2：terminal.go build tag 拆分
- 重命名 `internal/utils/terminal.go` → 添加 `//go:build !android`
- 新建 `internal/utils/terminal_mobile.go` → `//go:build android`

### Step 3：排查间接依赖链
- CI 中运行 `go mod why go.mongodb.org/mongo-driver/v2`
- CI 中运行 `go mod why github.com/quic-go/quic-go`
- 根据结果在适当位置断开依赖

### Step 4：server.go build tag 拆分
- 将 WebDAV/OpenList 路由注册拆分为独立文件
- 移动端版本仅注册核心 API 路由

### Step 5：gin 替换（移动端）
- 新建 `internal/server/router_mobile.go` → `//go:build android`
- 使用 `net/http` 标准库实现移动端路由
- 现有 `gin_app.go` → 添加 `//go:build !android`

### Step 6：验证
- CI 构建后对比二进制体积
- 运行 `go tool nm` 确认无多余符号
- 功能回归测试

---

## 预期效果

| 阶段 | 预估体积 | 减少比例 |
|------|---------|---------|
| 当前 | ~3.8MB | — |
| 阶段一完成后 | ~2.3-2.7MB | 30-40% |
| 阶段二完成后 | ~1.5-1.9MB | 50-60% |
| 阶段三完成后 | ~1.0-1.5MB | 60-75% |

> 注：以上为估算值，实际效果需在 CI 构建后测量。`go tool nm` 和 `go build -gcflags=-S` 可用于精确分析符号和代码段大小。
