// internal/v2/plugins/video/plugin.go

package video

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/container/detector"
	"github.com/Soltus/encv-go/internal/v2/container/fragment"
	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/namer"
	"github.com/Soltus/encv-go/internal/v2/physical"
	pluginInterfaces "github.com/Soltus/encv-go/internal/v2/plugins/interfaces"
	"github.com/Soltus/encv-go/internal/v2/plugins/interfaces/packer"
	"github.com/Soltus/encv-go/internal/v2/reader"
	"github.com/Soltus/encv-go/internal/v2/service"
	"github.com/Soltus/encv-go/internal/v2/types"
)

type VideoPlugin struct {
	ctx          context.Context
	cfg          *config.Config
	settings     VideoPluginConfig
	index        VideoIndex
	outputDir    string
	inputPath    string
	inputRootDir string
	// 记录实际用于加密的源文件路径
	// 如果 Remuxing 发生（如 MKV -> MP4），这里是临时文件路径；
	// 否则，它与 inputPath 一致。
	encryptedSourcePath string
	baseNamer           namer.BaseNamer           // 注入容器命名器
	chunkNamer          namer.ChunkNamer          // 注入分片命名器
	containerManager    *service.ContainerManager // 注入 ContainerManager
	physicalPacker      physical.PhysicalPacker
	trackExtensionsList []string
	// 【新增】存储分片集信息，用于不合并直接加密模式
	splitSets [][]string
	// 【新增】存储分片文件路径集合，用于快速查找
	splitPartPaths map[string]bool
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
	// 对于 MKV 源，是否保持其 MKV 封装
	// true: 保持 MKV (使用 mkvmerge 规范化，需要 MKVToolNix)
	// false: 转换为 fast-start MP4 (使用 ffmpeg)
	KeepMkvForMkvSource bool `json:"keep_mkv_for_mkv_source"`
	// 是否在打包后进行解密验证（耗时，默认关闭）
	// 启用后会对生成的容器进行全量解密并比对 MD5，确保解密逻辑无误。
	VerifyAfterPack bool `json:"verify_after_pack"`
	// 【新增】视频插件缓存目录，用于存放合并后的MKV和加密临时文件，为空则使用输出目录
	PluginCacheDir string `json:"plugin_cache_dir"`
	// 【新增】是否启用不合并直接加密模式
	// true: 分片MKV不合并，直接按顺序加密（更快，但可能不兼容某些播放器）
	// false: 先合并再加密（默认，兼容性更好）
	SkipMergeForSplitMKV bool `json:"skip_merge_for_split_mkv"`
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
		TrackExtensions:     ".ass,.srt,.dm.ass",
		KeepMkvForMkvSource: true,
		VerifyAfterPack:     false, // 默认关闭，避免影响打包速度
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
		{
			Key:          "keep_mkv_for_mkv_source",
			Type:         "bool",
			DefaultValue: true,
			Help:         "If the source is an MKV file, keep its MKV container (remuxed with mkvmerge). If disabled, all sources will be converted to fast-start MP4.",
		},
		{
			Key:          "verify_after_pack",
			Type:         "bool",
			DefaultValue: false,
			Help:         "If enabled, verifies the container integrity by decrypting it and checking MD5 after packing. This takes extra time.",
		},
		{
			Key:          "plugin_cache_dir",
			Type:         "string",
			DefaultValue: "",
			Help:         "Video plugin cache directory for storing merged MKV files and encryption temp files. Leave empty to use output directory. Setting this can cache intermediate products to avoid reprocessing.",
		},
		{
			Key:          "skip_merge_for_split_mkv",
			Type:         "bool",
			DefaultValue: false,
			Help:         "If enabled, split MKV files will NOT be merged before encryption. Instead, they will be encrypted sequentially as multiple streams. This is faster but may have compatibility issues with some players.",
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
		log.Printf("INFO: [%s] Physical chunking enabled. Size: %d MB\n", p.Name(), p.settings.ChunkSizeMB)
		p.physicalPacker = physical.NewFileChunkerPhysicalPacker(int64(p.settings.ChunkSizeMB)*1024*1024, p.chunkNamer)
	} else {
		log.Printf("INFO: [%s] Physical chunking disabled.\n", p.Name())
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
		// log.Printf("DEBUG: [VideoPlugin.CanDecrypt] Failed to detect kind for '%s': %v\n", containerPath, err)
		return false
	}
	return kind == IndexKindVideo
}

