// pkg/encv/api.go
package encv

import (
	"context"
	"fmt"

	"github.com/Soltus/encv-go/internal/server"
)

// DecryptOptions 定义解密所需的参数
type DecryptOptions struct {
	// OutputDir 解密后文件的输出目录
	OutputDir string
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

// 自定义错误类型
var (
	ErrMissingOptions = fmt.Errorf("password and output directory are required")
)
