package webdav

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/v2/container/manifest"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	"github.com/Soltus/encv-go/internal/v2/reader"
	"github.com/Soltus/encv-go/internal/v2/service"
	"github.com/Soltus/encv-go/internal/v2/types"
	goWebdav "golang.org/x/net/webdav"
)

// encvWebDAVFS 是一个自定义的 webdav.FileSystem
// 它拦截文件请求，如果文件是 ENCV 容器，则提供解密后的流
type encvWebDAVFS struct {
	dir string // WebDAV 服务的本地文件系统目录（绝对路径）
	// 【新增】WebDAV 的 URL 前缀 (例如 "/webdav/")
	webdavPrefix string
	// 【新增】注入 v2 架构的核心依赖
	readerService *service.ReaderService
	cfg           *config.Config

	// 【新增】文件名索引
	// key: 虚拟路径 (e.g., "/output/config.user.json")
	// value: 真实路径 (e.g., "A:\\path\\to\\output\\config.user.nosj.sccgt")
	pathIndex map[string]string
	// 【新增】用于保护 pathIndex 的互斥锁
	pathIndexMutex sync.RWMutex
}

// --- 辅助结构体 ---

// decryptedFile 实现了 webdav.File 接口
type decryptedFile struct {
	io.Reader // 嵌入流式 DecryptReader
	info      *decryptedFileInfo
	// 【关键】持有 DecryptReader 以便在 Close 时正确关闭它
	decryptReader reader.DecryptReader
}

// decryptedFileInfo 实现了 os.FileInfo 接口
type decryptedFileInfo struct {
	// name 存储解密后的原始文件名
	name string
	// originalName 存储容器加密后的文件名（磁盘上的真实文件）
	originalName string
	size         int64
	mode         os.FileMode
	modTime      time.Time
	isDir        bool
	// 用于满足 WebDAV 的额外属性
	mimeType string
	etag     string
}

// decryptedDir 实现了 webdav.File 接口，用于目录
// 它覆盖了 Readdir 方法，以提供解密后的文件列表
type decryptedDir struct {
	*os.File // 嵌入原始的文件句柄
	fs       *encvWebDAVFS
	name     string // WebDAV 路径名，例如 "/webdav/output"
}

// NewENCVFS 创建一个新的 encvWebDAVFS 实例
// 【修改】构造函数现在需要接收 ReaderService 和 Config
func NewENCVFS(ctx context.Context, readerService *service.ReaderService) goWebdav.FileSystem {
	cfg := config.FromContext(ctx)
	// 【关键】规范化服务目录路径，防止路径问题
	dir := cfg.Webdav.Dir
	var err error
	if dir == "/" {
		dir, err = os.Getwd()
		if err != nil {
			log.Fatalf("Failed to get current working directory for WebDAV: %v", err)
		}
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		log.Fatalf("Failed to resolve absolute path for WebDAV directory '%s': %v", dir, err)
	}
	// 规范化 WebDAV 前缀，确保它是一个以 '/' 开头且不以 '/' 结尾的路径
	webdavPrefix := strings.TrimSuffix(cfg.Webdav.Root, "/")
	if !strings.HasPrefix(webdavPrefix, "/") {
		webdavPrefix = "/" + webdavPrefix
	}

	fs := &encvWebDAVFS{
		pathIndex:     make(map[string]string),
		dir:           dir, // 使用处理过的绝对路径
		webdavPrefix:  webdavPrefix,
		readerService: readerService, // 【关键】注入依赖
		cfg:           cfg,           // 【关键】注入依赖
	}

	log.Printf("[WebDAV] Initializing FS for directory: %s, with prefix: %s", fs.dir, fs.webdavPrefix)

	// 启动时构建索引
	if err := fs.buildPathIndex(ctx); err != nil {
		log.Printf("[FATAL] Failed to build initial path index: %v", err)
	}

	return fs
}

