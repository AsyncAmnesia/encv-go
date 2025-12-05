package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/Soltus/encv-go/internal/utils"
	service_v2 "github.com/Soltus/encv-go/internal/v2/service"
)

// 处理可寻址的加密容器，支持 HTTP Range 请求以实现进度跳转
func (s *Server) serveSeekableContainer(w http.ResponseWriter, r *http.Request, fullPath string) {
	log.Printf("INFO: Entering serveFullContent for file: %s", fullPath)

	// 1. 获取解密器、索引和大小
	// 【关键简化】manager 不再需要
	decryptReader, index, originalSize, err := service_v2.NewSeekableDecryptReaderFromContainer_v2(r.Context(), fullPath, nil)
	// decryptReader, index, originalSize, err := service_v2.NewSeekableDecryptReaderFromContainer_v2(r.Context(), fullPath, s.containerManager)
	if err != nil {
		log.Printf("ERROR: [serveSeekableContainer] Failed to create decrypt reader for %s: %v", fullPath, err)
		http.Error(w, "Could not initialize decryption: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer decryptReader.Close()
	log.Printf("INFO: Successfully created decrypt reader for %s. Original filename: %s, Original Size: %d", fullPath, index.GetOriginalFilename(), originalSize)

	// 2. 处理 Range 请求
	rangeHeader := r.Header.Get("Range")
	var start, end int64
	statusCode := http.StatusOK

	if rangeHeader != "" {
		// 使用正则表达式解析 "bytes=start-end" 格式
		re := regexp.MustCompile(`bytes=(\d+)-(\d*)`)
		matches := re.FindStringSubmatch(rangeHeader)
		if len(matches) == 3 {
			start, _ = strconv.ParseInt(matches[1], 10, 64)
			if matches[2] != "" {
				end, _ = strconv.ParseInt(matches[2], 10, 64)
			} else {
				end = originalSize - 1 // 如果没有指定结束位置，则到文件末尾
			}

			// 验证范围的有效性
			if start >= originalSize || end >= originalSize || start > end {
				log.Printf("WARN: Invalid range requested: %s", rangeHeader)
				w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", originalSize))
				http.Error(w, "Requested Range Not Satisfiable", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			statusCode = http.StatusPartialContent
		}
	} else {
		// 没有 Range 请求，传输整个文件
		start = 0
		end = originalSize - 1
	}

	// 3. Seek 到请求的起始位置
	_, err = decryptReader.Seek(start, io.SeekStart)
	if err != nil {
		log.Printf("ERROR: Failed to seek to position %d: %v", start, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 4. 设置响应头
	originalFilename := index.GetOriginalFilename()
	w.Header().Set("Content-Type", utils.GetContentType(filepath.Ext(originalFilename)))
	w.Header().Set("Accept-Ranges", "bytes") // 告诉客户端我们支持 Range 请求
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", originalFilename))

	contentLength := end - start + 1
	w.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))

	if statusCode == http.StatusPartialContent {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, originalSize))
	}

	// 5. 显式写入状态码
	w.WriteHeader(statusCode)

	// 6. 【核心】使用 io.LimitReader 限制读取的字节数，然后流式传输
	readerToCopy := io.LimitReader(decryptReader, contentLength)
	writtenBytes, err := io.Copy(w, readerToCopy)
	if err != nil {
		// 在流式传输过程中发生错误，通常是客户端断开连接
		log.Printf("WARN: Stream to client was interrupted or failed for %s: %v", fullPath, err)
		return
	}

	log.Printf("INFO: Successfully streamed %d bytes to client for file %s.", writtenBytes, fullPath)
}
