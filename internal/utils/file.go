package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Soltus/encv-go/internal/container"
)

// 获取文件大小
func GetFileSize(path string) int64 {
	info, _ := os.Stat(path)
	if info == nil {
		return 0
	}
	return info.Size()
}

// 复制文件
func CopyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// 按长度降序排序扩展名
func SortExtensionsByLength(exts []string) []string {
	sorted := make([]string, len(exts))
	copy(sorted, exts)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})
	return sorted
}

// 剥离已知的扩展名
func StripKnownExtensions(filename string, exts []string) string {
	name := filename
	for _, ext := range exts {
		if strings.HasSuffix(name, ext) {
			name = strings.TrimSuffix(name, ext)
			break
		}
	}
	return name
}

// GenerateReversedExt 将文件扩展名倒序，例如 "jpg" -> "gpj"
func GenerateReversedExt(ext string) string {
	// 移除可能存在的前导点，例如 ".jpg" -> "jpg"
	if len(ext) > 0 && ext[0] == '.' {
		ext = ext[1:]
	}
	runes := []rune(ext)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// IsEncryptedContainer 通过读取文件头来检测是否为有效的 ENCV 容器
func IsEncryptedContainer(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		// 如果文件无法打开，我们不认为它是容器，但返回错误以便上层记录
		return false, fmt.Errorf("failed to open file '%s' for container detection: %w", filePath, err)
	}
	defer file.Close()

	// 读取头部用于类型检测
	magicMap := container.GetContainerMagicMap()
	maxMagicLen := 0
	for _, magic := range magicMap {
		if len(magic) > maxMagicLen {
			maxMagicLen = len(magic)
		}
	}

	magicHeader := make([]byte, maxMagicLen)
	bytesRead, err := io.ReadFull(file, magicHeader)
	if err != nil && err != io.ErrUnexpectedEOF {
		// 读取失败，不是有效容器
		return false, fmt.Errorf("failed to read header of '%s': %w", filePath, err)
	}

	// DetectContainerType 在成功时返回扩展名，失败时返回错误
	_, err = container.DetectContainerType(magicHeader[:bytesRead])
	if err != nil {
		// 魔法数字不匹配，不是我们的容器
		return false, nil // 返回 false，但 error 为 nil，表示“检测完毕，不是容器”
	}

	// 没有错误，说明是有效的容器
	return true, nil
}

// CreateFileForOutput 安全地创建一个用于输出的文件。
// 如果 force 为 true，它将覆盖已存在的文件。
// 如果 force 为 false，它将创建一个新文件，如果文件名冲突，则自动重命名为 (1), (2)...
// 它返回打开的文件句柄和实际使用的文件路径。
func CreateFileForOutput(path string, force bool) (*os.File, string, error) {
	// 如果强制覆盖，直接尝试创建（这会覆盖同名文件）
	if force {
		file, err := os.Create(path)
		if err != nil {
			return nil, "", fmt.Errorf("failed to force create file '%s': %w", path, err)
		}
		return file, path, nil
	}

	// --- 非强制模式 ---

	// 1. 尝试以独占模式创建文件（如果文件已存在，则会失败）
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if err == nil {
		// 创建成功，文件不存在
		return file, path, nil
	}

	// 2. 如果失败是因为文件已存在，则开始重命名逻辑
	if !os.IsExist(err) {
		// 是其他错误（如权限不足）
		return nil, "", fmt.Errorf("failed to create file '%s': %w", path, err)
	}

	// 3. 分解路径，准备生成新名称
	dir := filepath.Dir(path)
	ext := filepath.Ext(path)
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, ext)

	// 4. 循环查找可用的文件名
	for i := 1; ; i++ {
		newName := fmt.Sprintf("%s(%d)%s", name, i, ext)
		newPath := filepath.Join(dir, newName)

		file, err := os.OpenFile(newPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
		if err == nil {
			// 找到一个可用的名字，创建成功
			return file, newPath, nil
		}

		if !os.IsExist(err) {
			// 遇到其他错误，停止尝试
			return nil, "", fmt.Errorf("failed to create file '%s': %w", newPath, err)
		}
		// 如果是文件已存在，继续循环 i++
	}
}
