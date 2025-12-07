package proxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/openlist"
	"github.com/Soltus/encv-go/internal/v2/reader"
)

// serveEncryptedContainer 是一个全新的、统一的处理函数，用于处理所有远程加密容器
// 【核心】此函数的逻辑完全移植自本地 server.go，只修改了工厂创建部分
func (p *Proxy) serveEncryptedContainer(w http.ResponseWriter, r *http.Request, containerURL string, headers map[string]string, originalPath string) {
	log.Printf("INFO: [Proxy] Serving remote container: %s", containerURL)

	// 1. 【关键修改】直接创建远程工厂
	urlResolver := openlist.NewOpenListURLResolver(p.cfg, originalPath)
	factory, err := reader.NewRemoteDecryptReaderFactory(containerURL, p.cfg.Password, headers, urlResolver)
	if err != nil {
		log.Printf("ERROR: [Proxy] Failed to create remote decrypt reader factory: %v", err)
		http.Error(w, "Could not initialize decryption: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer factory.Close()

	// 2. 使用工厂创建解密器
	decryptReader, err := factory.NewDecryptReader(*p.cfg)
	if err != nil {
		log.Printf("ERROR: [Proxy] Failed to create decrypt reader: %v", err)
		http.Error(w, "Could not initialize decryption: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer decryptReader.Close()

	index := factory.GetIndex()
	originalSize := factory.GetOriginalSize()
	originalFilename := index.GetOriginalFilename()
	// 3. 【关键】使用与本地服务器完全相同的类型断言
	// 先尝试断言为支持快速跳转的 SeekTo 接口 (用于远程视频)
	type seeker interface{ SeekTo(offset int64) error }
	if s, ok := decryptReader.(seeker); ok {
		// 这是视频等可寻址流
		p.handleSeekableContent(w, r, s, originalSize, originalFilename)
		log.Printf("INFO: [Proxy] Successfully handleSeekableContent remote container: %s", containerURL)
	} else {
		// 这是图片等顺序流
		originalSize := factory.GetOriginalSize()
		p.handleSequentialContent(w, decryptReader, originalSize, originalFilename)
		log.Printf("INFO: [Proxy] Successfully handleSequentialContent remote container: %s", containerURL)
	}

}

// 【新增】从 server.go 复制过来的辅助函数
// handleSeekableContent 处理支持 Range 请求的可寻址内容 (使用 SeekTo)
func (p *Proxy) handleSeekableContent(w http.ResponseWriter, r *http.Request, seeker interface{ SeekTo(offset int64) error }, originalSize int64, originalFilename string) {
	// ... (这部分逻辑与您之前的版本几乎一样，只是调用 seeker.SeekTo(start)) ...
	// 为了完整，我把代码贴在这里
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

	if err := seeker.SeekTo(start); err != nil {
		if err == io.EOF {
			log.Printf("WARN: [Proxy] Client requested range starting at %d, which is beyond all data fragments.", start)
			w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", originalSize))
			http.Error(w, "Requested Range Not Satisfiable", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		log.Printf("ERROR: [Proxy] Fast seek failed: %v", err)
		http.Error(w, "Failed to seek to requested position", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", utils.GetContentType(filepath.Ext(originalFilename)))
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", originalFilename))
	w.Header().Set("Accept-Ranges", "bytes")
	contentLength := end - start + 1
	w.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))

	if statusCode == http.StatusPartialContent {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, originalSize))
	}
	w.WriteHeader(statusCode)

	readerToCopy := io.LimitReader(seeker.(io.Reader), contentLength) // seeker 也实现了 io.Reader
	if _, err := io.Copy(w, readerToCopy); err != nil {
		log.Printf("WARN: [Proxy] Stream to client was interrupted or failed: %v", err)
	}
}

// 【新增】从 server.go 复制过来的辅助函数
// handleSequentialContent 处理不支持 Range 请求的顺序内容
func (p *Proxy) handleSequentialContent(w http.ResponseWriter, sequentialReader io.Reader, originalSize int64, originalFilename string) {
	// 【关键】这部分逻辑与 server.go 中的完全一致，确保图片能正常显示
	w.Header().Set("Content-Length", fmt.Sprintf("%d", originalSize))
	w.Header().Set("Content-Type", utils.GetContentType(filepath.Ext(originalFilename)))
	w.WriteHeader(http.StatusOK)

	// if _, err := io.Copy(w, sequentialReader); err != nil {
	// 	log.Printf("WARN: [Proxy] Stream to client was interrupted or failed: %v", err)
	// }

	// 【关键诊断】手动进行数据传输，并校验完整性
	buf := make([]byte, 32*1024) // 32KB 缓冲区
	var totalWritten int64

	for {
		// 从解密后的流中读取数据
		n, readErr := sequentialReader.Read(buf)
		if n > 0 {
			written, writeErr := w.Write(buf[:n])
			totalWritten += int64(written)
			if writeErr != nil {
				log.Printf("ERROR: [Proxy] Failed to write to client response: %v", writeErr)
				return
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				// 正常到达流末尾
				break
			}
			// 【关键诊断】捕获连接被意外关闭的错误
			if readErr == io.ErrUnexpectedEOF {
				log.Printf("CRITICAL: [Proxy] UPSTREAM CONNECTION CLOSED PREMATURELY for %s. This is likely a server-side read timeout on large Range requests.", originalFilename)
				break // 退出循环，让完整性检查报告最终结果
			}
			// 读取时发生其他错误
			log.Printf("ERROR: [Proxy] Failed to read from decrypted stream: %v", readErr)
			return
		}
	}

	// 【核心诊断】检查数据完整性
	if totalWritten != originalSize {
		// 如果这个日志出现：上游数据被截断
		log.Printf("CRITICAL: [Proxy] DATA INTEGRITY CHECK FAILED for %s. Expected: %d bytes, Actually received: %d bytes.", originalFilename, originalSize, totalWritten)
		// 此时，浏览器已经收到了不完整的数据。这个日志是给开发者看的最终证据。
	} else {
		log.Printf("INFO: [Proxy] Data integrity check passed for %s. Transferred %d bytes.", originalFilename, totalWritten)
	}

}

// serveEncryptedContainerFromStream 通过一个临时服务器将流适配给 RemoteDecryptReaderFactory
func (p *Proxy) serveEncryptedContainerFromStream(w http.ResponseWriter, r *http.Request, encryptedStream io.ReadCloser, identifier string) {
	log.Printf("INFO: [Proxy] Serving container from stream for: %s", identifier)

	// 1. 【核心技巧】创建一个临时的、内存中的 HTTP 服务器
	// 这个服务器只有一个作用：将 encryptedStream 流式传输出去
	tempServer := httptest.NewServer(http.HandlerFunc(func(tempW http.ResponseWriter, tempR *http.Request) {
		// 将上游的流直接写入到临时服务器的响应中
		// 注意：这里我们不需要处理 Range 请求，因为 RemoteEncryptedContainerReader 会自己处理
		_, err := io.Copy(tempW, encryptedStream)
		if err != nil {
			log.Printf("WARN: [Proxy] Temporary server stream copy failed: %v", err)
		}
	}))
	defer tempServer.Close()
	defer encryptedStream.Close() // 确保原始流最终被关闭

	// 2. 获取临时服务器的 URL，这就是我们需要的 "containerURL"
	containerURL := tempServer.URL

	log.Printf("DEBUG: [Proxy] Created temporary server for stream at: %s", containerURL)

	// 3. 【关键】复用您现有的、完善的 serveEncryptedContainer 函数
	// 我们传递临时服务器的 URL、空 headers 和原始的 durl 作为路径标识符
	// serveEncryptedContainer 内部会调用 NewRemoteDecryptReaderFactory，一切都按部就班
	p.serveEncryptedContainer(w, r, containerURL, nil, identifier)
}
