package register

import (
	"context"

	"github.com/Soltus/encv-go/internal/packer"
	"github.com/Soltus/encv-go/internal/postdecrypt"
	"github.com/Soltus/encv-go/internal/processor"
	"github.com/Soltus/encv-go/internal/unpacker"
)

// Init 初始化 ENCV 库所需的所有内部组件。
// 它必须在调用任何其他 ENCV 功能之前被调用。
// 它接受一个 context.Context，以便在初始化期间传递必要的配置。
func Init(ctx context.Context) {
	// 按顺序初始化所有组件
	// 如果未来组件间有依赖关系，顺序将变得重要
	processor.InitProcessors(ctx) // 预处理器
	packer.InitPackers()          // 打包器
	unpacker.InitUnpackers(ctx)   // 解包器
	// unpacker.InitUnpackers_V2(ctx)
	postdecrypt.InitPostDecrypters() // 后处理器
}
