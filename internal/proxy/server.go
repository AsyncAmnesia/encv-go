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

	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/types"
)

// --- 全局 MIME 类型映射表 ---
var contentTypes = map[string]string{
	// Text
	"txt":        "text/plain; charset=utf-8",
	"htm":        "text/html; charset=utf-8",
	"html":       "text/html; charset=utf-8",
	"xml":        "text/xml; charset=utf-8",
	"java":       "text/x-java-source; charset=utf-8",
	"properties": "text/plain; charset=utf-8",
	"sql":        "text/plain; charset=utf-8",
	"js":         "application/javascript; charset=utf-8",
	"md":         "text/plain; charset=utf-8",
	"json":       "application/json; charset=utf-8",
	"conf":       "text/plain; charset=utf-8",
	"ini":        "text/plain; charset=utf-8",
	"vue":        "text/plain; charset=utf-8",
	"php":        "text/plain; charset=utf-8",
	"py":         "text/x-python; charset=utf-8",
	"bat":        "text/plain; charset=utf-8",
	"gitignore":  "text/plain; charset=utf-8",
	"yml":        "application/x-yaml; charset=utf-8",
	"yaml":       "application/x-yaml; charset=utf-8",
	"go":         "text/plain; charset=utf-8",
	"sh":         "application/x-sh; charset=utf-8",
	"c":          "text/plain; charset=utf-8",
	"cpp":        "text/plain; charset=utf-8",
	"h":          "text/plain; charset=utf-8",
	"hpp":        "text/plain; charset=utf-8",
	"tsx":        "text/plain; charset=utf-8",
	"vtt":        "text/plain; charset=utf-8",
	"srt":        "text/plain; charset=utf-8",
	"ass":        "text/plain; charset=utf-8",
	"rs":         "text/plain; charset=utf-8",
	"lrc":        "text/plain; charset=utf-8",
	"strm":       "text/plain; charset=utf-8",

	// Audio
	"mp3":  "audio/mpeg",
	"flac": "audio/flac",
	"ogg":  "audio/ogg",
	"m4a":  "audio/mp4",
	"wav":  "audio/wav",
	"opus": "audio/opus",
	"wma":  "audio/x-ms-wma",

	// Video
	"mp4":  "video/mp4",
	"mkv":  "video/x-matroska",
	"avi":  "video/x-msvideo",
	"mov":  "video/quicktime",
	"rmvb": "application/vnd.rn-realmedia-vbr",
	"webm": "video/webm",
	"flv":  "video/x-flv",
	"m3u8": "application/vnd.apple.mpegurl",
	"enc":  "video/mp4", // 解密后是 mp4

	// Image
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"tiff": "image/tiff",
	"png":  "image/png",
	"gif":  "image/gif",
	"bmp":  "image/bmp",
	"svg":  "image/svg+xml",
	"ico":  "image/x-icon",
	"swf":  "application/x-shockwave-flash",
	"webp": "image/webp",
	"avif": "image/avif",
}

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
	if ct, ok := contentTypes[ext]; ok {
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

		log.Printf("Received valid request for: %s", path)
		log.Printf("-> [Proxy] Incoming request: %s from %s", path, r.RemoteAddr)

		// --- 步骤 1: 获取文件的真实下载链接 (对任何文件都一样) ---
		fileInfo, err := GetFileURL(path, cfg.OpenListHost, cfg.Token)
		if err != nil {
			log.Printf("Error getting file URL for %s: %v", path, err)
			http.Error(w, "Failed to locate file", http.StatusInternalServerError)
			return
		}

		// --- 步骤 2: 判断文件类型，决定如何处理下载的内容 ---
		if strings.HasSuffix(path, ".enc") {
			// --- 分支 A: 处理 .enc 加密文件 ---
			log.Printf("-> [Proxy] Handling .enc file: %s", path)
			kviPath := strings.TrimSuffix(path, ".enc") + ".kvi"

			kviFileInfo, err := GetFileURL(kviPath, cfg.OpenListHost, cfg.Token)
			if err != nil {
				log.Printf("Error getting .kvi file URL: %v", err)
				http.Error(w, "Failed to locate index file", http.StatusInternalServerError)
				return
			}

			index, err := downloadAndParseIndex(kviFileInfo.Data.URL, kviFileInfo.Data.Header)
			if err != nil {
				log.Printf("Error processing index file: %v", err)
				http.Error(w, "Failed to process index file", http.StatusInternalServerError)
				return
			}

			serveDecryptedStream(w, fileInfo.Data.URL, fileInfo.Data.Header, index, cfg.VideoPassword)

		} else if strings.HasPrefix(path, "/p/") { // 策略 B: 拦截并修复 OpenList 的内部下载链接 /p/...
			log.Printf("-> [Proxy] Intercepted internal link: %s", path)
			// 构造完整的 URL，这个 URL 已经是带签名的最终下载地址
			fileURL := cfg.OpenListHost + path + "?" + r.URL.RawQuery
			// 使用增强版的流式传输函数来处理
			serveDirectStreamWithFix(w, fileURL, nil) // nil headers because the URL is already signed
			return
		} else {
			// --- 分支 B: 代理普通文件 ---
			// 策略 C: 处理标准文件请求，通过 API 获取下载链接
			log.Printf("-> [Proxy] Handling standard file: %s", path)
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

// downloadAndParseIndex 下载并解析 .kvi 文件
func downloadAndParseIndex(url string, headers map[string]string) (*types.VideoIndex, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download index file, status: %s", resp.Status)
	}

	var index types.VideoIndex
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, err
	}
	return &index, nil
}

// serveDecryptedStream 下载 .enc 文件，解密后流式传输给客户端
func serveDecryptedStream(w http.ResponseWriter, encURL string, headers map[string]string, index *types.VideoIndex, password string) {
	req, err := http.NewRequest("GET", encURL, nil)
	if err != nil {
		http.Error(w, "Failed to create request for encrypted file", http.StatusInternalServerError)
		return
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Failed to download encrypted file", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Failed to download encrypted file, status: %s", resp.Status), http.StatusInternalServerError)
		return
	}

	// --- 准备解密 ---
	salt, err := crypto.Base64Decode(index.Encryption.SaltBase64)
	if err != nil {
		http.Error(w, "Invalid salt in index file", http.StatusInternalServerError)
		return
	}
	key := crypto.GenerateKey(password, salt)

	iv, err := crypto.Base64Decode(index.Encryption.IVBase64)
	if err != nil {
		http.Error(w, "Invalid IV in index file", http.StatusInternalServerError)
		return
	}

	// --- 设置响应头并开始流式传输 ---
	w.Header().Set("Content-Type", "video/"+index.Format)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 使用 crypto.DecryptStream 进行流式解密
	if err := crypto.DecryptStream(resp.Body, w, key, iv); err != nil {
		log.Printf("Error decrypting stream: %v", err)
	}
}
