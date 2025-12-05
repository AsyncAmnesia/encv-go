package types

// VideoIndex 是加密视频的元数据索引文件
type VideoIndex struct {
	VideoID          string         `json:"video_id"`
	Format           string         `json:"format"`
	MimeType         string         `json:"mime_type"`
	Encryption       EncryptionInfo `json:"encryption"`
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
	SubtitleTracks    []SubtitleTracks `json:"subtitle_tracks,omitempty"`
}

func (v *VideoIndex) GetEncryptionInfo() EncryptionInfo { return v.Encryption }
func (v *VideoIndex) GetOriginalFilename() string       { return v.OriginalFilename }
func (v *VideoIndex) GetOriginalFileSize() int64        { return v.OriginalFileSize }
func (v *VideoIndex) GetOriginalFileMD5() string        { return v.OriginalFileMD5 }
func (v *VideoIndex) GetEncryptedFileMD5() string       { return v.EncryptedFileMD5 }
func (v *VideoIndex) GetMimeType() string               { return v.MimeType }
func (i *VideoIndex) UpdateCommonInfo(encInfo EncryptionInfo, originalFilename, encryptedFileMD5 string) {
	i.Encryption = encInfo
	i.OriginalFilename = originalFilename
	i.EncryptedFileMD5 = encryptedFileMD5
}

// SubtitleTrack 表示一个字幕或弹幕轨道
type SubtitleTracks struct {
	Language string `json:"language"`
	FileSize string `json:"file_size"`
	// Filename 是需要恢复成的原始文件名 (e.g., "myvideo.ass")
	Filename string `json:"filename"`
	// Title 是加密后处理的字幕文件名，字幕本身不加密 (e.g., "myvideo.4pm.ass")
	Title string `json:"title"`
	Note  string `json:"note,omitempty"`
}

// VideoKVI_v2 是视频容器专用的 KVI
type VideoKVI_v2 struct {
	KVI_v2
	VideoIndex *VideoIndex `json:"video_index"`
}

func (v VideoKVI_v2) GetKind() IndexKind {
	return IndexKindVideo
}

// 【关键新增】实现 KVIProvider 接口
func (v VideoKVI_v2) GetEncryptionInfo() KVI_v2 {
	return v.KVI_v2
}

func (v VideoKVI_v2) GetIndex() Index {
	return v.VideoIndex
}
