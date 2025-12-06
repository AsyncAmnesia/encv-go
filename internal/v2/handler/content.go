package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"

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

// HandleSequentialContent 处理不支持 Range 请求的顺序内容
func HandleSequentialContent(w http.ResponseWriter, sequentialReader reader.DecryptReader, originalSize int64) {
	w.Header().Set("Content-Length", fmt.Sprintf("%d", originalSize))
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, sequentialReader); err != nil {
		log.Printf("WARN: Stream to client was interrupted or failed: %v", err)
	}
}
