package plugins

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/v2/plugins/text"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildFullPluginSettings_NilInput 验证传入 nil 时返回所有插件的默认配置
func TestBuildFullPluginSettings_NilInput(t *testing.T) {
	result, err := BuildFullPluginSettings(nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	textSettingsRaw, ok := result["text"]
	require.True(t, ok, "结果中应包含 'text' 插件的配置")

	var textCfg text.TextPluginConfig
	err = json.Unmarshal(textSettingsRaw, &textCfg)
	require.NoError(t, err)
	assert.Equal(t, ".sccgt", textCfg.Ext, "text 插件应使用默认扩展名 .sccgt")
}

// TestBuildFullPluginSettings_WithUserSettings 验证用户配置能正确覆盖默认值
func TestBuildFullPluginSettings_WithUserSettings(t *testing.T) {
	userSettings := map[string]json.RawMessage{
		"text": json.RawMessage(`{"ext": ".custom"}`),
	}

	result, err := BuildFullPluginSettings(userSettings)
	require.NoError(t, err)
	require.NotNil(t, result)

	textSettingsRaw, ok := result["text"]
	require.True(t, ok)

	var textCfg text.TextPluginConfig
	err = json.Unmarshal(textSettingsRaw, &textCfg)
	require.NoError(t, err)
	assert.Equal(t, ".custom", textCfg.Ext, "用户自定义的 ext 应覆盖默认值")
}

// TestBuildFullPluginSettings_EmptyUserSettings 验证传入空 map（非 nil）时使用全部默认配置
func TestBuildFullPluginSettings_EmptyUserSettings(t *testing.T) {
	userSettings := map[string]json.RawMessage{}

	result, err := BuildFullPluginSettings(userSettings)
	require.NoError(t, err)
	require.NotNil(t, result)

	textSettingsRaw, ok := result["text"]
	require.True(t, ok)

	var textCfg text.TextPluginConfig
	err = json.Unmarshal(textSettingsRaw, &textCfg)
	require.NoError(t, err)
	assert.Equal(t, ".sccgt", textCfg.Ext, "空用户配置应回退到默认值")

	assert.Len(t, result, len(Plugins), "返回的插件数量应与 Plugins 列表一致")
}

// TestFindEncryptingPlugin_ByExtension_TXT 验证 .txt 文件能匹配到 text 插件
func TestFindEncryptingPlugin_ByExtension_TXT(t *testing.T) {
	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(txtPath, []byte("hello world"), 0644))

	plugin, err := FindEncryptingPlugin(txtPath)
	require.NoError(t, err)
	require.NotNil(t, plugin)
	assert.Equal(t, "text", plugin.Name())
}

// TestFindEncryptingPlugin_ByExtension_GO 验证 .go 文件能匹配到 text 插件
func TestFindEncryptingPlugin_ByExtension_GO(t *testing.T) {
	tmpDir := t.TempDir()
	goPath := filepath.Join(tmpDir, "main.go")
	require.NoError(t, os.WriteFile(goPath, []byte("package main"), 0644))

	plugin, err := FindEncryptingPlugin(goPath)
	require.NoError(t, err)
	require.NotNil(t, plugin)
	assert.Equal(t, "text", plugin.Name())
}

// TestFindEncryptingPlugin_NoMatch 验证不支持的扩展名返回错误
func TestFindEncryptingPlugin_NoMatch(t *testing.T) {
	plugin, err := FindEncryptingPlugin("/nonexistent.xyz")
	require.Error(t, err)
	assert.Nil(t, plugin)
	assert.Contains(t, err.Error(), ".xyz")
}

// TestTextPlugin_Initialize_WithSettings 验证带自定义设置的初始化能正确读取配置
func TestTextPlugin_Initialize_WithSettings(t *testing.T) {
	cfg := &config.Config{
		PluginSettings: map[string]json.RawMessage{
			"text": json.RawMessage(`{"ext": ".test"}`),
		},
	}
	ctx := config.NewContext(context.Background(), cfg)

	p := new(text.TextPlugin)
	err := p.Initialize(ctx)
	require.NoError(t, err)

	assert.Equal(t, ".test", p.GetContainerExtension())
}

// TestTextPlugin_Initialize_NoSettings 验证缺少插件设置时 Initialize 返回错误
func TestTextPlugin_Initialize_NoSettings(t *testing.T) {
	cfg := &config.Config{
		PluginSettings: map[string]json.RawMessage{},
	}
	ctx := config.NewContext(context.Background(), cfg)

	p := new(text.TextPlugin)
	err := p.Initialize(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no settings found for plugin 'text'")
}

func TestRegistry_NoAutoInstallBehavior(t *testing.T) {
	forbiddenSymbols := []string{
		"install", "Install", "Installer",
		"loadEnabled", "PluginManager",
		"download", "Download",
		"fetchPlugin", "FetchPlugin",
	}

	srcBytes, err := os.ReadFile("registry.go")
	require.NoError(t, err, "should be able to read registry.go source")
	src := string(srcBytes)

	publicFuncs := []string{
		"EncryptFileWithPlugin",
		"ProcessFileWithPlugin",
		"DecryptContainerWithPlugin",
		"WalkAndEncrypt",
		"FindEncryptingPlugin",
		"FindDecryptingPlugin",
	}

	for _, fnName := range publicFuncs {
		t.Run(fnName+"_no_install_refs", func(t *testing.T) {
			for _, sym := range forbiddenSymbols {
				if contains(src, sym) {
					t.Errorf("registry.go contains forbidden symbol %q which implies auto-install behavior; %s should be a pure file-processing function", sym, fnName)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
