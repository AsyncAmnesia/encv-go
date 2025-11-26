package proxy

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
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/types"
)

// Config 包含代理服务器所需的配置
type Config struct {
	Port                         int
	OpenListHost                 string
	Token                        string
	VideoPassword                string
	DisableSignatureVerification bool
}

// StartServer 启动代理服务器
func StartServer(cfg *Config) {
	http.HandleFunc("/", handleRequest(cfg))
	log.Printf("Starting ENCV proxy server on port %d", cfg.Port)
	log.Printf("Proxying for OpenList at: %s", cfg.OpenListHost)
	if cfg.DisableSignatureVerification {
		log.Println("!!! WARNING: Signature verification is DISABLED. This is insecure and should only be used for testing. !!!")
	}
	if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), nil); err != nil {
		log.Fatalf("Failed to start proxy server: %v", err)
	}
}

// getContentTypeFromExtension 根据文件扩展名获取 Content-Type
func getContentTypeFromExtension(fileURL string) string {
	// 使用 filepath.Ext 获取扩展名，它包含点号 (例如 ".txt")
	ext := strings.ToLower(filepath.Ext(fileURL))
	if len(ext) > 0 {
		ext = ext[1:] // 去掉点号
	}
	if ct, ok := config.ContentTypes[ext]; ok {
		return ct
	}
	// 如果找不到，返回默认的二进制流类型
	return "application/octet-stream"
}

// handleRequest 创建并返回 HTTP 处理函数
func handleRequest(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 浏览器在发起复杂跨域请求前，会先发送一个 OPTIONS 请求来询问服务器是否允许
		if r.Method == http.MethodOptions {
			log.Printf("-> [Proxy] Handling CORS preflight request for: %s", r.URL.Path)
			// 告诉浏览器，我们允许任何源的 GET, POST, OPTIONS 请求
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "*")
			w.Header().Set("Access-Control-Allow-Headers", "*")
			w.WriteHeader(http.StatusNoContent) // 204 状态码表示预检成功
			return
		}

		path := r.URL.Path
		sign := r.URL.Query().Get("sign")
		isInternalRequest := r.URL.Query().Get("internal_request") == "true"

		if path == "" {
			http.Error(w, "Missing 'path' parameter", http.StatusBadRequest)
			return
		}

		// --- 签名验证逻辑对两种文件类型都适用 ---
		if !cfg.DisableSignatureVerification {
			if sign == "" {
				http.Error(w, "Missing 'sign' parameter", http.StatusBadRequest)
				return
			}
			if !verifySign(path, sign, cfg) {
				log.Printf("Invalid signature for path: %s", path)
				http.Error(w, "Forbidden: Invalid signature", http.StatusForbidden)
				return
			}
		} else {
			log.Printf("-> [Security] Signature verification is disabled, allowing request for: %s", path)
		}

		// 如果是内部请求，则跳过签名验证
		if !isInternalRequest {
			if !cfg.DisableSignatureVerification {
				if sign == "" {
					http.Error(w, "Missing 'sign' parameter", http.StatusBadRequest)
					return
				}
				if !verifySign(path, sign, cfg) {
					log.Printf("Invalid signature for path: %s", path)
					http.Error(w, "Forbidden: Invalid signature", http.StatusForbidden)
					return
				}
			} else {
				log.Printf("-> [Security] Signature verification is disabled, allowing request for: %s", path)
			}
		} else {
			log.Printf("-> [Proxy] Handling internal request, skipping signature check for: %s", path)
		}

		log.Printf("Received valid request for: %s", path)
		log.Printf("-> [Proxy] Incoming request: %s from %s", path, r.RemoteAddr)

		// --- 核心逻辑：判断是否是容器文件 ---
		if config.IsContainerFile(path) {
			log.Printf("-> [Proxy] Detected container file: %s", path)
			fileInfo, err := GetFileURL(path, cfg.OpenListHost, cfg.Token)
			if err != nil {
				http.Error(w, "Failed to locate file", http.StatusInternalServerError)
				return
			}
			// 解包并提供服务
			serveContainerStream(w, fileInfo.Data.URL, fileInfo.Data.Header, cfg.VideoPassword)
			return
		}

		// --- 如果不是容器文件，则按普通文件处理 ---
		log.Printf("-> [Proxy] Not a container file, handling as standard file: %s", path)

		// --- 统一的普通文件处理逻辑 ---
		if strings.HasPrefix(path, "/p/") {
			// 策略 B: 拦截并修复 OpenList 的内部下载链接 /p/...
			log.Printf("-> [Proxy] Intercepted internal link: %s", path)
			fileURL := cfg.OpenListHost + path + "?" + r.URL.RawQuery
			serveDirectStreamWithFix(w, fileURL, nil)
			return
		} else {
			// 策略 C: 处理标准文件请求，通过 API 获取下载链接
			fileInfo, err := GetFileURL(path, cfg.OpenListHost, cfg.Token)
			if err != nil {
				log.Printf("Error getting file URL for %s: %v", path, err)
				http.Error(w, "Failed to locate file", http.StatusInternalServerError)
				return
			}
			serveDirectStreamWithFix(w, fileInfo.Data.URL, fileInfo.Data.Header)
		}
	}
}

