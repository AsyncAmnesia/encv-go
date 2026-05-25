# 修复 FFmpeg dlopen 符号缺失 + 确认前端变更生效

## 问题 1：`ff_graph_css_data` 符号缺失

### 错误信息
```
dlopen failed: cannot locate symbol "ff_graph_css_data" referenced by libffmpeg.so
```

### 根因分析
`ff_graph_css_data` 是 FFmpeg 8.0 `libavfilter` 中定义的常量数据（filter graph 的 CSS 复杂度/评分/相似度数据表）。

当前链接命令（[build-ffmpeg-android.sh:267-274](file:///workspace/app/encv-mobile/scripts/build-ffmpeg-android.sh#L267)）：
```
$CC $CFLAGS -shared -o libffmpeg.so \
    $FFMPEG_OBJS \
    $STATIC_LIBS \          ← 包含 libavfilter.a
    libx264.a \
    -lm -lz -llog \
    -Wl,--gc-sections \      ← 死代码消除
    -Wl,--allow-multiple-definition \
    $LDFLAGS
```

**问题**：`--gc-sections` 过于激进地删除了 `ff_graph_css_data` 所在的 section。这个符号不是被 fftools 代码**直接引用**的，而是通过 libavfilter 内部的函数指针表或初始化数组**间接引用**。链接器认为"没有直接引用 → 可以删除"，导致运行时 dlopen(RTLD_NOW) 找不到该符号。

### 修复方案
在链接命令中添加 `-Wl,--undefined=ff_graph_css_data`，强制链接器保留此符号：

```bash
-Wl,--undefined=ff_graph_css_data \
```

这比恢复 `--whole-archive` 更精准——只保留需要的符号，不会把所有 .a 的所有 object 都拉进来导致 .so 膨胀。

**注意**：FFmpeg 8.0 可能还有其他类似的间接引用符号。如果修复后出现新的 undefined symbol，需要用同样的方式处理，或者考虑对 libavfilter.a 使用 `--whole-archive`（仅这一个库）。

## 问题 2：前端 WebDAV + Tasks 报错显示未生效

### 可能原因分析
1. **APK 未重新构建**：用户手机上安装的是旧版 APK，不包含最新前端代码
2. **Capacitor sync 未执行**：`vite build` 输出到 `dist/`，但 Capacitor 未将最新的 `dist/` 同步到 Android assets
3. **开发服务器缓存**：如果用 dev server 模式运行，浏览器可能有缓存

### 验证步骤（用户侧）
1. 确认已执行完整构建流程：
   ```bash
   cd app/encv-mobile
   npx vite build          # → 输出到 dist/
   npx cap sync android    # → 同步 dist/ 到 android/app/src/main/assets/
   cd android && ./gradlew assembleDebug
   ```
2. 安装新生成的 APK 到设备
3. 清除应用数据后重新打开（避免 WebView 缓存）

### 代码侧确认（我们做）
确认我们的前端改动确实存在于源码中且能正确编译：
1. ✅ 已确认 WebDAV.vue 有内联结果区域（非 toast）
2. ✅ 已确认 Tasks.vue 有复制按钮 + error-detail-pre 展示区
3. ✅ 已确认 vue-tsc + vite build 通过

**结论**：代码改动是正确的，问题出在部署流程。需要在计划中提醒用户重新构建 APK。

## 实现步骤

### 步骤 1：修复 ff_graph_css_data 符号缺失
**文件**：`app/encv-mobile/scripts/build-ffmpeg-android.sh`

在 ffmpeg 和 ffprobe 的链接命令中，`-Wl,--gc-sections` 之后添加：
```bash
-Wl,--undefined=ff_graph_css_data \
```

具体位置：
- ffmpeg 链接命令：第 272 行 `-Wl,--gc-sections \` 之后
- ffprobe 链接命令：第 303 行 `-Wl,--gc-sections \` 之后

### 步骤 2：验证构建脚本语法
检查 shell 脚本无语法错误。

### 步骤 3：更新项目规则文档
**文件**：`.trae/rules/project_rules.md`

记录此符号问题及解决方案，作为后续类似问题的参考。

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `app/encv-mobile/scripts/build-ffmpeg-android.sh` | 修改 | 两处链接命令添加 `--undefined=ff_graph_css_data` |
| `.trae/rules/project_rules.md` | 修改 | 记录符号缺失问题 |

## 关于前端未生效的说明
前端代码改动（WebDAV 内联结果、Tasks 复制按钮、Settings 引擎详情）已经全部正确写入源码并通过 `vue-tsc --noEmit && vite build` 验证。要使改动生效，需要：

```bash
cd app/encv-mobile
npx vite build
npx cap sync android
cd android && ./gradlew assembleDebug
```
然后将生成的 APK 安装到设备上。
