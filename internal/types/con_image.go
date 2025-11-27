package types

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
