package unpacker

import (
	"context"

	"github.com/Soltus/encv-go/internal/container"
)

// GenericUnpacker 处理单文件容器（如图像、文本）
type GenericUnpacker struct {
	// 嵌入 BaseUnpacker 以获得通用功能
	BaseUnpacker
}

func (u *GenericUnpacker) Unpack(ctx context.Context, containerPath, detectedExt string) (*container.PackedData, error) {
	// 直接委托给嵌入的 BaseUnpacker 的方法
	return u.UnpackFromFile(ctx, containerPath, detectedExt)
}
