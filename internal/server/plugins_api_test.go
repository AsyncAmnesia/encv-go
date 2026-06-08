package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPluginsTestRouter(t *testing.T) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	s := &Server{}
	router.GET("/api/plugins", s.handlePluginsGin)
	return router
}

func TestHandlePluginsGin_ReturnsAllPlugins(t *testing.T) {
	router := setupPluginsTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/plugins", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	plugins, ok := response["plugins"].([]interface{})
	require.True(t, ok, "response should contain 'plugins' array")
	assert.Len(t, plugins, 7, "plugins array length should match plugins.Plugins slice length")
}

func TestHandlePluginsGin_ContainsVideoPlugin(t *testing.T) {
	router := setupPluginsTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/plugins", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string][]map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	found := false
	for _, p := range response["plugins"] {
		if p["name"] == "video" {
			found = true
			exts, ok := p["supportedExtensions"].([]interface{})
			require.True(t, ok, "video plugin should have supportedExtensions array")
			assert.NotEmpty(t, exts, "video plugin should have supported extensions")
			break
		}
	}
	assert.True(t, found, "result should contain a plugin with name='video'")
}

func TestHandlePluginsGin_EachPluginHasRequiredFields(t *testing.T) {
	router := setupPluginsTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/plugins", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string][]map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	for i, p := range response["plugins"] {
		_, hasName := p["name"]
		_, hasExts := p["supportedExtensions"]
		_, hasMime := p["supportedMimePrefixes"]
		_, hasContainer := p["containerExtension"]

		assert.True(t, hasName, "plugin at index %d should have 'name' field", i)
		assert.True(t, hasExts, "plugin at index %d should have 'supportedExtensions' field", i)
		assert.True(t, hasMime, "plugin at index %d should have 'supportedMimePrefixes' field", i)
		assert.True(t, hasContainer, "plugin at index %d should have 'containerExtension' field", i)
	}
}

func TestHandlePluginsGin_VideoPluginHasVideoMimePrefix(t *testing.T) {
	router := setupPluginsTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/plugins", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string][]map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	var videoPlugin map[string]interface{}
	for _, p := range response["plugins"] {
		if p["name"] == "video" {
			videoPlugin = p
			break
		}
	}
	require.NotNil(t, videoPlugin, "should find video plugin in response")

	mimePrefixes, ok := videoPlugin["supportedMimePrefixes"].([]interface{})
	require.True(t, ok, "video plugin should have supportedMimePrefixes array")
	assert.Contains(t, mimePrefixes, "video/", "video plugin's supportedMimePrefixes should contain 'video/'")
}
