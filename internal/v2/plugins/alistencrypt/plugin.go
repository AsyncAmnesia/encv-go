package alistencrypt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/alistencrypt"
	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/v2/crypto"
	pluginInterfaces "github.com/Soltus/encv-go/internal/v2/plugins/interfaces"
	"github.com/Soltus/encv-go/internal/v2/namer"
	"github.com/Soltus/encv-go/internal/v2/types"
)

type AlistEncryptPlugin struct {
	ctx        context.Context
	cfg        *config.Config
	settings   AlistEncryptPluginConfig
	outputDir  string
	inputPath  string
}

func (p *AlistEncryptPlugin) Name() string {
	return "alist_encrypt"
}

func (p *AlistEncryptPlugin) GetDefaultSettings() json.RawMessage {
	defaultCfg := AlistEncryptPluginConfig{
		Suffix:          ".bin",
		DefaultPassword: "",
		EncType:         "aesctr",
	}
	data, _ := json.Marshal(defaultCfg)
	return data
}

func (p *AlistEncryptPlugin) GetSettingsSchemaType() interface{} {
	return AlistEncryptPluginConfig{}
}

func (p *AlistEncryptPlugin) GetContainerExtension() string {
	return p.settings.Suffix
}

func (p *AlistEncryptPlugin) GetSettingFields() []pluginInterfaces.SettingField {
	return []pluginInterfaces.SettingField{
		{
			Key:          "suffix",
			Type:         "string",
			DefaultValue: ".bin",
			Help:         "Encrypted file suffix (e.g., '.bin'). Cannot be '.sccgv' or '.encv'.",
		},
		{
			Key:          "default_password",
			Type:         "string",
			DefaultValue: "",
			Help:         "Default password for encryption/decryption (optional, can be overridden per-operation).",
		},
		{
			Key:          "enc_type",
			Type:         "string",
			DefaultValue: "aesctr",
			Help:         "Encryption algorithm type. Currently only 'aesctr' is built-in.",
			Options:      []string{"aesctr"},
		},
	}
}

var reservedSuffixes = map[string]bool{".sccgv": true, ".encv": true}

func (p *AlistEncryptPlugin) Initialize(ctx context.Context) error {
	if ctx == p.ctx {
		return nil
	}
	p.ctx = ctx
	p.cfg = config.FromContext(ctx)

	settings, err := config.GetPluginSettingsFor[AlistEncryptPluginConfig](p.cfg, p.Name())
	if err != nil {
		return fmt.Errorf("could not get settings for plugin %s: %w", p.Name(), err)
	}
	p.settings = *settings

	suffix := p.settings.Suffix
	if reservedSuffixes[strings.ToLower(suffix)] {
		slog.Error("alist_encrypt: suffix conflicts with ENCV container format, falling back to .bin",
			"suffix", suffix)
		p.settings.Suffix = ".bin"
	} else if !strings.HasPrefix(suffix, ".") {
		slog.Warn("alist_encrypt: suffix does not start with '.', falling back to .bin",
			"suffix", suffix)
		p.settings.Suffix = ".bin"
	} else if len(suffix) > 16 {
		slog.Warn("alist_encrypt: suffix exceeds 16 chars, falling back to .bin",
			"suffix", suffix)
		p.settings.Suffix = ".bin"
	}

	if p.settings.EncType != "aesctr" {
		slog.Warn("alist_encrypt: unsupported enc_type, only aesctr is built-in",
			"enc_type", p.settings.EncType)
	}

	if p.settings.DefaultPassword == "" && p.cfg.Password != "" {
		p.settings.DefaultPassword = p.cfg.Password
	}

	return nil
}

func (p *AlistEncryptPlugin) GetMetadataExtractor() pluginInterfaces.MetadataExtractor {
	return nil
}

func (p *AlistEncryptPlugin) GetContentPreprocessor() pluginInterfaces.ContentPreprocessor {
	return nil
}

