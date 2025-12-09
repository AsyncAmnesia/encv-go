package file

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/Soltus/encv-go/internal/v2/container/detector"
	"github.com/Soltus/encv-go/internal/v2/plugins"
	v1 "github.com/Soltus/encv-go/pkg/admin/api/file/v1"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
)

// cAnalyze implements apiv1.IFileV1.
// @summary 分析文件
// @description 分析指定路径的文件，如果是容器文件则显示其结构，否则显示基本信息。
// @tags File
// @accept json
// @produce json
// @param data body v1.AnalyzeReq true "分析请求"
// @success 200 {object} v1.AnalyzeRes "成功"
// @router /file/analyze [post]
func (c *ControllerV1) Analyze(ctx context.Context, req *v1.AnalyzeReq) (res *v1.AnalyzeRes, err error) {
	absPath, err := c.resolvePath(req.Path)
	if err != nil {
		// 错误已在 resolvePath 中处理并格式化，直接返回
		return nil, err
	}

	// 检查文件是否存在
	stat, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, gerror.NewCode(gcode.CodeNotFound, "Not found: the file does not exist")
		}
		return nil, gerror.NewCode(gcode.CodeInternalError, "Internal server error: could not stat file")
	}

	var htmlContent string

	currentExt := strings.ToLower(filepath.Ext(absPath))

	// 【核心修复】从 plugins 包动态获取所有注册的容器扩展名
	containerExtensions := plugins.GetAllRegisteredContainerExtensions()
	isContainerFile := false
	for _, ext := range containerExtensions {
		if currentExt == strings.ToLower(ext) {
			isContainerFile = true
			break
		}
	}

	// 根据判断结果执行不同的分析逻辑
	if isContainerFile {
		htmlContent, err = detector.AnalyzeContainerV2(ctx, absPath, false)
		if err != nil {
			log.Printf("ERROR: Failed to analyze container '%s': %v", absPath, err)
			return nil, gerror.NewCode(gcode.CodeInternalError, "Analysis failed: "+err.Error())
		}
	} else {
		// 普通文件分析 (这里需要导入 html/template 包)
		htmlContent = fmt.Sprintf(`
			<h3>Basic File Information</h3>
			<table>
				<tr><td><strong>File Name:</strong></td><td>%s</td></tr>
				<tr><td><strong>Size:</strong></td><td>%d bytes</td></tr>
				<tr><td><strong>Mode:</strong></td><td>%s</td></tr>
				<tr><td><strong>Modified:</strong></td><td>%s</td></tr>
			</table>`,
			template.HTMLEscapeString(stat.Name()),
			stat.Size(),
			stat.Mode(),
			stat.ModTime().Format(time.RFC1123),
		)
	}

	return &v1.AnalyzeRes{
		HTMLContent: htmlContent,
	}, nil
}
