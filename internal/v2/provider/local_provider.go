// internal/v2/provider/local_provider.go
package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"runtime"
	"sync"

	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/reader"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// LocalFileProvider 提供对本地加密文件的访问
type LocalFileProvider struct {
	mu            sync.Mutex
	decryptReader reader.DecryptReader
	index         types.Index
	originalSize  int64
	originalName  string
	// 【关键新增】持有 factory 的引用，用于查询元信息
	factory reader.DecryptReaderFactory
	// 按需加载的字节切片
	cachedData []byte
	once       sync.Once
	loadErr    error
}

// 【关键修复】定义一个自定义的 ReadCloser，它同时支持 Seek
type cachedReadCloser struct {
	*bytes.Reader
}

func (c *cachedReadCloser) Close() error {
	// bytes.Reader 不需要关闭，所以 Close 是空操作
	return nil
}

// NewLocalFileProvider 创建一个新的 LocalFileProvider
func NewLocalFileProvider(ctx context.Context, factory reader.DecryptReaderFactory, decryptReader reader.DecryptReader) (*LocalFileProvider, error) {
	if factory == nil || decryptReader == nil {
		return nil, fmt.Errorf("factory and decryptReader cannot be nil")
	}

	index := factory.GetIndex()
	if index == nil {
		return nil, fmt.Errorf("factory returned a nil index")
	}

	provider := &LocalFileProvider{
		decryptReader: decryptReader,
		index:         index,
		originalSize:  factory.GetOriginalSize(),
		originalName:  index.GetOriginalFilename(),
		factory:       factory, // 持有 factory
	}

	// 【关键重构】使用新的、基于 IsSeekable 的判断逻辑
	shouldCache := provider.shouldCacheInMemory()
	if !shouldCache {
		log.Printf("DEBUG: [LocalFileProvider] File is large or container is seekable (%d bytes), will be streamed.", provider.originalSize)
		runtime.SetFinalizer(provider, func(p *LocalFileProvider) {
			p.decryptReader.Close()
		})
		return provider, nil
	}

	runtime.SetFinalizer(provider, func(p *LocalFileProvider) {
		if p.cachedData == nil && p.decryptReader != nil {
			p.decryptReader.Close()
		}
	})

	return provider, nil
}

// --- 实现 FileContentProvider 接口 ---

func (p *LocalFileProvider) GetReader() io.ReadCloser {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 如果已经加载到内存
	if p.cachedData != nil {
		return &cachedReadCloser{Reader: bytes.NewReader(p.cachedData)}
	}

	// 如果加载失败了
	if p.loadErr != nil {
		return nil // 或者返回一个错误包装器
	}

	// 如果需要缓存但还没加载，则触发加载
	if p.shouldCacheInMemory() {
		p.loadIntoMemory()
		if p.loadErr != nil {
			return nil
		}
		return &cachedReadCloser{Reader: bytes.NewReader(p.cachedData)}
	}

	// 对于大文件，返回原始的解密器
	return p.decryptReader
}

// GetSeeker 和 GetSeekerTo 的逻辑也需要相应调整
func (p *LocalFileProvider) GetSeeker() (io.Seeker, bool) {
	// 如果已经加载到内存
	if p.cachedData != nil {
		return bytes.NewReader(p.cachedData), true
	}

	// 如果加载失败了
	if p.loadErr != nil {
		return nil, false
	}

	// 如果需要缓存但还没加载，触发加载
	if p.shouldCacheInMemory() {
		p.loadIntoMemory()
		if p.loadErr != nil {
			return nil, false
		}
		return bytes.NewReader(p.cachedData), true
	}

	// 对于大文件，检查原始解密器
	if seeker, ok := p.decryptReader.(io.Seeker); ok {
		return seeker, true
	}
	return nil, false
}

