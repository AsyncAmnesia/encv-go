# Tasks

- [ ] Task 1: 对齐 alist-encrypt 插件文件名加密与 alist-encrypt-go
  - [ ] 1.1 克隆 alist-encrypt-go 到 /tmp/alist-encrypt-go，分析 EncryptedName 实现细节（编码格式、CRC6 多项式、AES-CTR 参数、mixBase64 字符表）
  - [ ] 1.2 对比 `internal/alistencrypt/filename.go` 与参考实现的差异，列出所有不一致点
  - [ ] 1.3 修复 filename.go 使其二进制兼容（EncryptedName 结构、mixBase64 编解码、CRC6 校验）
  - [ ] 1.4 同步 cipher.go / aesctr.go 确保密钥派生和 CTR 模式参数一致
  - [ ] 1.5 更新前端 `useAlistEncrypt.ts` 的 decodeAlistFilename 匹配后端格式

- [ ] Task 2: ENCV v4 容器 — 数据结构与标志位
  - [ ] 2.1 在 Manifest_v4 中新增 OriginalName 和 FilenameAlgorithm 字段
  - [ ] 2.2 定义 FlagFilenameEncrypted 常量 (0x0010)
  - [ ] 2.3 确保 WriteHeaderV4/ReadHeaderV4 正确处理新 flag 位（不破坏已有逻辑）

- [ ] Task 3: ENCV v4 容器 — 文件名解析优先级与展示层抽象
  - [ ] 3.1 后端文件列表 API：v4 容器返回时携带 original_name 元数据（明文或密文）
  - [ ] 3.2 实现文件名解析优先级逻辑：Manifest.original_name > 物理文件名
  - [ ] 3.3 Files.vue：渲染文件列表时使用 API 返回的 display_name（已按优先级解析）

- [ ] Task 4: ENCV v4 容器 — 文件名加密写入与读取
  - [ ] 4.1 v4 容器创建流程：当启用 FlagFilenameEncrypted 时，加密原始文件名存入 Manifest.original_name
  - [ ] 4.2 v4 容器打开流程：从 Manifest 读取 original_name，若 flag 已设置则解密
  - [ ] 4.3 解密失败时的 fallback 策略（显示物理文件名或占位符）

- [ ] Task 5: ENCV v4 容器 — 原始文件名修改 API
  - [ ] 5.1 实现 `PATCH /api/v1/file/rename` 端点，接收 { path, new_name, password? }
  - [ ] 5.2 后端更新 Manifest.original_name（若加密则先加密再存储）
  - [ ] 5.3 重写 Manifest 回磁盘并刷新内存缓存
  - [ ] 5.4 前端调用 rename API 后自动刷新文件列表

- [ ] Task 6: 边界情况处理
  - [ ] 6.1 超长文件名（>255 字节）：Manifest 完整存储 + 物理文件名自动缩短为安全长度
  - [ ] 6.2 空/空白文件名：Manifest 存储 + 展示层回退到物理文件名或 "(unnamed)"
  - [ ] 6.3 任意 Unicode 字符（emoji/CJK/RTL/控制字符）：UTF-8 全量存储，不做过滤
  - [ ] 6.4 加密操作作用于 UTF-8 字节序列，不依赖字符语义

- [ ] Task 7: 验证与测试
  - [ ] 7.1 Go 单元测试：alist-encrypt 文件名加解密往返一致性（含 CRC6 边界）
  - [ ] 7.2 Go 单元测试：v4 Manifest original_name 序列化/反序列化
  - [ ] 7.3 Go 单元测试：文件名解析优先级（有/无 original_name、有/无加密标志、解密失败 fallback）
  - [ ] 7.4 Go 单元测试：边界情况（超长 1000 字节、空字符串、纯 emoji、含 null byte）
  - [ ] 7.5 Go 单元测试：rename API 更新 Manifest 并持久化
  - [ ] 7.6 E2E 测试：创建带原始文件名的 v4 容器 → 物理重命名为乱码 → 列表仍显示原名 → 修改原名 → 列表立即反映
  - [ ] 7.7 vue-tsc + vitest + vite build 全量验证

# Task Dependencies
- [Task 1] 与 [Task 2] 可并行（两个独立功能）
- [Task 3] depends on [Task 2]
- [Task 4] depends on [Task 2, Task 3]
- [Task 5] depends on [Task 4]
- [Task 6] depends on [Task 4]
- [Task 7] depends on [Task 1, Task 4, Task 5, Task 6]
