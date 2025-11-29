// internal/webdav/fs.go

package webdav

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/service"
	"github.com/Soltus/encv-go/internal/utils"
	goWebdav "golang.org/x/net/webdav"
)

// NewENCVFS 创建一个新的 encvWebDAVFS 实例
func NewENCVFS(ctx context.Context) goWebdav.FileSystem {
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
		decryptionCache: make(map[string][]byte),
		pathIndex:       make(map[string]string),
		// 【关键】现在正确地初始化这些字段
		dir:          dir, // 使用处理过的绝对路径
		webdavPrefix: webdavPrefix,
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
	// 1. 检查路径是否以 WebDAV 前缀开头
	if !strings.HasPrefix(name, fs.webdavPrefix) {
		return "", fmt.Errorf("path '%s' is not under webdav root '%s'", name, fs.webdavPrefix)
	}

	// 2. 移除 WebDAV 前缀，得到相对路径
	// 例如: /webdav/output/video.mp4 - /webdav -> /output/video.mp4
	relativePath := strings.TrimPrefix(name, fs.webdavPrefix)

	// 3. 处理根目录请求
	// 如果相对路径为空或为 "/"，说明请求的是 WebDAV 根目录，映射到文件系统的 "."
	if relativePath == "" || relativePath == "/" {
		relativePath = "."
	} else {
		// 移除可能存在的开头的斜杠，因为 filepath.Join 会处理
		relativePath = strings.TrimPrefix(relativePath, "/")
	}

	// 4. 安全检查：防止路径遍历攻击
	if strings.Contains(relativePath, "..") {
		return "", fmt.Errorf("invalid path traversal attempt in '%s'", relativePath)
	}

	// 5. 拼接成最终的绝对路径
	fullPath := filepath.Join(fs.dir, relativePath)

	// 6. 最终安全检查：确保最终路径在服务目录内
	// 这是最重要的一道防线
	if !strings.HasPrefix(filepath.Clean(fullPath)+string(os.PathSeparator), filepath.Clean(fs.dir)+string(os.PathSeparator)) {
		// 特殊处理根目录本身
		if filepath.Clean(fullPath) != filepath.Clean(fs.dir) {
			return "", fmt.Errorf("resolved path '%s' is outside of serving directory '%s'", fullPath, fs.dir)
		}
	}

	return fullPath, nil
}

