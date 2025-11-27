package proxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/utils"
)

// Config 包含代理服务器所需的配置
type Config struct {
	Port                         int
	OpenListHost                 string
	Token                        string
	ContentPassword              string // 【修改】从 VideoPassword 改为 ContentPassword
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

// handleRequest 创建并返回 HTTP 处理函数
func handleRequest(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			log.Printf("-> [Proxy] Handling CORS preflight request for: %s", r.URL.Path)
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "*")
			w.Header().Set("Access-Control-Allow-Headers", "*")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Access-Control-Allow-Origin", "*") // 这里设置跨域是不起作用的哦
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
			if !OpenListVerifySign(path, sign, cfg) {
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
			fileInfo, err := OpenListGetFileURL(path, cfg.OpenListHost, cfg.Token)
			if err != nil {
				log.Printf("Error getting file URL for %s: %v", path, err)
				http.Error(w, "Failed to locate file", http.StatusInternalServerError)
				return
			}
			// 【修改】使用通用的密码字段
			// 在 handleRequest 函数中
			serveEncryptedContainer(w, fileInfo.Data.URL, fileInfo.Data.Header, cfg, path)
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
			fileInfo, err := OpenListGetFileURL(path, cfg.OpenListHost, cfg.Token)
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
func serveEncryptedContainer(w http.ResponseWriter, containerURL string, headers map[string]string, cfg *Config, originalPath string) {
	// 1. 检测容器类型
	magicMap := container.GetContainerMagicMap()
	maxMagicLen := 0
	for _, magic := range magicMap {
		if len(magic) > maxMagicLen {
			maxMagicLen = len(magic)
		}
	}

	containerStream, err := utils.GetRemoteStream(containerURL, headers)
	if err != nil {
		log.Printf("-> [Proxy] Failed to get container stream for type detection: %v", err)
		http.Error(w, "Failed to get container stream", http.StatusInternalServerError)
		return
	}
	defer containerStream.Close()

	magicHeader := make([]byte, maxMagicLen)
	bytesRead, err := io.ReadFull(containerStream, magicHeader)
	if err != nil && err != io.ErrUnexpectedEOF {
		log.Printf("-> [Proxy] Failed to read container magic header: %v", err)
		http.Error(w, "Invalid container file", http.StatusBadRequest)
		return
	}
	magicHeader = magicHeader[:bytesRead]

	detectedExt, err := container.DetectContainerType(magicHeader)
	if err != nil {
		log.Printf("-> [Proxy] Failed to detect container type: %v", err)
		http.Error(w, "Unknown container format", http.StatusBadRequest)
		return
	}
	log.Printf("-> [Proxy] Detected container extension: %s", detectedExt)

	// 2. 【关键】根据类型调用专门的处理器
	switch detectedExt {
	case config.GlobalConfig.BinExtGroup.Image:
		handleImageContainer(w, containerURL, headers, cfg)
	case config.GlobalConfig.BinExtGroup.Video:
		handleVideoContainer(w, containerURL, headers, cfg, originalPath)
	case config.GlobalConfig.BinExtGroup.Text: // 【新增】处理文本容器
		handleTextContainer(w, containerURL, headers, cfg)
	default:
		log.Printf("-> [Proxy] Unsupported container type: %s", detectedExt)
		http.Error(w, "Unsupported container type", http.StatusNotImplemented)
		return
	}
}

// 可以修复 CORS，直接从 URL 下载文件并流式传输给客户端
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

	w.WriteHeader(resp.StatusCode)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error streaming file to client: %v", err)
	}
}
