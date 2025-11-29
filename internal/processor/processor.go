package processor

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/types"
)

// Processor 定义了文件处理器的通用接口。
type Processor interface {
	// Process 接收一个输入文件路径，返回分析后的元数据。
	// Process(inputPath string) (types.Index, error)
	// Process 处理文件，返回元数据和一个用于读取处理后数据的流。
	// 调用者负责关闭返回的 io.ReadCloser。
	Process(inputPath string) (types.Index, io.ReadCloser, error)

	// SupportedMimePrefixes 返回该处理器支持的 MIME 类型前缀。
	// 例如，图像处理器返回 []string{"image/"}。
	// 这使得处理器可以自描述，而无需在注册表中硬编码。
	SupportedMimePrefixes() []string

	// ShouldProcess 根据文件路径判断是否应该处理此文件。
	// 允许处理器基于文件名、扩展名等业务规则进行二次判断。
	// 默认实现是返回 true。
	ShouldProcess(inputPath string) bool
}

var (
	// registry 的键是 MIME 前缀 (如 "image/")，值是对应的处理器
	registry = make(map[string]Processor)
	mu       sync.RWMutex
)

// Register 将一个处理器与一个 MIME 前缀关联起来
func Register(mimePrefix string, p Processor) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[mimePrefix]; exists {
		panic(fmt.Sprintf("processor for MIME prefix '%s' already registered", mimePrefix))
	}
	registry[mimePrefix] = p
}

// GetProcessor 根据完整的 MIME 类型查找处理器，Processor在 index 生成前，因此不适合反射匹配
// 它会进行前缀匹配，例如 "image/jpeg" 会匹配到注册为 "image/" 的处理器
func GetProcessor(mimeType string) (Processor, error) {
	mu.RLock()
	defer mu.RUnlock()

	// 为了匹配最具体的前缀（虽然目前场景下不必要，但这是个好习惯）
	// 我们将所有已注册的前缀排序，长的在前
	var prefixes []string
	for prefix := range registry {
		prefixes = append(prefixes, prefix)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(prefixes)))

	for _, prefix := range prefixes {
		if strings.HasPrefix(mimeType, prefix) {
			return registry[prefix], nil
		}
	}

	return nil, fmt.Errorf("no processor found for MIME type: %s", mimeType)
}

// 【关键修改】InitProcessors 现在是自驱动的
// 它遍历所有处理器实例，并让它们自己注册支持的 MIME 前缀
func InitProcessors(ctx context.Context) {
	if config.FromContext(ctx) == nil {
		panic("cannot initialize processors, configuration not found in context")
	}

	// 列出所有可用的处理器
	allProcessors := []Processor{
		&ImageProcessor{},
		&VideoProcessor{},
		&TextProcessor{},
		// 未来添加新容器时，只需在这里添加实例
		// &AudioProcessor{},
	}

	// 让每个处理器自己注册
	for _, p := range allProcessors {
		for _, prefix := range p.SupportedMimePrefixes() {
			Register(prefix, p)
		}
	}
}

// 【新增】IsMimeTypeSupported 检查给定的 MIME 类型是否有任何处理器支持
// 这成为全项目判断文件类型是否支持的唯一入口
func IsMimeTypeSupported(mimeType string) bool {
	mu.RLock()
	defer mu.RUnlock()

	// 为了匹配最具体的前缀，将所有已注册的前缀排序，长的在前
	var prefixes []string
	for prefix := range registry {
		prefixes = append(prefixes, prefix)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(prefixes)))

	for _, prefix := range prefixes {
		if strings.HasPrefix(mimeType, prefix) {
			return true
		}
	}
	return false
}
