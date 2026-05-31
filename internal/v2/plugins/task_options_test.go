package plugins_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	pluginInterfaces "github.com/Soltus/encv-go/internal/v2/plugins/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initPluginsForTaskOptions(t *testing.T) {
	t.Helper()
	cfg := &config.Config{
		Password: "global-test-pw",
		PluginSettings: map[string]json.RawMessage{
			"video":        json.RawMessage(`{"suffix": ".encv"}`),
			"alist_encrypt": json.RawMessage(`{"suffix": ".bin", "enc_type": "aesctr"}`),
			"text":         json.RawMessage(`{"suffix": ".sccgt"}`),
			"audio":        json.RawMessage(`{"suffix": ".sccga"}`),
			"image":        json.RawMessage(`{"suffix": ".sccgi"}`),
			"pdf":          json.RawMessage(`{"suffix": ".sccgp"}`),
			"wps":          json.RawMessage(`{"suffix": ".sccgw"}`),
		},
	}
	ctx := config.NewContext(context.Background(), cfg)
	err := plugins.InitializePlugins(ctx)
	require.NoError(t, err, "InitializePlugins should succeed")
}

func getPluginByName(name string) plugins.Plugin {
	for _, p := range plugins.Plugins {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

func TestVideoPlugin_GetTaskOptions(t *testing.T) {
	initPluginsForTaskOptions(t)
	p := getPluginByName("video")
	require.NotNil(t, p, "video plugin should exist")
	opts := p.GetTaskOptions()
	assert.Equal(t, pluginInterfaces.PasswordGlobal, opts.PasswordStrategy, "video should use global password")
	assert.True(t, opts.SupportVersionSelect, "video should support version select")
	assert.NotEmpty(t, opts.SupportedVersions, "video should have supported versions")
	require.Len(t, opts.ExtraFields, 2, "video should have 2 extra fields")
	assert.Equal(t, "stream_preset", opts.ExtraFields[0].Key)
	assert.Equal(t, "select", opts.ExtraFields[0].Type)
	assert.False(t, opts.ExtraFields[0].Required)
	assert.Equal(t, "balanced", opts.ExtraFields[0].DefaultValue)
	assert.Contains(t, opts.ExtraFields[0].Options, "balanced")
	assert.Contains(t, opts.ExtraFields[0].Options, "high_quality")
	assert.Equal(t, "encrypt", opts.ExtraFields[0].Condition)
	assert.Equal(t, "encrypt_filename", opts.ExtraFields[1].Key)
	assert.Equal(t, "bool", opts.ExtraFields[1].Type)
	assert.Equal(t, "false", opts.ExtraFields[1].DefaultValue)
	assert.Equal(t, "encrypt", opts.ExtraFields[1].Condition)
}

func TestAlistEncryptPlugin_GetTaskOptions(t *testing.T) {
	initPluginsForTaskOptions(t)
	p := getPluginByName("alist_encrypt")
	require.NotNil(t, p, "alist_encrypt plugin should exist")
	opts := p.GetTaskOptions()
	assert.Equal(t, pluginInterfaces.PasswordIndependent, opts.PasswordStrategy, "alist_encrypt should use independent password")
	assert.False(t, opts.SupportVersionSelect, "alist_encrypt should NOT support version select")
	require.Len(t, opts.ExtraFields, 3, "alist_encrypt should have 3 extra fields")
	assert.Equal(t, "plugin_password", opts.ExtraFields[0].Key)
	assert.Equal(t, "password", opts.ExtraFields[0].Type)
	assert.False(t, opts.ExtraFields[0].Required, "plugin_password should not be required")
	assert.Equal(t, "encode_filename", opts.ExtraFields[1].Key)
	assert.Equal(t, "bool", opts.ExtraFields[1].Type)
	assert.Equal(t, "false", opts.ExtraFields[1].DefaultValue)
	assert.Equal(t, "encrypt", opts.ExtraFields[1].Condition)
	assert.Equal(t, "enc_type", opts.ExtraFields[2].Key)
	assert.Equal(t, "select", opts.ExtraFields[2].Type)
	assert.Equal(t, "aesctr", opts.ExtraFields[2].DefaultValue)
	assert.Contains(t, opts.ExtraFields[2].Options, "aesctr")
	assert.Contains(t, opts.ExtraFields[2].Options, "chacha20")
	assert.Equal(t, "encrypt", opts.ExtraFields[2].Condition)
}

func TestOtherPlugins_DefaultToGlobal(t *testing.T) {
	initPluginsForTaskOptions(t)
	for _, name := range []string{"text", "audio", "image", "pdf"} {
		p := getPluginByName(name)
		require.NotNil(t, p, "%s plugin should exist", name)
		assert.Equal(t, pluginInterfaces.PasswordGlobal, p.GetTaskOptions().PasswordStrategy,
			"plugin %s should default to global password strategy", name)
	}
}
