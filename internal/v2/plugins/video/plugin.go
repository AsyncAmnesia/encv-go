// internal/v2/plugins/video/plugin.go

package video

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/container/detector"
	"github.com/Soltus/encv-go/internal/v2/container/fragment"
	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/namer"
	"github.com/Soltus/encv-go/internal/v2/physical"
	pluginInterfaces "github.com/Soltus/encv-go/internal/v2/plugins/interfaces"
	"github.com/Soltus/encv-go/internal/v2/reader"
	"github.com/Soltus/encv-go/internal/v2/service"
	"github.com/Soltus/encv-go/internal/v2/types"
)

type VideoPlugin struct {
	ctx                 context.Context
	cfg                 *config.Config
	settings            VideoPluginConfig
	index               VideoIndex
	outputDir           string
	inputPath           string
	inputRootDir        string
	tempEncPath         string
	salt                []byte
	iv                  []byte
	baseNamer           namer.BaseNamer           // 注入容器命名器
	chunkNamer          namer.ChunkNamer          // 注入分片命名器
	containerManager    *service.ContainerManager // 注入 ContainerManager
	physicalPacker      physical.PhysicalPacker
	trackExtensionsList []string
}

func (p *VideoPlugin) Name() string {
	return "video" // 这个字符串必须与配置文件中的键对应
}

// Plugin 接口实现
func (p *VideoPlugin) GetContainerExtension() string {
	return p.settings.Ext
}

type VideoPluginConfig struct {
	// 容器扩展名，包含点前缀，默认值为 ".sccgv"
	Ext string `json:"ext"`
	// 分片大小，0 表示不启用，允许的最小值为 30
	ChunkSizeMB int `json:"chunk_size_mb"`
	// 分片最大数量，0 表示不限制，优先级高于 ChunkSizeMB TODO: 待实现
	// ChunkMax              int    `json:"chunck_max"`

	// 是否启用轻量级主分片，启用后主分片只包含清单，不包含源数据
	LightMainChunkEnabled bool `json:"light_main_chunk_enabled"`
	// TrackExtensions 视频容器的字幕/轨道文件扩展名列表，它们并不会打包到容器里。
	TrackExtensions string `json:"track_extensions"`
}

func (p *VideoPlugin) GetSettingsSchemaType() interface{} {
	return VideoPluginConfig{}
}

// 2. 实现接口方法，返回默认配置的 JSON
func (p *VideoPlugin) GetDefaultSettings() json.RawMessage {
	defaultCfg := VideoPluginConfig{
		Ext:         ".sccgv",
		ChunkSizeMB: 0,
		// ChunkMax:    0,
		TrackExtensions: ".ass,.srt,.dm.ass",
	}
	data, _ := json.Marshal(defaultCfg) // 忽略错误，因为默认值是硬编码的，不会出错
	return data
}

func (p *VideoPlugin) GetSettingFields() []pluginInterfaces.SettingField {
	return []pluginInterfaces.SettingField{
		{
			Key:          "ext",
			Type:         "string",
			DefaultValue: ".sccgv",
			Help:         "The container file extension for encrypted video files (e.g., '.sccgv').",
		},
		{
			Key:          "chunk_size_mb",
			Type:         "number",
			DefaultValue: 0,
			Help:         "The chunk size in MB. Set to 0 to disable physical chunking. Minimum value is 30 if enabled.",
		},
		{
			Key:          "light_main_chunk_enabled",
			Type:         "bool",
			DefaultValue: false,
			Help:         "If enabled, the main chunk will only contain the manifest, not the source data.",
		},
		{
			Key:          "track_extensions",
			Type:         "text", // 使用 text 类型，让用户输入逗号分隔的字符串
			DefaultValue: ".ass,.srt,.dm.ass",
			Help:         "Subtitle/track file extensions, separated by commas (e.g., .ass,.srt,.dm.ass). These files are not packed into the container.",
		},
	}
}

