package reader

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io"

	"github.com/Soltus/encv-go/internal/v2/types"
)

// SequentialSeekableDecryptReader 用于解密 SeekableStream 类型的 fragments
// 支持顺序读取和 Seek 操作
type SequentialSeekableDecryptReader struct {
	containerReader  EncryptedContainerReader
	key, iv          []byte
	fragments        []types.Fragment_v2
	currentIndex     int
	currentReader    io.ReadCloser
	currentDecryptor io.Reader
	currentOffset    int64 // 当前在全局数据流中的位置
	totalSize        int64 // 总数据大小
}

func NewSequentialSeekableDecryptReader(cr EncryptedContainerReader, password string) (DecryptReader, error) {
	manifest := cr.GetManifest()
	kviProvider, err := cr.GetKVIProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal KVI from manifest: %w", err)
	}

	key, iv, err := deriveKeyAndIV(kviProvider, password)
	if err != nil {
		return nil, err
	}

	fragments := filterFragmentsByType(manifest.Fragments, string(types.FragmentType_SeekableStream))

	// 计算总大小
	var totalSize int64
	for _, frag := range fragments {
		totalSize += int64(frag.Length)
	}

	r := &SequentialSeekableDecryptReader{
		containerReader: cr,
		key:             key,
		iv:              iv,
		fragments:       fragments,
		totalSize:       totalSize,
		currentIndex:    0,
		currentOffset:   0,
	}
	return r, nil
}

func (r *SequentialSeekableDecryptReader) Read(p []byte) (n int, err error) {
	if len(r.fragments) == 0 {
		return 0, io.EOF
	}
	if r.currentDecryptor == nil {
		if err := r.setupFragmentAtIndex(r.currentIndex); err != nil {
			return 0, err
		}
	}
	n, err = r.currentDecryptor.Read(p)
	r.currentOffset += int64(n)
	if err == io.EOF {
		r.currentReader.Close()
		r.currentReader = nil
		r.currentDecryptor = nil
		r.currentIndex++
		// 【关键修复】如果已经读取了一些字节，先返回这些字节
		// 下次调用 Read 时会继续读取下一个 fragment
		if n > 0 {
			return n, nil
		}
		// 如果没有读取到字节，递归读取下一个 fragment
		return r.Read(p)
	}
	return n, err
}

func (r *SequentialSeekableDecryptReader) setupFragmentAtIndex(index int) error {
	if index >= len(r.fragments) {
		return io.EOF
	}
	frag := &r.fragments[index]

	// 获取 Fragment 的 Reader
	rawReader, err := r.containerReader.GetFragmentReader(frag.ID)
	if err != nil {
		return fmt.Errorf("failed to get reader for fragment %s: %w", frag.ID, err)
	}
	r.currentReader = rawReader

	// 创建 AES cipher
	block, err := aes.NewCipher(r.key)
	if err != nil {
		return fmt.Errorf("failed to create aes cipher: %w", err)
	}

	// 【关键修复】使用基础 IV 创建 CTR 流，然后跳过前面的字节来同步计数器
	// 这与加密时的行为一致：加密整个文件，然后分片打包
	stream := cipher.NewCTR(block, r.iv)

	// 跳过 frag.GlobalStartOffset 字节来同步计数器
	if frag.GlobalStartOffset > 0 {
		skipBuf := make([]byte, frag.GlobalStartOffset)
		stream.XORKeyStream(skipBuf, skipBuf)
	}

	r.currentDecryptor = &cipher.StreamReader{S: stream, R: rawReader}
	return nil
}

func (r *SequentialSeekableDecryptReader) Close() error {
	if r.currentReader != nil {
		r.currentReader.Close()
	}
	return r.containerReader.Close()
}

// Seek 实现 io.Seeker 接口，支持 HTTP Range 请求
func (r *SequentialSeekableDecryptReader) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = r.currentOffset + offset
	case io.SeekEnd:
		newOffset = r.totalSize + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}

	if newOffset < 0 {
		return 0, fmt.Errorf("cannot seek to negative offset: %d", newOffset)
	}
	if newOffset > r.totalSize {
		return 0, fmt.Errorf("cannot seek beyond end of file: %d > %d", newOffset, r.totalSize)
	}

	// 找到包含 newOffset 的 fragment
	var fragStart int64
	targetIndex := -1
	for i, frag := range r.fragments {
		fragLen := int64(frag.Length)
		if newOffset >= fragStart && newOffset < fragStart+fragLen {
			targetIndex = i
			break
		}
		fragStart += fragLen
	}

	if targetIndex == -1 {
		// 如果 newOffset 正好等于总大小，返回 EOF
		if newOffset == r.totalSize {
			r.currentIndex = len(r.fragments)
			r.currentOffset = newOffset
			if r.currentReader != nil {
				r.currentReader.Close()
				r.currentReader = nil
				r.currentDecryptor = nil
			}
			return newOffset, nil
		}
		return 0, fmt.Errorf("could not find fragment for offset %d", newOffset)
	}

	// 如果目标 fragment 与当前不同，或者当前没有 reader，重新设置
	if targetIndex != r.currentIndex || r.currentDecryptor == nil {
		if r.currentReader != nil {
			r.currentReader.Close()
			r.currentReader = nil
			r.currentDecryptor = nil
		}
		r.currentIndex = targetIndex
		if err := r.setupFragmentAtIndex(targetIndex); err != nil {
			return 0, err
		}
	}

	// 计算在 fragment 内的偏移
	localOffset := newOffset - fragStart
	if localOffset < 0 {
		return 0, fmt.Errorf("invalid local offset: %d", localOffset)
	}

	// 跳过 fragment 内的字节
	if localOffset > 0 {
		// 使用 io.CopyN 跳过字节
		_, err := io.CopyN(io.Discard, r.currentDecryptor, localOffset)
		if err != nil {
			return 0, fmt.Errorf("failed to skip %d bytes in fragment: %w", localOffset, err)
		}
	}

	r.currentOffset = newOffset
	return newOffset, nil
}
