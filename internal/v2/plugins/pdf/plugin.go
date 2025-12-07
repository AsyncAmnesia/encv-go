package pdf

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

type PDFPlugin struct {
	cfg              *config.Config
	settings         PDFPluginConfig
	index            PDFIndex
	outputDir        string
	inputPath        string
	inputRootDir     string
	tempEncPath      string
	salt             []byte
	iv               []byte
	baseNamer        namer.BaseNamer           // 注入容器命名器
	containerManager *service.ContainerManager // 注入 ContainerManager
	physicalPacker   physical.PhysicalPacker
}

func (p *PDFPlugin) Name() string {
	return "pdf" // 这个字符串必须与配置文件中的键对应
}

// Plugin 接口实现
func (p *PDFPlugin) GetContainerExtension() string {
	return p.settings.Ext
}

type PDFPluginConfig struct {
	// 容器扩展名，包含点前缀，默认值为 ".sccgpdf"
	Ext string `json:"ext"`
}

func (p *PDFPlugin) GetSettingsSchemaType() interface{} {
	return PDFPluginConfig{}
}

// 2. 实现接口方法，返回默认配置的 JSON
func (p *PDFPlugin) GetDefaultSettings() json.RawMessage {
	defaultCfg := PDFPluginConfig{
		Ext: ".sccgpdf",
	}
	data, _ := json.Marshal(defaultCfg) // 忽略错误，因为默认值是硬编码的，不会出错
	return data
}

// init 在包被导入时自动执行，完成自注册
func init() {
	types.RegisterKVIProvider(IndexKindPDF, func(rawKVI json.RawMessage) (types.KVIProvider, error) {
		var kvi PDFKVI_v2
		if err := json.Unmarshal(rawKVI, &kvi); err != nil {
			return nil, fmt.Errorf("failed to unmarshal KVI as PDFKVI_v2: %w", err)
		}
		return kvi, nil
	})
}

// Plugin 接口实现
func (p *PDFPlugin) Intialize(ctx context.Context) error {
	p.cfg = config.FromContext(ctx)
	settings, err := config.GetPluginSettingsFor[PDFPluginConfig](p.cfg, p.Name())
	if err != nil {
		return fmt.Errorf("could not get settings for plugin %s: %w", p.Name(), err)
	}
	p.settings = *settings // 将指针解引用，存入
	p.containerManager = service.NewContainerManager()
	p.baseNamer = namer.NewDefaultBaseNamer()
	p.physicalPacker = physical.NewSinglePhysicalPacker() // NoOpPacker 不需要 namer
	return nil
}

// Plugin 接口实现
//
//	返回在 Initialize 阶段已经配置好的 chunkNamer
func (p *PDFPlugin) GetChunkNamer() namer.ChunkNamer {
	return nil
}

// Plugin 接口实现
func (p *PDFPlugin) SupportedMimePrefixes() []string {
	return []string{
		"application/pdf", // pdf
	}
}

// Plugin 接口实现
func (p *PDFPlugin) ShouldProcess(inputPath string) bool {
	return true
}

//	Plugin 接口实现
//
// 判断此插件是否能解密给定的容器文件。
// 【关键修复】不再依赖不可靠的文件扩展名，而是通过读取容器元数据来判断其内容类型。
func (p *PDFPlugin) CanDecrypt(containerPath string) bool {
	kind, err := detector.DetectIndexKind(containerPath)
	if err != nil {
		// 如果无法判断类型（例如，文件损坏或不是 ENCV 容器），则认为不能解密
		// 这里的日志可以帮助调试
		// fmt.Printf("DEBUG: [IframePlugin.CanDecrypt] Failed to detect kind for '%s': %v\n", containerPath, err)
		return false
	}
	return kind == IndexKindPDF
}

// 【新增方法】实现 plugins.Plugin 接口
func (p *PDFPlugin) GetMetadataExtractor() pluginInterfaces.MetadataExtractor {
	return &PDFMetadataExtractor{}
}

// 【新增方法】实现 plugins.Plugin 接口
func (p *PDFPlugin) GetContentPreprocessor() pluginInterfaces.ContentPreprocessor {
	return &PDFContentPreprocessor{}
}

// --- 加密逻辑 ---

