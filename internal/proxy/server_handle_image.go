package proxy

import (
	"bytes"
	"context"
	"log"
	"net/http"

	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/utils"
)

// 专门处理图片容器
func handleImageContainer(ctx context.Context, w http.ResponseWriter, containerURL string, headers map[string]string) {
	// 1. 下载整个容器到内存
	log.Printf("-> [Proxy] Reading full image container into memory for reliable processing.")
	containerData, err := utils.ReadAllFromURL(containerURL, headers)
	if err != nil {
		log.Printf("-> [Proxy] Failed to read full image container: %v", err)
		http.Error(w, "Failed to read container", http.StatusInternalServerError)
		return
	}

	// 2. 从内存中检测类型和解包
	detectedExt, err := container.DetectContainerType(ctx, containerData)
	if err != nil {
		log.Printf("-> [Proxy] Failed to detect container type from memory: %v", err)
		http.Error(w, "Unknown container format", http.StatusBadRequest)
		return
	}
	log.Printf("-> [Proxy] Detected container extension: %s", detectedExt)

	reader := bytes.NewReader(containerData)
	magicMap, err := container.GetContainerMagicMap(ctx)
	packedData, err := container.Unpack(reader, magicMap[detectedExt])
	if err != nil {
		log.Printf("-> [Proxy] Failed to unpack image container from memory: %v", err)
		http.Error(w, "Invalid container file", http.StatusBadRequest)
		return
	}
	defer packedData.DataStream.Close()

	// 从内存中的字节数组创建 Reader
	index, err := utils.UnmarshalKVI(packedData.KVIData)
	if err != nil {
		log.Printf("-> [Proxy] Failed to parse KVI: %v", err)
		http.Error(w, "Failed to parse container metadata", http.StatusInternalServerError)
		return
	}

	// 3. 【关键】判断是否为动图，决定使用何种传输方式
	mimeType := index.GetMimeType()
	useChunked := mimeType == "image/gif" || mimeType == "image/webp" // WebP 动图也用 chunked

	// 4. 调用通用的解密服务函数
	if err := serveDecryptedStreamFromReader(ctx, w, packedData.DataStream, index, useChunked); err != nil {
		log.Printf("-> [Proxy] Failed to serve decrypted image stream: %v", err)
	}
}
