package types

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
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
	// OpenList iframe 类加密容器的扩展名。
	Iframe string `json:"iframe"`
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

// ValidateIndex 对 Index 进行通用和特定类型的验证，并收集所有发现的问题。
// 如果验证失败，它会返回一个包含所有错误信息和完整 Index JSON 内容的聚合错误。
// `context` 参数用于标识验证发生的阶段（如 "After Processor"）。
func ValidateIndex(index Index, context string) error {
	var allErrors []string

	// 0. 检查 index 本身是否为 nil
	if index == nil {
		return fmt.Errorf("[%s] validation failed: received a nil index", context)
	}

	// 1. 【核心】根据上下文分发验证逻辑
	switch context {
	case "AfterProcessor":
		allErrors = append(allErrors, validateAfterProcessor(index)...)
	case "AfterEncrypt":
		allErrors = append(allErrors, validateAfterEncrypt(index)...)
	}

	// 2. 如果收集到了任何错误，则生成报告
	if len(allErrors) > 0 {
		indexJSON, _ := json.MarshalIndent(index, "", "  ") // 忽略序列化错误，专注于验证错误
		errorMessage := fmt.Sprintf(
			"[%s] validation failed with %d error(s):\n--- Errors ---\n%s\n\n--- Index Dump ---\n%s",
			context,
			len(allErrors),
			strings.Join(allErrors, "\n"),
			string(indexJSON),
		)
		return fmt.Errorf(errorMessage)
	}

	return nil
}

// --- 上下文特定的验证函数 ---

// validateAfterProcessor 验证预处理阶段的数据完整性
func validateAfterProcessor(index Index) []string {
	var errs []string
	// 通用检查：所有文件在处理后就应有原始信息
	if index.GetOriginalFilename() == "" {
		errs = append(errs, "OriginalFilename is empty after processing")
	}
	if index.GetOriginalFileMD5() == "" {
		errs = append(errs, "OriginalFileMD5 is empty after processing")
	}
	if index.GetOriginalFileSize() <= 0 {
		errs = append(errs, "OriginalFileSize is invalid after processing")
	}

	// 特定类型检查
	switch v := index.(type) {
	case *VideoIndex:
		if v.Width <= 0 {
			errs = append(errs, "VideoIndex.Width must be > 0 after processing")
		}
		if v.Height <= 0 {
			errs = append(errs, "VideoIndex.Height must be > 0 after processing")
		}
		// 在此添加 ImageIndex, TextIndex 等的特定检查...
	}
	return errs
}

// validateAfterEncrypt 验证加密阶段的数据完整性
func validateAfterEncrypt(index Index) []string {
	var errs []string
	// 通用检查：再次确认原始信息
	errs = append(errs, validateAfterProcessor(index)...)

	// 【上下文特定】加密后，必须有加密后的文件MD5
	if index.GetEncryptedFileMD5() == "" {
		errs = append(errs, "EncryptedFileMD5 is empty after encryption")
	}
	return errs
}
