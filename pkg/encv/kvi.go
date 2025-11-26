// pkg/encv/kvi.go

package encv

import (
	"github.com/Soltus/encv-go/internal/container"
	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/types"
)

// ExtractKVI 从给定的 ENCV 容器文件（或其任意分片）中提取原始的 KVI 数据。
func ExtractKVI(anyChunkPath string) ([]byte, error) {
	// 1. 【关键修改】根据任意一个分片，找到主分片
	mainChunkPath, err := container.FindMainChunk(anyChunkPath)
	if err != nil {
		return nil, err
	}

	// 2. 【关键修改】使用新的分片解包函数
	packedData, err := container.UnpackChunked(mainChunkPath)
	if err != nil {
		return nil, err
	}
	defer packedData.VideoStream.Close()

	// 3. 返回 KVI 数据
	return packedData.KVIData, nil
}

// UnmarshalKVI 根据版本号智能地解析 KVI 数据。
func UnmarshalKVI(data []byte) (*types.VideoIndex, error) {
	return crypto.UnmarshalKVI(data)
}
