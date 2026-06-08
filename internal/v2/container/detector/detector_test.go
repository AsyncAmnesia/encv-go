package detector

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"

	"github.com/Soltus/encv-go/internal/v2/types"
)

func buildV4Data(t *testing.T) []byte {
	t.Helper()
	footer := types.EnvelopeFooterV4{Magic: types.MagicFooter_v2}
	footerBuf := &bytes.Buffer{}
	if err := binary.Write(footerBuf, binary.LittleEndian, &footer); err != nil {
		t.Fatalf("failed to write v4 footer: %v", err)
	}
	data := make([]byte, 6+footerBuf.Len())
	copy(data[0:4], types.MagicHeader_v2[:])
	binary.LittleEndian.PutUint16(data[4:6], 0x0004)
	copy(data[6:], footerBuf.Bytes())
	return data
}

func buildV2V3Data(t *testing.T, version uint16) []byte {
	t.Helper()
	footer := types.EnvelopeFooter_v2{Magic: types.MagicFooter_v2}
	footerBuf := &bytes.Buffer{}
	if err := binary.Write(footerBuf, binary.LittleEndian, &footer); err != nil {
		t.Fatalf("failed to write v2 footer: %v", err)
	}
	data := make([]byte, 6+footerBuf.Len())
	copy(data[0:4], types.MagicHeader_v2[:])
	binary.LittleEndian.PutUint16(data[4:6], version)
	copy(data[6:], footerBuf.Bytes())
	return data
}

func TestIsEncvContainerFromBytes_V4Magic(t *testing.T) {
	data := buildV4Data(t)

	ok, err := IsEncvContainerFromBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected true for V4 magic, got false")
	}
}

func TestIsEncvContainerFromBytes_V3Magic(t *testing.T) {
	data := buildV2V3Data(t, 0x0003)

	ok, err := IsEncvContainerFromBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected true for V3 magic, got false")
	}
}

func TestIsEncvContainerFromBytes_V2Magic(t *testing.T) {
	data := buildV2V3Data(t, 0x0002)

	ok, err := IsEncvContainerFromBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected true for V2 magic, got false")
	}
}

func TestIsEncvContainerFromBytes_BadMagic(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"NOTENC", []byte("NOTENC")},
		{"RandomBytes", []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x01}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := IsEncvContainerFromBytes(tc.data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok {
				t.Fatalf("expected false for bad magic %q, got true", string(tc.data))
			}
		})
	}
}

func TestIsEncvContainerFromBytes_ShortData(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"Empty", []byte{}},
		{"OneByte", []byte{0x45}},
		{"ThreeBytes", []byte("ENC")},
		{"FiveBytes", []byte("XXXX\x00")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := IsEncvContainerFromBytes(tc.data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok {
				t.Fatal("expected false for short data, got true")
			}
		})
	}
}

func TestIsEncvContainerFromBytes_Nil(t *testing.T) {
	ok, err := IsEncvContainerFromBytes(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected false for nil input, got false")
	}
}

func TestDetectContainerType_NonExistent(t *testing.T) {
	ct, err := DetectContainerType("/tmp/nonexistent_encv_file_xyz.encv")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
	if ct != types.ContainerTypeUnknown {
		t.Fatalf("expected ContainerTypeUnknown (%d), got %d", types.ContainerTypeUnknown, ct)
	}
}

func TestDetectContainerType_NotAContainer(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/garbage.bin"

	if err := os.WriteFile(path, []byte("this is not an ENCV container at all"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	ct, err := DetectContainerType(path)
	if err == nil {
		t.Fatal("expected error for non-container file, got nil")
	}
	if ct != types.ContainerTypeUnknown {
		t.Fatalf("expected ContainerTypeUnknown (%d), got %d", types.ContainerTypeUnknown, ct)
	}
}
