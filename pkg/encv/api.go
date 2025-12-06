// pkg/encv/api.go
package encv

import (
	"context"

	register_v2 "github.com/Soltus/encv-go/internal/v2/registry"
)

// Init 初始化 ENCV 库所需的所有内部组件。
// 它必须在调用任何其他 ENCV 功能之前被调用。
// 它接受一个 context.Context，以便在初始化期间传递必要的配置。
func Init(ctx context.Context) {
	// register.Init(ctx)
	register_v2.Init(ctx)
}

func InitV2(ctx context.Context) {
	register_v2.Init(ctx)
}
