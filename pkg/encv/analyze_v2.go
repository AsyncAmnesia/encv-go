// pkg/encv/analyze_v2.go

package encv

import (
	"context"

	"github.com/Soltus/encv-go/internal/v2/container/detector"
)

// AnalyzeContainerV2 可视化分析 v2 容器文件的结构
// printToStdout: 是否同时打印到标准输出（为了兼容CLI调用）
// 返回格式化的HTML内容和错误
func AnalyzeContainerV2(ctx context.Context, containerPath string, printToStdout bool) (string, error) {
	return detector.AnalyzeContainerV2(ctx, containerPath, printToStdout)
}
