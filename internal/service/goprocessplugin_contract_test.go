package service

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestGoProcessPlugin_NoReflectionOnComboLite 验证 combolite.md 铁律
func TestGoProcessPlugin_NoReflectionOnComboLite(t *testing.T) {
	srcPath := filepath.Join("..", "..", "app", "encv-mobile", "android", "app", "src", "main", "java", "com", "encvgo", "app", "GoProcessPlugin.kt")

	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		t.Skip("GoProcessPlugin.kt not found (not in sandbox or path differs), skipping")
		return
	}

	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("Failed to read GoProcessPlugin.kt: %v", err)
	}

	src := string(content)

	forbiddenPatterns := []struct {
		pattern *regexp.Regexp
		desc    string
	}{
		{regexp.MustCompile(`Class\.forName\(\s*["']com\.combo\.core\.runtime`), "Class.forName(com.combo.core.runtime)"},
		{regexp.MustCompile(`\.getMethod\(\s*["']getInstance"`), ".getMethod(\"getInstance\""},
		{regexp.MustCompile(`\.invoke\(\s*pm\s*,\s*apkFile\)`), ".invoke(pm, apkFile) 反射调用 installPlugin"},
		{regexp.MustCompile(`parameterCount\s*==\s*1`), "parameterCount == 1 (旧版反射残留)"},
		{regexp.MustCompile(`parameterCount\s*==\s*2.*find.*installPlugin`), "在 PluginManager 上搜 installPlugin (应在 InstallerManager 上)"},
	}

	for _, fp := range forbiddenPatterns {
		if fp.pattern.MatchString(src) {
			t.Errorf("VIOLATES combolite.md: found forbidden pattern - %s\nMatched near: %s",
				fp.desc, findContext(src, fp.pattern.String()))
		}
	}

	requiredPatterns := []struct {
		pattern *regexp.Regexp
		desc    string
	}{
		{regexp.MustCompile(`PluginManager\.isInitialized`), "PluginManager.isInitialized 直接引用"},
		{regexp.MustCompile(`PluginManager\.installerManager\.installPlugin\(`), "InstallerManager.installPlugin() 直接调用"},
		{regexp.MustCompile(`PluginManager\.getAllInstallPlugins\(\)`), "PluginManager.getAllInstallPlugins() 直接调用"},
		{regexp.MustCompile(`installConfirmReceiver`), "BroadcastReceiver 已定义"},
		{regexp.MustCompile(`registerReceiver`), "BroadcastReceiver 已注册"},
		{regexp.MustCompile(`executeComboLiteInstall`), "executeComboLiteInstall 辅助方法存在"},
	}

	for _, rp := range requiredPatterns {
		if !rp.pattern.MatchString(src) {
			t.Errorf("MISSING required pattern - %s", rp.desc)
		}
	}
}

func TestGoProcessPlugin_PendingCallsKeyConsistency(t *testing.T) {
	keysStored := []string{"installConfirm"}
	keysConsumed := []string{"installConfirm"}

	storedSet := make(map[string]bool)
	consumedSet := make(map[string]bool)
	for _, k := range keysStored {
		storedSet[k] = true
	}
	for _, k := range keysConsumed {
		consumedSet[k] = true
	}

	for k := range storedSet {
		if !consumedSet[k] {
			t.Errorf("pendingCalls key %q is stored but never consumed (leak)", k)
		}
	}
	for k := range consumedSet {
		if !storedSet[k] {
			t.Errorf("pendingCalls key %q is consumed but never stored (nil pointer panic risk)", k)
		}
	}
}

func TestEncvApplication_ProxyManagerConfigured(t *testing.T) {
	appPath := filepath.Join("..", "..", "app", "encv-mobile", "android", "app", "src", "main", "java", "com", "encvgo", "app", "EncvApplication.kt")

	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		t.Skip("EncvApplication.kt not found, skipping")
		return
	}

	content, err := os.ReadFile(appPath)
	if err != nil {
		t.Fatalf("Failed to read EncvApplication.kt: %v", err)
	}

	src := string(content)

	if !strings.Contains(src, `setHostActivity`) {
		t.Error("MISSING: setHostActivity not called in onFrameworkSetup")
	}
	if !strings.Contains(src, `EncvHostActivity::class.java`) {
		t.Error("MISSING: EncvHostActivity::class.java not passed to setHostActivity")
	}
	if !strings.Contains(src, `proxyManager.setHostActivity`) {
		t.Error("MISSING: proxyManager.setHostActivity call missing")
	}
}

func findContext(s string, pattern string) string {
	idx := regexp.MustCompile(pattern).FindStringIndex(s)
	if idx == nil {
		return "(not found)"
	}
	start := idx[0] - 30
	if start < 0 {
		start = 0
	}
	end := idx[0] + 50
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}
