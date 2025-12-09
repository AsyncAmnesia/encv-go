package webdav

import (
	"context"
	"fmt"
	"io"
	iofs "io/fs"
	"log"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/container/manifest"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	"github.com/Soltus/encv-go/internal/v2/reader"
	"github.com/Soltus/encv-go/internal/v2/service"
	"github.com/Soltus/encv-go/internal/v2/types"
	"github.com/fsnotify/fsnotify"
	goWebdav "golang.org/x/net/webdav"
)

// encvWebDAVFS 是一个自定义的 webdav.FileSystem
// 它拦截文件请求，如果文件是 ENCV 容器，则提供解密后的流
type encvWebDAVFS struct {
	dir string // WebDAV 服务的本地文件系统目录（绝对路径）
	// WebDAV 的 URL 前缀 (例如 "/webdav/")
	webdavPrefix string
	// 注入 v2 架构的核心依赖
	readerService *service.ReaderService
	cfg           *config.Config

	// 【优化】使用 atomic.Value 存储复合索引结构
	indexes atomic.Value
	// 用于保护 pathIndex 的互斥锁
	// pathIndexMutex sync.RWMutex
	// 【新增】用于通知索引构建完成
	indexReady chan struct{}
	// 【新增】Index 对象缓存
	// key: 容器文件的完整路径
	// value: types.Index
	indexCache sync.Map // sync.Map 适合读多写少的场景
	// 【新增】定义要排除的目录
	excludeDirs map[string]bool
	// 【新增】定义已知的容器文件扩展名
	containerExtensions map[string]bool
	// 【新增】文件系统监视器
	watcher *fsnotify.Watcher
}

// --- 辅助结构体 ---

// 【新增】一个结构体来持有所有索引数据
type pathIndexes struct {
	// 虚拟路径 -> 真实容器路径
	// 文件名索引
	// key: 虚拟路径 (e.g., "/output/config.user.json")
	// value: 真实路径 (e.g., "A:\\path\\to\\output\\config.user.nosj.sccgt")
	pathMap map[string]string
	// 父目录路径 -> 子项名称列表
	dirMap map[string][]string
	// 【新增】虚拟路径 -> 预计算的 FileInfo
	// 这样 ReadDir 就不需要在运行时调用 statFile 了
	fileInfoMap map[string]os.FileInfo
}

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

	// 1. 从插件系统获取扩展名列表
	registeredExtsSlice := plugins.GetAllRegisteredContainerExtensions()

	// 2. 将 []string 转换为 map[string]bool 以实现 O(1) 查找
	containerExtensionsMap := make(map[string]bool, len(registeredExtsSlice))
	for _, ext := range registeredExtsSlice {
		// 使用 strings.ToLower 确保匹配是大小写不敏感的
		containerExtensionsMap[strings.ToLower(ext)] = true
	}

	fs := &encvWebDAVFS{
		dir:                 dir, // 使用处理过的绝对路径
		webdavPrefix:        webdavPrefix,
		readerService:       readerService, // 【关键】注入依赖
		cfg:                 cfg,           // 【关键】注入依赖
		indexReady:          make(chan struct{}),
		containerExtensions: containerExtensionsMap,
		excludeDirs: map[string]bool{
			"node_modules": true,
			".git":         true,
			".idea":        true,
			// 可以根据需要添加更多
		},
	}
	// 初始化 atomic.Value，存储一个空的 map
	fs.indexes.Store(&pathIndexes{
		pathMap: make(map[string]string),
		dirMap:  make(map[string][]string),
	})

	// 启动后台索引构建和监视
	go fs.runIndexer(ctx)

	log.Printf("[WebDAV] FS initialized. Index building in the background.")
	log.Printf("[WebDAV] Registered container extensions for filtering: %v", registeredExtsSlice)

	return fs
}

