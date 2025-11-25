# encv

## 功能

请确保 `ffmpeg` 和 `mpv` 已安装并在环境变量中。

三大模块：加密、解密、流媒体。

## 项目结构

encv-go/
├── cmd/
│   └── encv/
│       └── main.go          # CLI 入口点
├── internal/
│   ├── config/
│   │   └── config.go        # 配置加载逻辑
│   ├── crypto/
│   │   └── crypto.go        # 加密/解密核心
│   ├── processor/
│   │   └── processor.go     # 视频处理核心
│   ├── server/
│   │   └── server.go        # HTTP 流媒体服务器
│   └── types/
│       └── types.go         # 公共数据结构
├── pkg/
│   └── mobile/
│       └── lib.go           # gomobile 绑定接口
├── config.user.json         # 示例配置文件
├── go.mod
└── go.sum

## 构建

```cmd
 go build ./cmd/encv
```

构建 AAR (用于 Android):

```cmd
gomobile bind -target=android -o encv.aar ./pkg/encv
```

## 测试

```cmd
./encv encrypt ./_videos
```

需要手动指定字幕让MPV识别，例如 `mpv http://localhost:1999/video/321 --sub-files=http://localhost:1999/subtitle/321.dm.ass`

```cmd
./encv serve ./output
```

Go 标准库 flag 包的一个核心行为：当 Parse 函数遇到第一个非标志（不以 - 开头）的参数时，它会停止解析后续的标志，因此用户应将所有标志（如 -o ）放在位置参数（如文件路径）之前。

```cmd
./encv decrypt -o ./_decrypt/ ./output/sample.enc
```
