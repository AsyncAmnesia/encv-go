package file

import (
	"context"
	"log"
	"os"
	"path/filepath"

	v1 "github.com/Soltus/encv-go/pkg/admin/api/file/v1"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
)

// cRename implements apiv1.IFileV1.
// @summary 重命名文件或目录
// @description 根据提供的旧路径和新名称重命名文件或目录。
// @tags File
// @accept json
// @produce json
// @param data body v1.RenameReq true "重命名请求"
// @success 200 {object} v1.RenameRes "成功"
// @router /file/rename [post]
func (c *ControllerV1) Rename(ctx context.Context, req *v1.RenameReq) (res *v1.RenameRes, err error) {

	// 1. 验证新名称
	if req.NewName == "." || req.NewName == ".." || filepath.Base(req.NewName) != req.NewName {
		return nil, gerror.NewCode(gcode.CodeInvalidParameter, "Bad request: newName is not a valid filename")
	}

	oldAbsPath, err := c.resolvePath(req.OldPath)
	if err != nil {
		// 错误已在 resolvePath 中处理并格式化，直接返回
		return nil, err
	}

	// 6. 【核心修复】构建新的绝对路径
	newAbsPath := filepath.Join(filepath.Dir(oldAbsPath), req.NewName)

	// 7. 检查目标是否存在 (使用绝对路径)
	if _, err := os.Stat(newAbsPath); err == nil {
		log.Printf("WARN: Rename failed, destination already exists: '%s'", newAbsPath)
		return nil, gerror.NewCode(gcode.CodeValidationFailed, "Conflict: a file with that name already exists")
	}

	// 8. 【核心】执行重命名 (使用绝对路径)
	log.Printf("INFO: Attempting to rename '%s' to '%s'", oldAbsPath, newAbsPath)
	err = os.Rename(oldAbsPath, newAbsPath)
	if err != nil {
		log.Printf("ERROR: Failed to rename file: %v", err)
		if os.IsNotExist(err) {
			return nil, gerror.NewCode(gcode.CodeNotFound, "Not found: the original file does not exist")
		} else if os.IsPermission(err) {
			return nil, gerror.NewCode(gcode.CodeNotAuthorized, "Forbidden: permission denied")
		} else {
			return nil, gerror.NewCode(gcode.CodeInternalError, "Internal server error: could not rename file")
		}
	}

	log.Printf("INFO: Successfully renamed to '%s'", newAbsPath)
	return &v1.RenameRes{
		Status:  "success",
		Message: "File renamed successfully.",
	}, nil
}
