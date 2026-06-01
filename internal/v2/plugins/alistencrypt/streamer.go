package alistencrypt

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// resolveShowName decodes the encrypted filename back to its original name.
// Falls back to "orig_<basename>" if the decode fails, matching ConvertShowName's contract.
func (p *AlistEncryptPlugin) resolveShowName(path, password string) string {
	encType := p.settings.EncType
	if encType == "" {
		encType = "aesctr"
	}
	showName := ConvertShowName(filepath.Base(path), password, encType)
	return showName
}

func (p *AlistEncryptPlugin) Stream(path string, password string) (io.ReadCloser, int64, string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, "", "", fmt.Errorf("failed to open file: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, "", "", fmt.Errorf("failed to stat file: %w", err)
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
			return nil, 0, "", "", fmt.Errorf("failed to seek back: %w", seekErr)
		}
	} else {
		plainSize = fileSize
	}

	dr, err := NewDecryptReader(f, password, fileSize)
	if err != nil {
		f.Close()
		return nil, 0, "", "", fmt.Errorf("failed to create decrypt reader: %w", err)
	}

	showName := p.resolveShowName(path, password)
	contentType := detectContentTypeByName(showName, path)

	sr := &seekableDecryptReader{
		DecryptReader: dr,
		closeFunc:     f.Close,
	}

	return sr, plainSize, contentType, showName, nil
}

func (p *AlistEncryptPlugin) ServeStream(w http.ResponseWriter, r *http.Request, path string, password string) error {
	rc, size, contentType, showName, err := p.Stream(path, password)
	if err != nil {
		return err
	}
	defer rc.Close()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	if showName != "" {
		// Sanitize: HTTP filename* uses RFC 5987; ASCII fallback for compatibility.
		safe := strings.NewReplacer("\\", "_", "/", "_", "\"", "_", "\r", "", "\n", "").Replace(showName)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"; filename*=UTF-8''%s`,
			safe, url.QueryEscape(showName)))
		w.Header().Set("X-AlistEncrypt-Original-Name", showName)
	}

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
	partialLen := end - start + 1
	w.Header().Set("Content-Length", strconv.FormatInt(partialLen, 10))
	w.WriteHeader(http.StatusPartialContent)

	remaining := partialLen
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

// contentTypeByExt returns the canonical MIME type for the given file extension.
// Falls back to application/octet-stream for unknown extensions.
func contentTypeByExt(ext string) string {
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
	if ct, ok := mimeMap[strings.ToLower(ext)]; ok {
		return ct
	}
	return "application/octet-stream"
}

// detectContentTypeByName chooses the MIME type using the decoded filename when
// available (so encrypted `.bin` containers carrying, e.g., a real MP4 are
// served as video/mp4 instead of octet-stream). Falls back to the on-disk path's
// extension if decoding didn't yield a useful name.
func detectContentTypeByName(showName, onDiskPath string) string {
	if showName != "" && !strings.HasPrefix(showName, OrigPrefix) {
		if ct := contentTypeByExt(filepath.Ext(showName)); ct != "application/octet-stream" {
			return ct
		}
	}
	return contentTypeByExt(filepath.Ext(onDiskPath))
}

// detectContentType keeps the old behaviour for callers that only have the
// on-disk path (e.g., unit tests asserting the legacy code path).
func detectContentType(path string) string {
	return contentTypeByExt(filepath.Ext(path))
}
