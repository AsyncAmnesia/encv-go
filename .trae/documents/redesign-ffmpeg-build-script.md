# 重构 FFmpeg Android 构建脚本：根治 graph 模块问题

## 根因分析（深度）

### 为什么 `ff_graph_css_data` 缺失

FFmpeg 8.0 的 `fftools/resources/` 目录中包含**非 C 语言资源文件**：

| 文件 | 类型 | 用途 |
|------|------|------|
| `graph.css` | CSS | Mermaid HTML 输出的样式表 |
| `graph.html` | HTML | Mermaid HTML 输出的模板 |
| `resman.c` | C | 资源管理器（读取上述数据） |

这些文件**不是 .c 文件**，不能直接编译。FFmpeg 官方构建系统通过以下管道将它们转换为可编译的 C 数组：

```
graph.css → [sed minify] → graph.css.min → [可选 gzip] → [bin2c 工具] → graph_css.c
                                                              ↓
                                              生成 const unsigned char ff_graph_css_data[]
                                              生成 const unsigned int ff_graph_css_len

graph.html → [可选 gzip] → [bin2c 工具] → graph_html.c
                                        ↓
                          生成 const unsigned char ff_graph_html_data[]
```

其中 `bin2c` 是 FFmpeg 自带的主机端工具（`ffbuild/bin2c.c`），负责将任意二进制/文本文件转换为 C 数组。

### 当前脚本为什么失败

