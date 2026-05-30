# Checklist

## Phase 1: 核心算法层

- [ ] AesCtrCipher 密钥派生链与 alist-encrypt-go 参考实现输出一致（PBKDF2 → hex → MD5 key + MD5 iv）
- [ ] AesCtrCipher.SetPosition() seek 后数据解密正确（使用测试向量验证偏移 0 / 中间位置 / 接近末尾）
- [ ] incrementIV 128-bit counter 进位与 Node.js aesCTR.js 完全一致（包括大数溢出场景）
- [ ] MixBase64 KSA shuffle 输出与参考实现一致（相同 password → 相同 64 字符字母表）
- [ ] MixBase64 Encode/Decode 往返编解码无损（UTF-8 中文文件名、特殊字符、长文件名）
- [ ] CRC6 校验位计算正确（encoded + passwdOutward → 6-bit CRC → sourceChars 映射）
- [ ] EncodeName → DecodeName 往返得到原始文件名（含中文、扩展名、空格等边界情况）
- [ ] V2 内容头 magic `AECTR2` 正确检测，NonceField 和 PlainSize 正确提取
- [ ] AutoDetectV2 在 V1（裸流）和 V2（带头）格式下均能正确选择解密路径
- [ ] DecryptReader 作为 io.Reader 包装器可正确流式读取和解密完整文件

## Phase 2: 配置与 API 层

- [ ] Config.AlistEncrypt 段可从 config.user.json 正确加载（enabled/suffix/enc_type/default_password）
- [ ] config.schema.json 包含 alist_encrypt 字段定义且前端可正确渲染配置 UI
- [ ] POST /api/alist-encrypt/decrypt 返回任务 ID，异步执行解密
- [ ] POST /api/alist-encrypt/encrypt 返回任务 ID，异步执行加密并附加后缀名
- [ ] GET /api/alist-encrypt/stream 支持 Range 请求，返回 206 Partial Content 和正确解密的数据
- [ ] GET /api/alist-encode/decode-filename 返回解码后的真实文件名
- [ ] TaskManager 正确处理 alist-decrypt / alist-encrypt 任务类型的状态流转
- [ ] WebSocket 推送包含进度百分比、速度、ETA 等信息
- [ ] 解密任务密码错误时返回明确的错误码（非 panic / 非通用错误）

## Phase 3: 移动端集成

- [ ] encv.ts API 函数可正确调用后端 endpoint 并解析响应
- [ ] Files.vue 中匹配 suffix 的文件显示加密标记和真实文件名
- [ ] 长按菜单出现「解密」和「流式预览」选项
- [ ] 流式预览 URL 可被 MPV/ArtPlayer 正确加载和播放
- [ ] Tasks.vue 中 alist-decrypt / alist-encrypt 任务正确展示状态、进度、错误信息
- [ ] 密码错误时前端显示特殊提示（区别于数据损坏等错误）

## 跨平台兼容性

- [ ] 算法实现在 Linux (CI) 和 Android (移动端) 上运行结果一致
- [ ] PBKDF2 使用标准库实现（Go: golang.org/x/crypto/pbkdf2），跨平台确定性输出
- [ ] MD5 使用 crypto/md5 标准库，跨平台一致性保证
