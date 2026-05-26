package handle

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/types"
)

func TestOpen_InvalidMagic(t *testing.T) {
	data := []byte("NOTENC")
	source := NewBytesSource(data, "test.invalid")

	handle, err := Open(source)
	if err == nil {
		t.Fatal("expected error for invalid magic, got nil")
	}
	if handle != nil {
		t.Fatal("expected nil handle on error")
	}
	if !strings.Contains(err.Error(), "not an ENCV container") {
		t.Fatalf("error message should mention 'not an ENCV container', got: %s", err.Error())
	}
}

func TestOpen_TruncatedData(t *testing.T) {
	data := make([]byte, 4)
	source := NewBytesSource(data, "test.truncated")

	handle, err := Open(source)
	if err == nil {
		t.Fatal("expected error for truncated data, got nil")
	}
	if handle != nil {
		t.Fatal("expected nil handle on error")
	}
}

func TestOpen_V4_ValidContainer(t *testing.T) {
	v4Manifest := &types.Manifest_v4{
		Version:          4,
		ContainerID:      "test-container-001",
		ContainerType:    "video",
		IsSeekable:       true,
		OriginalDuration: 120.5,
		Segments: []types.Segment_v4{
			{ID: "seg-0", Offset: 2060, Size: 102400, StartTime: 0.0, Duration: 10.0, Nonce: ""},
			{ID: "seg-1", Offset: 112460, Size: 204800, StartTime: 10.0, Duration: 20.0, Nonce: ""},
		},
		KVI:       []byte(`{"salt_base64":"dGVzdA==","iv_base64":"dGVzdA=="}`),
		Playlists: map[string][]string{"default": {"seg-0", "seg-1"}},
	}

	manifestJSON, err := v4Manifest.SerializeToJSON_v4()
	if err != nil {
		t.Fatalf("failed to serialize v4 manifest: %v", err)
	}

	obfuscatedManifest, err := crypto.ObfuscateManifest(manifestJSON)
	if err != nil {
		t.Fatalf("failed to obfuscate manifest: %v", err)
	}

	headerSize := types.EnvelopeHeaderSize_v4
	footerSize := types.EnvelopeFooterSize_v4
	manifestOffset := uint64(headerSize)
	manifestLength := uint64(len(obfuscatedManifest))

	header := &types.EnvelopeHeaderV4{
		Magic:          types.MagicHeader_v2,
		Version:        0x04,
		Flags:          0x0001,
		ContainerType:  types.ContainerTypeVideo,
		IsSeekable:     1,
		IDType:         0,
		IDLength:       0,
		ManifestOffset: manifestOffset,
		ManifestLength: manifestLength,
	}

	var headerBuf bytes.Buffer
	if err := types.WriteHeaderV4(&headerBuf, header); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}

	footer := &types.EnvelopeFooterV4{
		Magic:       types.MagicFooter_v2,
		GlobalCRC32: 0,
		Reserved:    [4]byte{},
	}

	var footerBuf bytes.Buffer
	if err := types.WriteFooterV4(&footerBuf, footer); err != nil {
		t.Fatalf("failed to write footer: %v", err)
	}

	totalSize := int(headerSize) + len(obfuscatedManifest) + int(footerSize)
	containerData := make([]byte, totalSize)
	copy(containerData[:headerSize], headerBuf.Bytes())
	copy(containerData[headerSize:headerSize+len(obfuscatedManifest)], obfuscatedManifest)
	copy(containerData[totalSize-footerSize:], footerBuf.Bytes())

	source := NewBytesSource(containerData, "test.encv")

	h, err := Open(source)
	if err != nil {
		t.Fatalf("failed to open valid v4 container: %v", err)
	}
	defer h.Close()

	if h.Version() != 4 {
		t.Errorf("expected Version()=4, got %d", h.Version())
	}
	if h.HeaderSize() != types.EnvelopeHeaderSize_v4 {
		t.Errorf("expected HeaderSize()=%d, got %d", types.EnvelopeHeaderSize_v4, h.HeaderSize())
	}
	if h.ContainerType() != types.ContainerTypeVideo {
		t.Errorf("expected ContainerType()=%d, got %d", types.ContainerTypeVideo, h.ContainerType())
	}
	if !h.IsSeekable() {
		t.Error("expected IsSeekable()=true, got false")
	}
	if h.ContainerID() != "test-container-001" {
		t.Errorf("expected ContainerID()='test-container-001', got '%s'", h.ContainerID())
	}
	if h.OriginalDuration() != 120.5 {
		t.Errorf("expected OriginalDuration()=120.5, got %f", h.OriginalDuration())
	}

	hdrV4 := h.HeaderV4()
	if hdrV4 == nil {
		t.Fatal("HeaderV4() returned nil")
	}
	if hdrV4.Magic != types.MagicHeader_v2 {
		t.Error("header magic mismatch")
	}
	if hdrV4.Version != 4 {
		t.Errorf("header Version=%d, want 4", hdrV4.Version)
	}
	if hdrV4.ManifestOffset != manifestOffset {
		t.Errorf("header ManifestOffset=%d, want %d", hdrV4.ManifestOffset, manifestOffset)
	}
	if hdrV4.ManifestLength != manifestLength {
		t.Errorf("header ManifestLength=%d, want %d", hdrV4.ManifestLength, manifestLength)
	}

	ftrV4 := h.FooterV4()
	if ftrV4 == nil {
		t.Fatal("FooterV4() returned nil")
	}
	if ftrV4.Magic != types.MagicFooter_v2 {
		t.Error("footer magic mismatch")
	}

	manifestV4 := h.ManifestV4()
	if manifestV4 == nil {
		t.Fatal("ManifestV4() returned nil")
	}
	if manifestV4.ContainerID != "test-container-001" {
		t.Errorf("manifest ContainerID='%s', want 'test-container-001'", manifestV4.ContainerID)
	}
	if len(manifestV4.Segments) != 2 {
		t.Errorf("manifest has %d segments, want 2", len(manifestV4.Segments))
	}
	if manifestV4.OriginalDuration != 120.5 {
		t.Errorf("manifest OriginalDuration=%f, want 120.5", manifestV4.OriginalDuration)
	}

	manifestV2 := h.Manifest()
	if manifestV2 == nil {
		t.Fatal("Manifest() returned nil")
	}
	if len(manifestV2.Fragments) != 2 {
		t.Errorf("adapted manifest has %d fragments, want 2", len(manifestV2.Fragments))
	}

	src := h.Source()
	if src == nil {
		t.Fatal("Source() returned nil")
	}
	if src.Size() != int64(totalSize) {
		t.Errorf("source Size()=%d, want %d", src.Size(), totalSize)
	}
	if src.Name() != "test.encv" {
		t.Errorf("source Name()='%s', want 'test.encv'", src.Name())
	}
}

