package image

import (
	"fmt"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/physical"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// 实现了 Packer 接口
type ImagePacker struct {
	physicalPacker physical.PhysicalPacker
}

// NewImagePacker 创建一个新的 ImagePacker，并注入物理分片策略
func NewImagePacker(pp physical.PhysicalPacker) *ImagePacker {
	return &ImagePacker{
		physicalPacker: pp,
	}
}

// Pack 实现 Packer 接口
func (p *ImagePacker) Pack(cfg *config.Config, req *physical.PackRequest) error {
	iIndex, ok := req.Index.(*types.ImageIndex)
	if !ok {
		return fmt.Errorf("ImagePacker received a non-image index")
	}

	// 1. 创建 Manifest
	imageKVI := types.ImageKVI_v2{
		KVI_v2: types.KVI_v2{
			SaltBase64: crypto.Base64Encode_v2(req.Salt),
			IVBase64:   crypto.Base64Encode_v2(req.IV),
		},
		ImageIndex: iIndex,
	}
	// 2. 直接使用请求中预先计算好的逻辑分片
	manifest, err := types.NewManifest_v2(imageKVI, req.LogicalFragments)
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
