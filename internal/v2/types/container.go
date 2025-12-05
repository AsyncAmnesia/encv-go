package types

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	// L1: Envelope

	EnvelopeHeaderSize_v2 = 16
	EnvelopeFooterSize_v2 = 32

	// L2/L3/L4: Block
	BlockTypeManifest_v2 uint16 = 0x0001
	BlockTypeKVI_v2      uint16 = 0x0002
	BlockTypeData_v2     uint16 = 0x0003
	BlockTypeRecovery_v2 uint16 = 0x0004

	// Crypto
	SaltSize_v2 = 16
	KeySize_v2  = 32
	IVSize_v2   = 16
)

var (
	ByteOrder_v2       = binary.LittleEndian
	ErrInvalidMagic_v2 = errors.New("invalid magic number")
	// 将数组转换为切片用法 types.MagicFooter_v2[:]
	MagicHeader_v2 = [4]byte{'E', 'N', 'V', 'C'}
	MagicFooter_v2 = [4]byte{'E', 'N', 'V', 'C'}
)

// FragmentType_v2 定义分片的用途类型，使其意图更加明确
type FragmentType_v2 string

const (
	// FragmentType_Metadata 用于存储 KVI 等元数据，通常只有一个
	FragmentType_Metadata FragmentType_v2 = "metadata"
	// FragmentType_SeekableStream 用于存储可寻址的数据流，如视频、大型日志文件
	// 支持随机访问，其 GlobalStartOffset 字段有效
	FragmentType_SeekableStream FragmentType_v2 = "seekable_stream"
	// FragmentType_AtomicFile 用于存储不可分割的原子文件，如文档、图片、数据库
	// 必须完整读取，其 GlobalStartOffset 字段无效
	FragmentType_AtomicFile FragmentType_v2 = "atomic_file"
)

// Fragment_v2 定义了清单中的一个分片项
type Fragment_v2 struct {
	ID       string          `json:"id"`                 // 唯一标识符
	Type     FragmentType_v2 `json:"type"`               // 【更新】分片类型，使用枚举
	Filename string          `json:"filename,omitempty"` // 可选，用于外部文件
	Length   uint64          `json:"length"`             // 数据长度（字节）

	// GlobalStartOffset 仅在 Type 为 FragmentType_SeekableStream 时有效。
	// 它表示该分片在整个虚拟数据流中的起始字节位置，用于 O(1) 寻址。
	GlobalStartOffset uint64 `json:"global_start_offset,omitempty"`
	PhysicalPath      string `json:"physical_path,omitempty"` // 作为提示，但不是唯一依据

	// 【关键新增】该片段对应加密数据块的 CRC32 校验和
	// 这是验证物理文件是否正确的“指纹”，与文件名无关
	DataCRC32 uint32 `json:"data_crc32"`
}

type EnvelopeHeader_v2 struct {
	Magic    [4]byte
	Version  uint16
	Flags    uint16
	Reserved [8]byte
}

type EnvelopeFooter_v2 struct {
	Magic          [4]byte
	ManifestOffset uint64
	ManifestLength uint64
	ManifestCRC32  uint32
	GlobalCRC32    uint32
	Reserved       [4]byte
}

// ContainerDescriptor 描述了一个容器的元信息
type ContainerDescriptor struct {
	FilePath   string
	IsSeekable bool
}

// KVIProvider 定义了从 KVI 数据中获取所需信息的通用接口
// Manifest_v2 将依赖此接口，而不是具体的结构体，从而实现解耦
type KVIProvider interface {
	// 获取索引类型，用于快速路由和判断
	GetKind() IndexKind
	// GetEncryptionInfo 返回用于序列化到 Manifest 的 KVI 数据
	GetEncryptionInfo() KVI_v2

	// GetIndex 返回实现了 types.Index 接口的文件索引，供上层服务使用
	GetIndex() Index
}

// KVI_v2 加密信息
type KVI_v2 struct {
	SaltBase64 string `json:"salt_base64"`
	IVBase64   string `json:"iv_base64"`
}

// Manifest_v2 容器清单，这是json的最外层
type Manifest_v2 struct {
	Version int64 `json:"version"`
	// Kind 现在是 Manifest 的顶级字段，用于标识 KVI 类型
	Kind IndexKind `json:"kind"`
	// KVI 字段持有原始的 JSON 数据，其具体类型由上层处理
	KVI        json.RawMessage `json:"kvi"` // index在里面
	Fragments  []Fragment_v2   `json:"fragments"`
	Redundancy struct {
		KVIBackupCRC string `json:"kvi_backup_crc,omitempty"`
	} `json:"redundancy,omitempty"`
}

// SerializeToJSON_v2 将清单序列化为 JSON 字节
func (m *Manifest_v2) SerializeToJSON_v2() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// GetFragmentByID 根据 ID 查找并返回一个 Fragment 的副本
func (m *Manifest_v2) GetFragmentByID(id string) (*Fragment_v2, error) {
	for _, frag := range m.Fragments {
		if frag.ID == id {
			// 返回一个副本以防止外部修改
			return &frag, nil
		}
	}
	return nil, fmt.Errorf("fragment with ID '%s' not found in manifest", id)
}

