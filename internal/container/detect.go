// internal/container/detect.go

package container

import (
	"bytes"
	"errors"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/crypto"
)

// getContainerMagicMap 动态构建容器扩展名到魔法数字的映射
// 这是实现配置驱动的关键
func getContainerMagicMap() map[string]string {
	// 从 GlobalConfig 读取用户定义的后缀
	return map[string]string{
		config.GlobalConfig.BinExtGroup.Video: crypto.SccgvMainChunkMagic,
		config.GlobalConfig.BinExtGroup.Text:  crypto.SccgtContainerMagicNumber,
		config.GlobalConfig.BinExtGroup.Audio: crypto.SccgaContainerMagicNumber,
		config.GlobalConfig.BinExtGroup.Image: crypto.SccgiContainerMagicNumber,
	}
}

// DetectContainerType 从文件头字节切片中检测容器类型
// 它返回的是容器类型的**扩展名**（例如 "sccgv"），而不是一个硬编码的常量
func DetectContainerType(header []byte) (string, error) {
	magicMap := getContainerMagicMap()
	for ext, magic := range magicMap {
		if bytes.HasPrefix(header, []byte(magic)) {
			// 找到匹配项，返回用户配置的扩展名
			return ext, nil
		}
	}

	return "", errors.New("unknown or unsupported container type")
}
