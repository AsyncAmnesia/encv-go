package container

import (
	"io"
)

// PackedData 包含解包后的数据
type PackedData struct {
	KVIData    []byte        // KVI 数据
	DataStream io.ReadCloser // 通用的加密数据流
}
