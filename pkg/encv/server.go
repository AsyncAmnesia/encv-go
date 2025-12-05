package encv

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/register"
	"github.com/Soltus/encv-go/internal/server"
	"github.com/Soltus/encv-go/internal/types"
)

func StartWebdav(ctx context.Context) (string, string, error) {
	return server.StartWebdav(ctx)
}

func FindServer(startPort int, maxTries int) (string, error) {
	return register.FindServer(startPort, maxTries)
}

// NewPlayer 创建一个新的播放器实例
func NewServer(ctx context.Context) *server.Server {
	return server.NewServer(ctx)
}

// 检查是否已有服务在运行，如果是则返回错误
func CheckForExistingService(startPort int) error {

	if startPort == 0 {
		startPort = 1999
	}
	log.Printf("-> Checking for existing ENCV services starting from port %d...", startPort)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	maxTries := 20

	for i := 0; i < maxTries; i++ {
		currentPort := startPort + i
		pingURL := fmt.Sprintf("http://127.0.0.1:%d/ping", currentPort)

		resp, err := client.Get(pingURL)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var pingResp types.PingResponse
			if json.NewDecoder(resp.Body).Decode(&pingResp) == nil && pingResp.Status == types.ServiceStatuses.OK {
				log.Printf("--------------------------------------------------")
				log.Printf("🔴 Found an existing ENCV server running at port %d!", currentPort)
				log.Printf("   Instance ID: %s", pingResp.InstanceID)
				log.Printf("   Main Dir:    %s", pingResp.ServerDirPath)
				if pingResp.WebdavDirPath != "" {
					log.Printf("   WebDAV Dir:  %s", pingResp.WebdavDirPath)
				}
				log.Println("--------------------------------------------------")
				log.Fatalf("To start a new server, please stop the existing one or run <start> command.")
			}
		}
	}
	return nil // 没有找到现有服务
}

// 解析服务标志的辅助函数
func ParseServerFlags(cmd *flag.FlagSet, cfg *config.Config, args []string) error {
	cmd.StringVar(&cfg.Password, "p", cfg.Password, "Password for decryption, overrides config file")
	cmd.StringVar(&cfg.Server.Dir, "d", cfg.Server.Dir, "Directory to serve from, overrides config file")
	cmd.IntVar(&cfg.Server.Port, "port", cfg.Server.Port, "Port to run the server on, overrides config file")
	return cmd.Parse(args)
}
