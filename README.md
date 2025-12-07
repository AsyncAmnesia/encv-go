# encv-go 怡念汐拂

[![zread](https://img.shields.io/badge/Ask_Zread-_.svg?style=flat&color=00b0aa&labelColor=000000&logo=data%3Aimage%2Fsvg%2Bxml%3Bbase64%2CPHN2ZyB3aWR0aD0iMTYiIGhlaWdodD0iMTYiIHZpZXdCb3g9IjAgMCAxNiAxNiIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTQuOTYxNTYgMS42MDAxSDIuMjQxNTZDMS44ODgxIDEuNjAwMSAxLjYwMTU2IDEuODg2NjQgMS42MDE1NiAyLjI0MDFWNC45NjAxQzEuNjAxNTYgNS4zMTM1NiAxLjg4ODEgNS42MDAxIDIuMjQxNTYgNS42MDAxSDQuOTYxNTZDNS4zMTUwMiA1LjYwMDEgNS42MDE1NiA1LjMxMzU2IDUuNjAxNTYgNC45NjAxVjIuMjQwMUM1LjYwMTU2IDEuODg2NjQgNS4zMTUwMiAxLjYwMDEgNC45NjE1NiAxLjYwMDFaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00Ljk2MTU2IDEwLjM5OTlIMi4yNDE1NkMxLjg4ODEgMTAuMzk5OSAxLjYwMTU2IDEwLjY4NjQgMS42MDE1NiAxMS4wMzk5VjEzLjc1OTlDMS42MDE1NiAxNC4xMTM0IDEuODg4MSAxNC4zOTk5IDIuMjQxNTYgMTQuMzk5OUg0Ljk2MTU2QzUuMzE1MDIgMTQuMzk5OSA1LjYwMTU2IDE0LjExMzQgNS42MDE1NiAxMy43NTk5VjExLjAzOTlDNS42MDE1NiAxMC42ODY0IDUuMzE1MDIgMTAuMzk5OSA0Ljk2MTU2IDEwLjM5OTlaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik0xMy43NTg0IDEuNjAwMUgxMS4wMzg0QzEwLjY4NSAxLjYwMDEgMTAuMzk4NCAxLjg4NjY0IDEwLjM5ODQgMi4yNDAxVjQuOTYwMUMxMC4zOTg0IDUuMzEzNTYgMTAuNjg1IDUuNjAwMSAxMS4wMzg0IDUuNjAwMUgxMy43NTg0QzE0LjExMTkgNS42MDAxIDE0LjM5ODQgNS4zMTM1NiAxNC4zOTg0IDQuOTYwMVYyLjI0MDFDMTQuMzk4NCAxLjg4NjY0IDE0LjExMTkgMS42MDAxIDEzLjc1ODQgMS42MDAxWiIgZmlsbD0iI2ZmZiIvPgo8cGF0aCBkPSJNNCAxMkwxMiA0TDQgMTJaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00IDEyTDEyIDQiIHN0cm9rZT0iI2ZmZiIgc3Ryb2tlLXdpZHRoPSIxLjUiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIvPgo8L3N2Zz4K&logoColor=ffffff)](https://zread.ai/Soltus/encv-go)

一个基于 `AES-256-CTR` 的强大命令行工具集/库，用于多种类型文件的加密、解密、流传输，并提供与 OpenList 的无缝代理集成与通用 HTTP / Webdav 服务。90%代码由AI提供。

> [!WARNING]
> 请确保 `ffmpeg` 已安装并添加到系统的环境变量中。

---

## 📖 用户使用指南

本部分将指导您如何安装、配置和使用 **怡念汐拂** 的各项功能。

### 🚀 安装

如果没有可执行程序资产，您需要从源代码构建 `encv` 和 `encv-proxy` ，通常这不需要。

```bash
# 构建 encv 主程序
go build -o encv ./cmd/encv
# 构建 encv-proxy 代理程序
go build -o encv-proxy ./cmd/encv-proxy
```

### ⚙️ 配置

`encv` 和 `encv-proxy` 共享一个配置文件 `config.user.json`。将其放置在可执行文件同级目录下，可以避免每次手动输入参数。

**`config.user.json` 示例:**

```json
{
  "$schema": "https://raw.githubusercontent.com/Soltus/encv-go/main/config.schema.json",
  "password": "my-encv_key，可以使用中文和标点符号✔",
  "output_path": "./output",
  "server": {
    "port": 2025,
    "dir": "/"
  },
  "proxy": {
    "port": 1998,
    "openlist_host": "http://localhost:5244"
  },
  "webdav": {
    "port": 1234,
    "root": "webdav",
    "dir": "./"
  },
  "plugin_settings": {
    "video": {
      "ext": ".sccgv",
      "chunk_size_mb": 100,
      "track_extensions": [".ass", ".srt", ".dm.ass", ".vtt"]
    },
    "image": {"ext": ".sccgi"},
    "audio": {"ext": ".sccga"},
    "text": {"ext": ".sccgt"},
    "wps": {"ext": ".sccgwps"},
    "pdf": {"ext": ".sccgpdf"}
  }
}

```

**配置项说明:**

根据 `config.schema.json` 生成。

#### 全局设置

| 配置键 | 类型 | 描述 | 示例值 |
|---|---|---|---|
| **password** | `string` | 用于加密和解密文件的主密码，请务必设置一个强密码。支持中文和特殊字符。 | `"my-encv_key，可以使用中文和标点符号✔"` |
| **output_path** | `string` | 加密后文件的输出目录，支持相对路径或绝对路径。 | `"./output"` |
| **recover** | `boolean` | 解密时是否覆盖已存在的文件。 | `false` |

#### 插件专属设置

| 配置键 | 类型 | 描述 | 示例值 |
|---|---|---|---|
| **plugin_settings.video.ext** | `string` | 视频类加密容器的扩展名。 | `".sccgv"` |
| **plugin_settings.video.chunk_size_mb** | `integer` | 视频分片大小（MB），0 表示禁用分片。 | `100` |
| **plugin_settings.video.light_main_chunk_enabled** | `boolean` | 是否启用轻量主分片模式。 | `true` |
| **plugin_settings.video.track_extensions** | `array` | 需要关联处理的字幕/轨道文件扩展名列表。 | `[".ass", ".srt", ".dm.ass", ".vtt"]` |
| **plugin_settings.image.ext** | `string` | 图像类加密容器的扩展名。 | `".sccgi"` |
| **plugin_settings.audio.ext** | `string` | 音频类加密容器的扩展名。 | `".sccga"` |
| **plugin_settings.text.ext** | `string` | 文本类加密容器的扩展名。 | `".sccgt"` |
| **plugin_settings.pdf.ext** | `string` | PDF类加密容器的扩展名。 | `".sccgpdf"` |
| **plugin_settings.wps.ext** | `string` | WPS类加密容器的扩展名。 | `".sccgwps"` |

#### 内置 HTTP 服务器设置

| 配置键 | 类型 | 描述 | 示例值 |
|---|---|---|---|
| **server.port** | `integer` | encv HTTP 流媒体服务器的监听端口，请勿与其他服务冲突。 | `2025` |
| **server.dir** | `string` | HTTP 服务器提供的文件系统根目录，支持相对路径。 | `"/"` |

#### OpenList 代理服务器设置

| 配置键 | 类型 | 描述 | 示例值 |
|---|---|---|---|
| **proxy.port** | `integer` | OpenList 代理服务的监听端口，请勿与其他服务冲突。 | `1998` |
| **proxy.openlist_host** | `string` | OpenList 服务的主机地址，支持协议和端口。 | `"http://localhost:5244"` |
| **proxy.token** | `string` | OpenList 的认证令牌（可选，不建议明文存储）。 | `""` |
| **proxy.disable_signature_verification** | `boolean` | 是否禁用 OpenList URL 签名验证。 | `false` |

#### WebDAV 服务器设置

| 配置键 | 类型 | 描述 | 示例值 |
|---|---|---|---|
| **webdav.port** | `integer` | encv WebDAV 服务器的监听端口，请勿与其他服务冲突。 | `1234` |
| **webdav.root** | `string` | WebDAV 服务的访问路径前缀（路由）。 | `"webdav"` |
| **webdav.dir** | `string` | WebDAV 服务映射的文件系统根目录，支持相对路径。 | `"./"` |

### 💻 核心用法

#### 加密视频

将指定目录下的所有视频文件加密，并处理相关字幕文件。

```bash
# 将 _videos 目录下的所有视频加密到 output 目录
./encv encrypt -o ./output ./_videos
```

#### 解密视频

将加密后的文件恢复为原始格式。

> [!NOTE]
> Go 标准库 `flag` 包的行为：所有 `-` 开头的标志参数（如 `-o`）必须放在位置参数（如文件路径）**之前**。

```bash
# 解密单个文件
./encv decrypt -o ./_decrypt/ "./output/sample.sccgv"
# 解密整个目录下的所有 .sccgv 文件
./encv decrypt -o ./_decrypt/ ./output
```

#### 查看 kvi 信息

> [!NOTE]
> Go 标准库 `flag` 包的行为：所有 `-` 开头的标志参数（如 `-s`）必须放在位置参数（如文件路径）**之前**。

```bash
# 基础用法
./encv kvi "./output/movie.sccgv"
# 保存到文件
./encv kvi -s kvi.json "./output/movie.sccgv"
```

#### HTTP服务

启动一个 HTTP 服务器，支持集成 Webdav 服务。

```bash
# 在 1999 端口启动服务，提供 ./output 目录下的文件
./encv start -port 1999 ./output
```

启用 HTTP 服务后，可以通过 URL 播放服务的路径和磁盘任意位置的加密视频。

HTTP 服务（`./encv server`）和 Webdav 服务（`./encv webdav`）也可以单独启动，但是不支持多实例。

> [!WARNING]
> 使用 `mpv` 播放器观看时，需要手动指定字幕文件

```bash
mpv http://localhost:1999/321.sccgv --sub-files=http://localhost:1999/321.ass
```

播放任意位置的视频：

```bash
mpv http://127.0.0.1:1999/stream?file=A%3A%5CLocal%5CCol-Study%5Cgo%5Cencv%5Coutput%5C321.4pm.sccgv
```

### 🔗 OpenList 集成

`encv-proxy` 是一个专为 OpenList 设计的代理服务，它能透明地解密 encv 加密容器，让 OpenList 可以直接播放加密内容。

> [!WARNING]
> 由于远程服务的特性，OpenList 集成不支持真正的视频寻址，只能发起 Range 请求，但是文件由 OpenList 提供，因此需要先加载对应的物理分片。

**配置步骤:**

1. **构建 `encv-proxy`** (见上文安装部分)。
2. **在 OpenList 中配置 WebDAV**:

   * 进入管理页面，找到存储设置。
   * 在【WebDAV 策略】中，选择 **使用代理地址**。
   * URL 地址填入 `http://localhost:1998` (根据你的 `proxy_port` 修改)。
3. **获取 OpenList 令牌**:

   * 在 OpenList 管理页面的【设置】->【其他】中，滚动到最底部，复制管理员令牌。
4. **启动 `encv-proxy`**:

   * **推荐方式 (命令行指定令牌)**:

     ```bash
     ./encv-proxy -token "openlist-***********************************"
     ```

   * **完整命令行参数**:

     ```bash
     ./encv-proxy -proxy-port 1998 -openlist-host "http://localhost:5244" -token "openlist-***********************************"
     ```

5. **为加密文件添加预览**:

   * 在 OpenList 管理页面的【设置】->【预览】中。
   * 根据配置项中的 `bin_ext_group` 按类别添加后缀名：

   ```json
   "bin_ext_group": {
       "text": "sccgt",
       "image": "sccgi",
       "audio": "sccga",
       "video": "sccgv"
     }
   ```

6. **通过 iframe 预览文件**：

   * 在 Openlist 管理页面的【设置】->【预览】中，iframe 预览追加以下配置：

     ```json
     "sccgpdf": {
       "ENCV PDF": "http://localhost:1998/_preview/pdf.html?file=$e_url",
     },
     "sccgt": {
        "ENCV Text": "http://localhost:1998/_preview/text.html?file=$e_url"
      }
     ```

   * 保存后即可在 OpenList 中预览加密后的 PDF 和文本文件。

   * 文本文件为什么需要 iframe 预览？因为测试过程中发现 OpenList 无法预览 50MB 的大型文本文件，而通过 encv 的 iframe 预览则没有问题。

OpenList Webdav 代理是很好的方式，只需要定义预览后缀名，不影响其他操作。但假如希望在其他平台通过 Webdav 预览加密容器，encv-go 也提供了支持，缺点是仅支持只读模式，而且性能可能远不如 OpenList 。通用 Webdav 服务将显示解密后的原始文件名作为“不存在”的文件，当请求打开时反查真实的加密容器并进行解密。**通用 Webdav 集成在 `encv` 主程序，`encv-proxy` 程序不提供**。

调试通用 webdav

```bash
curl -v -X PROPFIND http://localhost:2025/webdav/ -H "Depth: 1" -H "Content-Type: application/xml" -d '<?xml version="1.0" encoding="utf-8"?><D:propfind xmlns:D="DAV:"><D:prop><D:displayname/></D:prop></D:propfind>'
```

### Windows 上使用

`register` 命令可以注册加密容器后缀名的右键菜单，`unregister` 反注册。`openas` 命令注册容器后缀名打开方式（双击打开）

### Linux 上使用

#### 安装 ffmpeg

如果还没安装 ffmpeg

1. 下载FFmpeg压缩包

```bash
wget https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz
```

2. 解压文件

```bash
tar -xf ffmpeg-master-latest-linux64-gpl.tar.xz
```

3. 移动到系统目录

```bash
sudo mv ffmpeg-master-latest-linux64-gpl /opt/ffmpeg
```

4. 添加到PATH环境变量

```bash
echo 'export PATH="/opt/ffmpeg/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc  # 立即生效
```

WSL2 已经打通了 localhost

如果 localhost 不行，尝试在 WSL 终端中运行命令 `hostname -I`，获取 WSL 虚拟机的 IP 地址

---

## 🛠️ 开发者指南

本部分面向希望参与项目开发或进行调试的开发者。

### 📁 项目结构

```md
encv-go/
├── cmd/                  # 程序入口
│   ├── encv/             # encv 程序
│   └── encv-proxy/       # encv-proxy 代理程序（只代理 OpenList）
│   └── encv-schema/      # 仅用于生成 `config.schema.json`
├── internal/             # 内部包，不对外暴露
│   ├── config/           # 配置
│   ├── container/        # 加密容器相关
│   ├── crypto/           # 加解密核心逻辑
│   ├── middleware/       # 中间件
│   ├── packer/           # 加密打包
│   ├── postdecrypt/      # 解密后处理
│   ├── processor/        # 加密前分析，预处理和元数据处理
│   ├── proxy/            # OpenList 代理服务核心逻辑
│   ├── server/           # HTTP服务核心逻辑
│   ├── service/          # 加解密服务编排
│   └── types/            # 共享的数据结构定义
│   ├── unpacker/         # 解密解包
│   └── utils/            # 通用工具
│   └── webdav/           # Webdav服务核心逻辑
├── pkg/                  # 对外暴露的公共包，作为库调用
│   └── encv/
│      └── api.go
│      └── decrypt.go
│      └── encrypt.go
│      └── kvi.go
│      └── server.go
├── go.mod
└── README.md
└── config.user.json
└── config.schema.json
```

### 🔨 构建

```bash
# 构建 encv 主程序
go build -o encv ./cmd/encv
# 构建 encv-proxy 代理程序
go build -o encv-proxy ./cmd/encv-proxy
```

#### 构建 Android AAR (占位)

> [!NOTE]
> 此功能待后续实现。

```bash
gomobile bind -target=android -o encv.aar ./pkg/encv
```

### 🧪 测试

单元测试

```cmd
# 运行 service 包下的所有测试
go test ./internal/service -v

# 只运行特定测试
go test ./internal/service -run="TestContinuousRead|TestRandomSeek" -v
```

一个简单的测试流程是加密一个示例视频目录：

```bash
# 假设你有一个 _videos 目录
./encv encrypt -o ./_test_output ./_videos
```

然后检查 `_test_output` 目录下的 `.sccgv`, `.kvi` 和字幕文件是否符合预期。

生成 `config.schema.json` ：

```cmd
go run ./cmd/encv-schema > config.schema.json
```

在 `config.user.json` 中测试：

```json
"$schema": "./config.schema.json"
```

### 🐛 调试与技巧

* **依赖检查**: 确保系统已安装 `ffmpeg` 和 `mpv`，并且它们在 `PATH` 环境变量中。程序依赖它们进行视频处理和播放。
* **日志输出**: 程序使用标准的 `log` 包输出信息，可以通过环境变量 `GODEBUG` 或重定向标准输出来进行更详细的调试。
* **Flag 解析顺序**: 再次强调，所有 `-flag` 必须放在命令的最前面，否则 `flag.Parse()` 会提前停止解析，导致后续参数失效。