// 实现 plugins.Plugin 接口
func (p *VideoPlugin) GetMetadataExtractor() pluginInterfaces.MetadataExtractor {
	return &VideoMetadataExtractor{
		settings: p.settings,
		index:    &p.index,
	}
}

// 实现 plugins.Plugin 接口
func (p *VideoPlugin) GetContentPreprocessor() pluginInterfaces.ContentPreprocessor {
	return &VideoContentPreprocessor{
		settings:       p.settings,
		index:          &p.index,
		outputDir:      p.outputDir,
		splitPartPaths: p.splitPartPaths,
	}
}

// 实现 plugins.Plugin 接口
func (p *VideoPlugin) GetContentVirifier() pluginInterfaces.ContentVerifier {
	return &VideoContentVerifier{}
}

// 实现 plugins.Plugin 接口
func (p *VideoPlugin) GetPhysicalPacker() physical.PhysicalPacker {
	return p.physicalPacker
}

func (p *VideoPlugin) BuildFragments(logicalFileSize int64) ([]types.Fragment_v2, error) {
	return p.createGOPAlignedFragments(logicalFileSize)
}

func (p *VideoPlugin) GroupFiles(inputPaths []string, inputRootDir, outputDir string) ([]string, error) {

	// 1. 过滤出当前插件需要处理的 MKV 文件
	var mkvPaths []string
	for _, path := range inputPaths {
		if strings.ToLower(filepath.Ext(path)) == ".mkv" {
			mkvPaths = append(mkvPaths, path)
		}
	}
	if len(mkvPaths) == 0 {
		return inputPaths, nil // 没有 MKV，无需处理
	}

	// 2. 批量获取所有 MKV 的信息并缓存
	fmt.Println("-> [VIDEO_PLUGIN] Pre-scanning all MKV files in the directory for split parts...")
	mkvInfos, err := batchGetMkvInfos(mkvPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to batch scan MKV files: %w", err)
	}

	// 3. 根据 UID 关联分片
	splitSets := groupSplitParts(mkvInfos)

	// 【关键修复】始终保存分片集信息，供后续阶段使用（包括缓存文件名恢复）
	p.splitSets = splitSets
	// 【新增】构建分片路径集合，用于预处理时快速判断
	p.splitPartPaths = make(map[string]bool)
	for _, set := range splitSets {
		for _, path := range set {
			p.splitPartPaths[path] = true
		}
	}

	// 【新增】如果启用了不合并模式，直接返回所有文件路径（包括分片）
	// 加密阶段会处理多文件读取
	if p.settings.SkipMergeForSplitMKV && len(splitSets) > 0 {
		log.Printf("-> [VIDEO_PLUGIN] SkipMergeForSplitMKV enabled, processing %d split sets without merging\n", len(splitSets))
		return inputPaths, nil
	}

	// 4. 合并分片并构建最终的文件列表
	finalPaths := make([]string, 0)

	// 1. 首先将所有非 MKV 文件加入最终列表
	for _, path := range inputPaths {
		if strings.ToLower(filepath.Ext(path)) != ".mkv" {
			finalPaths = append(finalPaths, path)
		}
	}

	// 2. 然后处理所有分片集
	// 【修改】传入缓存目录
	cacheDir := p.settings.PluginCacheDir
	if cacheDir == "" {
		cacheDir = outputDir // 默认使用输出目录作为缓存
		log.Printf("-> [VIDEO_PLUGIN] PluginCacheDir not set, using outputDir as cache: %s\n", cacheDir)
	} else {
		log.Printf("-> [VIDEO_PLUGIN] Using configured PluginCacheDir: %s\n", cacheDir)
	}
	for _, set := range splitSets {
		log.Printf("-> [VIDEO_PLUGIN] Found a split set with %d parts. Merging...\n", len(set))
		mergedPath, err := mergeSplitPartsFromSet(set, outputDir, cacheDir)
		if err != nil {
			return nil, fmt.Errorf("failed to merge a split set: %w", err)
		}
		finalPaths = append(finalPaths, mergedPath)
	}

	// 3. 最后，将所有独立的（非分片）MKV 文件加入最终列表
	for _, info := range mkvInfos {
		if !info.IsSplitPart {
			finalPaths = append(finalPaths, info.Path)
		}
	}

	return finalPaths, nil
}