// resolvePath 将 WebDAV 路径安全地解析为本地文件系统绝对路径
func (fs *encvWebDAVFS) resolvePath(name string) (string, error) {
	// ... (此函数保持不变，是安全的路径解析核心) ...
	if !strings.HasPrefix(name, fs.webdavPrefix) {
		return "", fmt.Errorf("path '%s' is not under webdav root '%s'", name, fs.webdavPrefix)
	}
	relativePath := strings.TrimPrefix(name, fs.webdavPrefix)
	if relativePath == "" || relativePath == "/" {
		relativePath = "."
	} else {
		relativePath = strings.TrimPrefix(relativePath, "/")
	}
	if strings.Contains(relativePath, "..") {
		return "", fmt.Errorf("invalid path traversal attempt in '%s'", relativePath)
	}
	fullPath := filepath.Join(fs.dir, relativePath)
	if !strings.HasPrefix(filepath.Clean(fullPath)+string(os.PathSeparator), filepath.Clean(fs.dir)+string(os.PathSeparator)) {
		if filepath.Clean(fullPath) != filepath.Clean(fs.dir) {
			return "", fmt.Errorf("resolved path '%s' is outside of serving directory '%s'", fullPath, fs.dir)
		}
	}
	return fullPath, nil
}

// 【新增辅助函数】从容器路径获取 Index，封装了新的架构逻辑
func (fs *encvWebDAVFS) getIndexFromContainerPath(fullPath string) (types.Index, error) {
	// 1. 提取 Manifest 的原始 JSON 字节
	manifestBytes, err := manifest.ExtractManifest_v2(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract manifest bytes from %s: %w", fullPath, err)
	}

	// 2. 将字节反序列化为 Manifest 结构体
	manifestStruct, err := manifest.DeserializeFromJSON_v2(manifestBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize manifest from %s: %w", fullPath, err)
	}

	// 3. 使用注册表动态创建 KVIProvider
	provider, err := types.NewKVIProviderFromManifest(manifestStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to create KVI provider from %s: %w", fullPath, err)
	}

	// 4. 从 Provider 获取 Index
	return provider.GetIndex(), nil
}

// buildPathIndex 递归构建路径索引，储完整的、标准化的 WebDAV 虚拟路径
func (fs *encvWebDAVFS) buildPathIndex(ctx context.Context) error {
	log.Printf("[Index] Building path index for root: %s", fs.dir)
	fs.pathIndexMutex.Lock()
	defer fs.pathIndexMutex.Unlock()
	fs.pathIndex = make(map[string]string)

	return filepath.Walk(fs.dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		index, err := fs.getIndexFromContainerPath(p)
		if err != nil {
			// 不是容器或 KVI 损坏，跳过
			return nil
		}

		// 获取相对于服务目录的路径
		relPath, err := filepath.Rel(fs.dir, p)
		if err != nil {
			return err
		}

		// 获取容器所在的虚拟目录
		containerVirtualDir := fs.webdavPrefix + filepath.ToSlash(filepath.Dir(relPath))
		if containerVirtualDir == fs.webdavPrefix {
			containerVirtualDir += "/" // 确保根目录是 /webdav/
		}

		// 构建完整的、标准化的虚拟文件路径
		// path.Join 总是使用正斜杠，符合 URL 路径规范
		// 它能正确处理根目录和子目录的情况
		fullVirtualPath := path.Join(
			fs.webdavPrefix,                         // 例如 "/webdav"
			filepath.ToSlash(filepath.Dir(relPath)), // 例如 "output" 或 "."
			index.GetOriginalFilename(),             // 例如 "321.mp4"
		)

		log.Printf("[Index] Mapping: '%s' -> '%s'", fullVirtualPath, p)
		fs.pathIndex[fullVirtualPath] = p

		return nil
	})
}

