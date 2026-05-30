# Checklist

## Phase 1: Go 后端算法基础设施

### 算法隔离骨架
- [ ] Cipher 接口定义完整（5 个方法：SetPosition/Encrypt/Decrypt/Algorithm/BlockSize）
- [ ] CipherRegistry 使用 RWMutex 保护并发安全
- [ ] Registry init() **仅注册 aesctr**
- [ ] 查询 rc4md5/chacha20 时返回 ErrExtensionRequired（非 panic、非 fallback）
- [ ] `go vet ./internal/alistencrypt/...` 无 RC4/ChaCha20 import
- [ ] `grep -r "rc4\|RC4\|chacha\|ChaCha" internal/alistencrypt/` **无匹配**

### AES-128-CTR（Go 实现）
- [ ] 密钥派生链与 alist-encrypt-go 参考实现逐字节一致（PBKDF2→hex→MD5 key + MD5 iv）
- [ ] incrementIV 128-bit counter 进位与 Node.js aesCTR.js 完全一致（含大数溢出场景）
- [ ] SetPosition(0) / SetPosition(mid) / SetPosition(nearEnd) 后数据解密均正确
- [ ] 满足 Cipher 接口并在 Registry 中注册为 "aesctr"

### MixBase64 文件名（Go 实现）
- [ ] KSA shuffle 输出与参考实现一致（相同 password → 相同 64 字符字母表）
- [ ] Encode/Decode 往返无损（UTF-8 中文、特殊字符、长文件名）
- [ ] CRC6 校验位计算正确
- [ ] EncodeName → DecodeName 往返得到原始文件名

### V2 内容头 + DecryptReader（Go 实现）
- [ ] AECTR2 magic 正确检测，NonceField 和 PlainSize 正确提取
- [ ] AutoDetectV2 在 V1 裸流和 V2 带头格式下均能正确分支
- [ ] DecryptReader 作为 io.Reader 包装器可完整流式读取和解密
- [ ] DecryptReader 支持从中间位置 seek 后继续读取

## Phase 2: Go 后端 API 与服务

### 配置
- [ ] Config.AlistEncrypt 段可正确加载（enabled/suffix/enc_type/default_password）
- [ ] enc_type = "rc4md5" 时配置加载产生警告日志
- [ ] config.schema.json 包含 alist_encrypt 字段定义

### API Endpoint
- [ ] POST /api/alist-encrypt/decrypt 返回任务 ID，异步执行解密
- [ ] POST /api/alist-encrypt/encrypt 返回任务 ID，异步执行加密并附加后缀名
- [ ] GET /api/alist-encrypt/stream 支持 Range 请求，返回 206 Partial Content
- [ ] GET /api/alist-encode/decode-filename 返回解码后的真实文件名

### Service 层
- [ ] 解密流程端到端：加密文件 → AutoDetectV2 → AesCtrCipher → 明文输出
- [ ] 加密流程端到端：原始文件 → AesCtrCipher → 加密输出+suffix
- [ ] stream 端到端：Range 请求 → DecryptReader → 正确解密数据段
- [ ] decode-filename 端到端：encodedName + password → plainName

### TaskManager 集成
- [ ] alist-decrypt / alist-encrypt 任务状态流转正确（queued→running→completed/failed）
- [ ] WebSocket 推送包含进度百分比、速度、ETA
- [ ] 密码错误时返回特殊错误码（区别于数据损坏等错误）

## Phase 3: ComboLite 插件（业务编排层）

### 模块骨架
- [ ] build.gradle.kts 配置正确（android-library + aar2apk + compileOnly combolite.core）
- [ ] AndroidManifest.xml 声明 AlistDecryptActivity（exported=false）
- [ ] AlistDecryptPluginEntry.kt 实现 IPluginEntryClass

### Activity（纯 UI 调度，不含算法代码）
- [ ] 从 Intent extras 正确读取 action + filePath + password
- [ ] decrypt action 调用 Go 后端 API 并展示进度
- [ ] encrypt action 调用 Go 后端 API 并展示进度
- [ ] stream action 返回 Go 后端 stream URL 给宿主
- [ ] decode-filename action 调用 Go 后端 API 并返回结果
- [ ] 遵循 EncvHostActivity 四层超时防御规范
- [ ] `grep -r "AES\|cipher\|PBKDF2\|MD5" plugin-alist-decrypt/` **无匹配**（确认不含算法代码）

## Phase 4: 宿主端集成

### GoProcessPlugin
- [ ] decryptAlistFile / encryptAlistFile / streamAlistFile / decodeAlistFilename 存在
- [ ] 未安装后端服务时返回友好提示

### 前端 API (encv.ts)
- [ ] 四个新函数可正确调用并解析响应

### Files.vue
- [ ] 匹配 suffix 的文件显示加密标记和真实文件名
- [ ] 长按菜单出现「解密」和「流式预览」
- [ ] 流式预览 URL 可被播放器加载

### ExtensionsPage.vue
- [ ] alist-decrypt 扩展卡片显示正确
- [ ] 安装/启用/禁用/卸载操作正常工作

### Tasks.vue + i18n
- [ ] alist-decrypt/alist-encrypt 任务状态正确展示
- [ ] 错误信息包含密码错误特殊提示
- [ ] 中英文翻译 key 均已定义

## Phase 5: CI 构建
- [ ] settings.gradle.kts 包含 :plugin-alist-decret
- [ ] CI 中 plugin-alist-decrypt 的 aar2apk 构建步骤存在
- [ ] CI 中 `go test ./internal/alistencrypt/...` 步骤存在且通过

## 隔离性验证（CI 必须通过）
- [ ] `go build ./internal/alistencrypt/` 编译成功，产物中 **不包含** RC4/ChaCha20 符号
- [ ] `go test ./internal/alistencrypt/...` 全部通过（含隔离边界测试）
- [ ] ComboLite 插件中 **不 import 任何算法包**
