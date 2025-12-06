package encv

import (
	"context"

	"github.com/Soltus/encv-go/internal/service"
)

// ExtractKVI 从给定的 ENCV 容器文件中提取 KVI 数据，并以原始 JSON 字节切片的形式返回。
func ExtractKVI(ctx context.Context, containerPath string) ([]byte, error) {
	// 直接调用服务层函数
	return service.ExtractKVI(ctx, containerPath)
}
