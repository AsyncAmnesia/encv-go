package reader

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io"
	"log"
	"sort"

	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/types"
)

type RemoteStreamingDecryptReader struct {
	containerReader EncryptedContainerReader
	key             []byte
	baseIV          []byte

	streamFragments   []types.Fragment_v2
	currentFragIndex  int
	currentDataReader io.Reader
	currentRawReader  io.ReadCloser
	kviProvider       types.KVIProvider
}

func NewRemoteStreamingDecryptReader(cr EncryptedContainerReader, password string) (DecryptReader, error) {
	kviProvider, err := cr.GetKVIProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to get KVI provider: %w", err)
	}

	salt, err := crypto.Base64Decode_v2(kviProvider.GetEncryptionInfo().SaltBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode salt: %w", err)
	}
	baseIV, err := crypto.Base64Decode_v2(kviProvider.GetEncryptionInfo().IVBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base iv: %w", err)
	}
	key := crypto.GenerateKey_v2(password, salt, types.KeySize_v2)

	allFragments := cr.GetFragments()
	var streamFragments []types.Fragment_v2
	for _, f := range allFragments {
		if f.Type == types.FragmentType_SeekableStream {
			streamFragments = append(streamFragments, f)
		}
	}
	if len(streamFragments) == 0 {
		return nil, fmt.Errorf("no SeekableStream fragments found in container")
	}
	sort.Slice(streamFragments, func(i, j int) bool {
		return streamFragments[i].GlobalStartOffset < streamFragments[j].GlobalStartOffset
	})

	r := &RemoteStreamingDecryptReader{
		containerReader:  cr,
		key:              key,
		baseIV:           baseIV,
		streamFragments:  streamFragments,
		kviProvider:      kviProvider,
		currentFragIndex: -1,
	}

	if err := r.setupNextFragmentReader(); err != nil {
		return nil, fmt.Errorf("failed to setup first fragment reader: %w", err)
	}

	return r, nil
}

func (r *RemoteStreamingDecryptReader) setupNextFragmentReader() error {
	if r.currentRawReader != nil {
		_ = r.currentRawReader.Close()
		r.currentRawReader = nil
		r.currentDataReader = nil
	}

	r.currentFragIndex++
	if r.currentFragIndex >= len(r.streamFragments) {
		return io.EOF
	}

	frag := &r.streamFragments[r.currentFragIndex]
	log.Printf("DEBUG: [RemoteStreamingDecryptReader] Setting up reader for fragment '%s' (index %d)", frag.ID, r.currentFragIndex)

	rawReader, err := r.containerReader.GetFragmentReader(frag.ID)
	if err != nil {
		return fmt.Errorf("failed to get reader for fragment '%s': %w", frag.ID, err)
	}

	block, err := aes.NewCipher(r.key)
	if err != nil {
		_ = rawReader.Close()
		return fmt.Errorf("failed to create aes cipher for fragment %s: %w", frag.ID, err)
	}

	var stream cipher.Stream
	if frag.Type == types.FragmentType_SeekableStream {
		// 【关键修复】恢复对第一个 fragment 的特殊处理
		if frag.ID == "logical_fragment_0" {
			// 第一个 fragment 必须直接使用 baseIV
			stream = cipher.NewCTR(block, r.baseIV)
			log.Printf("DEBUG: [RemoteStreamingDecryptReader] Using base IV for the first fragment '%s'", frag.ID)
		} else {
			// 后续的 fragment 使用派生 IV
			iv, err := crypto.DeriveCTRIVForOffset_v2(r.baseIV, uint64(frag.GlobalStartOffset))
			if err != nil {
				_ = rawReader.Close()
				return fmt.Errorf("failed to derive IV for fragment %s: %w", frag.ID, err)
			}
			stream = cipher.NewCTR(block, iv)
			log.Printf("DEBUG: [RemoteStreamingDecryptReader] Derived IV for fragment '%s' at offset %d", frag.ID, frag.GlobalStartOffset)
		}
	} else {
		stream = cipher.NewCTR(block, r.baseIV)
		log.Printf("DEBUG: [RemoteStreamingDecryptReader] Using base IV for AtomicFile")
	}

	streamReader := &cipher.StreamReader{S: stream, R: rawReader}

	r.currentRawReader = rawReader
	r.currentDataReader = streamReader

	return nil
}

