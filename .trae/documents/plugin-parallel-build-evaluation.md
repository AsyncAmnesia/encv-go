# 单 Job 插件构建结构规范化

## 目标

将当前硬编码的 MPV 插件构建步骤参数化为**可复用模板**，使新增插件只需复制模板 + 改变量名。

## 当前问题

[android.yml](file:///workspace/.github/workflows/android.yml) 中插件相关步骤存在以下硬编码：

| 硬编码项 | 出现位置 | 值 |
|---------|---------|-----|
| 模块名 | `:plugin-mpv-player` | L261, L270 |
| 插件目录 | `plugin-mpv-player/src/main/jniLibs/...` | L124, L254 |
| APK 文件名 | `plugin-mpv-player-debug.apk`, `plugin-mpv-player-release.apk` | L250, L269, L272, L277 |
| assets 中的名称 | `mpv-player.apk` | L255, L277, L341, L376 |
| Release asset 名 | `mpv-jniLibs-arm64` | L127 |
| 验证中的 .so 列表 | `libmpv.so libplayer.so ...` | L206 |

新增第 2 个插件时需要在这些 **6+ 处** 同步修改，容易遗漏。

## 实施步骤

### 步骤 1: 定义插件配置变量

在 workflow 的 `env:` 或步骤开头统一定义所有插件参数：

```yaml
env:
  # ... existing env vars ...
  # Plugin registry (add new plugins here)
  PLUGIN_MPV_NAME: "mpv-player"
  PLUGIN_MPV_MODULE: "plugin-mpv-player"
  PLUGIN_MPV_ASSET: "mpv-player.apk"
```

### 步骤 2: 重构 "Download MPV native libraries" 为通用化

将：
```yaml
- name: Download MPV native libraries from Release
  if: inputs.skip_plugin != true
  run: |
    cd app/encv-mobile/plugin-mpv-player/src/main
    mkdir -p jniLibs/arm64-v8a
    ASSET_URL=$(curl -sf \
      "https://api.github.com/repos/${GITHUB_REPOSITORY}/releases/tags/mpv-native-libs" \
      | jq -r '.assets[] | select(.name | startswith("mpv-jniLibs-arm64")) | .browser_download_url')
    ...
```

改为使用变量的通用下载逻辑（Release tag 和 asset prefix 也从变量派生）：

```yaml
- name: Download plugin native libraries from Release
  if: inputs.skip_plugin != true
  env:
    PLUGIN_MODULE: ${{ env.PLUGIN_MPV_MODULE }}
    PLUGIN_LIBS_RELEASE: "mpv-native-libs"
    PLUGIN_LIBS_PREFIX: "mpv-jniLibs"
  run: |
    cd "app/encv-mobile/${PLUGIN_MODULE}/src/main"
    mkdir -p jniLibs/arm64-v8a
    ASSET_URL=$(curl -sf \
      "https://api.github.com/repos/${GITHUB_REPOSITORY}/releases/tags/${PLUGIN_LIBS_RELEASE}" \
      | jq -r --arg PREFIX "${PLUGIN_LIBS_PREFIX}" '.assets[] | select(.name | startswith($PREFIX)) | .browser_download_url')
    ...
```

### 步骤 3: 重构插件编译+打包为参数化块

合并 "Build MPV player plugin" + "Package MPV plugin as APK" 为一个统一的"构建并打包插件"步骤，使用 env 变量：

```yaml
- name: Build & package plugin (mpv-player)
  if: inputs.skip_plugin != true
  env:
    PLUGIN_MODULE: ${{ env.PLUGIN_MPV_MODULE }}
    PLUGIN_NAME: ${{ env.PLUGIN_MPV_NAME }}
    PLUGIN_ASSET: ${{ env.PLUGIN_MPV_ASSET }}
  run: |
    cd app/encv-mobile/android
    BUILD_TYPE="${{ inputs.version && 'release' || 'debug' }}"

    # ndk-build (if needed)
    if [ ! -f "${PLUGIN_MODULE}/src/main/jniLibs/arm64-v8a/libplayer.so" ]; then
      bash ../scripts/build-player-so.sh || echo "⚠️ libplayer.so build failed"
    fi

    # Gradle compile
    ./gradlew ":${PLUGIN_MODULE}:compile${BUILD_TYPE}Kotlin" --stacktrace 2>&1 || true

    # aar2apk convert
    ./gradlew "convert_${PLUGIN_MODULE}_${BUILD_TYPE}" --stacktrace

    # Copy to host assets
    PLUGIN_APK=$(find build -name "${PLUGIN_MODULE}-${BUILD_TYPE}.apk" -type f 2>/dev/null | head -1)
    if [ -n "$PLUGIN_APK" ] && [ -f "$PLUGIN_APK" ]; then
      mkdir -p app/src/main/assets/plugins
      cp "$PLUGIN_APK" "app/src/main/assets/plugins/${PLUGIN_ASSET}"
      echo "✅ ${PLUGIN_ASSET} ready"
    else
      echo "::error::Plugin APK not found"
      exit 1
    fi
```

### 步骤 4: 更新验证步骤

验证步骤中插件检查也使用变量：

```bash
echo "=== Plugin APK in assets ==="
if [ "${{ inputs.skip_plugin }}" = "true" ]; then
  echo "⏭️ Skipped (skip_plugin=true)"
elif unzip -l "$APK_PATH" | grep -q "${{ env.PLUGIN_MPV_ASSET }}"; then
  echo "✅ PASS: Plugin APK found"
else
  echo "::warning::Plugin APK not found"
fi
```

### 步骤 5: Verify native libraries 步骤同步更新

MPV jniLibs 检查部分也改用变量。

## 新增插件的模板（未来）

当需要添加第二个插件时，只需：

1. 在 `env:` 中添加一组新变量：
   ```yaml
   PLUGIN_XXX_NAME: "xxx-plugin"
   PLUGIN_XXX_MODULE: "plugin-xxx"
   PLUGIN_XXX_ASSET: "xxx-plugin.apk"
   ```

2. 复制 "Download plugin native libraries" 步骤，替换环境变量引用

3. 复制 "Build & package plugin" 步骤，替换环境变量引用

4. 在 aar2apk 配置中添加 `module(":plugin-xxx")`

5. 如果有独立的 native libs Release，在 `build-mpv-lib.yml` 或新建对应 workflow

## 修改文件

| 文件 | 改动 |
|------|------|
| `.github/workflows/android.yml` | 参数化所有插件步骤；env 中定义插件注册表 |
| `.github/workflows/build-mpv-lib.yml` | 无需改动（已是独立 workflow） |

## 不改动

- Gradle 配置 — aar2apk 的 `modules {}` 已是声明式，加一行即可
- 脚本文件 — `setup-mpv-libs.sh`, `build-player-so.sh` 与插件无关