func TestOpen_V4_BadHeaderCRC32(t *testing.T) {
	headerData := make([]byte, types.EnvelopeHeaderSize_v4)
	copy(headerData[0:4], types.MagicHeader_v2[:])
	binary.LittleEndian.PutUint16(headerData[4:6], 0x04)

	footerData := make([]byte, types.EnvelopeFooterSize_v4)
	copy(footerData[0:4], types.MagicFooter_v2[:])

	containerData := append(headerData, footerData...)
	source := NewBytesSource(containerData, "test-bad-crc.encv")

	_, err := Open(source)
	if err == nil {
		t.Fatal("expected error for bad CRC32, got nil")
	}
	if !strings.Contains(err.Error(), "CRC32") {
		t.Errorf("error should mention CRC32, got: %s", err.Error())
	}
}

func TestOpen_V4_BadFooterMagic(t *testing.T) {
	v4Manifest := &types.Manifest_v4{
		Version:       4,
		ContainerID:   "test-crc",
		ContainerType: "video",
		Segments:      []types.Segment_v4{{ID: "s0", Offset: 2060, Size: 100, StartTime: 0, Duration: 1, Nonce: ""}},
		KVI:           []byte(`{}`),
	}

	manifestJSON, _ := v4Manifest.SerializeToJSON_v4()
	obfuscatedManifest, _ := crypto.ObfuscateManifest(manifestJSON)

	header := &types.EnvelopeHeaderV4{
		Magic:          types.MagicHeader_v2,
		Version:        0x04,
		ContainerType:  types.ContainerTypeVideo,
		ManifestOffset: uint64(types.EnvelopeHeaderSize_v4),
		ManifestLength: uint64(len(obfuscatedManifest)),
	}
	var headerBuf bytes.Buffer
	types.WriteHeaderV4(&headerBuf, header)

	badFooter := make([]byte, types.EnvelopeFooterSize_v4)
	copy(badFooter[0:4], "BAD!")

	totalSize := types.EnvelopeHeaderSize_v4 + len(obfuscatedManifest) + types.EnvelopeFooterSize_v4
	containerData := make([]byte, totalSize)
	copy(containerData[:types.EnvelopeHeaderSize_v4], headerBuf.Bytes())
	copy(containerData[types.EnvelopeHeaderSize_v4:types.EnvelopeHeaderSize_v4+len(obfuscatedManifest)], obfuscatedManifest)
	copy(containerData[totalSize-types.EnvelopeFooterSize_v4:], badFooter)

	source := NewBytesSource(containerData, "test-bad-footer.encv")

	_, err := Open(source)
	if err == nil {
		t.Fatal("expected error for bad footer magic, got nil")
	}
	if !strings.Contains(err.Error(), "invalid magic") {
		t.Errorf("error should mention invalid magic, got: %s", err.Error())
	}
}

