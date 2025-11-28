package types

import "io"

// IndexKind 定义 KVI 的类型
type IndexKind string

const (
	IndexKindVideo  IndexKind = "video"
	IndexKindImage  IndexKind = "image"
	IndexKindText   IndexKind = "text"
	IndexKindIframe IndexKind = "iframe"
	// kvi version
	KviVersion int16 = 1
)

// FFProbeRawMetadata 用于直接解析 ffprobe 的 JSON 输出
type FFProbeRawMetadata struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"streams"`
}

// Index 是所有 KVI 结构体的通用接口
type Index interface {
	GetKind() IndexKind
	GetVersion() int16
	GetEncryptionInfo() EncryptionInfo
	GetOriginalFilename() string
	GetOriginalFileSize() int64
	GetMimeType() string
}

// EncryptionInfo 包含加密所需的信息
type EncryptionInfo struct {
	Algorithm  string `json:"algorithm"`
	IVBase64   string `json:"iv_base64"`
	SaltBase64 string `json:"salt_base64"`
}

type BinExtGroup struct {
	Text   string `json:"text"`
	Image  string `json:"image"`
	Audio  string `json:"audio"`
	Video  string `json:"video"`
	Iframe string `json:"iframe"`
}

// SccgvSettings 包含 SCCGV 容器的特定设置
type SccgvSettings struct {
	ChunkSizeMB int `json:"chunk_size"`
}

// UserConfig 用户配置文件结构
type UserConfig struct {
	Password        string        `json:"password"`
	OutputPath      string        `json:"outputPath"`
	Port            int           `json:"port"`
	TrackExtensions []string      `json:"trackExtensions"`
	BinExtGroup     BinExtGroup   `json:"bin_ext_group"`
	SccgvSettings   SccgvSettings `json:"sccgv_settings"`
	Recover         bool          `json:"recover" yaml:"recover"` // 是否在解密时强制覆盖已存在的文件
	Webdav          WebdavServer  `json:"webdav"`
}

// DecryptedContent 包含解密后的所有内容
type DecryptedContent struct {
	Index      Index
	DataStream io.ReadCloser
}

type WebdavServer struct {
	Port int    `json:"port"`
	Root string `json:"root"` // 路由（例如 /webdav/）
	Dir  string `json:"dir"`  // 件系统的根目录（例如 /path/to/your/files），而不是 WebDAV 的路由前缀（例如 /webdav/）
}
