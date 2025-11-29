package postdecrypt

import (
	"context"
	"fmt"

	"github.com/Soltus/encv-go/internal/types"
)

type ImagePostDecrypter struct{}

func (p *ImagePostDecrypter) PostDecrypt(ctx context.Context, content *types.DecryptedContent, containerPath, outputDir string) error {
	_, ok := content.Index.(*types.ImageIndex)
	if !ok {
		return fmt.Errorf("internal error: ImagePostDecrypter called with non-ImageIndex type")
	}

	// 未来可以在这里添加图像特有的后处理逻辑
	// 例如：生成缩略图、写入 EXIF 等
	return nil
}
