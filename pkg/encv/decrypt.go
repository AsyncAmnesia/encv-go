// pkg/encv/decrypt.go

package encv

import (
	"context"
	"errors"

	"github.com/Soltus/encv-go/internal/service"
)

var (
	// ErrMissingOptions 在解密选项未正确提供时返回。
	ErrMissingOptions = errors.New("decrypt options are missing or invalid, e.g., output directory is required")
)

// Decrypter 定义了所有解密相关操作的公共接口。
type Decrypter interface {
	// Decrypt 对给定路径（文件或目录）执行解密操作。
	Decrypt(ctx context.Context, inputPath string, opts service.DecryptOptions) error

	// ExtractKVI 从加密容器中提取 KVI 数据，无需密码。
	ExtractKVI(ctx context.Context, inputPath string) ([]byte, error)
}

// decrypter 是 Decrypter 接口的内部实现，充当适配器。
type decrypter struct{}

// NewDecrypter 是一个构造函数，它创建并返回一个新的 Decrypter 实例。
func NewDecrypter() Decrypter {
	return &decrypter{}
}

// Decrypt 实现了 Decrypter 接口。
func (d *decrypter) Decrypt(ctx context.Context, inputPath string, opts service.DecryptOptions) error {
	if err := validateDecryptOpts(opts); err != nil {
		return err
	}
	// 将实际工作委托给 internal/service
	return service.DecryptFileOrDir(ctx, inputPath, opts)
}

// ExtractKVI 实现了 Decrypter 接口。
func (d *decrypter) ExtractKVI(ctx context.Context, inputPath string) ([]byte, error) {
	// 将实际工作委托给 internal/service
	return service.ExtractKVI(ctx, inputPath)
}

// validateDecryptOpts 是一个内部辅助函数，用于验证选项。
func validateDecryptOpts(opts service.DecryptOptions) error {
	if opts.OutputDir == "" {
		return ErrMissingOptions
	}
	return nil
}
