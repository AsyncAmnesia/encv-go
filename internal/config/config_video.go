package config

// 获取当前配置的视频加密后缀（带点号）
func GetVideoEncExtension() string {
	return "." + GlobalConfig.BinExtGroup.Video
}

// GetSccgvChunkSize 获取 SCCGV 分片大小（单位：字节）
// 它会处理校验逻辑：最小 100MB，向下取整
// 注意：此函数仅在 IsSccgvChunkingEnabled() 返回 true 时才应被调用
func GetSccgvChunkSize() int {
	cfg, err := LoadUserConfig()
	if err != nil {
		// 理论上不应到达这里，但提供一个安全的默认值
		return 100 * 1024 * 1024
	}

	sizeMB := cfg.SccgvSettings.ChunkSizeMB
	if sizeMB < 100 {
		sizeMB = 100
	}
	return sizeMB * 1024 * 1024
}
