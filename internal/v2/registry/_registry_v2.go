// internal/v2/registry/registry_v2.go
package registry

import (
	"context"
	"fmt"
	"sync"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/pkg/plugin"
)

var (
	pluginRegistry_v2 []plugin.ContainerPlugin_v2
	registryMutex_v2  sync.RWMutex
)

// RegisterPlugin_v2 向全局注册表注册一个新的插件
func RegisterPlugin_v2(p plugin.ContainerPlugin_v2) {
	registryMutex_v2.Lock()
	defer registryMutex_v2.Unlock()
	pluginRegistry_v2 = append(pluginRegistry_v2, p)
}

// OpenFile_v2 是一个工厂函数，用于打开任何支持的容器文件
// 它会遍历所有已注册的插件，找到能处理该文件的插件，并返回一个 Reader
func OpenFile_v2(filePath, password string) (plugin.Reader_v2, error) {
	registryMutex_v2.RLock()
	plugins := make([]plugin.ContainerPlugin_v2, len(pluginRegistry_v2))
	copy(plugins, pluginRegistry_v2)
	registryMutex_v2.RUnlock()

	if len(plugins) == 0 {
		return nil, fmt.Errorf("no plugins are registered")
	}

	for _, p := range plugins {
		if p.Identify(filePath) {
			return p.GetReader(filePath, password)
		}
	}

	return nil, fmt.Errorf("no registered plugin could identify the file: %s", filePath)
}

// // init 函数在包被导入时自动执行，但是不支持参数，因为其生命周期过早
// func init() {
// }

func Init(ctx context.Context) {
	// 从上下文中获取配置
	cfg := config.FromContext(ctx)
	if cfg == nil {
		// 如果没有配置，v2 插件将无法注册，但这不应该导致程序崩溃
		// 可能应该记录一个警告
		// log.Println("Warning: No configuration found in context, skipping v2 plugin registration.")
		return
	}

	// 旧的插件注册方式，需要移除
	// RegisterPlugin_v2(plugins.NewSccgPlugin_v2(cfg))
}
