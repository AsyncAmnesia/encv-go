package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Soltus/encv-go/internal/config"
	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// https://raw.githubusercontent.com/Soltus/encv-go/refs/heads/main/config.schema.json
func main() {
	r := &jsonschema.Reflector{}
	err := r.AddGoComments("github.com/Soltus/encv-go", "./internal/config")
	err = r.AddGoComments("github.com/Soltus/encv-go", "./internal/types")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding Go comments: %v\n", err)
		os.Exit(1)
	}

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
