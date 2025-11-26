package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Soltus/encv-go/internal/proxy"
)

// Config 定义了代理服务器所需的所有配置
type Config struct {
	ProxyPort    int
	OpenListHost string
	// OpenList 认证，需要有权限
	Token string
	// 与 OpenList 挂载的配置是否启用签名一致，不过好像开关都一样
	DisableSignatureVerification bool
	// 视频解密密码 (与 encv-cli 共用)
	Password string
}

// loadConfig 从命令行和配置文件加载配置
func loadConfig() (*Config, error) {
	cfg := &Config{}

	// 1. 定义命令行参数
	var configFile string
	flag.IntVar(&cfg.ProxyPort, "proxy-port", 0, "Port for the proxy server to listen on.")
	flag.StringVar(&cfg.OpenListHost, "openlist-host", "", "URL of the OpenList server.")
	flag.StringVar(&cfg.Token, "token", "", "Admin token from OpenList (overrides config file).")
	flag.BoolVar(&cfg.DisableSignatureVerification, "disable-signature-verification", false, "Disable OpenList signature verification for testing purposes.")
	flag.StringVar(&cfg.Password, "password", "", "Password for decrypting video files.")
	flag.StringVar(&configFile, "config", "config.user.json", "Path to the user configuration file.")
	flag.Parse()

	// 2. 尝试从配置文件加载（如果存在）
	if _, err := os.Stat(configFile); err == nil {
		fileData, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
		}

		// 创建一个临时结构体来映射 JSON，字段名与配置文件保持一致
		var fileConfig struct {
			ProxyPort                    int    `json:"proxy_port"`
			OpenListHost                 string `json:"openlist_host"`
			Token                        string `json:"token"`
			DisableSignatureVerification bool   `json:"disable_signature_verification"`
			Password                     string `json:"password"`
		}
		if err := json.Unmarshal(fileData, &fileConfig); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %w", configFile, err)
		}

		// 如果命令行参数未设置（即仍为默认值），则使用配置文件中的值
		if cfg.ProxyPort == 0 {
			cfg.ProxyPort = fileConfig.ProxyPort
		}
		if cfg.OpenListHost == "" {
			cfg.OpenListHost = fileConfig.OpenListHost
		}
		if cfg.Token == "" {
			cfg.Token = fileConfig.Token
		}
		if !cfg.DisableSignatureVerification {
			cfg.DisableSignatureVerification = fileConfig.DisableSignatureVerification
		}
		if cfg.Password == "" {
			cfg.Password = fileConfig.Password
		}
	}
	// 3. 设置最终的默认值（如果命令行和配置文件都未指定）
	if cfg.ProxyPort == 0 {
		cfg.ProxyPort = 2025 // 默认端口
	}
	if cfg.OpenListHost == "" {
		cfg.OpenListHost = "http://localhost:5244" // 默认主机
	}
	return cfg, nil
}

// authenticate 获取有效的 Token
func authenticate(cfg *Config) error {
	if cfg.Token != "" {
		log.Println("Using provided token for OpenList authentication.")
		return nil
	}

	// 如果两种方式都没有提供
	return fmt.Errorf("authentication failed: please provide either a -token or -openlist-username and -openlist-password")
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 确保主机地址总是包含协议方案
	if !strings.HasPrefix(cfg.OpenListHost, "http://") && !strings.HasPrefix(cfg.OpenListHost, "https://") {
		log.Printf("Warning: openlist-host '%s' is missing a scheme, defaulting to http://", cfg.OpenListHost)
		cfg.OpenListHost = "http://" + cfg.OpenListHost
	}

	if err := authenticate(cfg); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	// 检查视频解密密码是否已提供
	if cfg.Password == "" {
		log.Fatalf("Password is required. Please provide it via -password flag or in config.user.json")
	}

	// 至此，cfg.Token 和 cfg.Password 必定不为空
	proxyCfg := &proxy.Config{
		Port:                         cfg.ProxyPort, // 使用新的字段名
		OpenListHost:                 cfg.OpenListHost,
		Token:                        cfg.Token,
		VideoPassword:                cfg.Password, // 映射到 proxy 包的 VideoPassword
		DisableSignatureVerification: cfg.DisableSignatureVerification,
	}

	proxy.StartServer(proxyCfg)
}
