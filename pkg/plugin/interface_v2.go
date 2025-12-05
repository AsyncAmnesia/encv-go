// pkg/plugin/interface_v2.go
package plugin

import (
	"io"
)

// Reader_v2 是一个更高级的读取器接口，它可能包含 Seek
// 我们将 io.Seeker 作为可选能力
type Reader_v2 interface {
	io.ReadCloser
	// 如果 Reader 支持 Seek，它应该实现 io.Seeker
	// 我们可以使用类型断言来检查
	// seeker, ok := reader.(io.Seeker)
}

// ContainerPlugin_v2 定义了容器插件的接口
type ContainerPlugin_v2 interface {
	// Identify 检查给定的文件是否能被此插件处理
	Identify(filePath string) bool

	// GetReader 打开文件并返回一个统一的 Reader_v2 接口
	// 插件内部决定使用 SeekableDecryptReader_v2 还是 SimpleDecryptReader_v2
	GetReader(filePath, password string) (Reader_v2, error)
}
