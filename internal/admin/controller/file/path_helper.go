package file

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
)

// resolvePath 将前端传来的相对路径解析为安全的绝对路径
// 这是所有文件操作（如 Rename, Analyze）的统一入口
func (c *ControllerV1) resolvePath(reqPath string) (string, error) {
	// 1. 获取 servingDir
	servingDir := c.servingDir
	if servingDir == "" {
		return "", gerror.NewCode(gcode.CodeInternalError, "Internal server error: serving directory is not configured")
	}

	// 2. 【关键修复】智能判断并构建最终的基准目录
	var finalServingDir string
	if filepath.IsAbs(servingDir) {
		// 如果 servingDir 已经是绝对路径（例如 Load 函数处理过的），则直接使用
		finalServingDir = servingDir
	} else {
		// 如果 servingDir 是相对路径（如 "./public"），则与当前工作目录拼接
		baseDir, err := os.Getwd()
		if err != nil {
			log.Printf("ERROR: Could not get current working directory: %v", err)
			return "", gerror.NewCode(gcode.CodeInternalError, "Internal server error: could not determine working directory")
		}
		finalServingDir = filepath.Join(baseDir, servingDir)
	}

	// 3. 清理前端路径，去掉开头的 '/'，避免 filepath.Join 将其视为操作系统绝对路径
	cleanReqPath := strings.TrimPrefix(reqPath, "/")

	// 4. 构建文件的绝对路径
	absPath := filepath.Join(finalServingDir, cleanReqPath)

	// 5. 【安全检查】防止目录遍历攻击
	relPath, err := filepath.Rel(finalServingDir, absPath)
	if err != nil {
		log.Printf("ERROR: Could not resolve relative path for '%s': %v", absPath, err)
		return "", gerror.NewCode(gcode.CodeInternalError, "Internal server error: path resolution failed")
	}
	if strings.HasPrefix(relPath, "..") {
		log.Printf("WARN: Forbidden attempt to access path outside serving directory: '%s'", absPath)
		return "", gerror.NewCode(gcode.CodeInvalidOperation, "Forbidden: access to path outside serving directory is not allowed")
	}

	return absPath, nil
}
