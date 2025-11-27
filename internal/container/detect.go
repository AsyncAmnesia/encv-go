// internal/container/detect.go

package container

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Soltus/encv-go/internal/config"
)

// getContainerMagicMap 动态构建容器扩展名到魔法数字的映射
// 这是获取所有容器魔法数字的公共入口
func GetContainerMagicMap() map[string]string {
	// 从 GlobalConfig 读取用户定义的后缀，并映射到固定的魔法数字
	// 魔法数字本身是格式规范的一部分，不应由用户配置
	return map[string]string{
		config.GlobalConfig.BinExtGroup.Video:  "encv-sccgv-chunk-main-v1", // SCCGV 主分片
		config.GlobalConfig.BinExtGroup.Text:   "encv-sccgt-container-v1",
		config.GlobalConfig.BinExtGroup.Audio:  "encv-sccga-container-v1",
		config.GlobalConfig.BinExtGroup.Image:  "encv-sccgi-container-v1",
		config.GlobalConfig.BinExtGroup.Iframe: "encv-sccgf-container-v1",
	}
}

// GetSubChunkMagicMap 获取子分片的魔法数字映射
func GetSubChunkMagicMap() map[string]string {
	// 目前只有视频有子分片
	return map[string]string{
		config.GlobalConfig.BinExtGroup.Video: "encv-sccgv-chunk-sub-v1", // SCCGV 子分片
	}
}

// DetectContainerType 从文件头字节切片中检测容器类型
func DetectContainerType(header []byte) (string, error) {
	magicMap := GetContainerMagicMap()
	for ext, magic := range magicMap {
		if bytes.HasPrefix(header, []byte(magic)) {
			return ext, nil
		}
	}

	return "", errors.New("unknown or unsupported container type")
}

// DetectContainerTypeFromFile 从文件路径中检测容器类型
// 【新增函数】封装了打开文件、读取头部和检测类型的逻辑
func DetectContainerTypeFromFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for detection: %w", err)
	}
	defer file.Close()

	// 计算需要读取的最大头部大小
	magicMap := GetContainerMagicMap()
	maxMagicLen := 0
	for _, magic := range magicMap {
		if len(magic) > maxMagicLen {
			maxMagicLen = len(magic)
		}
	}

	// 读取文件头部
	header := make([]byte, maxMagicLen)
	bytesRead, err := io.ReadFull(file, header)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", fmt.Errorf("failed to read header for detection: %w", err)
	}
	// 确保只传递实际读取的字节
	header = header[:bytesRead]

	// 调用现有的 DetectContainerType 函数进行检测
	return DetectContainerType(header)
}
