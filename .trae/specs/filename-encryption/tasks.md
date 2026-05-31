# Tasks

- [ ] Task 1: 对齐 alist-encrypt 插件文件名加密与 alist-encrypt-go
  - [ ] 1.1 克隆 alist-encrypt-go 到 /tmp/alist-encrypt-go，分析 EncryptedName 实现细节（编码格式、CRC6 多项式、AES-CTR 参数、mixBase64 字符表）
  - [ ] 1.2 对比 `internal/alistencrypt/filename.go` 与参考实现的差异，列出所有不一致点
  - [ ] 1.3 修复 filename.go 使其二进制兼容（EncryptedName 结构、mixBase64 编解码、CRC6 校验）
  - [ ] 1.4 同步 cipher.go / aesctr.go 确保密钥派生和 CTR 模式参数一致
  - [ ] 1.5 更新前端 `useAlistEncrypt.ts` 的 decodeAlistFilename 匹配后端格式

- [ ] Task 2: ENC-FN 核心算法实现 — 深度定制文件名编码器
  - [ ] 2.1 新建 `internal/v2/filename/charset.go`：定义 FNCharset 常量和字符集映射表（alnum/safe/hex/alpha + EncodeToCharset / DecodeFromCharset 函数）
  - [ ] 2.2 新建 `internal/v2/filename/kdf.go`：HKDF-SHA256 密钥派生，从 password → 主密钥 → S-box 种子(32B) + N 个轮密钥(每轮16B)
  - [ ] 2.3 新建 `internal/v2/filename/sbox.go`：从种子确定性生成 256 字节 S-box 置换表 + 逆 S-box（Fisher-Yates shuffle 用种子初始化的 PRNG）
  - [ ] 2.4 新建 `internal/v2/filename/feistel.go`：Feistel 网络 — 左右分块、多轮 F 函数（S-box 查表 + 轮密钥 XOR）、支持正向/逆向变换
  - [ ] 2.5 新建 `internal/v2/filename/encfn.go`：组装完整 Encode/Decode 流程（UTF-8→KDF→S-box→Feistel→Charset 映射），含 compact 和 structured 两种模式
  - [ ] 2.6 定义 ENC-FN 错误类型：ErrFNInvalidFormat, ErrFNChecksumMismatch, ErrFNCorrupt, ErrFNEmptyInput, ErrFNUnsupportedCharset

- [ ] Task 3: ENCV v4 容器数据结构扩展
  - [ ] 3.1 在 Manifest_v4 中新增 OriginalName 和 FilenameAlgorithm 字段
  - [ ] 3.2 定义 FlagFilenameEncrypted 常量 (0x0010)
  - [ ] 3.3 确保 WriteHeaderV4/ReadHeaderV4 正确处理新 flag 位

- [ ] Task 4: ENCV v4 容器 — 文件名解析优先级与展示层抽象
  - [ ] 4.1 后端文件列表 API：v4 容器返回时携带 original_name 元数据和 filename_alg 标识
  - [ ] 4.2 实现 ResolveDisplayName 函数：按优先级返回显示名（Manifest original_name 解码 > 物理文件名）
  - [ ] 4.3 Files.vue 渲染使用 API 返回的 display_name

- [ ] Task 5: ENCV v4 容器 — 文件名编码写入与读取集成
  - [ ] 5.1 v4 容器创建流程：当启用 FlagFilenameEncrypted 时，调用 ENC-FN.Encode 将原始文件名编码后存入 Manifest.original_name
  - [ ] 5.2 v4 容器打开流程：读取 Manifest.original_name，若 flag 已设置则 ENC-FN.Decode 还原
  - [ ] 5.3 解密失败 fallback：显示物理文件名或 "(encrypted-name)" 占位符

- [ ] Task 6: 原始文件名修改 API
  - [ ] 6.1 实现 `PATCH /api/v1/file/rename` 端点 { path, new_name, password? }
  - [ ] 6.2 后端更新 Manifest.original_name（ENC-FN.Encode 新值）+ 重写 Manifest 到磁盘
  - [ ] 6.3 前端调用 rename API 后自动刷新文件列表

- [ ] Task 7: 边界情况处理
  - [ ] 7.1 超长文件名（>255 字节）：Manifest 完整存储 ENC-FN 编码结果，物理文件名自动缩短为安全长度（SHA256 前 16 字节 hex 或截断）
  - [ ] 7.2 空/空白文件名：ENC-FN 接受空输入并返回空输出；展示层回退到物理文件名
  - [ ] 7.3 Unicode 全量测试：emoji 🎉、中文、阿拉伯文、RTL、null byte (\x00)、纯 ASCII、混合脚本
  - [ ] 7.4 ENC-FN 对所有输入基于 UTF-8 字节操作，不依赖 Unicode 语义

- [ ] Task 8: 验证与测试
  - [ ] 8.1 Go 单元测试：ENC-FN 往返一致性（Encode→Decode==原文），覆盖所有 Mode×Charset 组合（2×4=8 种）
  - [ ] 8.2 Go 单元测试：ENC-FN 密码敏感性（不同密码→完全不同输出）、确定性（相同输入→相同输出）
  - [ ] 8.3 Go 单元测试：ENC-FN 雪崩效应（改变明文 1 bit → 输出 ~50% 位变化）
  - [ ] 8.4 Go 单元测试：ENC-FN 错误处理（篡改检测、非法字符集、空输入、超长输入）
  - [ ] 8.5 Go 单元测试：alist-encrypt EncryptedName 往返一致性 + CRC6 边界
  - [ ] 8.6 Go 单元测试：v4 Manifest original_name 序列化/反序列化
  - [ ] 8.7 E2E 测试：创建带 ENC-FN 编码文件名的 v4 容器 → 物理重命名为乱码 → 列表仍显示原名 → rename 修改原名 → 列表立即反映
  - [ ] 8.8 vue-tsc + vitest + vite build 全量验证

# Task Dependencies
- [Task 1] 与 [Task 2] 可并行（两个独立功能）
- [Task 3] depends on 无（可并行）
- [Task 4] depends on [Task 2, Task 3]
- [Task 5] depends on [Task 2, Task 3, Task 4]
- [Task 6] depends on [Task 5]
- [Task 7] depends on [Task 2, Task 5]
- [Task 8] depends on [Task 1, Task 2, Task 5, Task 6, Task 7]