// statFile 获取文件信息，如果是 ENCV 容器，则返回原始文件信息
// 【关键】这个函数现在被设计为可以安全地处理文件和目录。
func (fs *encvWebDAVFS) statFile(ctx context.Context, fullPath string) (os.FileInfo, error) {
	// 步骤 1: 首先调用 os.Stat 获取路径的基本信息。
	// 这是判断一个路径是文件还是目录的最可靠方法。
	// 如果路径本身不存在或无法访问，os.Stat 会返回错误，我们直接将错误向上传递。
	baseInfo, err := os.Stat(fullPath)
	if err != nil {
		// 日志可以帮助我们确认是哪个路径出了问题
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
	log.Printf("[WebDAV-statFile] Path '%s' is a file, checking if it's a container.", fullPath)
	kviData, err := service.ExtractKVI(ctx, fullPath)
	if err != nil {
		// 如果不是容器或 KVI 提取失败，我们返回从 os.Stat 获取的原始文件信息。
		// 这避免了重复调用 os.Stat，更高效。
		log.Printf("[WebDAV-statFile] '%s' is not a container or KVI extraction failed, returning original file info. %s", fullPath, err)
		return baseInfo, nil
	}

	// 步骤 4: 如果是容器，解析 KVI 获取原始文件信息。
	index, err := utils.UnmarshalKVI(kviData)
	if err != nil {
		log.Printf("[WebDAV-statFile] KVI unmarshalling for '%s' failed, returning original file info. %s", fullPath, err)
		return baseInfo, nil
	}

	// 步骤 5: 创建并返回代表解密后文件的虚拟 FileInfo。
	origSize := index.GetOriginalFileSize()
	decryptedInfo := &decryptedFileInfo{
		name:         index.GetOriginalFilename(),
		originalName: filepath.Base(fullPath),
		size:         origSize,
		mode:         0444,
		modTime:      baseInfo.ModTime(), // 使用 baseInfo 的时间戳
		isDir:        false,
		mimeType:     mime.TypeByExtension(filepath.Ext(index.GetOriginalFilename())),
		etag:         `"` + baseInfo.ModTime().Format(time.RFC3339Nano) + "-" + fmt.Sprintf("%d", origSize) + `"`,
	}
	// log.Printf("[WebDAV-statFile] Successfully created info for container '%s' (original name: %s)", fullPath, decryptedInfo.Name())
	return decryptedInfo, nil
}

// statAsContainer 尝试将路径作为容器获取信息
func (fs *encvWebDAVFS) statAsContainer(ctx context.Context, fullPath string) (os.FileInfo, error) {
	if _, err := service.ExtractKVI(ctx, fullPath); err != nil {
		return nil, os.ErrNotExist
	}
	return fs.statFile(ctx, fullPath)
}

// statAsVirtualFile 尝试将路径作为虚拟文件获取信息
func (fs *encvWebDAVFS) statAsVirtualFile(ctx context.Context, fullPath string) (os.FileInfo, error) {
	log.Printf("[WebDAV-Stat] File not found at '%s', attempting reverse lookup.", fullPath)
	containerPath, err := fs.findContainerForDecryptedName(fullPath)
	if err != nil {
		return nil, os.ErrNotExist
	}
	log.Printf("[WebDAV-Stat] Found container '%s' for virtual path, delegating to statAsContainer.", containerPath)
	return fs.statAsContainer(ctx, containerPath)
}

// 关键实现 webdav.FileSystem 接口
func (fs *encvWebDAVFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	fullPath, err := fs.resolvePath(name)
	if err != nil {
		return nil, err
	}

	// 优先级 1: 尝试在文件系统上找到它（无论是文件还是目录）
	// statFile 现在能安全地处理这两种情况。
	if info, err := fs.statFile(ctx, fullPath); err == nil {
		return info, nil
	}

	// 优先级 2: 如果在磁盘上没找到，则尝试反向查找虚拟文件
	log.Printf("[WebDAV-Stat] File not found at '%s', attempting reverse lookup.", fullPath)
	containerPath, err := fs.findContainerForDecryptedName(fullPath)
	if err != nil {
		// 确实找不到，返回标准错误
		return nil, os.ErrNotExist
	}

	// 找到了容器，调用 statFile 获取其解密后的信息
	log.Printf("[WebDAV-Stat] Found container '%s' for virtual path, getting its info.", containerPath)
	return fs.statFile(ctx, containerPath)
}

// openAsDirectory 尝试将路径作为目录打开
func (fs *encvWebDAVFS) openAsDirectory(fullPath string, name string) (goWebdav.File, error) {
	if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
		log.Printf("[WebDAV-OpenFile] Opening directory: %s", fullPath)
		osFile, err := os.Open(fullPath)
		if err != nil {
			return nil, err
		}
		// 【关键】返回我们的自定义目录对象，而不是原始的 osFile
		return &decryptedDir{File: osFile, fs: fs, name: name}, nil
	}
	return nil, os.ErrNotExist
}

