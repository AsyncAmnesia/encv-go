package utils

import (
	"github.com/Soltus/encv-go/internal/config"
)

// getContentType 优先从全局配置中查找 MIME 类型，找不到则返回默认值
func GetContentType(format string) string {
	if ct, ok := config.ContentTypes[format]; ok {
		return ct
	}
	// 如果在全局映射中找不到，返回通用的二进制流类型
	return "application/octet-stream"
}
