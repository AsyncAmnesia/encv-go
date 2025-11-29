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

type BinExtGroup struct {
	// 文本类加密容器的扩展名。
	Text string `json:"text"`
	// 图像类加密容器的扩展名。
	Image string `json:"image"`
	// 音频类加密容器的扩展名。
	Audio string `json:"audio"`
	// 视频类加密容器的扩展名。
	Video string `json:"video"`
	// OpenList iframe 类加密容器的扩展名。
	Iframe string `json:"iframe"`
}

// SccgvSettings 包含 SCCGV 容器的特定设置
type SccgvSettings struct {
	// 分片大小（单位 MB），为 0 或为空禁用分片
	ChunkSizeMB int `json:"chunk_size"`
}

// --- WebDAV 服务器设置 ---
type WebdavServer struct {
	// encv Webdav 服务器的端口，请不要填写 encv HTTP Server、 OpenList 或其他已使用的端口
	Port int `json:"port"`
	// 路由（例如 /webdav/）
	Root string `json:"root"`
	// 文件系统的根目录（例如 /path/to/your/files），而不是 WebDAV 的路由前缀（例如 /webdav/）
	Dir string `json:"dir"`
}

// --- 内置HTTP服务器 设置 ---
type HttpServer struct {
	// encv HTTP 服务器的端口，请不要填写 encv Webdav Server、 OpenList 或其他已使用的端口
	Port int `json:"port"`
	// 文件系统的根目录（例如 /path/to/your/files），支持相对路径
	Dir string `json:"dir"`
}

// --- Openlist 代理服务器设置 ---
type OpenlistProxyServer struct {
	// Openlist Webdav 代理的端口，请不要填写 encv Webdav Server、 OpenList 或其他已使用的端口
	Port int `json:"port"`
	// OpenList 的主机名，一般是 IP 或者域名，比如 localhost
	OpenListHost string `json:"openlist_host"`
	// 不建议在 json 文件中存储 Token
	Token string `json:"token"`
	// 禁用签名，目前没发现这个值有什么影响
	DisableSignatureVerification bool `json:"disable_signature_verification"`
}

// DecryptedContent 包含解密后的所有内容
type DecryptedContent struct {
	Index      Index
	DataStream io.ReadCloser
}

// EncryptionInfo 包含加密所需的信息
type EncryptionInfo struct {
	Algorithm  string `json:"algorithm"`
	IVBase64   string `json:"iv_base64"`
	SaltBase64 string `json:"salt_base64"`
}
