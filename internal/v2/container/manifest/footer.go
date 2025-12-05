// internal/v2/container/manifest/footer.go

package manifest

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/Soltus/encv-go/internal/v2/types"
)

// ReadFooterFromFile 尝试从文件末尾读取并返回 Footer
// 如果 Footer 无效（如 Magic Number 错误），它会返回一个详细的错误
func ReadFooterFromFile(filePath string) (*types.EnvelopeFooter_v2, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// 获取文件大小
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := stat.Size()

	// Footer 是固定大小的
	footerSize := int64(binary.Size(types.EnvelopeFooter_v2{}))
	if fileSize < footerSize {
		return nil, fmt.Errorf("file is too small to contain a footer")
	}

	// Seek 到 Footer 的起始位置
	_, err = file.Seek(fileSize-footerSize, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to footer: %w", err)
	}

	// 读取 Footer
	var footer types.EnvelopeFooter_v2
	if err := binary.Read(file, types.ByteOrder_v2, &footer); err != nil {
		return nil, fmt.Errorf("failed to read footer: %w", err)
	}

	// 验证 Magic Number
	if footer.Magic != types.MagicFooter_v2 {
		return nil, fmt.Errorf("invalid magic number in footer: got '%s', want 'ENVC'", string(footer.Magic[:]))
	}

	return &footer, nil
}