[build-ffmpeg-android.sh](file:///workspace/app/encv-mobile/scripts/build-ffmpeg-android.sh) 第 232-239 行：

```bash
FFMPEG_OPTIONAL_DIRS="fftools/textformat fftools/resources"
for dir in $FFMPEG_OPTIONAL_DIRS; do
    if [ -d "${FFMPEG_SRC}/${dir}" ]; then
        for f in ${FFMOL_SRC}/${dir}/*.c; do    # ← 只匹配已存在的 .c 文件！
            [ -f "$f" ] && FFMPEG_FFTOOLS="$FFMPEG_FFTOOLS ${dir}/$(basename $f)"
        done
    fi
done
```

**致命缺陷**：`resources/` 目录下只有 `resman.c` 一个原生 .c 文件。`graph.css.c` 和 `graph.html.c` 是**构建时生成**的，脚本从未运行 `bin2c` 管道来生成它们。因此：
- `graphprint.c` 编译通过（它引用 `ff_graph_css_data` 的 **声明**）
- 链接时找不到 `ff_graph_css_data` 的 **定义**（因为 `graph.css.c` 从未生成）

### 设计层面的根本问题

当前脚本的架构是：**手动枚举每个 .c 文件并逐个编译**。这个方案存在系统性脆弱性：

1. **无法处理生成式源码**：`.css→.c`、`.html→.c`、`.ptx→.c` 等管道全部缺失
2. **每次 FFmpeg 升级都要同步**：新增目录/文件/依赖关系变化都会导致编译失败或链接缺失符号
3. **符号检查是打地鼠**：每修一个缺失符号就加一个检查，但根本问题没解决
4. **代码膨胀且难以维护**：大量 shell 循环、条件判断、日志重定向散布各处

## 解决方案：让 FFmpeg 自己的 Makefile 构建 fftools

### 核心思路

**不再手动枚举和编译 .c 文件**，改为利用 FFmpeg 已有的构建基础设施：

```
Phase 1: configure + make          → 构建静态库 .a（不变）
Phase 2: make ffmpeg_g ffprobe_g   → 让 FFmpeg Makefile 处理所有 fftools 编译
                                    → 自动处理 bin2c 资源生成、依赖关系、include 路径等
Phase 3: 从 _g 目标提取 .o 并重链接为 .so  → 产出 libffmpeg.so / libffprobe.so
```

关键优势：
- `make ffmpeg_g` 会自动执行完整的资源生成管道（bin2c、CSS minify 等）
- 新增文件/目录时无需修改脚本——Makefile 自动处理
- 所有编译标志、头文件路径、宏定义都由 configure 正确设置
- 彻底消除"遗漏某个 .c 文件"的可能性

### 具体实现步骤

#### 步骤 1：重构 Phase 2 — 使用 `make` 构建 fftools 对象

删除当前第 191-278 行的手动编译逻辑，替换为：

```bash
echo "=== Building fftools via FFmpeg Makefile ==="
cd "${FFMPEG_SRC}"

# 构建 fftools 的 _g（unstripped）版本
# FFmpeg Makefile 会自动处理：
#   - 编译 bin2c 主机工具
#   - graph.css → graph_css.c（含 ff_graph_css_data 符号）
#   - graph.html → graph_html.c（含 ff_graph_html_data 符号）
#   - 所有 fftools/graph/*.c、fftools/textformat/*.c、fftools/resources/*.c
#   - 正确的依赖顺序和编译标志
make ffmpeg_g ffprobe_g -j$(nproc) > "${LOG_DIR}/fftools-make.log" 2>&1
echo "✅ fftools built via make (log: ${LOG_DIR}/fftools-make.log})"
```

#### 步骤 2：重构 Phase 3 — 从 Makefile 产物提取并链接为 .so

`make ffmpeg_g` 产生的 `ffmpeg_g` 是未剥离的可执行文件。我们需要从中提取目标代码并重新链接为共享库。

**方法 A（推荐）：利用 FFmpeg Makefile 的中间 .o 文件**

FFmpeg Makefile 在构建过程中会生成所有 `.o` 文件到源码树下。我们可以直接使用这些 .o 文件进行重链接：

```bash
echo "=== Linking libffmpeg.so from fftools objects ==="

# 收集 FFmpeg Makefile 生成的 fftools .o 文件
# 这些文件位于 ${FFMPEG_SRC}/fftools/**/*.o 和相关子目录
FFMPEG_OBJS=$(find "${FFMPEG_SRC}/fftools" -name '*.o' ! -name '*ffprobe*' ! -name '*ffplay*')
FFPROBE_OBJS=$(find "${FFMPEG_SRC}/fftools" -name 'ffprobe.o' -o -name 'cmdutils.o' -o -name 'opt_common.o')
FFPROBE_OBJS="$FFPROBE_OBJS $(find "${FFMPEG_SRC}/fftools/textformat" -name '*.o')

$CC -shared -o "${FTOOLS_BUILD}/libffmpeg.so" \
    $FFMPEG_OBJS $STATIC_LIBS ${X264_INSTALL}/lib/libx264.a \
    -lm -lz -llog \
    -Wl,--gc-sections -Wl,--allow-multiple-definition \
    > "${LOG_DIR}/link_ffmpeg.log" 2>&1

$CC -shared -o "${FTOOLS_BUILD}/libffprobe.so" \
    $FFPROBE_OBJS $STATIC_LIBS ${X264_INSTALL}/lib/libx264.a \
    -lm -lz -llog \
    -Wl,--gc-sections -Wl,--allow-multiple-definition \
    > "${LOG_DIR}/link_ffprobe.log" 2>&1
```

> **注意**：`find` 收集 .o 的方式比手动枚举更健壮——新增任何源文件都会被自动包含。

#### 步骤 3：简化验证逻辑

移除针对特定符号（如 `ff_graph_css_data`）的专项检查。改用更通用的验证：

```bash
echo "=== Verifying exported symbols ==="
for lib in libffmpeg.so libffprobe.so; do
    echo "--- ${lib} ---"
    SYMS=$(${NM} -D "${OUTPUT_DIR}/${lib}" | grep -c "T ")
    echo "  ${SYMS} exported text symbols"
done

# 只验证我们真正调用的入口函数
for pair in "libffmpeg.so:ffmpeg_run:ffmpeg_reset" "libffprobe.so:ffprobe_run:ffprobe_reset"; do
    LIB=$(echo $pair | cut -d: -f1)
    ENTRY=$(echo $pair | cut -d: -f2)
    RESET=$(echo $pair | cut -d: -f3)
    if ${NM} -D "${OUTPUT_DIR}/${LIB}" | grep -q " ${ENTRY}$"; then
        echo "  ✅ ${ENTRY} found in ${LIB}"
    else
        echo "  ❌ ${ENTRY} NOT found in ${LIB}"
        exit 1
    fi
done
```

#### 步骤 4：清理 — 删除废弃的手动编译变量

删除以下不再需要的变量和逻辑块：
- `FFMPEG_CORE_FFTOOLS`（第 217 行）— 手动枚举的核心文件列表
- `FFMPEG_GRAPH_FFTOOLS`（第 224-230 行）— 手动扫描 graph 目录
- `FFMPEG_FFTOOLS` 的逐文件编译循环（第 252-278 行）— 整个核心编译循环
- `FFPROBE_FFTOOLS` 的手动定义（第 241-249 行）
- `is_core` / `is_graph` 分类逻辑（第 259-266 行）
- `CFLAGS` 中重复的 `-I` 路径（Makefile 会自动处理）
- `ff_graph_css_data` 专项检查（第 341-346 行）

## 修改后的脚本结构（概览）

```bash
#!/bin/bash
set -euo pipefail

# ── 配置区（不变） ──────────────────────────
FFMPEG_VERSION="8.0"
# ...

# ── 缓存检查（简化，只查入口函数） ──────────
if [ -f "${OUTPUT_DIR}/libffmpeg.so" ]; then
    # 只验证 ffmpeg_run / ffprobe_run 存在
    # 不再检查内部实现细节如 ff_graph_css_data
fi

# ── Phase 1: x264 构建（不变） ───────────────
# ...

# ── Phase 2: FFmpeg 库构建（不变） ────────────
# ./configure ... && make && make install

# ── Phase 3: fftools 构建（重构！） ───────────
# make ffmpeg_g ffprobe_g    ← 替代手动 .c 编译

# ── Phase 4: 重链接为 .so（重构！） ────────────
# find .o → $CC -shared → libffmpeg.so / libffprobe.so

# ── 剥离 + 验证（简化） ───────────────────────
# llvm-strip + nm 验证入口函数

# ── build-info.json（不变） ───────────────────
```

## 风险评估与回退

| 风险 | 概率 | 应对 |
|------|------|------|
| `make ffmpeg_g` 因缺少依赖失败 | 低 | Phase 1 的 `make` 已构建所有依赖库；如果失败查看 `${LOG_DIR}/fftools-make.log` |
| find 收集到不需要的 .o（如 ffplay） | 低 | 使用 `! -name '*ffplay*'` 排除；或用 OBJS-ffmpeg 变量从 Makefile 直接读取 |
| .o 文件路径在 FFmpeg 版本间变化 | 极低 | `find` 是基于目录结构的递归搜索，不依赖具体路径 |

## 预期效果

- **消除** `ff_graph_css_data` 类的符号缺失错误（根除）
- **未来升级 FFmpeg 时**只需更新版本号，无需同步修改文件列表
- **脚本行数减少约 40%**（删除手动编译循环和分类逻辑）
- **构建确定性提升**：FFmpeg Makefile 保证正确的编译顺序和依赖