// statFile 获取文件信息，如果是 ENCV 容器，则返回原始文件信息
// 【关键】这个函数现在被设计为可以安全地处理文件和目录。
func (fs *encvWebDAVFS) statFile(ctx context.Context, fullPath string) (os.FileInfo, error) {
	// 步骤 1: 首先调用 os.Stat 获取路径的基本信息。
	// 这是判断一个路径是文件还是目录的最可靠方法。
	// 如果路径本身不存在或无法访问，os.Stat 会返回错误，我们直接将错误向上传递。
	baseInfo, err := os.Stat(fullPath)
	if err != nil {
		log.Printf("[WebDAV-statFile] os.Stat failed for '%s': %v", fullPath, err)
		return nil, err
	}
	// 步骤 2: 检查它是否是一个目录。
	// 如果是目录，我们**直接返回**其信息，并且**绝不**进行任何容器检测。
	// 这是防止将目录误判为容器、从而避免 panic 的核心防线。
	if baseInfo.IsDir() {
		log.Printf("[WebDAV-statFile] Path '%s' is a directory, returning its info directly.", fullPath)
		return baseInfo, nil
	}

	// 步骤 3: 从这里开始，我们 100% 确定它是一个文件。
	// 现在可以安全地尝试将其作为 ENCV 容器来处理。
	index, err := fs.getIndexFromContainerPath(fullPath)
	if err != nil {
		// 不是容器或 KVI 损坏，返回原始文件信息
		log.Printf("[WebDAV-statFile] '%s' is not a container or KVI extraction failed, returning original file info. %s", fullPath, err)
		return baseInfo, nil
	}
	// 步骤 5: 创建并返回代表解密后文件的虚拟 FileInfo。
	origSize := index.GetOriginalFileSize()
	decryptedInfo := &decryptedFileInfo{
		name:         index.GetOriginalFilename(),
		originalName: filepath.Base(fullPath),
		size:         origSize,
		mode:         0444,
		modTime:      baseInfo.ModTime(),
		isDir:        false,
		mimeType:     mime.TypeByExtension(filepath.Ext(index.GetOriginalFilename())),
		etag:         `"` + baseInfo.ModTime().Format(time.RFC3339Nano) + "-" + fmt.Sprintf("%d", origSize) + `"`,
	}
	// log.Printf("[WebDAV-statFile] Successfully created info for container '%s' (original name: %s)", fullPath, decryptedInfo.Name())
	return decryptedInfo, nil
}

// 【完全重写】openAsContainer 使用新的 v2 架构进行流式解密
func (fs *encvWebDAVFS) openAsContainer(ctx context.Context, fullPath string) (goWebdav.File, error) {
	// 1. 使用插件系统找到合适的解密插件
	p, err := plugins.FindDecryptingPlugin(fullPath)
	if err != nil {
		return nil, fmt.Errorf("could not find decrypting plugin for %s: %w", fullPath, err)
	}
	p.Intialize(ctx)
	namer := p.GetChunkNamer()

	// 2. 【核心】使用 ReaderService 获取流式解密器，不再读取整个文件到内存
	decryptReader, index, _, err := fs.readerService.GetDecryptReader(*fs.cfg, fullPath, fs.cfg.Password, namer)
	if err != nil {
		return nil, fmt.Errorf("failed to create decrypt reader for %s: %w", fullPath, err)
	}

	// 3. 创建虚拟文件信息
	fileInfo := &decryptedFileInfo{
		name:         index.GetOriginalFilename(),
		originalName: filepath.Base(fullPath),
		size:         index.GetOriginalFileSize(),
		mode:         0444,
		modTime:      time.Now(), // 可以从 index 中获取更准确的时间
		isDir:        false,
		mimeType:     mime.TypeByExtension(filepath.Ext(index.GetOriginalFilename())),
		etag:         `"` + time.Now().Format(time.RFC3339Nano) + "-" + fmt.Sprintf("%d", index.GetOriginalFileSize()) + `"`,
	}

	// 4. 返回一个包装了流式解密器的 WebDAV 文件对象
	return &decryptedFile{
		Reader:        decryptReader,
		decryptReader: decryptReader, // 持有引用以便关闭
		info:          fileInfo,
	}, nil
}