// runIndexer 后台运行索引构建和监视任务
func (fs *encvWebDAVFS) runIndexer(ctx context.Context) {
	// 1. 首次构建
	if err := fs.buildPathIndexInBackground(ctx, nil); err != nil {
		log.Printf("[ERROR] Initial background index build failed: %v", err)
	} else {
		close(fs.indexReady) // 通知首次构建完成
		log.Printf("[Index] Initial background index build complete.")
	}

	// 2. 设置文件系统监视
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("[ERROR] Failed to create fsnotify watcher: %v", err)
		return
	}
	fs.watcher = watcher
	defer watcher.Close()

	// 递归添加监视路径
	_ = filepath.Walk(fs.dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
	log.Printf("[Index] File system watcher active for: %s", fs.dir)

	// 3. 监听变化并防抖重建
	var rebuildTimer *time.Timer
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// 【修复】将所有事件处理逻辑移到这里
			// 当文件被删除、重命名或写入时，清理缓存
			if event.Op&(fsnotify.Remove|fsnotify.Rename|fsnotify.Write) != 0 {
				fs.indexCache.Delete(event.Name)
			}

			// 我们只关心创建、写入、删除、重命名来触发索引重建
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0 {
				// 防抖：如果短时间内有多个事件，只触发一次重建
				if rebuildTimer != nil {
					rebuildTimer.Stop()
				}
				rebuildTimer = time.AfterFunc(2*time.Second, func() {
					log.Printf("[Index] Triggering background index rebuild due to file system changes.")

					// 【修改】获取旧索引，用于回退
					oldIndexes := fs.getIndexes()

					if err := fs.buildPathIndexInBackground(ctx, oldIndexes); err != nil {
						log.Printf("[ERROR] Background index rebuild failed: %v", err)
					} else {
						log.Printf("[Index] Background index rebuild complete.")
					}
				})
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[ERROR] File system watcher error: %v", err)
		case <-ctx.Done():
			log.Printf("[Index] Indexer shutting down.")
			return
		}
	}
}

//	在后台构建索引，并原子性地替换旧索引
//
// buildPathIndexInBackground 它只负责构建虚拟文件索引。而显示普通文件的责任，完全在于 ReadDir 函数。
func (fs *encvWebDAVFS) buildPathIndexInBackground(ctx context.Context, oldIndexes *pathIndexes) error {
	log.Printf("[Index] Building path index for root: %s", fs.dir)
	newPathMap := make(map[string]string)
	newDirMap := make(map[string][]string)
	newFileInfoMap := make(map[string]os.FileInfo)

	// 【修正】使用 filepath.WalkDir 以获得更好的性能和取消支持
	err := filepath.WalkDir(fs.dir, func(p string, d iofs.DirEntry, err error) error {
		// 在每次迭代前检查 context 是否已取消
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err != nil {
			return err
		}
		// 排除特定目录
		if d.IsDir() {
			dirName := d.Name()
			if fs.excludeDirs[dirName] {
				log.Printf("[Index] Skipping excluded directory: %s", p)
				return filepath.SkipDir // 跳过整个目录，不再深入
			}
			return nil // 是普通目录，继续
		}

		ext := strings.ToLower(filepath.Ext(p))
		if !fs.containerExtensions[ext] {
			// 不是容器文件，直接跳过，不做任何解析
			return nil
		}

		// 尝试解析容器
		index, err := fs.getIndexFromContainerPath(p)
		if err != nil {
			// 【关键修改】解析失败，尝试从旧索引中恢复
			log.Printf("[WARN] Failed to parse container '%s' during rebuild: %v. Attempting to restore from old index.", p, err)

			// 【修正】增加对 oldIndexes 的 nil 检查
			if oldIndexes != nil {
				// 遍历旧索引，查找当前物理路径 p 对应的虚拟路径
				for oldVirtualPath, oldRealPath := range oldIndexes.pathMap {
					if oldRealPath == p {
						// 找到了，从旧索引中恢复所有信息
						log.Printf("[INFO] Restoring entry for '%s' from old index (virtual path: '%s').", p, oldVirtualPath)

						parentDir := path.Dir(oldVirtualPath)
						fileName := path.Base(oldVirtualPath)

						newPathMap[oldVirtualPath] = p
						newDirMap[parentDir] = append(newDirMap[parentDir], fileName)

						if oldFileInfo, ok := oldIndexes.fileInfoMap[oldVirtualPath]; ok {
							newFileInfoMap[oldVirtualPath] = oldFileInfo
						}
						break // 找到后就退出循环
					}
				}
			}
			return nil // 处理完这个文件（无论成功与否），继续下一个
		}

		// 解析成功，正常添加到新索引
		info, err := d.Info()
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(fs.dir, p)
		if err != nil {
			return err
		}

		fullVirtualPath := path.Join(
			fs.webdavPrefix,
			filepath.ToSlash(filepath.Dir(relPath)),
			index.GetOriginalFilename(),
		)
		parentDir := path.Dir(fullVirtualPath)
		fileName := path.Base(fullVirtualPath)

		newPathMap[fullVirtualPath] = p
		newDirMap[parentDir] = append(newDirMap[parentDir], fileName)

		decryptedInfo := &decryptedFileInfo{
			name:         index.GetOriginalFilename(),
			originalName: filepath.Base(p),
			size:         index.GetOriginalFileSize(),
			mode:         0444,
			modTime:      info.ModTime(),
			isDir:        false,
			mimeType:     mime.TypeByExtension(filepath.Ext(index.GetOriginalFilename())),
			etag:         utils.GenETag(info.ModTime(), index.GetOriginalFileSize()),
		}
		newFileInfoMap[fullVirtualPath] = decryptedInfo

		return nil
	})
	if err != nil {
		return err
	}

	// 【关键】原子性地替换整个索引结构
	fs.indexes.Store(&pathIndexes{
		pathMap:     newPathMap,
		dirMap:      newDirMap,
		fileInfoMap: newFileInfoMap,
	})
	return nil
}

