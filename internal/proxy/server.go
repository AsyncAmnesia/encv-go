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

// isEncryptedFile 通过检查文件头判断 URL 指向的文件是否是 encv 加密文件
func isEncryptedFile(fileURL string, headers map[string]string) bool {
	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		log.Printf("-> [Proxy] Failed to create request for file header check: %v", err)
		return false
	}

	// 设置 Range 头，只请求文件的前 N 个字节
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", crypto.MagicNumberLength-1))
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("-> [Proxy] Failed to get file header: %v", err)
		return false
	}
	defer resp.Body.Close()

	// 检查服务器是否支持 Range 请求
	if resp.StatusCode != http.StatusPartialContent {
		log.Printf("-> [Proxy] Server does not support range requests for header check (status: %s), assuming not encrypted.", resp.Status)
		return false
	}

	magicBytes := make([]byte, crypto.MagicNumberLength)
	_, err = io.ReadFull(resp.Body, magicBytes)
	if err != nil {
		log.Printf("-> [Proxy] Failed to read file header: %v", err)
		return false
	}

	return string(magicBytes) == crypto.MagicNumber
}

// tryGetKVI 尝试获取并解析与给定路径对应的 KVI 文件
func tryGetKVI(path string, cfg *Config) (*types.VideoIndex, error) {
	// 生成一个候选 KVI 路径的列表，按优先级排序
	candidates := []string{}
	baseFilename := filepath.Base(path)
	dir := filepath.Dir(path)

	// --- 策略 1: 最高优先级 - 如果以 .enc 结尾，直接构造 ---
	// 这是最标准、最可靠的情况
	if strings.HasSuffix(path, ".enc") {
		kviPath := strings.TrimSuffix(path, ".enc") + ".kvi"
		candidates = append(candidates, kviPath)
	}

	// --- 策略 2: 中等优先级 - 假设用户只修改了最后的后缀 ---
	// 例如：movie.4pm.enc -> movie.4pm.en
	// 我们取文件名，去掉最后一个后缀，然后加上 .kvi
	baseName := strings.TrimSuffix(baseFilename, filepath.Ext(baseFilename))
	candidates = append(candidates, filepath.Join(dir, baseName+".kvi"))

	// --- 策略 3: 最低优先级 - 假设用户恢复了原始后缀 ---
	// 例如：movie.4pm.enc -> movie.mp4
	// 我们需要反向查找映射表
	originalExt := strings.TrimPrefix(filepath.Ext(baseFilename), ".")
	if encExt, ok := config.ContainerExtensionMap[originalExt]; ok {
		// movie.mp4 -> movie.4pm.kvi
		baseName := strings.TrimSuffix(baseFilename, filepath.Ext(baseFilename))
		candidates = append(candidates, filepath.Join(dir, baseName+encExt+".kvi"))
	}

	log.Printf("-> [Proxy] Trying KVI candidates for %s: %v", path, candidates)

	// 遍历所有候选路径，尝试下载和解析
	for _, kviPath := range candidates {
		// 【关键修复】将 Windows 路径分隔符替换为 URL 路径分隔符
		kviPathForAPI := strings.ReplaceAll(kviPath, "\\", "/")
		log.Printf("-> [Proxy] Attempting API call for KVI: %s", kviPathForAPI)

		kviFileInfo, err := GetFileURL(kviPathForAPI, cfg.OpenListHost, cfg.Token)
		if err != nil {
			if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
				log.Printf("-> [Proxy] Candidate not found: %s", kviPathForAPI)
				continue
			}
			return nil, err
		}

		index, parseErr := downloadAndParseIndex(kviFileInfo.Data.URL, kviFileInfo.Data.Header)
		if parseErr != nil {
			if strings.Contains(parseErr.Error(), "status: 404") || strings.Contains(parseErr.Error(), "status: 500") {
				log.Printf("-> [Proxy] Candidate failed to parse: %s", kviPathForAPI)
				continue
			}
			return nil, fmt.Errorf("failed to parse KVI candidate %s: %w", kviPathForAPI, parseErr)
		}

		log.Printf("-> [Proxy] Successfully found and parsed KVI: %s", kviPathForAPI)
		return index, nil
	}

	return nil, fmt.Errorf("kvi not found for any candidate path")
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

		// --- 步骤 1: 获取文件的真实下载链接 (对任何文件都一样) ---
		// fileInfo, err := GetFileURL(path, cfg.OpenListHost, cfg.Token)
		// if err != nil {
		// 	log.Printf("Error getting file URL for %s: %v", path, err)
		// 	http.Error(w, "Failed to locate file", http.StatusInternalServerError)
		// 	return
		// }

		// --- 核心逻辑变更：先找 KVI，再验证文件头 ---
		index, err := tryGetKVI(path, cfg)
		if err == nil {
			// --- 找到了 KVI，现在验证请求的文件本身是否是加密视频 ---
			log.Printf("-> [Proxy] Found KVI candidate for %s, verifying file header...", path)

			fileInfo, err := GetFileURL(path, cfg.OpenListHost, cfg.Token)
			if err != nil {
				log.Printf("Error getting file URL for verification %s: %v", path, err)
				http.Error(w, "Failed to locate file", http.StatusInternalServerError)
				return
			}

			if isEncryptedFile(fileInfo.Data.URL, fileInfo.Data.Header) {
				// --- 验证通过：确实是加密文件，进行解密 ---
				log.Printf("-> [Proxy] File header verified. Handling as encrypted file: %s", path)
				serveDecryptedStream(w, fileInfo.Data.URL, fileInfo.Data.Header, index, cfg.VideoPassword)
				return
			}

			// --- 验证失败：不是加密文件（例如是字幕），按普通文件处理 ---
			log.Printf("-> [Proxy] File header verification failed. Handling %s as a standard file.", path)
		} else {
			// --- 没找到 KVI，直接按普通文件处理 ---
			log.Printf("-> [Proxy] No KVI found for %s, handling as standard file.", path)
		}

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

	// 【关键修复】使用一个新的 client 实例，而不是 http.DefaultClient
	client := &http.Client{}
	resp, err := client.Do(req)
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

	// iv, err := crypto.Base64Decode(index.Encryption.IVBase64)
	// if err != nil {
	// 	http.Error(w, "Invalid IV in index file", http.StatusInternalServerError)
	// 	return
	// }

	// --- 设置响应头并开始流式传输 ---
	w.Header().Set("Content-Type", "video/"+index.Format)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 使用 crypto.DecryptStream 进行流式解密
	if err := crypto.DecryptStream(resp.Body, w, key); err != nil {
		log.Printf("Error decrypting stream: %v", err)
	}
}