// --- 实现 webdav.FileSystem 接口 ---

func (fs *encvWebDAVFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	fullPath, err := fs.resolvePath(name)
	if err != nil {
		return nil, err
	}

	// 1. 尝试直接获取文件信息（处理普通文件和目录）
	if info, err := fs.statFile(ctx, fullPath); err == nil {
		return info, nil
	}

	// 2. 如果在磁盘上没找到，则直接在索引中查找虚拟文件
	fs.pathIndexMutex.RLock()
	realPath, found := fs.pathIndex[name]
	fs.pathIndexMutex.RUnlock()

	if !found {
		// 确实找不到
		return nil, os.ErrNotExist
	}

	// 找到了容器，调用 statFile 获取其解密后的信息
	log.Printf("[WebDAV-Stat] Found container '%s' for virtual path '%s'", realPath, name)
	return fs.statFile(ctx, realPath)
}

func (fs *encvWebDAVFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (goWebdav.File, error) {
	// 1. 权限检查
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0 {
		return nil, os.ErrPermission
	}
	// 2. 路径解析
	fullPath, err := fs.resolvePath(name)
	if err != nil {
		return nil, err
	}

	// 3. 尝试打开目录
	if f, err := fs.openAsDirectory(fullPath, name); err == nil {
		return f, nil
	}

	// 4. 尝试直接打开文件 (这会处理普通文件和容器文件)
	if f, err := os.Open(fullPath); err == nil {
		if _, kviErr := manifest.ExtractKVI_v2(fullPath); kviErr == nil {
			// 是容器，关闭它，然后走解密流程
			f.Close()
			log.Printf("[WebDAV-OpenFile] Opened container '%s', now delegating to openAsContainer.", fullPath)
			return fs.openAsContainer(ctx, fullPath)
		}
		// 是普通文件，直接返回
		return f, nil
	}

	// 5. 直接打开失败，尝试在索引中查找虚拟文件
	fs.pathIndexMutex.RLock()
	realPath, found := fs.pathIndex[name]
	fs.pathIndexMutex.RUnlock()

	if !found {
		// 确实找不到
		return nil, os.ErrNotExist
	}

	// 找到了容器，调用 openAsContainer 来处理它
	log.Printf("[WebDAV-OpenFile] Found container '%s' for virtual path '%s'", realPath, name)
	return fs.openAsContainer(ctx, realPath)
}

// --- 实现 webdav.File 接口 ---

// 【关键修改】Close 现在会关闭底层的 DecryptReader
func (f *decryptedFile) Close() error {
	if f.decryptReader != nil {
		return f.decryptReader.Close()
	}
	return nil
}

func (f *decryptedFile) Stat() (os.FileInfo, error) {
	return f.info, nil
}

// 【关键修改】Seek 方法现在需要处理流式 Reader
func (f *decryptedFile) Seek(offset int64, whence int) (int64, error) {
	// 优先检查是否支持 io.Seeker (适用于本地文件)
	if seeker, ok := f.decryptReader.(io.Seeker); ok {
		return seeker.Seek(offset, whence)
	}
	// 其次检查是否支持自定义的 SeekTo (适用于远程视频)
	type seekerTo interface{ SeekTo(offset int64) error }
	if seekerTo, ok := f.decryptReader.(seekerTo); ok {
		if whence == io.SeekStart {
			err := seekerTo.SeekTo(offset)
			// SeekTo 不返回新位置，我们无法模拟，只能返回成功
			// 这对于某些 WebDAV 客户端可能不够，但这是流式限制下的最佳努力
			return 0, err
		}
		return 0, fmt.Errorf("SeekTo only supports SeekStart")
	}

	// 如果都不支持，返回错误
	log.Printf("WARN: [WebDAV-decryptedFile] Seek is not supported for this file type.")
	return 0, os.ErrInvalid
}

func (f *decryptedFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, os.ErrInvalid
}

func (f *decryptedFile) Write(p []byte) (n int, err error) {
	return 0, os.ErrPermission
}

// --- 辅助函数 ---

// openAsDirectory 尝试将路径作为目录打开
func (fs *encvWebDAVFS) openAsDirectory(fullPath string, name string) (goWebdav.File, error) {
	if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
		log.Printf("[WebDAV-OpenFile] Opening directory: %s", fullPath)
		osFile, err := os.Open(fullPath)
		if err != nil {
			return nil, err
		}
		return &decryptedDir{File: osFile, fs: fs, name: name}, nil
	}
	return nil, os.ErrNotExist
}