// --- 加密逻辑 ---

// Plugin 接口实现
// 在加密前处理，并更新 Index
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

	// 【关键修复】如果处理的是合并后的缓存文件，使用第一个分片的原始文件名
	originalFilename := p.index.OriginalFilename
	if p.settings.PluginCacheDir != "" && len(p.splitSets) > 0 {
		cacheDir := filepath.Clean(p.settings.PluginCacheDir)
		inputDir := filepath.Clean(filepath.Dir(inputPath))
		if cacheDir == inputDir {
			// 这是合并后的文件，使用第一个分片的文件名
			firstPartPath := p.splitSets[0][0]
			originalFilename = filepath.Base(firstPartPath)
			log.Printf("-> [VIDEO_PLUGIN] Using original filename from first split part: %s\n", originalFilename)
			// 更新 index 中的原始文件名
			p.index.OriginalFilename = originalFilename
		}
	}

	encryptedBaseName := p.baseNamer.GenerateEncryptedBaseName(originalFilename)

	// 调用字幕处理逻辑，它会修改 p.index.SubtitleTrack
	return p.HandleSubtitlesForEncryption(p.cfg, &p.index, outputDir, encryptedBaseName)
}

// Plugin 接口实现
// 执行核心的加密工作，并调用 Packer
func (p *VideoPlugin) Encrypt(dataReader io.Reader) (*crypto.EncryptionResult, error) {
	guardKey := fmt.Sprintf("%s|%s", p.inputPath, p.outputDir)

	var result *crypto.EncryptionResult
	err := utils.Do(guardKey, func() error {
		// 【关键修复】尝试捕获实际的源文件路径
		// 框架传递的 dataReader 可能是 os.Open(original)，
		// 也可能是 Preprocess 生成的 os.Open(temp_remuxed)。
		// 通过类型断言，我们可以获取底层文件句柄，从而获取真实路径。
		if file, ok := dataReader.(*os.File); ok {
			p.encryptedSourcePath = file.Name()
		} else {
			// 如果不是文件（如内存 Reader），回退到 inputPath
			p.encryptedSourcePath = p.inputPath
		}

		// 1. 执行加密
		// crypto.EncryptToTempFile 会读取 dataReader
		// 如果 dataReader 指向 Remux 后的文件，加密的也就是 Remux 后的内容
		var err error
		// 【修改】使用配置的缓存目录作为加密临时文件目录，未配置时使用 outputDir
		encryptTempDir := p.settings.PluginCacheDir
		if encryptTempDir == "" {
			encryptTempDir = p.outputDir
			log.Printf("-> [VIDEO_PLUGIN] PluginCacheDir not set, using outputDir for encryption temp files\n")
		} else {
			// 确保缓存目录存在
			if mkdirErr := os.MkdirAll(encryptTempDir, 0755); mkdirErr != nil {
				log.Printf("-> [VIDEO_PLUGIN] Failed to create cache dir, falling back to outputDir: %v\n", mkdirErr)
				encryptTempDir = p.outputDir
			} else {
				log.Printf("-> [VIDEO_PLUGIN] Using PluginCacheDir for encryption temp files: %s\n", encryptTempDir)
			}
		}
		result, err = crypto.EncryptToTempFile_v2(dataReader, p.cfg.Password, encryptTempDir)
		if err != nil {
			return fmt.Errorf("failed to encrypt to temp file: %w", err)
		}

		log.Printf("INFO: [%s] Encrypted to temporary file: %s\n", p.Name(), result.TempPath)
		log.Printf("DEBUG: [%s] Actual Encrypted Source Path: %s\n", p.Name(), p.encryptedSourcePath)
		log.Printf("✅ [%s] Encrypted successfully.\n", p.Name())
		return nil
	})

	return result, err
}

