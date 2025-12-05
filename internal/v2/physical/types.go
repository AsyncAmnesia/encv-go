package physical

import (
	"context"
	"io"

	"github.com/Soltus/encv-go/internal/v2/namer"
	"github.com/Soltus/encv-go/internal/v2/reader"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// PhysicalPacker 定义了物理分片的打包接口
type PhysicalPacker interface {
	// Pack 执行完整的物理打包，包括数据分片和 Manifest 写入
	// 【关键修改】接收 manifest 作为参数，并负责完成所有写入
	Pack(data io.Reader, manifest *types.Manifest_v2, req *PackRequest) (mainChunkPath string, err error)
}

// PhysicalUnpacker 定义了物理分片的解包接口
// 它负责处理物理分片，并返回一个可以像单一文件一样读取的容器路径
type PhysicalUnpacker interface {
	// Unpack 接收主容器文件路径（可能是第一个分片），并返回一个统一的容器文件路径
	// 返回的路径可以被 NewFileContainerReader_v2 直接使用
	// 同时返回一个 cleanup 函数，用于在操作完成后清理临时资源
	Unpack(ctx context.Context, mainContainerPath string) (unifiedContainerPath string, cleanup func(), err error)
}

// Unpacker 定义了从容器文件中解包数据的接口
type Unpacker interface {
	// Unpack 打开容器，并返回一个用于创建解密流的工厂、文件索引和原始大小
	// 【关键修改】直接返回 reader 包的工厂接口
	Unpack(ctx context.Context, containerPath string) (reader.DecryptReaderFactory, types.Index, int64, error)
}

// Packer 定义了将加密数据打包到容器的接口
type Packer interface {
	// Pack 将加密数据和元数据打包到容器文件
	Pack(ctx context.Context, req *PackRequest) error
}

// PackRequest 是打包请求的参数集合
type PackRequest struct {
	// BaseName 是不带容器扩展名的基础文件名，例如 "321.4pm"
	BaseName string
	// OutputDir 是输出目录
	OutputDir string
	// EncryptedDataReader 是加密后数据的来源，一个纯粹的流
	EncryptedDataReader io.Reader
	// Index 是文件的完整索引信息
	Index types.Index
	// Salt 和 IV 是加密参数
	Salt, IV []byte
	// LogicalFragments 是预先计算好的逻辑分片元数据
	LogicalFragments []types.Fragment_v2
	Namer            namer.ChunkNamer
	StartIdx         int

	// 一个可选的、最终的主文件名，设计为单文件打包使用。
	// 如果设置了此字段，Packer 将直接使用它，忽略 Namer。
	FinalFileName string
}
