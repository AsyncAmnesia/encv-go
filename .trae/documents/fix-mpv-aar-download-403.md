# 修复 CI 中 setup-mpv-libs.sh 下载 AAR 被 403 拒绝

## 问题

`setup-mpv-libs.sh` 从 Maven Central (`repo1.maven.org`) 下载 `mpv-android-lib` AAR 时返回 **403 Forbidden**。
GitHub Actions runner 共享 IP 段，频繁请求被 Maven Central 限流/封禁。

## 根因分析

当前 CI 对 FFmpeg 输出有缓存（`app/encv-mobile/android/app/src/main/jniLibs/arm64-v8a`），但 **MPV jniLibs 输出没有缓存**。每次 CI 运行都重新从 Maven Central 下载 AAR → 触发限流。

## 修改方案（3 层防御）

### 1. CI 层：缓存 MPV jniLibs 输出（主要修复）

**文件**: `.github/workflows/android.yml`

在现有 ffmpeg 缓存之后，添加 mpv jniLibs 缓存：

```yaml
      - name: Cache mpv native libraries (AAR extracted)
        id: cache-mpv-libs
        uses: actions/cache@v4
        with:
          path: app/encv-mobile/plugin-mpv-player/src/main/jniLibs
          key: mpv-libs-${{ hashFiles('app/encv-mobile/scripts/setup-mpv-libs.sh') }}
        restore-keys:
          mpv-libs-
```

key 基于 `setup-mpv-libs.sh` 脚本内容哈希——版本号变更时自动失效。

### 2. 脚本层：跳过已存在的下载（防御层）

**文件**: `app/encv-mobile/scripts/setup-mpv-libs.sh`

在 curl 之前检查目标文件是否已存在（缓存命中时跳过下载）：

```bash
# 如果 libmpv.so 已存在且非空，说明之前已成功提取过
if [ -f "$JNI_DIR/arm64-v8a/libmpv.so" ] && [ -s "$JNI_DIR/arm64-v8a/libmpv.so" ]; then
    echo "setup-mpv-libs: libmpv.so already exists, skipping download"
else
    curl -fSL --retry 3 --retry-delay 5 -A "Mozilla/5.0" -o "$AAR_TMP" "$AAR_URL"
    # ... 提取逻辑 ...
fi
```

curl 增加：
- `--retry 3 --retry-delay 5`: 失败重试 3 次，间隔 5 秒
- `-A "Mozilla/5.0"`: 设置 User-Agent 避免被识别为爬虫

### 3. 脚本层：libplayer.so 构建也跳过（一致性）

同样检查 `libplayer.so` 是否已存在，避免重复 ndk-build：

```bash
if [ ! -f "$JNI_DIR/arm64-v8a/libplayer.so" ]; then
    bash "$SCRIPT_DIR/build-player-so.sh"
else
    echo "setup-mpv-libs: libplayer.so already exists, skipping build"
fi
```

## 修改文件清单

| 文件 | 改动 |
|------|------|
| `.github/workflows/android.yml` | 在 ffmpeg 缓存后添加 mpv jniLibs 缓存步骤 |
| `app/encv-mobile/scripts/setup-mpv-libs.sh` | 添加存在性检查、curl 重试+UA、构建跳过逻辑 |

## 不改动的部分

- `build-player-so.sh` — 无需改动（它已有 prebuilt count 检查）
- `build-ffmpeg-android.sh` — 无关
- Gradle 配置 — 无关
