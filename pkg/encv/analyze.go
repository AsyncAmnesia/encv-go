// pkg/encv/analyze.go

package encv

import (
	"context"

	"github.com/Soltus/encv-go/internal/v2/container/detector"
)

// AnalyzeContainer 可视化分析 v3 / v4 ENCV 容器文件的结构。
// v2 容器已从项目中移除（不再属于 SupportedVersions），请使用 v3 或 v4 容器。
//
// 参数：
//   - containerPath: 容器文件绝对路径
//   - printToStdout: 是否同时打印到标准输出（兼容 CLI 调用）
//
// 返回：格式化的 HTML 报告 + 错误。
func AnalyzeContainer(ctx context.Context, containerPath string, printToStdout bool) (string, error) {
	return detector.AnalyzeContainer(ctx, containerPath, printToStdout)
}
