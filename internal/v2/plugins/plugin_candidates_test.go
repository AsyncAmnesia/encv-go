package plugins_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Soltus/encv-go/internal/v2/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initPluginsForCandidates(t *testing.T) {
	t.Helper()
	initPluginsForTaskOptions(t)
}

func TestFindAllEncryptingPlugins_VideoFile_MimeMatch_P0(t *testing.T) {
	initPluginsForCandidates(t)
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "test.mp4")
	require.NoError(t, os.WriteFile(videoPath, []byte("\x00\x00\x00\x18ftypmp42"), 0644))

	candidates := plugins.FindAllEncryptingPlugins(videoPath)
	assert.NotEmpty(t, candidates, "video file should have at least one candidate")

	videoCand := findCandidateByName(candidates, "video")
	require.NotNil(t, videoCand, "video plugin should be a candidate for .mp4")
	assert.Equal(t, "mime", videoCand.MatchType, "video should match via MIME type")
	assert.Equal(t, 0, videoCand.Priority, "video should be priority 0 (exact)")
}

func TestFindAllEncryptingPlugins_TextFile_P0(t *testing.T) {
	initPluginsForCandidates(t)
	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(txtPath, []byte("hello world"), 0644))

	candidates := plugins.FindAllEncryptingPlugins(txtPath)
	assert.NotEmpty(t, candidates, "text file should have at least one candidate")

	textCand := findCandidateByName(candidates, "text")
	require.NotNil(t, textCand, "text plugin should be a candidate for .txt")
	assert.Equal(t, 0, textCand.Priority, "text should be priority 0 (exact match)")
	assert.Contains(t, []string{"mime", "extension"}, textCand.MatchType,
		"text should match via MIME or extension (both are P0)")
}

func TestFindAllEncryptingPlugins_ArbitraryFile_GeneralP1(t *testing.T) {
	initPluginsForCandidates(t)
	tmpDir := t.TempDir()
	arbitraryPath := filepath.Join(tmpDir, "data.xyz123")
	require.NoError(t, os.WriteFile(arbitraryPath, []byte("binary data"), 0644))

	candidates := plugins.FindAllEncryptingPlugins(arbitraryPath)

	alistCand := findCandidateByName(candidates, "alist_encrypt")
	require.NotNil(t, alistCand, "alist_encrypt should handle arbitrary files (ShouldProcess=true)")
	assert.Equal(t, "general", alistCand.MatchType, "alist_encrypt should be a general candidate")
	assert.Equal(t, 1, alistCand.Priority, "alist_encrypt should be priority 1 (general)")
}

func TestFindAllEncryptingPlugins_VideoFile_IncludesGeneral_NoDuplication(t *testing.T) {
	initPluginsForCandidates(t)
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "movie.mp4")
	require.NoError(t, os.WriteFile(videoPath, []byte("\x00\x00\x00\x18ftypmp42"), 0644))

	candidates := plugins.FindAllEncryptingPlugins(videoPath)

	videoCand := findCandidateByName(candidates, "video")
	require.NotNil(t, videoCand, "video plugin should be present")
	assert.Equal(t, 0, videoCand.Priority, "video should be P0")

	alistCand := findCandidateByName(candidates, "alist_encrypt")
	require.NotNil(t, alistCand, "alist_encrypt should also be present as general fallback")
	assert.Equal(t, 1, alistCand.Priority, "alist_encrypt should be P1")

	names := make(map[string]bool)
	for _, c := range candidates {
		assert.False(t, names[c.Name], "candidate %q should not appear twice", c.Name)
		names[c.Name] = true
	}
}

func TestFindAllEncryptingPlugins_EmptyPath_GeneralCandidatesOnly(t *testing.T) {
	initPluginsForCandidates(t)
	candidates := plugins.FindAllEncryptingPlugins("")

	assert.NotEmpty(t, candidates, "empty path should return general candidates (plugins with ShouldProcess=true)")
	for _, c := range candidates {
		assert.Equal(t, "general", c.MatchType,
			"empty path has no MIME/extension, all candidates should be general type")
		assert.Equal(t, 1, c.Priority,
			"empty path candidates should be priority 1 (general)")
	}
}

func TestFindAllEncryptingPlugins_NonExistentFile_NoCrash(t *testing.T) {
	initPluginsForCandidates(t)
	assert.NotPanics(t, func() {
		plugins.FindAllEncryptingPlugins("/nonexistent/path/to/file.dat")
	}, "should not crash on non-existent file")
}

func findCandidateByName(candidates []plugins.PluginCandidate, name string) *plugins.PluginCandidate {
	for i := range candidates {
		if candidates[i].Name == name {
			return &candidates[i]
		}
	}
	return nil
}
