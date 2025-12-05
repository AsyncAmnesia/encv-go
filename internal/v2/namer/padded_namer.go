package namer

import "fmt"

const (
	ChunkNameRulePadded = ".padded" // 这是一个新的规则标识符
)

// PaddedNamer 是一个补零命名的实现，例如 .001, .002
type PaddedNamer struct {
	baseNamer    BaseNamer
	mainChunkExt string // 【关键】内部存储主容器后缀
	padding      int    // 补零的位数，例如 3 表示 001
}

func NewPaddedNamer(mainChunkExt string, baseNamer BaseNamer, padding int) *PaddedNamer {
	return &PaddedNamer{
		mainChunkExt: mainChunkExt,
		baseNamer:    baseNamer,
		padding:      padding,
	}
}

func (n *PaddedNamer) GenerateMainChunkName(baseName string) string {
	return baseName + n.mainChunkExt
}

func (n *PaddedNamer) ParseFirstChunkName(firstChunkPath string) (string, error) {
	return parseFirstChunkNameHelper(firstChunkPath, n.mainChunkExt)
}

func (n *PaddedNamer) GenerateDataChunkName(baseName string, index int) string {
	return fmt.Sprintf("%s.%0*d", baseName, n.padding, index)
}

func (n *PaddedNamer) GetFirstDataChunkIndex() int {
	return 1
}
