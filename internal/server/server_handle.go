package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	"github.com/Soltus/encv-go/internal/v2/reader"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// handleStreamRequest 处理 /stream?file=... 格式的请求
func (s *Server) handleStreamRequest(w http.ResponseWriter, r *http.Request) {
	// 1. 从查询参数中获取文件的绝对路径
	filePath := r.URL.Query().Get("file")
	if filePath == "" {
		http.Error(w, "Bad Request: 'file' query parameter is missing", http.StatusBadRequest)
		return
	}

	// 2. 【安全】进行路径清理和验证
	// 清理路径，防止路径遍历攻击（虽然这里是绝对路径，但这是个好习惯）
	cleanedFilePath := filepath.Clean(filePath)

	// 【重要】确保提供的路径在允许的服务目录内，防止恶意用户访问系统任意文件
	// 我们使用 filepath.Rel 来计算相对路径。如果结果以 ".." 开头，说明文件在 servingDir 之外
	relPath, err := filepath.Rel(s.servingDir, cleanedFilePath)
	if err != nil {
		http.Error(w, "Internal Server Error: could not resolve path", http.StatusInternalServerError)
		return
	}
	if strings.HasPrefix(relPath, "..") {
		http.Error(w, "Forbidden: access to path outside serving directory is not allowed", http.StatusForbidden)
		return
	}
	// 【核心】调用我们新的、统一的加密文件处理函数
	s.serveEncryptedFile(w, r, cleanedFilePath, true) // true 表示这是一个流式请求
}

// handlePing 处理 /ping 请求，返回带有服务信息的 JSON
func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	response := types.PingResponse{
		Status:        types.ServiceStatuses.OK,
		Version:       s.version,
		InstanceID:    s.instanceID,
		ServerDirPath: s.servingDir,
		WebdavDirPath: s.webdavDir, // 如果未启用，则为空字符串
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding ping response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// serveEncryptedFile 是一个全新的、统一的处理函数，用于处理所有加密容器
// 它会自动判断容器类型（可寻址/不可寻址）并采取最优策略

func (s *Server) serveEncryptedFile(w http.ResponseWriter, r *http.Request, fullPath string, isStreamEndpoint bool) {
	log.Printf("INFO: Serving encrypted file: %s (isStreamEndpoint: %v)", fullPath, isStreamEndpoint)

	// 1. 【核心】使用新的 ReaderService 获取解密器
	// ReaderService 内部会自动处理文件重建、工厂创建等所有复杂逻辑
	ctx := r.Context()
	cfg := config.FromContext(ctx)
	p, err := plugins.FindDecryptingPlugin(fullPath)
	if err != nil {
		log.Printf("ERROR: Failed to find suitable decrypting plugin for %s: %v", fullPath, err)
		http.Error(w, "Could not find suitable decrypting plugin: "+err.Error(), http.StatusInternalServerError)
		return
	}
	p.Intialize(ctx)
	namer := p.GetChunkNamer()
	decryptReader, index, originalSize, err := s.readerService.GetDecryptReader(*cfg, fullPath, cfg.Password, namer)
	if err != nil {
		log.Printf("ERROR: Failed to create decrypt reader for %s: %v", fullPath, err)
		http.Error(w, "Could not initialize decryption: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer decryptReader.Close()

	// 【关键修复】使用新的扩展接口进行类型断言
	// 现在，只有真正可寻址的 reader 才能被断言为 SeekableDecryptReader
	seeker, isSeekable := decryptReader.(reader.SeekableDecryptReader)

	originalFilename := index.GetOriginalFilename()
	w.Header().Set("Content-Type", utils.GetContentType(filepath.Ext(originalFilename)))
	// 【关键修复】统一使用 inline，让浏览器决定行为
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", originalFilename))

	// 3. 【分支处理】根据是否可寻址采取不同策略
	if isSeekable {
		// seeker 已经是 SeekableDecryptReader 类型，可以直接使用
		s.handleSeekableContent(w, r, seeker, originalSize, originalFilename)
		log.Printf("INFO: Successfully handleSeekableContent encrypted file: %s", fullPath)
	} else {
		s.handleSequentialContent(w, decryptReader, originalSize)
		log.Printf("INFO: Successfully handleSequentialContent encrypted file: %s", fullPath)
	}

}

// handleSeekableContent 处理支持 Range 请求的可寻址内容
func (s *Server) handleSeekableContent(w http.ResponseWriter, r *http.Request, seeker io.ReadSeekCloser, originalSize int64, originalFilename string) {
	// 【复用】这部分逻辑直接从您旧的 serveSeekableContainer 中移植，非常完善
	rangeHeader := r.Header.Get("Range")
	var start, end int64
	statusCode := http.StatusOK

	if rangeHeader != "" {
		re := regexp.MustCompile(`bytes=(\d+)-(\d*)`)
		matches := re.FindStringSubmatch(rangeHeader)
		if len(matches) == 3 {
			start, _ = strconv.ParseInt(matches[1], 10, 64)
			if matches[2] != "" {
				end, _ = strconv.ParseInt(matches[2], 10, 64)
			} else {
				end = originalSize - 1
			}
			if start >= originalSize || end >= originalSize || start > end {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", originalSize))
				http.Error(w, "Requested Range Not Satisfiable", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			statusCode = http.StatusPartialContent
		}
	} else {
		start = 0
		end = originalSize - 1
	}

	_, err := seeker.Seek(start, io.SeekStart)
	if err != nil {
		log.Printf("ERROR: [handleSeekableContent] Failed to seek to position %d: %v", start, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Accept-Ranges", "bytes")
	contentLength := end - start + 1
	w.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))

	if statusCode == http.StatusPartialContent {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, originalSize))
	}
	w.WriteHeader(statusCode)

	readerToCopy := io.LimitReader(seeker, contentLength)
	if _, err := io.Copy(w, readerToCopy); err != nil {
		log.Printf("WARN: Stream to client was interrupted or failed: %v", err)
	}
}

// handleSequentialContent 处理不支持 Range 请求的顺序内容
func (s *Server) handleSequentialContent(w http.ResponseWriter, sequentialReader io.ReadCloser, originalSize int64) {
	// 对于顺序流，不支持 Range 请求，直接返回整个文件
	w.Header().Set("Content-Length", fmt.Sprintf("%d", originalSize))
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, sequentialReader); err != nil {
		log.Printf("WARN: Stream to client was interrupted or failed: %v", err)
	}
}
