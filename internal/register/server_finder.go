package register

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Soltus/encv-go/internal/v2/types"
)

// findServer 尝试在指定端口范围内发现正在运行的 encv 服务器
func FindServer(startPort int, maxTries int) (string, error) {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	log.Printf("-> Discovering ENCV server, scanning ports from %d to %d...", startPort, startPort+maxTries-1)

	for i := 0; i < maxTries; i++ {
		currentPort := startPort + i
		pingURL := fmt.Sprintf("http://127.0.0.1:%d/ping", currentPort)

		resp, err := client.Get(pingURL)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusOK {
			var pingResp types.PingResponse
			if json.NewDecoder(resp.Body).Decode(&pingResp) == nil && pingResp.Status == types.ServiceStatuses.OK {
				resp.Body.Close()
				serverAddr := fmt.Sprintf("127.0.0.1:%d", currentPort)
				log.Printf("✅ Found ENCV server at %s (Instance: %s, Version: %s)", serverAddr, pingResp.InstanceID, pingResp.Version)
				return serverAddr, nil
			}
		}
		resp.Body.Close()
	}

	return "", fmt.Errorf("could not find any running ENCV server in port range %d-%d", startPort, startPort+maxTries-1)
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
