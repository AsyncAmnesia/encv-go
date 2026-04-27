package reader

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// fragmentIndex 用于快速定位 Fragment
type fragmentIndex struct {
	frag       *types.Fragment_v2
	start, end uint64 // 全局偏移的起始和结束
}

// VirtualSeekableDecryptReader 实现了高性能的可寻址解密流。
type VirtualSeekableDecryptReader struct {
	containerReader EncryptedContainerReader
	key             []byte
	iv              []byte

	streamFragments []types.Fragment_v2
	// 【性能优化】用于二分查找的索引，将 Seek 复杂度从 O(N) 降至 O(log N)
	fragmentIndex        []fragmentIndex
	currentFragmentIndex int

	// currentRawReader 是指向当前 fragment 的底层加密数据读取器
	currentRawReader io.ReadCloser
	// 直接保存解密后的数据读取器，避免多层包装导致的状态混乱
	currentDataReader io.Reader

	globalOffset int64

	// 【性能优化】buffer pool 用于复用内存，减少 GC 压力
	bufPool sync.Pool
}

func NewVirtualSeekableDecryptReader(cr EncryptedContainerReader, password string) (DecryptReader, error) {
	manifest := cr.GetManifest()
	kviProvider, err := cr.GetKVIProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal KVI from manifest: %w", err)
	}

	key, iv, err := deriveKeyAndIV(kviProvider, password)
	if err != nil {
		return nil, err
	}

	r := &VirtualSeekableDecryptReader{
		containerReader: cr,
		key:             key,
		iv:              iv,
		streamFragments: filterFragmentsByType(manifest.Fragments, string(types.FragmentType_SeekableStream)),
		bufPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 64*1024) // 初始容量 64KB
			},
		},
	}
	if len(r.streamFragments) == 0 {
		return nil, fmt.Errorf("no seekable stream fragments found in manifest")
	}
	sort.Slice(r.streamFragments, func(i, j int) bool {
		return r.streamFragments[i].GlobalStartOffset < r.streamFragments[j].GlobalStartOffset
	})

	// 初始化到第一个 fragment
	r.currentFragmentIndex = 0
	if err := r.setupCurrentFragmentReader(); err != nil {
		return nil, fmt.Errorf("failed to initialize first fragment reader: %w", err)
	}

	return r, nil
}

// setupCurrentFragmentReader 准备当前分片的读取器，采用多路径自适应策略
func (r *VirtualSeekableDecryptReader) setupCurrentFragmentReader() error {
	if r.currentRawReader != nil {
		_ = r.currentRawReader.Close()
		r.currentRawReader = nil
		r.currentDataReader = nil
	}

	if r.currentFragmentIndex >= len(r.streamFragments) {
		return io.EOF
	}
	frag := &r.streamFragments[r.currentFragmentIndex]

	// 计算在当前 fragment 内的局部偏移
	localOffset := uint64(0)
	if r.globalOffset > int64(frag.GlobalStartOffset) {
		localOffset = uint64(r.globalOffset) - frag.GlobalStartOffset
	}
	if localOffset >= frag.Length {
		return fmt.Errorf("seek offset (%d) beyond fragment %s length (%d)", localOffset, frag.ID, frag.Length)
	}

	// 获取 Fragment 的 Reader (此时返回的是完整的 Fragment Payload)
	rawReader, err := r.containerReader.GetFragmentReader(frag.ID)
	if err != nil {
		return fmt.Errorf("container is corrupt: failed to get reader for fragment '%s': %w", frag.ID, err)
	}

	block, err := aes.NewCipher(r.key)
	if err != nil {
		_ = rawReader.Close()
		return fmt.Errorf("failed to create aes cipher for fragment %s: %w", frag.ID, err)
	}

	// 1. 计算当前读取位置的绝对逻辑偏移
	// 这决定了 IV
	absGlobalOffset := frag.GlobalStartOffset + localOffset

	// 2. 推导 IV
	iv, derr := crypto.DeriveCTRIVForOffset_v2(r.iv, absGlobalOffset)
	if derr != nil {
		_ = rawReader.Close()
		return derr
	}

	stream := cipher.NewCTR(block, iv)
	streamReader := &cipher.StreamReader{S: stream, R: rawReader}

	// 3. 【关键修复】显式跳过 localOffset 字节
	// 如果 localOffset > 0，我们必须消耗掉前面的字节以同步 CTR 的计数器
	if localOffset > 0 {
		// 为了性能，我们不分配 huge buffer，而是循环读取
		// 但既然是内存中的 SectionReader，读取和丢弃相对较快
		// 使用一个小的 buffer 复用池
		buf := r.bufPool.Get().([]byte)
		defer r.bufPool.Put(buf)

		var discarded uint64
		for discarded < localOffset {
			// 计算这次读取的大小
			toRead := localOffset - discarded
			// cap buf to toRead (but preserve underlying array if possible)
			if cap(buf) < int(toRead) {
				buf = make([]byte, toRead)
			} else {
				buf = buf[:toRead]
			}

			n, err := io.ReadFull(streamReader, buf)
			if err != nil {
				_ = rawReader.Close()
				return fmt.Errorf("failed to discard %d bytes for alignment: %w", toRead, err)
			}
			discarded += uint64(n)
		}
	}

	// 此时 streamReader 已经同步到了 absGlobalOffset
	r.currentRawReader = rawReader
	r.currentDataReader = streamReader
	return nil
}

