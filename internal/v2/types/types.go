package types

import (
	"io"
)

// ServiceStatus 定义服务状态的类型
type ServiceStatus string

var ServiceStatuses = struct {
	OK    ServiceStatus
	Error ServiceStatus
}{
	OK:    "ok",
	Error: "error",
}

// PingResponse 是 /ping 端点返回的 JSON 结构
type PingResponse struct {
	Status        ServiceStatus `json:"status"`
	Version       string        `json:"version"`     // 应用版本号
	InstanceID    string        `json:"instance_id"` // 本次启动的唯一实例ID
	ServerDirPath string        `json:"server_dir"`  // 主服务映射的本地绝对路径
	WebdavDirPath string        `json:"webdav_dir"`  // WebDAV 服务映射的本地绝对路径 (如果启用)
}

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

// SubChunkInfo 存储子分片的元数据
type SubChunkInfo struct {
	Index    int    `json:"index"`    // 子分片的序号 (2, 3, 4...)
	Filename string `json:"filename"` // 子分片的文件名
	Size     int64  `json:"size"`     // 子分片大小
	MD5      string `json:"md5"`      // 子分片内容的 MD5 哈希
	// 【新增字段】记录该子分片在完整加密文件中的起始字节偏移量
	Offset int64 `json:"offset"`
}

// 所有扩展名都不带 .
type BinExtGroup struct {
	// 文本类加密容器的扩展名。
	Text string `json:"text"`
	// 图像类加密容器的扩展名。
	Image string `json:"image"`
	// 音频类加密容器的扩展名。
	Audio string `json:"audio"`
	// 视频类加密容器的扩展名。
	Video string `json:"video"`
	//word ppt excel 类加密容器的扩展名。
	WPS string `json:"wps"`
	// pdf 类加密容器的扩展名。
	PDF string `json:"pdf"`
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

// --- 管理后台服务器 设置 ---
type AdminServer struct {
	// encv 管理后台服务器的端口，请不要填写 encv Webdav Server、 OpenList 或其他已使用的端口
	Port int `json:"port"`
	// 管理员密码，留空则禁用登录
	Password string `json:"password"`
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
