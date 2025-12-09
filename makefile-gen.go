package main

import (
	"fmt"
	"os"
	"strings"
)

// BuildTarget 定义一个 Go 编译目标
type BuildTarget struct {
	Name       string // 目标名称, 如 "encv"
	SourcePath string // 源码路径, 如 "./cmd/encv"
}

// CopyTask 定义一个文件复制任务
type CopyTask struct {
	Name          string // 任务描述, 如 "User Config"
	SourceRelPath string // 源文件相对路径, 如 "config.user.json"
	DestFileName  string // 目标文件名, 通常与源文件名相同
}

func main() {
	// --- 【用户配置区】 ---
	// 默认的输出目录，可以修改它
	defaultOutputDir := "dist"

	// 在这里定义所有需要构建的目标
	buildTargets := []BuildTarget{
		{Name: "encv", SourcePath: "./cmd/encv"},
		{Name: "encv-proxy", SourcePath: "./cmd/encv-proxy"},
	}

	// 在这里定义所有需要复制的文件
	copyTasks := []CopyTask{
		{Name: "User Config", SourceRelPath: "config.user.json", DestFileName: "config.user.json"},
		{Name: "README", SourceRelPath: "README.md", DestFileName: "README.md"},
	}
	// --- 配置区结束 ---

	// 生成 Makefile
	if err := generateMakefile(defaultOutputDir, buildTargets, copyTasks); err != nil {
		fmt.Printf("Error generating Makefile: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Makefile generated successfully.")

	// 生成 PowerShell 脚本
	if err := generatePowerShellScript(defaultOutputDir, buildTargets, copyTasks); err != nil {
		fmt.Printf("Error generating build.ps1: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ build.ps1 generated successfully.")
	fmt.Println("\nAll build scripts generated with default output directory:", defaultOutputDir)
	fmt.Println("You can override it at runtime:")
	fmt.Println("  - On Linux/macOS: make OUTPUT_DIR=dist build-all")
	fmt.Println("  - On Windows: .\\build.ps1 -OutputDir dist")
}

// generateMakefile 生成 Makefile 文件
func generateMakefile(outputDir string, targets []BuildTarget, copies []CopyTask) error {
	var sb strings.Builder

	// .PHONY 声明
	sb.WriteString(".PHONY:")
	for _, t := range targets {
		sb.WriteString(" " + t.Name)
	}
	sb.WriteString(" copy-files build-all run clean\n\n")

	// 定义输出目录变量，允许命令行覆盖
	sb.WriteString(fmt.Sprintf("OUTPUT_DIR ?= %s\n\n", outputDir))

	// clean 目标
	sb.WriteString("# 清理编译产物\nclean:\n")
	sb.WriteString("\t@echo \"Cleaning up...\"\n")
	sb.WriteString("\trm -rf $(OUTPUT_DIR)/\n\n")

	// run 目标
	sb.WriteString("# 运行主程序 (开发模式，使用 go run)\nrun:\n")
	sb.WriteString("\tgo run ./cmd/encv start\n\n")

	// 生成每个目标的构建规则
	for _, t := range targets {
		sb.WriteString(fmt.Sprintf("# 编译 %s\n", t.Name))
		sb.WriteString(fmt.Sprintf("%s:\n", t.Name))
		sb.WriteString(fmt.Sprintf("\t@echo \"Building %s...\"\n", t.Name))
		sb.WriteString("\t@mkdir -p $(OUTPUT_DIR)\n")
		sb.WriteString(fmt.Sprintf("\tgo build -o $(OUTPUT_DIR)/%s %s\n\n", t.Name, t.SourcePath))
	}

	// 生成文件复制规则
	sb.WriteString("# 复制配置和文档文件\ncopy-files:\n")
	sb.WriteString("\t@echo \"Copying necessary files...\"\n")
	sb.WriteString("\t@mkdir -p $(OUTPUT_DIR)\n")
	for _, c := range copies {
		sb.WriteString(fmt.Sprintf("\t@cp %s $(OUTPUT_DIR)/\n", c.SourceRelPath))
	}
	sb.WriteString("\n")

	// build-all 目标
	sb.WriteString("# 编译所有程序并复制文件\nbuild-all:")
	for _, t := range targets {
		sb.WriteString(" " + t.Name)
	}
	sb.WriteString(" copy-files\n")
	sb.WriteString("\t@echo \"All binaries and files built successfully in ./$(OUTPUT_DIR)/\"\n")

	return os.WriteFile("Makefile", []byte(sb.String()), 0644)
}

// generatePowerShellScript 生成 build.ps1 文件
func generatePowerShellScript(outputDir string, targets []BuildTarget, copies []CopyTask) error {
	var sb strings.Builder

	// 文件头，定义参数
	sb.WriteString("# Windows 构建脚本\n")
	sb.WriteString("param(\n")
	sb.WriteString(fmt.Sprintf("    [string]$OutputDir = \"%s\"\n", outputDir)) // 定义参数并设置默认值
	sb.WriteString(")\n\n")

	sb.WriteString("Write-Host \"Starting build process... Output directory: $OutputDir\" -ForegroundColor Green\n\n")

	// 创建输出目录
	sb.WriteString("# 检查并创建输出目录\n")
	sb.WriteString("if (-not (Test-Path -Path $OutputDir)) {\n")
	sb.WriteString("    Write-Host \"Creating '$OutputDir' directory...\"\n")
	sb.WriteString("    New-Item -ItemType Directory -Force -Path $OutputDir\n")
	sb.WriteString("}\n\n")

	// 生成每个目标的构建命令
	for _, t := range targets {
		sb.WriteString(fmt.Sprintf("# 编译 %s\n", t.Name))
		sb.WriteString(fmt.Sprintf("Write-Host \"Building %s.exe...\"\n", t.Name))
		sb.WriteString(fmt.Sprintf("go build -o \"$OutputDir\\%s.exe\" %s\n", t.Name, t.SourcePath))
		sb.WriteString("if ($LASTEXITCODE -ne 0) {\n")
		sb.WriteString(fmt.Sprintf("    Write-Host \"Failed to build %s.exe\" -ForegroundColor Red\n", t.Name))
		sb.WriteString("    exit 1\n")
		sb.WriteString("}\n\n")
	}

	// 生成文件复制命令
	sb.WriteString("# 复制配置和文档文件\n")
	sb.WriteString("Write-Host \"Copying necessary files...\"\n")
	for _, c := range copies {
		// PowerShell 的路径拼接需要用 Join-Path 或手动处理，这里为了简单直接拼接
		sb.WriteString(fmt.Sprintf("Copy-Item -Path \"%s\" -Destination \"$OutputDir\\%s\"\n", c.SourceRelPath, c.DestFileName))
	}
	sb.WriteString("\n")

	// 文件尾
	sb.WriteString("Write-Host \"--------------------------------------------------\" -ForegroundColor Green\n")
	sb.WriteString("Write-Host \"All binaries and files built successfully in ./$OutputDir/\" -ForegroundColor Green\n")
	sb.WriteString("Write-Host \"--------------------------------------------------\" -ForegroundColor Green\n")

	return os.WriteFile("build.ps1", []byte(sb.String()), 0644)
}
