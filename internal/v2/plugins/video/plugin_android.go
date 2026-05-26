//go:build android

package video

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Soltus/encv-go/internal/v2/crypto"
	"github.com/Soltus/encv-go/internal/v2/namer"
	"github.com/Soltus/encv-go/internal/v2/physical"
	pluginInterfaces "github.com/Soltus/encv-go/internal/v2/plugins/interfaces"
	"github.com/Soltus/encv-go/internal/v2/types"
)

type VideoPlugin struct {
	trackExtensionsList []string
}

func (p *VideoPlugin) Name() string {
	return "video"
}

type VideoPluginConfig struct {
	Ext                            string `json:"ext"`
	ContainerChunkSizeMB           int    `json:"container_chunk_size_mb"`
	LightContainerMainChunkEnabled bool   `json:"light_container_main_chunk_enabled"`
	TrackExtensions                string `json:"track_extensions"`
	KeepMkvForMkvSource            bool   `json:"keep_mkv_for_mkv_source"`
	VerifyAfterPack                bool   `json:"verify_after_pack"`
	PluginCacheDir                 string `json:"plugin_cache_dir"`
	SkipMergeForSplitMKV           bool   `json:"skip_merge_for_split_mkv"`
	AllowNoReencode                bool   `json:"allow_no_reencode"`
	DefaultStreamPreset            string `json:"default_stream_preset"`
}

func (p *VideoPlugin) GetContainerExtension() string {
	return ".sccgv"
}

func (p *VideoPlugin) GetSettingsSchemaType() interface{} {
	return VideoPluginConfig{}
}

func (p *VideoPlugin) GetDefaultSettings() json.RawMessage {
	defaultCfg := VideoPluginConfig{
		Ext:                            ".sccgv",
		ContainerChunkSizeMB:           0,
		LightContainerMainChunkEnabled: false,
		TrackExtensions:                ".ass,.srt,.dm.ass",
		KeepMkvForMkvSource:            true,
		VerifyAfterPack:                false,
		AllowNoReencode:                false,
		DefaultStreamPreset:            "balanced",
	}
	data, _ := json.Marshal(defaultCfg)
	return data
}

func (p *VideoPlugin) GetSettingFields() []pluginInterfaces.SettingField {
	return []pluginInterfaces.SettingField{
		{Key: "ext", Type: "string", DefaultValue: ".sccgv", Help: "The container file extension for encrypted video files."},
		{Key: "container_chunk_size_mb", Type: "number", DefaultValue: 0, Help: "The chunk size in MB."},
		{Key: "light_container_main_chunk_enabled", Type: "bool", DefaultValue: false, Help: "Light main chunk mode."},
		{Key: "track_extensions", Type: "text", DefaultValue: ".ass,.srt,.dm.ass", Help: "Subtitle track extensions."},
		{Key: "keep_mkv_for_mkv_source", Type: "bool", DefaultValue: true, Help: "Keep MKV container for MKV sources."},
		{Key: "verify_after_pack", Type: "bool", DefaultValue: false, Help: "Verify after packing."},
		{Key: "plugin_cache_dir", Type: "string", DefaultValue: "", Help: "Cache directory."},
		{Key: "skip_merge_for_split_mkv", Type: "bool", DefaultValue: false, Help: "Skip merging split MKV."},
		{Key: "allow_no_reencode", Type: "bool", DefaultValue: false, Help: "Allow no re-encode."},
		{Key: "default_stream_preset", Type: "string", DefaultValue: "balanced", Help: "Default stream preset."},
	}
}

func init() {
	types.RegisterKVIProvider(IndexKindVideo, func(rawKVI json.RawMessage) (types.KVIProvider, error) {
		var kvi VideoKVI_v2
		if err := json.Unmarshal(rawKVI, &kvi); err != nil {
			return nil, fmt.Errorf("failed to unmarshal KVI: %w", err)
		}
		return kvi, nil
	})
}

func (p *VideoPlugin) Initialize(ctx context.Context) error {
	return nil
}

func (p *VideoPlugin) GetChunkNamer() namer.ChunkNamer {
	return namer.NewPaddedNamer(".sccgv", namer.NewDefaultBaseNamer(), 4)
}

func (p *VideoPlugin) SupportedMimePrefixes() []string {
	return []string{"video/", "application/vnd.rn-realmedia-vbr", "application/vnd.apple.mpegurl"}
}

func (p *VideoPlugin) SupportedExtensions() []string {
	return []string{"mp4", "mkv", "avi", "mov", "rmvb", "webm", "flv", "m3u8"}
}

func (p *VideoPlugin) ShouldProcess(inputPath string) bool {
	return true
}

func (p *VideoPlugin) CanDecrypt(containerPath string) bool {
	return false
}

func (p *VideoPlugin) GetMetadataExtractor() pluginInterfaces.MetadataExtractor {
	return nil
}

func (p *VideoPlugin) GetContentPreprocessor() pluginInterfaces.ContentPreprocessor {
	return nil
}

func (p *VideoPlugin) GetContentVirifier() pluginInterfaces.ContentVerifier {
	return nil
}

func (p *VideoPlugin) GetPhysicalPacker() physical.PhysicalPacker {
	return physical.NewSinglePhysicalPacker()
}

func (p *VideoPlugin) BuildFragments(logicalFileSize int64) ([]types.Fragment, error) {
	return nil, fmt.Errorf("video plugin: BuildFragments not available on android")
}

func (p *VideoPlugin) GroupFiles(inputPaths []string, inputRootDir, outputDir string) ([]string, error) {
	return inputPaths, nil
}

func (p *VideoPlugin) ContainerType() uint16 {
	return types.ContainerTypeVideo
}

func (p *VideoPlugin) DefaultIsSeekable(inputPath string) bool {
	return false
}

func (p *VideoPlugin) DisasterZones(inputPath string) []types.DisasterZone {
	return nil
}

func (p *VideoPlugin) SupportedContainerVersions() []int {
	return types.SupportedVersions
}

func (p *VideoPlugin) DefaultContainerVersion() int {
	return types.DefaultContainerVersion
}

func (p *VideoPlugin) ValidateVersion(version int) error {
	if !types.IsValidVersion(version) {
		return fmt.Errorf("video plugin: unsupported container version: %d", version)
	}
	return nil
}

func (p *VideoPlugin) PreEncryptProcessor(index types.Index, inputPath, inputRootDir, outputDir string) error {
	return fmt.Errorf("video plugin: PreEncryptProcessor not available on android")
}

func (p *VideoPlugin) Encrypt(dataReader io.Reader) (*crypto.EncryptionResult, error) {
	return nil, fmt.Errorf("video plugin: Encrypt not available on android, use native ffmpeg instead")
}

func (p *VideoPlugin) PostEncryptProcessor(result *crypto.EncryptionResult) error {
	return fmt.Errorf("video plugin: PostEncryptProcessor not available on android")
}

func (p *VideoPlugin) PreDecryptProcessor(containerPath, outputDir string) error {
	return nil
}

func (p *VideoPlugin) Decrypt(containerPath, outputDir string) error {
	return fmt.Errorf("video plugin: Decrypt not available on android, use native ffmpeg instead")
}

func (p *VideoPlugin) PostDecryptProcessor(containerPath string) error {
	return nil
}
