package types

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
