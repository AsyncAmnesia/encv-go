// pkg/encv/kvi.go

package encv

import (
	"os"

	"github.com/Soltus/encv-go/internal/container"
)

// ExtractKVI 从给定的 ENCV 容器文件中提取原始的 KVI 数据。
func ExtractKVI(containerPath string) ([]byte, error) {
	// 1. 打开容器文件
	containerFile, err := os.Open(containerPath)
	if err != nil {
		return nil, err
	}
	defer containerFile.Close()

	// 2. 解包
	packedData, err := container.Unpack(containerFile)
	if err != nil {
		return nil, err
	}
	// 我们只需要 KVI 数据，所以关闭视频流以释放资源
	defer packedData.VideoStream.Close()

	// 3. 返回 KVI 数据
	return packedData.KVIData, nil
}
