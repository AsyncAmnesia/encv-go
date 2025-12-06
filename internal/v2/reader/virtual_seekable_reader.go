package reader

import (
	"bytes"
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
	// currentDataReader 提供解密后的连续读取
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

func (r *VirtualSeekableDecryptReader) buildFragmentIndex() {
	var currentOffset uint64 = 0
	for _, frag := range r.streamFragments {
		r.fragmentIndex = append(r.fragmentIndex, fragmentIndex{
			frag:  &frag,
			start: currentOffset,
			end:   currentOffset + frag.Length - 1,
		})
		currentOffset += frag.Length
	}
}

// 【直接复用】源自旧代码的健壮实现
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

	// 【接口修正】使用统一的 GetFragmentReader
	rawReader, err := r.containerReader.GetFragmentReader(frag.ID)
	if err != nil {
		return fmt.Errorf("container is corrupt: failed to get reader for fragment '%s': %w", frag.ID, err)
	}

	block, err := aes.NewCipher(r.key)
	if err != nil {
		_ = rawReader.Close()
		return fmt.Errorf("failed to create aes cipher for fragment %s: %w", frag.ID, err)
	}

	// Try path using ReaderAt (SectionReader) - 最优路径
	if ra, ok := rawReader.(io.ReaderAt); ok {
		section := io.NewSectionReader(ra, int64(localOffset), int64(frag.Length-localOffset))
		totalOffset := frag.GlobalStartOffset + localOffset
		iv, derr := crypto.DeriveCTRIVForOffset_v2(r.iv, totalOffset)
		if derr != nil {
			_ = rawReader.Close()
			return derr
		}
		stream := cipher.NewCTR(block, iv)
		streamReader := &cipher.StreamReader{S: stream, R: section}

		offsetInBlock := int(totalOffset % uint64(aes.BlockSize))
		if offsetInBlock != 0 {
			tmp := make([]byte, offsetInBlock)
			if _, err := io.ReadFull(streamReader, tmp); err != nil {
				_ = rawReader.Close()
				return fmt.Errorf("failed to align stream for fragment %s: %w", frag.ID, err)
			}
		}

		r.currentRawReader = rawReader
		r.currentDataReader = streamReader
		return nil
	}

	// Try path using ReadSeeker - 次优路径
	if rs, ok := rawReader.(io.ReadSeeker); ok {
		if _, err := rs.Seek(int64(localOffset), io.SeekStart); err != nil {
			_ = rawReader.Close()
			return fmt.Errorf("failed to seek underlying fragment reader for %s: %w", frag.ID, err)
		}
		totalOffset := frag.GlobalStartOffset + localOffset
		iv, derr := crypto.DeriveCTRIVForOffset_v2(r.iv, totalOffset)
		if derr != nil {
			_ = rawReader.Close()
			return derr
		}
		stream := cipher.NewCTR(block, iv)
		streamReader := &cipher.StreamReader{S: stream, R: rs}

		offsetInBlock := int(totalOffset % uint64(aes.BlockSize))
		if offsetInBlock != 0 {
			tmp := make([]byte, offsetInBlock)
			if _, err := io.ReadFull(streamReader, tmp); err != nil {
				_ = rawReader.Close()
				return fmt.Errorf("failed to align stream for fragment %s: %w", frag.ID, err)
			}
		}

		r.currentRawReader = rawReader
		r.currentDataReader = streamReader
		return nil
	}

	// Fallback: 读取全部数据到内存 - 兜底路径
	encryptedData, err := io.ReadAll(rawReader)
	_ = rawReader.Close()
	if err != nil {
		return fmt.Errorf("container is corrupt: failed to read data for fragment '%s': %w", frag.ID, err)
	}
	totalOffset := frag.GlobalStartOffset
	ivForFrag, derr := crypto.DeriveCTRIVForOffset_v2(r.iv, totalOffset)
	if derr != nil {
		return derr
	}
	stream := cipher.NewCTR(block, ivForFrag)
	decryptedData := make([]byte, len(encryptedData))
	stream.XORKeyStream(decryptedData, encryptedData)
	decryptedSlice := decryptedData[localOffset:]
	r.currentDataReader = bytes.NewReader(decryptedSlice)
	r.currentRawReader = nil

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
			// 当前 fragment 读完，关闭并前进到下一个
			if r.currentRawReader != nil {
				_ = r.currentRawReader.Close()
				r.currentRawReader = nil
			}
			r.currentDataReader = nil
			r.currentFragmentIndex++
			// 继续循环以填充剩余 p
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

	// 【关键修复】如果目标位置与当前位置相同，则无需任何操作
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

	// 【关键修正】先更新 index，再调用 setup
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
