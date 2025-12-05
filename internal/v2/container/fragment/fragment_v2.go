package fragment // 逻辑分片

import (
	"fmt"

	"github.com/Soltus/encv-go/internal/v2/types"
	"github.com/dustin/go-humanize"
)

// 根据文件总大小和用户配置，智能计算最终的分片大小
func CalculateFragmentSize(totalFileSize int64, userConfiguredPhysicalSize int64) int64 {

	const (
		minLogicalSize        int64 = 4 * 1024 * 1024   // 逻辑分片最小 4MB
		defaultMaxLogicalSize int64 = 120 * 1024 * 1024 // 默认逻辑分片最大 120MB
		largeMaxLogicalSize   int64 = 240 * 1024 * 1024 // 极限逻辑分片最大 240MB
		smallFileThreshold    int64 = 500 * 1024 * 1024 // 500MB，小文件阈值
		targetFragments       int64 = 100               // 大文件的目标分片数
	)

	// 1. 【核心修正】确定最终的物理分片大小
	// userConfiguredPhysicalSize == 0 表示不分片，整个加密流作为一个物理块
	var physicalChunkSize int64
	if userConfiguredPhysicalSize > 0 {
		physicalChunkSize = userConfiguredPhysicalSize
	} else {
		physicalChunkSize = totalFileSize // 不分片，物理块大小等于文件大小
	}

	// 2. 动态确定逻辑分片的最大值
	maxLogicalSize := defaultMaxLogicalSize
	// 当物理块很大（或不分片）且文件本身也很大时，启用更大的逻辑分片
	if (userConfiguredPhysicalSize == 0 || userConfiguredPhysicalSize > 1*1024*1024*1024) && totalFileSize > 30*1024*1024*1024 {
		maxLogicalSize = largeMaxLogicalSize
		fmt.Printf("-> [VIDEO] Large file and high-performance storage detected, using max logical chunk size of %s.\n", humanize.Bytes(uint64(maxLogicalSize)))
	}

	// 3. 【关键约束】逻辑分片的最大值不能超过物理分片的大小
	if maxLogicalSize > physicalChunkSize {
		maxLogicalSize = physicalChunkSize
	}

	var logicalChunkSize int64

	// 4. 对小文件进行特殊处理
	if totalFileSize <= smallFileThreshold {
		if totalFileSize <= maxLogicalSize {
			logicalChunkSize = totalFileSize
		} else {
			logicalChunkSize = maxLogicalSize
		}
	} else {
		// 5. 对大文件，使用目标分片数进行智能计算
		idealSize := totalFileSize / int64(targetFragments)
		if idealSize < minLogicalSize {
			idealSize = minLogicalSize
		} else if idealSize > maxLogicalSize {
			idealSize = maxLogicalSize
		}
		logicalChunkSize = idealSize
	}

	// 6. 最终大小也不能小于最小逻辑大小
	if logicalChunkSize < minLogicalSize {
		logicalChunkSize = minLogicalSize
	}

	// 7. 最终大小不能超过物理分片大小（此检查为健壮性保留）
	if logicalChunkSize > physicalChunkSize {
		logicalChunkSize = physicalChunkSize
	}

	return logicalChunkSize
}

// CreateLogicalFragmentsFromSize 仅根据总大小和分片大小，创建逻辑分片元数据。
// 这是最高效的方式，因为它不需要任何 I/O 操作。
func CreateLogicalFragmentsFromSize(totalSize int64, fragmentSize int64) ([]types.Fragment_v2, error) {
	if fragmentSize <= 0 {
		return nil, fmt.Errorf("fragment size must be positive")
	}
	if totalSize < 0 {
		return nil, fmt.Errorf("total size cannot be negative")
	}

	var logicalFragments []types.Fragment_v2
	var globalOffset uint64 = 0
	chunkIndex := 0
	remaining := totalSize

	for remaining > 0 {
		currentChunkSize := fragmentSize
		if remaining < fragmentSize {
			currentChunkSize = remaining
		}

		fragID := fmt.Sprintf("video_chunk_%d", chunkIndex) // ID 生成策略可能需要泛化
		frag := types.Fragment_v2{
			ID:                fragID,
			Type:              types.FragmentType_SeekableStream,
			Length:            uint64(currentChunkSize),
			GlobalStartOffset: globalOffset,
		}
		logicalFragments = append(logicalFragments, frag)
		globalOffset += uint64(currentChunkSize)
		remaining -= currentChunkSize
		chunkIndex++
	}

	return logicalFragments, nil
}
