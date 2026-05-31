package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"

	"github.com/Soltus/encv-go/internal/v2/types"
)

// Config 是应用程序的顶层配置结构，包含所有子模块的配置。
type Config struct {
	// --- 全局设置 ---
	// Password 用于加密和解密视频文件，请务必设置一个强密码。
	Password string `json:"password"`
	// Recover 在解密时是否尝试覆盖已有文件。
	Recover bool `json:"recover"`
	// DefaultContainerVersion 默认容器版本（2=已弃用, 3=稳定, 4=推荐）
	DefaultContainerVersion int `json:"default_container_version"`
	// StrictDeprecatedVersion 是否严格禁止使用已弃用版本创建容器
	StrictDeprecatedVersion bool `json:"strict_deprecated_version"`

	// --- 加密/解密设置 ---
	// OutputPath 加密后的文件输出目录。
	OutputPath string `json:"output_path"`
	// TrackExtensions 视频容器的字幕/轨道文件扩展名列表，它们并不会打包到容器里。
	// TrackExtensions []string `json:"track_extensions"`
	// BinExtGroup 可以自定义加密容器文件的扩展名。已弃用，使用 PluginSettings. 替代。
	// BinExtGroup types.BinExtGroup `json:"bin_ext_group"`
	//  map 键是插件名，值是该插件的原始JSON配置
	PluginSettings map[string]json.RawMessage `json:"plugin_settings"`
	// 【关键新增】Provider 是一个可选的动态配置提供者
	// 如果 Provider 不为 nil，它将优先于 PluginSettings 被使用
	Provider ConfigProvider            `json:"-"` // 使用 `json:"-"` 确保它不会被序列化到文件
	Server   types.HttpServer          `json:"server"`
	Admin    types.AdminServer         `json:"admin"`
	Webdav   types.WebdavServer        `json:"webdav"`
	Proxy    types.OpenlistProxyServer `json:"proxy"`
	// --- 日志设置 ---
	// Log 配置结构化日志的输出级别和文件路径。
	Log types.LogConfig `json:"log"`
	// --- 预览设置 ---
	Preview *PreviewConfig `json:"preview,omitempty"`
	Mobile  *types.MobileConfig  `json:"mobile,omitempty"`
}

type PreviewConfig struct {
	TextExtensions []string `json:"text_extensions,omitempty"`
}

// ConfigProvider 定义了获取插件配置的抽象接口
// 任何实现了此接口的结构体都可以为 ENCV 插件提供配置
type ConfigProvider interface {
	// GetPluginSettings 根据插件名称获取其原始的 JSON 配置
	GetPluginSettings(pluginName string) (json.RawMessage, error)
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
		OutputPath:             "./encrypted",
		DefaultContainerVersion: 4,
		Server:     types.HttpServer{Port: 1999, Dir: "./"},
		Webdav: types.WebdavServer{
			Root: "",
			Dir:  "",
		},
		Proxy: types.OpenlistProxyServer{
			DisableSignatureVerification: false,
		},
		Log: types.LogConfig{
			Level: "info",
			File:  "",
		},
	}
}

func (c *Config) GetEffectiveDefaultVersion() int {
	if c.DefaultContainerVersion > 0 && types.IsValidVersion(c.DefaultContainerVersion) {
		return c.DefaultContainerVersion
	}
	return types.DefaultContainerVersion
}

func (c *Config) IsStrictMode() bool {
	return c.StrictDeprecatedVersion
}

// Load 加载配置。优先级（低→高）：
//   DefaultConfig() → config.user.json → config.dev.json（dev 最高优先级）
// 显式指定路径时走单文件模式（向后兼容）
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	if configPath != "" {
		return loadSingleFile(cfg, configPath)
	}

	candidates := findMergeCandidates()
	if candidates == nil {
		slog.Info("No config files found, using default settings")
		return finalize(cfg), nil
	}

	if candidates.User != "" {
		cfg = loadAndMerge(cfg, candidates.User)
	}
	if candidates.Dev != "" {
		cfg = loadAndMerge(cfg, candidates.Dev)
	}

	return finalize(cfg), nil
}

