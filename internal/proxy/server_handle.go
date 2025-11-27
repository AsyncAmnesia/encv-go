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
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// serveDecryptedStreamFromReader 从一个 io.ReadCloser 解密并服务流
// useChunked 为 true 时，使用分块传输；为 false 时，使用定长传输
func serveDecryptedStreamFromReader(w http.ResponseWriter, encryptedReader io.ReadCloser, index types.Index, password string, useChunked bool) error {
	defer encryptedReader.Close()

	// 1. 从 KVI 获取解密所需信息
	salt, err := crypto.Base64Decode(index.GetEncryptionInfo().SaltBase64)
	if err != nil {
		log.Printf("Error decoding salt: %v", err)
		http.Error(w, "Invalid salt in index file", http.StatusInternalServerError)
		return err
	}
	key := crypto.GenerateKey(password, salt)

	// 2. 【关键】创建一个透明的解密流
	// 它会从 encryptedReader 读取加密数据，并吐出解密后的原始数据
	decryptedReader, err := crypto.GetDecryptReader(encryptedReader, key)
	if err != nil {
		log.Printf("Error creating decrypt reader: %v", err)
		http.Error(w, "Failed to initialize decryption", http.StatusInternalServerError)
		return err
	}

	// 3. 【关键】设置响应头，不设置 Content-Length
	// 这会让 Go 自动使用分块传输，最稳健的流式方案
	originalFilename := index.GetOriginalFilename()
	contentType := index.GetMimeType()

	log.Printf("-> [Proxy] Streaming content: %s (%s) using chunked encoding.", originalFilename, contentType)

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", originalFilename))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	// 不再设置 Content-Length 和 Accept-Ranges

	// 4. 【关键】将解密流直接泵送到 HTTP 响应体
	if _, err := io.Copy(w, decryptedReader); err != nil {
		// 客户端断开连接是正常行为，不应记录为错误
		if !utils.IsConnectionClosedError(err) {
			log.Printf("Error streaming decrypted content to client: %v", err)
		}
		return err
	}

	// // 2. 从 KVI 获取原始文件信息
	// originalFilename := index.GetOriginalFilename()
	// originalFileSize := index.GetOriginalFileSize()
	// contentType := index.GetMimeType()

	// log.Printf("-> [Proxy] Decrypting content: %s (%s, %d bytes, chunked: %t)", originalFilename, contentType, originalFileSize, useChunked)

	// // 3. 先解密到一个内存缓冲区
	// var buffer bytes.Buffer
	// written, err := crypto.DecryptStream(encryptedReader, &buffer, key)
	// if err != nil {
	// 	log.Printf("Error decrypting to buffer: %v", err)
	// 	http.Error(w, "Failed to decrypt content", http.StatusInternalServerError)
	// 	return err
	// }

	// if written != originalFileSize {
	// 	log.Printf("!!! [CRITICAL WARNING] Decrypted size (%d) does NOT match KVI size (%d).", written, originalFileSize)
	// }

	// // 4. 【关键】根据模式设置不同的响应头
	// w.Header().Set("Content-Type", contentType)
	// w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", originalFilename))

	// if useChunked {
	// 	// 动图模式：不设置 Content-Length，让 Go 自动使用分块传输
	// 	log.Printf("-> [Proxy] Streaming %d bytes to client using chunked encoding.", buffer.Len())
	// } else {
	// 	// 视频/静图模式：设置 Content-Length
	// 	log.Printf("-> [Proxy] Streaming %d bytes to client with fixed length.", buffer.Len())
	// 	w.Header().Set("Content-Length", strconv.FormatInt(originalFileSize, 10))
	// }

	// // 5. 将缓冲区内容写入 HTTP 响应
	// if _, err := io.Copy(w, &buffer); err != nil {
	// 	log.Printf("Error streaming buffer to client: %v", err)
	// 	return err
	// }

	return nil
}

// handleImageContainer 专门处理图片容器
func handleImageContainer(w http.ResponseWriter, containerURL string, headers map[string]string, cfg *Config) {
	// 1. 下载整个容器到内存
	log.Printf("-> [Proxy] Reading full image container into memory for reliable processing.")
	containerData, err := utils.ReadAllFromURL(containerURL, headers)
	if err != nil {
		log.Printf("-> [Proxy] Failed to read full image container: %v", err)
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
		log.Printf("-> [Proxy] Failed to unpack image container from memory: %v", err)
		http.Error(w, "Invalid container file", http.StatusBadRequest)
		return
	}
	defer packedData.DataStream.Close()

	// 从内存中的字节数组创建 Reader
	index, err := types.UnmarshalKVI(packedData.KVIData)
	if err != nil {
		log.Printf("-> [Proxy] Failed to parse KVI: %v", err)
		http.Error(w, "Failed to parse container metadata", http.StatusInternalServerError)
		return
	}

	// 3. 【关键】判断是否为动图，决定使用何种传输方式
	mimeType := index.GetMimeType()
	useChunked := mimeType == "image/gif" || mimeType == "image/webp" // WebP 动图也用 chunked

	// 4. 调用通用的解密服务函数
	if err := serveDecryptedStreamFromReader(w, packedData.DataStream, index, cfg.ContentPassword, useChunked); err != nil {
		log.Printf("-> [Proxy] Failed to serve decrypted image stream: %v", err)
	}
}

// handleVideoContainer 专门处理视频容器
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
	index, err := types.UnmarshalKVI(kviData)
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

// handleTextContainer 专门处理文本容器
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
	index, err := types.UnmarshalKVI(packedData.KVIData)
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
