package video

import (
	"fmt"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/namer"
	"github.com/Soltus/encv-go/internal/v2/physical"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// VideoPacker 实现了 Packer 接口
type VideoPacker struct {
	physicalPacker physical.PhysicalPacker
}

// NewVideoPacker 创建一个新的 VideoPacker，并注入物理分片策略
func NewVideoPacker(pp physical.PhysicalPacker, namer namer.ChunkNamer) *VideoPacker {
	return &VideoPacker{
		physicalPacker: pp,
	}
}

// Pack 实现 Packer 接口
func (p *VideoPacker) Pack(cfg *config.Config, req *physical.PackRequest) error {
	vIndex, ok := req.Index.(*VideoIndex)
	if !ok {
		return fmt.Errorf("VideoPacker received a non-video index")
	}

	// 1. 创建 Manifest
	videoKVI := VideoKVI_v2{
		KVI_v2: types.KVI_v2{
			SaltBase64: crypto.Base64Encode_v2(req.Salt),
			IVBase64:   crypto.Base64Encode_v2(req.IV),
		},
		VideoIndex: vIndex,
	}
	// 2. 直接使用请求中预先计算好的逻辑分片
	manifest, err := types.NewManifest_v2(videoKVI, req.LogicalFragments)
	if err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	// 委托给 PhysicalPacker 完成所有打包工作
	mainChunkPath, err := p.physicalPacker.Pack(
		req.EncryptedDataReader,
		manifest, // 传递 manifest
		req,
	)
	if err != nil {
		return fmt.Errorf("physical packing failed: %w", err)
	}

	fmt.Printf("✅ [PACKER] Packed to: %s\n", mainChunkPath)
	return nil
}
