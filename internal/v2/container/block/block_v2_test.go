// // internal/v2/container/block_v2_test.go
package block

// import (
// 	"bytes"
// 	"testing"
// )

// func TestReadWriteBlock_v2(t *testing.T) {
// 	data := []byte("this is some test data for a block")
// 	var buf bytes.Buffer

// 	// Write block
// 	err := WriteBlock_v2(&buf, BlockTypeKVI_v2, data)
// 	if err != nil {
// 		t.Fatalf("Failed to write block: %v", err)
// 	}

// 	// Read block
// 	bufReader := bytes.NewReader(buf.Bytes())
// 	header, err := ReadBlockHeader_v2(bufReader)
// 	if err != nil {
// 		t.Fatalf("Failed to read block header: %v", err)
// 	}

// 	if header.Type != BlockTypeKVI_v2 {
// 		t.Errorf("Expected block type %d, got %d", BlockTypeKVI_v2, header.Type)
// 	}
// 	if header.Length != uint64(len(data)) {
// 		t.Errorf("Expected length %d, got %d", len(data), header.Length)
// 	}

// 	readData, err := ReadBlockData_v2(bufReader, header)
// 	if err != nil {
// 		t.Fatalf("Failed to read block data: %v", err)
// 	}

// 	if !bytes.Equal(readData, data) {
// 		t.Errorf("Read data does not match original data")
// 	}
// }
