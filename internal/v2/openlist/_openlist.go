package openlist

// 即将弃用

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/utils"
)

// FileInfoResponse 是 /api/fs/link 的响应结构
type FileInfoResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		URL           string              `json:"url"`
		Header        map[string][]string `json:"header"`
		Expiration    interface{}         `json:"Expiration"` // 可能是 null 或 string
		Concurrency   int                 `json:"concurrency"`
		PartSize      int                 `json:"part_size"`
		ContentLength int64               `json:"content_length"`
	} `json:"data"`
}

// 获取 OpenList 文件的真实下载链接和请求头
func OpenListGetFileURL(path, host, token string) (*FileInfoResponse, error) {
	apiURL := fmt.Sprintf("%s/api/fs/link", host)

	reqBody, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		return nil, fmt.Errorf("failed to create request body: %w", err)
	}

	resp, err := utils.MakeAuthenticatedRequest("POST", apiURL, string(reqBody), token)
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

// 验证 OpenList 的签名
func OpenListVerifySign(path, sign string, cfg *config.Config) bool {
	parts := strings.SplitN(sign, ":", 2)
	if len(parts) != 2 {
		return false
	}

	signature, expireTimestampStr := parts[0], parts[1]
	expireTS, err := strconv.ParseInt(expireTimestampStr, 10, 64)
	if err != nil {
		return false
	}

	if expireTS != 0 && time.Now().Unix() > expireTS {
		return false
	}

	var pathsToTest []string
	pathsToTest = append(pathsToTest, path)
	if strings.HasPrefix(path, "/") {
		pathsToTest = append(pathsToTest, path[1:])
	}

	for _, p := range pathsToTest {
		toSign := fmt.Sprintf("%s:%d", p, expireTS)
		h := hmac.New(sha256.New, []byte(cfg.Proxy.Token))
		h.Write([]byte(toSign))

		signatureWithPadding := base64.URLEncoding.EncodeToString(h.Sum(nil))
		signatureWithoutPadding := strings.TrimRight(signatureWithPadding, "=")

		if hmac.Equal([]byte(signature), []byte(signatureWithPadding)) {
			log.Printf("-> [Signature Debug] Signature matched for path: '%s' (with padding)", p)
			return true
		}
		if hmac.Equal([]byte(signature), []byte(signatureWithoutPadding)) {
			log.Printf("-> [Signature Debug] Signature matched for path: '%s' (without padding)", p)
			return true
		}
	}

	log.Printf("-> [Signature Debug] Signature did not match for any path variant. Original path: '%s'", path)
	return false
}

// OpenListURLResolver 现在通过复用签名来工作
type OpenListURLResolver struct {
	cfg      *config.Config
	basePath string // 主容器文件所在的逻辑目录，例如 "/encv/go/output"
}

// NewOpenListURLResolver 创建一个新的解析器实例，它将复用主容器文件的签名。
func NewOpenListURLResolver(cfg *config.Config, originalContainerPath string) *OpenListURLResolver {
	// 从原始路径中提取目录部分
	// originalContainerPath 示例: "/encv/go/output/321.4pm.sccgv"
	basePath := filepath.Dir(originalContainerPath) // 结果: "/encv/go/output"

	return &OpenListURLResolver{
		cfg:      cfg,
		basePath: basePath,
	}
}

// ResolveURL 为给定的物理分片路径获取一个带签名的 URL。
func (r *OpenListURLResolver) ResolveURL(physicalPath string) (string, error) {
	// 1. 构建物理分片的完整逻辑路径
	// physicalPath 示例: "321.4pm.0001"
	chunkLogicalPath := filepath.Join(r.basePath, physicalPath) // 在 Windows 上可能产生 "\encv\go\output\321.4pm.0001"

	// 【关键修复】强制将所有反斜杠替换为正斜杠，以兼容 Web 服务器
	chunkLogicalPath = strings.ReplaceAll(chunkLogicalPath, "\\", "/")

	log.Printf("DEBUG: [OpenListURLResolver] Resolved path for '%s' to '%s'", physicalPath, chunkLogicalPath)

	// 2. 调用 OpenListGetFileURL 函数为这个分片获取一个新的签名 URL
	fileInfo, err := OpenListGetFileURL(chunkLogicalPath, r.cfg.Proxy.OpenListHost, r.cfg.Proxy.Token)
	if err != nil {
		return "", fmt.Errorf("failed to get signed URL for path '%s': %w", chunkLogicalPath, err)
	}

	// 3. 返回获取到的 URL
	return fileInfo.Data.URL, nil
}
