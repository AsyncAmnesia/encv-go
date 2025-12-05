package reader

import (
	"fmt"
	"io"
	"log"

	"github.com/Soltus/encv-go/internal/v2/container/block"
	"github.com/Soltus/encv-go/internal/v2/types"
)

// scanPhysicalOffsets 统一扫描并建立所有在主文件中的分片的物理偏移映射。
// 它不再区分“单文件”或“物理分片”容器，而是只关注“哪些分片在主文件中”。
func (r *fileContainerReader) scanPhysicalOffsets() error {
	// 1. 筛选出所有需要从主文件中查找的分片
	var fragmentsInMainFile []types.Fragment_v2
	for _, frag := range r.manifest.Fragments {
		// 只有当 PhysicalPath 为空时，才表示它在主文件中
		if frag.PhysicalPath == "" {
			fragmentsInMainFile = append(fragmentsInMainFile, frag)
		}
	}

	// 2. 如果没有分片在主文件中，则无需扫描，直接返回。
	// 这种情况在理论上可能发生，虽然罕见。
	if len(fragmentsInMainFile) == 0 {
		log.Printf("INFO: No fragments found in the main file. Skipping physical offset scan.")
		return nil
	}

	// 3. 委托给专门的扫描器，只扫描主文件中的分片
	log.Printf("INFO: Found %d fragments in main file. Starting physical offset scan.", len(fragmentsInMainFile))
	return r.scanForDataBlocksInMainFile(fragmentsInMainFile)
}

// scanForDataBlocksInMainFile 扫描主文件，为指定的分片列表建立 ID -> Offset 映射。
// 【关键修改】它不再假设所有分片都在主文件，而是只处理传入的列表。
func (r *fileContainerReader) scanForDataBlocksInMainFile(fragmentsToScan []types.Fragment_v2) error {
	manifestBlockOffset, err := r.findManifestBlockOffset()
	if err != nil {
		return fmt.Errorf("failed to find manifest block for pre-scan: %w", err)
	}

	mainFile, err := globalFileHandlePool.Get(r.mainFilePath)
	if err != nil {
		return err
	}
	defer globalFileHandlePool.Put(mainFile)

	if _, err := mainFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to start for scanning: %w", err)
	}

	fragIndex := 0
	for {
		blockStartOffset, err := mainFile.Seek(0, io.SeekCurrent)
		if err != nil {
			return fmt.Errorf("failed to get current offset: %w", err)
		}
		if blockStartOffset >= manifestBlockOffset {
			break // 扫描到 Manifest 块即停止
		}

		header, err := block.ReadBlockHeader_v2(mainFile)
		if err != nil {
			return fmt.Errorf("failed to read block header at offset %d: %w", blockStartOffset, err)
		}

		if header.Type == types.BlockTypeData_v2 {
			if fragIndex >= len(fragmentsToScan) {
				return fmt.Errorf("found more data blocks in main file than expected (found %d, expected %d)", fragIndex+1, len(fragmentsToScan))
			}

			// 建立映射
			frag := fragmentsToScan[fragIndex]
			r.physicalOffsets[frag.ID] = uint64(blockStartOffset)
			log.Printf("DEBUG: Mapped fragment '%s' to offset %d", frag.ID, blockStartOffset)
			fragIndex++
		}

		if _, err = mainFile.Seek(int64(header.Length), io.SeekCurrent); err != nil {
			return fmt.Errorf("failed to seek past block data at offset %d: %w", blockStartOffset, err)
		}
	}

	if fragIndex != len(fragmentsToScan) {
		return fmt.Errorf("scan finished, but found %d data blocks in main file, expected %d", fragIndex, len(fragmentsToScan))
	}

	log.Printf("INFO: All %d fragments in main file successfully mapped.", len(fragmentsToScan))
	return nil
}
