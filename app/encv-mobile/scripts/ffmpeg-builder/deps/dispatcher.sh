#!/usr/bin/env bash
# deps/dispatcher.sh - 按 manifest external_libs 顺序构建所有 deps
#
# 用法（在 ffmpeg.sh 主链）：
#   source deps/dispatcher.sh
#   build_all_deps
#
# 强制要求：load_manifest 已执行（$EXTERNAL_LIBS 有值）
# 不引入新 deps：直接 return

set -euo pipefail

# 兄弟库 downloads.sh 提供 download_and_verify / extract_to / find_source_root
# 每个 dep build_* 都会用到，先 source
# shellcheck source=../common/downloads.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../common/downloads.sh"

# 单 dep 名字 → build 函数映射
# 加新 dep 在这里加一行，再写 deps/<name>.sh
_dep_builders() {
    local name="$1"
    case "$name" in
        libx264)     build_x264     ;;
        libmp3lame)  build_libmp3lame ;;
        *)
            die "No builder for dep: $name (add deps/${name}.sh + entry in _dep_builders)" ;;
    esac
}

build_all_deps() {
    log_section "build deps: $EXTERNAL_LIBS"
    require_cmd make "Install build-essential / xcode-select --install"
    require_cmd nproc "coreutils required"

    local dep
    for dep in $EXTERNAL_LIBS; do
        # source 对应 dep 脚本
        local dep_script="${BUILDER_DIR}/deps/${dep}.sh"
        if [ ! -f "$dep_script" ]; then
            die "dep script missing: $dep_script (declare in manifest.external_libs but no build file)"
        fi
        # shellcheck disable=SC1090
        source "$dep_script"
        _dep_builders "$dep"
    done
    log_ok "all deps built: $EXTERNAL_LIBS"
}
