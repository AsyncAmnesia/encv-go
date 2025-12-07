package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// https://raw.githubusercontent.com/Soltus/encv-go/main/config.schema.json
func main() {
	r := &jsonschema.Reflector{}
	err := r.AddGoComments("github.com/Soltus/encv-go", "./internal/config")
	err = r.AddGoComments("github.com/Soltus/encv-go", "./internal/v2/types")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding Go comments: %v\n", err)
		os.Exit(1)
	}
	// 1. 生成基础 Schema
	schema := r.Reflect(&config.Config{})

	// --- 【关键】正确地添加 $schema 属性 ---

	// 1. 检查 Definitions 是否存在
	if schema.Definitions == nil {
		fmt.Fprintf(os.Stderr, "Error: Generated schema has no definitions.\n")
		os.Exit(2)
	}

	// 2. 从 Definitions 中获取名为 "Config" 的定义
	configDef, ok := schema.Definitions["Config"]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: Could not find 'Config' definition in generated schema.\n")
		os.Exit(3)
	}

	// 3. 检查 Config 定义的 Properties 是否为 nil，如果是则初始化
	if configDef.Properties == nil {
		configDef.Properties = orderedmap.New[string, *jsonschema.Schema]()
	}

	// 4. 现在，安全地添加 $schema 属性
	configDef.Properties.Set("$schema", &jsonschema.Schema{
		Type:        "string",
		Description: "The JSON Schema file for validation, used by editors.",
		Format:      "uri",
	})

	// 调用辅助函数，动态注入插件 Schema
	if err := injectPluginSchemas(r, schema); err != nil {
		fmt.Fprintf(os.Stderr, "Error injecting plugin schemas: %v\n", err)
		os.Exit(2)
	}

	// --- 文件写入逻辑保持不变 ---
	schemaBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling schema to JSON: %v\n", err)
		os.Exit(4)
	}

	targetFile := "config.schema.json"
	tempFile := targetFile + ".tmp"
	err = os.WriteFile(tempFile, schemaBytes, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing temporary schema file: %v\n", err)
		os.Exit(5)
	}

	err = os.Rename(tempFile, targetFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error renaming temporary file to target file: %v\n", err)
		os.Exit(6)
	}

	fmt.Printf("✅ Successfully generated %s\n", targetFile)
}

// injectPluginSchemas 遍历所有已注册的插件，将它们的配置 Schema 动态注入到
// 主 schema 的 `plugin_settings` 部分。
func injectPluginSchemas(r *jsonschema.Reflector, schema *jsonschema.Schema) error {
	// 1. 健壮性检查：确保 Definitions 和 Config 定义存在
	if schema.Definitions == nil {
		return fmt.Errorf("generated schema has no definitions")
	}
	configDef, ok := schema.Definitions["Config"]
	if !ok {
		return fmt.Errorf("could not find 'Config' definition in generated schema")
	}

	// 2. 获取 `plugin_settings` 属性的 Schema 对象
	pluginSettingsProp, ok := configDef.Properties.Get("plugin_settings")
	if !ok {
		return fmt.Errorf("'plugin_settings' property not found in Config definition")
	}

	// 3. 准备 `plugin_settings` 的 Schema 结构
	pluginSettingsSchema := pluginSettingsProp
	pluginSettingsSchema.Type = "object"
	pluginSettingsSchema.Description = "插件专属配置。键是插件名，值是该插件的配置对象。"
	pluginSettingsSchema.Properties = orderedmap.New[string, *jsonschema.Schema]()
	pluginSettingsSchema.AdditionalProperties = jsonschema.FalseSchema // 禁止未定义的插件

	// 4. 遍历所有插件，为每个插件生成并注入其配置 Schema
	for _, p := range plugins.Plugins {
		pluginName := p.Name()
		settingsType := p.GetSettingsSchemaType()

		// 为插件的配置结构体生成 Schema
		pluginConfigSchema := r.Reflect(settingsType)

		// 将生成的 Schema 添加到 `plugin_settings` 的 `properties` 中
		pluginSettingsSchema.Properties.Set(pluginName, pluginConfigSchema)
	}

	return nil
}
