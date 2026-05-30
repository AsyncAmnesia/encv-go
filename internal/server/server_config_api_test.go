package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initPluginsWithSettings(t *testing.T, userSettings map[string]json.RawMessage) {
	t.Helper()
	fullSettings, err := plugins.BuildFullPluginSettings(userSettings)
	require.NoError(t, err, "BuildFullPluginSettings should succeed")

	cfg := &config.Config{
		PluginSettings: fullSettings,
	}
	ctx := config.NewContext(context.Background(), cfg)

	for _, p := range plugins.Plugins {
		require.NoError(t, p.Initialize(ctx), "plugin %s should initialize", p.Name())
	}
}

func TestValidateContainerExtensionsInConfig_NoConflict(t *testing.T) {
	initPluginsWithSettings(t, nil)

	raw := map[string]interface{}{
		"plugin_settings": map[string]interface{}{
			"alist_encrypt": map[string]interface{}{
				"enabled": true,
				"suffix":  ".bin",
			},
		},
	}
	result := validateContainerExtensionsInConfig(raw)
	assert.Empty(t, result, "unique suffix .bin should not trigger conflict")
}

func TestValidateContainerExtensionsInConfig_SuffixConflictWithVideo(t *testing.T) {
	initPluginsWithSettings(t, nil)

	raw := map[string]interface{}{
		"plugin_settings": map[string]interface{}{
			"alist_encrypt": map[string]interface{}{
				"enabled": true,
				"suffix":  ".sccgv",
			},
		},
	}
	result := validateContainerExtensionsInConfig(raw)
	assert.Contains(t, result, "conflict")
	assert.Contains(t, result, ".sccgv")
	assert.Contains(t, result, "video")
	assert.Contains(t, result, "alist_encrypt")
}

func TestValidateContainerExtensionsInConfig_SuffixConflictWithAudio(t *testing.T) {
	initPluginsWithSettings(t, nil)

	raw := map[string]interface{}{
		"plugin_settings": map[string]interface{}{
			"alist_encrypt": map[string]interface{}{
				"suffix": ".sccga",
			},
		},
	}
	result := validateContainerExtensionsInConfig(raw)
	assert.Contains(t, result, "conflict")
	assert.Contains(t, result, ".sccga")
	assert.Contains(t, result, "audio")
}

func TestValidateContainerExtensionsInConfig_ExtFieldConflict(t *testing.T) {
	initPluginsWithSettings(t, nil)

	raw := map[string]interface{}{
		"plugin_settings": map[string]interface{}{
			"alist_encrypt": map[string]interface{}{
				"ext": ".sccgi",
			},
		},
	}
	result := validateContainerExtensionsInConfig(raw)
	assert.Contains(t, result, "conflict")
	assert.Contains(t, result, ".sccgi")
	assert.Contains(t, result, "image")
}

func TestValidateContainerExtensionsInConfig_NoPluginSettings(t *testing.T) {
	initPluginsWithSettings(t, nil)

	raw := map[string]interface{}{
		"server": map[string]interface{}{
			"port": 2025,
		},
	}
	result := validateContainerExtensionsInConfig(raw)
	assert.Empty(t, result)
}

func TestValidateContainerExtensionsInConfig_EmptySuffixIgnored(t *testing.T) {
	initPluginsWithSettings(t, nil)

	raw := map[string]interface{}{
		"plugin_settings": map[string]interface{}{
			"alist_encrypt": map[string]interface{}{
				"suffix": "",
			},
		},
	}
	result := validateContainerExtensionsInConfig(raw)
	assert.Empty(t, result, "empty suffix should be skipped")
}

func TestValidateContainerExtensionsInConfig_DotOnlySuffixIgnored(t *testing.T) {
	initPluginsWithSettings(t, nil)

	raw := map[string]interface{}{
		"plugin_settings": map[string]interface{}{
			"alist_encrypt": map[string]interface{}{
				"suffix": ".",
			},
		},
	}
	result := validateContainerExtensionsInConfig(raw)
	assert.Empty(t, result, "dot-only suffix should be skipped")
}

func TestValidateContainerExtensionsInConfig_SamePluginNoConflict(t *testing.T) {
	initPluginsWithSettings(t, nil)

	raw := map[string]interface{}{
		"plugin_settings": map[string]interface{}{
			"video": map[string]interface{}{
				"ext": ".sccgv",
			},
		},
	}
	result := validateContainerExtensionsInConfig(raw)
	assert.Empty(t, result, "same plugin declaring its own extension is not a conflict")
}

func TestValidateContainerExtensionsInConfig_TSSourceDelegatesToText(t *testing.T) {
	initPluginsWithSettings(t, nil)

	raw := map[string]interface{}{
		"plugin_settings": map[string]interface{}{},
	}
	result := validateContainerExtensionsInConfig(raw)
	assert.Empty(t, result, "ts source extension overlap should not trigger container conflict")
}

func TestValidateContainerExtensionsInConfig_TSSourceDelegatesToVideo(t *testing.T) {
	initPluginsWithSettings(t, nil)

	raw := map[string]interface{}{
		"plugin_settings": map[string]interface{}{
			"alist_encrypt": map[string]interface{}{
				"suffix": ".sccgt",
			},
		},
	}
	result := validateContainerExtensionsInConfig(raw)
	assert.Contains(t, result, "conflict")
	assert.Contains(t, result, ".sccgt")
	assert.Contains(t, result, "text")
}
