package types

// TextIndex 是解密文本所需的所有元数据的容器
type TextIndex struct {
	Kind             IndexKind      `json:"kind"`
	Version          int16          `json:"version"`
	TextID           string         `json:"text_id"`
	OriginalFileSize int64          `json:"original_file_size"`
	MimeType         string         `json:"mime_type"`
	Format           string         `json:"format"` // e.g., "plain", "markdown"
	Encryption       EncryptionInfo `json:"encryption"`
	OriginalFilename string         `json:"original_filename"`
	EncryptedFileMD5 string         `json:"encrypted_file_md5"`
}

// 【新增】实现 Index 接口
func (t *TextIndex) GetKind() IndexKind                { return IndexKindText }
func (t *TextIndex) GetVersion() int16                 { return t.Version }
func (t *TextIndex) GetEncryptionInfo() EncryptionInfo { return t.Encryption }
func (t *TextIndex) GetOriginalFilename() string       { return t.OriginalFilename }
func (t *TextIndex) GetOriginalFileSize() int64        { return t.OriginalFileSize }
func (t *TextIndex) GetMimeType() string               { return t.MimeType }
