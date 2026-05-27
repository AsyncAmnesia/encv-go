# Tasks

- [ ] Task 1: 修复 bin2c 符号命名参数
  - [ ] SubTask 1.1: 修改 `build-ffmpeg-android.sh` Phase 2 中 CSS 文件的 bin2c 调用，将名称参数从 `$(basename "$res_file" .css)` 改为 `$(basename "${res_file}.c" .c | tr '.' '_')`，使 `graph.css` 生成 `ff_graph_css_data` 而非 `ff_graph_data`
  - [ ] SubTask 1.2: 修改 HTML 文件的 bin2c 调用，将名称参数从 `$(basename "$res_file" .html)` 改为 `$(basename "${res_file}.c" .c | tr '.' '_')`，使 `graph.html` 生成 `ff_graph_html_data` 而非 `ff_graph_data`
  - [ ] SubTask 1.3: 验证修改后 CSS 和 HTML 生成的符号名不再冲突（`ff_graph_css_data` vs `ff_graph_html_data`）

- [ ] Task 2: 确认 CONFIG_RESOURCE_COMPRESSION 配置
  - [ ] SubTask 2.1: 检查 FFmpeg configure 输出的 `config.h` 中 `CONFIG_RESOURCE_COMPRESSION` 的值（0 或 1）
  - [ ] SubTask 2.2: 如果 `CONFIG_RESOURCE_COMPRESSION=1`，在 Phase 2 中添加 gzip 压缩步骤（CSS → `.css.min.gz`，HTML → `.html.gz`），并让 bin2c 处理压缩后的文件
  - [ ] SubTask 2.3: 如果 `CONFIG_RESOURCE_COMPRESSION=0`，确认当前直接处理 minified CSS / 原始 HTML 的流程正确

- [ ] Task 3: 添加构建后符号验证
  - [ ] SubTask 3.1: 在 Phase 4 链接完成后、strip 之前，使用 `nm`（非 `-D`）检查 `libffmpeg.so` 中是否包含 `ff_graph_css_data` 和 `ff_graph_html_data` 符号
  - [ ] SubTask 3.2: 如果符号缺失，输出明确的错误信息并终止构建
  - [ ] SubTask 3.3: 在 Phase 4 的符号验证步骤中增加对资源符号的检查（当前仅检查 `ffmpeg_run` / `ffmpeg_reset`）

- [ ] Task 4: 评估 --allow-multiple-definition 的必要性
  - [ ] SubTask 4.1: 确认 `--allow-multiple-definition` 是否仍需保留（FFmpeg 多库重复符号如 `ff_log2_tab`）
  - [ ] SubTask 4.2: 如果保留，添加注释说明原因；如果不再需要（符号名修复后无重复），考虑移除
  - [ ] SubTask 4.3: 考虑添加 `-Wl,--warn-multiple-definition` 以在未来捕获类似问题

- [ ] Task 5: 清除构建缓存并重新构建验证
  - [ ] SubTask 5.1: 删除 `.ffmpeg-build/` 缓存目录，确保从头构建
  - [ ] SubTask 5.2: 执行完整构建流程，确认无编译/链接错误
  - [ ] SubTask 5.3: 使用 `nm` 验证输出的 `libffmpeg.so` 和 `libffprobe.so` 中包含正确的资源符号

# Task Dependencies

- Task 2 依赖 Task 1（符号名修复后才能正确验证压缩配置）
- Task 3 依赖 Task 1（符号验证需要正确的符号名）
- Task 5 依赖 Task 1、2、3、4（所有修复完成后才能进行完整构建验证）
- Task 1 和 Task 4 可并行执行
