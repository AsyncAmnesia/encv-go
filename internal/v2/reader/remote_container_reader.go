package reader

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/Soltus/encv-go/internal/utils"
	"github.com/Soltus/encv-go/internal/v2/container/block"
	"github.com/Soltus/encv-go/internal/v2/container/manifest"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// URLResolver 是一个用于将物理路径解析为完整、可访问的 URL 的接口。
// 这使得 remoteEncryptedContainerReader 不再与特定的 URL 生成逻辑（如 OpenList）耦合。
type URLResolver interface {
	ResolveURL(physicalPath string) (string, error)
}

// remoteEncryptedContainerReader 实现了 EncryptedContainerReader 接口
// 它通过 HTTP Range 请求与远程服务器交互，实现按需读取
type remoteEncryptedContainerReader struct {
	containerURL string
	headers      map[string]string
	urlResolver  URLResolver
	// 缓存，避免重复请求
	manifest *types.Manifest_v2
}

// NewRemoteEncryptedContainerReader 创建一个新的远程容器读取器
func NewRemoteEncryptedContainerReader(containerURL string, headers map[string]string, urlResolver URLResolver) (EncryptedContainerReader, error) {
	return &remoteEncryptedContainerReader{
		containerURL: containerURL,
		headers:      headers,
		urlResolver:  urlResolver, // 【关键】存储接口
	}, nil
}

// GetFragmentReader 按需获取指定 Fragment 的加密数据流
func (r *remoteEncryptedContainerReader) GetFragmentReader(fragID string) (io.ReadCloser, error) {
	manifest := r.GetManifest()

	frag, err := manifest.GetFragmentByID(fragID)
	if err != nil {
		return nil, fmt.Errorf("fragment '%s' not found: %w", fragID, err)
	}
	// 【关键修复】计算纯粹数据的起始位置和长度
	// frag.GlobalStartOffset 是整个块的起始位置
	// frag.Length 是纯数据的长度
	blockHeaderSize := int64(binary.Size(block.BlockHeader_v2{}))
	// 【关键修复】处理物理分片
	if frag.PhysicalPath != "" {
		log.Printf("DEBUG: [RemoteContainerReader] Fragment '%s' is a physical chunk at '%s'", fragID, frag.PhysicalPath)

		// 【核心】使用注入的 URLResolver 来获取有效的 URL
		chunkURL, err := r.urlResolver.ResolveURL(frag.PhysicalPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve URL for physical chunk '%s': %w", fragID, err)
		}

		log.Printf("DEBUG: [RemoteContainerReader] Requesting physical chunk from resolved URL: %s", chunkURL)

		// 使用新获取的 URL 请求数据
		resp, err := utils.GetRemoteStreamWithRange(chunkURL, r.headers, 0, -1)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch physical chunk '%s': %w", fragID, err)
		}

		// 跳过块头
		blockHeaderSize := int64(binary.Size(block.BlockHeader_v2{}))
		_, err = io.CopyN(io.Discard, resp.Body, blockHeaderSize)
		if err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to skip block header for physical chunk '%s': %w", fragID, err)
		}

		return resp.Body, nil
	}

	// 【原有逻辑】处理单一文件容器（或第一个 chunk）
	dataStartOffset := int64(frag.GlobalStartOffset) + blockHeaderSize
	dataEnd := dataStartOffset + int64(frag.Length) - 1

	log.Printf("DEBUG: [RemoteContainerReader] Requesting fragment '%s' data with range: %d-%d", fragID, dataStartOffset, dataEnd)

	resp, err := utils.GetRemoteStreamWithRange(r.containerURL, r.headers, dataStartOffset, dataEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch fragment '%s': %w", fragID, err)
	}

	// 对于 Range 请求，服务器返回的 Body 就是我们想要的数据，直接返回
	return resp.Body, nil
}

// responseBodyWrapper 确保底层的 http.Response.Body 被正确关闭
type responseBodyWrapper struct {
	io.ReadCloser
	resp *http.Response
}

func (w *responseBodyWrapper) Close() error {
	// 首先关闭我们创建的 ReadCloser
	if err := w.ReadCloser.Close(); err != nil {
		return err
	}
	// 然后关闭底层的 HTTP 响应体
	return w.resp.Body.Close()
}

