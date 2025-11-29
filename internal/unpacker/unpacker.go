package unpacker

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/container/chunked"
)

// Unpacker 定义了从磁盘解包特定类型加密容器的接口。
type Unpacker interface {
	Unpack(ctx context.Context, containerPath, detectedExt string) (*container.PackedData, error)
}

var (
	registry = make(map[string]Unpacker)
	mu       sync.RWMutex
)

// Register 将一个容器类型扩展名与一个 Unpacker 关联起来
func Register(containerExt string, u Unpacker) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[containerExt]; exists {
		panic(fmt.Sprintf("unpacker for type '%s' already registered", containerExt))
	}
	registry[containerExt] = u
}

// Unpack 发生在 index 生成前，因此不适合用反射匹配
// GetUnpacker 根据给定的容器类型扩展名查找对应的 Unpacker
func GetUnpacker(containerExt string) (Unpacker, error) {
	mu.RLock()
	defer mu.RUnlock()

	if u, ok := registry[containerExt]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("no unpacker found for container type: %s", containerExt)
}

// InitUnpackers 初始化并注册所有解包器
func InitUnpackers(ctx context.Context) {
	cfg := config.FromContext(ctx)
	Register(cfg.BinExtGroup.Video, &VideoUnpacker{})
	Register(cfg.BinExtGroup.Image, &GenericUnpacker{})
	Register(cfg.BinExtGroup.Text, &GenericUnpacker{}) // 复用 GenericUnpacker
}

// readCloser 组合一个 io.Reader 和一个 Close 函数，创建一个 io.ReadCloser。
// 这是 Go 中组合资源关闭操作的标准模式。
type readCloser struct {
	io.Reader
	closeFunc func() error
}

func (rc *readCloser) Close() error {
	return rc.closeFunc()
}

func newReadCloser(r io.Reader, closeFunc func() error) io.ReadCloser {
	return &readCloser{Reader: r, closeFunc: closeFunc}
}

// ===================================================================================
// 1. 通用单文件解包器
// ===================================================================================
// BaseUnpacker 提供了从文件解包的通用功能。
// 其他具体的 Unpacker 可以嵌入此结构体来复用逻辑。
type BaseUnpacker struct{}

// UnpackFromFile 是一个通用的辅助方法，用于从文件路径解包容器。
// 它封装了获取魔法数字、打开文件以及管理资源关闭的细节。
func (b *BaseUnpacker) UnpackFromFile(ctx context.Context, containerPath, detectedExt string) (*container.PackedData, error) {
	// 1. 获取魔法数字
	magicMap, err := container.GetContainerMagicMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get magic map: %w", err)
	}

	magic, ok := magicMap[detectedExt]
	if !ok {
		return nil, fmt.Errorf("internal error: extension '%s' not in magic map", detectedExt)
	}

	// 2. 打开容器文件
	file, err := os.Open(containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open container: %w", err)
	}

	// 3. 调用底层的 container.Unpack
	packedData, err := container.Unpack(file, magic)
	if err != nil {
		file.Close() // 出错时确保文件被关闭
		return nil, fmt.Errorf("failed to unpack container: %w", err)
	}

	// 4. 组合关闭函数，确保解密流和文件句柄都能被正确关闭
	packedData.DataStream = newReadCloser(packedData.DataStream, file.Close)
	return packedData, nil
}

// ===================================================================================
// 2. 通用分块容器解包器
// ===================================================================================
// BaseChunkedUnpacker 提供了从文件解包的通用功能（用于分块容器）。
type BaseChunkedUnpacker struct{}

// UnpackChunkedFromFile 是一个通用的辅助方法，用于从文件路径解包分块容器。
func (b *BaseChunkedUnpacker) UnpackChunkedFromFile(ctx context.Context, containerPath, binExt string) (*container.PackedData, error) {
	// 1. 获取主魔法数字和子魔法数字
	magicMap, err := container.GetContainerMagicMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get magic map: %w", err)
	}
	subMagicMap, err := container.GetSubChunkMagicMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get sub-magic map: %w", err)
	}

	mainMagic, ok := magicMap[binExt]
	if !ok {
		return nil, fmt.Errorf("internal error: extension '%s' not in magic map", binExt)
	}
	subMagic, ok := subMagicMap[binExt]
	if !ok {
		return nil, fmt.Errorf("internal error: extension '%s' not in sub-magic map", binExt)
	}

	// 2. 创建分块读取器
	reader, err := chunked.LocalReader(containerPath, mainMagic, subMagic)
	if err != nil {
		return nil, fmt.Errorf("failed to create chunked reader for %s: %w", containerPath, err)
	}
	if reader == nil {
		return nil, fmt.Errorf("internal error: chunked.LocalReader returned a nil reader for file %s", containerPath)
	}

	// 3. 返回解包后的数据
	return &container.PackedData{
		KVIData:    reader.KVIData,
		DataStream: reader, // reader 本身就是 io.ReadCloser
	}, nil
}