// ApplyMobileOverrides 在 ENCV_MOBILE=1 环境下将 mobile 段的配置合并到顶层字段。
// 供 config.Load() finalize() 和 API handler 共用，确保 GET /api/config 返回的也是生效后的值。
// 触发方式：
//   - 真机 Android 端：EncvGoService.kt 启动 Go 进程时设置 ENCV_MOBILE=1
//   - 移动端 dev 预览：通过 scripts/dev-mobile.sh 或手动 export ENCV_MOBILE=1 启动后端
func ApplyMobileOverrides(cfg *Config) {
	home := os.Getenv("HOME")
	if cfg.Mobile.ServerDir != "" {
		cfg.Server.Dir = cfg.Mobile.ServerDir
	} else if cfg.Server.Dir == "/" || cfg.Server.Dir == "." {
		cfg.Server.Dir = home
	}
	if cfg.Mobile.OutputPath != "" {
		cfg.OutputPath = cfg.Mobile.OutputPath
	} else if cfg.OutputPath == "" || cfg.OutputPath == "./encrypted" {
		cfg.OutputPath = filepath.Join(home, "encv-output")
	}
	if cfg.Mobile.WebdavDir != "" {
		cfg.Webdav.Dir = cfg.Mobile.WebdavDir
	}
}

func mergeConfig(base, overlay *Config) *Config {
	if overlay == nil {
		return base
	}
	baseData, err := json.Marshal(base)
	if err != nil {
		return base
	}
	overlayData, err := json.Marshal(overlay)
	if err != nil {
		return base
	}
	var baseMap, overlayMap map[string]interface{}
	if json.Unmarshal(baseData, &baseMap) != nil || json.Unmarshal(overlayData, &overlayMap) != nil {
		return base
	}
	deepMerge(baseMap, overlayMap)
	resultData, _ := json.Marshal(baseMap)
	var result Config
	if json.Unmarshal(resultData, &result) != nil {
		return base
	}
	result.Provider = base.Provider
	return &result
}

func deepMerge(base, overlay map[string]interface{}) {
	for k, ov := range overlay {
		if ov == nil {
			continue
		}
		bv, ok := base[k]
		if !ok {
			base[k] = ov
			continue
		}
		bm, bo := bv.(map[string]interface{})
		om, oo := ov.(map[string]interface{})
		if bo && oo {
			deepMerge(bm, om)
		} else if !isZeroValue(ov) {
			base[k] = ov
		}
	}
}

func isZeroValue(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	case reflect.String:
		return rv.String() == ""
	case reflect.Bool:
		return !rv.Bool()
	default:
		return false
	}
}

type mergeCandidates struct {
	Dev  string
	User string
}

func findMergeCandidates() *mergeCandidates {
	dirs := searchDirs()
	var c mergeCandidates
	for _, dir := range dirs {
		if c.Dev == "" && exists(filepath.Join(dir, "config.dev.json")) {
			c.Dev = filepath.Join(dir, "config.dev.json")
		}
		if c.User == "" && exists(filepath.Join(dir, "config.user.json")) {
			c.User = filepath.Join(dir, "config.user.json")
		}
		if c.Dev != "" && c.User != "" {
			break
		}
	}
	if c.Dev == "" && c.User == "" {
		return nil
	}
	return &c
}

func loadAndMerge(base *Config, path string) *Config {
	data, err := os.ReadFile(path)
	if err != nil {
		return base
	}
	var overlay Config
	if json.Unmarshal(data, &overlay) != nil {
		return base
	}
	slog.Info("Merged config file", "path", path)
	return mergeConfig(base, &overlay)
}

func loadSingleFile(cfg *Config, path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		slog.Info("Config file not found, using default settings", "path", path)
		return finalize(cfg), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", path, err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file '%s': %w", path, err)
	}
	return finalize(cfg), nil
}

