# Alist-Encrypt 兼容层 Spec（ComboLite 扩展）

## Why

encv-go 移动端已有 **MPV 播放器** 作为 ComboLite 插件（`plugin-mpv-player`），通过 aar2apk 构建为独立 APK、由用户安装、通过 ProxyManager 代理启动 Activity。现在需要以同样的架构新增一个 **alist-encrypt 解密扩展插件**，使用户能够：
- 解密 alist-encrypt 格式（AES-128-CTR）的加密文件
- 加密文件为 alist-encrypt 格式
- 流式预览加密视频（支持 seek，传给 MPV 播放）
- 在线解码显示真实文件名

## What Changes — 新增 ComboLite 插件 `plugin-alist-decrypt`

### 新增模块结构

```
app/encv-mobile/plugin-alist-decrypt/          ← 新增：Android Library 模块（与 plugin-mpv-player 同级）
├── build.gradle.kts                           ← id("com.android.library") + aar2apk
├── src/main/
│   ├── AndroidManifest.xml                    ← 声明 AlistDecryptActivity / AlistDecryptService
│   └── java/com/encvgo/plugin/alistdecrypt/
│       ├── AlistDecryptPluginEntry.kt         ← IPluginEntryClass 入口（ComboLite 加载点）
│       ├── cipher/
│       │   ├── Cipher.kt                     ← Cipher 接口（扩展点，算法隔离契约）
│       │   ├── AesCtrCipher.kt               ← AES-128-CTR 唯一内置实现
│       │   └── CounterIncrement.kt           ← 128-bit CTR counter 进位算法
│       ├── filename/
│       │   ├── MixBase64.kt                  ← KSA shuffle + Base64 编解码
│       │   └── CRC6.kt                       ← 6-bit CRC 校验
│       ├── content/
│       │   └── ContentHeader.kt              ← V2 头检测与解析（AECTR2 magic）
│       ├── stream/
│       │   └── DecryptInputStream.kt         ← InputStream 包装器（seek 支持）
│       ├── service/
│       │   └── AlistDecryptService.kt        ← 后台解密/加密任务 Service
│       ├── proxy/
│       │   └── LocalStreamServer.kt          ← 本地 HTTP 代理（解密流→MPV）
│       └── ui/
│           └── AlistDecryptActivity.kt       ← 插件 Activity（ProxyManager 代理启动）
```

### 已有代码修改清单

| 文件 | 改动 |
|------|------|
| `ExtensionsPage.vue` | 新增 `alist-decrypt` 扩展卡片（id/name/description/size） |
| `GoProcessPlugin.kt` | 新增 `decryptAlistFile` / `encryptAlistFile` / `streamAlistFile` / `decodeAlistFilename` 方法 |
| `PlayerEntry.kt` | 新增 `buildAlistDecryptIntent()` 路由方法 |
| `encv.ts`（前端 API） | 新增对应的 Capacitor 调用函数 |
| `Files.vue` | 识别 suffix 匹配文件 → 显示解密/预览操作入口 |
| `Tasks.vue` | 展示 alist-decrypt/alist-encrypt 任务状态 |
| `settings.ts`（i18n） | 新增 alist-encrypt 相关翻译 key |

### 不在 MVP 范围内（TODO）

- **OpenList 代理集成**：将解密能力接入 `internal/openlist/` 代理链
- **内部容器集成**：注册到 ENCV v2 plugins.Registry
- **桌面端 UI**：openlist 桌面客户端适配
- **RC4MD5 / ChaCha20 扩展包**：必须通过 Cipher 接口 + Register() 引入，禁止进入主包

---

## ADDED Requirements

### Requirement: 算法隔离架构（铁律）

主应用（plugin-alist-decrypt）**仅包含 AES-128-CTR 实现**，其他算法必须通过 Cipher 接口隔离。

```kotlin
// Cipher 接口 — 所有密码器的统一抽象
interface Cipher {
    fun setPosition(position: Long)
    fun encrypt(data: ByteArray)
    fun decrypt(data: ByteArray)
    fun algorithm(): String          // "AES-128-CTR" / "RC4-MD5" / "ChaCha20"
    fun blockSize(): Int
}

// Registry — 全局单例，仅注册内置算法
object CipherRegistry {
    private val factories = mutableMapOf<String, (password: String, fileSize: Long) -> Cipher>()

    init { register("aesctr", ::AesCtrCipher) }  // ← 仅此一个！

    fun register(encType: String, factory: ...)   // 扩展点
    fun create(password: String, encType: String, fileSize: Long): Cipher
}
```

**隔离规则**：
1. `plugin-alist-decrypt` 包内 **禁止出现 RC4 / ChaCha20 实现代码**
2. `CipherRegistry.init()` 中 **仅注册 aesctr**
3. enc_type 非 aesctr 时返回 `ErrExtensionRequired`
4. MixBase64 / CRC6 / ContentHeader 是所有算法共用的基础设施，放在主包中

### Requirement: AES-128-CTR 核心算法（唯一内置实现）

#### 密钥派生链（必须逐字节兼容 alist-encrypt-go）

```
输入: password (String), fileSize (Long)

Step 1: passwdOutward
  ├─ len == 32 → 直接用（已是 hex）
  └─ else → PBKDF2(pwd="AES-CTR", salt=password, iter=1000, dkLen=16, HmacSHA256)
         → hexEncode(key) → 32字符 hex 字符串

Step 2: Key (16 bytes) = MD5(passwdOutward + fileSize.toString())[:16]
Step 3: IV  (16 bytes) = MD5(fileSize.toString())[:16]

输出: AES/CTR/NoPadding(Key, IV)
```

