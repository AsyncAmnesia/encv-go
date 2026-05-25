package webdav

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/utils"
)

// webdavPathToIndexKey 将 WebDAV Handler 传入的绝对路径，转换为用于索引查找的标准键。
// 例如："/webdav/output/file.txt" -> "output/file.txt"
// 例如："/webdav/" -> "."
func (fs *encvWebDAVFS) webdavPathToIndexKey(webdavPath string) (string, error) {
	if strings.HasPrefix(webdavPath, fs.webdavPrefix) {
		key := strings.TrimPrefix(webdavPath, fs.webdavPrefix)
		key = strings.TrimPrefix(key, "/")
		key = strings.TrimSuffix(key, "/")
		if key == "" {
			return ".", nil
		}
		return key, nil
	}

	trimmed := strings.TrimSuffix(fs.webdavPrefix, "/")
	if webdavPath == trimmed {
		return ".", nil
	}

	return "", fmt.Errorf("path '%s' is not under webdav root '%s'", webdavPath, fs.webdavPrefix)
}

// physicalPathToIndexKey 将物理容器路径和解析出的虚拟文件名，组合成标准的索引键。
// 例如：physicalPath="A:\...\output\video.sccgv", virtualFilename="video.mp4" -> "output/video.mp4"
func (fs *encvWebDAVFS) physicalPathToIndexKey(physicalPath, virtualFilename string) (string, error) {
	// 1. 计算物理文件相对于服务根目录的相对路径
	relPath, err := filepath.Rel(fs.dir, physicalPath)
	if err != nil {
		return "", err
	}

	// 2. 获取该相对路径的目录部分
	virtualDir := filepath.ToSlash(filepath.Dir(relPath))

	// 3. 使用 path.Join 组合成最终的索引键
	// path.Join 会处理斜杠，确保格式正确
	return path.Join(virtualDir, virtualFilename), nil
}

// resolvePath 将 WebDAV 路径安全地解析为本地文件系统绝对路径
func (fs *encvWebDAVFS) resolvePath(name string) (string, error) {
	if !strings.HasPrefix(name, fs.webdavPrefix) {
		trimmed := strings.TrimSuffix(fs.webdavPrefix, "/")
		if name == trimmed {
			return fs.dir, nil
		}
		return "", fmt.Errorf("path '%s' is not under webdav root '%s'", name, fs.webdavPrefix)
	}

	userPath := strings.TrimPrefix(name, fs.webdavPrefix)
	if userPath == "" {
		userPath = "."
	}

	return utils.SafeResolveToAbsPath(fs.dir, userPath)
}
