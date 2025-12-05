// internal/v2/reader/file_container_reader.go

package reader

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/Soltus/encv-go/internal/v2/container/block"
	"github.com/Soltus/encv-go/internal/v2/container/manifest"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// fileContainerReader 是 EncryptedContainerReader 接口的一个健壮、自适应的具体实现。
// 它负责从单个或多个文件中读取原始的、加密的数据块，并具备强大的错误恢复能力。
type fileContainerReader struct {
	// 核心元数据，在构造时解析并缓存
	manifest     *types.Manifest_v2
	footer       *types.EnvelopeFooter_v2 // 可能为 nil
	kviProvider  types.KVIProvider
	containerDir string
	mainFilePath string

	// 运行时状态，用于物理偏移映射和外部文件缓存
	mu                sync.RWMutex
	physicalOffsets   map[string]uint64
	openExternalFiles map[string]*os.File // 缓存已打开的外部文件句柄
}

// NewEncryptedContainerReaderFromFile 是创建 EncryptedContainerReader 的入口。
// 它会解析并缓存所有必要的元数据，后续操作将基于这些缓存数据进行，非常高效。
func NewEncryptedContainerReaderFromFile(mainFilePath string) (EncryptedContainerReader, error) {
	log.Printf("DEBUG: [fileContainerReader] Initializing for: %s", mainFilePath)

	r := &fileContainerReader{
		mainFilePath:      mainFilePath,
		containerDir:      filepath.Dir(mainFilePath),
		physicalOffsets:   make(map[string]uint64),
		openExternalFiles: make(map[string]*os.File),
	}

	// 【核心移植】源自旧代码的健壮逻辑：带回退的 Manifest 读取
	manifest, footer, err := readManifestWithFallback(mainFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest from '%s': %w", mainFilePath, err)
	}
	r.manifest = manifest
	r.footer = footer

	kviProvider, err := types.NewKVIProviderFromManifest(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal KVI from manifest: %w", err)
	}
	r.kviProvider = kviProvider

	// 【核心移植】源自旧代码的智能逻辑：自适应物理偏移扫描
	if err := r.scanPhysicalOffsets(); err != nil {
		return nil, fmt.Errorf("failed to scan physical offsets: %w", err)
	}

	log.Printf("INFO: [fileContainerReader] Initialization complete.")
	return r, nil
}

// NewFileContainerReaderFromMetadata 是一个新的、轻量级的构造函数。
// 它使用预先解析好的 manifest 和 physicalOffsets 来创建 reader，避免了重复的文件扫描。
func NewFileContainerReaderFromMetadata(mainFilePath string, manifest *types.Manifest_v2, physicalOffsets map[string]uint64) (*fileContainerReader, error) {
	kviProvider, err := types.NewKVIProviderFromManifest(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal KVI from manifest: %w", err)
	}

	r := &fileContainerReader{
		mainFilePath:      mainFilePath,
		containerDir:      filepath.Dir(mainFilePath),
		manifest:          manifest,
		kviProvider:       kviProvider,
		physicalOffsets:   physicalOffsets, // 【关键】直接使用传入的 map，不扫描
		openExternalFiles: make(map[string]*os.File),
	}

	return r, nil
}

// GetManifest 返回已解析的容器清单。
func (r *fileContainerReader) GetManifest() *types.Manifest_v2 {
	return r.manifest
}

// GetKVIProvider 返回解析后的 KVI provider
func (r *fileContainerReader) GetKVIProvider() (types.KVIProvider, error) {
	return types.NewKVIProviderFromManifest(r.manifest)
}

func (r *fileContainerReader) GetFragments() []types.Fragment_v2 {
	return r.manifest.Fragments
}

// GetFragmentReader 根据 Fragment ID，返回一个读取该 Fragment 原始加密数据的 io.ReadCloser。
// 此方法是线程安全的，并集成了数据校验和错误恢复机制。

