## 插件系统

### 新的、清晰的架构

1. **`internal/v2/plugins/interface.go`** : 定义所有插件必须实现的统一接口。
2. **`internal/v2/plugins/registry.go`** : 插件注册表，负责加载和管理所有插件。
3. **`internal/v2/plugins/generic/`** : 通用插件，处理所有非特殊文件。
4. **`internal/v2/plugins/video/`** : 视频插件，处理视频和字幕。
