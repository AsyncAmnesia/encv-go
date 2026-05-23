package service

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/container/detector"
	"github.com/Soltus/encv-go/internal/v2/handler"
	"github.com/Soltus/encv-go/internal/v2/namer"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	"github.com/Soltus/encv-go/internal/v2/provider"
	"github.com/Soltus/encv-go/internal/v2/service"
)

type ForbiddenError struct{ Err error }

func (e *ForbiddenError) Error() string { return e.Err.Error() }

type NotFoundError struct{ Err error }

func (e *NotFoundError) Error() string { return e.Err.Error() }

type BadRequestError struct{ Err error }

func (e *BadRequestError) Error() string { return e.Err.Error() }

type PermissionError struct{ Err error }

func (e *PermissionError) Error() string { return e.Err.Error() }

type UnsupportedMediaTypeError struct{ Err error }

func (e *UnsupportedMediaTypeError) Error() string { return e.Err.Error() }

type FileInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	IsDirectory bool   `json:"isDirectory"`
	IsEncrypted bool   `json:"isEncrypted"`
	Size        int64  `json:"size"`
	Modified    string `json:"modified"`
}

type FileContentResult struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

type MobileService struct {
	servingDir     string
	taskManager    *TaskManager
	wsHub          *WSHub
	fileIndex      *fileIndex
	cfg            *config.Config
	readerService  *service.ReaderService
	contentHandler *handler.ContentHandler
	chunkNamers    []namer.ChunkNamer
}

func NewMobileService(servingDir string, cfg *config.Config) *MobileService {
	wsHub := NewWSHub()
	return &MobileService{
		servingDir:  servingDir,
		taskManager: NewTaskManager(servingDir, cfg, wsHub),
		wsHub:       wsHub,
		cfg:         cfg,
	}
}

func (s *MobileService) ListFiles(queryPath string) ([]FileInfo, error) {
	if queryPath == "" {
		queryPath = "/"
	}

	absPath, err := utils.SafeURLToAbsPath(s.servingDir, queryPath)
	if err != nil {
		slog.Error("SafeURLToAbsPath failed", "path", queryPath, "error", err)
		return nil, &ForbiddenError{Err: err}
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &NotFoundError{Err: err}
		}
		if isPermissionError(err) {
			slog.Warn("ReadDir permission denied", "path", absPath, "error", err)
			return nil, &PermissionError{Err: err}
		}
		slog.Error("ReadDir failed", "path", absPath, "error", err)
		return nil, err
	}

	var files []FileInfo
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		filePath := queryPath + "/" + entry.Name()
		if queryPath == "/" {
			filePath = "/" + entry.Name()
		}

		info, err := entry.Info()
		if err != nil {
			slog.Warn("Failed to get file info, using fallback", "name", entry.Name(), "error", err)
			files = append(files, FileInfo{
				Name:        entry.Name(),
				Path:        filePath,
				IsDirectory: entry.IsDir(),
				Size:        0,
				Modified:    "",
			})
			continue
		}

		isEncrypted := false
		if !entry.IsDir() {
			entryAbsPath := filepath.Join(absPath, entry.Name())
			if _, detectErr := detector.DetectContainer(entryAbsPath); detectErr == nil {
				isEncrypted = true
			}
		}

		files = append(files, FileInfo{
			Name:        entry.Name(),
			Path:        filePath,
			IsDirectory: entry.IsDir(),
			IsEncrypted: isEncrypted,
			Size:        info.Size(),
			Modified:    info.ModTime().Format(time.RFC3339),
		})
	}

	slog.Info("ListFiles result", "path", queryPath, "count", len(files))
	return files, nil
}

func (s *MobileService) DeleteFile(queryPath string) error {
	if queryPath == "" {
		return &BadRequestError{Err: errors.New("'path' query parameter is required")}
	}

	absPath, err := utils.SafeURLToAbsPath(s.servingDir, queryPath)
	if err != nil {
		slog.Error("SafeURLToAbsPath failed", "path", queryPath, "error", err)
		return &ForbiddenError{Err: err}
	}

	slog.Warn("DeleteFile", "path", queryPath)
	err = os.Remove(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &NotFoundError{Err: err}
		}
		slog.Error("Remove failed", "path", absPath, "error", err)
		return err
	}

	return nil
}