// SeekTo 将读取器的状态定位到指定的全局偏移量。
// 这是一个高效的跳转操作，不会下载和丢弃中间数据。
func (r *RemoteStreamingDecryptReader) SeekTo(offset int64) error {
	// 1. 根据 offset 找到目标 fragment
	var targetFrag *types.Fragment_v2
	var targetIndex int = -1
	for i, frag := range r.streamFragments {
		// 【关键修复】将所有值转换为 int64 进行比较，避免类型提升和溢出问题
		fragStart := int64(frag.GlobalStartOffset)
		fragEnd := int64(frag.GlobalStartOffset) + int64(frag.Length)
		if offset >= fragStart && offset < fragEnd {
			targetFrag = &r.streamFragments[i]
			targetIndex = i
			break
		}
	}
	if targetFrag == nil {
		// 【关键修复】如果 offset 超出所有 fragment 的数据范围，返回 EOF
		return io.EOF
	}

	// 2. 如果目标 fragment 不是当前 fragment，则切换
	if r.currentFragIndex != targetIndex {
		if r.currentRawReader != nil {
			_ = r.currentRawReader.Close()
		}
		// 将索引设置为前一个，这样 setupNextFragmentReader 就能正确设置到我们想要的 fragment
		r.currentFragIndex = targetIndex - 1
		r.currentDataReader = nil
		r.currentRawReader = nil

		// 调用设置逻辑
		if err := r.setupNextFragmentReader(); err != nil {
			return fmt.Errorf("failed to setup target fragment '%s': %w", targetFrag.ID, err)
		}
	}

	// 3. 跳过 fragment 内部的偏移量
	// 【关键修复】同样使用 int64 进行计算
	offsetWithinFragment := offset - int64(targetFrag.GlobalStartOffset)
	if offsetWithinFragment > 0 {
		log.Printf("DEBUG: [RemoteStreamingDecryptReader] Seeking %d bytes into fragment '%s'", offsetWithinFragment, targetFrag.ID)
		// 这个 io.CopyN 是在单个 fragment 内部进行，数据已经在内存或本地缓存中，非常快
		_, err := io.CopyN(io.Discard, r.currentDataReader, offsetWithinFragment)
		if err != nil {
			return fmt.Errorf("failed to seek within fragment '%s': %w", targetFrag.ID, err)
		}
	}

	return nil
}

// Read 实现了 io.Reader 接口。
// 这是让 DecryptReader 能够与 io.CopyN 等 I/O 原语协同工作的关键。
func (r *RemoteStreamingDecryptReader) Read(p []byte) (n int, err error) {
	for {
		// 1. 如果当前没有数据读取器，说明已经到末尾
		if r.currentDataReader == nil {
			return 0, io.EOF
		}

		// 2. 尝试从当前 fragment 的数据读取器中读取数据
		n, err = r.currentDataReader.Read(p)

		// 3. 如果成功读取，直接返回
		if err == nil {
			return n, nil
		}

		// 4. 如果遇到 EOF，说明当前 fragment 读完，需要切换到下一个
		if err == io.EOF {
			log.Printf("DEBUG: [RemoteStreamingDecryptReader] Fragment finished, switching to next.")
			// 尝试设置下一个 fragment 的读取器
			setupErr := r.setupNextFragmentReader()
			if setupErr != nil {
				// 如果设置下一个 fragment 失败（比如已经是最后一个），则返回真正的 EOF
				if setupErr == io.EOF {
					return 0, io.EOF
				}
				// 其他错误则返回
				return 0, setupErr
			}
			// 成功切换，继续循环，从新的 fragment 读取数据
			continue
		}

		// 5. 其他读取错误，直接返回
		return n, err
	}
}

func (r *RemoteStreamingDecryptReader) Close() error {
	if r.currentRawReader != nil {
		return r.currentRawReader.Close()
	}
	return nil
}

// 为了兼容上层逻辑，我们让它也实现 GetIndex
func (r *RemoteStreamingDecryptReader) GetIndex() types.Index {
	return r.kviProvider.GetIndex()
}
