package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/Soltus/encv-go/internal/container/chunked"
)

// debugSubChunkHeader 读取并打印子分片文件的头部信息
func debugSubChunkHeader(filePath, expectedMagic string) error {
	fmt.Printf("--- Debugging Sub-Chunk Header for: %s ---\n", filePath)

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// 调用我们修复过的 ReadSubHeader 函数
	header, err := chunked.ReadSubHeader(file, expectedMagic)
	if err != nil {
		return fmt.Errorf("failed to read sub-chunk header: %w", err)
	}

	// 打印整个头部结构体
	fmt.Printf("Successfully parsed header:\n")
	fmt.Printf("%+v\n", header)

	// 为了更深入的分析，我们也可以打印原始字节
	fmt.Printf("\n--- Raw Header Bytes (first %d bytes) ---\n", binary.Size(header))
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek to start: %w", err)
	}
	rawBytes := make([]byte, binary.Size(header))
	_, err = io.ReadFull(file, rawBytes)
	if err != nil {
		return fmt.Errorf("failed to read raw bytes: %w", err)
	}
	fmt.Printf("%x\n", rawBytes)

	return nil
}