func (s *MobileService) ReadFileContent(queryPath string) (*FileContentResult, error) {
	if queryPath == "" {
		return nil, &BadRequestError{Err: errors.New("'path' query parameter is required")}
	}

	absPath, err := utils.SafeURLToAbsPath(s.servingDir, queryPath)
	if err != nil {
		return nil, &ForbiddenError{Err: err}
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &NotFoundError{Err: err}
		}
		return nil, err
	}

	slog.Info("ReadFileContent", "path", queryPath, "size", info.Size())

	if info.IsDir() {
		return nil, &BadRequestError{Err: errors.New("path is a directory")}
	}

	maxSize := int64(2 << 20)
	if info.Size() > maxSize {
		return nil, &BadRequestError{Err: errors.New("file too large")}
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		slog.Error("ReadFile failed", "path", absPath, "error", err)
		return nil, err
	}

	content := string(data)
	encoding := "utf-8"
	if !isValidUTF8(data) {
		encoding = "binary"
	}

	return &FileContentResult{
		Name:     filepath.Base(absPath),
		Path:     queryPath,
		Size:     info.Size(),
		Content:  content,
		Encoding: encoding,
	}, nil
}

func (s *MobileService) TestWebDAV(url, username, password string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return &BadRequestError{Err: err}
	}

	if username != "" || password != "" {
		httpReq.SetBasicAuth(username, password)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

func (s *MobileService) GetTaskManager() *TaskManager {
	return s.taskManager
}

func (s *MobileService) GetWSHub() *WSHub {
	return s.wsHub
}

func (s *MobileService) SetServingDir(dir string) {
	s.servingDir = dir
	s.taskManager.servingDir = dir
}

func (s *MobileService) SetEncryptedFileDeps(readerService *service.ReaderService, contentHandler *handler.ContentHandler, chunkNamers []namer.ChunkNamer) {
	s.readerService = readerService
	s.contentHandler = contentHandler
	s.chunkNamers = chunkNamers
}

func (s *MobileService) GetServingDir() string {
	return s.servingDir
}

func (s *MobileService) CheckStoragePermission() bool {
	if s.servingDir == "" {
		slog.Warn("CheckStoragePermission: servingDir is empty")
		return false
	}
	f, err := os.Open(s.servingDir)
	if err != nil {
		slog.Warn("CheckStoragePermission: cannot open servingDir", "dir", s.servingDir, "error", err)
		return false
	}
	f.Close()
	slog.Info("CheckStoragePermission: OK", "dir", s.servingDir)
	return true
}

func isPermissionError(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		if errno, ok := pathErr.Err.(syscall.Errno); ok {
			return errno == syscall.EACCES
		}
	}
	return false
}

func isValidUTF8(data []byte) bool {
	for i := 0; i < len(data); {
		if data[i] < 0x80 {
			i++
			continue
		}
		_, size := decodeUTF8Rune(data[i:])
		if size == 0 {
			return false
		}
		i += size
	}
	return true
}

func decodeUTF8Rune(data []byte) (rune, int) {
	if len(data) == 0 {
		return 0, 0
	}
	b := data[0]
	if b < 0x80 {
		return rune(b), 1
	}
	var n uint32
	var size int
	switch {
	case b&0xe0 == 0xc0:
		n = uint32(b & 0x1f)
		size = 2
	case b&0xf0 == 0xe0:
		n = uint32(b & 0x0f)
		size = 3
	case b&0xf8 == 0xf0:
		n = uint32(b & 0x07)
		size = 4
	default:
		return 0, 0
	}
	if len(data) < size {
		return 0, 0
	}
	for i := 1; i < size; i++ {
		if data[i]&0xc0 != 0x80 {
			return 0, 0
		}
		n = n<<6 | uint32(data[i]&0x3f)
	}
	return rune(n), size
}

