package types

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

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
	GetOriginalFileMD5() string
	GetEncryptedFileMD5() string
	GetMimeType() string // 重要方法，实现错误会影响前端预览
	// 一个统一的接口，用于更新加密后产生的通用元数据
	UpdateCommonInfo(encInfo EncryptionInfo, originalFilename, encryptedFileMD5 string)
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
	ChunkSizeMB int64 `json:"chunk_size"`
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
	case "AfterPack":
		allErrors = append(allErrors, validateAfterPack(index)...)
	default:
		allErrors = append(allErrors, fmt.Sprintf("unknown validation context: %s", context))
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

// validateAfterPack 验证打包阶段的数据完整性
func validateAfterPack(index Index) []string {
	var errs []string
	// 通用检查：继承加密后的所有检查
	errs = append(errs, validateAfterEncrypt(index)...)

	// 【上下文特定】打包后，对于分块容器，应有分块信息
	switch v := index.(type) {
	case *VideoIndex:
		if len(v.SubChunks) == 0 {
			errs = append(errs, "VideoIndex.SubChunks is empty after packing")
		}
		// 其他类型如果也有分块，在此添加检查...
	}
	return errs
}
