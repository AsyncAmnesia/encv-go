package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/container/chunked"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// 专门处理视频容器
func handleVideoContainer(w http.ResponseWriter, containerURL string, headers map[string]string, cfg *Config, originalPath string) {
	// 1. 使用范围请求精确获取主分片的头部信息
	magicMap := container.GetContainerMagicMap()
	mainMagic := magicMap[config.GlobalConfig.BinExtGroup.Video]

	headerData, err := utils.DownloadRange(containerURL, headers, 0, chunked.MainChunkHeaderSize-1)
	if err != nil {
		log.Printf("-> [Proxy] Failed to download main chunk header: %v", err)
		http.Error(w, "Failed to get container header", http.StatusInternalServerError)
		return
	}

	headerReader := bytes.NewReader(headerData)
	header, err := chunked.ReadMainHeader(headerReader, mainMagic)
	if err != nil {
		log.Printf("-> [Proxy] Failed to parse main chunk header: %v", err)
		http.Error(w, "Invalid container header", http.StatusBadRequest)
		return
	}

	// 2. 使用范围请求精确获取 KVI 数据
	kviStart := chunked.MainChunkHeaderSize
	kviEnd := kviStart + int64(header.KVILength) - 1
	kviData, err := utils.DownloadRange(containerURL, headers, kviStart, kviEnd)
	if err != nil {
		log.Printf("-> [Proxy] Failed to download KVI: %v", err)
		http.Error(w, "Failed to get container metadata", http.StatusInternalServerError)
		return
	}

	// 3. 解析 KVI
	index, err := utils.UnmarshalKVI(kviData)
	if err != nil {
		log.Printf("-> [Proxy] Failed to parse KVI: %v", err)
		http.Error(w, "Failed to parse container metadata", http.StatusInternalServerError)
		return
	}

	// 4. 创建一个提供子分片流的函数
	baseDir := filepath.Dir(originalPath)
	baseDir = strings.ReplaceAll(baseDir, "\\", "/")

	provider := func(subChunkInfo types.SubChunkInfo) (io.ReadCloser, error) {
		subChunkOriginalPath := baseDir + "/" + subChunkInfo.Filename
		log.Printf("-> [Proxy] Getting stream for sub-chunk: %s", subChunkOriginalPath)

		subChunkFileInfo, err := OpenListGetFileURL(subChunkOriginalPath, cfg.OpenListHost, cfg.Token)
		if err != nil {
			return nil, fmt.Errorf("failed to get signed URL for %s: %w", subChunkOriginalPath, err)
		}

		return utils.GetRemoteStream(subChunkFileInfo.Data.URL, subChunkFileInfo.Data.Header)
	}

	// 5. 获取主分片的数据流
	mainDataStart := kviEnd + 1
	mainDataReq, _ := http.NewRequest("GET", containerURL, nil)
	mainDataReq.Header.Set("Range", fmt.Sprintf("bytes=%d-", mainDataStart))
	for k, v := range headers {
		mainDataReq.Header.Set(k, v)
	}
	mainDataResp, err := (&http.Client{}).Do(mainDataReq)
	if err != nil {
		log.Printf("-> [Proxy] Failed to get main data stream: %v", err)
		http.Error(w, "Failed to get main data", http.StatusInternalServerError)
		return
	}
	if mainDataResp.StatusCode != http.StatusPartialContent {
		mainDataResp.Body.Close()
		log.Printf("-> [Proxy] Server does not support range requests for main data, status: %s", mainDataResp.Status)
		http.Error(w, "Server does not support range requests", http.StatusInternalServerError)
		return
	}

	// 6. 创建新的 StreamingChunkedReader
	streamingReader, err := chunked.StreamingReader(mainDataResp.Body, kviData, provider)
	if err != nil {
		mainDataResp.Body.Close()
		log.Printf("-> [Proxy] Failed to create streaming reader: %v", err)
		http.Error(w, "Failed to process container", http.StatusInternalServerError)
		return
	}

	// 7. 【关键】调用通用的解密服务函数，视频使用定长传输
	if err := serveDecryptedStreamFromReader(w, streamingReader, index, cfg.ContentPassword, false); err != nil {
		log.Printf("-> [Proxy] Failed to serve decrypted video stream: %v", err)
	}
}