// Plugin 接口实现
// 加密后处理器
func (p *VideoPlugin) PostEncryptProcessor(result *crypto.EncryptionResult) error {
	// 1. 【性能优化】使用传入的 result 中的精确大小
	logicalDataSize := result.EncryptedPayloadSize

	// 2. 生成逻辑分片 (视频特有逻辑)
	var logicalFragments []types.Fragment_v2
	var err error

	logicalFragments, err = p.createGOPAlignedFragments(logicalDataSize)
	if err != nil {
		log.Printf("WARN: Failed to align fragments to GOP (%v). Falling back to size-based fragments.\n", err)
		baseSize := fragment.CalculateFragmentSize(logicalDataSize, int64(p.settings.ChunkSizeMB)*1024*1024)
		logicalFragments, err = fragment.CreateLogicalFragmentsFromSize(logicalDataSize, baseSize, types.FragmentType_SeekableStream)
		if err != nil {
			return fmt.Errorf("fallback fragment creation failed: %w", err)
		}
	}

	// 更新索引中的文件大小（保留原始明文文件大小用于元数据）
	if stat, err := os.Stat(p.inputPath); err == nil {
		p.index.OriginalFileSize = stat.Size()
	}

	// 3. 构造 Video 特有的 Manifest（KVI + LogicalFragments）
	// 使用 result 中的 Salt 和 IV
	kvi := VideoKVI_v2{
		KVI_v2: types.KVI_v2{
			SaltBase64: crypto.Base64Encode_v2(result.Salt),
			IVBase64:   crypto.Base64Encode_v2(result.IV),
		},
		VideoIndex: &p.index,
	}
	manifest, err := types.NewManifest_v2(kvi, logicalFragments)
	if err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	// 4. 准备通用 PackRequest
	finalFilename := p.chunkNamer.GenerateMainChunkName(p.baseNamer.GenerateEncryptedBaseName(p.index.OriginalFilename))
	finalBaseName := strings.TrimSuffix(finalFilename, p.settings.Ext)

	startIdx := 1
	if !p.settings.LightMainChunkEnabled {
		startIdx = 0
	}

	// 5. 【重构】直接构造 PackParams，不再构造嵌套的 PackRequest
	packParams := &packer.PackParams{
		// --- 核心数据 ---
		Manifest:       manifest,
		PhysicalPacker: p.physicalPacker,
		TempEncPath:    result.TempPath,

		// --- 加密参数 ---
		Salt:                 result.Salt,
		IV:                   result.IV,
		SaltIVHeaderSize:     result.SaltIVHeaderSize,
		EncryptedPayloadSize: result.EncryptedPayloadSize,

		// --- Packer 配置字段 ---
		BaseName:              finalBaseName,
		OutputDir:             p.outputDir,
		Index:                 &p.index,
		Namer:                 p.chunkNamer,
		StartIdx:              startIdx,
		LightMainChunkEnabled: p.settings.LightMainChunkEnabled,
		HeaderVersion:         3,
		SpecialIDType:         types.IDType_Raw,
		SpecialID:             nil,
	}

	if p.settings.ChunkSizeMB == 0 {
		packParams.FinalFileName = finalFilename
	}

	// 6. 调用唯一通用代理：packer.StandardPostEncrypt
	// Helper 内部会组装 physical.PackRequest
	if err := packer.StandardPostEncrypt(packParams); err != nil {
		// 打包失败时，清理临时文件
		os.Remove(result.TempPath)
		return fmt.Errorf("packing failed: %w", err)
	}

	// 7. 清理临时加密文件（打包成功后不再需要）
	os.Remove(result.TempPath)

	// 8. 验证（如果启用）
	if p.settings.VerifyAfterPack {
		return p.verifyContainer()
	}

	log.Printf("✅ [%s] packed successfully.\n", p.Name())
	return nil
}