type IndexStats struct {
	TotalFiles  int   `json:"totalFiles"`
	TotalDirs   int   `json:"totalDirs"`
	TotalSize   int64 `json:"totalSize"`
	IndexedAt   string `json:"indexedAt"`
	IsIndexing  bool  `json:"isIndexing"`
	LastBuildMs int64 `json:"lastBuildMs"`
}

type indexEntry struct {
	Path        string
	Name        string
	IsDirectory bool
	Size        int64
	Modified    string
}

type fileIndex struct {
	mu       sync.RWMutex
	entries  []indexEntry
	stats    IndexStats
	building bool
}

func (s *MobileService) SearchFiles(queryPath string, keyword string, recursive bool) ([]FileInfo, error) {
	if queryPath == "" {
		queryPath = "/"
	}
	if keyword == "" {
		return s.ListFiles(queryPath)
	}

	absPath, err := utils.SafeURLToAbsPath(s.servingDir, queryPath)
	if err != nil {
		return nil, &ForbiddenError{Err: err}
	}

	keyword = strings.ToLower(keyword)
	var results []FileInfo
	const maxResults = 200

	if recursive && s.fileIndex != nil {
		s.fileIndex.mu.RLock()
		hasIndex := len(s.fileIndex.entries) > 0
		s.fileIndex.mu.RUnlock()

		if hasIndex {
			s.fileIndex.mu.RLock()
			for _, entry := range s.fileIndex.entries {
				if len(results) >= maxResults {
					break
				}
				if !strings.Contains(strings.ToLower(entry.Name), keyword) {
					continue
				}
				if !strings.HasPrefix(entry.Path, queryPath) && queryPath != "/" {
					continue
				}
				if queryPath == "/" && !strings.HasPrefix(entry.Path, "/") {
					continue
				}
				results = append(results, FileInfo{
					Name:        entry.Name,
					Path:        entry.Path,
					IsDirectory: entry.IsDirectory,
					Size:        entry.Size,
					Modified:    entry.Modified,
				})
			}
			s.fileIndex.mu.RUnlock()
			slog.Info("SearchFiles using index", "path", queryPath, "keyword", keyword, "count", len(results))
			return results, nil
		}
	}

	if recursive {
		err = filepath.WalkDir(absPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if len(results) >= maxResults {
				return fs.SkipAll
			}
			if strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if strings.Contains(strings.ToLower(d.Name()), keyword) {
				relPath, relErr := filepath.Rel(absPath, path)
				if relErr != nil {
					relPath = d.Name()
				}
				urlPath := queryPath
				if queryPath == "/" {
					urlPath = ""
				}
				urlPath += "/" + filepath.ToSlash(relPath)

				info, infoErr := d.Info()
				size := int64(0)
				modified := ""
				if infoErr == nil {
					size = info.Size()
					modified = info.ModTime().Format(time.RFC3339)
				}

				results = append(results, FileInfo{
					Name:        d.Name(),
					Path:        urlPath,
					IsDirectory: d.IsDir(),
					Size:        size,
					Modified:    modified,
				})
			}
			return nil
		})
		if err != nil {
			slog.Error("SearchFiles WalkDir failed", "path", absPath, "error", err)
			return nil, err
		}
	} else {
		entries, err := os.ReadDir(absPath)
		if err != nil {
			if isPermissionError(err) {
				return nil, &PermissionError{Err: err}
			}
			return nil, err
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if strings.Contains(strings.ToLower(entry.Name()), keyword) {
				filePath := queryPath + "/" + entry.Name()
				if queryPath == "/" {
					filePath = "/" + entry.Name()
				}
				info, infoErr := entry.Info()
				size := int64(0)
				modified := ""
				if infoErr == nil {
					size = info.Size()
					modified = info.ModTime().Format(time.RFC3339)
				}
				results = append(results, FileInfo{
					Name:        entry.Name(),
					Path:        filePath,
					IsDirectory: entry.IsDir(),
					Size:        size,
					Modified:    modified,
				})
			}
		}
	}

	slog.Info("SearchFiles result", "path", queryPath, "keyword", keyword, "recursive", recursive, "count", len(results))
	return results, nil
}

