package unpacker

// import (
// 	"context"
// 	"fmt"

// 	"github.com/Soltus/encv-go/internal/container"
// 	"github.com/Soltus/encv-go/internal/container/chunked"
// )

// // UnpackSeekableFromFile 是一个辅助方法，用于从文件路径创建可寻址的 PackedData。
// func (b *BaseChunkedUnpacker) UnpackSeekableFromFile(ctx context.Context, containerPath, binExt string) (*container.PackedData, error) {
// 	magicMap, err := container.GetContainerMagicMap(ctx)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get magic map: %w", err)
// 	}
// 	subMagicMap, err := container.GetSubChunkMagicMap(ctx)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get sub-magic map: %w", err)
// 	}

// 	mainMagic, ok := magicMap[binExt]
// 	if !ok {
// 		return nil, fmt.Errorf("internal error: extension '%s' not in magic map", binExt)
// 	}
// 	subMagic, ok := subMagicMap[binExt]
// 	if !ok {
// 		return nil, fmt.Errorf("internal error: extension '%s' not in sub-magic map", binExt)
// 	}

// 	packedData, err := chunked.CreateSeekablePackedData(containerPath, mainMagic, subMagic)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create seekable packed data for %s: %w", containerPath, err)
// 	}

// 	return packedData, nil
// }

// // UnpackSeekable 提供了可寻址的解包能力
// func (u *VideoUnpacker) UnpackSeekable(ctx context.Context, containerPath, detectedExt string) (*container.PackedData, error) {
// 	return u.UnpackSeekableFromFile(ctx, containerPath, detectedExt)
// }
