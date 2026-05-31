package alistencrypt

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

)

type seekableDecryptReader struct {
	*DecryptReader
	closeFunc func() error
}

func (s *seekableDecryptReader) Close() error {
	if s.closeFunc != nil {
		return s.closeFunc()
	}
	return nil
}

func (p *AlistEncryptPlugin) Stream(path string, password string) (io.ReadCloser, int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to open file: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, "", fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := info.Size()

	var plainSize int64
	if fileSize >= 32 {
		peekBuf := make([]byte, 32)
		n, _ := io.ReadFull(f, peekBuf)
		if n == 32 && bytes.Equal(peekBuf[:6], []byte(AECTR2Magic)) {
			header, headerErr := DetectContentHeader(peekBuf)
			if headerErr == nil && header != nil {
				plainSize = header.PlainSize
			} else {
				plainSize = fileSize
			}
		} else {
			plainSize = fileSize
		}
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			f.Close()
			return nil, 0, "", fmt.Errorf("failed to seek back: %w", seekErr)
		}
	} else {
		plainSize = fileSize
	}

	dr, err := NewDecryptReader(f, password, fileSize)
	if err != nil {
		f.Close()
		return nil, 0, "", fmt.Errorf("failed to create decrypt reader: %w", err)
	}

	contentType := detectContentType(path)

	sr := &seekableDecryptReader{
		DecryptReader: dr,
		closeFunc:     f.Close,
	}

	return sr, plainSize, contentType, nil
}

func (p *AlistEncryptPlugin) ServeStream(w http.ResponseWriter, r *http.Request, path string, password string) error {
	rc, size, contentType, err := p.Stream(path, password)
	if err != nil {
		return err
	}
	defer rc.Close()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Accept-Ranges", "bytes")

	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		_, err = io.Copy(w, rc)
		return err
	}

	var start, end int64
	_, err = fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid range format")
		return nil
	}
	if start < 0 || end < 0 {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		fmt.Fprintf(w, "range must be non-negative")
		return nil
	}
	if start > size {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		fmt.Fprintf(w, "range start exceeds content length")
		return nil
	}
	if end >= size {
		end = size - 1
	}

	if seeker, ok := rc.(io.Seeker); ok {
		if _, err := seeker.Seek(start, io.SeekStart); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("stream does not support seeking")
	}

	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	w.WriteHeader(http.StatusPartialContent)

	remaining := end - start + 1
	buf := make([]byte, 32*1024)
	for remaining > 0 {
		readN := len(buf)
		if int64(readN) > remaining {
			readN = int(remaining)
		}
		n, err := rc.Read(buf[:readN])
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			remaining -= int64(n)
		}
		if err != nil {
			break
		}
	}

	return nil
}

func detectContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	mimeMap := map[string]string{
		".mp4":  "video/mp4",
		".mkv":  "video/x-matroska",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".wmv":  "video/x-ms-wmv",
		".flv":  "video/x-flv",
		".webm": "video/webm",
		".mp3":  "audio/mpeg",
		".flac": "audio/flac",
		".wav":  "audio/wav",
		".ogg":  "audio/ogg",
		".m4a":  "audio/mp4",
		".aac":  "audio/aac",
		".pdf":  "application/pdf",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".bmp":  "image/bmp",
		".txt":  "text/plain",
		".html": "text/html",
		".xml":  "application/xml",
		".json": "application/json",
	}
	if ct, ok := mimeMap[ext]; ok {
		return ct
	}
	return "application/octet-stream"
}
