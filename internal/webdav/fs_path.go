package webdav

import (
	"path"
	"path/filepath"
	"strings"
)

// webdavPathToIndexKey 将 WebDAV Handler 传入的绝对路径，转换为用于索引查找的标准键。
// 例如："/webdav/output/file.txt" -> "output/file.txt"
// 例如："/webdav/" -> "."
func (fs *encvWebDAVFS) webdavPathToIndexKey(webdavPath string) string {
	// 1. 剥离 WebDAV 挂载点前缀
	key := strings.TrimPrefix(webdavPath, fs.webdavPrefix)

	// 2. 剥除剩余部分可能存在的前导斜杠，以匹配索引键格式
	key = strings.TrimPrefix(key, "/")

	// 【关键修复】剥离剩余部分可能存在的末尾斜杠，以兼容不同客户端
	key = strings.TrimSuffix(key, "/")

	// 3. 规范化根目录
	if key == "" {
		return "." // WebDAV根目录映射为 "."
	}

	return key
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
