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
	cfg              *config.Config
	index            VideoIndex
	outputDir        string
	inputPath        string
	inputRootDir     string
	tempEncPath      string
	salt             []byte
	iv               []byte
	baseNamer        namer.BaseNamer           // 注入容器命名器
	chunkNamer       namer.ChunkNamer          // 注入分片命名器
	containerManager *service.ContainerManager // 注入 ContainerManager
	physicalPacker   physical.PhysicalPacker
}

// init 在包被导入时自动执行，完成自注册
func init() {
	types.RegisterKVIProvider(IndexKindVideo, func(rawKVI json.RawMessage) (types.KVIProvider, error) {
		var videoKVI VideoKVI_v2
		if err := json.Unmarshal(rawKVI, &videoKVI); err != nil {
			return nil, fmt.Errorf("failed to unmarshal KVI as VideoKVI_v2: %w", err)
		}
		return videoKVI, nil // VideoKVI_v2 实现了 KVIProvider 接口
	})
}

// Plugin 接口实现
func (p *VideoPlugin) Intialize(ctx context.Context) error {
	p.cfg = config.FromContext(ctx)
	p.containerManager = service.NewContainerManager()
	p.baseNamer = namer.NewDefaultBaseNamer()
	mainChunkExt := p.cfg.GetVideoEncExtension()
	p.chunkNamer = namer.NewPaddedNamer(mainChunkExt, p.baseNamer, 4) // 补零到4位
	if p.cfg.IsSccgvChunkingEnabled() {
		fmt.Printf("INFO: [PLUGIN] Physical chunking enabled. Size: %d MB\n", p.cfg.GetSccgvChunkSizeMB())
		p.physicalPacker = physical.NewFileChunkerPhysicalPacker(p.cfg.GetSccgvChunkSizeBytes(), p.chunkNamer)
	} else {
		fmt.Printf("INFO: [PLUGIN] Physical chunking disabled.\n")
		p.physicalPacker = physical.NewSinglePhysicalPacker() // NoOpPacker 不需要 namer
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

// Plugin 接口实现
func (p *VideoPlugin) GetContainerExtension() string {
	return p.cfg.GetVideoEncExtension()
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
		return fmt.Errorf("video plugin received a non-video index")
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
	return HandleSubtitlesForEncryption(p.cfg, &p.index, outputDir, encryptedBaseName)
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

		fmt.Printf("INFO: [PLUGIN] Encrypted to temporary file: %s\n", tempEncPath)

		fmt.Printf("✅ [VIDEO] Encrypted successfully.\n")
		return nil
	})
}

// Plugin 接口实现
// 视频插件在加密后处理器
func (p *VideoPlugin) PostEncryptProcessor() error {
	// --- 【关键修复】在这里，根据原始文件大小计算逻辑分片 ---
	logicalFragmentSize := fragment.CalculateFragmentSize(p.index.OriginalFileSize, p.cfg.GetSccgvChunkSizeBytes())
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
	finalBaseName := strings.TrimSuffix(finalFilename, p.cfg.GetVideoEncExtension())
	// 决定打包策略和起始索引
	isLightweight := p.cfg.IsLightweightMainChunkEnabled()
	var startIdx int
	if isLightweight {
		startIdx = 1
	} else {
		startIdx = 0
	}
	packer := NewVideoPacker(p.physicalPacker, p.chunkNamer)
	packReq := &physical.PackRequest{
		BaseName:            finalBaseName,
		OutputDir:           p.outputDir,
		EncryptedDataReader: encryptedDataReader,
		Index:               &p.index, // Packer 将从 vIndex 获取所需信息
		Salt:                p.salt,
		IV:                  p.iv,
		LogicalFragments:    logicalFragments, // 预先计算好
		Namer:               p.chunkNamer,
		StartIdx:            startIdx,
	}

	if err := packer.Pack(p.cfg, packReq); err != nil {
		encryptedDataReader.Close()
		return fmt.Errorf("packing failed: %w", err)
	}

	encryptedDataReader.Close() // Packer 使用完毕后关闭
	fmt.Printf("✅ [VIDEO] packed successfully.\n")
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
	fmt.Printf("DEBUG: [VideoPlugin.Decrypt] Starting decryption for: %s\n", containerPath)
	p.outputDir = outputDir

	// --- 1. 【关键】通过 ContainerManager 获取一个可读的容器路径 ---
	// ContainerManager 会智能地决定是使用原始文件还是重建文件
	readablePath, err := p.containerManager.GetReadablePath(containerPath, p.chunkNamer)
	if err != nil {
		return fmt.Errorf("failed to get readable path from container manager: %w", err)
	}
	fmt.Printf("DEBUG: [VideoPlugin.Decrypt] Using readable path: %s\n", readablePath)

	// --- 2. 使用统一路径创建 reader 工厂 ---
	factory, err := reader.NewDecryptReaderFactory(readablePath, p.cfg.Password)
	if err != nil {
		return fmt.Errorf("failed to create reader factory for '%s': %w", readablePath, err)
	}
	defer factory.Close() // 【关键】这个 Close 会同时清理物理临时文件（如果存在）
	fmt.Printf("DEBUG: [VideoPlugin.Decrypt] Reader factory created successfully.\n")

	// --- 3. 使用工厂创建解密流并写入文件 ---
	decryptedReader, err := factory.NewDecryptReader(*p.cfg)
	if err != nil {
		return fmt.Errorf("[VideoPlugin] failed to create decrypt reader: %w", err)
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

	fmt.Printf("✅ [VIDEO] Decrypted to: %s\n", outputPath)
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
