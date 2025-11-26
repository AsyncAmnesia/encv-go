package processor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/types"
	"github.com/Soltus/encv-go/internal/utils"
)

// ProcessVideo 是主协调器函数，负责调用各个处理步骤
func ProcessVideo(inputPath, outputEncPath, outputIndexPath, password string, salt []byte, trackExtensions []string, originalFilename string, encBaseName string) error {
	fmt.Printf("-> Processing %s...\n", filepath.Base(inputPath))

	// --- Step 1: Pre-processing with FFmpeg ---
	tempPath, err := preprocessVideoWithFFmpeg(inputPath)
	if err != nil {
		return err
	}
	defer os.Remove(tempPath)

	// --- Step 2: Find, sort, copy and record subtitle tracks ---
	subtitleTracks, err := discoverAndProcessSubtitles(inputPath, trackExtensions, encBaseName)
	if err != nil {
		return err
	}

	// --- Step 3: Encrypt video ---
	iv, err := encryptVideoFile(tempPath, outputEncPath, password, salt)
	if err != nil {
		return err
	}

	// --- Step 4: Calculate MD5 of the encrypted file ---
	md5Sum, err := utils.FileMD5(outputEncPath)
	if err != nil {
		return err
	}

	// --- Step 5: Get metadata ---
	metadata, err := getProcessedMetadata(tempPath)
	if err != nil {
		return err
	}

	// --- Step 6: Create and write the KVI file ---
	index := &types.VideoIndex{
		Version:          crypto.KviVersion, // 使用常量
		VideoID:          fmt.Sprintf("vid-%d", os.Getuid()),
		OriginalFileSize: metadata.FileSize,
		Format:           metadata.Format,
		Encryption: types.EncryptionInfo{
			Algorithm:  crypto.Algorithm,
			IVBase64:   crypto.Base64Encode(iv),
			SaltBase64: crypto.Base64Encode(salt),
		},
		SeekTable:        []interface{}{},
		DurationSeconds:  metadata.Duration,
		Resolution:       metadata.Resolution,
		OriginalFilename: originalFilename,
		EncryptedFileMD5: md5Sum,
		Subtitles:        subtitleTracks,
	}

	return createKVIFile(outputIndexPath, index)
}