func (s *MobileService) GetIndexStats() *IndexStats {
	if s.fileIndex == nil {
		s.fileIndex = &fileIndex{}
	}
	s.fileIndex.mu.RLock()
	defer s.fileIndex.mu.RUnlock()
	stats := s.fileIndex.stats
	return &stats
}

func (s *MobileService) RebuildIndex() {
	if s.fileIndex == nil {
		s.fileIndex = &fileIndex{}
	}
	s.fileIndex.mu.Lock()
	if s.fileIndex.building {
		s.fileIndex.mu.Unlock()
		return
	}
	s.fileIndex.building = true
	s.fileIndex.mu.Unlock()

	go func() {
		start := time.Now()
		var entries []indexEntry
		var totalSize int64
		var fileCount, dirCount int

		filepath.WalkDir(s.servingDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}

			relPath, relErr := filepath.Rel(s.servingDir, path)
			if relErr != nil {
				return nil
			}
			urlPath := "/" + filepath.ToSlash(relPath)

			info, infoErr := d.Info()
			size := int64(0)
			modified := ""
			if infoErr == nil {
				size = info.Size()
				modified = info.ModTime().Format(time.RFC3339)
			}

			if d.IsDir() {
				dirCount++
			} else {
				fileCount++
				totalSize += size
			}

			entries = append(entries, indexEntry{
				Path:        urlPath,
				Name:        d.Name(),
				IsDirectory: d.IsDir(),
				Size:        size,
				Modified:    modified,
			})
			return nil
		})

		elapsed := time.Since(start).Milliseconds()

		s.fileIndex.mu.Lock()
		s.fileIndex.entries = entries
		s.fileIndex.stats = IndexStats{
			TotalFiles:  fileCount,
			TotalDirs:   dirCount,
			TotalSize:   totalSize,
			IndexedAt:   time.Now().Format(time.RFC3339),
			IsIndexing:  false,
			LastBuildMs: elapsed,
		}
		s.fileIndex.building = false
		s.fileIndex.mu.Unlock()

		slog.Info("RebuildIndex completed", "files", fileCount, "dirs", dirCount, "ms", elapsed)
	}()
}

func (s *MobileService) ClearIndex() {
	if s.fileIndex == nil {
		return
	}
	s.fileIndex.mu.Lock()
	defer s.fileIndex.mu.Unlock()
	s.fileIndex.entries = nil
	s.fileIndex.stats = IndexStats{}
	slog.Info("ClearIndex completed")
}

var mediaExtensions = map[string]bool{
	"mp4": true, "mkv": true, "avi": true, "mov": true,
	"wmv": true, "flv": true, "webm": true, "m4v": true,
	"ts": true, "mpg": true, "mpeg": true, "3gp": true,
	"mp3": true, "flac": true, "wav": true, "aac": true,
	"ogg": true, "wma": true, "m4a": true, "opus": true,
	"encv": true,
}

type chunkNamerAdapter struct {
	namers []namer.ChunkNamer
}

func (a *chunkNamerAdapter) GenerateMainChunkName(baseName string) string {
	if len(a.namers) > 0 {
		return a.namers[0].GenerateMainChunkName(baseName)
	}
	return baseName
}

func (a *chunkNamerAdapter) ParseFirstChunkName(firstChunkPath string) (string, error) {
	for _, n := range a.namers {
		base, err := n.ParseFirstChunkName(firstChunkPath)
		if err == nil {
			return base, nil
		}
	}
	return "", fmt.Errorf("no suitable namer found for path: %s", firstChunkPath)
}

func (a *chunkNamerAdapter) GenerateDataChunkName(baseName string, index int) string {
	if len(a.namers) > 0 {
		return a.namers[0].GenerateDataChunkName(baseName, index)
	}
	return fmt.Sprintf("%s.%d", baseName, index)
}

