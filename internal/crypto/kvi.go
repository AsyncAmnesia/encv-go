package crypto

import (
	"encoding/json"
	"fmt"

	"github.com/Soltus/encv-go/internal/types"
)

// UnmarshalKVI 根据版本号智能地解析 KVI 数据。
func UnmarshalKVI(data []byte) (*types.VideoIndex, error) {
	// 1. 将 JSON 解析到一个通用的 map 中
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse KVI into raw map: %w", err)
	}

	// 2. 检查并提取 'version' 字段
	versionRaw, ok := raw["version"]
	if !ok {
		return nil, fmt.Errorf("KVI is missing 'version' field")
	}

	// JSON 数字默认解析为 float64，我们使用 json.Number 以避免精度问题
	var versionNum json.Number
	if err := json.Unmarshal(versionRaw, &versionNum); err != nil {
		return nil, fmt.Errorf("failed to parse 'version' as a number: %w", err)
	}

	version, err := versionNum.Int64()
	if err != nil {
		return nil, fmt.Errorf("failed to convert 'version' to int64: %w", err)
	}

	// 3. 根据版本号进行分发解析
	switch int16(version) {
	case KviVersion:
		// 对于当前版本，直接解析到 VideoIndex 结构体
		var index types.VideoIndex
		if err := json.Unmarshal(data, &index); err != nil {
			return nil, fmt.Errorf("failed to parse KVI for version %d: %w", version, err)
		}
		return &index, nil

	default:
		return nil, fmt.Errorf("unsupported KVI version: %d", version)
	}
}
