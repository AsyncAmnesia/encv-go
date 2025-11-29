package unpacker

import (
	"context"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/container"
)

// VideoUnpacker 实现 Unpacker 接口，用于处理视频容器
type VideoUnpacker struct {
	BaseChunkedUnpacker
}

func (u *VideoUnpacker) Unpack(ctx context.Context, containerPath string, _ string) (*container.PackedData, error) {
	// 对于 VideoUnpacker，类型是已知的，直接从配置获取
	cfg := config.FromContext(ctx)
	videoExt := cfg.BinExtGroup.Video

	// 委托给嵌入的 BaseChunkedUnpacker 的方法
	return u.UnpackChunkedFromFile(ctx, containerPath, videoExt)
}
