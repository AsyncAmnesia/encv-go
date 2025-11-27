package types

type IframeIndex struct {
	Kind             IndexKind      `json:"kind"`
	Version          int16          `json:"version"`
	ID               string         `json:"id"`
	OriginalFileSize int64          `json:"original_file_size"`
	MimeType         string         `json:"mime_type"`
	Format           string         `json:"format"` // e.g., "plain", "markdown"
	Encryption       EncryptionInfo `json:"encryption"`
	OriginalFilename string         `json:"original_filename"`
	EncryptedFileMD5 string         `json:"encrypted_file_md5"`
}

// 实现 Index 接口
func (t *IframeIndex) GetKind() IndexKind                { return IndexKindIframe }
func (t *IframeIndex) GetVersion() int16                 { return t.Version }
func (t *IframeIndex) GetEncryptionInfo() EncryptionInfo { return t.Encryption }
func (t *IframeIndex) GetOriginalFilename() string       { return t.OriginalFilename }
func (t *IframeIndex) GetOriginalFileSize() int64        { return t.OriginalFileSize }
func (t *IframeIndex) GetMimeType() string               { return t.MimeType }