// statAsContainer 尝试将路径作为容器获取信息
func (fs *encvWebDAVFS) statAsContainer(ctx context.Context, fullPath string) (os.FileInfo, error) {
	if _, err := manifest.ExtractKVI_v2(fullPath); err != nil {
		return nil, os.ErrNotExist
	}
	return fs.statFile(ctx, fullPath)
}

// statAsVirtualFile 尝试将路径作为虚拟文件获取信息
// func (fs *encvWebDAVFS) statAsVirtualFile(ctx context.Context, fullPath string) (os.FileInfo, error) {
// 	log.Printf("[WebDAV-Stat] File not found at '%s', attempting reverse lookup.", fullPath)
// 	containerPath, err := fs.findContainerForDecryptedName(fullPath)
// 	if err != nil {
// 		return nil, os.ErrNotExist
// 	}
// 	log.Printf("[WebDAV-Stat] Found container '%s' for virtual path, delegating to statAsContainer.", containerPath)
// 	return fs.statAsContainer(ctx, containerPath)
// }

// openAsVirtualFile 尝试将路径作为解密后的虚拟文件打开
// func (fs *encvWebDAVFS) openAsVirtualFile(ctx context.Context, fullPath string) (goWebdav.File, error) {
// 	log.Printf("[WebDAV-OpenFile] File not found at '%s', attempting reverse lookup.", fullPath)
// 	containerPath, err := fs.findContainerForDecryptedName(fullPath)
// 	if err != nil {
// 		return nil, os.ErrNotExist
// 	}
// 	log.Printf("[WebDAV-OpenFile] Found container '%s' for virtual path, delegating to openAsContainer.", containerPath)
// 	return fs.openAsContainer(ctx, containerPath)
// }

// findContainerForDecryptedName 根据解密后的文件名反向查找对应的容器文件路径
// func (fs *encvWebDAVFS) findContainerForDecryptedName(fullPath string) (string, error) {
// 	relPath, err := filepath.Rel(fs.dir, fullPath)
// 	if err != nil {
// 		return "", err
// 	}
// 	virtualPath := "/" + filepath.ToSlash(relPath)
// 	fs.pathIndexMutex.RLock()
// 	defer fs.pathIndexMutex.RUnlock()
// 	if realPath, found := fs.pathIndex[virtualPath]; found {
// 		log.Printf("[findContainer] Found in index: '%s' -> '%s'", virtualPath, realPath)
// 		return realPath, nil
// 	}
// 	log.Printf("[findContainer] Not found in index for '%s'", virtualPath)
// 	return "", os.ErrNotExist
// }