func finalize(cfg *Config) *Config {
	if cfg.Server.Dir == "/" {
		cfg.Server.Dir, _ = os.Getwd()
	}
	if os.Getenv("ENCV_MOBILE") == "1" && cfg.Mobile != nil {
		ApplyMobileOverrides(cfg)
	}
	slog.Info("Configuration loaded", "log_level", cfg.Log.Level)
	return cfg
}

func searchDirs() []string {
	var dirs []string
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, wd)
	}
	if exePath, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exePath))
	}
	return dirs
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// GetPluginSettingsFor 是一个泛型辅助函数，用于安全地获取并解析特定插件的配置。
// T 是插件配置结构体的类型，例如 VideoPluginConfig。
// 它会从 map 中查找插件配置，并将其反序列化为 T 类型的指针。
func GetPluginSettingsFor[T any](cfg *Config, pluginName string) (*T, error) {
	var rawSettings json.RawMessage
	var err error

	// 1. 优先检查是否有动态 Provider
	if cfg.Provider != nil {
		rawSettings, err = cfg.Provider.GetPluginSettings(pluginName)
		if err != nil {
			return nil, fmt.Errorf("failed to get settings for plugin '%s' from provider: %w", pluginName, err)
		}
	} else {
		// 2. 如果没有 Provider，则使用传统的 PluginSettings map
		result, ok := cfg.PluginSettings[pluginName]
		if !ok {
			// 在没有 Provider 的情况下，如果 map 中没有，说明用户确实没配置
			return nil, fmt.Errorf("no settings found for plugin '%s'", pluginName)
		}
		rawSettings = result
	}

	// 3. 使用统一的辅助函数解析配置
	return UnmarshalPluginSettings[T](rawSettings, pluginName)
}

// UnmarshalPluginSettings 是一个通用的辅助函数，用于将原始 JSON 解析为具体的插件配置结构体
// 它不依赖于任何全局的 config 对象
func UnmarshalPluginSettings[T any](rawSettings json.RawMessage, pluginName string) (*T, error) {
	var settings T
	if len(rawSettings) == 0 {
		// 如果没有提供配置，返回零值
		return &settings, nil
	}
	if err := json.Unmarshal(rawSettings, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

// 智能地查找 config.user.json 文件
func FindConfigPath(flagPath string) (string, error) {
	// 1. 最高优先级：命令行标志指定的路径
	if flagPath != "" {
		if _, err := os.Stat(flagPath); err == nil {
			slog.Info("Using config from command-line flag", "path", flagPath)
			return flagPath, nil
		}
		return "", fmt.Errorf("config file specified by flag not found: %s", flagPath)
	}

	// 2. 次高优先级：环境变量
	if envPath := os.Getenv("ENCV_CONFIG_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			slog.Info("Using config from environment variable ENCV_CONFIG_PATH", "path", envPath)
			return envPath, nil
		}
		return "", fmt.Errorf("config file from environment variable not found: %s", envPath)
	}

	// 3. 再次优先级：当前工作目录
	// 这完美适配了 `go run ./cmd/encv start` 的场景
	wd, err := os.Getwd()
	if err == nil {
		wdConfigPath := filepath.Join(wd, "config.user.json")
		if _, err := os.Stat(wdConfigPath); err == nil {
			slog.Info("Using config from current working directory", "path", wdConfigPath)
			return wdConfigPath, nil
		}
	}

	// 4. 最低优先级：可执行文件所在目录
	// 这适配了生产环境，将配置文件和二进制文件放在一起
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		exeConfigPath := filepath.Join(exeDir, "config.user.json")
		if _, err := os.Stat(exeConfigPath); err == nil {
			slog.Info("Using config from executable directory", "path", exeConfigPath)
			return exeConfigPath, nil
		}
	}

	return "", fmt.Errorf("config.user.json not found in any of the standard locations (cwd, exe dir, env var, or flag)")
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

// GetTextPreviewExtensions 返回所有 MIME 类型含 "text/" 的扩展名列表
func GetTextPreviewExtensions() []string {
	var exts []string
	for ext, mime := range ContentTypes {
		if len(mime) >= 5 && mime[:5] == "text/" {
			exts = append(exts, ext)
		}
	}
	return exts
}
