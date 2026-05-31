# Tasks

- [ ] Task 1: 对齐 alist-encrypt 插件文件名加密与 alist-encrypt-go
  - [ ] 1.1 克隆 alist-encrypt-go 到 /tmp/alist-encrypt-go，分析其 EncryptedName 实现细节（编码格式、CRC6 多项式、AES-CTR 模式参数）
  - [ ] 1.2 对比 `internal/alistencrypt/filename.go` 与参考实现的差异，列出所有不一致点
  - [ ] 1.3 修复 filename.go 使其与参考实现二进制兼容（EncryptedName 结构、mixBase64 编码表、CRC6 校验逻辑）
  - [ ] 1.4 同步更新 cipher.go 和 aesctr.go 确保密钥派生和 CTR 模式参数一致
  - [ ] 1.5 更新前端 `useAlistEncrypt.ts` 的 decodeAlistFilename 函数，确保解码流程匹配后端

- [ ] Task 2: ENCV v4 容器文件名混淆 — 数据结构扩展
  - [ ] 2.1 在 `types/segment_v4.go` 的 Manifest_v4 中新增 OriginalName 和 FilenameAlgorithm 字段
  - [ ] 2.2 在 `types/header_v4.go` 中定义 FlagFilenameEncrypted 常量 (0x0010)
  - [ ] 2.3 在 `types/types.go` 中定义 FilenameObfuscator 接口和 AES-GCM-SIV 默认实现
  - [ ] 2.4 确保 HeaderV4 的 WriteHeaderV4/ReadHeaderV4 正确处理新 flag 位

- [ ] Task 3: ENCV v4 容器文件名混淆 — 后端集成
  - [ ] 3.1 实现 FilenameObfuscator 接口的 AES-GCM-SIV 策略（使用容器密码派生独立密钥）
  - [ ] 3.2 在 v4 容器创建流程中，当 FlagFilenameEncrypted 被设置时自动加密原始文件名并写入 Manifest_v4.OriginalName
  - [ ] 3.3 在 v4 容器打开/读取流程中，从 Manifest 还原原始文件名供 API 层使用
  - [ ] 3.4 新增 API 端点 `GET /api/v1/file/original-name?path=...` 返回解密后的原始文件名

- [ ] Task 4: ENCV v4 容器文件名混淆 — 前端集成
  - [ ] 4.1 在 Files.vue 文件列表渲染中，检测 v4 容器的 FlagFilenameEncrypted 标志
  - [ ] 4.2 若标志存在则调用 original-name API 或直接从列表接口返回的元数据中获取原始文件名显示
  - [ ] 4.3 若标志不存在则保持现有行为不变
  - [ ] 4.4 在 PluginSettings 中为通用 v4 容器添加 filename_obfuscation_strategy 配置项

- [ ] Task 5: 验证与测试
  - [ ] 5.1 编写 Go 单元测试验证 alist-encrypt 文件名加解密的往返一致性（含 CRC6 校验边界情况）
  - [ ] 5.2 编写 Go 单元测试验证 v4 容器 OriginalName 字段的序列化/反序列化
  - [ ] 5.3 手动端到端测试：创建带文件名混淆的 v4 容器 → 前端显示原始名称 → 重启后再读取一致性
  - [ ] 5.4 vue-tsc + vitest + vite build 全量验证

# Task Dependencies
- [Task 2] depends on [Task 1] (v4 架构设计可并行，但建议先完成 alist-encrypt 对齐作为参考)
- [Task 3] depends on [Task 2]
- [Task 4] depends on [Task 3]
- [Task 5] depends on [Task 1, Task 3, Task 4]
