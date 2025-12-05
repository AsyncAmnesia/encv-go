package types

type IframeIndex struct {
	ID                string `json:"id"`
	OriginalFileSize  int64  `json:"original_file_size"`
	MimeType          string `json:"mime_type"`
	Format            string `json:"format"` // e.g., "plain", "markdown"
	OriginalFilename  string `json:"original_filename"`
	OriginalInputPath string `json:"originalInputPath"`
	OriginalFileMD5   string `json:"original_file_md5"`
	EncryptedFileMD5  string `json:"encrypted_file_md5"`
}

// 实现 Index 接口
func (t *IframeIndex) GetOriginalFilename() string { return t.OriginalFilename }
func (t *IframeIndex) GetOriginalFileSize() int64  { return t.OriginalFileSize }
func (v *IframeIndex) GetOriginalFileMD5() string  { return v.OriginalFileMD5 }
func (v *IframeIndex) GetEncryptedFileMD5() string { return v.EncryptedFileMD5 }
func (t *IframeIndex) GetMimeType() string         { return t.MimeType }