func serveContainerStream(w http.ResponseWriter, containerURL string, headers map[string]string, password string) {
	// 1. 从 OpenList 获取容器文件的流
	containerStream, err := getRemoteStream(containerURL, headers)
	if err != nil {
		log.Printf("-> [Proxy] Failed to get container stream: %v", err)
		http.Error(w, "Failed to get container stream", http.StatusInternalServerError)
		return
	}
	defer containerStream.Close()

	// 2. 解包
	packedData, err := container.Unpack(containerStream)
	if err != nil {
		log.Printf("-> [Proxy] Failed to unpack container: %v", err)
		http.Error(w, "Invalid container file", http.StatusBadRequest)
		return
	}
	defer packedData.VideoStream.Close()

	// 3. 解析 KVI
	var index types.VideoIndex
	if err := json.Unmarshal(packedData.KVIData, &index); err != nil {
		log.Printf("-> [Proxy] Failed to parse KVI from container: %v", err)
		http.Error(w, "Failed to parse KVI", http.StatusInternalServerError)
		return
	}

	// 4. 解密视频流并服务给客户端
	if err := serveDecryptedStreamFromReader(w, packedData.VideoStream, &index, password); err != nil {
		// 错误已在 serveDecryptedStreamFromReader 内部处理，这里只记录
		log.Printf("-> [Proxy] Failed to serve decrypted stream: %v", err)
	}
}

// serveDecryptedStreamFromReader 从一个 io.Reader 解密并服务流
// 它现在直接将解密后的流写入 http.ResponseWriter
func serveDecryptedStreamFromReader(w http.ResponseWriter, encryptedReader io.Reader, index *types.VideoIndex, password string) error {
	// --- 准备解密密钥 ---
	salt, err := crypto.Base64Decode(index.Encryption.SaltBase64)
	if err != nil {
		log.Printf("Error decoding salt: %v", err)
		http.Error(w, "Invalid salt in index file", http.StatusInternalServerError)
		return err
	}
	key := crypto.GenerateKey(password, salt)

	// --- 设置响应头 ---
	w.Header().Set("Content-Type", "video/"+index.Format)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// --- 调用 crypto.DecryptStream 直接解密到 w ---
	if err := crypto.DecryptStream(encryptedReader, w, key); err != nil {
		log.Printf("Error decrypting stream: %v", err)
		// 注意：由于已经开始写入响应头，这里不能再调用 http.Error
		// 客户端可能会收到一个不完整的响应
		return err
	}

	return nil
}

// serveDirectStreamWithFix 是 serveDirectStream 的增强版，可以修复响应头和 CORS，直接从 URL 下载文件并流式传输给客户端
func serveDirectStreamWithFix(w http.ResponseWriter, fileURL string, headers map[string]string) {
	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		log.Printf("Error creating request to download file: %v", err)
		http.Error(w, "Failed to download file", http.StatusInternalServerError)
		return
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error downloading file from %s: %v", fileURL, err)
		http.Error(w, "Failed to download file", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// 将上游的响应头复制给客户端
	for key, values := range resp.Header {
		w.Header()[key] = values
	}

	// --- 关键修复点 1: 添加 CORS 头 ---
	// 这允许任何网页访问我们代理的资源
	w.Header().Set("Access-Control-Allow-Origin", "*")
	log.Printf("-> [Proxy Fix] Added CORS headers for request: %s", fileURL)

	// --- 关键修复点 2: 根据文件扩展名强制设置正确的 Content-Type ---
	contentType := getContentTypeFromExtension(fileURL)
	w.Header().Set("Content-Type", contentType)
	log.Printf("-> [Proxy Fix] Forced Content-Type to '%s' for file: %s", contentType, fileURL)

	w.WriteHeader(resp.StatusCode)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error streaming file to client: %v", err)
	}
}

// getRemoteStream 创建一个 HTTP GET 请求并返回响应体的 ReadCloser
func getRemoteStream(fileURL string, headers map[string]string) (io.ReadCloser, error) {
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
		resp.Body.Close() // 确保在错误情况下关闭 Body
		return nil, fmt.Errorf("remote server returned status %s for %s", resp.Status, fileURL)
	}

	return resp.Body, nil
}

// verifySign 验证 OpenList 的签名
func verifySign(path, sign string, cfg *Config) bool {
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

	// 构建所有可能的待签名路径
	var pathsToTest []string

	// 2. 始终尝试完整路径（作为后备）
	pathsToTest = append(pathsToTest, path)
	if strings.HasPrefix(path, "/") {
		pathsToTest = append(pathsToTest, path[1:])
	}

	// 对所有可能的路径进行签名匹配
	for _, p := range pathsToTest {
		toSign := fmt.Sprintf("%s:%d", p, expireTS)
		h := hmac.New(sha256.New, []byte(cfg.Token))
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