// openAsContainer 尝试将路径作为 ENCV 容器打开并解密
func (fs *encvWebDAVFS) openAsContainer(ctx context.Context, fullPath string) (goWebdav.File, error) {
	// 检查是否是容器
	if _, err := service.ExtractKVI(ctx, fullPath); err != nil {
		return nil, os.ErrNotExist
	}

	// log.Printf("[WebDAV-OpenFile] '%s' IS a container, proceeding with decryption.", fullPath)

	// 检查缓存
	if decryptedData, found := fs.decryptionCache[fullPath]; found {
		// log.Printf("[WebDAV-OpenFile] '%s' found in cache, serving from memory.", fullPath)
		fileInfo, _ := fs.statFile(ctx, fullPath)
		return &decryptedFile{
			Reader: bytes.NewReader(decryptedData),
			info:   fileInfo.(*decryptedFileInfo),
		}, nil
	}

	// 缓存未命中，执行解密
	content, err := service.DecryptContainer(ctx, fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt container %s: %w", fullPath, err)
	}
	defer content.DataStream.Close()

	decryptedBytes, err := io.ReadAll(content.DataStream)
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted content for %s: %w", fullPath, err)
	}
	// log.Printf("[WebDAV-OpenFile] Successfully decrypted %d bytes for '%s'", len(decryptedBytes), fullPath)

	// 存入缓存
	fs.decryptionCache[fullPath] = decryptedBytes

	// 获取原始文件信息
	fileInfo, err := fs.statFile(ctx, fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get stat info for decrypted file %s: %w", fullPath, err)
	}

	return &decryptedFile{
		Reader: bytes.NewReader(decryptedBytes),
		info:   fileInfo.(*decryptedFileInfo),
	}, nil
}

// openAsVirtualFile 尝试将路径作为解密后的虚拟文件打开
func (fs *encvWebDAVFS) openAsVirtualFile(ctx context.Context, fullPath string) (goWebdav.File, error) {
	log.Printf("[WebDAV-OpenFile] File not found at '%s', attempting reverse lookup.", fullPath)
	containerPath, err := fs.findContainerForDecryptedName(fullPath)
	if err != nil {
		return nil, os.ErrNotExist
	}
	// 找到了容器，调用 openAsContainer 来处理它
	log.Printf("[WebDAV-OpenFile] Found container '%s' for virtual path, delegating to openAsContainer.", containerPath)
	return fs.openAsContainer(ctx, containerPath)
}

// 关键实现  webdav.FileSystem 接口
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

	// 3. 尝试打开目录，并传递 WebDAV 路径名
	if f, err := fs.openAsDirectory(fullPath, name); err == nil {
		return f, nil
	}

	// 4. 尝试直接打开文件 (这会处理普通文件和容器文件)
	if f, err := os.Open(fullPath); err == nil {
		// 如果打开成功，需要判断是不是容器
		if _, kviErr := service.ExtractKVI(ctx, fullPath); kviErr == nil {
			// 是容器，关闭它，然后走解密流程
			f.Close()
			log.Printf("[WebDAV-OpenFile] Opened container '%s', now delegating to openAsContainer.", fullPath)
			return fs.openAsContainer(ctx, fullPath)
		}
		// 是普通文件，直接返回
		// log.Printf("[WebDAV-OpenFile] Opening standard file: %s", fullPath)
		return f, nil
	}

	// 5. 直接打开失败，尝试反向查找虚拟文件
	log.Printf("[WebDAV-OpenFile] Direct open failed for '%s', attempting reverse lookup.", fullPath)
	return fs.openAsVirtualFile(ctx, fullPath)
}

// getCachedFileInfo 是一个辅助函数，用于从缓存中获取或创建FileInfo
// 这是一个简化的实现，实际项目中可能需要更复杂的缓存结构
func (fs *encvWebDAVFS) getCachedFileInfo(ctx context.Context, fullPath string, decryptedData []byte) *decryptedFileInfo {
	// 在真实场景中，你应该把FileInfo对象也存入缓存
	// 这里为了简化，我们重新解析一次
	kviData, _ := service.ExtractKVI(ctx, fullPath)
	index, _ := utils.UnmarshalKVI(kviData)
	containerInfo, _ := os.Stat(fullPath)

	return &decryptedFileInfo{
		name:    index.GetOriginalFilename(),
		size:    int64(len(decryptedData)),
		mode:    0444,
		modTime: containerInfo.ModTime(),
		isDir:   false,
	}
}

