// internal/server/agent_plugin_bridge.go
//
// 把 encv-go 插件系统接入 AI agent 工具系统。
//
// 设计目标：
//   - 12 个工具：6 个插件（video/audio/image/wps/pdf/text）× 2 操作（encrypt/decrypt）
//   - 工具描述用中文，符合用户使用习惯
//   - schema 极简：仅 input_path / output_dir（先满足"AI 能调用"，复杂参数后续扩展）
//   - 复用 plugins.EncryptFileWithPlugin / DecryptContainerWithPlugin 高层 API
//   - 安全：所有 encrypt/decrypt 操作都标记为 NeedConfirm=true，前端必须弹 ApprovalCard
//
// 不做的事（明确范围）：
//   - 不集成 OpenList 工具（用户已砍掉）
//   - 不做 4 决策的复杂 confirm 流程（先 accept/decline 两个够用）
//   - 不做断点续传（先做核心工具调用闭环）
//   - 不做虚拟列表/分组渲染（后续 phase）
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/Soltus/encv-go/internal/v2/plugins"
	encvPlugins "github.com/Soltus/encv-go/pkg/encv/plugins"
)

// pluginToolDef 描述一个由插件支持的 agent 工具
type pluginToolDef struct {
	name        string
	description string
	op          string // "encrypt" | "decrypt"
}

// pluginOpsByName 建立 "video_encrypt" / "video_decrypt" → pluginName + op 的映射
var pluginOpsByName = func() map[string]pluginToolDef {
	m := make(map[string]pluginToolDef)
	for _, p := range encvPlugins.Plugins() {
		// 跳过 OpenList 相关插件（用户已砍掉 OpenList 集成）
		// 命名约定：以 "alist" 开头的插件都属于 OpenList 工具族
		if strings.HasPrefix(p.Name(), "alist") {
			continue
		}
		// 中文插件名映射（用户可见名）
		cnName := pluginNameCN(p.Name())
		m[p.Name()+"_encrypt"] = pluginToolDef{
			name:        p.Name() + "_encrypt",
			description: fmt.Sprintf("使用 %s 插件加密文件为 .encv 容器", cnName),
			op:          "encrypt",
		}
		m[p.Name()+"_decrypt"] = pluginToolDef{
			name:        p.Name() + "_decrypt",
			description: fmt.Sprintf("使用 %s 插件解密 .encv 容器为原始文件", cnName),
			op:          "decrypt",
		}
	}
	return m
}()

// pluginNameCN 把插件名翻译成中文（用户可读）
func pluginNameCN(name string) string {
	switch name {
	case "video":
		return "视频"
	case "audio":
		return "音频"
	case "image":
		return "图片"
	case "wps":
		return "WPS 文档"
	case "pdf":
		return "PDF"
	case "text":
		return "文本"
	default:
		return name
	}
}

// toolSchemaEncrypt / toolSchemaDecrypt 极简 schema：input_path + output_dir
var toolSchemaEncrypt = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"input_path": map[string]interface{}{
			"type":        "string",
			"description": "要加密的源文件绝对路径",
		},
		"output_dir": map[string]interface{}{
			"type":        "string",
			"description": "加密产物输出目录（绝对路径）",
		},
	},
	"required": []string{"input_path", "output_dir"},
}

var toolSchemaDecrypt = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"container_path": map[string]interface{}{
			"type":        "string",
			"description": ".encv 容器文件绝对路径",
		},
		"output_dir": map[string]interface{}{
			"type":        "string",
			"description": "解密产物输出目录（绝对路径）",
		},
	},
	"required": []string{"container_path", "output_dir"},
}

// ListPluginTools 返回所有插件工具的元信息（name + description + schema）。
// 前端可调用以渲染"已启用工具"列表。
func ListPluginTools() []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(pluginOpsByName))
	for _, def := range pluginOpsByName {
		schema := toolSchemaEncrypt
		if def.op == "decrypt" {
			schema = toolSchemaDecrypt
		}
		out = append(out, map[string]interface{}{
			"name":        def.name,
			"description": def.description,
			"parameters":  schema,
			"needConfirm": true, // 加密/解密均需用户确认（写入文件是高危操作）
		})
	}
	return out
}

