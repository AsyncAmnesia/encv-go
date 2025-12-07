// internal/v2/plugins/registry.go

package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/namer"
	"github.com/Soltus/encv-go/internal/v2/plugins/audio"
	"github.com/Soltus/encv-go/internal/v2/plugins/image"
	pluginInterfaces "github.com/Soltus/encv-go/internal/v2/plugins/interfaces"
	"github.com/Soltus/encv-go/internal/v2/plugins/pdf"
	"github.com/Soltus/encv-go/internal/v2/plugins/text"
	"github.com/Soltus/encv-go/internal/v2/plugins/video"
	"github.com/Soltus/encv-go/internal/v2/plugins/wps"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// Plugins 是所有已注册插件的列表，顺序代表优先级
var Plugins = []Plugin{
	&video.VideoPlugin{},
	&audio.AudioPlugin{},
	&image.ImagePlugin{},
	&wps.WPSPlugin{},
	&pdf.PDFPlugin{},
	&text.TextPlugin{},
}

// Plugin 定义了加解密插件的完整接口
type Plugin interface {
	// 【新增】插件必须实现此方法，返回其唯一的名称标识符
	// 这个名称将作为在配置文件中查找其配置的键。
	Name() string
	// 【新增】插件必须实现此方法，返回其默认配置的 JSON 字节流
	GetDefaultSettings() json.RawMessage
	// 【新增】返回插件配置结构体的零值实例，用于生成 JSON Schema
	GetSettingsSchemaType() interface{}

	//  返回此插件创建的容器文件扩展名，包含点前缀
	GetContainerExtension() string

	Intialize(ctx context.Context) error
	// --- 提供处理策略 ---
	GetMetadataExtractor() pluginInterfaces.MetadataExtractor
	GetContentPreprocessor() pluginInterfaces.ContentPreprocessor

	// --- 文件识别与处理 ---
	// GetChunkNamer 返回插件用于其容器的 ChunkNamer。
	GetChunkNamer() namer.ChunkNamer

	//  返回此插件支持的 MIME 类型前缀列表，用于匹配，优先级最高
	SupportedMimePrefixes() []string
	//  返回此插件支持的文件扩展名列表（不含前缀点），用于匹配，优先级次之
	SupportedExtensions() []string
	// 用于排除不想处理的文件，返回 false 表示不处理
	ShouldProcess(inputPath string) bool

	// --- 加密逻辑 ---

	// 加密预处理器
	PreEncryptProcessor(index types.Index, inputPath, inputRootDir, outputDir string) error
	// 将处理后的文件加密到指定容器
	Encrypt(dataReader io.Reader) error
	// 加密后处理器
	PostEncryptProcessor() error
	// Packer 打包器 TODO

	// --- 解密逻辑 ---

	//  判断此插件是否能解密给定的容器文件
	CanDecrypt(containerPath string) bool
	// Unpacker 解包器 TODO
	// 解密预处理器
	PreDecryptProcessor(containerPath, outputDir string) error
	// Decrypt 解密容器文件到指定目录
	Decrypt(containerPath, outputDir string) error
	// 解密后处理器
	PostDecryptProcessor(containerPath string) error
}

// normalizeExtension 确保扩展名带有前导点，使其符合标准格式
func normalizeExtension(ext string) string {
	if !strings.HasPrefix(ext, ".") {
		return "." + ext
	}
	return ext
}

// --- 延迟初始化相关变量 ---
var (
	once              sync.Once
	registeredExtsMap map[string]bool // 存储带点扩展名，用于 O(1) 查找
	registeredExts    []string        // 存储带点扩展名，用于列表返回
)

// 在 InitializePlugins 之后调用
func initializeExtensions() {
	registeredExtsMap = make(map[string]bool)
	tempMap := make(map[string]bool)

	// 此时，我们假设所有插件都已经被 Initialize 过了
	for _, p := range Plugins {
		ext := p.GetContainerExtension()
		if ext != "" {
			// 规范化为带点的格式
			normalizedExt := normalizeExtension(ext)
			tempMap[normalizedExt] = true
		}
	}

	// 将最终结果存入缓存
	registeredExts = make([]string, 0, len(tempMap))
	for ext := range tempMap {
		registeredExtsMap[ext] = true
		registeredExts = append(registeredExts, ext)
	}
}

// GetAllRegisteredContainerExtensions 返回所有已注册插件的容器扩展名（带点号）
// 此函数是线程安全的，并且会在第一次被调用时自动完成初始化。
func GetAllRegisteredContainerExtensions() []string {
	once.Do(initializeExtensions)
	return registeredExts
}

// IsContainerPath 检查路径是否是已知的容器文件（基于扩展名）
// 此函数是线程安全的，并且会在第一次被调用时自动完成初始化。
func IsContainer(path string) bool {
	once.Do(initializeExtensions)
	ext := strings.ToLower(filepath.Ext(path))
	return registeredExtsMap[ext]
}

