package namer

import (
	"fmt"
)

const (
	ChunkNameRuleSequential = ".part"
)

// SequentialNamer 是一个顺序命名的实现，例如 .part1, .part2
type SequentialNamer struct {
	baseNamer    BaseNamer
	suffix       string // 例如 ".part"
	mainChunkExt string // 【关键】内部存储主容器后缀
}

func NewSequentialNamer(mainChunkExt string, baseNamer BaseNamer, suffix string) *SequentialNamer {
	return &SequentialNamer{
		mainChunkExt: mainChunkExt,
		baseNamer:    baseNamer,
		suffix:       suffix,
	}
}

// GenerateMainChunkName 使用内部存储的 mainChunkExt
func (n *SequentialNamer) GenerateMainChunkName(baseName string) string {
	return baseName + n.mainChunkExt
}

// ParseFirstChunkName 使用内部存储的 mainChunkExt
func (n *SequentialNamer) ParseFirstChunkName(firstChunkPath string) (string, error) {
	return parseFirstChunkNameHelper(firstChunkPath, n.mainChunkExt)
}

// GenerateDataChunkName 覆盖核心结构体的方法，实现自己的逻辑
func (n *SequentialNamer) GenerateDataChunkName(baseName string, index int) string {
	return fmt.Sprintf("%s%s%d", baseName, n.suffix, index)
}
func (n *SequentialNamer) GetFirstDataChunkIndex() int {
	return 1
}
