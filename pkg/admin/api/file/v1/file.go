package v1

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
)

type (
	// RenameReq 重命名请求
	RenameReq struct {
		g.Meta  `path:"/file/rename" method:"POST" tags:"File" summary:"Rename a file or directory"`
		OldPath string `json:"oldPath" v:"required" dc:"The relative path of the file/directory to rename, from the serving root"`
		NewName string `json:"newName" v:"required" dc:"The new name for the file/directory"`
	}
	// RenameRes 重命名响应
	RenameRes struct {
		Status  string `json:"status" dc:"Status of the operation, e.g., 'success' or 'error'"`
		Message string `json:"message" dc:"A detailed message about the operation result"`
	}

	// AnalyzeReq 分析文件请求
	AnalyzeReq struct {
		g.Meta `path:"/file/analyze" method:"post" tags:"file" summary:"分析文件"`
		Path   string `json:"path" v:"required#文件路径不能为空"`
	}

	// AnalyzeRes 分析文件响应
	AnalyzeRes struct {
		HTMLContent string `json:"htmlContent"`
	}
)

// IFileV1 文件操作接口 V1
type IFileV1 interface {
	// Rename 重命名文件或目录
	Rename(ctx context.Context, req *RenameReq) (res *RenameRes, err error)
}