#### Seek 支持（视频播放必需）

```
setPosition(position):
  1. iv = copy(originalIv)
  2. blockCount = position / 16
  3. incrementIV(blockCount)    // 128-bit 大端分段进位
  4. cipher.init(Cipher.DECRYPT_MODE, key, IvParameterSpec(iv))
  5. discard offset = position % 16 bytes
```

`incrementIV`: 将 IV 视为 4 个 uint32 大端整数，从最低段开始进位。

#### Scenario: 视频文件 seek 播放
- **WHEN** 用户在播放器拖动进度条到 position=1048576
- **THEN** setPosition 正确重建 CTR 流，后续读取在该偏移处正确解密

### Requirement: 文件名 MixBase64 加解密

完整移植 alist-encrypt-go 的 `filename.go`：

- **KSA shuffle**: passwdOutward → 64 字符自定义字母表（Fisher-Yates 变体）
- **MixBase64 Encode/Decode**: 基于自定义 alphabet 的 Base64 变种
- **CRC6 校验**: 多项式 x^6+x+1, 反射输入输出, 6-bit 结果映射到 sourceChars
- **EncodeName**: plainName → KSA→Encode→CRC6→encoded+crcChar
- **DecodeName**: encodedName → stripCRC6→Verify→Decode→plainName（验证失败返回空串）

#### Scenario: 文件名往返
- **WHEN** EncodeName("测试视频.mp4", "mypassword") 后 DecodeName 回来
- **THEN** 得到 "测试视频.mp4"

### Requirement: V2 内容头自动检测

| Offset | Len | Content |
|--------|-----|---------|
| 0 | 6 | Magic `"AECTR2"` |
| 6 | 1 | Version (`0x02`) |
| 7 | 1 | Reserved |
| 8 | 16 | NonceField |
| 24 | 8 | PlainSize (BE uint64) |

- Magic 匹配 → V2 模式（跳过 32 字节头，用 NonceField 初始化）
- Magic 不匹配 → V1/Legacy 模式（裸流，fileSize 即 plainSize）

### Requirement: 可配置后缀名

插件接收宿主传入的配置（通过 Intent extras 或共享 SharedPreferences）：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `suffix` | `.sccgv` | 加密文件识别后缀 |
| `defaultPassword` | `""` | 默认密码（空=每次输入） |
| `encType` | `aesctr` | 加密算法（MVP 仅支持此值） |

enc_type 非 aesctr 时：配置加载警告 + 运行时返回 `ErrExtensionRequired`

### Requirement: 解密/加密任务（后台 Service）

通过 `AlistDecryptService`（IntentService）执行异步任务：

- **解密**: 读取加密文件 → DetectV2 → AesCtrCipher.DecryptReader → 写入目标路径
- **加密**: 读取原始文件 → AesCtrCipher.EncryptWriter → 写入目标路径+suffix（可选 V2 头）
- **进度报告**: 已处理字节/总字节 → 通过 LocalBroadcast / ResultReceiver 回调宿主
- **状态**: queued → running → completed / failed（复用宿主 TaskManager 展示模式）

### Requirement: 流式预览（LocalStreamServer）

解密后的流通过本地 HTTP server 提供给 MPV 播放：

```
[加密文件] → DecryptInputStream(支持 Range seek)
  → LocalHttpServer(localhost:随机端口, 单连接)
    → MPV 播放 localhost URL
```

- **Endpoint**: `GET /stream?path=<uri>&password=<pwd>`
- **HTTP Range**: 完整支持（206 Partial Content）
- **Content-Type**: 根据原始扩展名推断
- **生命周期**: 随 Activity 销毁而停止

#### Scenario: MPV 播放加密视频
- **WHEN** 用户选择「流式预览」加密视频
- **THEN** 启动 LocalStreamServer → 返回 localhost URL → MPV 加载并正常播放/seek

### Requirement: 宿主集成路径（GoProcessPlugin → 插件）

前端调用链：

```
Files.vue (长按→解密/预览)
  → encv.ts: decryptAlistFile({path, password, mode})
    → GoProcessPlugin.decryptAlistFile(call)
      → EncvComboLiteHost.isPluginAvailable("com.encvgo.plugin.alistdecrypt")
      → EncvComboLiteHost.createProxyIntent(context,
            "com.encvgo.plugin.alistdecrypt",
            "com.encvgo.plugin.alistdecrypt.ui.AlistDecryptActivity",
            EncvHostActivity::class.java,
            mapOf("action" to "decrypt", "filePath" to path, "password" to password))
      → startActivityForResult(intent, REQUEST_CODE_ALIST_DECRYPT)
        → EncvHostActivity → ProxyManager → AlistDecryptActivity
          → 执行解密/加密/流式预览 → setResult → finish
            → onActivityResult → call.resolve(result)
```

### Requirement: 扩展管理页集成

ExtensionsPage.vue 新增扩展卡片：

```typescript
{
  id: 'alist-decrypt',
  name: 'Alist-Encrypt 解密',
  description: '支持 AES-128-CTR 加密文件的解密、加密和流式预览',
  installed: boolean,     // checkInstalledPlugins["com.encvgo.plugin.alistdecrypt"]
  enabled: boolean,
  sizeDisplay: '~200 KB', // 纯 Kotlin，无 native 库
}
```

COMBO_LITE_ID 映射: `'alist-decrypt' → 'com.encvgo.plugin.alistdecrypt'`

---

## MODIFIED Requirements

无（纯新增功能）。

## REMOVED Requirements

无。
