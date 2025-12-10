package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/reader"
)

// HandleSeekableContent 处理支持 Range 请求的可寻址内容
func HandleSeekableContent(w http.ResponseWriter, r *http.Request, seeker reader.SeekableDecryptReader, originalSize int64, originalFilename string) {
	// 【复用】这部分逻辑直接从 server 中移植
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
		log.Printf("ERROR: [HandleSeekableContent] Failed to seek to position %d: %v", start, err)
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

// handleSeekableContent 处理支持 Range 请求的可寻址内容 (使用 SeekTo)
func HandleSeekableContentProxy(w http.ResponseWriter, r *http.Request, seeker interface{ SeekTo(offset int64) error }, originalSize int64, originalFilename string) {
	// 和 HandleSeekableContent 的版本几乎一样，只是调用 seeker.SeekTo(start)) ...
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

// HandleSequentialContent 处理不支持 Range 请求的顺序内容
func HandleSequentialContent(w http.ResponseWriter, sequentialReader reader.DecryptReader, originalSize int64) {
	w.Header().Set("Content-Length", fmt.Sprintf("%d", originalSize))
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, sequentialReader); err != nil {
		log.Printf("WARN: Stream to client was interrupted or failed: %v", err)
	}
}

func HandleSequentialContentProxy(w http.ResponseWriter, sequentialReader io.Reader, originalSize int64, originalFilename string) {
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