// executePluginTool 执行一个插件工具调用。
// 返回 (outputJSON, error)。outputJSON 描述执行结果（output_path / error / 耗时等）。
//
// 调用方负责决策 confirm / decline / cancel。
func executePluginTool(ctx context.Context, toolName, argsJSON string) (string, error) {
	def, ok := pluginOpsByName[toolName]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}

	switch def.op {
	case "encrypt":
		return runPluginEncrypt(ctx, def, argsJSON)
	case "decrypt":
		return runPluginDecrypt(ctx, def, argsJSON)
	default:
		return "", fmt.Errorf("unknown op: %s", def.op)
	}
}

func runPluginEncrypt(ctx context.Context, def pluginToolDef, argsJSON string) (string, error) {
	var args struct {
		InputPath string `json:"input_path"`
		OutputDir string `json:"output_dir"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return errJSON("invalid_args", err.Error()), nil
	}
	if args.InputPath == "" || args.OutputDir == "" {
		return errJSON("missing_args", "input_path 和 output_dir 必填"), nil
	}

	// 推断插件（按文件类型）
	pluginName := strings.TrimSuffix(def.name, "_encrypt")
	p, err := plugins.FindEncryptingPlugin(args.InputPath)
	if err != nil {
		// 兜底：尝试用 toolName 找 plugin
		var err2 error
		p, err2 = findPluginByName(pluginName)
		if err2 != nil {
			return errJSON("plugin_not_found", err.Error()), nil
		}
	}

	// 验证插件名匹配（防止 AI 用 video 工具处理 pdf 文件）
	if p.Name() != pluginName {
		return errJSON("plugin_mismatch",
			fmt.Sprintf("文件类型需要 %s 插件，但你调用的是 %s 工具", p.Name(), pluginName),
		), nil
	}

	inputRootDir := filepath.Dir(args.InputPath)
	outputPath, err := plugins.EncryptFileWithPlugin(ctx, p, args.InputPath, inputRootDir, args.OutputDir)
	if err != nil {
		slog.Warn("agent: plugin encrypt failed", "plugin", p.Name(), "input", args.InputPath, "error", err)
		return errJSON("encrypt_failed", err.Error()), nil
	}

	return okJSON(map[string]interface{}{
		"plugin":   p.Name(),
		"op":       "encrypt",
		"input":    args.InputPath,
		"output":   outputPath,
	}), nil
}

func runPluginDecrypt(ctx context.Context, def pluginToolDef, argsJSON string) (string, error) {
	var args struct {
		ContainerPath string `json:"container_path"`
		OutputDir     string `json:"output_dir"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return errJSON("invalid_args", err.Error()), nil
	}
	if args.ContainerPath == "" || args.OutputDir == "" {
		return errJSON("missing_args", "container_path 和 output_dir 必填"), nil
	}

	// 推断插件
	pluginName := strings.TrimSuffix(def.name, "_decrypt")
	p, err := plugins.FindDecryptingPlugin(args.ContainerPath)
	if err != nil {
		var err2 error
		p, err2 = findPluginByName(pluginName)
		if err2 != nil {
			return errJSON("plugin_not_found", err.Error()), nil
		}
	}

	if p.Name() != pluginName {
		return errJSON("plugin_mismatch",
			fmt.Sprintf("容器类型需要 %s 插件，但你调用的是 %s 工具", p.Name(), pluginName),
		), nil
	}

	outputPath, err := plugins.DecryptContainerWithPlugin(ctx, p, args.ContainerPath, args.OutputDir)
	if err != nil {
		slog.Warn("agent: plugin decrypt failed", "plugin", p.Name(), "input", args.ContainerPath, "error", err)
		return errJSON("decrypt_failed", err.Error()), nil
	}

	return okJSON(map[string]interface{}{
		"plugin":   p.Name(),
		"op":       "decrypt",
		"input":    args.ContainerPath,
		"output":   outputPath,
	}), nil
}

// findPluginByName 在 plugins 列表中按 name 查找插件
func findPluginByName(name string) (plugins.Plugin, error) {
	for _, p := range plugins.Plugins {
		if p.Name() == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("plugin %q not found in registry", name)
}

// okJSON 把 map 序列化为成功 JSON 字符串
func okJSON(m map[string]interface{}) string {
	b, _ := json.Marshal(m)
	return string(b)
}

// errJSON 把错误包装为 {"error": code, "message": msg} JSON
func errJSON(code, msg string) string {
	return okJSON(map[string]interface{}{"error": code, "message": msg})
}
