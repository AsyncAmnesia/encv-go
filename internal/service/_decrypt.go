// internal/service/decrypt.go
// 底层工具层：提供与容器、加密等底层交互的纯函数，无状态，可复用。

package service

import (
	"context"
	"fmt"
	"io"

	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/container/chunked"
	"github.com/Soltus/encv-go/internal/types"
)

// DecryptContainer 从一个加密容器文件路径中解密内容，返回解密后的数据流和索引。
// 此函数现在复用与 open-stream 完全相同的健壮解密逻辑。
func DecryptContainer(ctx context.Context, containerPath string) (*types.DecryptedContent, error) {
	// 【核心改动】直接调用我们统一的、经过验证的解密逻辑
	// 它返回一个可寻址的解密流、索引和原始大小
	decryptedStream, index, _, err := NewSeekableDecryptReaderFromContainer(ctx, containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create unified decrypt reader for %s: %w", containerPath, err)
	}

	// 返回结果，decryptedStream 本身就实现了 io.ReadCloser
	return &types.DecryptedContent{
		Index:      index,
		DataStream: decryptedStream,
	}, nil
}

func ExtractKVI(ctx context.Context, containerPath string) ([]byte, error) {
	// 1. 检测容器类型
	detectedExt, err := container.DetectMainOrSubContainerType(ctx, containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect container type for %s: %w", containerPath, err)
	}

	// 2. 获取主容器的魔法数字
	magicMap, err := container.GetContainerMagicMap(ctx)
	if err != nil {
		return nil, err
	}
	mainMagic := magicMap[detectedExt]

	// 3. 【核心改动】使用我们导出的、健壮的 GetKVIDataOnly 函数
	kviData, err := chunked.GetKVIDataOnly(containerPath, mainMagic)
	if err != nil {
		return nil, fmt.Errorf("failed to get KVI data: %w", err)
	}

	return kviData, nil
}

// newReadCloser 组合一个 io.Reader 和一个 Close 函数，创建一个 io.ReadCloser。
func newReadCloser(r io.Reader, closeFunc func() error) io.ReadCloser {
	return &readCloser{Reader: r, closeFunc: closeFunc}
}

// readCloser 是一个简单的 io.ReadCloser 实现。
type readCloser struct {
	io.Reader
	closeFunc func() error
}

func (rc *readCloser) Close() error {
	return rc.closeFunc()
}