// ReadDir 实现 webdav.FileSystem 接口，用于列出目录内容
func (fs *encvWebDAVFS) ReadDir(ctx context.Context, name string) ([]os.FileInfo, error) {
	fullPath, err := fs.resolvePath(name)
	if err != nil {
		return nil, err
	}
	log.Printf("[WebDAV-ReadDir] Reading directory (using index): %s", fullPath)

	fs.pathIndexMutex.RLock()
	defer fs.pathIndexMutex.RUnlock()

	var fileInfos []os.FileInfo
	processedDirs := make(map[string]bool) // 防止重复添加目录

	// 遍历索引，找出属于当前目录的文件
	for virtualPath, realPath := range fs.pathIndex {
		// 检查虚拟路径是否在当前请求的目录下
		if !strings.HasPrefix(virtualPath, name) {
			continue
		}

		// 获取文件名部分
		_, err := filepath.Rel(name, virtualPath)
		if err != nil {
			continue
		}

		// 检查是否是子目录
		relPath := strings.TrimPrefix(virtualPath, name)
		if strings.Contains(relPath, "/") {
			subDirName := strings.Split(relPath, "/")[0]
			if !processedDirs[subDirName] {
				subDirFullPath := filepath.Join(fullPath, subDirName)
				if info, err := os.Stat(subDirFullPath); err == nil && info.IsDir() {
					fileInfos = append(fileInfos, info)
					processedDirs[subDirName] = true
				}
			}
			continue
		}

		// 是当前目录下的文件，调用 statFile 获取信息
		info, err := fs.statFile(ctx, realPath)
		if err != nil {
			log.Printf("[WebDAV-ReadDir] Skipping FILE '%s' because statFile failed: %v", realPath, err)
			continue
		}
		fileInfos = append(fileInfos, info)
	}

	// 【重要】还要处理目录下可能存在的普通文件（非容器）
	entries, _ := os.ReadDir(fullPath)
	for _, entry := range entries {
		if entry.IsDir() {
			if !processedDirs[entry.Name()] {
				if info, err := entry.Info(); err == nil {
					fileInfos = append(fileInfos, info)
				}
			}
			continue
		}
		// 如果是普通文件，且不在索引中，则添加它
		entryVirtualPath := filepath.Join(name, entry.Name())
		if _, exists := fs.pathIndex[entryVirtualPath]; !exists {
			if info, err := entry.Info(); err == nil {
				fileInfos = append(fileInfos, info)
			}
		}
	}

	return fileInfos, nil
}

// --- 其他需要实现的 webdav.FileSystem 方法 ---

// Mkdir, RemoveAll, Rename 都不支持
func (fs *encvWebDAVFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	return os.ErrPermission
}

func (fs *encvWebDAVFS) RemoveAll(ctx context.Context, name string) error {
	return os.ErrPermission
}

func (fs *encvWebDAVFS) Rename(ctx context.Context, oldName, newName string) error {
	return os.ErrPermission
}

// findContainerForDecryptedName 根据解密后的文件名反向查找对应的容器文件路径
func (fs *encvWebDAVFS) findContainerForDecryptedName(fullPath string) (string, error) {
	// fullPath 是真实路径，我们需要转换为虚拟路径来查找索引
	relPath, err := filepath.Rel(fs.dir, fullPath)
	if err != nil {
		return "", err
	}
	virtualPath := "/" + filepath.ToSlash(relPath)

	fs.pathIndexMutex.RLock()
	defer fs.pathIndexMutex.RUnlock()

	if realPath, found := fs.pathIndex[virtualPath]; found {
		log.Printf("[findContainer] Found in index: '%s' -> '%s'", virtualPath, realPath)
		return realPath, nil
	}

	log.Printf("[findContainer] Not found in index for '%s'", virtualPath)
	return "", os.ErrNotExist
}

func (f *decryptedFile) Stat() (os.FileInfo, error) {
	return f.info, nil
}

// bytes.Reader 不需要关闭，所以返回 nil 即可
func (f *decryptedFile) Close() error {
	return nil
}

