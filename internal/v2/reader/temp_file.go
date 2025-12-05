package reader

import (
	"fmt"
	"io"
	"os"
)

// TempFileReadCloser 包装了 *os.File，在 Close 时会同时关闭文件并删除底层文件。
// 这是一个通用的工具，适用于任何需要返回一个可自动清理的临时文件读取器的场景。
type TempFileReadCloser struct {
	file *os.File
	path string
}

// NewTempFileReadCloser 是一个构造函数，它打开指定路径的文件并包装成 TempFileReadCloser。
// 如果文件打开失败，它会返回错误。
func NewTempFileReadCloser(filePath string) (io.ReadCloser, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open temp file '%s': %w", filePath, err)
	}
	return &TempFileReadCloser{file: file, path: filePath}, nil
}

// Read 实现了 io.Reader 接口
func (t *TempFileReadCloser) Read(p []byte) (n int, err error) {
	return t.file.Read(p)
}

// Close 实现了 io.Closer 接口。
// 它会关闭文件，并尝试删除文件。删除失败时只会打印警告，不会返回错误，以避免中断主流程。
func (t *TempFileReadCloser) Close() error {
	err := t.file.Close()
	// 尝试删除临时文件，即使失败也不应中断主流程
	if rmErr := os.Remove(t.path); rmErr != nil {
		// 使用 fmt.Printf 以避免在库代码中强制引入日志格式
		fmt.Printf("Warning: failed to remove temp file '%s': %v\n", t.path, rmErr)
	}
	return err
}