func (fs *encvWebDAVFS) ReadDir(ctx context.Context, name string) ([]os.FileInfo, error) {
	// 1. 解析请求的路径
	fullPath, err := fs.resolvePath(name)
	if err != nil {
		return nil, err
	}
	log.Printf("[WebDAV-ReadDir] Reading directory: %s (full path: %s)", name, fullPath)

	fs.pathIndexMutex.RLock()
	defer fs.pathIndexMutex.RUnlock()

	// 使用 map 来存储结果，键为文件/目录名，可以自动去重
	fileInfos := make(map[string]os.FileInfo)

	// 2. 从索引中收集文件和子目录
	for virtualPath, realPath := range fs.pathIndex {
		// 检查虚拟路径是否在当前请求的目录下
		if !strings.HasPrefix(virtualPath, name) {
			continue
		}

		// 获取相对于请求路径的部分
		// 例如: virtualPath="/webdav/output/subdir/file.txt", name="/webdav/output"
		// relPart 应该是 "subdir/file.txt"
		relPart := strings.TrimPrefix(virtualPath, name)
		if !strings.HasPrefix(relPart, "/") {
			relPart = "/" + relPart // 确保以 / 开头，方便后续处理
		}

		parts := strings.Split(strings.Trim(relPart, "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			continue
		}

		firstPart := parts[0] // 这是直接位于当前目录下的条目名

		// 如果我们已经处理过这个条目（无论是文件还是目录），就跳过
		if _, exists := fileInfos[firstPart]; exists {
			continue
		}

		// 判断是文件还是子目录
		if len(parts) == 1 { // 是文件
			info, err := fs.statFile(ctx, realPath)
			if err != nil {
				log.Printf("[WebDAV-ReadDir] Skipping indexed FILE '%s' because statFile failed: %v", realPath, err)
				continue
			}
			fileInfos[firstPart] = info
		} else { // 是子目录
			// 尝试获取磁盘上对应物理目录的信息
			subDirFullPath := filepath.Join(fullPath, firstPart)
			if info, err := os.Stat(subDirFullPath); err == nil && info.IsDir() {
				fileInfos[firstPart] = info
			} else {
				// 如果物理目录不存在，但索引中有，我们创建一个虚拟的目录信息
				// 这对于只包含容器的虚拟目录很有用
				virtualDirInfo := &decryptedFileInfo{
					name:    firstPart,
					size:    0,
					mode:    os.ModeDir | 0555,
					modTime: time.Now(),
					isDir:   true,
				}
				fileInfos[firstPart] = virtualDirInfo
			}
		}
	}

	// 3. 添加磁盘上存在但不在索引中的文件
	entries, _ := os.ReadDir(fullPath)
	for _, entry := range entries {
		entryName := entry.Name()
		if _, exists := fileInfos[entryName]; !exists {
			if info, err := entry.Info(); err == nil {
				fileInfos[entryName] = info
			}
		}
	}

	// 4. 将 map 转换为切片返回
	var result []os.FileInfo
	for _, info := range fileInfos {
		result = append(result, info)
	}

	return result, nil
}

func (fs *encvWebDAVFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	return os.ErrPermission
}

func (fs *encvWebDAVFS) RemoveAll(ctx context.Context, name string) error {
	return os.ErrPermission
}

func (fs *encvWebDAVFS) Rename(ctx context.Context, oldName, newName string) error {
	return os.ErrPermission
}

// decryptedFileInfo, decryptedDir 的方法保持不变
func (dfi *decryptedFileInfo) Name() string        { return dfi.name }
func (dfi *decryptedFileInfo) Size() int64         { return dfi.size }
func (dfi *decryptedFileInfo) Mode() os.FileMode   { return dfi.mode }
func (dfi *decryptedFileInfo) ModTime() time.Time  { return dfi.modTime }
func (dfi *decryptedFileInfo) IsDir() bool         { return dfi.isDir }
func (dfi *decryptedFileInfo) Sys() interface{}    { return nil }
func (dfi *decryptedFileInfo) ContentType() string { return dfi.mimeType }
func (dfi *decryptedFileInfo) ETag() string        { return dfi.etag }

func (d *decryptedDir) Stat() (os.FileInfo, error) { return d.File.Stat() }
func (d *decryptedDir) Close() error               { return d.File.Close() }
func (d *decryptedDir) Readdir(count int) ([]os.FileInfo, error) {
	return d.fs.ReadDir(context.Background(), d.name)
}
func (d *decryptedDir) Read(p []byte) (n int, err error)             { return 0, os.ErrInvalid }
func (d *decryptedDir) Seek(offset int64, whence int) (int64, error) { return 0, os.ErrInvalid }
func (d *decryptedDir) Write(p []byte) (n int, err error)            { return 0, os.ErrPermission }
