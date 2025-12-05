package iframe

import (
	"github.com/Soltus/encv-go/internal/v2/types"
)

const IndexKindIframe types.IndexKind = "iframe"

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

// 视频容器专用的 KVI
type IframeKVI_v2 struct {
	types.KVI_v2
	TextIndex *IframeIndex `json:"Text_index"`
}

func (v IframeKVI_v2) GetKind() types.IndexKind {
	return IndexKindIframe
}

// 【关键新增】实现 KVIProvider 接口
func (v IframeKVI_v2) GetEncryptionInfo() types.KVI_v2 {
	return v.KVI_v2
}

func (v IframeKVI_v2) GetIndex() types.Index {
	return v.TextIndex
}
