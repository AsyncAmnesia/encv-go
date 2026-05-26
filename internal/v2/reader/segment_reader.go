package reader

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"

	containerhandle "github.com/Soltus/encv-go/internal/v2/container/handle"
	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/types"
)

type V4ContainerInfo struct {
	Header   *types.EnvelopeHeaderV4
	Footer   *types.EnvelopeFooterV4
	Manifest *types.Manifest_v4
	FilePath string
	Key      []byte
}

func OpenV4Container(filePath string, password string) (*V4ContainerInfo, error) {
	src, err := containerhandle.NewFileSource(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open container source: %w", err)
	}

	h, err := containerhandle.Open(src)
	if err != nil {
		src.Close()
		return nil, fmt.Errorf("failed to open container handle: %w", err)
	}
	defer h.Close()

	if h.Version() != 4 {
		return nil, fmt.Errorf("not a v4 container (version: %d)", h.Version())
	}

	var kvi struct {
		SaltBase64 string `json:"salt_base64"`
		IVBase64   string `json:"iv_base64"`
	}
	if err := json.Unmarshal(h.Manifest().KVI, &kvi); err != nil {
		return nil, fmt.Errorf("failed to parse KVI from manifest: %w", err)
	}

	salt, err := base64.StdEncoding.DecodeString(kvi.SaltBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode salt from KVI: %w", err)
	}

	key := crypto.GenerateKey_v2(password, salt, types.KeySize_v2)

	return &V4ContainerInfo{
		Header:   h.HeaderV4(),
		Footer:   h.FooterV4(),
		Manifest: h.ManifestV4(),
		FilePath: filePath,
		Key:      key,
	}, nil
}

type SegmentSeekableReader struct {
	info       *V4ContainerInfo
	file       *os.File
	playlist   []types.Segment_v4
	plainSizes []int64
	offset     int64
}

func segmentPlainSize(seg types.Segment_v4) int64 {
	nonce, err := base64.StdEncoding.DecodeString(seg.Nonce)
	if err != nil {
		return 0
	}
	return int64(seg.Size) - int64(types.SegmentHeaderSize) - int64(len(nonce))
}

func NewSegmentSeekableReader(info *V4ContainerInfo, playlistName string) (*SegmentSeekableReader, error) {
	playlist, err := info.Manifest.ResolvePlaylist(playlistName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve playlist '%s': %w", playlistName, err)
	}

	f, err := os.Open(info.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	plainSizes := make([]int64, len(playlist))
	for i, seg := range playlist {
		plainSizes[i] = segmentPlainSize(seg)
	}

	return &SegmentSeekableReader{
		info:       info,
		file:       f,
		playlist:   playlist,
		plainSizes: plainSizes,
		offset:     0,
	}, nil
}

func (r *SegmentSeekableReader) totalSize() int64 {
	var total int64
	for _, sz := range r.plainSizes {
		total += sz
	}
	return total
}

func (r *SegmentSeekableReader) locateOffset(off int64) (segIdx int, offsetInSegment int64, ok bool) {
	var accumulated int64
	for i, sz := range r.plainSizes {
		segEnd := accumulated + sz
		if off < segEnd {
			return i, off - accumulated, true
		}
		accumulated = segEnd
	}
	if off == accumulated {
		return len(r.playlist), 0, true
	}
	return -1, 0, false
}

func (r *SegmentSeekableReader) ReadAt(p []byte, off int64) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	total := r.totalSize()
	if off >= total {
		return 0, io.EOF
	}
	if off < 0 {
		return 0, fmt.Errorf("negative offset: %d", off)
	}

	segIdx, offsetInSeg, ok := r.locateOffset(off)
	if !ok || segIdx >= len(r.playlist) {
		return 0, io.EOF
	}

	written := 0
	for written < len(p) && segIdx < len(r.playlist) {
		seg := r.playlist[segIdx]

		nonce, decErr := base64.StdEncoding.DecodeString(seg.Nonce)
		if decErr != nil {
			return written, fmt.Errorf("failed to decode nonce for segment '%s': %w", seg.ID, decErr)
		}

		nonceSize := uint64(len(nonce))
		encDataSize := seg.Size - uint64(types.SegmentHeaderSize) - nonceSize
		encData := make([]byte, encDataSize)

		dataOffset := int64(seg.Offset) + int64(types.SegmentHeaderSize) + int64(nonceSize)
		if _, seekErr := r.file.Seek(dataOffset, io.SeekStart); seekErr != nil {
			return written, fmt.Errorf("failed to seek to segment '%s': %w", seg.ID, seekErr)
		}
		if _, readErr := io.ReadFull(r.file, encData); readErr != nil {
			return written, fmt.Errorf("failed to read segment '%s': %w", seg.ID, readErr)
		}

		plainData, decErr := crypto.DecryptSegment(encData, nonce, r.info.Key)
		if decErr != nil {
			return written, fmt.Errorf("failed to decrypt segment '%s': %w", seg.ID, decErr)
		}

		available := len(plainData) - int(offsetInSeg)
		if available <= 0 {
			segIdx++
			offsetInSeg = 0
			continue
		}

		toCopy := len(p) - written
		if toCopy > available {
			toCopy = available
		}
		copy(p[written:], plainData[offsetInSeg:offsetInSeg+int64(toCopy)])
		written += toCopy
		segIdx++
		offsetInSeg = 0
	}

	if written == 0 {
		return 0, io.EOF
	}
	return written, nil
}

