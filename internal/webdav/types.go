package webdav

import (
	"bytes"
	"os"
	"sync"
	"time"
)

// encvWebDAVFS 是一个自定义的 webdav.FileSystem
// 它拦截文件请求，如果文件是 ENCV 容器，则提供解密后的流
type encvWebDAVFS struct {
	// dir 是 WebDAV 服务的根目录
	dir string
	// password 用于解密内容
	password string
	// 【新增】WebDAV 的 URL 前缀 (例如 "/webdav/")
	webdavPrefix string
	// 【新增】内存缓存，用于存储已解密的小文件内容
	// key: 文件绝对路径, value: 解密后的字节数据
	decryptionCache map[string][]byte

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
	// io.ReadCloser
	// 【关键修改】直接嵌入 *bytes.Reader，它自带 Read 和 Seek
	*bytes.Reader
	info *decryptedFileInfo
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