// BuildFullPluginSettings 构建一个完整的插件配置映射
// userSettings: 从用户配置文件中读取的原始 map
// 返回一个包含所有插件配置（用户+默认）的完整 map
func BuildFullPluginSettings(userSettings map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	fullSettings := make(map[string]json.RawMessage)

	for _, p := range Plugins {
		name := p.Name()

		// 1. 获取插件的默认配置
		defaults := p.GetDefaultSettings()

		// 2. 检查用户是否为该插件提供了配置
		userProvided, exists := userSettings[name]

		if !exists || len(userProvided) == 0 {
			// 如果用户没有提供配置，则完全使用默认值
			fullSettings[name] = defaults
		} else {
			// 如果用户提供了配置，则将其与默认值合并
			merged, err := mergeJSONObjects(defaults, userProvided)
			if err != nil {
				return nil, fmt.Errorf("failed to merge settings for plugin '%s': %w", name, err)
			}
			fullSettings[name] = merged
		}
	}
	return fullSettings, nil
}

// 辅助函数。 合并两个 JSON 对象，userConfig 中的键会覆盖 defaults 中的键
func mergeJSONObjects(defaults, userConfig json.RawMessage) (json.RawMessage, error) {
	defaultsMap := make(map[string]interface{})
	userMap := make(map[string]interface{})

	if err := json.Unmarshal(defaults, &defaultsMap); err != nil {
		return nil, fmt.Errorf("invalid default settings JSON: %w", err)
	}
	if err := json.Unmarshal(userConfig, &userMap); err != nil {
		return nil, fmt.Errorf("invalid user settings JSON: %w", err)
	}

	// 合并：用户配置覆盖默认配置
	for key, userValue := range userMap {
		defaultsMap[key] = userValue
	}

	return json.Marshal(defaultsMap)
}

func InitializePlugins(ctx context.Context) error {
	for _, p := range Plugins {
		pluginName := p.Name()
		fmt.Printf("Initializing plugin: %s\n", pluginName)

		// 3. 调用插件的初始化方法
		if err := p.Intialize(ctx); err != nil {
			return fmt.Errorf("failed to initialize plugin %s: %w", pluginName, err)
		}
	}
	return nil
}

// FindEncryptingPlugin 为给定的输入文件查找合适的加密插件
// 优先级：
// 1. 通过 MIME 类型前缀匹配
// 2. 如果没有匹配到，则通过文件扩展名匹配
// 3. 最后通过 ShouldProcess 进行最终确认
func FindEncryptingPlugin(inputPath string) (Plugin, error) {
	ext := strings.ToLower(filepath.Ext(inputPath))
	mimeType, err := utils.DetectFileMIMEType(inputPath)
	if err != nil {
		log.Printf("DEBUG: [FindEncryptingPlugin] Could not determine MIME type for '%s'. Skipping MIME-based match.", inputPath)
	}

	var candidates []Plugin

	// --- 阶段 1: MIME 类型匹配 (优先) ---
	if mimeType != "" {
		log.Printf("DEBUG: [FindEncryptingPlugin] Found MIME type '%s' for '%s'. Trying MIME-based match.", mimeType, inputPath)
		for _, p := range Plugins {
			for _, prefix := range p.SupportedMimePrefixes() {
				if strings.HasPrefix(mimeType, prefix) {
					log.Printf("DEBUG: [FindEncryptingPlugin] Plugin '%T' is a MIME candidate for prefix '%s'.", p, prefix)
					candidates = append(candidates, p)
					break // 找到一个匹配的前缀就足够了，进入下一个插件
				}
			}
		}
	} else {
		log.Printf("DEBUG: [FindEncryptingPlugin] Could not determine MIME type for '%s'. Skipping MIME-based match.", inputPath)
	}

	// --- 阶段 2: 文件扩展名匹配 (兜底) ---
	// 如果没有从 MIME 匹配中找到候选插件，则尝试扩展名匹配
	if len(candidates) == 0 {
		log.Printf("DEBUG: [FindEncryptingPlugin] No MIME-based candidates found for '%s'. Trying extension-based match.", inputPath)
		// 获取不带点的扩展名，用于比较
		extWithoutDot := ext
		if len(extWithoutDot) > 0 {
			extWithoutDot = extWithoutDot[1:]
		}

		for _, p := range Plugins {
			for _, supportedExt := range p.SupportedExtensions() {
				// 比较时不区分大小写
				if strings.ToLower(supportedExt) == extWithoutDot {
					log.Printf("DEBUG: [FindEncryptingPlugin] Plugin '%T' is an extension candidate for extension '%s'.", p, supportedExt)
					candidates = append(candidates, p)
					break // 找到一个匹配的扩展名就足够了，进入下一个插件
				}
			}
		}
	}

	// --- 阶段 3: 最终确认 ---
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no suitable plugin found to encrypt file: %s (MIME: '%s', Ext: '%s')", inputPath, mimeType, ext)
	}

	log.Printf("DEBUG: [FindEncryptingPlugin] Found %d candidates for '%s'. Running ShouldProcess.", len(candidates), inputPath)
	for _, p := range candidates {
		if p.ShouldProcess(inputPath) {
			log.Printf("INFO: [FindEncryptingPlugin] Successfully selected plugin '%T' for file '%s'.", p, inputPath)
			return p, nil
		}
	}

	// --- 阶段 4: 失败 ---
	return nil, fmt.Errorf("all candidate plugins for file '%s' were rejected by ShouldProcess", inputPath)
}

