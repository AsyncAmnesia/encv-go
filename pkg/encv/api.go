// pkg/encv/api.go
package encv

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/crypto"
	"github.com/Soltus/encv-go/internal/processor"
	"github.com/Soltus/encv-go/internal/server"
	"github.com/Soltus/encv-go/internal/types"
)

// EncryptOptions 定义加密所需的参数
type EncryptOptions struct {
	// Password 用于加密和解密的密码
	Password string
	// OutputDir 加密后文件的输出目录
	OutputDir string
	// TrackExtensions 需要关联的字幕/弹幕文件扩展名
	TrackExtensions []string
}

// Encrypt 加密单个视频文件或整个目录。
// 这是加密功能的统一入口。
func Encrypt(inputPath string, opts EncryptOptions) error {
	if opts.Password == "" || opts.OutputDir == "" {
		return ErrMissingOptions
	}
	if len(opts.TrackExtensions) == 0 {
		opts.TrackExtensions = []string{".ass", ".srt", ".dm.ass"}
	}

	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// 为批量加密生成一个统一的盐值，以确保所有文件都可以用同一个密码播放
	salt := make([]byte, crypto.SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		return fmt.Errorf("failed to stat input path: %w", err)
	}

	if info.IsDir() {
		return encryptDir(inputPath, opts, salt)
	}
	return encryptFile(inputPath, opts, salt)
}

// encryptFile 加密单个文件
func encryptFile(inputPath string, opts EncryptOptions, salt []byte) error {
	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputEncPath := filepath.Join(opts.OutputDir, baseName+".enc")
	outputIndexPath := filepath.Join(opts.OutputDir, baseName+".kvi")

	return processor.ProcessVideo(inputPath, outputEncPath, outputIndexPath, opts.Password, salt, opts.TrackExtensions)
}

// encryptDir 加密目录中的所有视频文件
func encryptDir(inputDir string, opts EncryptOptions, salt []byte) error {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return fmt.Errorf("failed to read input directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".mp4" || ext == ".mkv" {
			fullPath := filepath.Join(inputDir, entry.Name())
			if err := encryptFile(fullPath, opts, salt); err != nil {
				// 打印错误但继续处理其他文件
				fmt.Printf("Error processing %s: %v\n", entry.Name(), err)
			}
		}
	}
	return nil
}

// DecryptOptions 定义解密所需的参数
type DecryptOptions struct {
	// Password 用于解密的密码
	Password string
	// OutputDir 解密后文件的输出目录
	OutputDir string
}

// Decrypt 解密单个视频文件及其关联轨道。
func Decrypt(inputEncPath string, opts DecryptOptions) error {
	if opts.Password == "" || opts.OutputDir == "" {
		return ErrMissingOptions
	}

	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// --- Step 1: 查找并读取 .kvi 索引文件 ---
	baseFilename := strings.TrimSuffix(filepath.Base(inputEncPath), filepath.Ext(inputEncPath))
	indexPath := filepath.Join(filepath.Dir(inputEncPath), baseFilename+".kvi")

	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("index file not found: %w", err)
	}

	var index types.VideoIndex
	if err := json.Unmarshal(indexData, &index); err != nil {
		return fmt.Errorf("failed to parse index file: %w", err)
	}

	// --- Step 2: 解密视频文件 ---
	salt, err := crypto.Base64Decode(index.Encryption.SaltBase64)
	if err != nil {
		return fmt.Errorf("failed to decode salt: %w", err)
	}
	key := crypto.GenerateKey(opts.Password, salt)

	iv, err := crypto.Base64Decode(index.Encryption.IVBase64)
	if err != nil {
		return fmt.Errorf("failed to decode IV: %w", err)
	}

	originalFilename := filepath.Base(index.OriginalFilename)
	if originalFilename == "" || originalFilename == "unknown" {
		originalFilename = baseFilename + ".mp4" // 默认扩展名
	}
	outputVideoPath := filepath.Join(opts.OutputDir, originalFilename)

	encFile, err := os.Open(inputEncPath)
	if err != nil {
		return fmt.Errorf("failed to open encrypted file: %w", err)
	}
	defer encFile.Close()

	decFile, err := os.Create(outputVideoPath)
	if err != nil {
		return fmt.Errorf("failed to create output video file: %w", err)
	}
	defer decFile.Close()

	if err := crypto.DecryptStream(encFile, decFile, key, iv); err != nil {
		return fmt.Errorf("failed to decrypt video stream: %w", err)
	}

	// --- Step 3: 复制轨道文件 ---
	sourceDir := filepath.Dir(inputEncPath)
	for _, track := range index.Subtitles {
		sourceTrackPath := filepath.Join(sourceDir, track.Filename)
		destTrackPath := filepath.Join(opts.OutputDir, track.Filename)

		if _, err := os.Stat(sourceTrackPath); os.IsNotExist(err) {
			fmt.Printf("Warning: Track file not found at source: %s\n", sourceTrackPath)
			continue
		}

		if err := copyFile(sourceTrackPath, destTrackPath); err != nil {
			fmt.Printf("Warning: Failed to copy track %s: %v\n", track.Filename, err)
		}
	}

	return nil
}

// Player 封装了流媒体服务器，提供对外接口
type Player struct {
	p *server.Player
}

// NewPlayer 创建一个新的播放器实例
func NewPlayer(dir, password string) *Player {
	return &Player{p: server.NewPlayer(dir, password)}
}

// Start 启动服务器，返回监听的地址
func (p *Player) Start(port int) (string, error) {
	return p.p.Start(port)
}

// Stop 停止服务器
func (p *Player) Stop() {
	p.p.Stop()
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// 自定义错误类型
var (
	ErrMissingOptions = fmt.Errorf("password and output directory are required")
)
