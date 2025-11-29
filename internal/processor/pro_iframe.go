package processor

type IframeProcessor struct{}

// 实现 Processor 接口
func (p *IframeProcessor) SupportedMimePrefixes() []string {
	return []string{
		"application/msword", // doc
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document", // docx
		"application/vnd.ms-excel", // xls
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",         // xlsx
		"application/vnd.ms-powerpoint",                                             // ppt
		"application/vnd.openxmlformats-officedocument.presentationml.presentation", // pptx
		"application/pdf",      // pdf
		"application/epub+zip", // epub
	}
}

// 实现 Processor 接口
func (p *IframeProcessor) ShouldProcess(inputPath string) bool {
	return true
}
