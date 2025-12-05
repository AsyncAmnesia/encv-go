package postdecrypt

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/Soltus/encv-go/internal/types"
)

// PostDecrypter 定义了在文件解密后执行的特定类型操作的接口。
type PostDecrypter interface {
	// PostDecrypt 执行特定类型的后处理逻辑。
	// content: 包含解密后的数据流和解析出的元数据。
	// containerPath: 原始容器文件的路径（例如，用于寻找同目录的字幕）。
	// outputDir: 解密后文件的输出目录（例如，用于放置还原的字幕）。
	PostDecrypt(ctx context.Context, content *types.DecryptedContent, containerPath, outputDir string) error
}

var (
	registry = make(map[reflect.Type]PostDecrypter)
	mu       sync.RWMutex
)

// Register 将一个 types.Index 的具体类型与一个 PostDecrypter 关联起来
func Register(indexType types.Index, p PostDecrypter) {
	mu.Lock()
	defer mu.Unlock()
	t := reflect.TypeOf(indexType)
	if _, exists := registry[t]; exists {
		panic(fmt.Sprintf("post-decrypter for type '%s' already registered", t))
	}
	registry[t] = p
}

// GetPostDecrypter 根据给定的 types.Index 实例查找对应的 PostDecrypter
func GetPostDecrypter(index types.Index) (PostDecrypter, error) {
	mu.RLock()
	defer mu.RUnlock()

	t := reflect.TypeOf(index)
	if p, ok := registry[t]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("no post-decrypter found for index type: %T", index)
}

// InitPostDecrypters 初始化并注册所有后处理器
func InitPostDecrypters() {
	// 为了让 Register 能获取到正确的类型，我们需要传入实例的指针
	// Register(&types.VideoIndex{}, &VideoPostDecrypter{})
	Register(&types.ImageIndex{}, &ImagePostDecrypter{})
	Register(&types.TextIndex{}, &TextPostDecrypter{})
	// 未来添加新容器时，只需在这里添加一行
	// Register(&types.AudioIndex{}, &AudioPostDecrypter{})
}
