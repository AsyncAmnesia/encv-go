# ENCV-Mobile CI/CD 构建踩坑记录

> 本文档记录在将 encv-mobile (Ionic Vue + Capacitor) 接入 GitHub Actions 自动构建 Android APK 过程中遇到的所有问题及解决方案。

---

## 1. APK 实机后端未连接

### 现象
APK 安装到真机后显示"后端未连接"，且 `lib/` 目录为空。

### 根因（3 个独立问题）

#### 1.1 AndroidManifest 缺少 `usesCleartextTraffic="true"`
- **原因**：Android 9+ 默认禁止明文 HTTP 流量，前端无法连接 `http://127.0.0.1:2025`
- **修复**：在 AndroidManifest.xml 的 `<application>` 标签添加：
  ```xml
  android:usesCleartextTraffic="true"
  android:networkSecurityConfig="@xml/network_security_config"
  ```
- **配套**：创建 `res/xml/network_security_config.xml` 允许 127.0.0.1/localhost 明文

#### 1.2 Android 10+ (API 29+) noexec 限制
- **原因**：从 Android 10 开始，`filesDir` 挂载为 noexec，无法直接执行二进制文件
- **修复**：MainActivity.kt 多位置回退策略：
  ```kotlin
  val candidateDirs = listOf(
      filesDir to "filesDir",
      cacheDir to "cacheDir",
      getExternalFilesDir(null) to "externalFilesDir",
  )
  for ((dir, name) in candidateDirs) {
      // 尝试每个目录，找到可执行的就用
  }
  ```

#### 1.3 "原生库是空的" 是正常现象
- Go 二进制打包在 `assets/encv-go`，不是 JNI 的 `lib/*.so`
- MainActivity.kt 在启动时从 assets 提取到 filesDir/cacheDir 后执行

---

## 2. heredoc 缩进导致 shell 语法错误

### 现象
```
warning: here-document at line 8 delimited by end-of-file (wanted 'EOF')
syntax error: unexpected end of file
```

### 原因
YAML 的 `run: |` 块中，heredoc 内容行和终止符都带有 YAML 缩进空格。bash 无法匹配终止符。

### 错误写法
```yaml
- run: |
    cat > file << 'EOF'
    content line with indent   # 这些前导空格保留到生成的脚本
    EOF                       # bash 找不到这个 EOF（有前导空格）
```

### 正确写法
```yaml
- run: |
    echo "line1" > file       # 用 echo 替代 heredoc
    echo "line2" >> file
```

---

## 3. jarsigner 签名失败 → 无效安装包

### 现象
```
jarsigner failed, attempting unsigned APK
```
安装时报"无效安装包"。

### 原因
- AGP 8.x 产出的 release APK 使用 **APK Signature Scheme v2/v3/v4**
- `jarsigner` 只支持 **v1 (JAR 签名)**，对 v2+ APK 操作必然失败

### 解决方案
放弃 jarsigner 后处理，改用 **Gradle 原生签名**：
```groovy
// build.gradle 中配置 signingConfigs + signingConfig
android {
    signingConfigs {
        release {
            storeFile file('../keystore/release.jks')
            storePassword 'xxx'
            keyAlias 'xxx'
            keyPassword 'xxx'
        }
    }
}
buildTypes {
    release {
        signingConfig signingConfigs.release   // Gradle 原生签名 v1+v2+v3
    }
}
```

---

## 4. 固定签名实现覆盖安装

### 需求
每次构建使用相同签名，否则 Android 不允许覆盖安装不同签名的 APK。

### 方案
- keystore 放入仓库的 `android/keystore/release.jks`
- CI 使用 `actions/cache@v4` 跨 run 复用同一文件
- 首次构建自动生成，后续从 cache 恢复
- artifact 只上传 APK，不上传 keystore

```yaml
- uses: actions/cache@v4
  with:
    path: app/encv-mobile/android/keystore
    key: encv-release-keystore
```

---

## 5. cap add android 重复添加错误

### 现象
```
android platform already exists.
To re-add this platform, first remove ./android
Error: Process completed with exit code 1
```

### 原因
仓库 checkout 后 `./android` 目录已存在（包含 git 提交的 keystore），但不是完整的 Capacitor 项目。

### 解决方案
判断依据改为检查核心文件是否存在：
```bash
if [ -f "$ANDROID_DIR/build.gradle" ] && [ -f "$ANDROID_DIR/variables.gradle" ]; then
    npx cap sync android          # 有效项目，同步即可
else
    rm -rf "$ANDROID_DIR"
    npx cap add android && npx cap sync android  # 无效项目，重建
fi
```

**注意**：删除前必须备份 keystore 到 `/tmp`，cap sync 后恢复。

---

## 6. shrinkResources 要求 minifyEnabled 先开启

### 现象
```
Removing unused resources requires unused code shrinking to be turned on.
```

### 原因
Gradle 要求 `shrinkResources true` 必须配合 `minifyEnabled true`。正则注入时如果顺序错乱或只注入了其中一个就会报错。

### 解决方案
确保注入顺序：
```groovy
release {
    minifyEnabled true        // 先开代码压缩
    shrinkResources true      // 再开资源压缩（依赖上一行）
    signingConfig ...         // 最后引用签名
}
```

