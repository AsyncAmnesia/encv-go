# Checklist

## Phase 1: 插件模块骨架

- [ ] `plugin-alist-decrypt/build.gradle.kts` 存在且配置正确（android-library + aar2apk + compileOnly combolite.core）
- [ ] `AndroidManifest.xml` 声明 AlistDecryptActivity（exported=false）和 AlistDecryptService
- [ ] `AlistDecryptPluginEntry.kt` 实现 IPluginEntryClass 接口（onLoad/onUnload/pluginModule）
- [ ] Cipher 接口定义完整（5 个方法：setPosition/encrypt/decrypt/algorithm/blockSize）
- [ ] CipherRegistry 单例 init() 仅注册 aesctr
- [ ] `grep -r "rc4\|RC4\|chacha\|ChaCha" plugin-alist-decrypt/` **无匹配结果**（隔离验证）

## Phase 2: 核心算法实现

### AES-128-CTR
- [ ] AesCtrCipher 密钥派生链与 alist-encrypt-go 参考实现输出一致
- [ ] incrementIV 128-bit counter 进位与 Node.js aesCTR.js 完全一致（含大数溢出场景）
- [ ] setPosition(0) / setPosition(mid) / setPosition(nearEnd) 后数据解密均正确
- [ ] AesCtrCipher 满足 Cipher 接口编译通过并在 Registry 中注册为 "aesctr"

### MixBase64 文件名
- [ ] KSA shuffle 输出与参考实现一致（相同 password → 相同 64 字符字母表）
- [ ] Encode/Decode 往返无损（UTF-8 中文、特殊字符、长文件名）
- [ ] CRC6 校验位计算正确
- [ ] EncodeName → DecodeName 往返得到原始文件名

### V2 内容头
- [ ] AECTR2 magic 正确检测，NonceField 和 PlainSize 正确提取
- [ ] AutoDetectV2 在 V1 裸流和 V2 带头格式下均能正确分支

### DecryptInputStream
- [ ] 作为 InputStream 包装器可完整读取和解密文件
- [ ] skip()/seek 到中间位置后继续读取数据正确

## Phase 3: 插件功能层

### AlistDecryptActivity
- [ ] 从 Intent extras 正确读取 action + filePath + password
- [ ] decrypt action 启动 Service 并显示进度
- [ ] encrypt action 启动 Service 并显示进度
- [ ] stream action 启动 LocalStreamServer 并通过 setResult 返回 URL
- [ ] decode-filename action 同步返回解码后文件名
- [ ] 遵循 EncvHostActivity 四层超时防御机制（L1 半透明主题 / L2 5s 超时 finish / L3 onResume 诊断 / L4 Promise 兜底）

### AlistDecryptService
- [ ] 解密流程端到端正确：加密文件 → AutoDetectV2 → AesCtrCipher → 明文输出
- [ ] 加密流程端到端正确：原始文件 → AesCtrCipher → 加密输出+suffix
- [ ] 进度通过 LocalBroadcast 正确报告给 Activity
- [ ] 密码错误时返回特殊错误码（非通用异常）

### LocalStreamServer
- [ ] GET /stream 返回解密后的视频数据
- [ ] HTTP Range 请求返回 206 Partial Content
- [ ] Content-Type 根据扩展名正确推断
- [ ] Server 在 Activity onDestroy 时正确停止

## Phase 4: 宿主端集成

### GoProcessPlugin
- [ ] decryptAlistFile / encryptAlistFile / streamAlistFile / decodeAlistFilename 四个 @PluginMethod 存在
- [ ] 未安装插件时返回友好错误提示（非 crash）
- [ ] 插件已安装时正确构建代理 Intent 并 startActivityForResult

### PlayerEntry
- [ ] buildAlistDecryptIntent() 方法存在且逻辑正确
- [ ] onActivityResult 正确处理 REQUEST_CODE_ALIST_DECRYPT 结果回调

### 前端 API (encv.ts)
- [ ] 四个新函数可正确调用并解析响应

### Files.vue
- [ ] 匹配 suffix 的文件显示加密标记
- [ ] 真实文件名通过 decodeAlistFilename 显示
- [ ] 长按菜单出现「解密」和「流式预览」（插件已安装时）
- [ ] 流式预览 URL 可被播放器加载

### ExtensionsPage.vue
- [ ] alist-decrypt 扩展卡片显示正确（name/description/installed/enabled/sizeDisplay）
- [ ] 安装/启用/禁用/卸载操作正常工作
- [ ] COMBO_LITE_ID_MAP 包含 'alist-decrypt' → 'com.encvgo.plugin.alistdecrypt'

### Tasks.vue + i18n
- [ ] alist-decrypt/alist-encrypt 任务状态正确展示
- [ ] 错误信息包含密码错误特殊提示
- [ ] 中英文翻译 key 均已定义

## Phase 5: CI 构建

- [ ] settings.gradle.kts 包含 `:plugin-alist-decrypt` 模块引用
- [ ] CI workflow 中 plugin-alist-decrypt 的 aar2apk 构建步骤存在
- [ ] 构建产物为有效的 .apk 文件
- [ ] 插件 APK 可通过 EncvComboLiteHost.installPlugin() 成功安装
- [ ] 安装后 EncvComboLiteHost.isPluginAvailable("com.encvgo.plugin.alistdecrypt") 返回 true
