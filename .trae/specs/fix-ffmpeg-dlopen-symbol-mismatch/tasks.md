# Tasks

- [x] Task 1: 修复 bin2c 符号命名参数
  - [x] SubTask 1.1: 修改 `build-ffmpeg-android.sh` Phase 2 中 CSS 文件的 bin2c 调用，将名称参数从 `$(basename "$res_file" .css)` 改为 `$(basename "${base}" | sed 's/\.[^.]*$//' | tr '.' '_')`，使 `graph.css` 生成 `ff_graph_css_data` 而非 `ff_graph_data`
  - [x] SubTask 1.2: 修改 HTML 文件的 bin2c 调用，使用相同的命名公式，使 `graph.html` 生成 `ff_graph_html_data` 而非 `ff_graph_data`
  - [x] SubTask 1.3: 验证修改后 CSS 和 HTML 生成的符号名不再冲突（`ff_graph_css_data` vs `ff_graph_html_data`）

- [x] Task 2: 确认 CONFIG_RESOURCE_COMPRESSION 配置
  - [x] SubTask 2.1: 检查 FFmpeg configure 输出的 `config.h` 中 `CONFIG_RESOURCE_COMPRESSION` 的值——FFmpeg 8.0 默认启用（当 zlib+gzip 可用时）
  - [x] SubTask 2.2: 添加 `--disable-resource-compression` 到 FFmpeg configure 命令，禁用 gzip 压缩（与构建脚本生成未压缩资源数据的方式一致）
  - [x] SubTask 2.3: 添加 Phase 2 后的验证步骤，检查 config.h 中 CONFIG_RESOURCE_COMPRESSION 是否为 0，若为 1 则报错退出

- [x] Task 3: 添加构建后符号验证
  - [x] SubTask 3.1: 在 Phase 4 链接完成后、strip 之前，使用 `nm`（非 `-D`）检查 `libffmpeg.so` 中是否包含 `ff_graph_css_data` 和 `ff_graph_html_data` 符号
  - [x] SubTask 3.2: 如果符号缺失，输出明确的错误信息并终止构建
  - [x] SubTask 3.3: 在 Phase 4 的符号验证步骤中增加对资源符号的检查

- [x] Task 4: 评估 --allow-multiple-definition 的必要性
  - [x] SubTask 4.1: 确认 `--allow-multiple-definition` 仍需保留（FFmpeg 多库重复符号如 `ff_log2_tab`）
  - [x] SubTask 4.2: 添加注释说明原因
  - [x] SubTask 4.3: 符号名修复后 fftools 自身不再有重复定义，但 FFmpeg 静态库间仍有重复符号，保留该选项

- [ ] Task 5: 清除构建缓存并重新构建验证
  - [ ] SubTask 5.1: 删除 `.ffmpeg-build/` 缓存目录，确保从头构建
  - [ ] SubTask 5.2: 执行完整构建流程，确认无编译/链接错误
  - [ ] SubTask 5.3: 使用 `nm` 验证输出的 `libffmpeg.so` 和 `libffprobe.so` 中包含正确的资源符号

# Task Dependencies

- Task 2 依赖 Task 1（符号名修复后才能正确验证压缩配置）
- Task 3 依赖 Task 1（符号验证需要正确的符号名）
- Task 5 依赖 Task 1、2、3、4（所有修复完成后才能进行完整构建验证）
- Task 1 和 Task 4 可并行执行

# 备注

- Task 5 需要在 CI 环境中执行（沙箱无法运行 Java/Gradle/NDK 构建）
- 构建缓存清理命令：`rm -rf app/encv-mobile/scripts/.ffmpeg-build/`
- 构建后验证命令：`nm <build_dir>/ftools-build/libffmpeg.so | grep ff_graph_css_data`