// getIndexes 安全地获取当前索引结构
func (fs *encvWebDAVFS) getIndexes() *pathIndexes {
	if idx, ok := fs.indexes.Load().(*pathIndexes); ok {
		return idx
	}
	return &pathIndexes{pathMap: make(map[string]string), dirMap: make(map[string][]string)}
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

// 带缓存的 Index 获取
func (fs *encvWebDAVFS) getIndexFromContainerPathWithCache(fullPath string) (types.Index, error) {
	// 1. 尝试从缓存加载
	if cachedIndex, ok := fs.indexCache.Load(fullPath); ok {
		return cachedIndex.(types.Index), nil
	}

	// 2. 缓存未命中，从文件加载
	index, err := fs.getIndexFromContainerPath(fullPath)
	if err != nil {
		return nil, err
	}

	// 3. 加载成功，存入缓存
	// 可以考虑使用文件的 ModTime 作为缓存有效性判断，但为简单起见，这里不做
	fs.indexCache.Store(fullPath, index)
	return index, nil
}

// 从容器路径获取 Index，封装了新的架构逻辑
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

// statFile 获取文件信息，如果是 ENCV 容器，则返回原始文件信息
// 【关键】这个函数现在被设计为可以安全地处理文件和目录。
func (fs *encvWebDAVFS) statFile(ctx context.Context, fullPath string) (os.FileInfo, error) {
	// 步骤 1: 首先调用 os.Stat 获取路径的基本信息。
	// 这是判断一个路径是文件还是目录的最可靠方法。
	// 如果路径本身不存在或无法访问，os.Stat 会返回错误，我们直接将错误向上传递。
	info, err := os.Stat(fullPath)
	if err != nil {
		log.Printf("[WebDAV-statFile] os.Stat failed for '%s': %v", fullPath, err)
		return nil, err
	}
	// 步骤 2: 检查它是否是一个目录。
	// 如果是目录，我们**直接返回**其信息，并且**绝不**进行任何容器检测。
	// 这是防止将目录误判为容器、从而避免 panic 的核心防线。
	if info.IsDir() {
		log.Printf("[WebDAV-statFile] Path '%s' is a directory, returning its info directly.", fullPath)
		return info, nil
	}

	// 步骤 3: 从这里开始，我们 100% 确定它是一个文件。
	// 现在可以安全地尝试将其作为 ENCV 容器来处理。
	index, err := fs.getIndexFromContainerPathWithCache(fullPath)
	if err != nil {
		// 不是容器或 KVI 损坏，返回原始文件信息
		log.Printf("[WebDAV-statFile] '%s' is not a container or KVI extraction failed, returning original file info. %s", fullPath, err)
		return info, nil
	}
	// 步骤 5: 创建并返回代表解密后文件的虚拟 FileInfo。
	origSize := index.GetOriginalFileSize()
	decryptedInfo := &decryptedFileInfo{
		name:         index.GetOriginalFilename(),
		originalName: filepath.Base(fullPath),
		size:         origSize,
		mode:         0444,
		modTime:      info.ModTime(),
		isDir:        false,
		mimeType:     mime.TypeByExtension(filepath.Ext(index.GetOriginalFilename())),
		etag:         utils.GenETag(info.ModTime(), index.GetOriginalFileSize()),
	}
	// log.Printf("[WebDAV-statFile] Successfully created info for container '%s' (original name: %s)", fullPath, decryptedInfo.Name())
	return decryptedInfo, nil
}

// 【完全重写】openAsContainer 使用新的 v2 架构进行流式解密
func (fs *encvWebDAVFS) openAsContainer(ctx context.Context, fullPath string, cachedInfo os.FileInfo) (goWebdav.File, error) {
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
	var fileInfo *decryptedFileInfo
	if cachedInfo != nil {
		// 类型断言，因为我们确定它就是 *decryptedFileInfo
		fileInfo = cachedInfo.(*decryptedFileInfo)
	} else {
		containerInfo, err := os.Stat(fullPath)
		if err != nil {
			return nil, err
		}
		fileInfo = &decryptedFileInfo{
			name:         index.GetOriginalFilename(),
			originalName: filepath.Base(fullPath),
			size:         index.GetOriginalFileSize(),
			mode:         0444,
			modTime:      containerInfo.ModTime(),
			isDir:        false,
			mimeType:     mime.TypeByExtension(filepath.Ext(index.GetOriginalFilename())),
			etag:         utils.GenETag(containerInfo.ModTime(), index.GetOriginalFileSize()),
		}
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
	// 【修改】使用新的无锁索引获取方法
	indexes := fs.getIndexes()
	realPath, found := indexes.pathMap[name]

	if !found {
		// 确实找不到
		return nil, os.ErrNotExist
	}

	// 【关键修改】直接从缓存中获取 FileInfo，确保与 ReadDir 一致
	if fileInfo, ok := indexes.fileInfoMap[name]; ok {
		log.Printf("[WebDAV-Stat] Found container '%s' for virtual path '%s', returning CACHED info.", realPath, name)
		return fileInfo, nil
	}

	// 【降级处理】如果缓存中也没有（极小概率），则实时计算
	log.Printf("[WebDAV-Stat] Cache miss for '%s', falling back to real-time stat.", name)
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
		} else {
			// 是普通文件，直接返回
			return f, nil
		}
	}

	// 5. 直接打开失败，尝试在索引中查找虚拟文件
	// 【修改】使用新的无锁索引获取方法
	indexes := fs.getIndexes()
	realPath, found := indexes.pathMap[name]

	if !found {
		// 确实找不到
		return nil, os.ErrNotExist
	}

	// 【关键修改】将缓存信息传递给 openAsContainer
	cachedInfo, hasCachedInfo := indexes.fileInfoMap[name]
	if !hasCachedInfo {
		log.Printf("[WebDAV-OpenFile] Cache miss for '%s', openAsContainer will generate its own info.", name)
	} else {
		log.Printf("[WebDAV-OpenFile] Found container '%s' for virtual path '%s', passing CACHED info.", realPath, name)
	}

	// 找到了容器，调用 openAsContainer 来处理它
	return fs.openAsContainer(ctx, realPath, cachedInfo)
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

// ReadDir 方法完全重写，变为高性能且无竞争
func (fs *encvWebDAVFS) ReadDir(ctx context.Context, name string) ([]os.FileInfo, error) {
	// 1. 获取当前索引（这是一个内存快照，非常快）
	indexes := fs.getIndexes()

	// 2. 使用 map 来存储结果，键为文件/目录名，可以自动去重
	fileInfos := make(map[string]os.FileInfo)

	// 3. 从 dirMap 中快速获取虚拟文件/目录列表
	if virtualNames, found := indexes.dirMap[name]; found {
		for _, virtualName := range virtualNames {
			virtualPath := path.Join(name, virtualName)
			// 【关键修改】直接从缓存中获取 FileInfo，不再调用 statFile
			// 这避免了所有磁盘 I/O 和潜在的竞争条件
			if info, ok := indexes.fileInfoMap[virtualPath]; ok {
				fileInfos[virtualName] = info
			} else {
				// 理论上不应发生，但如果发生则记录，表明索引内部不一致
				log.Printf("[WebDAV-ReadDir] Inconsistency: dirMap contains '%s' but fileInfoMap does not.", virtualPath)
			}
		}
	}

	// 4. 添加磁盘上存在但不在索引中的物理文件/目录
	// 这部分逻辑保持不变，用于处理普通文件
	fullPath, err := fs.resolvePath(name)
	if err != nil {
		return nil, err
	}
	entries, _ := os.ReadDir(fullPath)
	for _, entry := range entries {
		entryName := entry.Name()
		// 如果这个条目已经被虚拟文件覆盖了，则跳过
		if _, exists := fileInfos[entryName]; exists {
			continue
		}

		// 直接判断物理文件是否是容器
		entryPath := filepath.Join(fullPath, entryName)
		if _, err := manifest.ExtractKVI_v2(entryPath); err == nil {
			// 是容器，跳过，因为它的虚拟文件已经被处理了
			continue
		}

		// 不是容器，是普通文件或目录，直接添加
		if info, err := entry.Info(); err == nil {
			fileInfos[entryName] = info
		}
	}

	// 5. 将 map 转换为切片返回
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
