// internal/tools/register.go
//
// 一站式注册所有内置工具。
// Server 启动时调用 RegisterAll() 一次性把 11 个工具塞进 GlobalRegistry。
package tools

// RegisterAll 把所有内置工具注册到 GlobalRegistry。
// 失败时 panic（启动期问题应立即暴露，不要静默）。
//
// 工具清单（v1 + v2 共 11 个）：
//   v1: list_mounts, list_files, read_file          (兼容)
//   v2: search_files, get_metadata, read_file_v2,
//       edit_metadata, batch_rename, delete_file,
//       command_run
//
// 幂等性：同名 tool 已存在则跳过（不报错）。
// 这让多个测试 init / NewServer 共存成为可能：
//   - e2e_test.go 的 init 触发一次
//   - WebDAV 测试通过 NewServer 触发第二次
//   第二次全部为 no-op。
func RegisterAll() {
	for _, def := range AllToolDefs() {
		if GlobalRegistry.Has(def.Name) {
			continue
		}
		MustRegister(def)
	}
}

// AllToolDefs 返回所有内置 ToolDef（顺序：v1 先，v2 后）。
// 测试 / 文档生成可使用。
func AllToolDefs() []*ToolDef {
	return []*ToolDef{
		// v1 工具（兼容旧剧本）
		// 注意：v1 的 list_mounts / list_files / read_file 仍由
		// agent_fs_bridge.go 的 executeFSTool 处理，**不再走 ToolRegistry**
		// 保持向后兼容（如果注册同名 tool 会冲突）
		// v2 工具
		SearchFilesDef(),
		GetMetadataDef(),
		ReadFileV2Def(),
		EditMetadataDef(),
		BatchRenameDef(),
		DeleteFileDef(),
		CommandRunDef(),
	}
}