// GetManifest 按需获取并缓存 Manifest
func (r *remoteEncryptedContainerReader) GetManifest() *types.Manifest_v2 {
	if r.manifest != nil {
		return r.manifest
	}

	// 1. 下载 Footer (最后 32 字节)
	footerResp, err := utils.GetRemoteStreamWithRange(r.containerURL, r.headers, -32, -1)
	if err != nil {
		log.Printf("ERROR: [RemoteContainerReader] failed to fetch footer: %v", err)
		return nil
	}
	defer footerResp.Body.Close()
	footerData, err := io.ReadAll(footerResp.Body)
	if err != nil {
		log.Printf("ERROR: [RemoteContainerReader] failed to read footer data: %v", err)
		return nil
	}

	footer, err := manifest.ParseFooterFromBytes(footerData)
	if err != nil {
		log.Printf("ERROR: [RemoteContainerReader] failed to parse footer: %v", err)
		return nil
	}

	// 2. 【关键修正】下载整个 Manifest 块（头+数据）
	// Footer.ManifestLength 是 Manifest 块数据（JSON部分）的长度
	// 我们需要加上块头的长度来获取整个块
	blockHeaderSize := int64(binary.Size(block.BlockHeader_v2{}))
	manifestBlockSize := blockHeaderSize + int64(footer.ManifestLength)

	manifestStart := int64(footer.ManifestOffset)
	manifestEnd := manifestStart + manifestBlockSize - 1

	log.Printf("DEBUG: [RemoteContainerReader] Requesting manifest block from %d to %d", manifestStart, manifestEnd)
	manifestResp, err := utils.GetRemoteStreamWithRange(r.containerURL, r.headers, manifestStart, manifestEnd)
	if err != nil {
		log.Printf("ERROR: [RemoteContainerReader] failed to fetch manifest block: %v", err)
		return nil
	}
	defer manifestResp.Body.Close()
	manifestBlockData, err := io.ReadAll(manifestResp.Body)
	if err != nil {
		log.Printf("ERROR: [RemoteContainerReader] failed to read manifest block data: %v", err)
		return nil
	}

	// 3. 【关键修正】从块数据中解析出 JSON
	// 使用与 ReadManifestFromFile 完全相同的逻辑
	blockReader := bytes.NewReader(manifestBlockData)
	manifestHeader, err := block.ReadBlockHeader_v2(blockReader)
	if err != nil {
		log.Printf("ERROR: [RemoteContainerReader] failed to read manifest block header: %v", err)
		return nil
	}
	if manifestHeader.Type != types.BlockTypeManifest_v2 {
		log.Printf("ERROR: [RemoteContainerReader] expected manifest block type, got %d", manifestHeader.Type)
		return nil
	}

	manifestData, err := block.ReadBlockData_v2(blockReader, manifestHeader)
	if err != nil {
		log.Printf("ERROR: [RemoteContainerReader] failed to read manifest block data: %v", err)
		return nil
	}

	// 4. 反序列化 JSON
	manifest, err := manifest.ParseManifestFromBytes(manifestData)
	if err != nil {
		log.Printf("ERROR: [RemoteContainerReader] failed to parse manifest from JSON: %v", err)
		return nil
	}

	r.manifest = manifest
	return manifest
}

// GetKVIProvider 从缓存的 Manifest 中获取 KVI
func (r *remoteEncryptedContainerReader) GetKVIProvider() (types.KVIProvider, error) {
	manifest := r.GetManifest()
	if manifest == nil {
		return nil, fmt.Errorf("could not get manifest to retrieve KVI")
	}
	return types.NewKVIProviderFromManifest(manifest)
}

// GetFragments 返回 Manifest 中的所有片段定义
func (r *remoteEncryptedContainerReader) GetFragments() []types.Fragment_v2 {
	manifest := r.GetManifest()
	if manifest == nil {
		return nil
	}
	return manifest.Fragments
}

// Close 对于远程读取器是空操作
func (r *remoteEncryptedContainerReader) Close() error {
	return nil
}
