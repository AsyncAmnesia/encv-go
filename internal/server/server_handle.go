package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/provider"
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

	// 使用通用工具函数进行安全解析
	cleanedFilePath, err := utils.SafeURLToAbsPath(s.servingDir, filePath)
	if err != nil {
		// 根据错误类型返回不同的 HTTP 状态码
		if strings.Contains(err.Error(), "forbidden") {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	// 【核心】调用我们新的、统一的加密文件处理函数
	s.serveEncryptedFile(w, r, cleanedFilePath)
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

func (s *Server) serveEncryptedFile(w http.ResponseWriter, r *http.Request, fullPath string) {
	ctx := r.Context()
	factory, err := reader.NewDecryptReaderFactory(fullPath, s.cfg.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	decryptReader, err := factory.NewDecryptReader()
	if err != nil {
		factory.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	prov, err := provider.NewLocalFileProvider(ctx, factory, decryptReader)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer prov.Close()

	// 【核心】使用统一的内容处理器来服务文件
	s.contentHandler.ServeFile(w, r, prov)
}
