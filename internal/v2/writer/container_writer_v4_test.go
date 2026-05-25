package writer

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/Soltus/encv-go/internal/v2/container/detector"
	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/reader"
	"github.com/Soltus/encv-go/internal/v2/types"
)

func makeTestKVI(salt []byte) json.RawMessage {
	kvi := map[string]string{
		"salt_base64": base64.StdEncoding.EncodeToString(salt),
		"iv_base64":   base64.StdEncoding.EncodeToString(make([]byte, crypto.IVSize_v2)),
	}
	data, _ := json.Marshal(kvi)
	return data
}

func makeTestManifest(salt []byte, segments []types.Segment_v4, playlistIDs []string) *types.Manifest_v4 {
	return &types.Manifest_v4{
		Version:       4,
		ContainerID:   "test-container",
		ContainerType: "video",
		IsSeekable:    true,
		Segments:      segments,
		Playlists:     map[string][]string{"default": playlistIDs},
		KVI:           makeTestKVI(salt),
	}
}

func writeAndOpenContainer(t *testing.T, password string, manifest *types.Manifest_v4, segResults []*crypto.SegmentEncryptionResult) (*reader.V4ContainerInfo, string) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "v4test-*.encv")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	err = WriteV4Container(&V4WriteParams{
		OutputPath:     tmpPath,
		IsMain:         true,
		ContainerType:  types.ContainerTypeVideo,
		IsSeekable:     true,
		IDType:         types.IDType_Raw,
		Manifest:       manifest,
		SegmentResults: segResults,
	})
	if err != nil {
		os.Remove(tmpPath)
		t.Fatalf("WriteV4Container: %v", err)
	}

	info, err := reader.OpenV4Container(tmpPath, password)
	if err != nil {
		os.Remove(tmpPath)
		t.Fatalf("OpenV4Container: %v", err)
	}

	return info, tmpPath
}

func TestV4ContainerSingleSegment(t *testing.T) {
	testData := make([]byte, 1024)
	if _, err := rand.Read(testData); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	salt, err := crypto.GenerateSalt_v2(16)
	if err != nil {
		t.Fatalf("GenerateSalt_v2: %v", err)
	}
	key := crypto.GenerateKey_v2("testpassword", salt, 32)

	encResult, err := crypto.EncryptSegment(testData, key, 0)
	if err != nil {
		t.Fatalf("EncryptSegment: %v", err)
	}

	segments := []types.Segment_v4{
		{ID: "seg-0", StartTime: 0, Duration: 10},
	}
	manifest := makeTestManifest(salt, segments, []string{"seg-0"})

	info, tmpPath := writeAndOpenContainer(t, "testpassword", manifest, []*crypto.SegmentEncryptionResult{encResult})
	defer os.Remove(tmpPath)

	sr, err := reader.NewSegmentSeekableReader(info, "default")
	if err != nil {
		t.Fatalf("NewSegmentSeekableReader: %v", err)
	}
	defer sr.Close()

	decrypted := make([]byte, len(testData))
	_, err = io.ReadFull(sr, decrypted)
	if err != nil {
		t.Fatalf("ReadFull: %v", err)
	}

	if !bytes.Equal(decrypted, testData) {
		t.Errorf("decrypted data does not match original")
	}
}

