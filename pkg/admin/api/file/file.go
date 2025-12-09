// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package file

import (
	"context"

	"github.com/Soltus/encv-go/pkg/admin/api/file/v1"
)

type IFileV1 interface {
	Rename(ctx context.Context, req *v1.RenameReq) (res *v1.RenameRes, err error)
	Analyze(ctx context.Context, req *v1.AnalyzeReq) (res *v1.AnalyzeRes, err error)
}
