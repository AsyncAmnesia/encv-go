package wps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	"github.com/Soltus/encv-go/internal/v2/plugins/interfaces/packer"
	"github.com/Soltus/encv-go/internal/v2/reader"
	"github.com/Soltus/encv-go/internal/v2/service"
	"github.com/Soltus/encv-go/internal/v2/types"
)

type WPSPlugin struct {
	ctx              context.Context
	cfg              *config.Config
	settings         WPSPluginConfig
	index            WPSIndex
	outputDir        string
	inputPath        string
	inputRootDir     string
	baseNamer        namer.BaseNamer           // 注入容器命名器
	containerManager *service.ContainerManager // 注入 ContainerManager
	physicalPacker   physical.PhysicalPacker
}

func (p *WPSPlugin) Name() string {
	return "wps" // 这个字符串必须与配置文件中的键对应
}

// Plugin 接口实现
func (p *WPSPlugin) GetContainerExtension() string {
	return p.settings.Ext
}

type WPSPluginConfig struct {
	// 容器扩展名，包含点前缀，默认值为 ".sccgwps"
	Ext string `json:"ext"`
}

func (p *WPSPlugin) GetSettingsSchemaType() interface{} {
	return WPSPluginConfig{}
}

// 2. 实现接口方法，返回默认配置的 JSON
func (p *WPSPlugin) GetDefaultSettings() json.RawMessage {
	defaultCfg := WPSPluginConfig{
		Ext: ".sccgwps",
	}
	data, _ := json.Marshal(defaultCfg) // 忽略错误，因为默认值是硬编码的，不会出错
	return data
}

func (p *WPSPlugin) GetSettingFields() []pluginInterfaces.SettingField {
	return []pluginInterfaces.SettingField{
		{
			Key:          "ext",
			Type:         "string",
			DefaultValue: ".sccgwps",
			Help:         "The container file extension for encrypted text files (e.g., '.sccgwps').",
		},
	}
}

// init 在包被导入时自动执行，完成自注册
func init() {
	types.RegisterKVIProvider(IndexKindWPS, func(rawKVI json.RawMessage) (types.KVIProvider, error) {
		var kvi WPSKVI_v2
		if err := json.Unmarshal(rawKVI, &kvi); err != nil {
			return nil, fmt.Errorf("failed to unmarshal KVI: %w", err)
		}
		return kvi, nil
	})
}

// Plugin 接口实现
func (p *WPSPlugin) Initialize(ctx context.Context) error {
	if ctx == p.ctx {
		return nil // 避免重复初始化
	}
	p.ctx = ctx
	p.cfg = config.FromContext(ctx)
	settings, err := config.GetPluginSettingsFor[WPSPluginConfig](p.cfg, p.Name())
	if err != nil {
		return fmt.Errorf("could not get settings for plugin %s: %w", p.Name(), err)
	}
	p.settings = *settings // 将指针解引用，存入
	p.containerManager = service.NewContainerManager()
	p.baseNamer = namer.NewDefaultBaseNamer()
	p.physicalPacker = physical.NewSinglePhysicalPacker()
	return nil
}

// Plugin 接口实现
//
//	返回在 Initialize 阶段已经配置好的 chunkNamer
func (p *WPSPlugin) GetChunkNamer() namer.ChunkNamer {
	return nil
}

// Plugin 接口实现
func (p *WPSPlugin) SupportedMimePrefixes() []string {
	return []string{
		"application/msword", // doc
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document", // docx
		"application/vnd.ms-excel", // xls
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",         // xlsx
		"application/vnd.ms-powerpoint",                                             // ppt
		"application/vnd.openxmlformats-officedocument.presentationml.presentation", // pptx
	}
}

// Plugin 接口实现
func (p *WPSPlugin) SupportedExtensions() []string {
	// 当 MIME 类型无法识别时，通过这些扩展名进行兜底匹配
	return []string{
		"doc",
		"docx",
		"xls",
		"xlsx",
		"ppt",
		"pptx",
	}
}

// Plugin 接口实现
func (p *WPSPlugin) ShouldProcess(inputPath string) bool {
	return true
}

//	Plugin 接口实现
//
// 判断此插件是否能解密给定的容器文件。
// 【关键修复】不再依赖不可靠的文件扩展名，而是通过读取容器元数据来判断其内容类型。
func (p *WPSPlugin) CanDecrypt(containerPath string) bool {
	kind, err := detector.DetectIndexKind(containerPath)
	if err != nil {
		// 如果无法判断类型（例如，文件损坏或不是 ENCV 容器），则认为不能解密
		// 这里的日志可以帮助调试
		// log.Printf("DEBUG: [IframePlugin.CanDecrypt] Failed to detect kind for '%s': %v\n", containerPath, err)
		return false
	}
	return kind == IndexKindWPS
}

// 实现 plugins.Plugin 接口
func (p *WPSPlugin) GetMetadataExtractor() pluginInterfaces.MetadataExtractor {
	return &WPSMetadataExtractor{}
}

// 实现 plugins.Plugin 接口
func (p *WPSPlugin) GetContentPreprocessor() pluginInterfaces.ContentPreprocessor {
	return &WPSContentPreprocessor{}
}

// 实现 plugins.Plugin 接口
func (p *WPSPlugin) GetContentVirifier() pluginInterfaces.ContentVerifier {
	return nil
}

func (p *WPSPlugin) GroupFiles(inputPaths []string, inputRootDir, outputDir string) ([]string, error) {
	return inputPaths, nil
}

func (p *WPSPlugin) ContainerType() uint16 {
	return types.ContainerTypeDocument
}

func (p *WPSPlugin) DefaultIsSeekable(inputPath string) bool {
	return false
}

func (p *WPSPlugin) DisasterZones(inputPath string) []types.DisasterZone {
	return nil
}

