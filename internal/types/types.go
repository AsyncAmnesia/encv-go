package types

// SubtitleTrack 表示一个字幕或弹幕轨道
type SubtitleTrack struct {
	Language string `json:"language"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
}

// VideoIndex 是加密视频的元数据索引文件
type VideoIndex struct {
	VideoID          string          `json:"video_id"`
	OriginalFileSize int64           `json:"original_file_size"`
	Format           string          `json:"format"`
	Encryption       EncryptionInfo  `json:"encryption"`
	SeekTable        []interface{}   `json:"seek_table"` // 留空
	DurationSeconds  float64         `json:"duration_seconds"`
	Resolution       string          `json:"resolution"`
	OriginalFilename string          `json:"original_filename"`
	Subtitles        []SubtitleTrack `json:"subtitles"`
}

// EncryptionInfo 包含加密所需的信息
type EncryptionInfo struct {
	Algorithm  string `json:"algorithm"`
	IVBase64   string `json:"iv_base64"`
	SaltBase64 string `json:"salt_base64"`
}

// UserConfig 用户配置文件结构
type UserConfig struct {
	Password        string   `json:"password"`
	OutputPath      string   `json:"outputPath"`
	Port            int      `json:"port"`
	TrackExtensions []string `json:"trackExtensions"`
}
