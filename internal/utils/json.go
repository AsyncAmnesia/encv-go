package utils

import (
	"encoding/json"
	"fmt"
	"log"

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

// extractVersionFromRaw 是一个辅助函数，用于从原始JSON map中提取并验证版本号
func extractVersionFromRaw(raw map[string]json.RawMessage) (int16, error) {
	versionRaw, ok := raw["version"]
	if !ok {
		return 0, fmt.Errorf("KVI is missing 'version' field")
	}

	var version int16
	if err := json.Unmarshal(versionRaw, &version); err != nil {
		return 0, fmt.Errorf("failed to parse 'version' field: %w", err)
	}

	return version, nil
}

// UnmarshalKVI 统一解析 KVI，返回 Index 接口
func UnmarshalKVI(data []byte) (types.Index, error) {
	// 1. 解包到原始 map 以进行灵活的字段检查
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse KVI into raw map: %w", err)
	}

	// 2. 【优化】使用辅助函数检查并验证版本（必须存在）
	version, err := extractVersionFromRaw(raw)
	if err != nil {
		return nil, err // 错误信息已在辅助函数中包装好
	}
	if version != types.KviVersion {
		return nil, fmt.Errorf("unsupported KVI version: %d", version)
	}

	// 3. 检查 'kind' 字段以进行分发
	kindRaw, hasKind := raw["kind"]
	if !hasKind {
		// 4. 【向后兼容】如果没有 'kind' 字段，则假定为旧版的 VideoIndex
		log.Printf("-> [Warning] KVI is missing 'kind' field, attempting to parse as legacy VideoIndex.")
		var index types.VideoIndex
		if err := json.Unmarshal(data, &index); err != nil {
			return nil, fmt.Errorf("failed to parse KVI as legacy VideoIndex: %w", err)
		}
		return &index, nil
	}

	// 5. 如果 'kind' 字段存在，则解析它并分发
	var kind types.IndexKind
	if err := json.Unmarshal(kindRaw, &kind); err != nil {
		return nil, fmt.Errorf("failed to parse 'kind' field: %w", err)
	}

	return unmarshalIndexByKind(data, kind)
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
