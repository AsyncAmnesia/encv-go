package audio

import (
	"fmt"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/physical"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// 实现了 Packer 接口
type AudioPacker struct {
	physicalPacker physical.PhysicalPacker
}

// 并注入物理分片策略
func NewAudioPacker(pp physical.PhysicalPacker) *AudioPacker {
	return &AudioPacker{
		physicalPacker: pp,
	}
}

// Pack 实现 Packer 接口
func (p *AudioPacker) Pack(cfg *config.Config, req *physical.PackRequest) error {
	index, ok := req.Index.(*AudioIndex)
	if !ok {
		return fmt.Errorf("AudioPacker received a non-Audio index")
	}

	// 1. 创建 Manifest
	kvi := AudioKVI_v2{
		KVI_v2: types.KVI_v2{
			SaltBase64: crypto.Base64Encode_v2(req.Salt),
			IVBase64:   crypto.Base64Encode_v2(req.IV),
		},
		AudioIndex: index,
	}
	// 2. 直接使用请求中预先计算好的逻辑分片
	manifest, err := types.NewManifest_v2(kvi, req.LogicalFragments)
	if err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	// 委托给 PhysicalPacker 完成所有打包工作
	_, err = p.physicalPacker.Pack(
		req.EncryptedDataReader,
		manifest, // 传递 manifest
		req,
	)
	if err != nil {
		return fmt.Errorf("physical packing failed: %w", err)
	}
	return nil
}
