package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/types"
)

// Config 是应用程序的顶层配置结构，包含所有子模块的配置。
type Config struct {
	// --- 全局设置 ---
	// Password 用于加密和解密视频文件，请务必设置一个强密码。
	Password string `json:"password"`
	// Recover 在解密时是否尝试覆盖已有文件。
	Recover bool `json:"recover"`

	// --- 加密/解密设置 ---
	// OutputPath 加密后的文件输出目录。
	OutputPath string `json:"output_path"`
	// TrackExtensions 视频容器的字幕/轨道文件扩展名列表，它们并不会打包到容器里。
	TrackExtensions []string `json:"track_extensions"`
	// BinExtGroup 可以自定义加密容器文件的扩展名。
	BinExtGroup types.BinExtGroup `json:"bin_ext_group"`
	// SccgvSettings SCCGV (视频分片) 相关的设置。
	SccgvSettings types.SccgvSettings       `json:"sccgv_settings"`
	Server        types.HttpServer          `json:"server"`
	Webdav        types.WebdavServer        `json:"webdav"`
	Proxy         types.OpenlistProxyServer `json:"proxy"`
}

// contextKey 是一个不导出的类型，用于防止 context 中的 key 冲突。
// 这是一个在 Go 中使用 context.WithValue 的标准做法。
type contextKey string

// configKey 是我们用来存储配置的私有 key。
const configKey = contextKey("encv-go-config")

// NewContext 将配置对象存入一个新的 context 中，并返回这个新 context。
// parent: 父 context，通常是 context.Background() 或请求的 context
// cfg: 要存储的配置对象
func NewContext(parent context.Context, cfg *Config) context.Context {
	return context.WithValue(parent, configKey, cfg)
}

// FromContext 从 context 中提取配置对象。
// 如果 context 中没有存储配置，它会返回 nil。
func FromContext(ctx context.Context) *Config {
	if cfg, ok := ctx.Value(configKey).(*Config); ok {
		return cfg
	}
	return nil
}

// DefaultConfig 返回一个包含所有默认值的配置实例。
func DefaultConfig() *Config {
	return &Config{
		OutputPath:      "./encrypted",
		TrackExtensions: []string{".ass", ".srt", ".dm.ass"},
		BinExtGroup: types.BinExtGroup{
			Text:   "sccgt",
			Image:  "sccgi",
			Audio:  "sccga",
			Video:  "sccgv",
			Iframe: "sccgf",
		},
		SccgvSettings: types.SccgvSettings{
			ChunkSizeMB: 0, // 0 表示不启用分片
		},
		Server: types.HttpServer{Port: 1999, Dir: "./"},
		Webdav: types.WebdavServer{
			Port: 2299,
			Root: "/webdav/",
			Dir:  "./output",
		},
		Proxy: types.OpenlistProxyServer{
			Port:                         2025,
			OpenListHost:                 "http://localhost:5244",
			DisableSignatureVerification: false,
		},
	}
}

// Load 从指定的文件路径加载配置。
// 它会先用默认值初始化，然后用配置文件内容进行覆盖。
// 如果文件不存在，则返回默认配置。
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("-> [Config] Config file '%s' not found, using default settings.\n", configPath)
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", configPath, err)
	}

	// json.Unmarshal 会将文件中的非零值字段覆盖到 cfg 上
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file '%s': %w", configPath, err)
	}

	fmt.Printf("-> [Config] Successfully loaded configuration from '%s'.\n", configPath)
	return cfg, nil
}

// 返回所有已知的容器扩展名（带点号）
func (c *Config) GetAllContainerExtensions() []string {
	return []string{
		"." + c.BinExtGroup.Video,  // .sccgv
		"." + c.BinExtGroup.Text,   // .sccgt
		"." + c.BinExtGroup.Audio,  // .sccga
		"." + c.BinExtGroup.Image,  // .sccgi
		"." + c.BinExtGroup.Iframe, // .sccgf
	}
}

// IsContainerPath 检查路径是否是已知的容器文件（基于扩展名）
// 这是一个快速的、非 I/O 的检查，适用于初步筛选。
func (c *Config) IsContainerPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, containerExt := range c.GetAllContainerExtensions() {
		if ext == containerExt {
			return true
		}
	}
	return false
}

// IsContainerFile 为了向后兼容保留，建议直接使用 IsContainerPath
func (c *Config) IsContainerFile(path string) bool {
	return c.IsContainerPath(path)
}

// --- 全局 MIME 类型映射表，以 OpenList 为准 ---
var ContentTypes = map[string]string{
	// Text
	"txt":        "text/plain; charset=utf-8",
	"htm":        "text/html; charset=utf-8",
	"html":       "text/html; charset=utf-8",
	"xml":        "text/xml; charset=utf-8",
	"java":       "text/x-java-source; charset=utf-8",
	"properties": "text/plain; charset=utf-8",
	"sql":        "text/plain; charset=utf-8",
	"js":         "application/javascript; charset=utf-8",
	"md":         "text/plain; charset=utf-8",
	"json":       "application/json; charset=utf-8",
	"conf":       "text/plain; charset=utf-8",
	"ini":        "text/plain; charset=utf-8",
	"vue":        "text/plain; charset=utf-8",
	"php":        "text/plain; charset=utf-8",
	"py":         "text/x-python; charset=utf-8",
	"bat":        "text/plain; charset=utf-8",
	"gitignore":  "text/plain; charset=utf-8",
	"yml":        "application/x-yaml; charset=utf-8",
	"yaml":       "application/x-yaml; charset=utf-8",
	"go":         "text/plain; charset=utf-8",
	"sh":         "application/x-sh; charset=utf-8",
	"c":          "text/plain; charset=utf-8",
	"cpp":        "text/plain; charset=utf-8",
	"h":          "text/plain; charset=utf-8",
	"hpp":        "text/plain; charset=utf-8",
	"tsx":        "text/plain; charset=utf-8",
	"vtt":        "text/plain; charset=utf-8",
	"srt":        "text/plain; charset=utf-8",
	"ass":        "text/plain; charset=utf-8",
	"rs":         "text/plain; charset=utf-8",
	"lrc":        "text/plain; charset=utf-8",
	"strm":       "text/plain; charset=utf-8",

	// Audio
	"mp3":  "audio/mpeg",
	"flac": "audio/flac",
	"ogg":  "audio/ogg",
	"m4a":  "audio/mp4",
	"wav":  "audio/wav",
	"opus": "audio/opus",
	"wma":  "audio/x-ms-wma",

	// Video
	"mp4":  "video/mp4",
	"mkv":  "video/x-matroska",
	"avi":  "video/x-msvideo",
	"mov":  "video/quicktime",
	"rmvb": "application/vnd.rn-realmedia-vbr",
	"webm": "video/webm",
	"flv":  "video/x-flv",
	"m3u8": "application/vnd.apple.mpegurl",

	// Image
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"tiff": "image/tiff",
	"png":  "image/png",
	"gif":  "image/gif",
	"bmp":  "image/bmp",
	"svg":  "image/svg+xml",
	"ico":  "image/x-icon",
	"swf":  "application/x-shockwave-flash",
	"webp": "image/webp",
	"avif": "image/avif",

	// Iframe
	"doc":  "application/msword",
	"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"xls":  "application/vnd.ms-excel",
	"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"ppt":  "application/vnd.ms-powerpoint",
	"pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	"pdf":  "application/pdf",
	"epub": "application/epub+zip",
}
