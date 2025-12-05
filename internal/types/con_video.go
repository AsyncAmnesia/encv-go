package types

// VideoIndex 是加密视频的元数据索引文件
type VideoIndex struct {
	Kind             IndexKind      `json:"kind"`
	Version          int16          `json:"version"`
	VideoID          string         `json:"video_id"`
	Format           string         `json:"format"`
	MimeType         string         `json:"mime_type"`
	Encryption       EncryptionInfo `json:"encryption"`
	SeekTable        []interface{}  `json:"seek_table"` // 留空
	DurationSeconds  float64        `json:"duration_seconds"`
	Resolution       string         `json:"resolution"`
	Width            int            `json:"width"`
	Height           int            `json:"height"`
	OriginalFileSize int64          `json:"original_file_size"`
	// 原始视频文件的完整路径，用于 Packer 查找关联文件（如字幕）
	OriginalInputPath string           `json:"originalInputPath"`
	OriginalFilename  string           `json:"original_filename"`
	OriginalFileMD5   string           `json:"original_file_md5"`
	EncryptedFileMD5  string           `json:"encrypted_file_md5"`
	SubChunks         []SubChunkInfo   `json:"sub_chunks"`
	SubtitleTracks    []SubtitleTracks `json:"subtitle_tracks,omitempty"`
}

func (v *VideoIndex) GetKind() IndexKind                { return IndexKindVideo }
func (v *VideoIndex) GetVersion() int16                 { return v.Version }
func (v *VideoIndex) GetEncryptionInfo() EncryptionInfo { return v.Encryption }
func (v *VideoIndex) GetOriginalFilename() string       { return v.OriginalFilename }
func (v *VideoIndex) GetOriginalFileSize() int64        { return v.OriginalFileSize }
func (v *VideoIndex) GetOriginalFileMD5() string        { return v.OriginalFileMD5 }
func (v *VideoIndex) GetEncryptedFileMD5() string       { return v.EncryptedFileMD5 }
func (v *VideoIndex) GetMimeType() string               { return v.MimeType }
func (v *VideoIndex) GetSubChunks() []SubChunkInfo      { return v.SubChunks }
func (v *VideoIndex) HasSubChunks() bool {
	return len(v.SubChunks) > 0
}
func (i *VideoIndex) UpdateCommonInfo(encInfo EncryptionInfo, originalFilename, encryptedFileMD5 string) {
	i.Encryption = encInfo
	i.OriginalFilename = originalFilename
	i.EncryptedFileMD5 = encryptedFileMD5
}

// SubtitleTrack 表示一个字幕或弹幕轨道
type SubtitleTracks struct {
	Language string `json:"language"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
	Note     string `json:"note,omitempty"`
}