// Read 实现 io.Reader 接口，使用健壮的循环逻辑
func (r *VirtualSeekableDecryptReader) Read(p []byte) (n int, err error) {
	totalRead := 0
	for totalRead < len(p) {
		if r.currentFragmentIndex >= len(r.streamFragments) {
			return totalRead, io.EOF
		}
		if r.currentDataReader == nil {
			if setupErr := r.setupCurrentFragmentReader(); setupErr != nil {
				if errors.Is(setupErr, io.EOF) {
					return totalRead, io.EOF
				}
				return totalRead, setupErr
			}
		}

		bytesRead, readErr := r.currentDataReader.Read(p[totalRead:])
		totalRead += bytesRead
		r.globalOffset += int64(bytesRead)

		if readErr == io.EOF {
			if r.currentRawReader != nil {
				_ = r.currentRawReader.Close()
				r.currentRawReader = nil
			}
			r.currentDataReader = nil
			r.currentFragmentIndex++
			continue
		}
		if readErr != nil {
			return totalRead, readErr
		}
	}
	return totalRead, nil
}

// Seek 实现 io.Seeker 接口，修正了多余逻辑
func (r *VirtualSeekableDecryptReader) Seek(offset int64, whence int) (int64, error) {
	totalSize := int64(r.streamFragments[len(r.streamFragments)-1].GlobalStartOffset + r.streamFragments[len(r.streamFragments)-1].Length)

	var newGlobalOffset int64
	switch whence {
	case io.SeekStart:
		newGlobalOffset = offset
	case io.SeekCurrent:
		newGlobalOffset = r.globalOffset + offset
	case io.SeekEnd:
		newGlobalOffset = totalSize + offset
	default:
		return r.globalOffset, fmt.Errorf("invalid whence value: %d", whence)
	}

	if newGlobalOffset < 0 {
		return r.globalOffset, fmt.Errorf("negative seek position")
	}

	// 允许在文件末尾 Seek
	if newGlobalOffset > totalSize {
		return r.globalOffset, fmt.Errorf("seek offset %d out of bounds [0, %d]", newGlobalOffset, totalSize)
	}

	// 如果目标位置与当前位置相同，则无需任何操作
	if newGlobalOffset == r.globalOffset {
		return r.globalOffset, nil
	}

	fragIdx := sort.Search(len(r.streamFragments), func(i int) bool {
		return int64(r.streamFragments[i].GlobalStartOffset+r.streamFragments[i].Length) > newGlobalOffset
	})
	if fragIdx == len(r.streamFragments) || int64(r.streamFragments[fragIdx].GlobalStartOffset) > newGlobalOffset {
		// 处理 Seek 到文件末尾的情况
		if newGlobalOffset == totalSize {
			r.currentFragmentIndex = len(r.streamFragments)
			r.currentDataReader = nil
			r.globalOffset = newGlobalOffset
			return r.globalOffset, nil
		}
		return r.globalOffset, fmt.Errorf("seek position %d not inside any fragment", newGlobalOffset)
	}

	// 先更新 index，再调用 setup
	r.currentFragmentIndex = fragIdx
	r.globalOffset = newGlobalOffset

	if r.currentRawReader != nil {
		_ = r.currentRawReader.Close()
		r.currentRawReader = nil
	}
	r.currentDataReader = nil

	// setupCurrentFragmentReader 会根据 r.globalOffset 自动定位到正确位置
	if err := r.setupCurrentFragmentReader(); err != nil {
		return r.globalOffset, err
	}

	return r.globalOffset, nil
}

func (r *VirtualSeekableDecryptReader) Close() error {
	if r.currentRawReader != nil {
		return r.currentRawReader.Close()
	}
	return nil
}

// --- 辅助函数 ---

// deriveKeyAndIV 从 KVI 和密码派生密钥和 IV
func deriveKeyAndIV(kviProvider types.KVIProvider, password string) (key, iv []byte, err error) {
	salt, err := crypto.Base64Decode_v2(kviProvider.GetEncryptionInfo().SaltBase64)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode salt: %w", err)
	}
	iv, err = crypto.Base64Decode_v2(kviProvider.GetEncryptionInfo().IVBase64)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode iv: %w", err)
	}
	key = crypto.GenerateKey_v2(password, salt, types.KeySize_v2)
	return key, iv, nil
}

// filterFragmentsByType 筛选出指定类型的 Fragment
func filterFragmentsByType(frags []types.Fragment_v2, fragType string) []types.Fragment_v2 {
	var result []types.Fragment_v2
	for _, f := range frags {
		if string(f.Type) == fragType {
			result = append(result, f)
		}
	}
	return result
}
