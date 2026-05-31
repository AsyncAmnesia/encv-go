package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/utils"
	containerhandle "github.com/Soltus/encv-go/internal/v2/container/handle"
	"github.com/Soltus/encv-go/internal/v2/container/detector"
	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/filename"
	"github.com/Soltus/encv-go/internal/v2/handler"
	"github.com/Soltus/encv-go/internal/v2/namer"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	"github.com/Soltus/encv-go/internal/v2/provider"
	"github.com/Soltus/encv-go/internal/v2/service"
	"github.com/Soltus/encv-go/internal/v2/types"
)

type DirReader interface {
	ReadDir(name string) ([]fs.DirEntry, error)
}

type ContainerDetector interface {
	DetectContainer(path string) (interface{}, error)
}

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
	DisplayName string `json:"display_name,omitempty"`
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

	// --- 可选依赖注入（用于测试） ---
	dirReader        DirReader          // nil = 使用 os.ReadDir
	containerDetector ContainerDetector  // nil = 使用 detector.DetectContainer
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
		return nil, &ForbiddenError{Err: err}
	}

	entries, err := s.readDir(absPath)
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
			if s.detectContainer(entryAbsPath) {
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

func (s *MobileService) CreateDirectory(parentPath, name string) error {
	if name == "" {
		return &BadRequestError{Err: errors.New("directory name cannot be empty")}
	}
	if len(name) > 255 {
		return &BadRequestError{Err: errors.New("directory name too long (max 255 characters)")}
	}
	if strings.ContainsAny(name, "\000/") {
		return &BadRequestError{Err: errors.New("directory name contains illegal characters")}
	}
	if strings.Contains(name, "..") {
		return &ForbiddenError{Err: errors.New("directory name contains path traversal sequence")}
	}

	fullPath := filepath.Join(s.servingDir, parentPath, name)

	absServing, _ := filepath.Abs(s.servingDir)
	absFull, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absFull, absServing) {
		return &ForbiddenError{Err: errors.New("path traversal detected")}
	}

	if err := os.Mkdir(fullPath, 0755); err != nil {
		if os.IsExist(err) {
			return &BadRequestError{Err: fmt.Errorf("directory already exists: %s", name)}
		}
		slog.Error("Mkdir failed", "path", fullPath, "error", err)
		return err
	}

	slog.Info("Directory created", "path", fullPath)
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

	if s.detectContainer(absPath) {
		return nil, &BadRequestError{Err: errors.New("is_encv_container: use /api/file/info endpoint for metadata")}
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

type FileInfoResult struct {
	Name            string                 `json:"name"`
	DisplayName     string                 `json:"display_name,omitempty"`
	Path            string                 `json:"path"`
	Size            int64                  `json:"size"`
	Modified        string                 `json:"modified"`
	MimeType        string                 `json:"mime_type"`
	Category        string                 `json:"category"`
	IsDirectory     bool                   `json:"is_directory"`
	IsEncrypted     bool                   `json:"is_encrypted"`
	IsEncvContainer bool                   `json:"is_encv_container"`
	Container       map[string]interface{} `json:"container,omitempty"`
}

func (s *MobileService) GetFileInfo(queryPath string) (*FileInfoResult, error) {
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

	ext := strings.ToLower(filepath.Ext(queryPath))
	mimeType := utils.GetContentType(ext)

	category := "other"
	if strings.HasPrefix(mimeType, "image/") {
		category = "image"
	} else if strings.HasPrefix(mimeType, "video/") {
		category = "video"
	} else if strings.HasPrefix(mimeType, "audio/") {
		category = "audio"
	} else if strings.HasPrefix(mimeType, "text/") || mimeType == "application/pdf" || mimeType == "application/epub+zip" {
		category = "document"
	}

	result := &FileInfoResult{
		Name:            filepath.Base(absPath),
		Path:            queryPath,
		Size:            info.Size(),
		Modified:        info.ModTime().Format(time.RFC3339),
		MimeType:        mimeType,
		Category:        category,
		IsDirectory:     info.IsDir(),
		IsEncrypted:     false,
		IsEncvContainer: false,
	}

	isContainer := false
	if !info.IsDir() {
		if s.detectContainer(absPath) {
			isContainer = true
		}
	}

	if isContainer {
		result.IsEncvContainer = true
		result.IsEncrypted = true
		result.Category = "encrypted"

		src, srcErr := containerhandle.NewFileSource(absPath)
		if srcErr != nil {
			slog.Warn("GetFileInfo: failed to open file source", "path", queryPath, "error", srcErr)
			result.Container = map[string]interface{}{
				"error": "cannot open container file: " + srcErr.Error(),
			}
			return result, nil
		}
		defer src.Close()

		h, openErr := containerhandle.Open(src)
		if openErr != nil {
			slog.Warn("GetFileInfo: ContainerHandle.Open failed", "path", queryPath, "error", openErr)
			result.Container = map[string]interface{}{
				"error": "cannot read container metadata: " + openErr.Error(),
			}
			return result, nil
		}
		defer h.Close()

		result.Container = map[string]interface{}{
			"version":        h.Version(),
			"container_type": containerTypeToString(h.ContainerType()),
			"is_seekable":    h.IsSeekable(),
		}

		if h.Version() == 4 && h.ManifestV4() != nil {
		mf := h.ManifestV4()
		hdr := h.HeaderV4()

		containerID := mf.ContainerID
		if containerID == "" {
			containerID = "(auto)"
		}
		result.Container["container_id"] = containerID
		if cidStr, ok := result.Container["container_id"].(string); ok {
			if !utf8.ValidString(cidStr) || !isPrintableJSONString(cidStr) {
				result.Container["container_id"] = "(non-printable data)"
			}
		}
		if v, ok := result.Container["version"]; ok {
			switch val := v.(type) {
			case int:
				if val < 0 || val > 999 {
					result.Container["version"] = "?"
				}
			case float64:
				if val < 0 || val > 999 {
					result.Container["version"] = "?"
				}
			case string:
				if !utf8.ValidString(val) || !isPrintableJSONString(val) {
					result.Container["version"] = "?"
				}
			}
		}
		result.Container["original_duration"] = mf.OriginalDuration
		result.Container["segment_count"] = len(mf.Segments)
		result.Container["manifest_size"] = hdr.ManifestLength
		result.Container["header"] = map[string]interface{}{
			"flags":           hdr.Flags,
			"manifest_offset": hdr.ManifestOffset,
			"manifest_length": hdr.ManifestLength,
		}

		displayName, _ := filename.ResolveDisplayName(
			context.Background(), result.Name, mf, hdr.Flags, "", filename.FNConfig{},
		)
		if displayName != result.Name {
			result.DisplayName = displayName
		}
		result.Container["original_name"] = mf.OriginalName
		result.Container["filename_alg"] = mf.FilenameAlgorithm

		mfBytes, err := json.Marshal(mf)
		if err != nil {
			slog.Warn("GetFileInfo: failed to marshal manifest v4", "path", queryPath, "error", err)
			result.Container["manifest"] = nil
		} else if !utf8.Valid(mfBytes) {
			slog.Warn("GetFileInfo: manifest v4 produced invalid UTF-8", "path", queryPath)
			result.Container["manifest"] = "(contains invalid utf-8 data)"
		} else {
			var mfMap map[string]interface{}
			if err := json.Unmarshal(mfBytes, &mfMap); err != nil {
				slog.Warn("GetFileInfo: failed to unmarshal manifest v4", "path", queryPath, "error", err)
				result.Container["manifest"] = nil
			} else {
				delete(mfMap, "kvi")
				sanitizeManifestMap(mfMap)
				result.Container["manifest"] = mfMap
			}
		}
	} else if h.Manifest() != nil {
			result.Container["manifest"] = h.Manifest()
		}
	}

	return result, nil
}

type RenameFileRequest struct {
	Path     string `json:"path"`
	NewName  string `json:"new_name"`
	Password string `json:"password,omitempty"`
}

type RenameFileResponse struct {
	Success      bool   `json:"success"`
	DisplayName string `json:"display_name"`
	Error        string `json:"error,omitempty"`
}

func (s *MobileService) RenameFile(req *RenameFileRequest) (*RenameFileResponse, error) {
	if req.Path == "" {
		return nil, &BadRequestError{Err: errors.New("'path' is required")}
	}
	if req.NewName == "" {
		return nil, &BadRequestError{Err: errors.New("'new_name' is required")}
	}

	absPath, err := utils.SafeURLToAbsPath(s.servingDir, req.Path)
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
	if info.IsDir() {
		return nil, &BadRequestError{Err: errors.New("path is a directory, not an ENCV container")}
	}

	src, srcErr := containerhandle.NewFileSource(absPath)
	if srcErr != nil {
		return nil, &BadRequestError{Err: fmt.Errorf("cannot open container file: %w", srcErr)}
	}
	defer src.Close()

	h, openErr := containerhandle.Open(src)
	if openErr != nil {
		return nil, &BadRequestError{Err: fmt.Errorf("cannot read container metadata: %w", openErr)}
	}
	defer h.Close()

	if h.Version() != 4 || h.ManifestV4() == nil {
		return nil, &BadRequestError{Err: errors.New("only v4 containers support original_name rename")}
	}

	mf := h.ManifestV4()
	hdr := h.HeaderV4()

	isEncrypted := hdr.Flags&types.FlagFilenameEncrypted != 0

	var storedName string
	if isEncrypted {
		if req.Password == "" {
			return nil, &BadRequestError{Err: errors.New("password is required for encrypted filename")}
		}
		fnCfg := filename.FNConfig{}
		encoded, encErr := fnCfg.Encode([]byte(req.NewName))
		if encErr != nil {
			slog.Warn("RenameFile: FNConfig.Encode failed", "path", req.Path, "error", encErr)
			return nil, fmt.Errorf("failed to encode filename: %w", encErr)
		}
		storedName = encoded
	} else {
		storedName = req.NewName
	}

	mf.OriginalName = storedName
	if isEncrypted {
		mf.FilenameAlgorithm = "enc-fn:v1"
	}

	manifestJSON, serErr := mf.SerializeToJSON_v4()
	if serErr != nil {
		return nil, fmt.Errorf("failed to serialize manifest: %w", serErr)
	}

	obfuscated, obfErr := crypto.ObfuscateManifest(manifestJSON)
	if obfErr != nil {
		return nil, fmt.Errorf("failed to obfuscate manifest: %w", obfErr)
	}

	oldManifestLen := int(hdr.ManifestLength)
	newManifestLen := len(obfuscated)

	file, fileErr := os.OpenFile(absPath, os.O_RDWR, 0644)
	if fileErr != nil {
		return nil, fmt.Errorf("cannot open file for writing: %w", fileErr)
	}
	defer file.Close()

	_, writeErr := file.WriteAt(obfuscated, int64(hdr.ManifestOffset))
	if writeErr != nil {
		return nil, fmt.Errorf("failed to write manifest: %w", writeErr)
	}

	if newManifestLen < oldManifestLen {
		pad := make([]byte, oldManifestLen-newManifestLen)
		file.WriteAt(pad, int64(hdr.ManifestOffset)+int64(newManifestLen))
	}

	if newManifestLen != oldManifestLen {
		hdr.ManifestLength = uint32(newManifestLen)
		if storedName != "" && mf.FilenameAlgorithm != "" {
			hdr.Flags |= types.FlagFilenameEncrypted
		}
		if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
			slog.Warn("RenameFile: failed to seek to header for update", "error", seekErr)
		} else if wErr := types.WriteHeaderV4(file, hdr); wErr != nil {
			slog.Warn("RenameFile: failed to update header ManifestLength", "error", wErr)
		}
	}

	displayName := req.NewName
	if isEncrypted {
		displayName = req.NewName
	}

	slog.Info("RenameFile success", "path", req.Path, "newName", req.NewName,
		"encrypted", isEncrypted, "oldManifestLen", oldManifestLen, "newManifestLen", newManifestLen)

	return &RenameFileResponse{
		Success:      true,
		DisplayName: displayName,
	}, nil
}

func sanitizeManifestMap(m map[string]interface{}) {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			if !utf8.ValidString(val) || !isPrintableJSONString(val) {
				m[k] = "(non-printable data)"
			}
		case []interface{}:
			for i, item := range val {
				if sub, ok := item.(map[string]interface{}); ok {
					sanitizeManifestMap(sub)
					val[i] = sub
				} else if s, ok := item.(string); ok {
					if !utf8.ValidString(s) || !isPrintableJSONString(s) {
						val[i] = "(non-printable data)"
					}
				}
			}
		case map[string]interface{}:
			sanitizeManifestMap(val)
		}
	}
}

