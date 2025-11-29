// pkg/encv/encrypt.go

package encv

import (
	"context"

	"github.com/Soltus/encv-go/internal/service"
)

// Encrypter 定义了加密操作的公共接口。
// 这是提供给外部使用者（例如 main.go 或其他项目）的稳定 API。
type Encrypter interface {
	// Encrypt 对给定路径（文件或目录）执行加密操作。
	Encrypt(ctx context.Context, inputPath string) error
}

// encrypter 是 Encrypter 接口的内部实现。
// 它充当公共 API 和 internal/service 之间的适配器。
type encrypter struct{}

// NewEncrypter 是一个构造函数，它创建并返回一个新的 Encrypter 实例。
func NewEncrypter() Encrypter {
	return &encrypter{}
}

// Encrypt 实现了 Encrypter 接口。
// 它将实际的加密工作委托给 internal 包中的实现。
func (e *encrypter) Encrypt(ctx context.Context, inputPath string) error {
	return service.Encrypt(ctx, inputPath)
}
