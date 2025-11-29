package postdecrypt

import (
	"context"
	"fmt"

	"github.com/Soltus/encv-go/internal/types"
)

type TextPostDecrypter struct{}

func (p *TextPostDecrypter) PostDecrypt(ctx context.Context, content *types.DecryptedContent, containerPath, outputDir string) error {
	_, ok := content.Index.(*types.TextIndex)
	if !ok {
		return fmt.Errorf("internal error: TextPostDecrypter called with non-TextIndex type")
	}

	// 未来可以在这里添加文本特有的后处理逻辑
	// 例如：转换编码、格式化等
	return nil
}
