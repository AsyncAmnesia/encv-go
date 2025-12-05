// internal/crypto/seekable_ctr.go

package crypto

import (
	"crypto/aes"
	"encoding/binary"
)

// CalculateIVForPosition 根据初始 IV 和目标位置，计算出 CTR 模式下该位置应有的 IV。
// 这是加密和解密两端共享的核心逻辑。
// initialIV: 加密开始时使用的 IV。
// position: 目标字节位置。
// 返回: 目标位置对应的 IV。
func CalculateIVForPosition(initialIV []byte, position int64) []byte {
	blockSize := int64(aes.BlockSize)
	blockIndex := position / blockSize // 现在是 int64 / int64，类型匹配

	// 【核心共享逻辑】采用 "Nonce + Counter" 模型，并使用 LittleEndian 字节序
	// 这与加密端写入文件头的习惯保持一致。
	newIV := make([]byte, aes.BlockSize)
	copy(newIV, initialIV)

	counter := binary.LittleEndian.Uint64(newIV[8:])
	binary.LittleEndian.PutUint64(newIV[8:], counter+uint64(blockIndex))

	return newIV
}