// init 在包被导入时自动执行，完成自注册
func init() {
	types.RegisterKVIProvider(IndexKindVideo, func(rawKVI json.RawMessage) (types.KVIProvider, error) {
		var kvi VideoKVI_v2
		if err := json.Unmarshal(rawKVI, &kvi); err != nil {
			return nil, fmt.Errorf("failed to unmarshal KVI: %w", err)
		}
		return kvi, nil
	})
}

// Plugin 接口实现
func (p *VideoPlugin) Initialize(ctx context.Context) error {
	if ctx == p.ctx {
		return nil // 避免重复初始化
	}
	p.ctx = ctx
	p.cfg = config.FromContext(ctx)
	// 2. 【关键】使用泛型辅助函数，安全地获取并解析本插件的配置
	settings, err := config.GetPluginSettingsFor[VideoPluginConfig](p.cfg, p.Name())
	if err != nil {
		return fmt.Errorf("could not get settings for plugin %s: %w", p.Name(), err)
	}
	p.settings = *settings // 将指针解引用，存入
	if p.settings.ChunkSizeMB > 0 && p.settings.ChunkSizeMB < 30 {
		p.settings.ChunkSizeMB = 30 // 强制修改为 30
	}
	// 处理逗号分隔的字符串，并存储到 trackExtensionsList
	if p.settings.TrackExtensions != "" {
		parts := strings.Split(p.settings.TrackExtensions, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				p.trackExtensionsList = append(p.trackExtensionsList, trimmed)
			}
		}
	}
	p.containerManager = service.NewContainerManager()
	p.baseNamer = namer.NewDefaultBaseNamer()
	p.chunkNamer = namer.NewPaddedNamer(p.settings.Ext, p.baseNamer, 4) // 补零到4位
	if p.settings.ChunkSizeMB > 0 {
		fmt.Printf("INFO: [%s] Physical chunking enabled. Size: %d MB\n", p.Name(), p.settings.ChunkSizeMB)
		p.physicalPacker = physical.NewFileChunkerPhysicalPacker(int64(p.settings.ChunkSizeMB)*1024*1024, p.chunkNamer)
	} else {
		fmt.Printf("INFO: [%s] Physical chunking disabled.\n", p.Name())
		p.physicalPacker = physical.NewSinglePhysicalPacker()
	}
	return nil
}

// Plugin 接口实现
//
//	返回在 Initialize 阶段已经配置好的 chunkNamer
func (p *VideoPlugin) GetChunkNamer() namer.ChunkNamer {
	return p.chunkNamer
}

// Plugin 接口实现
func (p *VideoPlugin) SupportedMimePrefixes() []string {
	return []string{"video/", "application/vnd.rn-realmedia-vbr", "application/vnd.apple.mpegurl"}
}

// Plugin 接口实现
func (p *VideoPlugin) SupportedExtensions() []string {
	// 当 MIME 类型无法识别时，通过这些扩展名进行兜底匹配
	return []string{
		"mp4",
		"mkv",
		"avi",
		"mov",
		"rmvb",
		"webm",
		"flv",
		"m3u8",
	}
}

// Plugin 接口实现
func (p *VideoPlugin) ShouldProcess(inputPath string) bool {
	ext := strings.ToLower(filepath.Ext(inputPath))
	if ext == ".srt" || ext == ".ass" || ext == ".vtt" {
		return false
	}
	return true
}

//	Plugin 接口实现
//
// 判断此插件是否能解密给定的容器文件。
// 【关键修复】不再依赖不可靠的文件扩展名，而是通过读取容器元数据来判断其内容类型。
func (p *VideoPlugin) CanDecrypt(containerPath string) bool {
	kind, err := detector.DetectIndexKind(containerPath)
	if err != nil {
		// 如果无法判断类型（例如，文件损坏或不是 ENCV 容器），则认为不能解密
		// 这里的日志可以帮助调试
		// fmt.Printf("DEBUG: [VideoPlugin.CanDecrypt] Failed to detect kind for '%s': %v\n", containerPath, err)
		return false
	}
	return kind == IndexKindVideo
}

