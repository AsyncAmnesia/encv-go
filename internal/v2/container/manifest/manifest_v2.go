// internal/v2/container/manifest_v2.go
package manifest

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/Soltus/encv-go/internal/v2/container/block"
	"github.com/Soltus/encv-go/internal/v2/container/envelope"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// DeserializeFromJSON_v2 从 JSON 字节反序列化清单
func DeserializeFromJSON_v2(data []byte) (*types.Manifest_v2, error) {
	var manifest types.Manifest_v2
	err := json.Unmarshal(data, &manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}
	return &manifest, nil
}

// ReadManifestFromFile 直接从文件路径读取并解析 Manifest 和 Footer
// 这是一个低级函数，用于避免包之间的循环依赖
func ReadManifestFromFile(filePath string) (*types.Manifest_v2, *types.EnvelopeFooter_v2, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// 1. 读取 Footer
	footer, err := envelope.ReadEnvelopeFooter_v2(file)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read envelope footer: %w", err)
	}

	// 2. 定位到 Manifest
	if _, err := file.Seek(int64(footer.ManifestOffset), io.SeekStart); err != nil {
		return nil, nil, fmt.Errorf("failed to seek to manifest: %w", err)
	}

	// 3. 读取 Manifest 块头和数据
	manifestHeader, err := block.ReadBlockHeader_v2(file)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read manifest block header: %w", err)
	}
	if manifestHeader.Type != types.BlockTypeManifest_v2 {
		return nil, nil, fmt.Errorf("expected manifest block type, got %d", manifestHeader.Type)
	}

	manifestData, err := block.ReadBlockData_v2(file, manifestHeader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read manifest data: %w", err)
	}

	// 4. 反序列化
	manifest, err := DeserializeFromJSON_v2(manifestData)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to deserialize manifest: %w", err)
	}

	return manifest, footer, nil
}

// ScanManifestFromFile 从头扫描文件，寻找 Manifest 块
func ScanManifestFromFile(filePath string) (*types.Manifest_v2, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	for {
		// 1. 读取块头
		var header block.BlockHeader_v2
		if err := binary.Read(file, binary.LittleEndian, &header); err != nil {
			if err == io.EOF {
				break // 文件结束
			}
			return nil, fmt.Errorf("failed to read block header during scan: %w", err)
		}

		// 2. 检查是否是 Manifest 块
		if header.Type == types.BlockTypeManifest_v2 {
			// log.Printf("DEBUG: Found Manifest block at offset %d", block.GetBlockHeader_v2_Size())
			// 3. 读取块数据
			data := make([]byte, header.Length)
			if _, err := io.ReadFull(file, data); err != nil {
				return nil, fmt.Errorf("failed to read manifest data: %w", err)
			}

			// 4. 反序列化
			var manifest types.Manifest_v2
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, fmt.Errorf("failed to unmarshal scanned manifest: %w", err)
			}
			return &manifest, nil
		}

		// 3. 如果不是，跳过这个块的数据
		if _, err := file.Seek(int64(header.Length), io.SeekCurrent); err != nil {
			return nil, fmt.Errorf("failed to skip block data during scan: %w", err)
		}
	}

	return nil, fmt.Errorf("manifest block not found in file")
}

// ExtractKVI_v2 从 v2 容器文件中直接扫描并提取 KVI 块的数据，不依赖清单。
func ExtractKVI_v2(containerPath string) ([]byte, error) {
	file, err := os.Open(containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file '%s': %w", containerPath, err)
	}
	defer file.Close()

	// log.Printf("INFO: Scanning '%s' for KVI block...", containerPath)

	// 从头开始扫描，直到找到 KVI 块
	for {
		// 记录当前块的起始偏移量
		blockStartOffset, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("failed to get current offset: %w", err)
		}

		// 读取块头
		header, err := block.ReadBlockHeader_v2(file)
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("reached end of file but KVI block was not found in '%s'", containerPath)
			}
			return nil, fmt.Errorf("failed to read block header: %w", err)
		}

		// 如果是 KVI 块，就读取它的数据
		if header.Type == types.BlockTypeKVI_v2 {
			log.Printf("DEBUG: Found KVI block at offset %d (length %d)", blockStartOffset, header.Length)
			// 使用 ReadBlockData_v2 来读取并验证 CRC
			kviData, err := block.ReadBlockData_v2(file, header)
			if err != nil {
				return nil, fmt.Errorf("failed to read KVI block data: %w", err)
			}
			return kviData, nil
		}

		// 如果不是 KVI 块，就跳过它
		_, err = file.Seek(int64(header.Length), io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("failed to seek past block data: %w", err)
		}
	}
}

// ExtractManifest_v2 从 v2 容器文件中直接扫描并提取 Manifest 块的数据。
func ExtractManifest_v2(containerPath string) ([]byte, error) {
	file, err := os.Open(containerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file '%s': %w", containerPath, err)
	}
	defer file.Close()

	// log.Printf("INFO: Scanning '%s' for Manifest block...", containerPath)

	for {
		header, err := block.ReadBlockHeader_v2(file)
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("reached end of file but Manifest block was not found in '%s'", containerPath)
			}
			return nil, fmt.Errorf("failed to read block header: %w", err)
		}

		if header.Type == types.BlockTypeManifest_v2 {
			// log.Printf("DEBUG: Found Manifest block.")
			manifestData, err := block.ReadBlockData_v2(file, header)
			if err != nil {
				return nil, fmt.Errorf("failed to read Manifest block data: %w", err)
			}
			return manifestData, nil
		}

		_, err = file.Seek(int64(header.Length), io.SeekCurrent)
		if err != nil {
			return nil, fmt.Errorf("failed to seek past block data: %w", err)
		}
	}
}

// 从指定偏移量读取 Manifest
func ReadManifestAt(filePath string, offset int64, length int64) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	header, err := block.ReadBlockHeader_v2(file)
	if err != nil {
		return nil, err
	}

	if header.Type != types.BlockTypeManifest_v2 {
		return nil, fmt.Errorf("block at offset %d is not a manifest", offset)
	}

	return block.ReadBlockData_v2(file, header)
}

// ParseManifestFromBytes 从 JSON 字节切片中解析 Manifest
func ParseManifestFromBytes(data []byte) (*types.Manifest_v2, error) {
	return DeserializeFromJSON_v2(data)
}