// FindDecryptingPlugin 为给定的容器文件查找合适的解密插件
func FindDecryptingPlugin(containerPath string) (Plugin, error) {
	for _, p := range Plugins {
		if p.CanDecrypt(containerPath) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no suitable plugin found to decrypt container: %s", containerPath)
}

// ProcessFileWithPlugin 是一个通用的辅助函数，它使用插件提供的策略来处理文件。
// 这个函数封装了打开文件、提取元数据和预处理内容的通用流程。
func ProcessFileWithPlugin(p Plugin, inputPath string) (types.Index, io.ReadCloser, error) {
	// 1. 使用插件提供的元数据提取器获取索引
	extractor := p.GetMetadataExtractor()
	index, err := extractor.ExtractMetadata(inputPath)
	if err != nil {
		return nil, nil, fmt.Errorf("metadata extraction failed for '%s': %w", inputPath, err)
	}

	// 2. 使用插件提供的内容预处理器获取处理后的数据流
	preprocessor := p.GetContentPreprocessor()
	dataReader, err := preprocessor.Preprocess(inputPath)
	if err != nil {
		return nil, nil, fmt.Errorf("content preprocessing failed for '%s': %w", inputPath, err)
	}

	// 3. 返回索引和数据流
	return index, dataReader, nil
}

// EncryptFileWithPlugin 是一个新的辅助函数，封装了完整的加密流程
func EncryptFileWithPlugin(ctx context.Context, plugin Plugin, inputPath, inputRootDir, outputDir string) error {
	plugin.Intialize(ctx)

	index, dataReader, err := ProcessFileWithPlugin(plugin, inputPath)
	if err != nil {
		return fmt.Errorf("plugin failed to process file '%s': %w", inputPath, err)
	}
	defer dataReader.Close()

	// 2. 执行预处理器
	if err := plugin.PreEncryptProcessor(index, inputPath, inputRootDir, outputDir); err != nil {
		return fmt.Errorf("pre-encryption failed for '%s': %w", inputPath, err)
	}
	log.Printf("✅ [EncryptFileWithPlugin] PreEncryptProcessor successfully.\n")

	// 3. 执行核心加密
	if err := plugin.Encrypt(dataReader); err != nil {
		return fmt.Errorf("encryption failed for '%s': %w", inputPath, err)
	}
	log.Printf("✅ [EncryptFileWithPlugin] Encrypt successfully.\n")

	// 4. 执行后处理器
	if err := plugin.PostEncryptProcessor(); err != nil {
		return fmt.Errorf("post-encryption failed for '%s': %w", inputPath, err)
	}
	log.Printf("✅ [EncryptFileWithPlugin] PostEncryptProcessor successfully.\n")

	return nil
}

// DecryptContainerWithPlugin 是一个新的辅助函数，封装了完整的解密流程
func DecryptContainerWithPlugin(ctx context.Context, plugin Plugin, containerPath, outputDir string) error {
	plugin.Intialize(ctx)
	// 1. 执行预处理器
	if err := plugin.PreDecryptProcessor(containerPath, outputDir); err != nil {
		return fmt.Errorf("pre-decryption failed for '%s': %w", containerPath, err)
	}
	log.Printf("✅ [DecryptContainerWithPlugin] PreDecryptProcessor successfully.\n")

	// 2. 执行核心解密
	if err := plugin.Decrypt(containerPath, outputDir); err != nil {
		return fmt.Errorf("decryption failed for '%s': %w", containerPath, err)
	}
	log.Printf("✅ [DecryptContainerWithPlugin] Decrypt successfully.\n")

	// 4. 执行后处理器
	if err := plugin.PostDecryptProcessor(containerPath); err != nil {
		return fmt.Errorf("post-encryption failed for '%s': %w", containerPath, err)
	}
	log.Printf("✅ [DecryptContainerWithPlugin] PostDecryptProcessor successfully.\n")

	return nil
}

// 遍历文件夹自动选择插件加密，这是使用 EncryptFileWithPlugin 而不是 Plugin.EncryptFile 的原因。
func WalkAndEncrypt(ctx context.Context, walkPath string, inputRootDir, outputDir string) error {
	return filepath.WalkDir(walkPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		plugin, err := FindEncryptingPlugin(path)
		if err != nil {
			fmt.Printf("INFO: Skipping file, no handler found: %s\n%v\n", path, err)
			return nil
		}

		fmt.Printf("INFO: Found plugin '%T' for file: %s\n", plugin, path)
		if err := EncryptFileWithPlugin(ctx, plugin, path, inputRootDir, outputDir); err != nil {
			fmt.Printf("WARN: Failed to encrypt '%s' with plugin '%T': %v. Continuing...\n", path, plugin, err)
		}
		return nil
	})
}
