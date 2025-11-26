package proxy

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
	ext := strings.ToLower(filepath.Ext(fileURL))
	if len(ext) > 0 {
		ext = ext[1:]
	}
	if ct, ok := config.ContentTypes[ext]; ok {
		return ct
	}
	return "application/octet-stream"
}

// handleRequest 创建并返回 HTTP 处理函数
func handleRequest(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			log.Printf("-> [Proxy] Handling CORS preflight request for: %s", r.URL.Path)
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "*")
			w.Header().Set("Access-Control-Allow-Headers", "*")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		path := r.URL.Path
		sign := r.URL.Query().Get("sign")
		isInternalRequest := r.URL.Query().Get("internal_request") == "true"

		if path == "" {
			http.Error(w, "Missing 'path' parameter", http.StatusBadRequest)
			return
		}

		if !isInternalRequest && !cfg.DisableSignatureVerification {
			if sign == "" {
				http.Error(w, "Missing 'sign' parameter", http.StatusBadRequest)
				return
			}
			if !verifySign(path, sign, cfg) {
				log.Printf("Invalid signature for path: %s", path)
				http.Error(w, "Forbidden: Invalid signature", http.StatusForbidden)
				return
			}
		} else if cfg.DisableSignatureVerification {
			log.Printf("-> [Security] Signature verification is disabled, allowing request for: %s", path)
		} else {
			log.Printf("-> [Proxy] Handling internal request, skipping signature check for: %s", path)
		}

		log.Printf("Received valid request for: %s", path)

		// --- 核心逻辑：判断是否是 ENCV 容器文件 ---
		// 【关键修改】使用新的通用检查函数
		if config.IsContainerPath(path) {
			log.Printf("-> [Proxy] Detected ENCV container file: %s", path)
			fileInfo, err := GetFileURL(path, cfg.OpenListHost, cfg.Token)
			if err != nil {
				log.Printf("Error getting file URL for %s: %v", path, err)
				http.Error(w, "Failed to locate file", http.StatusInternalServerError)
				return
			}
			// 【关键修改】调用通用的容器处理函数
			serveEncryptedContainer(w, fileInfo.Data.URL, fileInfo.Data.Header, cfg.VideoPassword)
			return
		}

		// --- 如果不是容器文件，则按普通文件处理 ---
		log.Printf("-> [Proxy] Not a container file, handling as standard file: %s", path)
		if strings.HasPrefix(path, "/p/") {
			log.Printf("-> [Proxy] Intercepted internal link: %s", path)
			fileURL := cfg.OpenListHost + path + "?" + r.URL.RawQuery
			serveDirectStreamWithFix(w, fileURL, nil)
			return
		} else {
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

// serveEncryptedContainer 通用地处理所有 ENCV 容器
func serveEncryptedContainer(w http.ResponseWriter, containerURL string, headers map[string]string, password string) {
	// 1. 从 OpenList 获取容器文件的流
	containerStream, err := getRemoteStream(containerURL, headers)
	if err != nil {
		log.Printf("-> [Proxy] Failed to get container stream: %v", err)
		http.Error(w, "Failed to get container stream", http.StatusInternalServerError)
		return
	}
	defer containerStream.Close()

	// 2. 读取文件头用于类型检测
	maxMagicLen := len(crypto.SccgvContainerMagicNumber) // 假设这是最长的
	magicHeader := make([]byte, maxMagicLen)
	bytesRead, err := io.ReadFull(containerStream, magicHeader)
	if err != nil && err != io.ErrUnexpectedEOF {
		log.Printf("-> [Proxy] Failed to read container magic header: %v", err)
		http.Error(w, "Invalid container file", http.StatusBadRequest)
		return
	}
	magicHeader = magicHeader[:bytesRead]

	// 3. 使用 io.MultiReader 将读取的头部和剩余的流组合起来
	peekedStream := io.MultiReader(bytes.NewReader(magicHeader), containerStream)

	// 4. 检测容器类型，现在返回的是扩展名
	detectedExt, err := container.DetectContainerType(magicHeader)
	if err != nil {
		log.Printf("-> [Proxy] Failed to detect container type: %v", err)
		http.Error(w, "Unknown container format", http.StatusBadRequest)
		return
	}
	log.Printf("-> [Proxy] Detected container extension: %s", detectedExt)

	// 5. 【关键修改】根据从配置中获取的扩展名进行解包
	var packedData *container.PackedData
	switch detectedExt {
	case config.GlobalConfig.BinExtGroup.Video:
		// 视频容器需要下载到临时文件
		tempFile, err := os.CreateTemp("", "encv-main-chunk-*."+detectedExt)
		if err != nil {
			log.Printf("-> [Proxy] Failed to create temp file: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		tempPath := tempFile.Name()
		defer os.Remove(tempPath)
		defer tempFile.Close()

		if _, err := io.Copy(tempFile, peekedStream); err != nil {
			log.Printf("-> [Proxy] Failed to download main chunk to temp file: %v", err)
			http.Error(w, "Failed to process container", http.StatusInternalServerError)
			return
		}
		tempFile.Close()

		packedData, err = container.UnpackChunked(tempPath)

	// case config.GlobalConfig.BinExtGroup.Text:
	//     packedData, err = container.UnpackText(peekedStream) // 未来实现
	// case config.GlobalConfig.BinExtGroup.Audio:
	//     packedData, err = container.UnpackAudio(peekedStream) // 未来实现
	// case config.GlobalConfig.BinExtGroup.Image:
	//     packedData, err = container.UnpackImage(peekedStream) // 未来实现

	default:
		log.Printf("-> [Proxy] Unsupported container type: %s", detectedExt)
		http.Error(w, "Unsupported container type", http.StatusNotImplemented)
		return
	}

	if err != nil {
		log.Printf("-> [Proxy] Failed to unpack container: %v", err)
		http.Error(w, "Invalid container file", http.StatusBadRequest)
		return
	}
	defer packedData.VideoStream.Close()

	// 6. 解析 KVI 并解密流 (后续逻辑保持不变)
	index, err := crypto.UnmarshalKVI(packedData.KVIData)
	if err != nil {
		log.Printf("-> [File] Failed to parse KVI: %v", err)
		http.Error(w, "Failed to parse container metadata", http.StatusInternalServerError)
		return
	}

	if err := serveDecryptedStreamFromReader(w, packedData.VideoStream, index, password); err != nil {
		log.Printf("-> [Proxy] Failed to serve decrypted stream: %v", err)
	}
}

// serveDecryptedStreamFromReader 从一个 io.Reader 解密并服务流
func serveDecryptedStreamFromReader(w http.ResponseWriter, encryptedReader io.Reader, index *types.VideoIndex, password string) error {
	salt, err := crypto.Base64Decode(index.Encryption.SaltBase64)
	if err != nil {
		log.Printf("Error decoding salt: %v", err)
		http.Error(w, "Invalid salt in index file", http.StatusInternalServerError)
		return err
	}
	key := crypto.GenerateKey(password, salt)

	w.Header().Set("Content-Type", "video/"+index.Format)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if err := crypto.DecryptStream(encryptedReader, w, key); err != nil {
		log.Printf("Error decrypting stream: %v", err)
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

	for key, values := range resp.Header {
		w.Header()[key] = values
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	log.Printf("-> [Proxy Fix] Added CORS headers for request: %s", fileURL)

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
		resp.Body.Close()
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

	var pathsToTest []string
	pathsToTest = append(pathsToTest, path)
	if strings.HasPrefix(path, "/") {
		pathsToTest = append(pathsToTest, path[1:])
	}

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