func (r *fileContainerReader) GetFragmentReader(fragID string) (io.ReadCloser, error) {
	frag, err := r.findFragmentByID(fragID)
	if err != nil {
		return nil, err
	}

	headerSize := int64(binary.Size(block.BlockHeader_v2{}))

	// 情况 1: 数据在主文件中
	if frag.PhysicalPath == "" {
		// 【关键修复】这个分支现在只处理单文件容器
		blockHeaderOffset, ok := r.physicalOffsets[fragID]
		if !ok {
			return nil, fmt.Errorf("physical data offset for fragment '%s' not found in main file map. Is this a physical chunked container?", fragID)
		}
		// ... (后续逻辑不变，从主文件读取)
		mainFile, err := globalFileHandlePool.Get(r.mainFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to get main file handle from pool: %w", err)
		}
		if err := r.verifyFragmentAt(mainFile, int64(blockHeaderOffset), frag); err != nil {
			globalFileHandlePool.Put(mainFile)
			return nil, fmt.Errorf("fragment '%s' in main file is corrupt: %w", fragID, err)
		}
		dataStartOffset := int64(blockHeaderOffset) + headerSize
		section := io.NewSectionReader(mainFile, dataStartOffset, int64(frag.Length))
		return &pooledFileHandleWrapper{ReadCloser: io.NopCloser(section), file: mainFile}, nil
	}

	// 情况 2: 数据在外部文件中（物理分片）
	// 这个分支现在专门处理物理分片容器
	expectedChunkPath := filepath.Join(r.containerDir, frag.PhysicalPath)
	extFile, err := globalFileHandlePool.Get(expectedChunkPath)
	if err == nil {
		// 打开成功 -> 验证
		if err := r.verifyFragmentAt(extFile, 0, frag); err == nil {
			section := io.NewSectionReader(extFile, headerSize, int64(frag.Length))
			return &pooledFileHandleWrapper{ReadCloser: io.NopCloser(section), file: extFile}, nil
		}
		// 验证失败，归还句柄，进入恢复模式
		globalFileHandlePool.Put(extFile)
		log.Printf("WARN: fragment '%s' at expected path '%s' is corrupt: %v", fragID, expectedChunkPath, err)
	} else {
		log.Printf("WARN: fragment '%s' at expected path '%s' cannot be opened: %v", fragID, expectedChunkPath, err)
	}

	// 进入恢复模式
	recoveredFile, err := r.findAndOpenFragmentRecovery(frag)
	if err != nil {
		return nil, err
	}
	section := io.NewSectionReader(recoveredFile, headerSize, int64(frag.Length))
	return &pooledFileHandleWrapper{ReadCloser: io.NopCloser(section), file: recoveredFile}, nil
}

// Close 关闭容器及其打开的所有外部资源。
func (r *fileContainerReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var combinedErr error
	for path, f := range r.openExternalFiles {
		if f == nil {
			continue
		}
		if err := f.Close(); err != nil {
			if combinedErr == nil {
				combinedErr = fmt.Errorf("failed to close external file %s: %w", path, err)
			} else {
				combinedErr = fmt.Errorf("%v; failed to close external file %s: %w", combinedErr, path, err)
			}
		}
		// 【关键集成】关闭时也要通过句柄池
		globalFileHandlePool.Put(f)
	}
	r.openExternalFiles = make(map[string]*os.File)
	return combinedErr
}

// --- 以下是所有移植自旧代码的、经过微调的私有辅助函数 ---

func (r *fileContainerReader) findFragmentByID(fragID string) (*types.Fragment_v2, error) {
	for _, frag := range r.manifest.Fragments {
		if frag.ID == fragID {
			return &frag, nil
		}
	}
	return nil, fmt.Errorf("fragment with ID '%s' not found in manifest", fragID)
}

// readManifestWithFallback 尝试从 Footer 读取，失败则扫描整个文件
func readManifestWithFallback(mainFilePath string) (*types.Manifest_v2, *types.EnvelopeFooter_v2, error) {
	_manifest, footer, err := manifest.ReadManifestFromFile(mainFilePath)
	if err == nil {
		log.Printf("DEBUG: Read manifest from footer successfully.")
		return _manifest, footer, nil
	}
	log.Printf("WARN: Footer is invalid, falling back to scan. Reason: %v", err)
	_manifest, err = manifest.ScanManifestFromFile(mainFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("fallback scan also failed: %w", err)
	}
	log.Printf("INFO: Scan successful. Found manifest without a valid footer.")
	return _manifest, nil, nil
}

// findManifestBlockOffset 是一个辅助函数，用于找到 Manifest 块的起始偏移量。
// 它会从文件开头扫描，直到找到第一个 BlockTypeManifest_v2 类型的块。
// 注意：此函数不负责读取 Manifest 数据，只返回其位置。
func (r *fileContainerReader) findManifestBlockOffset() (int64, error) {
	// 【关键修改 1】从全局文件句柄池获取文件句柄
	fileHandle, err := globalFileHandlePool.Get(r.mainFilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file handle from pool for manifest scan: %w", err)
	}
	// 【关键修改 2】使用 defer 确保文件句柄被归还，防止泄漏
	defer globalFileHandlePool.Put(fileHandle)

	// 【关键修改 3】所有文件操作都作用于从池中借来的句柄
	if _, err := fileHandle.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("failed to seek to start for manifest scan: %w", err)
	}

	for {
		// 获取当前偏移量
		currentOffset, err := fileHandle.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, fmt.Errorf("failed to get current offset during manifest scan: %w", err)
		}

		// 读取块头
		header, err := block.ReadBlockHeader_v2(fileHandle)
		if err != nil {
			// 如果读到文件末尾还没找到，也是一种明确的错误
			if err == io.EOF {
				return 0, fmt.Errorf("reached end of file without finding a manifest block")
			}
			return 0, fmt.Errorf("failed to read block header during scan: %w", err)
		}

		// 检查是否是 Manifest 块
		if header.Type == types.BlockTypeManifest_v2 {
			// 找到了，返回其起始偏移量
			return currentOffset, nil
		}

		// 不是 Manifest 块，跳过其数据部分，继续扫描
		if _, err := fileHandle.Seek(int64(header.Length), io.SeekCurrent); err != nil {
			return 0, fmt.Errorf("failed to skip block data at offset %d: %w", currentOffset, err)
		}
	}
}
