// pkg/encv/api.go
package encv

import (
	"fmt"

	"github.com/Soltus/encv-go/internal/server"
)

// EncryptOptions 定义加密所需的参数
type EncryptOptions struct {
	// Password 用于加密和解密的密码
	Password string
	// OutputDir 加密后文件的输出目录
	OutputDir string
	// TrackExtensions 需要关联的字幕/弹幕文件扩展名
	TrackExtensions []string
}

// DecryptOptions 定义解密所需的参数
type DecryptOptions struct {
	// Password 用于解密的密码
	Password string
	// OutputDir 解密后文件的输出目录
	OutputDir string
	// 【新增】命令行指定的强制覆盖标志
	Force bool
}

// Player 封装了流媒体服务器，提供对外接口
type Player struct {
	p *server.Player
}

// NewPlayer 创建一个新的播放器实例
func NewPlayer(dir, password string) *Player {
	return &Player{p: server.NewPlayer(dir, password)}
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
