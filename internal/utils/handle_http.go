package utils

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

// downloadRange 下载 URL 的指定字节范围
func DownloadRange(url string, headers map[string]string, start, end int64) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create range request for %s: %w", url, err)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute range request for %s: %w", url, err)
	}
	defer resp.Body.Close()

	// 检查服务器是否支持范围请求
	if resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("server does not support range requests for %s, status: %s", url, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// ReadAllFromURL 从指定的 URL 下载所有数据，并返回为字节切片
func ReadAllFromURL(url string, headers map[string]string) ([]byte, error) {
	// 1. 创建 HTTP GET 请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", url, err)
	}

	// 2. 添加自定义请求头
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// 3. 执行请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request for %s: %w", url, err)
	}
	defer resp.Body.Close()

	// 4. 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned non-200 status for %s: %s", url, resp.Status)
	}

	// 5. 读取所有响应体数据
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %s: %w", url, err)
	}

	return body, nil
}

// 创建一个 HTTP GET 请求并返回响应体的 ReadCloser
func GetRemoteStream(fileURL string, headers map[string]string) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", fileURL, err)
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request for %s: %w", fileURL, err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("remote server returned status %s for %s", resp.Status, fileURL)
	}

	return resp.Body, nil
}

// 创建一个带有正确认证头的 HTTP 请求
func MakeAuthenticatedRequest(method, url, body, token string) (*http.Response, error) {
	var reqBody io.Reader
	if body != "" {
		reqBody = bytes.NewBuffer([]byte(body))
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")

	// --- 关键修正：根据 Token 格式决定是否添加 Bearer 前缀 ---
	if strings.Contains(token, ".") {
		// JWT 格式，使用 Bearer 前缀
		req.Header.Add("Authorization", "Bearer "+token)
		log.Printf("-> [Auth Debug] Using Bearer token for request to %s", url)
	} else {
		// 永久 Token 格式，直接使用
		req.Header.Add("Authorization", token)
		log.Printf("-> [Auth Debug] Using permanent token for request to %s", url)
	}

	client := &http.Client{}
	return client.Do(req)
}

// isConnectionClosedError 判断错误是否由客户端断开连接引起
func IsConnectionClosedError(err error) bool {
	// 处理 Go 1.16+ 的特定错误
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	// 处理旧版本或更通用的错误
	errStr := err.Error()
	return strings.Contains(errStr, "connection reset by peer") || strings.Contains(errStr, "broken pipe")
}