func TestAdaptV4ToV2_Mapping(t *testing.T) {
	v4Manifest := &types.Manifest_v4{
		Version:       4,
		ContainerID:   "adapt-test",
		ContainerType: "audio",
		IsSeekable:    false,
		Segments: []types.Segment_v4{
			{ID: "seg-a", Offset: 1000, Size: 50000, StartTime: 0.0, Duration: 5.0, Nonce: "AAAA"},
			{ID: "seg-b", Offset: 51018, Size: 100000, StartTime: 5.0, Duration: 15.0, Nonce: "BBBB"},
			{ID: "seg-c", Offset: 151036, Size: 75000, StartTime: 20.0, Duration: 10.0, Nonce: ""},
		},
		KVI: []byte(`{"salt_base64":"abc","iv_base64":"def"}`),
	}

	header := &types.EnvelopeHeaderV4{
		Version: 0x04,
	}

	result := AdaptV4ToV2(v4Manifest, header)

	if result == nil {
		t.Fatal("AdaptV4ToV2 returned nil")
	}

	if len(result.Fragments) != len(v4Manifest.Segments) {
		t.Fatalf("fragment count mismatch: got %d, want %d", len(result.Fragments), len(v4Manifest.Segments))
	}

	for i, seg := range v4Manifest.Segments {
		if result.Fragments[i].ID != seg.ID {
			t.Errorf("Fragment[%d].ID=%q, want %q", i, result.Fragments[i].ID, seg.ID)
		}
		if result.Fragments[i].Type != types.FragmentType_SeekableStream {
			t.Errorf("Fragment[%d].Type=%q, want %q", i, result.Fragments[i].Type, types.FragmentType_SeekableStream)
		}
	}

	if result.Kind != "audio" {
		t.Errorf("Kind=%q, want \"audio\"", result.Kind)
	}
	if result.Version != int64(header.Version) {
		t.Errorf("Version=%d, want %d", result.Version, header.Version)
	}
}