// 【新增方法】实现 plugins.Plugin 接口
func (p *VideoPlugin) GetMetadataExtractor() pluginInterfaces.MetadataExtractor {
	return &VideoMetadataExtractor{}
}

// 【新增方法】实现 plugins.Plugin 接口
func (p *VideoPlugin) GetContentPreprocessor() pluginInterfaces.ContentPreprocessor {
	return &VideoContentPreprocessor{}
}

// --- 加密逻辑 ---

// Plugin 接口实现
// 在加密前处理字幕，并更新 Index
func (p *VideoPlugin) PreEncryptProcessor(index types.Index, inputPath, inputRootDir, outputDir string) error {
	vIndex, ok := index.(*VideoIndex)
	if !ok {
		return fmt.Errorf("%s plugin received a non-%s index", p.Name(), p.Name())
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	p.index = *vIndex
	p.inputPath = inputPath
	p.inputRootDir = inputRootDir
	p.outputDir = outputDir

	encryptedBaseName := p.baseNamer.GenerateEncryptedBaseName(p.index.OriginalFilename)

	// 调用字幕处理逻辑，它会修改 p.index.SubtitleTrack
	return p.HandleSubtitlesForEncryption(p.cfg, &p.index, outputDir, encryptedBaseName)
}

// Plugin 接口实现
// 执行核心的加密工作，并调用 Packer
func (p *VideoPlugin) Encrypt(dataReader io.Reader) error {
	guardKey := fmt.Sprintf("%s|%s", p.inputPath, p.outputDir)

	return utils.Do(guardKey, func() error {

		// --- 1. 【抽离】将加密逻辑委托给 crypto 包 ---
		tempEncPath, salt, iv, err := crypto.EncryptToTempFile(dataReader, p.cfg.Password, p.outputDir)
		if err != nil {
			return fmt.Errorf("failed to encrypt to temp file: %w", err)
		}
		p.tempEncPath = tempEncPath
		p.salt = salt
		p.iv = iv

		fmt.Printf("INFO: [%s] Encrypted to temporary file: %s\n", p.Name(), tempEncPath)

		fmt.Printf("✅ [%s] Encrypted successfully.\n", p.Name())
		return nil
	})
}

// Plugin 接口实现
// 视频插件在加密后处理器
func (p *VideoPlugin) PostEncryptProcessor() error {
	// --- 【关键修复】在这里，根据原始文件大小计算逻辑分片 ---
	logicalFragmentSize := fragment.CalculateFragmentSize(p.index.OriginalFileSize, int64(p.settings.ChunkSizeMB)*1024*1024)
	logicalFragments, err := fragment.CreateLogicalFragmentsFromSize(p.index.OriginalFileSize, logicalFragmentSize, types.FragmentType_SeekableStream)
	if err != nil {
		return fmt.Errorf("failed to create logical fragments from size: %w", err)
	}
	// 打印出生成的片段数量，用于调试
	fmt.Printf("-> [PLUGIN] Generated %d logical fragments.\n", len(logicalFragments))

	// 重新打开临时文件，作为加密数据源传递给 Packer
	encryptedDataReader, err := os.Open(p.tempEncPath)
	if err != nil {
		return fmt.Errorf("failed to open temp file for packing: %w", err)
	}
	defer os.Remove(p.tempEncPath) // 确保在使用完毕后删除临时文件

	// --- 5. 创建 Packer 并执行打包 ---
	encryptedBaseName := p.baseNamer.GenerateEncryptedBaseName(p.index.OriginalFilename)
	finalFilename := p.chunkNamer.GenerateMainChunkName(encryptedBaseName)
	finalBaseName := strings.TrimSuffix(finalFilename, p.settings.Ext)
	// 决定打包策略和起始索引
	var startIdx int
	if p.settings.LightMainChunkEnabled {
		startIdx = 1
	} else {
		startIdx = 0
	}
	packer := NewVideoPacker(p.physicalPacker)
	packReq := &physical.PackRequest{
		BaseName:            finalBaseName,
		OutputDir:           p.outputDir,
		EncryptedDataReader: encryptedDataReader,
		Index:               &p.index, // Packer 将从 vIndex 获取所需信息
		Salt:                p.salt,
		IV:                  p.iv,
		LogicalFragments:    logicalFragments, // 预先计算好
		Namer:               p.chunkNamer,     // 注入分片命名器
		StartIdx:            startIdx,
	}
	// 【关键修复】在单文件模式下，必须显式设置 FinalFileName
	if p.settings.ChunkSizeMB == 0 {
		packReq.FinalFileName = finalFilename
		fmt.Printf("DEBUG [PostEncryptProcessor]: Set packReq.FinalFileName for single-file mode: '%s'\n", packReq.FinalFileName)
	}

	if err := packer.Pack(p.cfg, packReq); err != nil {
		encryptedDataReader.Close()
		return fmt.Errorf("%s packing failed: %w", finalFilename, err)
	}

	encryptedDataReader.Close() // Packer 使用完毕后关闭
	fmt.Printf("✅ [%s] packed successfully.\n", p.Name())
	return nil
}

// --- 解密逻辑 ---

// Plugin 接口实现
// 视频插件在解密前无需额外操作
func (p *VideoPlugin) PreDecryptProcessor(containerPath, outputDir string) error {
	// 视频插件在此阶段无需操作
	return nil
}

// Plugin 接口实现
func (p *VideoPlugin) Decrypt(containerPath, outputDir string) error {
	fmt.Printf("DEBUG: [%s] Starting decryption for: %s\n", p.Name(), containerPath)
	p.outputDir = outputDir

	// --- 1. 【关键】通过 ContainerManager 获取一个可读的容器路径 ---
	// ContainerManager 会智能地决定是使用原始文件还是重建文件
	readablePath, err := p.containerManager.GetReadablePath(containerPath, p.chunkNamer)
	if err != nil {
		return fmt.Errorf("failed to get readable path from container manager: %w", err)
	}
	fmt.Printf("DEBUG: [%s] Using readable path: %s\n", p.Name(), readablePath)

	// --- 2. 使用统一路径创建 reader 工厂 ---
	factory, err := reader.NewDecryptReaderFactory(readablePath, p.cfg.Password)
	if err != nil {
		return fmt.Errorf("failed to create reader factory for '%s': %w", readablePath, err)
	}
	defer factory.Close() // 【关键】这个 Close 会同时清理物理临时文件（如果存在）
	fmt.Printf("DEBUG: [%s] Reader factory created successfully.\n", p.Name())

	// --- 3. 使用工厂创建解密流并写入文件 ---
	decryptedReader, err := factory.NewDecryptReader()
	if err != nil {
		return fmt.Errorf("[%s] failed to create decrypt reader: %w", p.Name(), err)
	}
	defer decryptedReader.Close()

	// 从 KVI 获取原始文件名
	index := factory.GetIndex()
	vIndex, ok := index.(*VideoIndex)
	if !ok {
		return fmt.Errorf("container is not a video container")
	}

	outputPath := filepath.Join(outputDir, vIndex.GetOriginalFilename())
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	if _, err := io.Copy(outputFile, decryptedReader); err != nil {
		return fmt.Errorf("failed to write decrypted video stream: %w", err)
	}

	p.index = *vIndex

	fmt.Printf("✅ [%s] Decrypted to: %s\n", p.Name(), outputPath)
	return nil
}

// Plugin 接口实现
// 在解密后处理字幕还原
func (p *VideoPlugin) PostDecryptProcessor(containerPath string) error {

	containerDir := filepath.Dir(containerPath)
	// 注意：这里的 vIndex.SubtitleTrack 应该在 Decrypt 方法中被正确设置
	if err := RestoreSubtitlesForDecryption(&p.index, containerDir, p.outputDir); err != nil {
		return fmt.Errorf("failed to restore subtitles: %w", err)
	}

	return nil
}
