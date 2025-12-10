package file

import (
	"github.com/Soltus/encv-go/internal/utils"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
)

// resolvePath 将前端传来的相对路径解析为安全的绝对路径
// 这是所有文件操作（如 Rename, Analyze）的统一入口
func (c *ControllerV1) resolvePath(reqPath string) (string, error) {
	// 直接调用通用工具函数，c.servingDir 是基础目录，reqPath 是用户路径
	absPath, err := utils.SafeResolveToAbsPath(c.servingDir, reqPath)
	// log.Printf("reqPath->%s | servingDir->%s | absPath->%s", reqPath, c.servingDir, absPath)
	if err != nil {
		// 可以在这里将通用错误包装成业务错误
		return "", gerror.WrapCode(gcode.CodeInvalidOperation, err)
	}
	return absPath, nil
}