// KVIProviderFactory 是一个工厂函数，它知道如何从原始 JSON 数据创建一个特定的 KVIProvider
type KVIProviderFactory func(rawKVI json.RawMessage) (KVIProvider, error)

// kviProviderRegistry 是一个私有的中央注册表，用于存储所有已注册的 KVI 提供者工厂
var kviProviderRegistry = make(map[IndexKind]KVIProviderFactory)

// RegisterKVIProvider 允许外部插件注册自己。
// 通常在插件的 init() 函数中调用。
// 如果重复注册同一个 Kind，将会 panic，以帮助开发者及早发现配置错误。
func RegisterKVIProvider(kind IndexKind, factory KVIProviderFactory) {
	if _, exists := kviProviderRegistry[kind]; exists {
		panic(fmt.Sprintf("attempted to register duplicate KVI provider for kind: %s", kind))
	}
	kviProviderRegistry[kind] = factory
}

// NewKVIProviderFromManifest 【重构】使用注册表动态创建 KVIProvider
// 现在这个函数是通用的，不再需要为每个新插件修改它。
func NewKVIProviderFromManifest(manifest *Manifest_v2) (KVIProvider, error) {
	factory, exists := kviProviderRegistry[manifest.Kind]
	if !exists {
		// 提供一个友好的错误信息，列出所有已注册的 Kind
		registeredKinds := make([]string, 0, len(kviProviderRegistry))
		for k := range kviProviderRegistry {
			registeredKinds = append(registeredKinds, string(k))
		}
		return nil, fmt.Errorf("unsupported or unknown index kind: '%s'. Registered kinds are: %v", manifest.Kind, registeredKinds)
	}

	// 调用对应插件注册的工厂函数来创建实例
	return factory(manifest.KVI)
}

// NewKVIProviderFromManifest 是一个便利函数，用于从 Manifest 实例中直接解析 KVIProvider
// func NewKVIProviderFromManifest(manifest *Manifest_v2) (KVIProvider, error) {
// 	switch manifest.Kind {
// 	case IndexKindVideo:
// 		var videoKVI VideoKVI_v2
// 		if err := json.Unmarshal(manifest.KVI, &videoKVI); err != nil {
// 			return nil, fmt.Errorf("failed to unmarshal KVI as VideoKVI_v2: %w", err)
// 		}
// 		return videoKVI, nil
// 	case IndexKindImage:
// 		var imageKVI ImageKVI_v2
// 		if err := json.Unmarshal(manifest.KVI, &imageKVI); err != nil {
// 			return nil, fmt.Errorf("failed to unmarshal KVI as VideoKVI_v2: %w", err)
// 		}
// 		return imageKVI, nil
// 	default:
// 		return nil, fmt.Errorf("unsupported or unknown index kind: %s", manifest.Kind)
// 	}
// }

// NewManifest_v2 是一个工厂函数，用于创建 Manifest_v2 实例
// 它接收一个 KVIProvider 接口，并将其序列化为 json.RawMessage
func NewManifest_v2(kviProvider KVIProvider, fragments []Fragment_v2) (*Manifest_v2, error) {
	// 1. 将 KVIProvider 接口实例序列化为 JSON 字节切片
	// json.Marshal 可以处理任何具有可导出字段的结构体，或者实现了 json.Marshaler 接口的类型
	// 我们的 VideoKVI_v2 完全符合这个条件
	kviBytes, err := json.Marshal(kviProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal KVI provider: %w", err)
	}

	// 2. 将字节切片转换为 json.RawMessage
	// json.RawMessage 本质上就是 []byte，这个转换是类型安全的
	rawKVI := json.RawMessage(kviBytes)

	// 3. 创建并返回 Manifest_v2 实例
	manifest := &Manifest_v2{
		Version:   ContainerVersion,
		Kind:      kviProvider.GetKind(),
		KVI:       rawKVI,
		Fragments: fragments,
	}

	return manifest, nil
}

// IndexKind 定义 KVI 的类型
type IndexKind string

const (
	IndexKindIframe  IndexKind = "iframe"
	ContainerVersion int64     = 2
)

// Index 是所有 KVI 结构体的通用接口
type Index interface {
	GetOriginalFilename() string
	GetOriginalFileSize() int64
	GetOriginalFileMD5() string
	GetEncryptedFileMD5() string
	GetMimeType() string // 重要方法，实现错误会影响前端预览
}

// NoOpIndex 是一个安全的、无操作的 Index 实现，用于在发生严重内部错误时防止 panic。
type NoOpIndex struct{}

func (i *NoOpIndex) GetMimeType() string         { return "application/octet-stream" }
func (i *NoOpIndex) GetOriginalFilename() string { return "corrupted" }
func (i *NoOpIndex) GetOriginalFileSize() int64  { return 0 }
func (i *NoOpIndex) GetOriginalFileMD5() string  { return "" }
func (i *NoOpIndex) GetEncryptedFileMD5() string { return "" }
