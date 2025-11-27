package config

// 获取当前配置的图像加密后缀（带点号）
func GetImageEncExtension() string {
	return "." + GlobalConfig.BinExtGroup.Image
}