func (p *AlistEncryptPlugin) GetContentVirifier() pluginInterfaces.ContentVerifier {
	return nil
}

func (p *AlistEncryptPlugin) GetChunkNamer() namer.ChunkNamer {
	return nil
}

func (p *AlistEncryptPlugin) SupportedMimePrefixes() []string {
	return nil
}

func (p *AlistEncryptPlugin) SupportedExtensions() []string {
	return nil
}

func (p *AlistEncryptPlugin) ShouldProcess(inputPath string) bool {
	return true
}

func (p *AlistEncryptPlugin) GroupFiles(inputPaths []string, inputRootDir, outputDir string) ([]string, error) {
	return inputPaths, nil
}

func (p *AlistEncryptPlugin) PreEncryptProcessor(index types.Index, inputPath, inputRootDir, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	p.inputPath = inputPath
	p.outputDir = outputDir
	return nil
}

func (p *AlistEncryptPlugin) Encrypt(dataReader io.Reader) (*crypto.EncryptionResult, error) {
	password := p.resolvePassword()

	result, err := EncryptToFile(dataReader, password, p.outputDir, &p.settings)
	if err != nil {
		return nil, fmt.Errorf("alist_encrypt encryption failed: %w", err)
	}

	return result, nil
}

func (p *AlistEncryptPlugin) PostEncryptProcessor(result *crypto.EncryptionResult) error {
	originalFilename := filepath.Base(p.inputPath)

	finalPath, err := RenameToFinalEncrypted(result.TempPath, originalFilename, p.outputDir, p.settings.Suffix)
	if err != nil {
		os.Remove(result.TempPath)
		return fmt.Errorf("failed to rename encrypted file: %w", err)
	}

	slog.Info("alist_encrypt: encryption complete", "output", finalPath)
	return nil
}

func (p *AlistEncryptPlugin) CanDecrypt(containerPath string) bool {
	ext := strings.ToLower(filepath.Ext(containerPath))
	if ext != p.settings.Suffix {
		return false
	}
	return PeekIsAECTR2(containerPath)
}

func (p *AlistEncryptPlugin) PreDecryptProcessor(containerPath, outputDir string) error {
	return nil
}

func (p *AlistEncryptPlugin) Decrypt(containerPath, outputDir string) error {
	ext := strings.ToLower(filepath.Ext(containerPath))
	if ext != p.settings.Suffix {
		return &alistencrypt.DecryptionError{Reason: "invalid format: extension mismatch", Err: alistencrypt.ErrInvalidFormat}
	}

	password := p.resolvePassword()

	if err := DecryptFile(containerPath, outputDir, password, p.settings.EncType); err != nil {
		return fmt.Errorf("alist_encrypt decryption failed for '%s': %w", containerPath, err)
	}

	slog.Info("alist_encrypt: decryption complete", "source", containerPath, "output_dir", outputDir)
	return nil
}

func (p *AlistEncryptPlugin) PostDecryptProcessor(containerPath string) error {
	return nil
}

const containerTypeAlistEncrypt uint16 = 0x000A

func (p *AlistEncryptPlugin) ContainerType() uint16 {
	return containerTypeAlistEncrypt
}

func (p *AlistEncryptPlugin) DefaultIsSeekable(inputPath string) bool {
	return true
}

func (p *AlistEncryptPlugin) DisasterZones(inputPath string) []types.DisasterZone {
	return nil
}

func (p *AlistEncryptPlugin) SupportedContainerVersions() []int {
	return nil
}

func (p *AlistEncryptPlugin) DefaultContainerVersion() int {
	return 0
}

func (p *AlistEncryptPlugin) ValidateVersion(version int) error {
	return fmt.Errorf("alist_encrypt plugin does not use ENCV container versions")
}

func (p *AlistEncryptPlugin) resolvePassword() string {
	if p.settings.DefaultPassword != "" {
		return p.settings.DefaultPassword
	}
	if p.cfg != nil && p.cfg.Password != "" {
		return p.cfg.Password
	}
	return ""
}
