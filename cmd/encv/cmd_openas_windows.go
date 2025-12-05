//go:build windows
// +build windows

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Soltus/encv-go/internal/config"
	"golang.org/x/sys/windows/registry"
)

// handleOpenAsCommand 在 Windows 注册表中注册文件关联（双击行为）
func handleOpenAsCommand(cfg *config.Config) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exePathQuoted := fmt.Sprintf(`"%s"`, exePath)

	// 定义不同类型的命令模板
	// 视频和音频使用流式播放
	streamCommand := fmt.Sprintf(`%s open-stream "%%1"`, exePathQuoted)
	// 图片和文本使用临时文件
	tempFileCommand := fmt.Sprintf(`%s open-temp "%%1"`, exePathQuoted)

	// 定义要注册的扩展名、类型和对应的命令
	extensionsToRegister := map[string]struct {
		ext     string
		command string
	}{
		"video": {cfg.BinExtGroup.Video, streamCommand},
		"audio": {cfg.BinExtGroup.Audio, streamCommand},
		"image": {cfg.BinExtGroup.Image, tempFileCommand},
		"text":  {cfg.BinExtGroup.Text, tempFileCommand},
	}

	for kind, item := range extensionsToRegister {
		if item.ext == "" {
			log.Printf("Warning: Extension for kind '%s' is not configured, skipping.", kind)
			continue
		}

		log.Printf("-> Registering .%s extension with command: %s", item.ext, item.command)
		if err := registerSingleExtension(item.ext, item.command); err != nil {
			return fmt.Errorf("failed to register .%s: %w", item.ext, err)
		}
	}

	return nil
}

// registerSingleExtension 为单个扩展名创建注册表项
// 现在接受一个完整的 command 字符串作为参数
func registerSingleExtension(ext, command string) error {
	progID := fmt.Sprintf("encv.%s", ext)

	// --- 注册表操作 ---

	// 1. 创建 HKEY_CLASSES_ROOT\.ext 项
	extKey, _, err := registry.CreateKey(registry.CLASSES_ROOT, `.`+ext, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("could not create extension key: %w", err)
	}
	defer extKey.Close()

	if err := extKey.SetStringValue("", progID); err != nil {
		return fmt.Errorf("could not set extension key's default value: %w", err)
	}

	// 2. 创建 HKEY_CLASSES_ROOT\encv.ext 项
	progIDKey, _, err := registry.CreateKey(registry.CLASSES_ROOT, progID, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("could not create ProgID key: %w", err)
	}
	defer progIDKey.Close()

	if err := progIDKey.SetStringValue("", "ENCV Encrypted File"); err != nil {
		return fmt.Errorf("could not set ProgID description: %w", err)
	}

	// 3. 创建 HKEY_CLASSES_ROOT\encv.ext\shell\open\command 项
	commandKey, _, err := registry.CreateKey(registry.CLASSES_ROOT, progID+`\shell\open\command`, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("could not create command key: %w", err)
	}
	defer commandKey.Close()

	// 设置要执行的命令
	if err := commandKey.SetStringValue("", command); err != nil {
		return fmt.Errorf("could not set command: %w", err)
	}

	return nil
}
