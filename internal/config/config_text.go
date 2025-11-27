package config

// 获取当前配置的文本加密后缀（带点号）
func GetTextEncExtension() string {
	return "." + GlobalConfig.BinExtGroup.Text
}