// createGOPAlignedFragments 基于关键帧位置生成逻辑分片
// 【通用性】适用于所有视频，依赖 p.index.KeyFrameOffsets
func (p *VideoPlugin) createGOPAlignedFragments(fileSize int64) ([]types.Fragment_v2, error) {
	// 如果没有提取到关键帧（例如 ffprobe 失败），则无法进行 GOP 对齐
	if len(p.index.KeyFrameOffsets) == 0 {
		return nil, fmt.Errorf("no keyframe offsets available for GOP alignment")
	}

	// 计算目标逻辑分片大小（例如 4MB）
	targetLogicalSize := fragment.CalculateFragmentSize(fileSize, int64(p.settings.ChunkSizeMB)*1024*1024)
	minLogicalSize := int64(2 * 1024 * 1024) // 2MB

	var fragments []types.Fragment_v2
	currentOffset := uint64(0)
	fragIndex := 0

	for currentOffset < uint64(fileSize) {
		targetEndOffset := currentOffset + uint64(targetLogicalSize)
		actualEndOffset := targetEndOffset

		// 1. 在 KeyFrameOffsets 中查找第一个 >= targetEndOffset 的关键帧
		k := sort.Search(len(p.index.KeyFrameOffsets), func(i int) bool {
			return p.index.KeyFrameOffsets[i] >= targetEndOffset
		})

		// 2. 尝试使用找到的关键帧（或其前一个关键帧）作为分片结束点
		if k < len(p.index.KeyFrameOffsets) {
			candidateKF := p.index.KeyFrameOffsets[k]
			// 检查这个关键帧是否在合理范围内
			// 策略：优先向后对齐到 I 帧
			if candidateKF > currentOffset {
				// 确保 resulting fragment >= minLogicalSize
				if int64(candidateKF-currentOffset) >= minLogicalSize {
					actualEndOffset = candidateKF
				} else if k+1 < len(p.index.KeyFrameOffsets) {
					// 如果使用这个 KF 太小，尝试用下一个 KF
					nextKF := p.index.KeyFrameOffsets[k+1]
					if int64(nextKF-currentOffset) < int64(targetLogicalSize)*2 { // 不超过目标的两倍
						actualEndOffset = nextKF
					}
				}
			}
		}

		// 3. 边界保护：不能超过文件末尾
		if actualEndOffset > uint64(fileSize) {
			actualEndOffset = uint64(fileSize)
		}

		// 4. 创建 Fragment
		frag := types.Fragment_v2{
			ID:                fmt.Sprintf("logical_fragment_%d", fragIndex),
			Type:              types.FragmentType_SeekableStream,
			GlobalStartOffset: currentOffset,
			Length:            actualEndOffset - currentOffset,
		}
		fragments = append(fragments, frag)

		// 记录详细信息用于调试
		// log.Printf("DEBUG: [GOP Frag] ID=%d, Start=%d, End=%d, Length=%d", fragIndex, currentOffset, actualEndOffset, frag.Length)

		currentOffset = actualEndOffset
		fragIndex++
	}

	// 对比计算出的总大小与文件大小
	var totalCalculatedLen uint64 = 0
	for _, f := range fragments {
		totalCalculatedLen += f.Length
	}
	log.Printf("INFO: [GOP Frag] FileSize=%d, CalculatedTotalLen=%d, Diff=%d", fileSize, totalCalculatedLen, int64(fileSize)-int64(totalCalculatedLen))

	// 【修复】如果计算出的总大小与文件大小不一致，说明 GOP 对齐导致最后一块丢失，这里强制截断最后一个 Fragment
	if totalCalculatedLen > uint64(fileSize) {
		missingBytes := totalCalculatedLen - uint64(fileSize)
		log.Printf("WARN: [GOP Frag] Calculated total length exceeds file size by %d bytes. Trimming last fragment.", missingBytes)
		lastFrag := &fragments[len(fragments)-1]
		lastFrag.Length = lastFrag.Length - uint64(missingBytes)
		totalCalculatedLen -= uint64(missingBytes)
	}

	if totalCalculatedLen != uint64(fileSize) {
		return nil, fmt.Errorf("GOP Fragments total length (%d) still mismatches file size (%d)", totalCalculatedLen, fileSize)
	}

	return fragments, nil
}

