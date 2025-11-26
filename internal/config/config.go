package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/types"
)

// GlobalConfig 存储全局加载的配置
var GlobalConfig *types.UserConfig

// 默认配置
var defaultConfig = types.UserConfig{
	OutputPath:      "./encrypted",
	TrackExtensions: []string{".ass", ".srt", ".dm.ass"},
	BinExtGroup: types.BinExtGroup{
		// 其他类型暂时保留原有逻辑的映射，但暂时不用
		Text:  "sccgt",
		Image: "sccgi",
		Audio: "sccga",
		// video 的默认值是 "enc"
		Video: "enc",
	},
}

// LoadUserConfig 加载配置文件，如果文件不存在或部分配置缺失，则使用默认值
func LoadUserConfig() (*types.UserConfig, error) {
	// 1. 初始化为默认配置
	// 使用值拷贝，避免后续修改影响 defaultConfig
	cfg := defaultConfig
	GlobalConfig = &cfg

	// 2. 尝试读取用户配置文件
	configPath := filepath.Join("config.user.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("-> [Config] config.user.json not found, using default settings.")
		return GlobalConfig, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return GlobalConfig, fmt.Errorf("failed to read config.user.json: %w", err)
	}

	// 3. 解析用户配置
	var userConfig types.UserConfig
	if err := json.Unmarshal(data, &userConfig); err != nil {
		return GlobalConfig, fmt.Errorf("failed to parse config.user.json: %w", err)
	}

	// 【关键修复】合并所有需要的配置项（用户配置覆盖默认配置）
	if userConfig.Password != "" {
		GlobalConfig.Password = userConfig.Password
	}
	if userConfig.OutputPath != "" {
		GlobalConfig.OutputPath = userConfig.OutputPath
	}
	if len(userConfig.TrackExtensions) > 0 {
		GlobalConfig.TrackExtensions = userConfig.TrackExtensions
	}
	if userConfig.BinExtGroup.Text != "" {
		GlobalConfig.BinExtGroup.Text = userConfig.BinExtGroup.Text
	}
	if userConfig.BinExtGroup.Image != "" {
		GlobalConfig.BinExtGroup.Image = userConfig.BinExtGroup.Image
	}
	if userConfig.BinExtGroup.Audio != "" {
		GlobalConfig.BinExtGroup.Audio = userConfig.BinExtGroup.Audio
	}
	if userConfig.BinExtGroup.Video != "" {
		GlobalConfig.BinExtGroup.Video = userConfig.BinExtGroup.Video
	}

	fmt.Printf("-> [Config] Loaded user config. Video extension set to '.%s'\n", GlobalConfig.BinExtGroup.Video)
	return GlobalConfig, nil
}

// GetVideoEncExtension 获取当前配置的视频加密后缀（带点号）
func GetVideoEncExtension() string {
	return "." + GlobalConfig.BinExtGroup.Video
}

// 【重构函数】IsContainerFile 检查路径是否是已知的容器文件
// 这个函数现在与 tryGetKVI 的逻辑类似，但更简单，因为它只需要判断，不需要生成
func IsContainerFile(path string) bool {
	return strings.HasSuffix(path, GetVideoEncExtension())
}

// --- 全局 MIME 类型映射表 ---
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
}
