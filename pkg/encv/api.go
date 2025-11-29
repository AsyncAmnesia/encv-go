// pkg/encv/api.go
package encv

import (
	"context"

	"github.com/Soltus/encv-go/internal/packer"
	"github.com/Soltus/encv-go/internal/postdecrypt"
	"github.com/Soltus/encv-go/internal/processor"
	"github.com/Soltus/encv-go/internal/server"
	"github.com/Soltus/encv-go/internal/unpacker"
)

// Init 初始化 ENCV 库所需的所有内部组件。
// 它必须在调用任何其他 ENCV 功能之前被调用。
// 它接受一个 context.Context，以便在初始化期间传递必要的配置。
func Init(ctx context.Context) {
	// 按顺序初始化所有组件
	// 如果未来组件间有依赖关系，顺序将变得重要
	processor.InitProcessors(ctx)    // 预处理器
	packer.InitPackers()             // 打包器
	unpacker.InitUnpackers(ctx)      // 解包器
	postdecrypt.InitPostDecrypters() // 后处理器
}

// Player 封装了流媒体服务器，提供对外接口
type Player struct {
	p *server.Player
}

// NewPlayer 创建一个新的播放器实例
func NewPlayer(ctx context.Context) *Player {
	return &Player{p: server.NewPlayer(ctx)}
}

// Start 启动服务器，返回监听的地址
func (p *Player) Start(port int) (string, error) {
	return p.p.Start(port)
}

// Stop 停止服务器
func (p *Player) Stop() {
	p.p.Stop()
}
