package processor

type AudioProcessor struct{}

// 实现 Processor 接口
func (p *AudioProcessor) SupportedMimePrefixes() []string {
	return []string{"audio/"}
}

// 实现 Processor 接口
func (p *AudioProcessor) ShouldProcess(inputPath string) bool {
	return true
}