---

## 7. 正则替换 build.gradle 的各种灾难

### 问题汇总

| 尝试 | 结果 | 原因 |
|------|------|------|
| 行内插入 `release {` 后追加内容 | minifyEnabled 丢失或重复 | `[^}]*?` 匹配行为不可预测 |
| 整体替换 `release {}` 块 | 匹配到 `signingConfigs.release` | 第一个 `release {}` 不是目标块 |
| 先注入 signingConfigs 再替换 release | 把 minifyEnabled 写进 SigningConfig 对象 | 同上，顺序反了 |
| 用 `buildTypes\s*\{...release\` 限定上下文 | 结构被破坏，花括号不匹配 | Capacitor 生成的格式变化 |

### 最终解决方案：完全不用正则改内部结构

使用 Gradle 的 **`apply from`** 机制：

**build.gradle（Capacitor 生成，我们只加 1 行）**
```groovy
plugins { id "com.android.application" }

apply from: '../encv-release.gradle'     // ← 唯一的修改

android { ... }                             // 完全不动
```

**encv-release.gradle（我们的文件，永远稳定）**
```groovy
android {
    signingConfigs {
        release { storeFile file('../keystore/release.jks'); ... }
    }
    ndk { abiFilters 'arm64-v8a' }
}

android.buildTypes.release {
    minifyEnabled true
    shrinkResources true
    signingConfig signingConfigs.release
}
```

**关键优势**：
- 幂等性：`c.includes('encv-release.gradle')` 检查防止重复注入
- 隔离性：所有复杂配置在外部文件，不触碰生成的内容
- 可调试：本地可直接编辑 `.gradle` 文件测试

---

## 8. Capacitor hooks 配置方式

### 错误
```typescript
// capacitor.config.ts — TypeScript 类型不支持 hooks 属性
hooks: {
  afterSync: async () => { ... }   // TS2353: 'hooks' does not exist in type 'CapacitorConfig'
}
```

### 正确（Capacitor 8 官方方式）
```json
// package.json scripts
{
  "scripts": {
    "capacitor:sync:after": "node scripts/post-cap-sync.mjs"
  }
}
```

源码验证：`sync.js` 中调用 `runHooks(config, platformName, rootDir, 'capacitor:sync:after')`，查找 `pkg.scripts['capacitor:sync:after']` 并执行。

---

## 9. 文件放置位置：overlay vs 生成目录

### 规则
**任何自定义文件都必须放在 `android-overlay/` 目录**，由 CI 步骤复制到 `android/`。

| 文件类型 | 存放位置 | 复制目标 |
|----------|---------|---------|
| Kotlin 源码 | `android-overlay/app/src/main/java/...` | `android/app/src/main/java/...` |
| XML 资源 | `android-overlay/app/src/main/res/xml/...` | `android/app/src/main/res/xml/...` |
| Gradle 配置 | `android-overlay/encv-release.gradle` | `android/encv-release.gradle` |
| Assets | 项目根目录 `assets/` | `android/app/src/main/assets/` |

**原因**：`npx cap sync android` 会重新生成整个 `android/` 目录，放在里面的任何自定义文件都会被清除。

---

## 10. Release 构建 APKSigner 强制校验

### 做法
构建后用 `apksigner verify --print-certs` 强制检验签名：
```bash
./gradlew assembleRelease
apksigner verify --print-certs "$APK_PATH" || {
    echo "::error::APK is NOT SIGNED!"
    exit 1          # 不再糊弄，直接让 CI 红掉
}
```

---

## 11. Release APK 全部优化措施汇总

| 优化项 | 效果 | 实现方式 |
|--------|------|---------|
| `-ldflags="-s -w"` | 去除调试符号/DWARF，二进制 ~30% | Go 编译参数 |
| `abiFilters 'arm64-v8a'` | 仅 ARM64，去掉 x86/armeabi-v7a ~30% | build.gradle ndk |
| `minifyEnabled true` | R8 代码混淆压缩 | encv-release.gradle |
| `shrinkResources true` | 移除未使用资源（语言包等） | encv-release.gradle |
| `zipalign` | 内存对齐优化 | Gradle 原生处理 |

---

## 12. 构建步骤正确顺序（最终版）

```
Checkout → Setup tools (Node/Go/JDK/Android SDK)
    ↓
npm install && npm run build                    # 前端构建
go build (CGO_ENABLED=0, arm64)               # Go 后端二进制
    ↓
Restore/Create keystore (cache 或新建)         # 签名文件就位
    ↓
npx cap sync android                           # 触发 afterSync hook
  → post-cap-sync.mjs:
      - root/build.gradle: +kotlin plugin
      - app/build.gradle: +kotlin-android, apply from encv-release.gradle
      - versionCode/versionName 注入
    ↓
cp assets (Go binary + config)                  # 打包后端
cp overlay files (MainActivity + manifest + encv-release.gradle)
    ↓
./gradlew assembleRelease                         # Gradle 原生签名 + zipalign
apksigner verify                                 # 强制签名校验
    ↓
upload artifact (仅 APK)                        # 不上传 keystore
```

**核心原则**：`cap sync` 之前准备 keystore，`cap sync` 之后复制 overlay 文件。