func (r *SegmentSeekableReader) Seek(offset int64, whence int) (int64, error) {
	total := r.totalSize()
	switch whence {
	case io.SeekStart:
		r.offset = offset
	case io.SeekCurrent:
		r.offset += offset
	case io.SeekEnd:
		r.offset = total + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}
	if r.offset < 0 {
		r.offset = 0
		return 0, fmt.Errorf("cannot seek to negative offset")
	}
	if r.offset > total {
		r.offset = total
	}
	return r.offset, nil
}

func (r *SegmentSeekableReader) Read(p []byte) (n int, err error) {
	n, err = r.ReadAt(p, r.offset)
	r.offset += int64(n)
	return n, err
}

func (r *SegmentSeekableReader) Close() error {
	return r.file.Close()
}

type SegmentSequentialReader struct {
	info     *V4ContainerInfo
	file     *os.File
	playlist []types.Segment_v4
	segIndex int
	segReader io.Reader
}

func NewSegmentSequentialReader(info *V4ContainerInfo, playlistName string) (*SegmentSequentialReader, error) {
	playlist, err := info.Manifest.ResolvePlaylist(playlistName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve playlist '%s': %w", playlistName, err)
	}

	f, err := os.Open(info.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return &SegmentSequentialReader{
		info:     info,
		file:     f,
		playlist: playlist,
		segIndex: 0,
	}, nil
}

func (r *SegmentSequentialReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	if r.segIndex >= len(r.playlist) {
		return 0, io.EOF
	}

	if r.segReader == nil {
		if err := r.setupNextSegment(); err != nil {
			return 0, err
		}
	}

	n, err = r.segReader.Read(p)
	if err == io.EOF {
		r.segReader = nil
		r.segIndex++
		if n > 0 {
			return n, nil
		}
		return r.Read(p)
	}
	return n, err
}

func (r *SegmentSequentialReader) setupNextSegment() error {
	if r.segIndex >= len(r.playlist) {
		return io.EOF
	}

	seg := r.playlist[r.segIndex]

	nonce, err := base64.StdEncoding.DecodeString(seg.Nonce)
	if err != nil {
		return fmt.Errorf("failed to decode nonce for segment '%s': %w", seg.ID, err)
	}

	nonceSize := uint64(len(nonce))
	encDataSize := seg.Size - uint64(types.SegmentHeaderSize) - nonceSize
	encData := make([]byte, encDataSize)

	dataOffset := int64(seg.Offset) + int64(types.SegmentHeaderSize) + int64(nonceSize)
	if _, err := r.file.Seek(dataOffset, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to segment '%s': %w", seg.ID, err)
	}
	if _, err := io.ReadFull(r.file, encData); err != nil {
		return fmt.Errorf("failed to read segment '%s': %w", seg.ID, err)
	}

	plainData, err := crypto.DecryptSegment(encData, nonce, r.info.Key)
	if err != nil {
		return fmt.Errorf("failed to decrypt segment '%s': %w", seg.ID, err)
	}

	r.segReader = newBytesReader(plainData)
	return nil
}

func (r *SegmentSequentialReader) Close() error {
	return r.file.Close()
}

type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{data: data, pos: 0}
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func SeekByTime(info *V4ContainerInfo, timeSeconds float64) (segmentIndex int, offsetInSegment int64, err error) {
	segments := info.Manifest.Segments
	if len(segments) == 0 {
		return 0, 0, fmt.Errorf("manifest has no segments")
	}

	for i, seg := range segments {
		segEnd := seg.StartTime + seg.Duration
		if timeSeconds < segEnd || (i == len(segments)-1 && timeSeconds >= seg.StartTime) {
			if timeSeconds < seg.StartTime {
				return i, 0, nil
			}
			progress := (timeSeconds - seg.StartTime) / seg.Duration
			if progress < 0 {
				progress = 0
			}
			if progress > 1 {
				progress = 1
			}
			plainSize := segmentPlainSize(seg)
			offset := int64(progress * float64(plainSize))
			return i, offset, nil
		}
	}

	return 0, 0, fmt.Errorf("time %v is beyond the end of the content", timeSeconds)
}
