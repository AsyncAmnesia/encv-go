package types

type GenericIndex struct {
	ID               string         `json:"id"`
	OriginalFileSize int64          `json:"original_file_size"`
	MimeType         string         `json:"mime_type"`
	Format           string         `json:"format"` // e.g., "plain", "markdown"
	Encryption       EncryptionInfo `json:"encryption"`
	OriginalFilename string         `json:"original_filename"`
	OriginalFileMD5  string         `json:"original_file_md5"`
	EncryptedFileMD5 string         `json:"encrypted_file_md5"`
}

// 实现 Index 接口
func (t *GenericIndex) GetEncryptionInfo() EncryptionInfo { return t.Encryption }
func (t *GenericIndex) GetOriginalFilename() string       { return t.OriginalFilename }
func (t *GenericIndex) GetOriginalFileSize() int64        { return t.OriginalFileSize }
func (v *GenericIndex) GetOriginalFileMD5() string        { return v.OriginalFileMD5 }
func (v *GenericIndex) GetEncryptedFileMD5() string       { return v.EncryptedFileMD5 }
func (t *GenericIndex) GetMimeType() string               { return t.MimeType }
func (i *GenericIndex) UpdateCommonInfo(encInfo EncryptionInfo, originalFilename, encryptedFileMD5 string) {
	i.Encryption = encInfo
	i.OriginalFilename = originalFilename
	i.EncryptedFileMD5 = encryptedFileMD5
}

// VideoKVI_v2 是视频容器专用的 KVI
type GenericKVI_v2 struct {
	KVI_v2
	GenericIndex *GenericIndex `json:"video_index"`
}

func (v GenericKVI_v2) GetKind() IndexKind {
	return IndexKindGeneric
}

// 【关键新增】实现 KVIProvider 接口
func (v GenericKVI_v2) GetEncryptionInfo() KVI_v2 {
	return v.KVI_v2
}

func (v GenericKVI_v2) GetIndex() Index {
	return v.GenericIndex
}
