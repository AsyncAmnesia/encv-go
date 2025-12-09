package register

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/types"
	"github.com/gogf/gf/v2/net/ghttp"
)

// StartHttpHandlerWithRetry 安全地启动一个 http.Handler。
// 它会进行端口递增，并通过带 instanceID 的 /ping 进行自检，防止端口劫持。
func StartHttpHandlerWithRetry(handler http.Handler, initialPort int, instanceID, version string) (string, error) {
	maxTries := 100
	for i := 0; i < maxTries; i++ {
		currentPort := initialPort + i
		addr := fmt.Sprintf(":%d", currentPort)

		listener, err := net.Listen("tcp", addr)
		if err != nil {
			logAndContinue(err, currentPort)
			continue
		}

		go func() {
			log.Printf("Backend: Attempting to start on %s...", addr)
			backendServer := &http.Server{Handler: handler}
			if serveErr := backendServer.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
				log.Printf("Backend server on %s encountered an error: %v", listener.Addr().String(), serveErr)
			}
		}()

		// 等待服务启动
		time.Sleep(150 * time.Millisecond)

		// 执行带身份验证的自检
		if err := performPingCheck(currentPort, instanceID, version); err != nil {
			log.Printf("Backend self-check on port %d failed: %v. Trying next port...", currentPort, err)
			listener.Close()
			continue
		}

		// 自检成功！
		actualAddr := listener.Addr().String()
		log.Printf("✅ Backend server successfully started and listening on %s", actualAddr)
		return actualAddr, nil
	}

	return "", fmt.Errorf("failed to start http.Handler after %d tries", maxTries)
}

// StartGfServerWithRetry 安全地启动一个 *ghttp.Server。
// 它会进行端口递增，自动注册 /ping 路由，并通过带 instanceID 的自检，防止端口劫持。
func StartGfServerWithRetry(server *ghttp.Server, initialPort int, instanceID, version string) (string, error) {
	// 【核心】为 GoFrame 服务器注册自检用的 /ping 路由
	server.BindHandler("/ping", func(r *ghttp.Request) {
		r.Response.WriteJson(types.PingResponse{
			Status:     types.ServiceStatuses.OK,
			Version:    version,
			InstanceID: instanceID,
		})
	})

	maxTries := 100
	for i := 0; i < maxTries; i++ {
		currentPort := initialPort + i
		server.SetPort(currentPort)

		if err := server.Start(); err != nil {
			logAndContinue(err, currentPort)
			continue
		}

		// 等待 GoFrame 完成监听
		time.Sleep(100 * time.Millisecond)

		// 【关键修复】使用正确的 API 获取实际监听的端口号
		actualPort := server.GetListenedPort()
		if actualPort == 0 {
			server.Shutdown()
			return "", fmt.Errorf("failed to get listened port from ghttp.Server")
		}

		// 【关键修复】使用实际的端口号进行自检
		if err := performPingCheck(actualPort, instanceID, version); err != nil {
			log.Printf("Proxy self-check on port %d failed: %v. Trying next port...", actualPort, err)
			server.Shutdown()
			continue
		}

		// 自检成功！
		// 使用 GetListenedAddress() 获取完整地址用于日志和返回
		actualAddr := server.GetListenedAddress()
		log.Printf("✅ Proxy server successfully started and listening on %s", actualAddr)
		return actualAddr, nil
	}

	return "", fmt.Errorf("failed to start ghttp.Server after %d tries", maxTries)
}

// performPingCheck 是一个通用的 ping 检查函数
func performPingCheck(port int, expectedInstanceID, expectedVersion string) error {
	pingURL := fmt.Sprintf("http://127.0.0.1:%d/ping", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(pingURL)
	if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping request failed: %w", err)
	}
	defer resp.Body.Close()

	var pingResp types.PingResponse
	if err := json.NewDecoder(resp.Body).Decode(&pingResp); err != nil {
		return fmt.Errorf("could not decode ping response: %w", err)
	}

	if pingResp.InstanceID != expectedInstanceID {
		return fmt.Errorf("instance ID mismatch: expected %s, got %s", expectedInstanceID, pingResp.InstanceID)
	}

	if pingResp.Version != expectedVersion {
		// 版本不匹配通常不是劫持，但可以作为警告
		log.Printf("Warning: version mismatch on port %d: expected %s, got %s", port, expectedVersion, pingResp.Version)
	}

	return nil
}

// logAndContinue 是一个辅助函数，用于记录端口占用错误
func logAndContinue(err error, port int) {
	if utils.IsAddrInUseErr(err) {
		log.Printf("Port %d is in use, trying next port...", port)
	} else {
		log.Printf("Failed to start on port %d: %v. Trying next port...", port, err)
	}
}