// verifyContainer 将验证逻辑提取为单独方法，保持 PostEncryptProcessor 简洁
func (p *VideoPlugin) verifyContainer() error {
	finalFilename := p.chunkNamer.GenerateMainChunkName(p.baseNamer.GenerateEncryptedBaseName(p.index.OriginalFilename))
	mainChunkFullPath := filepath.Join(p.outputDir, finalFilename)

	verifyTempDir := filepath.Join(p.outputDir, ".encv_verify_"+utils.RandomString(8))
	if err := os.MkdirAll(verifyTempDir, 0755); err != nil {
		return fmt.Errorf("failed to create verification temp dir: %w", err)
	}

	if err := p.Decrypt(mainChunkFullPath, verifyTempDir); err != nil {
		os.RemoveAll(verifyTempDir)
		return fmt.Errorf("verification failed: decryption error occurred: %w", err)
	}

	decrypedFilePath := filepath.Join(verifyTempDir, p.index.OriginalFilename)
	verifier := p.GetContentVirifier().(*VideoContentVerifier)

	sourcePath := p.encryptedSourcePath
	if sourcePath == "" {
		sourcePath = p.inputPath
	}

	if err := verifier.Verify(sourcePath, decrypedFilePath); err != nil {
		os.RemoveAll(verifyTempDir)
		return fmt.Errorf("container verification failed: %w", err)
	}
	os.RemoveAll(verifyTempDir)
	log.Printf("✅ [%s] Container verified successfully.\n", p.Name())
	return nil
}

// --- 解密逻辑 ---

// Plugin 接口实现
func (p *VideoPlugin) PreDecryptProcessor(containerPath, outputDir string) error {
	p.outputDir = outputDir
	return nil
}

// Plugin 接口实现
func (p *VideoPlugin) Decrypt(containerPath, outputDir string) error {
	log.Printf("DEBUG: [%s] Starting decryption for: %s\n", p.Name(), containerPath)
	p.outputDir = outputDir

	// --- 1. 【关键】通过 ContainerManager 获取一个可读的容器路径 ---
	// ContainerManager 会智能地决定是使用原始文件还是重建文件
	readablePath, err := p.containerManager.GetReadablePath(containerPath, p.chunkNamer)
	if err != nil {
		return fmt.Errorf("failed to get readable path from container manager: %w", err)
	}
	log.Printf("DEBUG: [%s] Using readable path: %s\n", p.Name(), readablePath)

	// --- 2. 使用统一路径创建 reader 工厂 ---
	factory, err := reader.NewDecryptReaderFactory(readablePath, p.cfg.Password)
	if err != nil {
		return fmt.Errorf("failed to create reader factory for '%s': %w", readablePath, err)
	}
	defer factory.Close() // 【关键】这个 Close 会同时清理物理临时文件（如果存在）
	log.Printf("DEBUG: [%s] Reader factory created successfully.\n", p.Name())

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

	log.Printf("✅ [%s] Decrypted to: %s\n", p.Name(), outputPath)
	return nil
}

// Plugin 接口实现
// 在解密后处理
func (p *VideoPlugin) PostDecryptProcessor(containerPath string) error {

	containerDir := filepath.Dir(containerPath)
	// 注意：这里的 vIndex.SubtitleTrack 应该在 Decrypt 方法中被正确设置
	if err := RestoreSubtitlesForDecryption(&p.index, containerDir, p.outputDir); err != nil {
		return fmt.Errorf("failed to restore subtitles: %w", err)
	}

	return nil
}
