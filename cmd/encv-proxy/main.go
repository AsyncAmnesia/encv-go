package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/proxy"
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/pkg/encv"
)

func main() {
	// 【新增】在程序最开始就设置日志
	logFilePath := utils.SetupLogging("encv-proxy.log")

	// %TEMP%/encv.log
	log.Printf("Received Args: %v\n", os.Args)
	// 也打印到控制台，方便在 PowerShell 中查看
	fmt.Printf("-> Log file is at: %s\n", logFilePath)
	// 1. 加载基础配置（默认值 + 配置文件），后期修改为可自定义配置文件路径
	cfg, err := config.Load("config.user.json")
	if err != nil {
		log.Fatalf("Failed to load base config: %v", err)
	}

	// 2. 定义 Proxy 特有的命令行参数，默认值来自已加载的配置
	flag.IntVar(&cfg.Proxy.Port, "proxy-port", cfg.Proxy.Port, "Port for the proxy server.")
	flag.StringVar(&cfg.Proxy.OpenListHost, "openlist-host", cfg.Proxy.OpenListHost, "URL of the OpenList server.")
	flag.StringVar(&cfg.Proxy.Token, "token", cfg.Proxy.Token, "Admin token from OpenList.")
	flag.BoolVar(&cfg.Proxy.DisableSignatureVerification, "disable-signature-verification", cfg.Proxy.DisableSignatureVerification, "Disable signature verification.")
	flag.StringVar(&cfg.Password, "password", cfg.Password, "Password for decrypting video files.")

	// 解析所有命令行参数
	flag.Parse()
	finalCtx := config.NewContext(context.Background(), cfg)
	encv.Init(finalCtx)

	// 确保主机地址总是包含协议方案
	if !strings.HasPrefix(cfg.Proxy.OpenListHost, "http://") && !strings.HasPrefix(cfg.Proxy.OpenListHost, "https://") {
		log.Printf("Warning: openlist-host '%s' is missing a scheme, defaulting to http://", cfg.Proxy.OpenListHost)
		cfg.Proxy.OpenListHost = "http://" + cfg.Proxy.OpenListHost
	}

	if err := authenticate(finalCtx); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	// 3. 验证必要的配置
	if cfg.Password == "" {
		log.Fatalf("Password is required. Please provide it via -password flag or in config.user.json")
	}
	if cfg.Proxy.Token == "" {
		log.Fatalf("OpenList token is required. Please provide it via -token flag or in config.user.json")
	}

	// 确保主机地址包含协议
	if !strings.HasPrefix(cfg.Proxy.OpenListHost, "http://") && !strings.HasPrefix(cfg.Proxy.OpenListHost, "https://") {
		log.Printf("Warning: openlist-host '%s' is missing a scheme, defaulting to http://", cfg.Proxy.OpenListHost)
		cfg.Proxy.OpenListHost = "http://" + cfg.Proxy.OpenListHost
	}

	if err := authenticate(finalCtx); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	// 【修改】创建 Proxy 实例并启动它
	proxyServer := proxy.NewProxy(finalCtx)
	proxyServer.StartServer()
}

// authenticate 获取有效的 Token
func authenticate(ctx context.Context) error {
	cfg := config.FromContext(ctx)
	if cfg.Proxy.Token != "" {
		log.Println("Using provided token for OpenList authentication.")
		return nil
	}

	// 如果两种方式都没有提供
	return fmt.Errorf("authentication failed: please provide either a -token or -openlist-username and -openlist-password")
}
