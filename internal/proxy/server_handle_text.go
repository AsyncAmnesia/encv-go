package proxy

import (
	"bytes"
	"log"
	"net/http"

	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/utils"
)

// 专门处理文本容器
func handleTextContainer(w http.ResponseWriter, containerURL string, headers map[string]string, cfg *Config) {
	// 1. 下载整个容器到内存
	log.Printf("-> [Proxy] Reading full text container into memory for reliable processing.")
	containerData, err := utils.ReadAllFromURL(containerURL, headers)
	if err != nil {
		log.Printf("-> [Proxy] Failed to read full text container: %v", err)
		http.Error(w, "Failed to read container", http.StatusInternalServerError)
		return
	}

	// 2. 从内存中检测类型和解包
	detectedExt, err := container.DetectContainerType(containerData)
	if err != nil {
		log.Printf("-> [Proxy] Failed to detect container type from memory: %v", err)
		http.Error(w, "Unknown container format", http.StatusBadRequest)
		return
	}
	log.Printf("-> [Proxy] Detected container extension: %s", detectedExt)

	reader := bytes.NewReader(containerData)
	magicMap := container.GetContainerMagicMap()
	packedData, err := container.Unpack(reader, magicMap[detectedExt])
	if err != nil {
		log.Printf("-> [Proxy] Failed to unpack text container from memory: %v", err)
		http.Error(w, "Invalid container file", http.StatusBadRequest)
		return
	}
	defer packedData.DataStream.Close()

	// 3. 解析 KVI
	index, err := utils.UnmarshalKVI(packedData.KVIData)
	if err != nil {
		log.Printf("-> [Proxy] Failed to parse KVI: %v", err)
		http.Error(w, "Failed to parse container metadata", http.StatusInternalServerError)
		return
	}

	// 4. 【关键修改】调用通用的解密服务函数
	// 文本不需要像GIF/WebP那样的特殊分块传输，所以 useChunked 为 false
	if err := serveDecryptedStreamFromReader(w, packedData.DataStream, index, cfg.ContentPassword, true); err != nil {
		log.Printf("-> [Proxy] Failed to serve decrypted text stream: %v", err)
	}
}