// 【关键修改】让 Seek 方法真正地调用嵌入的 Reader 的 Seek
func (f *decryptedFile) Seek(offset int64, whence int) (int64, error) {
	// 直接调用嵌入的 bytes.Reader 的 Seek 方法
	return f.Reader.Seek(offset, whence)
}

// Readdir 不支持，因为它不是目录
func (f *decryptedFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, os.ErrInvalid
}

// 【修复 2】添加 Write 方法以满足接口要求，但返回权限错误
func (f *decryptedFile) Write(p []byte) (n int, err error) {
	return 0, os.ErrPermission
}

func (dfi *decryptedFileInfo) Name() string {
	// 返回解密后的原始文件名，让客户端看到它
	return dfi.name
}

// func (dfi *decryptedFileInfo) Name() string       { return dfi.name }
func (dfi *decryptedFileInfo) Size() int64        { return dfi.size }
func (dfi *decryptedFileInfo) Mode() os.FileMode  { return dfi.mode }
func (dfi *decryptedFileInfo) ModTime() time.Time { return dfi.modTime }
func (dfi *decryptedFileInfo) IsDir() bool        { return dfi.isDir }
func (dfi *decryptedFileInfo) Sys() interface{}   { return nil }

// 【关键】实现 ContentTyper 接口
func (dfi *decryptedFileInfo) ContentType() string {
	return dfi.mimeType
}

// 【关键】实现 ETager 接口
func (dfi *decryptedFileInfo) ETag() string {
	return dfi.etag
}

// Stat 返回原始目录信息
func (d *decryptedDir) Stat() (os.FileInfo, error) {
	return d.File.Stat()
}

// Close 关闭原始文件句柄
func (d *decryptedDir) Close() error {
	return d.File.Close()
}

// 【关键】覆盖 Readdir 方法，调用我们自定义的 ReadDir 逻辑
func (d *decryptedDir) Readdir(count int) ([]os.FileInfo, error) {
	// log.Printf("[decryptedDir] Readdir called for '%s', delegating to fs.ReadDir.", d.name)
	// 调用我们自定义的 fs.ReadDir
	return d.fs.ReadDir(context.Background(), d.name)
}

// 其他方法对目录无意义，返回错误
func (d *decryptedDir) Read(p []byte) (n int, err error)             { return 0, os.ErrInvalid }
func (d *decryptedDir) Seek(offset int64, whence int) (int64, error) { return 0, os.ErrInvalid }
func (d *decryptedDir) Write(p []byte) (n int, err error)            { return 0, os.ErrPermission }

// buildPathIndex 递归构建路径索引
func (fs *encvWebDAVFS) buildPathIndex(ctx context.Context) error {
	// 使用已经解析好的绝对路径
	log.Printf("[Index] Building path index for root: %s", fs.dir)
	fs.pathIndexMutex.Lock()
	defer fs.pathIndexMutex.Unlock()

	// 清空旧索引
	fs.pathIndex = make(map[string]string)

	// 使用已经解析好的绝对路径
	return filepath.Walk(fs.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// 检查是否是容器
		kviData, err := service.ExtractKVI(ctx, path)
		if err != nil {
			return nil // 不是容器，跳过
		}

		index, err := utils.UnmarshalKVI(kviData)
		if err != nil {
			return nil // KVI 损坏，跳过
		}

		// 获取虚拟路径
		relPath, err := filepath.Rel(fs.dir, path)
		if err != nil {
			return err
		}
		virtualPath := "/" + filepath.ToSlash(relPath)

		// 获取原始文件名，并构建新的虚拟路径
		dir := filepath.Dir(virtualPath)
		originalFilename := index.GetOriginalFilename()
		newVirtualPath := filepath.Join(dir, originalFilename)
		if dir == "." || dir == "/" {
			newVirtualPath = "/" + originalFilename
		}

		log.Printf("[Index] Mapping: '%s' -> '%s'", newVirtualPath, path)
		fs.pathIndex[newVirtualPath] = path

		return nil
	})
}