func isPrintableJSONString(s string) bool {
	for _, r := range s {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
		if r >= 0x7F && r <= 0x9F {
			return false
		}
	}
	return true
}

type WebDAVTestResult struct {
	Success     bool   `json:"success"`
	Reachable   bool   `json:"reachable"`
	IsWebDAV    bool   `json:"is_webdav"`
	AuthOK      bool   `json:"auth_ok"`
	DirReadable bool   `json:"dir_readable"`
	StatusCode  int    `json:"status_code"`
	DAVHeader   string `json:"dav_header,omitempty"`
	Error       string `json:"error,omitempty"`
}

func (s *MobileService) TestWebDAV(urlStr, username, password string) (*WebDAVTestResult, error) {
	result := &WebDAVTestResult{
		StatusCode: 0,
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	propfindBody := `<?xml version="1.0" encoding="UTF-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/></d:prop></d:propfind>`

	req, err := http.NewRequest("PROPFIND", urlStr, strings.NewReader(propfindBody))
	if err != nil {
		result.Error = fmt.Sprintf("invalid URL: %v", err)
		return result, nil
	}
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("Depth", "0")

	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("连接失败: %v", err)
		return result, nil
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.DAVHeader = resp.Header.Get("Dav")
	result.Reachable = true

	switch resp.StatusCode {
	case http.StatusMultiStatus:
		result.IsWebDAV = true
		result.AuthOK = true
		result.DirReadable = true
		result.Success = true
	case http.StatusUnauthorized:
		result.IsWebDAV = true
		result.AuthOK = false
		result.Success = false
		result.Error = "认证失败，请检查用户名和密码"
	case http.StatusForbidden:
		result.IsWebDAV = true
		result.AuthOK = false
		result.Success = false
		result.Error = "访问被拒绝，权限不足"
	case http.StatusNotFound:
		result.IsWebDAV = false
		result.Success = false
		result.Error = fmt.Sprintf("路径不存在 (HTTP %d)，请检查 WebDAV 地址是否正确", resp.StatusCode)
	default:
		if result.DAVHeader != "" {
			result.IsWebDAV = true
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				result.AuthOK = true
				result.DirReadable = true
				result.Success = true
			} else {
				result.Success = false
				result.Error = fmt.Sprintf("服务器返回 HTTP %d，但未返回标准 WebDAV MultiStatus 响应", resp.StatusCode)
			}
		} else {
			result.IsWebDAV = false
			result.Success = false
			result.Error = fmt.Sprintf("该地址返回了 HTTP %d，但未检测到 WebDAV 协议支持。这看起来不是一个 WebDAV 服务器（普通 HTTP 网站也会返回 2xx）。请确认地址和端口正确。", resp.StatusCode)
		}
	}

	if result.IsWebDAV && !result.DirReadable && result.StatusCode == http.StatusMultiStatus {
		req2, _ := http.NewRequest("PROPFIND", urlStr, strings.NewReader(propfindBody))
		if req2 != nil {
			req2.Header.Set("Content-Type", "application/xml; charset=utf-8")
			req2.Header.Set("Depth", "1")
			if username != "" || password != "" {
				req2.SetBasicAuth(username, password)
			}
			resp2, err2 := client.Do(req2)
			if err2 == nil {
				defer resp2.Body.Close()
				if resp2.StatusCode == http.StatusMultiStatus {
					result.DirReadable = true
				}
			}
		}
	}

	return result, nil
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

func containerTypeToString(ct uint16) string {
	switch ct {
	case 1:
		return "video"
	case 2:
		return "audio"
	case 3:
		return "image"
	case 4:
		return "document"
	case 5:
		return "text"
	default:
		return fmt.Sprintf("unknown(%d)", ct)
	}
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

func (s *MobileService) readDir(path string) ([]fs.DirEntry, error) {
	if s.dirReader != nil {
		return s.dirReader.ReadDir(path)
	}
	return os.ReadDir(path)
}

func (s *MobileService) detectContainer(path string) bool {
	if s.containerDetector != nil {
		_, err := s.containerDetector.DetectContainer(path)
		return err == nil
	}
	_, err := detector.DetectContainer(path)
	return err == nil
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
	TotalFiles  int    `json:"totalFiles"`
	TotalDirs   int    `json:"totalDirs"`
	TotalSize   int64  `json:"totalSize"`
	IndexedAt   string `json:"indexedAt"`
	IsIndexing  bool   `json:"isIndexing"`
	LastBuildMs int64  `json:"lastBuildMs"`
	Source      string `json:"source"`
	Containers  int    `json:"containers"`
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
	"jpg": true, "jpeg": true, "png": true, "gif": true,
	"webp": true, "bmp": true, "svg": true, "heic": true,
	"heif": true, "avif": true,
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

	absPath, err := utils.SafeURLToAbsPath(s.servingDir, filePath)
	if err != nil {
		return &BadRequestError{Err: fmt.Errorf("failed to resolve path: %w", err)}
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			resolved, resolveErr := utils.SafeURLToAbsPath(s.servingDir, filePath)
			if resolveErr == nil {
				if resolvedInfo, statErr := os.Stat(resolved); statErr == nil {
					slog.Info("StreamExternalFile: absolute path not found, resolved via servingDir", "input", absPath, "resolved", resolved)
					absPath = resolved
					info = resolvedInfo
					err = nil
				}
			}
		}
	}

	if err != nil {
		if os.IsNotExist(err) {
			return &NotFoundError{Err: fmt.Errorf("file not found: %s (also tried servingDir resolution)", absPath)}
		}
		return &ForbiddenError{Err: err}
	}

	if info.IsDir() {
		return &BadRequestError{Err: errors.New("path is a directory")}
	}

	if s.detectContainer(absPath) {
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
		if errors.Is(err, types.ErrWrongPassword) {
			http.Error(w, `{"error":"wrong_password","message":"密码可能错误，请检查后重试"}`, http.StatusForbidden)
			return
		}
		if errors.Is(err, types.ErrDataCorrupted) {
			http.Error(w, `{"error":"data_corrupted","message":"文件数据已损坏"}`, http.StatusUnprocessableEntity)
			return
		}
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
