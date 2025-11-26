package utils

import (
	"io"
	"os"
	"sort"
	"strings"
)

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