// Plugin 接口实现
// 在加密前处理字幕，并更新 Index
func (p *PDFPlugin) PreEncryptProcessor(index types.Index, inputPath, inputRootDir, outputDir string) error {
	vIndex, ok := index.(*PDFIndex)
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
	return nil
}

// Plugin 接口实现
// 执行核心的加密工作，并调用 Packer
func (p *PDFPlugin) Encrypt(dataReader io.Reader) error {
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
func (p *PDFPlugin) PostEncryptProcessor() error {
	// --- 【关键修复】在这里，根据原始文件大小计算逻辑分片 ---
	logicalFragmentSize := fragment.CalculateFragmentSize(p.index.OriginalFileSize, 0)
	logicalFragments, err := fragment.CreateLogicalFragmentsFromSize(p.index.OriginalFileSize, logicalFragmentSize, types.FragmentType_AtomicFile)
	if err != nil {
		return fmt.Errorf("failed to create logical fragments from size: %w", err)
	}
	// 打印出生成的片段数量，用于调试
	fmt.Printf("-> [%s] Generated %d logical fragments.\n", p.Name(), len(logicalFragments))

	// 重新打开临时文件，作为加密数据源传递给 Packer
	encryptedDataReader, err := os.Open(p.tempEncPath)
	if err != nil {
		return fmt.Errorf("failed to open temp file for packing: %w", err)
	}
	defer os.Remove(p.tempEncPath) // 确保在使用完毕后删除临时文件

	// --- 5. 创建 Packer 并执行打包 ---
	encryptedBaseName := p.baseNamer.GenerateEncryptedBaseName(p.index.OriginalFilename)
	finalBaseName := strings.TrimSuffix(encryptedBaseName, p.settings.Ext)
	packer := NewPDFPacker(p.physicalPacker)
	packReq := &physical.PackRequest{
		BaseName:            finalBaseName,
		OutputDir:           p.outputDir,
		EncryptedDataReader: encryptedDataReader,
		Index:               &p.index, // Packer 将从 vIndex 获取所需信息
		Salt:                p.salt,
		IV:                  p.iv,
		LogicalFragments:    logicalFragments, // 预先计算好
		FinalFileName:       encryptedBaseName + p.settings.Ext,
	}

	if err := packer.Pack(p.cfg, packReq); err != nil {
		encryptedDataReader.Close()
		return fmt.Errorf("packing failed: %w", err)
	}

	encryptedDataReader.Close() // Packer 使用完毕后关闭
	fmt.Printf("✅ [%s] packed successfully.\n", p.Name())
	return nil
}

// --- 解密逻辑 ---

// Plugin 接口实现
// 视频插件在解密前无需额外操作
func (p *PDFPlugin) PreDecryptProcessor(containerPath, outputDir string) error {
	// 视频插件在此阶段无需操作
	return nil
}

// Plugin 接口实现
func (p *PDFPlugin) Decrypt(containerPath, outputDir string) error {
	fmt.Printf("DEBUG: [%s] Starting decryption for: %s\n", p.Name(), containerPath)
	p.outputDir = outputDir

	// --- 1. 【关键】通过 ContainerManager 获取一个可读的容器路径 ---
	// ContainerManager 会智能地决定是使用原始文件还是重建文件
	readablePath, err := p.containerManager.GetReadablePath(containerPath, nil)
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
	decryptedReader, err := factory.NewDecryptReader(*p.cfg)
	if err != nil {
		return fmt.Errorf("[%s] failed to create decrypt reader: %w", p.Name(), err)
	}
	defer decryptedReader.Close()
	_, isSeekable := decryptedReader.(io.Seeker)
	if isSeekable {
		fmt.Printf("INFO: [%s] Container is SEEKABLE. Decrypting full content.\n", p.Name())
	} else {
		fmt.Printf("INFO: [%s] Container is ATOMIC. Decrypting full content.\n", p.Name())
	}

	// 从 KVI 获取原始文件名
	index := factory.GetIndex()
	vIndex, ok := index.(*PDFIndex)
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

	fmt.Printf("✅ [%s] Decrypted to: %s\n", p.Name(), outputPath)
	return nil
}

// Plugin 接口实现
// 在解密后处理字幕还原
func (p *PDFPlugin) PostDecryptProcessor(containerPath string) error {

	return nil
}
