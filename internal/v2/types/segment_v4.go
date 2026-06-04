package types

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"sync"
)

const SegmentHeaderSize = 18

type SegmentHeader struct {
	SegmentID  uint32
	DataLength uint64
	NonceSize  uint16
	DataCRC32  uint32
}

type KeyframeEntry struct {
	Offset uint64  `json:"offset"`
	Time   float64 `json:"time"`
}

type Segment_v4 struct {
	ID           string          `json:"id"`
	Offset       uint64          `json:"offset"`
	Size         uint64          `json:"size"`
	StartTime    float64         `json:"start_time"`
	Duration     float64         `json:"duration"`
	Nonce        string          `json:"nonce"`
	KeyframeInfo []KeyframeEntry `json:"keyframe_info,omitempty"`
}

type DisasterZone struct {
	Name   string `json:"name"`
	Offset int64  `json:"offset"`
	Size   int64  `json:"size"`
}

type EDLEntry struct {
	Time    float64 `json:"time"`
	Action  string  `json:"action"`
	Segment string  `json:"segment"`
}

type ChapterInfo_v4 struct {
	Time  float64 `json:"time"`
	Title string  `json:"title"`
}

type Manifest_v4 struct {
	Version           uint16              `json:"version"`
	ContainerID       string              `json:"container_id"`
	ContainerType     string              `json:"container_type"`
	IsSeekable        bool                `json:"is_seekable"`
	OriginalDuration  float64             `json:"original_duration,omitempty"`
	Segments          []Segment_v4        `json:"segments"`
	Playlists         map[string][]string `json:"playlists"`
	Chapters          []ChapterInfo_v4    `json:"chapters,omitempty"`
	DisasterZones     []DisasterZone      `json:"disaster_zones,omitempty"`
	KVI               json.RawMessage     `json:"kvi"`
	EDLHistory        []EDLEntry          `json:"edl_history,omitempty"`
	OriginalName      string              `json:"original_name,omitempty"`
	FilenameAlgorithm string              `json:"filename_alg,omitempty"`
}

var manifestV4BufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 4096))
	},
}

func (m *Manifest_v4) SerializeToJSON_v4() ([]byte, error) {
	buf := manifestV4BufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer manifestV4BufferPool.Put(buf)

	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return nil, err
	}

	size := buf.Len()
	if size == 0 {
		return nil, nil
	}
	out := make([]byte, size-1)
	copy(out, buf.Bytes()[:size-1])
	return out, nil
}

func DeserializeManifest_v4(data []byte) (*Manifest_v4, error) {
	var m Manifest_v4
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Manifest_v4) GetSegmentByID(id string) (*Segment_v4, error) {
	for _, seg := range m.Segments {
		if seg.ID == id {
			return &seg, nil
		}
	}
	return nil, fmt.Errorf("segment with ID '%s' not found in manifest", id)
}

func (m *Manifest_v4) GetPlaylist(name string) ([]string, error) {
	if name == "" {
		name = "default"
	}
	ids, ok := m.Playlists[name]
	if !ok {
		return nil, fmt.Errorf("playlist '%s' not found in manifest", name)
	}
	return ids, nil
}

func (m *Manifest_v4) ResolvePlaylist(name string) ([]Segment_v4, error) {
	ids, err := m.GetPlaylist(name)
	if err != nil {
		return nil, err
	}

	segments := make([]Segment_v4, 0, len(ids))
	for _, id := range ids {
		seg, err := m.GetSegmentByID(id)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve segment '%s' in playlist: %w", id, err)
		}
		segments = append(segments, *seg)
	}
	return segments, nil
}

func (m *Manifest_v4) GetOriginalName() string      { return m.OriginalName }
func (m *Manifest_v4) GetFilenameAlgorithm() string { return m.FilenameAlgorithm }

func (h *SegmentHeader) MarshalBinary() ([]byte, error) {
	buf := make([]byte, SegmentHeaderSize)
	ByteOrder_v2.PutUint32(buf[0:4], h.SegmentID)
	ByteOrder_v2.PutUint64(buf[4:12], h.DataLength)
	ByteOrder_v2.PutUint16(buf[12:14], h.NonceSize)
	ByteOrder_v2.PutUint32(buf[14:18], h.DataCRC32)
	return buf, nil
}

func (h *SegmentHeader) UnmarshalBinary(data []byte) error {
	if len(data) < SegmentHeaderSize {
		return fmt.Errorf("segment header requires at least %d bytes, got %d", SegmentHeaderSize, len(data))
	}
	h.SegmentID = ByteOrder_v2.Uint32(data[0:4])
	h.DataLength = ByteOrder_v2.Uint64(data[4:12])
	h.NonceSize = ByteOrder_v2.Uint16(data[12:14])
	h.DataCRC32 = ByteOrder_v2.Uint32(data[14:18])
	return nil
}

var _ encoding.BinaryMarshaler = (*SegmentHeader)(nil)
var _ encoding.BinaryUnmarshaler = (*SegmentHeader)(nil)