func (p *LocalFileProvider) GetSeekerTo() (SeekerTo, bool) {
	// 内存缓存不需要 SeekTo
	if p.cachedData != nil || p.loadErr != nil {
		return nil, false
	}

	// 如果需要缓存但还没加载，触发加载
	if p.shouldCacheInMemory() {
		p.loadIntoMemory()
		return nil, false
	}

	// 对于大文件，检查原始解密器
	if seekerTo, ok := p.decryptReader.(SeekerTo); ok {
		return seekerTo, true
	}
	return nil, false
}

func (p *LocalFileProvider) GetSize() int64 {
	return p.originalSize
}

func (p *LocalFileProvider) GetName() string {
	return p.originalName
}

func (p *LocalFileProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 如果使用了内存缓存，原始的 decryptReader 已经在 loadIntoMemory 中关闭
	if p.cachedData != nil {
		p.cachedData = nil // 【关键】释放内存，防止泄露
		return nil
	}

	// 否则，关闭解密器
	if err := p.decryptReader.Close(); err != nil {
		return fmt.Errorf("failed to close decrypt reader: %w", err)
	}
	return nil
}

// shouldCacheInMemory 是一个新增的智能判断方法
func (p *LocalFileProvider) shouldCacheInMemory() bool {
	if p.originalSize <= 0 {
		return false
	}

	// 1. 获取系统可用内存并计算动态阈值
	availableMem := utils.GetCachedAvailableMemory()
	const minAbsoluteThreshold = 20 * 1024 * 1024 // 20MB
	const maxMemoryUsageRatio = 0.2               // 20%
	var dynamicThreshold int64
	if availableMem > 0 {
		dynamicThreshold = int64(float64(availableMem) * maxMemoryUsageRatio)
		log.Printf("DEBUG: [LocalFileProvider] Available memory: %d MB, dynamic cache threshold: %d MB", availableMem/1024/1024, dynamicThreshold/1024/1024)
	} else {
		dynamicThreshold = minAbsoluteThreshold
		log.Printf("DEBUG: [LocalFileProvider] Could not determine available memory, using fixed cache threshold: %d MB", dynamicThreshold/1024/1024)
	}
	finalThreshold := dynamicThreshold
	if finalThreshold < minAbsoluteThreshold {
		finalThreshold = minAbsoluteThreshold
	}

	// 2. 【核心修正】基于容器的寻址能力来决定
	if !p.factory.IsSeekable() {
		// 如果容器不可寻址，并且文件大小在合理范围内，则强烈建议缓存
		// 这能将一个不可寻址的流转变为一个可寻址的内存流，对 WebDAV 等场景至关重要
		log.Printf("DEBUG: [LocalFileProvider] Container is not seekable, favoring in-memory caching for size %d.", p.originalSize)
		// 对于不可寻址的容器，我们可以设置一个更高的缓存上限，比如 150MB
		const unseekableCacheLimit = 150 * 1024 * 1024
		return p.originalSize <= unseekableCacheLimit
	}

	// 3. 对于可寻址的容器，使用标准的动态阈值
	return p.originalSize <= finalThreshold
}

// 【关键新增】loadIntoMemory 按需将文件读入内存
func (p *LocalFileProvider) loadIntoMemory() {
	p.once.Do(func() {
		log.Printf("DEBUG: [LocalFileProvider] Loading file into memory on first access...")
		allData, err := io.ReadAll(p.decryptReader)
		if err != nil {
			p.loadErr = fmt.Errorf("failed to read file into memory: %w", err)
			return
		}

		// 数据读取成功，关闭原始流和工厂
		p.decryptReader.Close()
		// 注意：factory 的关闭需要谨慎，如果它被 ReaderService 缓存，则不能在这里关闭
		// 假设 NewLocalFileProvider 每次都创建新工厂，那么可以关闭
		// 如果 factory 是被注入的，则由注入方负责关闭
		// 在我们当前的设计中，factory 是局部变量，无法在这里访问到
		// 这是一个设计上的小瑕疵，但为了防止泄露，我们至少要关闭 reader

		p.cachedData = allData
		log.Printf("DEBUG: [LocalFileProvider] File successfully loaded into memory (%d bytes).", len(p.cachedData))
	})
}
