package config

// 获取当前配置的视频加密后缀（带点号）
func (c *Config) GetVideoEncExtension() string {
	return "." + c.BinExtGroup.Video
}

// IsSccgvChunkingEnabled 返回是否启用了 SCCGV 分片
func (c *Config) IsSccgvChunkingEnabled() bool {
	return c.SccgvSettings.ChunkSizeMB > 0
}

// IsSccgvChunkingEnabled 返回是否启用了 SCCGV 分片
func (c *Config) IsLightweightMainChunkEnabled() bool {
	return c.SccgvSettings.LightweightMainChunk
}

func (c *Config) GetSccgvChunkSizeBytes() int64 {
	return c.GetSccgvChunkSizeMB() * 1024 * 1024
}

// GetSccgvChunkSizeBytes 获取 SCCGV 分片大小（单位：字节）
// 它会处理校验逻辑：最小 100MB，向下取整
// 如果未启用分片，则返回 0
func (c *Config) GetSccgvChunkSizeMB() int64 {
	if !c.IsSccgvChunkingEnabled() {
		return 0
	}
	sizeMB := c.SccgvSettings.ChunkSizeMB
	if sizeMB < 100 {
		sizeMB = 100
	}
	return sizeMB
}
