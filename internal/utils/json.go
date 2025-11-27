package utils

import (
	"encoding/json"
	"fmt"

	"github.com/Soltus/encv-go/internal/types"
)

// unmarshalIndexByKind 是一个内部辅助函数，假设 'kind' 字段存在且有效
func unmarshalIndexByKind(data []byte, kind types.IndexKind) (types.Index, error) {
	switch kind {
	case types.IndexKindVideo:
		var index types.VideoIndex
		if err := json.Unmarshal(data, &index); err != nil {
			return nil, fmt.Errorf("failed to parse VideoIndex: %w", err)
		}
		return &index, nil
	case types.IndexKindImage:
		var index types.ImageIndex
		if err := json.Unmarshal(data, &index); err != nil {
			return nil, fmt.Errorf("failed to parse ImageIndex: %w", err)
		}
		return &index, nil
	case types.IndexKindText: // 【新增】处理 TextIndex
		var index types.TextIndex
		if err := json.Unmarshal(data, &index); err != nil {
			return nil, fmt.Errorf("failed to parse TextIndex: %w", err)
		}
		return &index, nil
	default:
		return nil, fmt.Errorf("unknown KVI kind: %s", kind)
	}
}

// UnmarshalKVI 统一解析 KVI，返回 Index 接口，并支持向后兼容
func UnmarshalKVI(data []byte) (types.Index, error) {
	// 1. 首先尝试解析包含 'kind' 和 'version' 的头部
	var header struct {
		Kind    types.IndexKind `json:"kind"`
		Version int16           `json:"version"`
	}

	// 使用 json.RawMessage 来避免在 'kind' 不存在时完全失败
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse KVI into raw map: %w", err)
	}

	// 检查并解析 'version' 字段 (这是必须的)
	versionRaw, ok := raw["version"]
	if !ok {
		return nil, fmt.Errorf("KVI is missing 'version' field")
	}
	var versionNum json.Number
	if err := json.Unmarshal(versionRaw, &versionNum); err != nil {
		return nil, fmt.Errorf("failed to parse 'version' as a number: %w", err)
	}
	version, err := versionNum.Int64()
	if err != nil {
		return nil, fmt.Errorf("failed to convert 'version' to int64: %w", err)
	}
	if int16(version) != types.KviVersion {
		return nil, fmt.Errorf("unsupported KVI version: %d", version)
	}

	// 2. 尝试解析 'kind' 字段
	kindRaw, hasKind := raw["kind"]
	if hasKind {
		if err := json.Unmarshal(kindRaw, &header.Kind); err != nil {
			return nil, fmt.Errorf("failed to parse 'kind' field: %w", err)
		}
		// 如果 'kind' 字段存在，就使用它来分发
		return unmarshalIndexByKind(data, header.Kind)
	}

	// 3. 【向后兼容】如果没有 'kind' 字段，则假定为旧版的 VideoIndex
	fmt.Println("-> [Warning] KVI is missing 'kind' field, attempting to parse as legacy VideoIndex.")
	var index types.VideoIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse KVI as legacy VideoIndex: %w", err)
	}
	return &index, nil
}

// 检查 KVI 数据的版本是否受支持
func CheckKviVersion(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to parse KVI for version check: %w", err)
	}

	versionRaw, ok := raw["version"]
	if !ok {
		return fmt.Errorf("KVI is missing 'version' field")
	}

	var versionNum json.Number
	if err := json.Unmarshal(versionRaw, &versionNum); err != nil {
		return fmt.Errorf("failed to parse 'version' as a number: %w", err)
	}

	version, err := versionNum.Int64()
	if err != nil {
		return fmt.Errorf("failed to convert 'version' to int64: %w", err)
	}

	if int16(version) != types.KviVersion {
		return fmt.Errorf("unsupported KVI version: %d", version)
	}

	return nil
}
