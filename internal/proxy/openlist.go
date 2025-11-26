package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// FileInfoResponse 是 /api/fs/link 的响应结构
type FileInfoResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		URL           string            `json:"url"`
		Header        map[string]string `json:"header"`
		Expiration    interface{}       `json:"Expiration"` // 可能是 null 或 string
		Concurrency   int               `json:"concurrency"`
		PartSize      int               `json:"part_size"`
		ContentLength int64             `json:"content_length"`
	} `json:"data"`
}

// makeAuthenticatedRequest 创建一个带有正确认证头的 HTTP 请求
func makeAuthenticatedRequest(method, url, body, token string) (*http.Response, error) {
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

// GetFileURL 获取文件的真实下载链接和请求头
func GetFileURL(path, host, token string) (*FileInfoResponse, error) {
	apiURL := fmt.Sprintf("%s/api/fs/link", host)

	reqBody, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		return nil, fmt.Errorf("failed to create request body: %w", err)
	}

	resp, err := makeAuthenticatedRequest("POST", apiURL, string(reqBody), token)
	if err != nil {
		return nil, fmt.Errorf("failed to call OpenList API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	bodyString := string(bodyBytes)
	log.Printf("-> [OpenList Debug] Raw response from /api/fs/link for path '%s': %s", path, bodyString)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenList API returned non-200 status: %d, body: %s", resp.StatusCode, bodyString)
	}

	var fileInfo FileInfoResponse
	if err := json.Unmarshal(bodyBytes, &fileInfo); err != nil {
		return nil, fmt.Errorf("failed to parse OpenList API response: %w, body: %s", err, bodyString)
	}

	// --- 关键修正：检查嵌套在 data 中的 URL ---
	if fileInfo.Data.URL == "" {
		return nil, fmt.Errorf("OpenList API returned an empty URL in the response, body: %s", bodyString)
	}

	return &fileInfo, nil
}

// LoginRequest 是登录请求的结构
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse 是登录响应的结构
type LoginResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Token string `json:"token"`
	} `json:"data"`
}

// LoginAndGetToken 通过用户名和密码登录并获取 Token，这个方法获取的 Token 没用，已弃用
func loginAndGetToken(host, username, password string) (string, error) {
	loginURL := fmt.Sprintf("%s/api/auth/login", host)

	loginData := LoginRequest{Username: username, Password: password}
	reqBody, err := json.Marshal(loginData)
	if err != nil {
		return "", fmt.Errorf("failed to create login request body: %w", err)
	}

	resp, err := http.Post(loginURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to call OpenList login API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	bodyString := string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenList login failed: status %d, body: %s", resp.StatusCode, bodyString)
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(bodyBytes, &loginResp); err != nil {
		return "", fmt.Errorf("failed to parse OpenList login response: %w, body: %s", err, bodyString)
	}

	// 检查 API 业务逻辑是否成功
	if loginResp.Code != 200 {
		return "", fmt.Errorf("OpenList API returned an error: code %d, message: %s", loginResp.Code, loginResp.Message)
	}

	// 检查 Token 是否为空
	if loginResp.Data.Token == "" {
		return "", fmt.Errorf("received empty token from OpenList, raw response: %s", bodyString)
	}

	return loginResp.Data.Token, nil
}
