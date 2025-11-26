# encv-go 怡念汐拂

一个强大的命令行工具集/库，用于视频文件的加密、解密、流媒体传输，并提供与 OpenList 的无缝代理集成。

> [!WARNING]
> 请确保 `ffmpeg` 和 `mpv` 已安装并添加到系统的环境变量中。

---

## 📖 用户使用指南

本部分将指导您如何安装、配置和使用 `encv` 的各项功能。

### 🚀 安装

如果没有资产，您需要从源代码构建 `encv` 和 `encv-proxy` ，通常这不需要。

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
  "password": "my-encv_key，可以使用中文和标点符号✔",
  "outputPath": "./output",
  "port": 1999,
  "proxy_port": 1998,
  "openlist_host": "http://localhost:5244",
  "trackExtensions": [".ass", ".srt", ".dm.ass", ".vtt"],
  "bin_ext_group": {
    "text": "sccgt",
    "image": "sccgi",
    "audio": "sccga",
    "video": "sccgv"
  },
  "sccgv_settings": { "chunk_size": 100 }
}
```

**配置项说明:**

| 键                  | 描述                                      |
| ------------------- | ----------------------------------------- |
| `password`        | 用于加密和解密视频的密码。                |
| `outputPath`      | 加密后文件的默认输出目录。                |
| `port`            | `encv serve` 流媒体服务的默认监听端口。 |
| `proxy_port`      | `encv-proxy` 代理服务的默认监听端口。   |
| `openlist_host`   | OpenList 服务的地址。                     |
| `bin_ext_group`   | 定义加密类型的容器后缀名，不建议自定义    |
| `trackExtensions` | 需要处理的字幕/轨道文件扩展名列表。       |

### 💻 核心用法

#### 1. 加密视频

将指定目录下的所有视频文件加密，并处理相关字幕文件。

```bash
# 将 _videos 目录下的所有视频加密到 output 目录
./encv encrypt -o ./output ./_videos
```

#### 2. 流媒体服务

启动一个 HTTP 服务器，用于在线流式播放加密后的视频。

```bash
# 在 1999 端口启动服务，提供 ./output 目录下的文件
./encv serve -port 1999 ./output
```

使用 `mpv` 播放器观看时，需要手动指定字幕文件：

```bash
mpv http://localhost:1999/321.sccgv --sub-files=http://localhost:1999/321.ass
```

#### 3. 解密视频

将加密后的文件恢复为原始格式。

> [!NOTE]
> Go 标准库 `flag` 包的行为：所有 `-` 开头的标志参数（如 `-o`）必须放在位置参数（如文件路径）**之前**。

```bash
# 解密单个文件
./encv decrypt -o ./_decrypt/ "./output/sample.sccgv"
# 解密整个目录下的所有 .sccgv 文件
./encv decrypt -o ./_decrypt/ ./output
```

### 🔗 OpenList 集成

`encv-proxy` 是一个专为 OpenList 设计的代理服务，它能透明地解密视频，让 OpenList 可以直接播放加密内容。

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

* 保存后即可在 OpenList 中预览加密文件。

---

## 🛠️ 开发者指南

本部分面向希望参与项目开发或进行调试的开发者。

### 📁 项目结构

```md
encv-go/
├── cmd/                 # 程序入口
│   ├── encv/           # encv 程序
│   └── encv-proxy/     # encv-proxy 代理程序（只代理 OpenList）
├── internal/            # 内部包，不对外暴露
│   ├── config/
│   ├── container/
│   ├── crypto/         # 加解密核心逻辑
│   ├── processor/      # 预处理和元数据处理
│   ├── proxy/          # OpenList 代理服务核心逻辑
│   ├── server/          # HTTP服务（包括通用 webdav）核心逻辑
│   └── types/          # 共享的数据结构定义
│   └── utils/          # 通用工具
├── pkg/                # 对外暴露的公共包，作为库调用
│   └── encv/
│      └── api.go
│      └── decrypt.go
│      └── encrypt.go
│      └── kvi.go
├── go.mod
└── README.md
└── config.user.json
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

一个简单的测试流程是加密一个示例视频目录：

```bash
# 假设你有一个 _videos 目录
./encv encrypt -o ./_test_output ./_videos
```

然后检查 `_test_output` 目录下的 `.sccgv`, `.kvi` 和字幕文件是否符合预期。

### 🐛 调试与技巧

* **依赖检查**: 确保系统已安装 `ffmpeg` 和 `mpv`，并且它们在 `PATH` 环境变量中。程序依赖它们进行视频处理和播放。
* **日志输出**: 程序使用标准的 `log` 包输出信息，可以通过环境变量 `GODEBUG` 或重定向标准输出来进行更详细的调试。
* **Flag 解析顺序**: 再次强调，所有 `-flag` 必须放在命令的最前面，否则 `flag.Parse()` 会提前停止解析，导致后续参数失效。