func (a *chunkNamerAdapter) GetFirstDataChunkIndex() int {
	if len(a.namers) > 0 {
		return a.namers[0].GetFirstDataChunkIndex()
	}
	return 1
}

func (a *chunkNamerAdapter) IsDataChunk(filename string) bool {
	for _, n := range a.namers {
		if n.IsDataChunk(filename) {
			return true
		}
	}
	return false
}

func (s *MobileService) FileExists(queryPath string) (bool, error) {
	absPath, err := utils.SafeURLToAbsPath(s.servingDir, queryPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, &ForbiddenError{Err: err}
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		if isPermissionError(err) {
			return false, &ForbiddenError{Err: err}
		}
		return false, err
	}
	return info != nil, nil
}

func (s *MobileService) CheckEncryptOutputExists(sourcePath, targetDir string) (bool, string, error) {
	sourceAbs, err := utils.SafeURLToAbsPath(s.servingDir, sourcePath)
	if err != nil {
		return false, "", &ForbiddenError{Err: err}
	}

	outputName, err := plugins.PredictEncryptOutputName(sourceAbs, s.cfg)
	if err != nil {
		return false, "", err
	}

	outputDir := targetDir
	if outputDir == "" {
		outputDir = filepath.Dir(sourcePath)
		if outputDir == "" {
			outputDir = "/"
		}
	}

	outputPath := outputDir
	if outputPath == "/" {
		outputPath = "/" + outputName
	} else {
		outputPath = strings.TrimRight(outputPath, "/") + "/" + outputName
	}

	outputAbs, err := utils.SafeURLToAbsPath(s.servingDir, outputPath)
	if err != nil {
		return false, outputPath, nil
	}

	_, err = os.Stat(outputAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return false, outputPath, nil
		}
		return false, outputPath, err
	}
	return true, outputPath, nil
}

func (s *MobileService) StreamExternalFile(w http.ResponseWriter, r *http.Request, filePath string) error {
	if filePath == "" {
		return &BadRequestError{Err: errors.New("'path' query parameter is required")}
	}

	absPath := filepath.Clean(filePath)
	if !filepath.IsAbs(absPath) {
		return &BadRequestError{Err: errors.New("path must be absolute")}
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &NotFoundError{Err: err}
		}
		return &ForbiddenError{Err: err}
	}

	if info.IsDir() {
		return &BadRequestError{Err: errors.New("path is a directory")}
	}

	if _, detectErr := detector.DetectContainer(absPath); detectErr == nil {
		slog.Info("StreamExternalFile: detected ENCV container, serving decrypted", "path", absPath)
		if s.readerService == nil || s.contentHandler == nil {
			slog.Error("StreamExternalFile: encrypted file detected but dependencies not initialized")
			return &BadRequestError{Err: errors.New("encrypted file service not available")}
		}
		s.serveEncryptedExternalFile(w, r, absPath)
		return nil
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	if len(ext) > 0 {
		ext = ext[1:]
	}
	if !mediaExtensions[ext] {
		return &UnsupportedMediaTypeError{Err: errors.New("file is not a supported media file")}
	}

	slog.Info("StreamExternalFile", "path", absPath, "size", info.Size())
	http.ServeFile(w, r, absPath)
	return nil
}

func (s *MobileService) serveEncryptedExternalFile(w http.ResponseWriter, r *http.Request, fullPath string) {
	ctx := r.Context()
	adapterNamer := &chunkNamerAdapter{namers: s.chunkNamers}

	factory, decryptReader, _, _, err := s.readerService.GetDecryptReader(
		*s.cfg,
		fullPath,
		s.cfg.Password,
		adapterNamer,
	)
	if err != nil {
		slog.Error("GetDecryptReader failed", "path", fullPath, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer decryptReader.Close()

	prov, err := provider.NewLocalFileProvider(ctx, factory, decryptReader)
	if err != nil {
		slog.Error("NewLocalFileProvider failed", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer prov.Close()

	s.contentHandler.ServeFile(w, r, prov)
}
