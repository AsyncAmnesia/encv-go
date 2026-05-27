# 修复 libffprobe.so 缺少 ffprobe_run 符号

## 根因

**`-Wl,--gc-sections`（死代码消除）将 `ffprobe_run` 作为未引用符号删除了。**

链接器在构建共享库时，从所有输入的 `.o` 文件出发做可达性分析：
- 如果一个函数/数据没有被任何其他代码**直接或间接引用**，就被视为"死代码"
- `ffprobe_run` 是 `.so` 的入口点（供 Go 端 dlopen 后调用），但在 `.so` **内部**没有任何其他函数调用它
- 因此链接器认为它是死代码并删除了

**为什么 `libffmpeg.so` 的 `ffmpeg_run` 没被删？**
`ffmpeg.c` 的调用图比 `ffprobe.c` 复杂得多——内部有大量初始化链、回调注册、全局构造等间接引用路径，恰好让 `ffmpeg_run` 保持在可达图中。这是**偶然正确**，不是设计保证。

## 修复方案

在两个 `.so` 的链接命令中添加 `-Wl,-u,SYMBOL`（即 `--undefined=SYMBOL`），明确告诉链接器："这个符号被视为外部必需，不得删除"。

### 修改文件

[`build-ffmpeg-android.sh`](file:///workspace/app/encv-mobile/scripts/build-ffmpeg-android.sh)

### 修改 1：libffmpeg.so 链接命令（第 330 行）

```diff
 $CC $CFLAGS -shared -o "${FTOOLS_BUILD}/libffmpeg.so" \
     $FFMPEG_OBJS \
     $STATIC_LIBS \
     ${X264_INSTALL}/lib/libx264.a \
     -lm -lz -llog \
+    -Wl,-u,ffmpeg_run \
+    -Wl,-u,ffmpeg_reset \
     -Wl,--gc-sections \
     -Wl,--allow-multiple-definition \
```

### 修改 2：libffprobe.so 链接命令（第 344 行）

```diff
 $CC $CFLAGS -shared -o "${FTOOLS_BUILD}/libffprobe.so" \
     $FFPROBE_OBJS \
     $STATIC_LIBS \
     ${X264_INSTALL}/lib/libx264.a \
     -lm -lz -llog \
+    -Wl,-u,ffprobe_run \
+    -Wl,-u,ffprobe_reset \
     -Wl,--gc-sections \
     -Wl,--allow-multiple-definition \
```

## 原理说明

| 标志 | 作用 |
|------|------|
| `-Wl,-u,SYMBOL` | 等价于 `--undefined=SYMBOL`，告诉链接器"存在一个外部未定义引用指向 SYMBOL"，因此 SYMBOL 的定义必须保留 |
| 与 `--gc-sections` 配合 | gc-sections 只删除**确实不可达**的代码；`-u` 让入口函数变为"可达"（因为假想的外部调用者需要它） |

这是构建插件式共享库的标准做法——入口点符号必须显式声明为不可 GC。
