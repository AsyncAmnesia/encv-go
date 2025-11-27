package types

import (
	"encoding/json"
	"fmt"
)

// IndexKind 定义 KVI 的类型
type IndexKind string

const (
	IndexKindVideo IndexKind = "video"
	IndexKindImage IndexKind = "image"
	// kvi version
	KviVersion int16 = 1
)

// Index 是所有 KVI 结构体的通用接口
type Index interface {
	GetKind() IndexKind
	GetVersion() int16
	GetEncryptionInfo() EncryptionInfo
	GetOriginalFilename() string
	GetOriginalFileSize() int64
	GetMimeType() string
}

// SubtitleTrack 表示一个字幕或弹幕轨道
type SubtitleTrack struct {
	Language string `json:"language"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
	Note     string `json:"note,omitempty"`
}

// SubChunkInfo 存储子分片的元数据
type SubChunkInfo struct {
	Index    int    `json:"index"`    // 子分片的序号 (2, 3, 4...)
	Filename string `json:"filename"` // 子分片的文件名
	MD5      string `json:"md5"`      // 子分片内容的 MD5 哈希
}

// VideoIndex 是加密视频的元数据索引文件
type VideoIndex struct {
	Kind             IndexKind       `json:"kind"`
	Version          int16           `json:"version"`
	VideoID          string          `json:"video_id"`
	OriginalFileSize int64           `json:"original_file_size"`
	Format           string          `json:"format"`
	MimeType         string          `json:"mime_type"`
	Encryption       EncryptionInfo  `json:"encryption"`
	SeekTable        []interface{}   `json:"seek_table"` // 留空
	DurationSeconds  float64         `json:"duration_seconds"`
	Resolution       string          `json:"resolution"`
	OriginalFilename string          `json:"original_filename"`
	EncryptedFileMD5 string          `json:"encrypted_file_md5"`
	SubChunks        []SubChunkInfo  `json:"sub_chunks"`
	Subtitles        []SubtitleTrack `json:"subtitles"`
}

func (v *VideoIndex) GetKind() IndexKind                { return IndexKindVideo }
func (v *VideoIndex) GetVersion() int16                 { return v.Version }
func (v *VideoIndex) GetEncryptionInfo() EncryptionInfo { return v.Encryption }
func (v *VideoIndex) GetOriginalFilename() string       { return v.OriginalFilename }
func (v *VideoIndex) GetOriginalFileSize() int64        { return v.OriginalFileSize }
func (v *VideoIndex) GetMimeType() string               { return v.MimeType }

// ImageIndex 是解密图像所需的所有元数据的容器
type ImageIndex struct {
	Kind             IndexKind      `json:"kind"`
	Version          int16          `json:"version"`
	ImageID          string         `json:"image_id"`
	OriginalFileSize int64          `json:"original_file_size"`
	MimeType         string         `json:"mime_type"`
	Format           string         `json:"format"`
	Encryption       EncryptionInfo `json:"encryption"`
	OriginalFilename string         `json:"original_filename"`
	EncryptedFileMD5 string         `json:"encrypted_file_md5"`
	Width            int            `json:"width"`
	Height           int            `json:"height"`
}

// 【新增】实现 Index 接口
func (i *ImageIndex) GetKind() IndexKind                { return IndexKindImage }
func (i *ImageIndex) GetVersion() int16                 { return i.Version }
func (i *ImageIndex) GetEncryptionInfo() EncryptionInfo { return i.Encryption }
func (i *ImageIndex) GetOriginalFilename() string       { return i.OriginalFilename }
func (v *ImageIndex) GetOriginalFileSize() int64        { return v.OriginalFileSize }
func (v *ImageIndex) GetMimeType() string               { return v.MimeType }

// EncryptionInfo 包含加密所需的信息
type EncryptionInfo struct {
	Algorithm  string `json:"algorithm"`
	IVBase64   string `json:"iv_base64"`
	SaltBase64 string `json:"salt_base64"`
}

type BinExtGroup struct {
	Text  string `json:"text"`
	Image string `json:"image"`
	Audio string `json:"audio"`
	Video string `json:"video"`
}

// SccgvSettings 包含 SCCGV 容器的特定设置
type SccgvSettings struct {
	ChunkSizeMB int `json:"chunk_size"`
}

// UserConfig 用户配置文件结构
type UserConfig struct {
	Password        string        `json:"password"`
	OutputPath      string        `json:"outputPath"`
	Port            int           `json:"port"`
	TrackExtensions []string      `json:"trackExtensions"`
	BinExtGroup     BinExtGroup   `json:"bin_ext_group"`
	SccgvSettings   SccgvSettings `json:"sccgv_settings"`
	Recover         bool          `json:"recover" yaml:"recover"` // 是否在解密时强制覆盖已存在的文件
}

// unmarshalIndexByKind 是一个内部辅助函数，假设 'kind' 字段存在且有效
func unmarshalIndexByKind(data []byte, kind IndexKind) (Index, error) {
	switch kind {
	case IndexKindVideo:
		var index VideoIndex
		if err := json.Unmarshal(data, &index); err != nil {
			return nil, fmt.Errorf("failed to parse VideoIndex: %w", err)
		}
		return &index, nil
	case IndexKindImage:
		var index ImageIndex
		if err := json.Unmarshal(data, &index); err != nil {
			return nil, fmt.Errorf("failed to parse ImageIndex: %w", err)
		}
		return &index, nil
	default:
		return nil, fmt.Errorf("unknown KVI kind: %s", kind)
	}
}

// UnmarshalKVI 统一解析 KVI，返回 Index 接口，并支持向后兼容
func UnmarshalKVI(data []byte) (Index, error) {
	// 1. 首先尝试解析包含 'kind' 和 'version' 的头部
	var header struct {
		Kind    IndexKind `json:"kind"`
		Version int16     `json:"version"`
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
	if int16(version) != KviVersion {
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
	var index VideoIndex
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

	if int16(version) != KviVersion {
		return fmt.Errorf("unsupported KVI version: %d", version)
	}

	return nil
}