// --- 加密逻辑 ---

// Plugin 接口实现
// 在加密前处理，并更新 Index
func (p *WPSPlugin) PreEncryptProcessor(index types.Index, inputPath, inputRootDir, outputDir string) error {
	vIndex, ok := index.(*WPSIndex)
	if !ok {
		return fmt.Errorf("[%s] plugin received a non-%s index", p.Name(), p.Name())
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	p.index = *vIndex
	p.inputPath = inputPath
	p.inputRootDir = inputRootDir
	p.outputDir = outputDir
	return nil
}

// Plugin 接口实现
// 执行核心的加密工作
func (p *WPSPlugin) Encrypt(dataReader io.Reader) (*crypto.EncryptionResult, error) {
	guardKey := fmt.Sprintf("%s|%s", p.inputPath, p.outputDir)
	var result *crypto.EncryptionResult

	err := utils.Do(guardKey, func() error {
		// 1. 执行加密
		// crypto.EncryptToTempFile 会读取 dataReader 并生成带有 Salt/IV 头的临时文件
		var err error
		result, err = crypto.EncryptToTempFile_v2(dataReader, p.cfg.Password, p.outputDir)
		if err != nil {
			return fmt.Errorf("failed to encrypt to temp file: %w", err)
		}

		log.Printf("INFO: [%s] Encrypted to temporary file: %s (Payload: %d bytes)\n", p.Name(), result.TempPath, result.EncryptedPayloadSize)
		log.Printf("✅ [%s] Encrypted successfully.\n", p.Name())
		return nil
	})

	return result, err
}

// Plugin 接口实现
// 加密后处理器
func (p *WPSPlugin) PostEncryptProcessor(result *crypto.EncryptionResult) error {
	// 1. 【性能优化】使用传入的 result 中的精确大小
	logicalDataSize := result.EncryptedPayloadSize

	// 2. 生成逻辑分片
	logicalFragments, err := fragment.CreateLogicalFragmentsFromSize(logicalDataSize, logicalDataSize, types.FragmentType_AtomicFile)
	if err != nil {
		return fmt.Errorf("failed to create logical fragments from size: %w", err)
	}
	log.Printf("-> [%s] Generated %d logical fragments.\n", p.Name(), len(logicalFragments))

	// 3. 构造 Manifest (使用 result 中的 Salt 和 IV)
	kvi := WPSKVI_v2{
		KVI: types.KVI{
			SaltBase64: crypto.Base64Encode_v2(result.Salt),
			IVBase64:   crypto.Base64Encode_v2(result.IV),
		},
		WPSIndex: &p.index,
	}
	manifest, err := types.NewManifest(kvi, logicalFragments)
	if err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	// 4. 准备通用 PackParams
	encryptedBaseName := p.baseNamer.GenerateEncryptedBaseName(p.index.OriginalFilename)
	finalFilename := encryptedBaseName + p.settings.Ext
	finalBaseName := strings.TrimSuffix(finalFilename, p.settings.Ext)

	// 5. 【重构】直接构造 PackParams
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
		BaseName:      finalBaseName,
		OutputDir:     p.outputDir,
		Index:         &p.index,
		HeaderVersion: 4,
		ContainerType: p.ContainerType(),
		IsSeekable:    p.DefaultIsSeekable(p.inputPath),
		SpecialIDType: types.IDType_Raw,
		SpecialID:     nil,
		FinalFileName: finalFilename,
		// Namer, StartIdx, LightMainChunkEnabled 等在 Single 模式下不需要，
		// Helper 会处理零值，Packer 也会处理零值
	}

	// 6. 调用 Helper
	if err := packer.StandardPostEncrypt(packParams); err != nil {
		// 清理临时文件
		os.Remove(result.TempPath)
		return fmt.Errorf("packing failed: %w", err)
	}

	// 7. 清理临时文件
	os.Remove(result.TempPath)

	log.Printf("✅ [%s] packed successfully.\n", p.Name())
	return nil
}

// --- 解密逻辑 ---

// Plugin 接口实现
// 解密前无需额外操作
func (p *WPSPlugin) PreDecryptProcessor(containerPath, outputDir string) error {
	return nil
}

// Plugin 接口实现
func (p *WPSPlugin) Decrypt(containerPath, outputDir string) error {
	log.Printf("DEBUG: [%s] Starting decryption for: %s\n", p.Name(), containerPath)
	p.outputDir = outputDir

	// --- 1. 【关键】通过 ContainerManager 获取一个可读的容器路径 ---
	// ContainerManager 会智能地决定是使用原始文件还是重建文件
	readablePath, err := p.containerManager.GetReadablePath(containerPath, nil)
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
	_, isSeekable := decryptedReader.(io.Seeker)
	if isSeekable {
		log.Printf("INFO: [%s] Container is SEEKABLE. Decrypting full content.\n", p.Name())
	} else {
		log.Printf("INFO: [%s] Container is ATOMIC. Decrypting full content.\n", p.Name())
	}

	// 从 KVI 获取原始文件名
	index := factory.GetIndex()
	vIndex, ok := index.(*WPSIndex)
	if !ok {
		return fmt.Errorf("container is not a imgae container")
	}

	outputPath := filepath.Join(outputDir, vIndex.GetOriginalFilename())
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	if _, err := io.Copy(outputFile, decryptedReader); err != nil {
		return fmt.Errorf("failed to write decrypted imgae stream: %w", err)
	}

	p.index = *vIndex

	log.Printf("✅ [%s] Decrypted to: %s\n", p.Name(), outputPath)
	return nil
}

// Plugin 接口实现
// 在解密后处理
func (p *WPSPlugin) PostDecryptProcessor(containerPath string) error {

	return nil
}