func TestV4ContainerMultipleSegments(t *testing.T) {
	const numSegments = 3
	const segDataSize = 2048

	salt, err := crypto.GenerateSalt_v2(16)
	if err != nil {
		t.Fatalf("GenerateSalt_v2: %v", err)
	}
	key := crypto.GenerateKey_v2("testpassword", salt, 32)

	testDatas := make([][]byte, numSegments)
	segResults := make([]*crypto.SegmentEncryptionResult, numSegments)
	segments := make([]types.Segment_v4, numSegments)
	playlistIDs := make([]string, numSegments)

	for i := 0; i < numSegments; i++ {
		testDatas[i] = make([]byte, segDataSize)
		if _, err := rand.Read(testDatas[i]); err != nil {
			t.Fatalf("rand.Read segment %d: %v", i, err)
		}

		encResult, err := crypto.EncryptSegment(testDatas[i], key, uint32(i))
		if err != nil {
			t.Fatalf("EncryptSegment %d: %v", i, err)
		}
		segResults[i] = encResult

		segID := fmt.Sprintf("seg-%d", i)
		segments[i] = types.Segment_v4{
			ID:        segID,
			StartTime: float64(i * 10),
			Duration:  10,
		}
		playlistIDs[i] = segID
	}

	manifest := makeTestManifest(salt, segments, playlistIDs)

	info, tmpPath := writeAndOpenContainer(t, "testpassword", manifest, segResults)
	defer os.Remove(tmpPath)

	sr, err := reader.NewSegmentSeekableReader(info, "default")
	if err != nil {
		t.Fatalf("NewSegmentSeekableReader: %v", err)
	}
	defer sr.Close()

	allDecrypted, err := io.ReadAll(sr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	expected := make([]byte, 0, numSegments*segDataSize)
	for _, d := range testDatas {
		expected = append(expected, d...)
	}

	if !bytes.Equal(allDecrypted, expected) {
		t.Errorf("decrypted data does not match original concatenated data (got %d bytes, want %d)", len(allDecrypted), len(expected))
	}
}

func TestV4ContainerEmptyContainer(t *testing.T) {
	salt, err := crypto.GenerateSalt_v2(16)
	if err != nil {
		t.Fatalf("GenerateSalt_v2: %v", err)
	}

	manifest := makeTestManifest(salt, []types.Segment_v4{}, []string{})

	tmpFile, err := os.CreateTemp("", "v4test-empty-*.encv")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	err = WriteV4EmptyContainer(&V4WriteParams{
		OutputPath:    tmpPath,
		IsMain:        true,
		ContainerType: types.ContainerTypeVideo,
		IsSeekable:    true,
		IDType:        types.IDType_Raw,
		Manifest:      manifest,
	})
	if err != nil {
		t.Fatalf("WriteV4EmptyContainer: %v", err)
	}

	info, err := reader.OpenV4Container(tmpPath, "testpassword")
	if err != nil {
		t.Fatalf("OpenV4Container: %v", err)
	}

	if len(info.Manifest.Segments) != 0 {
		t.Errorf("expected empty segments, got %d", len(info.Manifest.Segments))
	}
}

func TestV4ContainerSegmentIndependence(t *testing.T) {
	const numSegments = 3
	const segDataSize = 2048

	salt, err := crypto.GenerateSalt_v2(16)
	if err != nil {
		t.Fatalf("GenerateSalt_v2: %v", err)
	}
	key := crypto.GenerateKey_v2("testpassword", salt, 32)

	testDatas := make([][]byte, numSegments)
	segResults := make([]*crypto.SegmentEncryptionResult, numSegments)
	segments := make([]types.Segment_v4, numSegments)
	playlistIDs := make([]string, numSegments)

	for i := 0; i < numSegments; i++ {
		testDatas[i] = make([]byte, segDataSize)
		if _, err := rand.Read(testDatas[i]); err != nil {
			t.Fatalf("rand.Read segment %d: %v", i, err)
		}

		encResult, err := crypto.EncryptSegment(testDatas[i], key, uint32(i))
		if err != nil {
			t.Fatalf("EncryptSegment %d: %v", i, err)
		}
		segResults[i] = encResult

		segID := fmt.Sprintf("seg-%d", i)
		segments[i] = types.Segment_v4{
			ID:        segID,
			StartTime: float64(i * 10),
			Duration:  10,
		}
		playlistIDs[i] = segID
	}

	manifest := makeTestManifest(salt, segments, playlistIDs)

	info, tmpPath := writeAndOpenContainer(t, "testpassword", manifest, segResults)
	defer os.Remove(tmpPath)

	sr, err := reader.NewSegmentSeekableReader(info, "default")
	if err != nil {
		t.Fatalf("NewSegmentSeekableReader: %v", err)
	}
	defer sr.Close()

	seg0Data := make([]byte, segDataSize)
	n, err := sr.ReadAt(seg0Data, 0)
	if err != nil && err != io.EOF {
		t.Fatalf("ReadAt segment 0: %v", err)
	}
	if n != segDataSize {
		t.Fatalf("ReadAt segment 0: expected %d bytes, got %d", segDataSize, n)
	}
	if !bytes.Equal(seg0Data, testDatas[0]) {
		t.Errorf("segment 0 data does not match original")
	}

	seg2Data := make([]byte, segDataSize)
	n, err = sr.ReadAt(seg2Data, int64(2*segDataSize))
	if err != nil && err != io.EOF {
		t.Fatalf("ReadAt segment 2: %v", err)
	}
	if n != segDataSize {
		t.Fatalf("ReadAt segment 2: expected %d bytes, got %d", segDataSize, n)
	}
	if !bytes.Equal(seg2Data, testDatas[2]) {
		t.Errorf("segment 2 data does not match original")
	}
}

func TestV4ContainerManifestObfuscation(t *testing.T) {
	testData := make([]byte, 1024)
	if _, err := rand.Read(testData); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	salt, err := crypto.GenerateSalt_v2(16)
	if err != nil {
		t.Fatalf("GenerateSalt_v2: %v", err)
	}
	key := crypto.GenerateKey_v2("testpassword", salt, 32)

	encResult, err := crypto.EncryptSegment(testData, key, 0)
	if err != nil {
		t.Fatalf("EncryptSegment: %v", err)
	}

	segments := []types.Segment_v4{
		{ID: "seg-0", StartTime: 0, Duration: 10},
	}
	manifest := makeTestManifest(salt, segments, []string{"seg-0"})

	info, tmpPath := writeAndOpenContainer(t, "testpassword", manifest, []*crypto.SegmentEncryptionResult{encResult})
	defer os.Remove(tmpPath)

	rawData, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	manifestStart := int(info.Header.ManifestOffset)
	manifestEnd := manifestStart + int(info.Header.ManifestLength)
	if manifestEnd > len(rawData) {
		t.Fatalf("manifest range [%d:%d] exceeds file size %d", manifestStart, manifestEnd, len(rawData))
	}

	rawManifest := rawData[manifestStart:manifestEnd]

	if bytes.Contains(rawManifest, []byte("version")) {
		t.Errorf("obfuscated manifest contains plaintext JSON key 'version'")
	}
	if bytes.Contains(rawManifest, []byte("segments")) {
		t.Errorf("obfuscated manifest contains plaintext JSON key 'segments'")
	}
	if bytes.Contains(rawManifest, []byte("container_id")) {
		t.Errorf("obfuscated manifest contains plaintext JSON key 'container_id'")
	}
}

func TestV4ContainerDetection(t *testing.T) {
	testData := make([]byte, 512)
	if _, err := rand.Read(testData); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	salt, err := crypto.GenerateSalt_v2(16)
	if err != nil {
		t.Fatalf("GenerateSalt_v2: %v", err)
	}
	key := crypto.GenerateKey_v2("testpassword", salt, 32)

	encResult, err := crypto.EncryptSegment(testData, key, 0)
	if err != nil {
		t.Fatalf("EncryptSegment: %v", err)
	}

	segments := []types.Segment_v4{
		{ID: "seg-0", StartTime: 0, Duration: 10},
	}
	manifest := makeTestManifest(salt, segments, []string{"seg-0"})

	_, tmpPath := writeAndOpenContainer(t, "testpassword", manifest, []*crypto.SegmentEncryptionResult{encResult})
	defer os.Remove(tmpPath)

	containerType, err := detector.DetectContainerType(tmpPath)
	if err != nil {
		t.Fatalf("DetectContainerType: %v", err)
	}
	if containerType != types.ContainerTypeVideo {
		t.Errorf("expected ContainerTypeVideo (%d), got %d", types.ContainerTypeVideo, containerType)
	}

	isSeekable, err := detector.DetectIsSeekable(tmpPath)
	if err != nil {
		t.Fatalf("DetectIsSeekable: %v", err)
	}
	if !isSeekable {
		t.Errorf("expected seekable container, got non-seekable")
	}
}

func TestV4ContainerSeekByTime(t *testing.T) {
	const numSegments = 3
	const segDataSize = 2048

	salt, err := crypto.GenerateSalt_v2(16)
	if err != nil {
		t.Fatalf("GenerateSalt_v2: %v", err)
	}
	key := crypto.GenerateKey_v2("testpassword", salt, 32)

	testDatas := make([][]byte, numSegments)
	segResults := make([]*crypto.SegmentEncryptionResult, numSegments)
	segments := make([]types.Segment_v4, numSegments)
	playlistIDs := make([]string, numSegments)

	for i := 0; i < numSegments; i++ {
		testDatas[i] = make([]byte, segDataSize)
		if _, err := rand.Read(testDatas[i]); err != nil {
			t.Fatalf("rand.Read segment %d: %v", i, err)
		}

		encResult, err := crypto.EncryptSegment(testDatas[i], key, uint32(i))
		if err != nil {
			t.Fatalf("EncryptSegment %d: %v", i, err)
		}
		segResults[i] = encResult

		segID := fmt.Sprintf("seg-%d", i)
		segments[i] = types.Segment_v4{
			ID:        segID,
			StartTime: float64(i * 10),
			Duration:  10,
		}
		playlistIDs[i] = segID
	}

	manifest := makeTestManifest(salt, segments, playlistIDs)

	info, tmpPath := writeAndOpenContainer(t, "testpassword", manifest, segResults)
	defer os.Remove(tmpPath)

	segIdx, _, err := reader.SeekByTime(info, 5.0)
	if err != nil {
		t.Fatalf("SeekByTime(5.0): %v", err)
	}
	if segIdx != 0 {
		t.Errorf("SeekByTime(5.0): expected segment 0, got %d", segIdx)
	}

	segIdx, _, err = reader.SeekByTime(info, 15.0)
	if err != nil {
		t.Fatalf("SeekByTime(15.0): %v", err)
	}
	if segIdx != 1 {
		t.Errorf("SeekByTime(15.0): expected segment 1, got %d", segIdx)
	}

	segIdx, _, err = reader.SeekByTime(info, 25.0)
	if err != nil {
		t.Fatalf("SeekByTime(25.0): %v", err)
	}
	if segIdx != 2 {
		t.Errorf("SeekByTime(25.0): expected segment 2, got %d", segIdx)
	}

	segIdx, _, err = reader.SeekByTime(info, 0.0)
	if err != nil {
		t.Fatalf("SeekByTime(0.0): %v", err)
	}
	if segIdx != 0 {
		t.Errorf("SeekByTime(0.0): expected segment 0, got %d", segIdx)
	}
}
