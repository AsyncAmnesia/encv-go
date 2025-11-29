package config

// 获取当前配置的图像加密后缀（带点号）
func (c *Config) GetIframeEncExtension() string {
	return "." + c.BinExtGroup.Iframe
}
