package packer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/types"
)

// GenericPacker 处理单文件容器（如图像、文本）的打包
type GenericPacker struct {
	BasePacker
}

func (p *GenericPacker) Pack(ctx context.Context, baseName, outputDir string, encryptedDataReader io.Reader, index types.Index) error {
	cfg := config.FromContext(ctx)

	// 1. 决策：通过 switch 将 IndexKind 映射到二进制扩展名
	var binExt string
	switch index.GetKind() {
	case types.IndexKindImage:
		binExt = cfg.BinExtGroup.Image
	case types.IndexKindText:
		binExt = cfg.BinExtGroup.Text
	default:
		return fmt.Errorf("GenericPacker received an unsupported IndexKind: %s", index.GetKind())
	}

	finalPath := filepath.Join(outputDir, baseName+"."+binExt)

	// 2. 决策：获取魔法数字
	magicMap, err := container.GetContainerMagicMap(ctx)
	if err != nil {
		return fmt.Errorf("failed to get magic map: %w", err)
	}
	magic, ok := magicMap[binExt]
	if !ok {
		return fmt.Errorf("no magic number found for extension: %s", binExt)
	}

	// 3. 准备：序列化 KVI
	kviData, err := json.Marshal(index)
	if err != nil {
		return fmt.Errorf("failed to marshal KVI: %w", err)
	}

	// 4. 委托：调用基类的工具方法完成打包
	if err := p.WriteSingleFileContainer(finalPath, []byte(magic), kviData, encryptedDataReader); err != nil {
		return fmt.Errorf("failed to pack container: %w", err)
	}

	fmt.Printf("-> Successfully packed container: %s\n", finalPath)
	return nil
}
